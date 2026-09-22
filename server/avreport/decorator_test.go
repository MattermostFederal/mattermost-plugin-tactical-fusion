package avreport

import (
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/decorators"
	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/decorators/airport"
	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/decorators/dtg"
	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/decorators/location"
)

func newTagger(t *testing.T) *decorators.Tagger {
	t.Helper()

	registry, err := decorators.NewDefaultRegistry(
		&dtg.Decorator{},
		&location.Decorator{},
		&airport.Decorator{},
		&Decorator{},
	)
	if err != nil {
		t.Fatalf("registry: %v", err)
	}
	return &decorators.Tagger{Registry: registry, URLPrefix: "/plugins/tf/decorate"}
}

func decorate(t *testing.T, message string) string {
	t.Helper()
	return newTagger(t).Decorate(message, ref)
}

const metarLine = "METAR KJFK 221651Z 28012KT 10SM FEW250 24/12 A3012"

func TestEachKindDecoratesOnOneLine(t *testing.T) {
	for _, message := range []string{
		metarLine,
		"SPECI KJFK 221651Z 28012KT 10SM FEW250 24/12 A3012",
		"KJFK 221651Z 28012KT 10SM FEW250 24/12 A3012",
		"TAF PGUA 221720Z 2218/2324 07012KT P6SM SCT025",
		"!HNL 09/123 HNL RWY 08L/26R CLSD 2609221200-2609232359",
		"wx: " + metarLine,
		"current " + metarLine + "\nnext line",
	} {
		t.Run(message, func(t *testing.T) {
			out := decorate(t, message)
			if !strings.Contains(out, "/decorate/avreport?") {
				t.Fatalf("was not decorated: %q", out)
			}
			if strings.Contains(out, "/decorate/dtg?") {
				t.Fatalf("the time group inside the report was linked separately: %q", out)
			}
			if strings.Count(out, "/decorate/") != 1 {
				t.Fatalf("more than one link: %q", out)
			}
		})
	}
}

func TestTheTerminatorStaysOutsideTheLink(t *testing.T) {
	out := decorate(t, metarLine+"=")
	if !strings.HasSuffix(out, ")=") {
		t.Fatalf("the terminator was swallowed or dropped: %q", out)
	}
	if strings.Contains(out, "A3012=](") {
		t.Fatalf("the terminator reached the label: %q", out)
	}
}

func TestTheLinkCarriesTheReportAndItsInstant(t *testing.T) {
	out := decorate(t, metarLine)
	if !strings.HasPrefix(out, "["+metarLine+"](") {
		t.Fatalf("the label is not the report: %q", out)
	}

	params, ok := (&Decorator{}).Parse(metarLine, ref)
	if !ok {
		t.Fatal("Parse refused the report")
	}
	if params.Get(ParamValue) != metarLine {
		t.Errorf("v = %q", params.Get(ParamValue))
	}
	want := time.Date(2026, time.September, 22, 16, 51, 0, 0, time.UTC).UnixMilli()
	if params.Get(ParamInstant) != strconv.FormatInt(want, 10) {
		t.Errorf("t = %q, want %d", params.Get(ParamInstant), want)
	}
}

func TestWhatDoesNotDecorate(t *testing.T) {
	for _, message := range []string{
		"metar kjfk 221651z 28012kt 10sm",
		"KJFK 221651Z ready for departure",
		"the METAR is late",
		"see wx:METAR KJFK 221651Z 28012KT",
		"a/METAR KJFK 221651Z 28012KT",
		"TAF KJFK 221720Z\nFM230000 09008KT P6SM FEW030",
		"!JFK 09/123",
		"```\n" + metarLine + "\n```",
		"`" + metarLine + "`",
	} {
		t.Run(message, func(t *testing.T) {
			if strings.Contains(decorate(t, message), "/decorate/avreport?") {
				t.Fatalf("was decorated: %q", decorate(t, message))
			}
		})
	}
}

func TestAGarbledReportLeavesTheTimeGroupToDTG(t *testing.T) {
	out := decorate(t, "KJFK 221651Z ready for departure")
	if !strings.Contains(out, "/decorate/dtg?") {
		t.Fatalf("the time group was not linked on its own: %q", out)
	}
}

func TestAMultiLineTAFInsideProseIsNeitherLinkedNorStamped(t *testing.T) {
	message := "tonight:\nTAF KJFK 221720Z 2218/2324 28012KT P6SM\n  FM230000 09008KT P6SM FEW030\nthanks"
	out := decorate(t, message)
	if strings.Contains(out, "/decorate/avreport?") {
		t.Fatalf("a multi-line TAF was linked by its first line: %q", out)
	}
}

func TestAReportInsideAUSMTFMessageIsProtected(t *testing.T) {
	message := "WX/" + metarLine + "//\nAMPN/NONE//"
	if strings.Contains(decorate(t, message), "/decorate/avreport?") {
		t.Fatal("a report inside a USMTF set line was rewritten")
	}
}

func TestPatternsFollowTheirSwitches(t *testing.T) {
	off := &Decorator{Enabled: func() Formats { return Formats{} }}
	if len(off.Patterns()) != 0 {
		t.Fatal("a switched-off decorator still contributes patterns")
	}

	metarOnly := &Decorator{Enabled: func() Formats { return Formats{METAR: true} }}
	if len(metarOnly.Patterns()) != 2 {
		t.Fatalf("%d patterns with only METAR on, want 2", len(metarOnly.Patterns()))
	}
	if _, ok := metarOnly.Parse("TAF PGUA 221720Z 2218/2324 07012KT P6SM SCT025", ref); ok {
		t.Fatal("Parse accepted a TAF with TAF off")
	}
	if _, ok := metarOnly.Parse(metarLine, ref); !ok {
		t.Fatal("Parse refused a METAR with METAR on")
	}
}

func TestParseRefusesWhatTheStampReads(t *testing.T) {
	if _, ok := (&Decorator{}).Parse("TAF KJFK 221720Z 2218/2324 28012KT P6SM\nFM230000 09008KT", ref); ok {
		t.Fatal("a multi-line report was accepted as a link")
	}
	if _, ok := (&Decorator{}).Parse(strings.Repeat("x", MaxSourceRunes+1), ref); ok {
		t.Fatal("an over-long value was accepted")
	}
}

func TestValidateRequiresTheInstantToRoundTrip(t *testing.T) {
	params, _ := (&Decorator{}).Parse(metarLine, ref)
	if _, ok := Validate(params); !ok {
		t.Fatal("the decorator's own params were refused")
	}

	for name, mutate := range map[string]func(url.Values){
		"another instant": func(p url.Values) { p.Set(ParamInstant, "1700000000000") },
		"no instant":      func(p url.Values) { p.Del(ParamInstant) },
		"a bad instant":   func(p url.Values) { p.Set(ParamInstant, "soon") },
		"a huge instant":  func(p url.Values) { p.Set(ParamInstant, "99999999999999999") },
		"no report":       func(p url.Values) { p.Del(ParamValue) },
		"a garbled report": func(p url.Values) {
			p.Set(ParamValue, "hello there")
		},
		"a padded report": func(p url.Values) { p.Set(ParamValue, " "+metarLine) },
		"a second report": func(p url.Values) { p.Set(ParamValue, metarLine+"\n"+metarLine) },
	} {
		t.Run(name, func(t *testing.T) {
			clone := url.Values{}
			for key, values := range params {
				clone[key] = append([]string(nil), values...)
			}
			mutate(clone)
			if _, ok := Validate(clone); ok {
				t.Fatal("a hand-edited link was accepted")
			}
		})
	}
}

func TestValidateAcceptsTheMultiLineReportADetailsLinkCarries(t *testing.T) {
	for _, text := range []string{
		"TAF KJFK 221720Z 2218/2324 28012KT P6SM\nFM230000 09008KT P6SM FEW030",
		"A1234/26 NOTAMN\nQ) KZNY/QMRLC/IV/NBO/A/000/999/4038N07347W005\nA) KJFK B) 2609221200 C) 2609232359\nE) RWY 04L/22R CLSD",
	} {
		report, err := Decode(text, ref)
		if err != nil {
			t.Fatalf("Decode(%q): %v", text, err)
		}
		params := url.Values{ParamValue: {report.Raw}, ParamInstant: {strconv.FormatInt(report.Instant(), 10)}}
		if _, ok := Validate(params); !ok {
			t.Errorf("a multi-line report was refused: %q", text)
		}
		if _, ok := (&Decorator{}).Parse(text, ref); ok {
			t.Errorf("a multi-line report was accepted as a link token: %q", text)
		}
	}
}

func TestANotamWithNoInstantValidatesAtZero(t *testing.T) {
	line := "!JFK 09/001 JFK TWY A CLSD"
	params, ok := (&Decorator{}).Parse(line, ref)
	if !ok {
		t.Fatal("Parse refused a NOTAM with no effective time")
	}
	if params.Get(ParamInstant) != "0" {
		t.Errorf("t = %q, want 0", params.Get(ParamInstant))
	}
	if _, ok := Validate(params); !ok {
		t.Fatal("the zero instant did not round-trip")
	}
}

func TestRenderPageShowsTheDecodeAndEscapes(t *testing.T) {
	params, _ := (&Decorator{}).Parse(metarLine, ref)
	rec := httptest.NewRecorder()
	(&Decorator{}).RenderPage(rec, params)

	if rec.Code != 200 {
		t.Fatalf("status = %d", rec.Code)
	}
	body := rec.Body.String()
	for _, want := range []string{
		"METAR KJFK",
		"John F. Kennedy International Airport",
		`href="airport?v=KJFK"`,
		"280° at 12 kt",
		"few clouds at 25,000 ft",
		"30.12 inHg",
		"<pre>" + metarLine + "</pre>",
		`href="location?f=dd&amp;v=`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("the page does not carry %q", want)
		}
	}
	if got := rec.Header().Get("Content-Security-Policy"); !strings.Contains(got, "script-src 'none'") {
		t.Errorf("policy = %q, want script-src 'none'", got)
	}

	hostile := "METAR KJFK 221651Z 28012KT 10SM FEW250 24/12 A3012 <script>alert(1)</script>"
	hostileParams, ok := (&Decorator{}).Parse(hostile, ref)
	if !ok {
		t.Fatal("the hostile report was refused, so escaping was not exercised")
	}
	rec = httptest.NewRecorder()
	(&Decorator{}).RenderPage(rec, hostileParams)
	if strings.Contains(rec.Body.String(), "<script>") {
		t.Error("author text reached the page as markup")
	}
}

func TestRenderPageRefusesAHandEditedLink(t *testing.T) {
	rec := httptest.NewRecorder()
	(&Decorator{}).RenderPage(rec, url.Values{ParamValue: {metarLine}, ParamInstant: {"1"}})
	if rec.Code != 400 || !strings.Contains(rec.Body.String(), "TF-17004") {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
}

func TestDecorationIsIdempotent(t *testing.T) {
	once := decorate(t, metarLine)
	if twice := decorate(t, once); twice != once {
		t.Fatalf("the second pass changed the message:\n%q\n%q", once, twice)
	}
}

func TestLooksLikeHeader(t *testing.T) {
	for line, want := range map[string]bool{
		metarLine:                            true,
		"TAF KJFK 221720Z 2218/2324 28012KT": true,
		"!JFK 09/123 JFK RWY 04L CLSD":       true,
		"A1234/26 NOTAMN":                    true,
		"KJFK 221651Z 28012KT":               true,
		"KJFK 221651Z ready":                 false,
		"hello":                              false,
		"":                                   false,
	} {
		if got := LooksLikeHeader(line); got != want {
			t.Errorf("LooksLikeHeader(%q) = %v, want %v", line, got, want)
		}
	}
}

func TestAReportInATableRowKeepsTheCellDelimiterOutOfTheLabel(t *testing.T) {
	out := decorate(t, "| "+metarLine+" |")
	if strings.Contains(out, "A3012 |](") {
		t.Fatalf("a raw pipe reached the link label: %q", out)
	}
	if !strings.Contains(out, `A3012 \|](`) {
		t.Fatalf("the pipe was not escaped in the label: %q", out)
	}
}
