package airport

import (
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/decorators"
)

func newTagger(t *testing.T) *decorators.Tagger {
	t.Helper()

	registry, err := decorators.NewDefaultRegistry(&Decorator{})
	if err != nil {
		t.Fatalf("registry: %v", err)
	}
	return &decorators.Tagger{Registry: registry, URLPrefix: "/plugins/tf/decorate"}
}

func decorate(t *testing.T, message string) string {
	t.Helper()
	return newTagger(t).Decorate(message, time.Now().UTC())
}

func decorated(t *testing.T, message string) bool {
	t.Helper()
	return decorate(t, message) != message
}

func TestLabeledIdentsDecorate(t *testing.T) {
	for _, message := range []string{
		"ICAO:KIND",
		"LOC:KIND",
		"DEPLOC:KLAX",
		"ARRLOC:PHIK",
		"ICAO :KIND",
		"ICAO\t:KIND",
		"launch from DEPLOC:KLAX at first light",
	} {
		t.Run(message, func(t *testing.T) {
			out := decorate(t, message)
			if out == message {
				t.Fatalf("was not decorated")
			}
			if !strings.Contains(out, "/decorate/airport?v=KIND") &&
				!strings.Contains(out, "/decorate/airport?v=KLAX") &&
				!strings.Contains(out, "/decorate/airport?v=PHIK") {
				t.Fatalf("unexpected link in %q", out)
			}
			if strings.Contains(out, ":") && !strings.Contains(out, "?v=") {
				t.Fatalf("a label survived in %q", out)
			}
		})
	}
}

// The label is CONSUMED, the way the DTG moniker's is: once the token is a link
// that says what it is, the label in front of it repeats that.
//
// The cost is accepted rather than overlooked, and this is where it is visible:
// a USMTF set line quoted verbatim loses its field label from the stored
// message, permanently.
func TestTheLabelIsConsumed(t *testing.T) {
	for _, label := range []string{"ICAO", "LOC", "DEPLOC", "ARRLOC"} {
		out := decorate(t, label+":KIND")
		if !strings.HasPrefix(out, "[KIND](") {
			t.Errorf("%s: the label survived: %q", label, out)
		}
		if strings.Contains(out, label+":") {
			t.Errorf("%s: the label is still in the message: %q", label, out)
		}
	}
}

// The link's text is the author's own token. The airfield name is data that can
// change between builds, and this rewrites the stored message permanently.
func TestTheLinkIsLabelledWithTheToken(t *testing.T) {
	out := decorate(t, "ICAO:KIND")
	if strings.Contains(out, "Indianapolis") {
		t.Fatalf("the airfield name reached the stored message: %q", out)
	}
}

// A USMTF set line ends "//". The terminator is matched so the guard can look
// past it, but it is outside the replaced group, so it stays in the message
// even though the label in front does not.
func TestSetLineTerminatorSurvivesTheLabelBeingConsumed(t *testing.T) {
	out := decorate(t, "DEPLOC:RJTT//")
	if out == "DEPLOC:RJTT//" {
		t.Fatal("a real USMTF set line was declined")
	}
	if !strings.HasPrefix(out, "[RJTT](") {
		t.Fatalf("the label was not consumed: %q", out)
	}
	if !strings.HasSuffix(out, ")//") {
		t.Fatalf("the terminator was swallowed into the link: %q", out)
	}
}

// Allowing a trailing "/" outright would have reopened this. The optional
// terminator cannot match a single slash, so the guard still sees "/".
func TestPathShapedTextIsRefused(t *testing.T) {
	for _, message := range []string{
		"logs/ICAO:KIND",
		"ICAO:KIND/foo",
		"/ICAO:KIND/",
		"see_ICAO:KIND",
		"x-ICAO:KIND",
	} {
		t.Run(message, func(t *testing.T) {
			if decorated(t, message) {
				t.Fatalf("was rewritten: %q", decorate(t, message))
			}
		})
	}
}

// RE2's \s includes \n. With `*` on both sides a label ending one line would
// claim whatever started the next, and 343 of these idents are dictionary
// words, so this would rewrite ordinary prose permanently.
func TestALabelCannotReachTheNextLine(t *testing.T) {
	for _, message := range []string{
		"Divert to LOC:\nFAST movers inbound",
		"ICAO:\r\nKIND",
		"DEPLOC:\n\nKLAX",
	} {
		t.Run(message, func(t *testing.T) {
			if decorated(t, message) {
				t.Fatalf("reached across the line break: %q", decorate(t, message))
			}
		})
	}
}

func TestBareIdentsAreNeverDecorated(t *testing.T) {
	for _, message := range []string{
		"KIND", "USCG", "FACT", "FAST", "LIMA", "UNIT",
		"the USCG cutter is inbound",
		"that was a KIND thing to do",
	} {
		t.Run(message, func(t *testing.T) {
			if decorated(t, message) {
				t.Fatalf("a bare ident was decorated: %q", decorate(t, message))
			}
		})
	}
}

// A lower-case label is where the collision lives: "loc: fast turnaround" would
// otherwise link FAST, which is Somerset East Airport.
func TestLowerCaseLabelsAreDeclined(t *testing.T) {
	for _, message := range []string{
		"loc: fast turnaround on the ramp",
		"icao:kind",
		"Icao:KIND",
		"deploc:KLAX",
		"ICAO:kind",
	} {
		t.Run(message, func(t *testing.T) {
			if decorated(t, message) {
				t.Fatalf("was decorated: %q", decorate(t, message))
			}
		})
	}
}

// Four letters vouches for nothing, so validation is by lookup.
func TestUnknownIdentsDecline(t *testing.T) {
	for _, message := range []string{"ICAO:QZQZ", "LOC:HOME"} {
		t.Run(message, func(t *testing.T) {
			if decorated(t, message) {
				t.Fatalf("was decorated: %q", decorate(t, message))
			}
		})
	}
}

// The other half of validation-by-lookup, and the uncomfortable half: 343 of
// these idents are ordinary English words, so a word written directly against
// an upper-case label IS a code and does decorate. What keeps that from
// reaching prose is the separator, which admits nothing after the colon.
func TestAWordShapedIdentBehindALabelDecorates(t *testing.T) {
	if _, ok := Lookup("FAST"); !ok {
		t.Fatal("FAST is no longer in the database; pick another word-shaped ident rather than deleting this")
	}

	if !decorated(t, "ICAO:FAST") {
		t.Error("ICAO:FAST did not decorate, so validation is no longer by lookup")
	}
	if decorated(t, "LOC: FAST") {
		t.Errorf("a space after the label reached ordinary prose: %q", decorate(t, "LOC: FAST"))
	}
}

func TestWrongLengthIsDeclined(t *testing.T) {
	for _, message := range []string{"ICAO:KIN", "ICAO:KINDX", "ICAO:K1ND"} {
		t.Run(message, func(t *testing.T) {
			out := decorate(t, message)
			if strings.Contains(out, "/decorate/airport") {
				t.Fatalf("was decorated: %q", out)
			}
		})
	}
}

// Guards are zero width, so one token cannot eat the separator the next needs.
func TestTwoAirfieldsOnOneLineBothDecorate(t *testing.T) {
	out := decorate(t, "DEPLOC:KLAX ARRLOC:PHIK")
	if strings.Count(out, "/decorate/airport?v=") != 2 {
		t.Fatalf("expected two links, got %q", out)
	}
}

func TestProtectedSpansAreLeftAlone(t *testing.T) {
	for _, message := range []string{
		"`ICAO:KIND`",
		"```\nICAO:KIND\n```",
		"[ICAO:KIND](https://example.com)",
		"https://example.com/ICAO:KIND",
	} {
		t.Run(message, func(t *testing.T) {
			if decorated(t, message) {
				t.Fatalf("a protected span was rewritten: %q", decorate(t, message))
			}
		})
	}
}

// A link this decorator already wrote is itself a protected span.
func TestDecorationIsIdempotent(t *testing.T) {
	once := decorate(t, "DEPLOC:KIND")
	twice := decorate(t, once)
	if once != twice {
		t.Fatalf("second pass changed the message:\n%q\n%q", once, twice)
	}
}

func TestPatternsAreEmptyWhenSwitchedOff(t *testing.T) {
	d := &Decorator{Enabled: func() Formats { return Formats{} }}
	if len(d.Patterns()) != 0 {
		t.Fatal("a switched-off decorator still contributes patterns")
	}
}

func TestParseReturnsOnlyTheIdent(t *testing.T) {
	d := &Decorator{}
	params, ok := d.Parse("KIND", time.Now())
	if !ok {
		t.Fatal("KIND was declined")
	}
	if len(params) != 1 || params.Get(ParamValue) != "KIND" {
		t.Fatalf("params = %v", params)
	}
}

func TestParseDeclinesAnythingNotFourUpperCaseLetters(t *testing.T) {
	d := &Decorator{}
	for _, value := range []string{"", "KIN", "KINDD", "kind", "K1ND", "KIND ", "K-ND"} {
		if params, ok := d.Parse(value, time.Now()); ok {
			t.Errorf("Parse(%q) = %v, want a refusal", value, params)
		}
	}
}

func TestRenderPageRefusesAMalformedIdent(t *testing.T) {
	for _, v := range []string{"", "KIN", "kind", "KIND1", "../etc"} {
		t.Run(v, func(t *testing.T) {
			rec := httptest.NewRecorder()
			(&Decorator{}).RenderPage(rec, url.Values{ParamValue: {v}})
			if rec.Code != 400 {
				t.Fatalf("status = %d, want 400", rec.Code)
			}
		})
	}
}

// A refreshed database that drops an ident must not turn every message naming
// it into a permanent 400.
func TestRenderPageAnswersForAnIdentThisBuildDoesNotHold(t *testing.T) {
	rec := httptest.NewRecorder()
	(&Decorator{}).RenderPage(rec, url.Values{ParamValue: {"QZQZ"}})

	if rec.Code != 200 {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "QZQZ") {
		t.Error("the page does not name the code it was asked about")
	}
	if !strings.Contains(body, "not in this build") {
		t.Error("the page does not say why there is nothing to show")
	}
}

// The readings are deliberately absent: the page links to the coordinate page
// rather than reprinting what it already renders.
func TestRenderPageShowsNoReadingsAndLinksToTheCoordinate(t *testing.T) {
	rec := httptest.NewRecorder()
	(&Decorator{}).RenderPage(rec, url.Values{ParamValue: {"KIND"}})

	body := rec.Body.String()
	for _, reading := range []string{"16S EJ 6047 9661", "394302.3N0861739.8W", "Natural Earth"} {
		if strings.Contains(body, reading) {
			t.Errorf("the page reprints the %q reading", reading)
		}
	}

	// The PLACE is the link, rather than a separate line under the table: the
	// place is where the airfield is, so it is the thing to follow to see where
	// that is. Relative and a sibling of this page, so it resolves on a subpath
	// install without the renderer knowing the subpath.
	want := `<a href="location?f=dd&amp;v=39.7173%2C-86.2944">Indianapolis, IN, US</a>`
	if !strings.Contains(body, want) {
		t.Errorf("the place does not link to the coordinate page; wanted %s", want)
	}
}

// An airfield with no usable position gets no link, because a link to a view
// that would refuse it is worse than none.
func TestRenderPageOffersNoCoordinateLinkWithoutAPosition(t *testing.T) {
	body := renderBody("KIND", Details{Ident: "KIND", Name: "Somewhere", Place: "Nowhere, XX"}, true)

	if strings.Contains(body, "href=\"location?") {
		t.Error("a link was offered for an airfield with no position")
	}
	if !strings.Contains(body, "Somewhere") {
		t.Error("the airfield is not rendered at all")
	}
	if !strings.Contains(body, "no coordinate readings") {
		t.Error("the page does not say why there is no position")
	}
}

func TestRenderPageShowsTheAirfield(t *testing.T) {
	rec := httptest.NewRecorder()
	(&Decorator{}).RenderPage(rec, url.Values{ParamValue: {"KIND"}})

	body := rec.Body.String()
	for _, want := range []string{
		"Indianapolis International Airport",
		"KIND",
		"Indianapolis, IN, US",
		"797 ft",
		"IND",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("the page does not carry %q", want)
		}
	}
}

// PageStatic with no script at all, which is the narrowest policy this
// framework serves and is what this page should want.
func TestThePageCarriesNoScript(t *testing.T) {
	rec := httptest.NewRecorder()
	(&Decorator{}).RenderPage(rec, url.Values{ParamValue: {"KIND"}})

	if got := rec.Header().Get("Content-Security-Policy"); !strings.Contains(got, "script-src 'none'") {
		t.Fatalf("policy = %q, want script-src 'none'", got)
	}
	if strings.Contains(rec.Body.String(), "<script") {
		t.Fatal("the page carries a script")
	}
}

// A tab-indented line is an indented code block, and the tagger protects its
// interior. Worth pinning here because the airfield grammar is the one whose
// tokens are short enough to sit alone on such a line.
func TestATabIndentedCodeLineIsNotDecorated(t *testing.T) {
	for _, message := range []string{"\tICAO:KIND", "    ICAO:KIND"} {
		if decorated(t, message) {
			t.Errorf("an indented code line was decorated: %q", decorate(t, message))
		}
	}
}
