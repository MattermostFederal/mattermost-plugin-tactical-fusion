package dtg

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/decorators"
)

func TestParseProducesResolvedParams(t *testing.T) {
	d := &Decorator{}

	params, ok := d.Parse("091630ZAUG26", ref)
	if !ok {
		t.Fatal("Parse rejected a valid DTG")
	}

	want := time.Date(2026, time.August, 9, 16, 30, 0, 0, time.UTC).UnixMilli()
	if got := params.Get("t"); got != strconv.FormatInt(want, 10) {
		t.Fatalf("t = %s, want %d", got, want)
	}
	if got := params.Get("dtg"); got != "091630ZAUG26" {
		t.Fatalf("dtg = %s, want 091630ZAUG26", got)
	}
	if got := params.Get("z"); got != "Z" {
		t.Fatalf("z = %s, want Z", got)
	}
	if got := params.Get("a"); got != "" {
		t.Fatalf("a = %q, want empty", got)
	}
}

func TestParseRejectionReturnsNotOK(t *testing.T) {
	d := &Decorator{}

	if _, ok := d.Parse("311200ZFEB26", ref); ok {
		t.Fatal("Parse accepted 31 February")
	}
}

// The patterns must match the tokens the tagger will hand to Parse, and must
// not match things that merely look similar.
func TestPatternsMatchExpectedTokens(t *testing.T) {
	d := &Decorator{}

	shouldMatch := []string{"091630ZAUG26", "091630ZAUG2026", "091630Z"}
	for _, value := range shouldMatch {
		t.Run("match "+value, func(t *testing.T) {
			if !anyPatternMatches(d, value) {
				t.Fatalf("no pattern matched %q", value)
			}
		})
	}

	shouldNotMatch := []string{
		"091630",       // no zone letter
		"91630ZAUG26",  // only five leading digits
		"ABC123",       // not a DTG at all
		"0916302AUG26", // digit where the zone letter goes
	}
	for _, value := range shouldNotMatch {
		t.Run("no match "+value, func(t *testing.T) {
			if anyPatternMatches(d, value) {
				t.Fatalf("a pattern matched %q, want no match", value)
			}
		})
	}
}

// A six digit run followed by a letter must not be claimed as a short-form DTG
// when it is really the start of a longer word, which is what \b guards.
func TestShortFormDoesNotMatchInsideLongerToken(t *testing.T) {
	d := &Decorator{}
	short := patternFor(t, d, `\b`+dtgBareExpr+`\b`)

	for _, value := range []string{"091630ZAUG26", "091630Zfoo", "091630Z_foo"} {
		if short.Regexp.MatchString(value) {
			t.Fatalf("the short pattern matched inside %q, want no match", value)
		}
	}
}

// patternFor finds a shipped pattern by its source.
//
// By source rather than by position: indexing into Patterns() couples a test to
// the order they happen to be declared in, which is a tiebreak detail, and
// silently retargets the test at a different pattern the day one is inserted.
func patternFor(t *testing.T, d *Decorator, source string) decorators.Pattern {
	t.Helper()

	for _, p := range d.Patterns() {
		if p.Regexp.String() == source {
			return p
		}
	}

	t.Fatalf("no pattern with source %q", source)
	return decorators.Pattern{}
}

func anyPatternMatches(d *Decorator, value string) bool {
	for _, p := range d.Patterns() {
		if loc := p.Regexp.FindStringIndex(value); loc != nil && loc[0] == 0 && loc[1] == len(value) {
			return true
		}
	}
	return false
}

func TestRenderPageRendersTable(t *testing.T) {
	d := &Decorator{}
	params, ok := d.Parse("091630ZAUG26", ref)
	if !ok {
		t.Fatal("Parse rejected a valid DTG")
	}

	rec := httptest.NewRecorder()
	d.RenderPage(rec, params)

	if rec.Code != 200 {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	body := rec.Body.String()

	for _, want := range []string{"091630ZAUG26", "Zulu (UTC)", "16:30", "<table>"} {
		if !strings.Contains(body, want) {
			t.Fatalf("body is missing %q", want)
		}
	}
	// The body carries a relative offset and a wall clock, so it is not a
	// function of its params and must not be cached.
	if got := rec.Header().Get("Cache-Control"); got != "no-store" {
		t.Fatalf("Cache-Control = %q, want no-store", got)
	}
}

// The page must be complete without JavaScript: the only script is the live
// clock, which is pure enhancement.
func TestRenderPageWorksWithoutJavaScript(t *testing.T) {
	d := &Decorator{}
	params, _ := d.Parse("091630ZAUG26", ref)

	rec := httptest.NewRecorder()
	d.RenderPage(rec, params)

	beforeScript, _, found := strings.Cut(rec.Body.String(), "<script")
	if !found {
		t.Fatal("expected a live clock script")
	}
	if !strings.Contains(beforeScript, "<table>") {
		t.Fatal("the timezone table must render before, and independently of, any script")
	}
}

func TestRenderPageShowsAssumedNote(t *testing.T) {
	d := &Decorator{}
	params, ok := d.Parse("091630Z", ref)
	if !ok {
		t.Fatal("Parse rejected a valid short-form DTG")
	}

	rec := httptest.NewRecorder()
	d.RenderPage(rec, params)

	if !strings.Contains(rec.Body.String(), "taken from the date the message was posted") {
		t.Fatal("the short form must say that month and year were inferred")
	}
}

// Params arrive from an untrusted URL, so every one is re-validated.
func TestRenderPageRejectsInvalidParams(t *testing.T) {
	valid, ok := (&Decorator{}).Parse("091630ZAUG26", ref)
	if !ok {
		t.Fatal("Parse rejected a valid DTG")
	}

	// Positive control. Without it a wrong baseline would make every case below
	// pass for the wrong reason.
	t.Run("the unmutated baseline renders", func(t *testing.T) {
		rec := httptest.NewRecorder()
		(&Decorator{}).RenderPage(rec, valid)
		if rec.Code != 200 {
			t.Fatalf("status = %d, want 200 for the baseline params", rec.Code)
		}
	})

	cases := []struct {
		name   string
		mutate func(url.Values)
	}{
		{"missing t", func(v url.Values) { v.Del("t") }},
		{"non-numeric t", func(v url.Values) { v.Set("t", "abc") }},
		{"negative t", func(v url.Values) { v.Set("t", "-1") }},
		{"absurdly large t", func(v url.Values) { v.Set("t", "99999999999999") }},
		{"missing dtg", func(v url.Values) { v.Del("dtg") }},
		{"malformed dtg", func(v url.Values) { v.Set("dtg", "not-a-dtg") }},
		{"missing z", func(v url.Values) { v.Del("z") }},
		{"invalid zone letter", func(v url.Values) { v.Set("z", "J") }},
		{"multi-character z", func(v url.Values) { v.Set("z", "ZZ") }},
		{"unknown assumed code", func(v url.Values) { v.Set("a", "x") }},

		// Each parameter below is individually well-formed but disagrees with
		// the others. On a public route the URL is user-supplied, so a crafted
		// link could otherwise show an arbitrary DTG beside an unrelated
		// instant and a third zone.
		{"instant does not match the dtg", func(v url.Values) { v.Set("t", "0") }},
		{"zone does not match the dtg", func(v url.Values) { v.Set("z", "B") }},
		{"assumed code claims more than the dtg does", func(v url.Values) { v.Set("a", "my") }},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			params := url.Values{}
			for k, v := range valid {
				params[k] = append([]string(nil), v...)
			}
			tc.mutate(params)

			rec := httptest.NewRecorder()
			(&Decorator{}).RenderPage(rec, params)

			if rec.Code != 400 {
				t.Fatalf("status = %d, want 400 for %s", rec.Code, tc.name)
			}
		})
	}
}

// Nothing from the query string reaches the page unescaped.
func TestRenderPageEscapesHostileParams(t *testing.T) {
	params := url.Values{
		"t":   {"1786293000000"},
		"dtg": {`091630Z"><script>alert(1)</script>`},
		"z":   {"Z"},
		"a":   {""},
	}

	rec := httptest.NewRecorder()
	(&Decorator{}).RenderPage(rec, params)

	// The canonical grammar rejects it outright, which is the first line of
	// defence, and the error page echoes nothing from the request.
	if rec.Code != 400 {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
	if strings.Contains(rec.Body.String(), "<script>alert(1)</script>") {
		t.Fatal("an unescaped script tag reached the page")
	}
}

// This table is duplicated in webapp/src/decorators/dtg/relative.spec.ts on
// purpose. The sidebar, this server-rendered page and the page's countdown
// script all format the same instant, so a divergence would show the same DTG
// two different ways depending on where you looked.
func TestRelativeTo(t *testing.T) {
	now := time.Date(2026, time.August, 9, 12, 0, 0, 0, time.UTC)

	cases := []struct {
		name   string
		target time.Time
		want   string
	}{
		{"same instant", now, "now"},
		{"seconds only", now.Add(15 * time.Second), "in 15s"},
		{"minutes keep seconds", now.Add(65 * time.Second), "in 1m 5s"},
		{"hours keep everything below", now.Add(3*time.Hour + 20*time.Minute + 15*time.Second), "in 3h 20m 15s"},
		{"zero units are kept once a larger one appears", now.Add(time.Hour), "in 1h 0m 0s"},
		{"days keep everything below", now.Add(50 * time.Hour), "in 2d 2h 0m 0s"},
		{"past instants count up", now.Add(-90 * time.Minute), "1h 30m 0s ago"},
		{"past seconds", now.Add(-5 * time.Second), "5s ago"},

		// Sub-second differences round toward "now" rather than showing
		// "in 0s", so the display never sits on a value that reads as though
		// nothing is happening.
		{"sub-second ahead", now.Add(400 * time.Millisecond), "now"},
		{"sub-second behind", now.Add(-400 * time.Millisecond), "now"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := relativeTo(now, tc.target); got != tc.want {
				t.Fatalf("relativeTo() = %q, want %q", got, tc.want)
			}
		})
	}
}

// The heading names the category, not the value. The canonical DTG is already
// the first line of the body. This must match PANEL_TITLE in
// webapp/src/decorators/dtg/index.ts so the page and the sidebar agree.
func TestPageHeadingIsTheCategory(t *testing.T) {
	d := &Decorator{}
	params, _ := d.Parse("091630ZAUG26", ref)

	rec := httptest.NewRecorder()
	d.RenderPage(rec, params)

	body := rec.Body.String()
	for _, want := range []string{"<title>Date/Time</title>", "<h1>Date/Time</h1>"} {
		if !strings.Contains(body, want) {
			t.Fatalf("page is missing %q", want)
		}
	}
}

// This table is duplicated in webapp/src/decorators/dtg/describe.spec.ts on
// purpose. The sidebar and this page describe the same instant, so a divergence
// would read as two different times for one message.
func TestDescribeInstant(t *testing.T) {
	cases := []struct {
		name    string
		instant time.Time
		zone    byte
		want    string
	}{
		// A zone letter names a whole-hour offset; the offset below is derived
		// from it, exactly as validateParams does.
		{
			"zulu is shown as stored",
			time.Date(2026, time.August, 9, 16, 30, 0, 0, time.UTC), 'Z',
			"09 Aug 2026 16:30 Z",
		},
		{
			// 16:30Z is 18:30 in B, which is UTC+2.
			"a positive zone is shown in its own local time",
			time.Date(2026, time.August, 9, 16, 30, 0, 0, time.UTC), 'B',
			"09 Aug 2026 18:30 B",
		},
		{
			// 16:30Z is 11:30 in R, which is UTC-5.
			"a negative zone is shown in its own local time",
			time.Date(2026, time.August, 9, 16, 30, 0, 0, time.UTC), 'R',
			"09 Aug 2026 11:30 R",
		},
		{
			// 01:00Z in M, which is UTC+12, is the 9th at 13:00.
			"the zone offset can roll the date back",
			time.Date(2026, time.August, 9, 1, 0, 0, 0, time.UTC), 'M',
			"09 Aug 2026 13:00 M",
		},
		{
			// 23:00Z in R, which is UTC-5, is still the 9th at 18:00.
			"the zone offset can roll the date forward",
			time.Date(2026, time.August, 9, 23, 0, 0, 0, time.UTC), 'R',
			"09 Aug 2026 18:00 R",
		},
		{
			"single digit days and months are padded",
			time.Date(2026, time.January, 1, 5, 7, 0, 0, time.UTC), 'Z',
			"01 Jan 2026 05:07 Z",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			offset, ok := zoneOffsetHours(tc.zone)
			if !ok {
				t.Fatalf("zoneOffsetHours(%c) reported unknown", tc.zone)
			}

			if got := describeInstant(tc.instant, offset*60, string(tc.zone)); got != tc.want {
				t.Fatalf("describeInstant() = %q, want %q", got, tc.want)
			}
		})
	}
}

// Both renderings lead with how far away the instant is, then the plain-language
// reading, then the token as the author wrote it. Asserting the order here is
// what keeps the page and the sidebar telling the same story.
func TestPageOrdersCountdownFirst(t *testing.T) {
	d := &Decorator{}
	params, _ := d.Parse("091630ZAUG26", ref)

	rec := httptest.NewRecorder()
	d.RenderPage(rec, params)
	body := rec.Body.String()

	countdown := strings.Index(body, `class="countdown"`)
	described := strings.Index(body, `class="described"`)
	canonical := strings.Index(body, `class="dtg"`)

	if countdown < 0 || described < 0 || canonical < 0 {
		t.Fatalf("missing an element: countdown=%d described=%d canonical=%d", countdown, described, canonical)
	}
	if countdown >= described || described >= canonical {
		t.Fatalf("wrong order: countdown=%d described=%d canonical=%d", countdown, described, canonical)
	}
	if !strings.Contains(body, "09 Aug 2026 16:30 Z") {
		t.Fatal("the page is missing the plain-language reading")
	}
}

// The page cannot read the webapp's CSS variables, so the webapp tells it which
// theme to paint itself with. Without the hint it falls back to the operating
// system preference.
func TestPageHonoursTheThemeParam(t *testing.T) {
	base := "t=1786293000000&dtg=091630ZAUG26&z=Z&a="

	cases := []struct {
		name  string
		query string
		want  string
	}{
		{"no hint follows the operating system", base, ""},
		{"dark", base + "&_theme=dark", ` data-theme="dark"`},
		{"light", base + "&_theme=light", ` data-theme="light"`},

		// The value reaches a stylesheet, so anything unrecognised is dropped
		// rather than echoed.
		{"unknown keyword", base + "&_theme=neon", ""},
		{"injection attempt", base + `&_theme="><script>alert(1)</script>`, ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			params, err := url.ParseQuery(tc.query)
			if err != nil {
				t.Fatalf("bad test query: %v", err)
			}

			rec := httptest.NewRecorder()
			(&Decorator{}).RenderPage(rec, params)

			if rec.Code != 200 {
				t.Fatalf("status = %d, want 200", rec.Code)
			}

			// Only the root tag matters. The stylesheet mentions data-theme in
			// its own selectors, which is not the same thing.
			body := rec.Body.String()
			_, fromRoot, found := strings.Cut(body, "<html")
			if !found {
				t.Fatal("no <html> tag in the page")
			}
			openTag, _, found := strings.Cut("<html"+fromRoot, ">")
			if !found {
				t.Fatal("unterminated <html> tag")
			}

			if got := strings.TrimPrefix(openTag, `<html lang="en"`); got != tc.want {
				t.Fatalf("root tag attribute = %q, want %q", got, tc.want)
			}
		})
	}
}

// An explicit light theme has to survive a reader whose machine prefers dark,
// since that mismatch is the whole reason the hint exists.
func TestExplicitLightBeatsTheSystemPreference(t *testing.T) {
	params, _ := url.ParseQuery("t=1786293000000&dtg=091630ZAUG26&z=Z&a=&_theme=light")

	rec := httptest.NewRecorder()
	(&Decorator{}).RenderPage(rec, params)

	body := rec.Body.String()
	if !strings.Contains(body, `:root:not([data-theme="light"])`) {
		t.Fatal("the dark media query is not guarded against an explicit light theme")
	}
}

// This table is duplicated in webapp/src/decorators/dtg/relative.spec.ts on
// purpose. The sidebar, this page and the page's countdown script all decide
// urgency independently, so a divergence would flash in one place and not
// another.
func TestIsUrgent(t *testing.T) {
	now := time.Date(2026, time.August, 9, 12, 0, 0, 0, time.UTC)

	cases := []struct {
		name   string
		offset time.Duration
		want   bool
	}{
		{"at the instant", 0, true},
		{"well inside, ahead", 5 * time.Minute, true},
		{"well inside, behind", -5 * time.Minute, true},
		{"one second inside, ahead", (30 * time.Minute) - time.Second, true},
		{"exactly on the threshold, ahead", 30 * time.Minute, true},
		{"exactly on the threshold, behind", -30 * time.Minute, true},
		{"one second outside, ahead", (30 * time.Minute) + time.Second, false},
		{"one second outside, behind", -((30 * time.Minute) + time.Second), false},
		{"far away", 48 * time.Hour, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := isUrgent(now, now.Add(tc.offset)); got != tc.want {
				t.Fatalf("isUrgent(%s) = %v, want %v", tc.offset, got, tc.want)
			}
		})
	}
}

// The countdown is marked urgent server-side too, so it is already red before
// any script runs.
func TestPageMarksAnImminentCountdownUrgent(t *testing.T) {
	d := &Decorator{}

	imminent, ok := d.Parse(FormatZulu(time.Now().Add(10*time.Minute)), time.Now())
	if !ok {
		t.Fatal("Parse rejected a generated DTG")
	}
	distant, ok := d.Parse(FormatZulu(time.Now().Add(48*time.Hour)), time.Now())
	if !ok {
		t.Fatal("Parse rejected a generated DTG")
	}

	cases := []struct {
		name   string
		params url.Values
		want   string
	}{
		{"imminent", imminent, `class="countdown urgent"`},
		{"distant", distant, `class="countdown"`},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			d.RenderPage(rec, tc.params)

			if !strings.Contains(rec.Body.String(), tc.want) {
				t.Fatalf("page is missing %q", tc.want)
			}
		})
	}
}

// The pulse runs for every reader by decision, so the two mitigations that
// remain have to hold: a rate well below what can trigger photosensitivity, and
// a signal that does not rest on movement or colour alone.
func TestUrgentPulseIsAccessible(t *testing.T) {
	d := &Decorator{}
	params, _ := d.Parse("091630ZAUG26", ref)

	rec := httptest.NewRecorder()
	d.RenderPage(rec, params)
	body := rec.Body.String()

	for _, want := range []string{
		"@keyframes mc-pulse",

		// Roughly 0.8Hz, against a 3Hz photosensitivity threshold. Shortening
		// this materially needs the same conversation again.
		"animation: mc-pulse 1.2s",

		"border-left: 4px solid var(--urgent)",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("page is missing %q", want)
		}
	}

	// Deliberately absent. See the comment on the rule in decorators/page.go.
	if strings.Contains(body, "prefers-reduced-motion") {
		t.Fatal("the reduced-motion guard is back; it was removed on purpose")
	}
}

// The page must no longer show a wall clock: the only time-dependent value is
// the countdown.
func TestRenderPageHasNoWallClock(t *testing.T) {
	d := &Decorator{}
	params, _ := d.Parse("091630ZAUG26", ref)

	rec := httptest.NewRecorder()
	d.RenderPage(rec, params)

	body := rec.Body.String()
	for _, unwanted := range []string{"Current time", `id="now"`} {
		if strings.Contains(body, unwanted) {
			t.Fatalf("page still contains %q", unwanted)
		}
	}
	if !strings.Contains(body, `id="rel"`) {
		t.Fatal("page is missing the countdown element")
	}
}

// The day badge is measured against the DTG's own zone date. Measuring against
// UTC reads as wrong for a DTG that was never expressed in UTC.
func TestDayDeltaIsRelativeToTheDTGZone(t *testing.T) {
	// M is UTC+12, so 01:00M on the 9th is 13:00Z on the 8th. The reader wrote
	// "the 9th", so the UTC row must be badged as a day behind that, not the
	// other way round.
	parsed, ok := parseDTG("090100MAUG26", ref)
	if !ok {
		t.Fatal("parseDTG rejected a valid DTG")
	}
	instant := parsed.resolveInstant()

	if want := time.Date(2026, time.August, 8, 13, 0, 0, 0, time.UTC); !instant.Equal(want) {
		t.Fatalf("resolveInstant() = %s, want %s", instant, want)
	}

	reference := zoneReferenceDate(instant, 12*60)
	if reference.Day() != 9 {
		t.Fatalf("zone reference day = %d, want 9, the day the author wrote", reference.Day())
	}
	if got := dayDelta(reference, instant.UTC()); got != -1 {
		t.Fatalf("dayDelta() = %d, want -1", got)
	}

	// The same instant measured against UTC would report 0, which is the bug
	// this baseline choice avoids.
	if got := dayDelta(instant.UTC(), instant.UTC()); got != 0 {
		t.Fatalf("dayDelta() against UTC = %d, want 0", got)
	}
}

// RFC 3339 is the one other format safe to claim in general chat: the T and the
// mandatory zone make an accidental match essentially impossible.
func TestPatternsMatchTimestamps(t *testing.T) {
	d := &Decorator{}
	accepted := []struct {
		token     string
		canonical string
		offset    string
	}{
		{"2026-08-09T16:30:00Z", "2026-08-09T16:30:00Z", "0"},
		{"2026-08-09T16:30Z", "2026-08-09T16:30:00Z", "0"},
		{"2026-08-09T16:30:00.500Z", "2026-08-09T16:30:00Z", "0"},
		{"2026-08-09T20:30:00+04:00", "2026-08-09T20:30:00+04:00", "240"},
		{"2026-08-09T11:30:00-05:00", "2026-08-09T11:30:00-05:00", "-300"},

		// Half-hour offsets are exactly what a military zone letter cannot say.
		{"2026-08-09T22:00:00+05:30", "2026-08-09T22:00:00+05:30", "330"},

		// RFC 3339 permits a lowercase t and z.
		{"2026-08-09t16:30:00z", "2026-08-09T16:30:00Z", "0"},
	}

	for _, tc := range accepted {
		t.Run(tc.token, func(t *testing.T) {
			if !matchesAnyPattern(d, tc.token) {
				t.Fatalf("no pattern matched %q", tc.token)
			}

			params, ok := d.Parse(tc.token, ref)
			if !ok {
				t.Fatalf("Parse rejected %q", tc.token)
			}
			if got := params.Get("dtg"); got != tc.canonical {
				t.Fatalf("canonical = %q, want %q", got, tc.canonical)
			}
			if got := params.Get("o"); got != tc.offset {
				t.Fatalf("offset = %q, want %q", got, tc.offset)
			}

			// A timestamp says "o" and never "z": that is what tells the two
			// forms apart, and every link already in a message keeps its "z".
			if params.Has("z") {
				t.Fatalf("a timestamp emitted a zone letter: %v", params)
			}

			// Whichever offset it was written in, it means one instant.
			instant := time.Date(2026, time.August, 9, 16, 30, 0, 0, time.UTC)
			if got := params.Get("t"); got != strconv.FormatInt(instant.UnixMilli(), 10) {
				t.Fatalf("t = %q, want %d", got, instant.UnixMilli())
			}
		})
	}
}

// Everything here would be a corruption of somebody's message.
func TestTimestampsDeclineTheUnsafe(t *testing.T) {
	declined := []string{
		// The basic form turns up inside filenames, where a hyphen is a word
		// boundary and this would match mid-attachment-name.
		"20260809T163000Z",

		// No zone, so there is no instant to resolve without inventing one.
		"2026-08-09T16:30:00",
		"2026-08-09 16:30:00",
		"2026-08-09",

		// Epoch seconds: any ten-digit number would match.
		"1786293000",

		// Real-looking but impossible.
		"2026-02-30T16:30:00Z",
		"2026-13-09T16:30:00Z",
		"2026-08-09T25:30:00Z",
		"2026-08-09T16:60:00Z",

		// An offset no place on earth uses.
		"2026-08-09T16:30:00+99:00",
	}

	d := &Decorator{}
	for _, token := range declined {
		t.Run(token, func(t *testing.T) {
			if !matchesAnyPattern(d, token) {
				// Never matched, so it never reaches Parse. That is a pass.
				return
			}
			if _, ok := d.Parse(token, ref); ok {
				t.Fatalf("Parse accepted %q, which would rewrite somebody's message", token)
			}
		})
	}
}

func matchesAnyPattern(d *Decorator, token string) bool {
	for _, pattern := range d.Patterns() {
		if pattern.Regexp.FindString(token) == token {
			return true
		}
	}
	return false
}

// The page is a pure function of the query string on a public route, so a
// crafted link must not be able to make it render three unrelated things side
// by side as though they agreed.
func TestRenderPageRejectsInconsistentTimestamps(t *testing.T) {
	d := &Decorator{}
	params, ok := d.Parse("2026-08-09T20:30:00+04:00", ref)
	if !ok {
		t.Fatal("Parse rejected a valid timestamp")
	}

	// A positive control: the untouched params really do render.
	rec := httptest.NewRecorder()
	d.RenderPage(rec, params)
	if rec.Code != http.StatusOK {
		t.Fatalf("baseline status = %d, want 200", rec.Code)
	}

	cases := map[string]func(url.Values){
		"an offset that disagrees with the timestamp": func(v url.Values) { v.Set("o", "0") },
		// One minute off. 1786293000000 would have been the right instant for
		// this very timestamp, which is not much of a disagreement.
		"an instant that disagrees":               func(v url.Values) { v.Set("t", "1786293060000") },
		"a canonical that is not normalised":      func(v url.Values) { v.Set("dtg", "2026-08-09T20:30+04:00") },
		"an assumed flag a timestamp cannot have": func(v url.Values) { v.Set("a", "my") },
		"a zone letter as well as an offset":      func(v url.Values) { v.Set("z", "Z") },
		"neither a zone letter nor an offset":     func(v url.Values) { v.Del("o") },
		"a nonsense offset":                       func(v url.Values) { v.Set("o", "banana") },
	}

	for name, corrupt := range cases {
		t.Run(name, func(t *testing.T) {
			crafted := url.Values{}
			for key, values := range params {
				crafted[key] = append([]string(nil), values...)
			}
			corrupt(crafted)

			rec := httptest.NewRecorder()
			d.RenderPage(rec, crafted)

			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400 for %v", rec.Code, crafted)
			}
		})
	}
}

// The reading is rendered in the offset the author wrote, not in UTC.
func TestPageDescribesATimestampInItsOwnOffset(t *testing.T) {
	d := &Decorator{}
	params, ok := d.Parse("2026-08-09T22:00:00+05:30", ref)
	if !ok {
		t.Fatal("Parse rejected a valid timestamp")
	}

	rec := httptest.NewRecorder()
	d.RenderPage(rec, params)

	if !strings.Contains(rec.Body.String(), "09 Aug 2026 22:00 +05:30") {
		t.Fatalf("page does not read the timestamp in its own offset:\n%s", rec.Body.String())
	}
}

// End to end through the tagger, which is the only path that exercises the
// pattern, Pattern.Value and Parse together.
//
// Asserting the pattern and Parse separately is not enough, and did not catch a
// capturing group in the timestamp pattern that made Pattern.Value hand ":00"
// to Parse: every token matched, every token parsed, and nothing decorated.
func TestTaggerDecoratesTimestamps(t *testing.T) {
	registry, err := decorators.NewDefaultRegistry(&Decorator{})
	if err != nil {
		t.Fatalf("failed to build the registry: %v", err)
	}
	tagger := &decorators.Tagger{Registry: registry, URLPrefix: "/p"}

	cases := []struct {
		message string
		want    string
	}{
		{"2026-08-09T16:30:00Z", "[2026-08-09T16:30:00Z](/p/dtg?"},
		{"2026-08-09T16:30Z", "[2026-08-09T16:30Z](/p/dtg?"},
		{"logged at 2026-08-09T22:00:00+05:30", "[2026-08-09T22:00:00+05:30](/p/dtg?"},

		// The link text is the token as the author wrote it, and the canonical
		// form travels in the query string instead.
		{"2026-08-09t16:30:00z", "[2026-08-09t16:30:00z](/p/dtg?"},
	}

	for _, tc := range cases {
		t.Run(tc.message, func(t *testing.T) {
			got := tagger.Decorate(tc.message, ref)
			if !strings.Contains(got, tc.want) {
				t.Fatalf("Decorate(%q) = %q, want a link containing %q", tc.message, got, tc.want)
			}
		})
	}
}

// The two grammars have to coexist in one message without either eating the
// other.
func TestTaggerDecoratesBothFormsTogether(t *testing.T) {
	registry, err := decorators.NewDefaultRegistry(&Decorator{})
	if err != nil {
		t.Fatalf("failed to build the registry: %v", err)
	}
	tagger := &decorators.Tagger{Registry: registry, URLPrefix: "/p"}

	got := tagger.Decorate("from 091630ZAUG26 until 2026-08-09T20:30:00+04:00", ref)

	if !strings.Contains(got, "[091630ZAUG26](/p/dtg?") {
		t.Fatalf("the date-time group was not decorated: %s", got)
	}
	if !strings.Contains(got, "[2026-08-09T20:30:00+04:00](/p/dtg?") {
		t.Fatalf("the timestamp was not decorated: %s", got)
	}
}

// A timestamp inside a span the tagger protects is left exactly as written,
// the same as any other token.
func TestTaggerLeavesProtectedTimestampsAlone(t *testing.T) {
	registry, err := decorators.NewDefaultRegistry(&Decorator{})
	if err != nil {
		t.Fatalf("failed to build the registry: %v", err)
	}
	tagger := &decorators.Tagger{Registry: registry, URLPrefix: "/p"}

	for _, message := range []string{
		"the format is `2026-08-09T16:30:00Z`",
		"see https://example.com/logs/2026-08-09T16:30:00Z for detail",
		"[2026-08-09T16:30:00Z](https://example.com/window)",
	} {
		t.Run(message, func(t *testing.T) {
			if got := tagger.Decorate(message, ref); got != message {
				t.Fatalf("Decorate(%q) rewrote it to %q", message, got)
			}
		})
	}
}

// Some military formats put "DTG:" in front of a time to mark where it starts.
// The moniker is matched so it can be consumed, and only the time is captured,
// so the link reads as the time alone.
func TestMonikerIsStrippedFromTheLink(t *testing.T) {
	registry, err := decorators.NewDefaultRegistry(&Decorator{})
	if err != nil {
		t.Fatalf("failed to build the registry: %v", err)
	}
	tagger := &decorators.Tagger{Registry: registry, URLPrefix: "/p"}

	cases := []struct {
		message string
		want    string
	}{
		{"DTG: 091630ZAUG26", "[091630ZAUG26](/p/dtg?"},
		{"DTG:091630ZAUG26", "[091630ZAUG26](/p/dtg?"},
		{"DTG :091630ZAUG26", "[091630ZAUG26](/p/dtg?"},

		// The moniker is case-insensitive; the token is not.
		{"dtg: 091630ZAUG26", "[091630ZAUG26](/p/dtg?"},
		{"Dtg: 091630ZAUG26", "[091630ZAUG26](/p/dtg?"},

		// Every shape the bare patterns accept, since both are built from the
		// same sub-expressions.
		{"DTG: 091630Z", "[091630Z](/p/dtg?"},
		// The link text is the token as written; the canonical form travels in
		// the query string, exactly as it does for a bare token.
		{"DTG: 091630ZAUG2026", "[091630ZAUG2026](/p/dtg?a=&dtg=091630ZAUG26&"},
		{"DTG: 2026-08-09T16:30:00Z", "[2026-08-09T16:30:00Z](/p/dtg?"},

		{"sent DTG: 091630ZAUG26 as ordered", "sent [091630ZAUG26](/p/dtg?"},
	}

	for _, tc := range cases {
		t.Run(tc.message, func(t *testing.T) {
			got := tagger.Decorate(tc.message, ref)

			if !strings.Contains(got, tc.want) {
				t.Fatalf("Decorate(%q) = %q, want a link containing %q", tc.message, got, tc.want)
			}

			// The whole point: the moniker is gone from the rendered text.
			if strings.Contains(strings.ToUpper(got), "DTG:") {
				t.Fatalf("Decorate(%q) = %q, want the moniker consumed", tc.message, got)
			}
		})
	}
}

// The moniker vouches for nothing. It marks where a time starts, so a token it
// marks still has to be one, and everything declined on its own stays declined.
func TestMonikerDoesNotWidenWhatIsAccepted(t *testing.T) {
	registry, err := decorators.NewDefaultRegistry(&Decorator{})
	if err != nil {
		t.Fatalf("failed to build the registry: %v", err)
	}
	tagger := &decorators.Tagger{Registry: registry, URLPrefix: "/p"}

	for _, message := range []string{
		// Short form in a zone other than Zulu: too loose to claim, labelled
		// or not.
		"DTG: 091630R",

		// No zone letter at all.
		"DTG: 091630",

		// Impossible dates stay impossible.
		"DTG: 311200ZFEB26",
		"DTG: 092400ZAUG26",

		// A moniker with nothing after it is just text.
		"DTG:",
		"DTG: soon",

		// Not the moniker: part of a longer word.
		"XDTG: soon",
	} {
		t.Run(message, func(t *testing.T) {
			if got := tagger.Decorate(message, ref); got != message {
				t.Fatalf("Decorate(%q) rewrote it to %q", message, got)
			}
		})
	}
}

// The moniker composes: it only ever labels a grammar that is switched on in
// its own right, so a moniker with nothing left to label is dropped entirely
// rather than left matching a token the admin has turned off.
func TestMonikerForFollowsTheEnabledGrammars(t *testing.T) {
	cases := []struct {
		name    string
		formats Formats
		want    *decorators.Pattern
	}{
		{"moniker off", Formats{Military: true, Timestamp: true}, nil},
		{"both grammars on", Formats{Moniker: true, Military: true, Timestamp: true}, &monikerAny},
		{"military only", Formats{Moniker: true, Military: true}, &monikerMilitary},
		{"timestamp only", Formats{Moniker: true, Timestamp: true}, &monikerTimestamp},
		{"nothing left to label", Formats{Moniker: true}, nil},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := monikerFor(tc.formats)

			if tc.want == nil {
				if ok {
					t.Fatalf("monikerFor(%+v) returned a pattern, want none", tc.formats)
				}
				if got.Regexp != nil {
					t.Fatalf("monikerFor(%+v) returned %v with ok=false, want the zero Pattern", tc.formats, got.Regexp)
				}
				return
			}

			if !ok {
				t.Fatalf("monikerFor(%+v) returned no pattern, want one", tc.formats)
			}
			if got.Regexp != tc.want.Regexp {
				t.Fatalf("monikerFor(%+v) = %v, want %v", tc.formats, got.Regexp, tc.want.Regexp)
			}
		})
	}
}

// The same rule seen from the outside: a decorator with only the moniker
// switched on contributes no patterns at all, so nothing matches and the post
// takes the same path as a message with no token in it.
func TestMonikerAloneContributesNoPatterns(t *testing.T) {
	d := &Decorator{Enabled: func() Formats { return Formats{Moniker: true} }}

	if got := d.Patterns(); len(got) != 0 {
		t.Fatalf("Patterns() returned %d patterns, want none", len(got))
	}

	registry, err := decorators.NewDefaultRegistry(d)
	if err != nil {
		t.Fatalf("failed to build the registry: %v", err)
	}
	tagger := &decorators.Tagger{Registry: registry, URLPrefix: "/p"}

	message := "DTG: 091630ZAUG26 and 091630ZAUG26"
	if got := tagger.Decorate(message, ref); got != message {
		t.Fatalf("Decorate(%q) rewrote it to %q", message, got)
	}
}

// A labelled token inside a protected span is left exactly as written, moniker
// and all.
func TestMonikerIsLeftAloneWhenProtected(t *testing.T) {
	registry, err := decorators.NewDefaultRegistry(&Decorator{})
	if err != nil {
		t.Fatalf("failed to build the registry: %v", err)
	}
	tagger := &decorators.Tagger{Registry: registry, URLPrefix: "/p"}

	for _, message := range []string{
		"the header reads `DTG: 091630ZAUG26`",
		"[DTG: 091630ZAUG26](https://example.com/window)",
	} {
		t.Run(message, func(t *testing.T) {
			if got := tagger.Decorate(message, ref); got != message {
				t.Fatalf("Decorate(%q) rewrote it to %q", message, got)
			}
		})
	}
}
