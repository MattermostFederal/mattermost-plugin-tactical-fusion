package location

import (
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

var ref = time.Date(2026, time.August, 11, 12, 0, 0, 0, time.UTC)

func mustParams(t *testing.T, d *Decorator, token string) url.Values {
	t.Helper()
	params, ok := d.Parse(token, ref)
	if !ok {
		t.Fatalf("Parse(%q) rejected a valid token", token)
	}
	return params
}

func TestParseEmitsFormatAndCanonical(t *testing.T) {
	d := &Decorator{}
	params := mustParams(t, d, "34°03'22\"N 118°15'00\"W")

	if got := params.Get(paramFormat); got != string(FormatDMS) {
		t.Fatalf("f = %q, want %q", got, FormatDMS)
	}
	if got := params.Get(paramValue); got != "340322N1181500W" {
		t.Fatalf("v = %q, want the canonical form", got)
	}
	if got := params.Get(paramRaw); got != "34°03'22\"N 118°15'00\"W" {
		t.Fatalf("r = %q, want the author's own text", got)
	}
}

// The common case for the USMTF family: the author already typed the canonical
// form, so r would only repeat v and is left out of the URL entirely.
func TestParseOmitsRawWhenItWouldRepeatTheCanonicalForm(t *testing.T) {
	d := &Decorator{}
	params := mustParams(t, d, "3510N07901W")

	if params.Has(paramRaw) {
		t.Fatalf("r = %q, want it omitted when it equals v", params.Get(paramRaw))
	}
}

// A link this decorator wrote must render. A token accepted at decoration and
// rejected at render would be rewritten permanently into a message whose own
// page answers 400, and editing the post by hand would be the only way back.
func TestEveryAcceptedTokenRendersItsOwnPage(t *testing.T) {
	d := &Decorator{}

	for _, tc := range acceptedTokens {
		t.Run(tc.token, func(t *testing.T) {
			params := mustParams(t, d, tc.token)

			rec := httptest.NewRecorder()
			d.RenderPage(rec, params)

			if rec.Code != 200 {
				t.Fatalf("RenderPage() status = %d, want 200", rec.Code)
			}
		})
	}
}

// The page's one script is pinned by digest, never allowed by 'unsafe-inline'.
//
// This page echoes the author's own text from a message, on a public route
// whose query string anybody can write, so the property that has to survive is
// that an escaping mistake stays inert. A hash keeps it: injected markup does
// not match the digest and is blocked, where 'unsafe-inline' would run it.
func TestRenderPagePinsItsScriptByDigest(t *testing.T) {
	d := &Decorator{}
	params := mustParams(t, d, "3510N07901W")

	rec := httptest.NewRecorder()
	d.RenderPage(rec, params)

	policy := rec.Header().Get("Content-Security-Policy")
	if !strings.Contains(policy, "script-src 'sha256-") {
		t.Fatalf("Content-Security-Policy = %q, want the copy script pinned by digest", policy)
	}
	if strings.Contains(policy, "script-src 'unsafe-inline'") {
		t.Fatalf("Content-Security-Policy = %q, want no unsafe-inline: this page echoes author text", policy)
	}
	// And it is exactly one script, the one the policy names.
	if n := strings.Count(rec.Body.String(), "<script"); n != 1 {
		t.Fatalf("the page carries %d script elements, want exactly the copy script", n)
	}
}

// Crafted links. Each mutation must reach the error page rather than a panel
// that renders the pieces side by side as though they agreed.
func TestValidateParamsRejectsCraftedLinks(t *testing.T) {
	valid := url.Values{
		paramFormat: {string(FormatDMS)},
		paramValue:  {"340322N1181500W"},
	}

	cases := []struct {
		name   string
		mutate func(url.Values)
	}{
		{"unknown format", func(v url.Values) { v.Set(paramFormat, "usng") }},
		{"missing format", func(v url.Values) { v.Del(paramFormat) }},
		{"missing value", func(v url.Values) { v.Del(paramValue) }},
		{"format disagrees with token", func(v url.Values) { v.Set(paramFormat, string(FormatLATM)) }},
		{"token is not a coordinate", func(v url.Values) { v.Set(paramValue, "not a coordinate") }},
		{"token out of range", func(v url.Values) { v.Set(paramValue, "999999N1181500W") }},
		{"non-canonical alias", func(v url.Values) { v.Set(paramValue, "34 03 22 N 118 15 00 W") }},
		{"prose in the token", func(v url.Values) { v.Set(paramValue, "TARGET 340322N1181500W") }},
	}

	d := &Decorator{}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			params := url.Values{}
			for k, vs := range valid {
				params[k] = append([]string(nil), vs...)
			}
			tc.mutate(params)

			if _, ok := validateParams(params); ok {
				t.Fatal("validateParams accepted a crafted link")
			}

			rec := httptest.NewRecorder()
			d.RenderPage(rec, params)
			if rec.Code != 400 {
				t.Fatalf("RenderPage() status = %d, want 400", rec.Code)
			}
		})
	}
}

// The four gates on r, one case each. A failure must reject the whole link
// rather than just dropping the row: a link carrying an r this plugin would
// never have written is not one this plugin wrote.
func TestValidateParamsRejectsBadRaw(t *testing.T) {
	cases := []struct {
		name string
		raw  string
	}{
		{"gate 1: too long", strings.Repeat("3", maxRawBytes+1)},
		{"gate 2: markup", `34°03'22"N 118°15'00"W<script>`},
		{"gate 2: ampersand", `34°03'22"N&118°15'00"W`},
		{"gate 2: control character", "34\x0003'22\"N 118°15'00\"W"},
		{"gate 2: unexpected letters", `34°03'22"N 118°15'00"W ALFA`},
		{"gate 3: prose around a real token", `TARGET 34°03'22"N 118°15'00"W`},
		{"gate 4: a valid token naming somewhere else", `35°03'22"N 118°15'00"W`},
	}

	d := &Decorator{}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			params := url.Values{
				paramFormat: {string(FormatDMS)},
				paramValue:  {"340322N1181500W"},
				paramRaw:    {tc.raw},
			}

			if _, ok := validateParams(params); ok {
				t.Fatal("validateParams accepted an r it should have refused")
			}

			rec := httptest.NewRecorder()
			d.RenderPage(rec, params)
			if rec.Code != 400 {
				t.Fatalf("RenderPage() status = %d, want 400", rec.Code)
			}
		})
	}
}

// r may differ from the canonical form only in the ways normalization erases.
func TestValidateParamsAcceptsAnEquivalentRaw(t *testing.T) {
	params := url.Values{
		paramFormat: {string(FormatDMS)},
		paramValue:  {"340322N1181500W"},
		paramRaw:    {`34° 03' 22" N, 118° 15' 00" W`},
	}

	page, ok := validateParams(params)
	if !ok {
		t.Fatal("validateParams rejected an equivalent spelling")
	}
	if page.raw != `34° 03' 22" N, 118° 15' 00" W` {
		t.Fatalf("raw = %q, want the author's spelling preserved", page.raw)
	}
}

// An absent r reads as "the author wrote the canonical form", not as a missing
// value, so nothing downstream has to special-case it.
func TestAbsentRawFallsBackToTheCanonicalForm(t *testing.T) {
	params := url.Values{
		paramFormat: {string(FormatLATM)},
		paramValue:  {"3510N07901W"},
	}

	page, ok := validateParams(params)
	if !ok {
		t.Fatal("validateParams rejected a link with no r")
	}
	if page.raw != "3510N07901W" {
		t.Fatalf("raw = %q, want the canonical form", page.raw)
	}
}

// Every emitted r must survive its own gates, checked over the whole corpus
// rather than case by case.
func TestEveryEmittedRawSurvivesItsOwnGates(t *testing.T) {
	d := &Decorator{}

	for _, tc := range acceptedTokens {
		params := mustParams(t, d, tc.token)
		if _, ok := validateParams(params); !ok {
			t.Errorf("validateParams rejected the params Parse produced for %q", tc.token)
		}
	}
}

func TestPatternsFollowTheEnabledFormats(t *testing.T) {
	cases := []struct {
		name    string
		formats Formats
		want    int
	}{
		{"everything off", Formats{}, 0},
		{"signed dd only", Formats{DDSigned: true}, 1},
		{"signed dd with labels", Formats{DDSigned: true, Moniker: true}, 2},
		{"moniker alone has nothing to label", Formats{Moniker: true}, 0},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d := &Decorator{Enabled: func() Formats { return tc.formats }}
			if got := len(d.Patterns()); got != tc.want {
				t.Fatalf("len(Patterns()) = %d, want %d", got, tc.want)
			}
		})
	}
}

// A format that is switched off must not be matched, whatever else is on.
func TestDisabledFormatIsNotParsed(t *testing.T) {
	d := &Decorator{Enabled: func() Formats { return Formats{USMTF: true} }}

	if _, ok := d.Parse("34.0561, -118.2500", ref); ok {
		t.Fatal("Parse accepted a signed decimal token while that format is off")
	}
	if _, ok := d.Parse("3510N07901W", ref); !ok {
		t.Fatal("Parse rejected a USMTF token while that format is on")
	}
}

// LATD is available behind a label only. Seven characters, two of them letters,
// resolving to a 111 km square is not something to detect bare.
func TestLATDIsNotABarePattern(t *testing.T) {
	d := &Decorator{}

	var bare bool
	for _, p := range d.Patterns() {
		if p.ReplaceGroup == 0 && p.Regexp.MatchString("35N079W") {
			bare = true
		}
	}
	if bare {
		t.Fatal("a bare pattern matches LATD, want it available behind a label only")
	}
}

// The invariant that matters most: nothing this decorator writes into a message
// may be rejected by the page behind it.
//
// A token accepted at decoration and refused at render is rewritten permanently
// into somebody's post, and its own page answers 400 forever, with hand-editing
// the only way back. Two separate bugs reached exactly that: separators built
// from \s let a coordinate span a line break, and canonicalizing both halves to
// the coarser digit count could collapse a pair onto Null Island, which the
// parser then declined.
func TestNothingAcceptedAtDecorationIsRejectedAtRender(t *testing.T) {
	d := &Decorator{}

	messages := []string{
		"34 03 22 N\n118 15 00 W",
		"34.0561N,\n118.2500W",
		"34.0561N,\t118.2500W",
		"34.0561N,\r\n118.2500W",
		"34 03 22 N\n\n118 15 00 W",
		"0.0000,0.00001",
		"0.00001,0.0000",
		"34.1 N, 118.2500 W",
		"34.05619,-118.2501",
		"34" + strings.Repeat(" ", 46) + "03 22N 118 15 00W",
	}

	for _, msg := range messages {
		t.Run(msg, func(t *testing.T) {
			params, ok := d.Parse(msg, ref)
			if !ok {
				// Declining is always a safe outcome.
				return
			}

			rec := httptest.NewRecorder()
			d.RenderPage(rec, params)
			if rec.Code != 200 {
				t.Fatalf("Parse accepted %q but its own page answered %d (params %v)", msg, rec.Code, params)
			}
		})
	}
}

// The same invariant, asserted over the whole positive corpus rather than a
// list somebody has to remember to extend.
func TestEveryEmittedLinkRenders(t *testing.T) {
	d := &Decorator{}

	for _, tc := range acceptedTokens {
		params, ok := d.Parse(tc.token, ref)
		if !ok {
			t.Errorf("Parse rejected %q, which the parser accepts", tc.token)
			continue
		}

		rec := httptest.NewRecorder()
		d.RenderPage(rec, params)
		if rec.Code != 200 {
			t.Errorf("Parse accepted %q but its page answered %d", tc.token, rec.Code)
		}
	}
}

// A coordinate must never span a line break: the label of a markdown link
// cannot contain a blank line, so such a link does not render as one at all and
// the reader sees raw markup instead of their message.
func TestTokensNeverSpanALineBreak(t *testing.T) {
	d := &Decorator{}

	for _, msg := range []string{
		"34.0561N,\n118.2500W",
		"34.0561N,\n\n118.2500W",
		"34 03 22 N\n118 15 00 W",
	} {
		if _, ok := d.Parse(msg, ref); ok {
			t.Errorf("Parse accepted %q, which spans a line break", msg)
		}
	}
}

// The public page runs a regex over a caller-supplied value, so the value is
// bounded first. Linear-time matching is not a reason to do unbounded work
// somebody else chose.
func TestOversizedValueIsRejectedBeforeMatching(t *testing.T) {
	params := url.Values{
		paramFormat: {string(FormatDMS)},
		paramValue:  {strings.Repeat("3", 100000)},
	}

	if _, ok := validateParams(params); ok {
		t.Fatal("validateParams accepted an oversized value")
	}
}

// The copy affordance is per row and sits beside the value it copies.
//
// Value rows get one; prose rows do not, because a reader pasting "about 11 m"
// or "WGS 84" into another system has copied a sentence rather than a position.
func TestPageOffersACopyControlPerValueRow(t *testing.T) {
	d := &Decorator{}
	rec := httptest.NewRecorder()
	d.RenderPage(rec, mustParams(t, d, "3510N07901W"))
	body := rec.Body.String()

	for _, label := range []string{"MGRS", "Lat / lon", "DMS", "DDM", "UTM", "Original text"} {
		if !strings.Contains(body, `aria-label="Copy `+label+`"`) {
			t.Errorf("no copy control for the %q row", label)
		}
	}

	for _, label := range []string{"Resolution", "Datum"} {
		if strings.Contains(body, `aria-label="Copy `+label+`"`) {
			t.Errorf("the %q row is prose and must not offer a copy control", label)
		}
	}

	// The acknowledgement is announced, not only drawn.
	if !strings.Contains(body, `role="status"`) {
		t.Error("no live region for the copy acknowledgement")
	}
}

// The script reads the value out of the row rather than being handed one.
//
// That is what keeps a coordinate out of the script entirely: nothing is
// interpolated, so there is no second escaping context to get wrong, and the
// script stays a constant the policy can pin by digest.
func TestPageCopyScriptCarriesNoCoordinate(t *testing.T) {
	d := &Decorator{}
	rec := httptest.NewRecorder()
	d.RenderPage(rec, mustParams(t, d, "3510N07901W"))

	body := rec.Body.String()
	start := strings.Index(body, "<script>")
	end := strings.Index(body, "</script>")
	if start < 0 || end < start {
		t.Fatal("no script element on the page")
	}

	if script := body[start:end]; strings.Contains(script, "3510N07901W") || strings.Contains(script, "35.17") {
		t.Fatalf("the copy script carries a coordinate:\n%s", script)
	}
}

// Without JavaScript, or on an origin with no clipboard, the buttons never
// appear: the stylesheet hides them and only the script reveals them. A control
// that cannot work should not be on screen, and every value stays selectable.
func TestPageCopyControlsAreHiddenUntilTheScriptEnablesThem(t *testing.T) {
	if !strings.Contains(pageStyles, ".loc-copy button { display: none; }") {
		t.Error("copy buttons are not hidden by default")
	}
	if !strings.Contains(pageStyles, ".loc-can-copy .loc-copy button {") {
		t.Error("nothing reveals the copy buttons")
	}
	if !strings.Contains(copyScript, "navigator.clipboard") ||
		!strings.Contains(copyScript, "loc-can-copy") {
		t.Error("the script does not gate the buttons on clipboard support")
	}
}

// The page says each reading once, and matches the panel in doing so.
//
// There used to be a lead line above the table repeating the grid reference,
// three lines above the labeled row that already carried it with a copy button
// beside it. The page and the panel are two implementations of the same layout
// in two languages, so a change to one that misses the other is the standing
// failure mode here; this is the page half, and
// LocationPanel.pw.tsx's "shows each reading once" is the other.
func TestRenderPageHasNoLeadLineAboveTheTable(t *testing.T) {
	d := &Decorator{}
	params := mustParams(t, d, "18S UJ 23478 06483")

	rec := httptest.NewRecorder()
	d.RenderPage(rec, params)

	body := rec.Body.String()

	// The reference appears in its own row and in the row carrying the author's
	// spelling, which here is the same string. A third would be the lead back.
	if got := strings.Count(body, "18S UJ 23478 06483"); got != 2 {
		t.Errorf("the grid reference appears %d times, want 2 (its row, and the author's text)", got)
	}

	// The class the lead was written with belongs to the DTG page now.
	if strings.Contains(body, `class="described"`) {
		t.Error("the location page still writes a lead line above its table")
	}
}

// MGRS and UTM are separate switches, and each one governs only itself.
//
// They used to share one, and splitting them is the whole point of UTM shipping
// disabled: an install that wants grid references without the ambiguous band
// letter has to be able to have exactly that. A split that leaked in either
// direction would give it neither.
func TestMGRSAndUTMSwitchIndependently(t *testing.T) {
	cases := []struct {
		name          string
		formats       Formats
		mgrsOK, utmOK bool
	}{
		{"both on", Formats{MGRS: true, UTM: true}, true, true},
		{"the shipped default: MGRS without UTM", Formats{MGRS: true}, true, false},
		{"UTM without MGRS", Formats{UTM: true}, false, true},
		{"neither", Formats{}, false, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d := &Decorator{Enabled: func() Formats { return tc.formats }}

			if _, ok := d.Parse("18S UJ 23478 06483", ref); ok != tc.mgrsOK {
				t.Errorf("MGRS parsed = %v, want %v", ok, tc.mgrsOK)
			}
			if _, ok := d.Parse("33U 291000 5628000", ref); ok != tc.utmOK {
				t.Errorf("UTM parsed = %v, want %v", ok, tc.utmOK)
			}
		})
	}
}

// Switching UTM off must not cost a UTM ROW, only a UTM link.
//
// The switch governs which tokens in a message become links. Every decorated
// coordinate still carries a UTM reading in its panel and on its page, because
// that is derived from the position rather than matched in the text, and losing
// it would make this a much more expensive default than it is meant to be.
//
// Asserted over BOTH switch states rather than one. RenderPage never reads
// d.Enabled, so a single-state version of this test proved nothing about the
// switch: flipping its setup to AllFormats left it passing. Running both states
// is what makes the claim in the name true.
func TestUTMSwitchedOffStillRendersTheUTMRow(t *testing.T) {
	for _, tc := range []struct {
		name    string
		formats Formats
	}{
		{"UTM off, which is the shipped default", Formats{MGRS: true}},
		{"UTM on", Formats{MGRS: true, UTM: true}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			d := &Decorator{Enabled: func() Formats { return tc.formats }}

			params, ok := d.Parse("18S UJ 23478 06483", ref)
			if !ok {
				t.Fatal("Parse declined an MGRS token while MGRS is on")
			}

			rec := httptest.NewRecorder()
			d.RenderPage(rec, params)

			if rec.Code != 200 {
				t.Fatalf("RenderPage() status = %d, want 200", rec.Code)
			}
			if !strings.Contains(rec.Body.String(), "<th>UTM</th>") {
				t.Error("the page has no UTM row")
			}
			if !strings.Contains(rec.Body.String(), "18S 323478E 4306483N") {
				t.Error("the UTM row has no value in it")
			}
		})
	}
}
