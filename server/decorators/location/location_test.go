package location

import (
	"html"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/decorators"
)

var ref = time.Date(2026, time.August, 11, 12, 0, 0, 0, time.UTC)

// taggerFor runs one decorator through the real tagger.
//
// tagger_test.go has the same helper and cannot be shared with: it is in the
// external test package, which is the right place for the corpus sweeps and the
// wrong one for a test that needs to build a Formats value field by field.
func taggerFor(t *testing.T, d *Decorator) *decorators.Tagger {
	t.Helper()

	registry, err := decorators.NewDefaultRegistry(d)
	if err != nil {
		t.Fatalf("NewDefaultRegistry() = %v", err)
	}
	return &decorators.Tagger{
		Registry:  registry,
		URLPrefix: "/plugins/com.mattermost.plugin-tactical-fusion/decorate",
	}
}

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

// The page runs no inline script at all.
//
// It used to carry one, pinned by digest, because the copy controls were
// hand-written here. The page renders the same React bundle the sidebar does
// now, so there is nothing inline left to pin and the property is stronger for
// it: 'unsafe-inline' is absent, so an escaping mistake in the author's own
// text cannot become a running script however it is spelled.
func TestRenderPageRunsNoInlineScript(t *testing.T) {
	d := &Decorator{}
	params := mustParams(t, d, "3510N07901W")

	rec := httptest.NewRecorder()
	d.RenderPage(rec, params)

	policy := rec.Header().Get("Content-Security-Policy")
	if strings.Contains(policy, "script-src") && strings.Contains(policy, "script-src 'unsafe-inline'") {
		t.Fatalf("Content-Security-Policy = %q, want no unsafe-inline: this page echoes author text", policy)
	}

	body := rec.Body.String()
	if n := strings.Count(body, "<script>"); n != 0 {
		t.Fatalf("the page carries %d inline scripts; everything it runs should come from the bundle", n)
	}
	if n := strings.Count(body, "<script "); n != 1 {
		t.Fatalf("the page carries %d script elements, want exactly the page bundle", n)
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

// MGRS and UTM are separate switches, and each governs only its own grammar.
//
// This is the pin CLAUDE.md names for the one format that ships OFF. UTM is the
// only grammar here that can decorate a real coordinate and point at the wrong
// place, because its band letter is ambiguous, so an install has to be able to
// have grid references without it. Deleted as collateral by the commit that
// added the map, which left the highest-consequence setting in the package
// unpinned in both directions while CLAUDE.md still advertised this test.
//
// Both tokens are driven under all four states, which is what makes the name
// true: the surviving test parsed an MGRS token only, so it said nothing about
// UTM being off, UTM working with MGRS off, or neither being on.
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

// The three area-reference systems are three switches, and each governs only
// itself.
//
// Split for the same reason MGRS and UTM are: they are separate notations an
// install may want separately, and a reader who never sees a GARS code written
// should not have to accept its false positives in order to decline it.
//
// Two of the three are label-only, so this also pins the composition with the
// moniker switch: without it there is no way to reach them at all, which is the
// same shape LATD has.
func TestAreaFormatsSwitchIndependently(t *testing.T) {
	const (
		georef   = "GEOREF:GJNJ5753"
		gars     = "GARS:006AG39"
		plusCode = "849VCWC8+R9"
	)

	for _, tc := range []struct {
		name                         string
		formats                      Formats
		georefOK, garsOK, plusCodeOK bool
	}{
		{"all three on", Formats{GEOREF: true, GARS: true, PlusCode: true, Moniker: true}, true, true, true},
		{"GEOREF alone", Formats{GEOREF: true, Moniker: true}, true, false, false},
		{"GARS alone", Formats{GARS: true, Moniker: true}, false, true, false},
		{"Plus Codes alone", Formats{PlusCode: true, Moniker: true}, false, false, true},
		{
			// The grammars are all three still enabled here, so Parse still
			// reads their tokens; what the moniker switch takes away is the
			// pattern that could ever hand them one, which is why the two
			// halves of this test disagree on this row and only on this row.
			"no moniker", Formats{GEOREF: true, GARS: true, PlusCode: true}, true, true, true,
		},
		{"none", Formats{}, false, false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			d := &Decorator{Enabled: func() Formats { return tc.formats }}

			// Parse is given the token rather than the label, since the label is
			// consumed by the pattern and never reaches it. What the moniker
			// switch governs is whether a pattern exists at all, which Patterns
			// answers below.
			if _, ok := d.Parse("GJNJ5753", ref); ok != tc.georefOK {
				t.Errorf("GEOREF parsed = %v, want %v", ok, tc.georefOK)
			}
			if _, ok := d.Parse("006AG39", ref); ok != tc.garsOK {
				t.Errorf("GARS parsed = %v, want %v", ok, tc.garsOK)
			}
			if _, ok := d.Parse(plusCode, ref); ok != tc.plusCodeOK {
				t.Errorf("Plus Code parsed = %v, want %v", ok, tc.plusCodeOK)
			}

			// And end to end through the tagger, which is what actually decides
			// whether a message is rewritten.
			tg := taggerFor(t, d)
			for _, row := range []struct {
				text string
				want bool
			}{
				{georef, tc.georefOK && tc.formats.Moniker},
				{gars, tc.garsOK && tc.formats.Moniker},
				{plusCode, tc.plusCodeOK},
			} {
				if got := tg.Decorate(row.text, ref) != row.text; got != row.want {
					t.Errorf("Decorate(%q) decorated = %v, want %v", row.text, got, row.want)
				}
			}
		})
	}
}

// Switching a format off must not cost its ROW, only its links.
//
// The same claim TestUTMSwitchedOffStillRendersTheUTMRow makes, extended to the
// three notations added with the area-reference systems. A derived row comes
// from the position rather than from the text, so an install that never wants a
// GARS code matched still gets a GARS reading for every coordinate it does
// match, and turning the switch off stays as cheap as it is meant to be.
//
// Read out of the conversion the shell carries rather than out of a rendered
// table: every surface renders from format.ts now, so the page is a shell and
// the derived readings travel in data-conversion.
func TestAreaFormatsSwitchedOffStillRenderTheirRows(t *testing.T) {
	d := &Decorator{Enabled: func() Formats { return Formats{MGRS: true} }}

	params, ok := d.Parse("18S UJ 23478 06483", ref)
	if !ok {
		t.Fatal("Parse declined an MGRS token while MGRS is on")
	}

	rec := httptest.NewRecorder()
	d.RenderPage(rec, params)

	if rec.Code != 200 {
		t.Fatalf("RenderPage() status = %d, want 200", rec.Code)
	}

	for _, row := range []struct{ field, value string }{
		{"georef", "GJNJ57885337"},
		{"gars", "206LT26"},
		{"pluscode", "87C4VXQ7+RV44"},
	} {
		want := html.EscapeString(`"` + row.field + `":"` + row.value + `"`)
		if !strings.Contains(rec.Body.String(), want) {
			t.Errorf("the page carries no %s reading", row.field)
		}
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
			// The row is derived from the position rather than matched in the
			// text, so switching the grammar off costs no row. It reaches the
			// page in the conversion the shell carries.
			if !strings.Contains(rec.Body.String(), html.EscapeString(`"utm":"18S 323478E 4306483N"`)) {
				t.Errorf("the page carries no UTM reading: %s", rec.Body.String())
			}
		})
	}
}

// A decorator that draws no inline map declares no post type.
//
// Answering "" here rather than gating in the hook is what keeps the stamp from
// being written at all, and the stamp is the expensive half: Elasticsearch and
// OpenSearch index a custom_* post and then never match it, and Post.Type
// survives every edit once set, with no MessageWillBeUpdated hook to clear one.
func TestPostTypeIsEmptyWhenTheInlineMapIsOff(t *testing.T) {
	for _, tc := range []struct {
		name string
		maps Maps
		want string
	}{
		{"every surface on", AllMaps, PostType},
		{"inline alone on", Maps{Inline: true}, PostType},
		{"inline off", Maps{Panel: true, Page: true}, ""},
		{"every surface off", Maps{}, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			d := &Decorator{Maps: func() Maps { return tc.maps }}
			if got := d.PostType(); got != tc.want {
				t.Fatalf("PostType() = %q, want %q", got, tc.want)
			}
		})
	}
}

// A nil selector means every surface, the way a nil Enabled means every format,
// so nothing that builds a bare Decorator has to know this switch exists.
func TestPostTypeDefaultsToOnWithNoSelector(t *testing.T) {
	if got := (&Decorator{}).PostType(); got != PostType {
		t.Fatalf("PostType() = %q, want %q", got, PostType)
	}
}
