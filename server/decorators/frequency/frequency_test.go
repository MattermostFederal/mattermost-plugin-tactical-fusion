package frequency

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

func TestLabeledFrequenciesDecorate(t *testing.T) {
	for message, want := range map[string]string{
		"FREQ:121.5":             "121.5",
		"FREQ:243.0":             "243.0",
		"FREQ:118.300 MHZ":       "118.300+MHZ",
		"FREQ:8992 KHZ":          "8992+KHZ",
		"FREQ:8992":              "8992",
		"FREQ :121.5":            "121.5",
		"guard on FREQ:121.5//":  "121.5",
		"FREQ:121.5\nFREQ:243.0": "121.5",
	} {
		t.Run(message, func(t *testing.T) {
			out := decorate(t, message)
			if !strings.Contains(out, "/decorate/frequency?v="+want) {
				t.Fatalf("was not decorated as expected: %q", out)
			}
			if strings.Contains(out, "FREQ") {
				t.Fatalf("the label survived: %q", out)
			}
		})
	}
}

func TestTheSetTerminatorStaysOutsideTheLink(t *testing.T) {
	out := decorate(t, "FREQ:121.5//")
	if !strings.HasSuffix(out, ")//") {
		t.Fatalf("the terminator was swallowed or dropped: %q", out)
	}
}

func TestTheLinkIsLabelledWithTheToken(t *testing.T) {
	out := decorate(t, "FREQ:118.300 MHZ")
	if !strings.HasPrefix(out, "[118.300 MHZ](") {
		t.Fatalf("the label is not the author's token: %q", out)
	}
}

func TestWhatDoesNotDecorate(t *testing.T) {
	for _, message := range []string{
		"121.5",
		"freq:121.5",
		"Freq:121.5",
		"FREQ: 121.5",
		"FREQ:1.5",
		"FREQ:1500.0",
		"FREQ:999",
		"FREQ:123456",
		"FREQ:121.5 GHZ",
		"FREQ:121.5 KHZ",
		"FREQ:8992 MHZ",
		"XFREQ:121.5",
		"`FREQ:121.5`",
		"FREQ:121.5.6",
	} {
		t.Run(message, func(t *testing.T) {
			if out := decorate(t, message); strings.Contains(out, "/decorate/frequency?") {
				t.Fatalf("was decorated: %q", out)
			}
		})
	}
}

func TestATrailingDigitIsRefusedNotTruncated(t *testing.T) {
	if out := decorate(t, "FREQ:121.5678"); strings.Contains(out, "/decorate/") {
		t.Fatalf("a four-digit fraction was partly linked: %q", out)
	}
}

func TestTwoFrequenciesOnOneLineBothDecorate(t *testing.T) {
	out := decorate(t, "FREQ:121.5 FREQ:243.0")
	if strings.Count(out, "/decorate/frequency?") != 2 {
		t.Fatalf("not both linked: %q", out)
	}
}

func TestParseNormalizesToKilohertz(t *testing.T) {
	for token, want := range map[string]int{
		"121.5":       121_500,
		"118.3":       118_300,
		"118.300":     118_300,
		"118.300 MHZ": 118_300,
		"118.300MHZ":  118_300,
		"8992":        8_992,
		"8992 KHZ":    8_992,
		"2182 KHZ":    2_182,
		"406.0":       406_000,
		"1300.0":      1_300_000,
		"2.0":         2_000,
	} {
		f, ok := ParseToken(token)
		if !ok {
			t.Errorf("%q was refused", token)
			continue
		}
		if f.KHz != want {
			t.Errorf("%q = %d kHz, want %d", token, f.KHz, want)
		}
	}
}

func TestParseRefusesOutsideTheRange(t *testing.T) {
	for _, token := range []string{"1.999", "1300.001", "1999", "1300001", "0.5", "121.5 KHZ", "8992 MHZ"} {
		if _, ok := ParseToken(token); ok {
			t.Errorf("%q was accepted", token)
		}
	}
}

func TestDescribeNamesTheBandAndTheUse(t *testing.T) {
	for token, want := range map[string]Details{
		"121.5":     {Token: "121.5", MHz: "121.500", KHz: "121500", Band: "VHF air band", Channel: "25 kHz channel", Use: "Aeronautical emergency"},
		"243.0":     {Token: "243.0", MHz: "243.000", KHz: "243000", Band: "UHF military air band", Use: "UHF military emergency"},
		"118.305":   {Token: "118.305", MHz: "118.305", KHz: "118305", Band: "VHF air band", Channel: "8.33 kHz channel"},
		"8992 KHZ":  {Token: "8992 KHZ", MHz: "8.992", KHz: "8992", Band: "HF aeronautical"},
		"156.8":     {Token: "156.8", MHz: "156.800", KHz: "156800", Band: "VHF marine", Use: "Marine channel 16, distress and calling"},
		"1090.0":    {Token: "1090.0", MHz: "1090.000", KHz: "1090000", Band: OutsideBands},
		"114.1":     {Token: "114.1", MHz: "114.100", KHz: "114100", Band: "VHF navigation"},
		"406.0":     {Token: "406.0", MHz: "406.000", KHz: "406000", Band: "Distress beacons", Use: "Distress beacon, COSPAS-SARSAT"},
		"137.0":     {Token: "137.0", MHz: "137.000", KHz: "137000", Band: OutsideBands},
		"136.975":   {Token: "136.975", MHz: "136.975", KHz: "136975", Band: "VHF air band", Channel: "25 kHz channel"},
		"122.75":    {Token: "122.75", MHz: "122.750", KHz: "122750", Band: "VHF air band", Channel: "25 kHz channel", Use: "Air-to-air, fixed wing"},
		"30000 KHZ": {Token: "30000 KHZ", MHz: "30.000", KHz: "30000", Band: "HF aeronautical"},
	} {
		f, ok := ParseToken(token)
		if !ok {
			t.Fatalf("%q was refused", token)
		}
		if got := Describe(f); got != want {
			t.Errorf("Describe(%q) = %+v, want %+v", token, got, want)
		}
	}
}

func TestEveryBandIsOrderedAndDisjoint(t *testing.T) {
	last := 0
	for _, band := range Bands {
		if band.LowKHz <= last || band.HighKHz < band.LowKHz {
			t.Fatalf("band %q (%d to %d) is out of order or empty", band.Name, band.LowKHz, band.HighKHz)
		}
		if band.LowKHz < MinKHz || band.HighKHz > MaxKHz {
			t.Fatalf("band %q lies outside what Parse accepts", band.Name)
		}
		last = band.HighKHz
	}
}

func TestEveryAllocationLiesInsideABand(t *testing.T) {
	for _, allocation := range Allocations {
		if BandOf(allocation.KHz) == OutsideBands {
			t.Errorf("%d kHz (%s) is inside no band", allocation.KHz, allocation.Use)
		}
	}
}

func TestPatternsAreEmptyWhenSwitchedOff(t *testing.T) {
	d := &Decorator{Enabled: func() Formats { return Formats{} }}
	if len(d.Patterns()) != 0 {
		t.Fatal("a switched-off decorator still contributes patterns")
	}
}

func TestValidateRefusesWhatThePatternWouldNotProduce(t *testing.T) {
	for _, value := range []string{"", " 121.5", "121.5 ", "121,5", "121.5 mhz", "12.15e1", "121.5\n", "121.5 MHZ KHZ", "5000000"} {
		if _, ok := Validate(url.Values{ParamValue: {value}}); ok {
			t.Errorf("%q was accepted", value)
		}
	}
	if _, ok := Validate(url.Values{ParamValue: {"118.300\tMHZ"}}); !ok {
		t.Error("a tab between the number and the unit, which the pattern allows, was refused")
	}
}

func TestRenderPageShowsTheFrequency(t *testing.T) {
	rec := httptest.NewRecorder()
	(&Decorator{}).RenderPage(rec, url.Values{ParamValue: {"121.5"}})

	if rec.Code != 200 {
		t.Fatalf("status = %d", rec.Code)
	}
	body := rec.Body.String()
	for _, want := range []string{"121.5", "VHF air band", "121.500", "121500", "25 kHz channel", "Aeronautical emergency"} {
		if !strings.Contains(body, want) {
			t.Errorf("the page does not carry %q", want)
		}
	}
	if got := rec.Header().Get("Content-Security-Policy"); !strings.Contains(got, "script-src 'none'") {
		t.Errorf("policy = %q, want script-src 'none'", got)
	}
}

func TestRenderPageRefusesAHandEditedLink(t *testing.T) {
	rec := httptest.NewRecorder()
	(&Decorator{}).RenderPage(rec, url.Values{ParamValue: {"<b>121.5</b>"}})
	if rec.Code != 400 || !strings.Contains(rec.Body.String(), "TF-17005") {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "<b>") {
		t.Fatal("author text reached the page as markup")
	}
}

func TestDecorationIsIdempotent(t *testing.T) {
	once := decorate(t, "FREQ:121.5")
	if twice := decorate(t, once); twice != once {
		t.Fatalf("the second pass changed the message:\n%q\n%q", once, twice)
	}
}
