package airport

import (
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestLabeledIATACodesDecorate(t *testing.T) {
	for _, message := range []string{
		"IATA:HNL",
		"IATA :LAX",
		"IATA\t:KIN",
		"arrive IATA:HNL by dawn",
	} {
		t.Run(message, func(t *testing.T) {
			out := decorate(t, message)
			if out == message {
				t.Fatal("was not decorated")
			}
			if !strings.Contains(out, "/decorate/airport?i=") {
				t.Fatalf("the link does not carry the IATA parameter: %q", out)
			}
			if strings.Contains(out, "IATA:") {
				t.Fatalf("the label survived: %q", out)
			}
		})
	}
}

func TestIATALinkCarriesTheCodeAsWritten(t *testing.T) {
	out := decorate(t, "IATA:HNL")
	if !strings.HasPrefix(out, "[HNL](") {
		t.Fatalf("the link is not labeled with the author's code: %q", out)
	}
	if strings.Contains(out, "PHNL") {
		t.Fatalf("the resolved ident reached the stored message: %q", out)
	}
}

func TestIATASetLineTerminatorSurvives(t *testing.T) {
	out := decorate(t, "IATA:HNL//")
	if !strings.HasPrefix(out, "[HNL](") || !strings.HasSuffix(out, ")//") {
		t.Fatalf("the terminator did not survive outside the link: %q", out)
	}
}

func TestBareIATACodesAreNeverDecorated(t *testing.T) {
	for _, message := range []string{"HNL", "LAX", "fly to LAX tonight", "IATA HNL", "IATA: HNL"} {
		t.Run(message, func(t *testing.T) {
			if decorated(t, message) {
				t.Fatalf("was decorated: %q", decorate(t, message))
			}
		})
	}
}

func TestLowerCaseIATALabelsAreDeclined(t *testing.T) {
	for _, message := range []string{"iata:hnl", "Iata:HNL", "IATA:hnl"} {
		t.Run(message, func(t *testing.T) {
			if decorated(t, message) {
				t.Fatalf("was decorated: %q", decorate(t, message))
			}
		})
	}
}

func TestUnknownIATACodesDecline(t *testing.T) {
	for _, message := range []string{"IATA:QQQ", "IATA:HN", "IATA:HNLX", "IATA:H1L"} {
		t.Run(message, func(t *testing.T) {
			if strings.Contains(decorate(t, message), "/decorate/airport") {
				t.Fatalf("was decorated: %q", decorate(t, message))
			}
		})
	}
}

func TestTheIATAPatternIsOffWithItsSwitch(t *testing.T) {
	d := &Decorator{Enabled: func() Formats { return Formats{Airfield: true} }}
	if len(d.Patterns()) != 1 {
		t.Fatalf("%d patterns with IATA off, want the ICAO one alone", len(d.Patterns()))
	}

	off := &Decorator{Enabled: func() Formats { return Formats{IATA: true} }}
	if len(off.Patterns()) != 0 {
		t.Fatal("the IATA pattern is contributed while the parent is off")
	}
}

func TestParseAnswersTheIATAParamForAThreeLetterCode(t *testing.T) {
	params, ok := (&Decorator{}).Parse("HNL", time.Now())
	if !ok {
		t.Fatal("HNL was declined")
	}
	if len(params) != 1 || params.Get(ParamIATA) != "HNL" {
		t.Fatalf("params = %v, want i=HNL alone", params)
	}
}

func TestEveryIATACodeNamesOneAirfield(t *testing.T) {
	if IATACount() < 5000 {
		t.Fatalf("only %d IATA codes, far fewer than the data should hold", IATACount())
	}

	seen := map[string]string{}
	for ident, a := range airfields {
		if a.IATA == "" {
			continue
		}
		if !MatchesIATAShape(a.IATA) {
			t.Fatalf("%s carries IATA code %q, which is not three upper-case letters", ident, a.IATA)
		}
		if other, dup := seen[a.IATA]; dup {
			t.Fatalf("IATA code %s names both %s and %s", a.IATA, other, ident)
		}
		seen[a.IATA] = ident

		resolved, ok := LookupIATA(a.IATA)
		if !ok || resolved.Ident != ident {
			t.Fatalf("IATA code %s resolves to %q, want %s", a.IATA, resolved.Ident, ident)
		}
	}
}

func TestIATALookupPins(t *testing.T) {
	for code, ident := range map[string]string{"HNL": "PHNL", "LAX": "KLAX", "KIN": "MKJP", "LHR": "EGLL"} {
		a, ok := LookupIATA(code)
		if !ok || a.Ident != ident {
			t.Errorf("%s resolves to %q, want %s", code, a.Ident, ident)
		}
	}
	if _, ok := LookupIATA("hnl"); ok {
		t.Error("a lower-case code resolved, so the index folds case")
	}
}

func TestReferenceFromParamsRequiresExactlyOneCode(t *testing.T) {
	for name, params := range map[string]url.Values{
		"both":    {ParamValue: {"PHNL"}, ParamIATA: {"HNL"}},
		"neither": {},
		"other":   {"x": {"PHNL"}},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := ReferenceFromParams(params); err != ErrParamsConflict {
				t.Fatalf("err = %v, want ErrParamsConflict", err)
			}
		})
	}

	for name, params := range map[string]url.Values{
		"short ident":   {ParamValue: {"PHN"}},
		"lower ident":   {ParamValue: {"phnl"}},
		"long iata":     {ParamIATA: {"HNLL"}},
		"lower iata":    {ParamIATA: {"hnl"}},
		"empty iata":    {ParamIATA: {""}},
		"digit in iata": {ParamIATA: {"H1L"}},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := ReferenceFromParams(params); err != ErrParamsInvalid {
				t.Fatalf("err = %v, want ErrParamsInvalid", err)
			}
		})
	}
}

func TestReferenceResolvesEitherCode(t *testing.T) {
	byIdent, err := ReferenceFromParams(url.Values{ParamValue: {"PHNL"}})
	if err != nil {
		t.Fatal(err)
	}
	byCode, err := ReferenceFromParams(url.Values{ParamIATA: {"HNL"}})
	if err != nil {
		t.Fatal(err)
	}

	for _, ref := range []Reference{byIdent, byCode} {
		ident, ok := ref.Resolve()
		if !ok || ident != "PHNL" {
			t.Errorf("%v resolves to %q, want PHNL", ref, ident)
		}
	}
	if byIdent.Code() != "PHNL" || byCode.Code() != "HNL" {
		t.Errorf("codes are %q and %q, want the author's own", byIdent.Code(), byCode.Code())
	}

	unknown, _ := ReferenceFromParams(url.Values{ParamIATA: {"QQQ"}})
	if _, ok := unknown.Resolve(); ok {
		t.Error("an unknown IATA code resolved")
	}
}

func TestRenderPageResolvesAnIATACode(t *testing.T) {
	rec := httptest.NewRecorder()
	(&Decorator{}).RenderPage(rec, url.Values{ParamIATA: {"HNL"}})

	if rec.Code != 200 {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	for _, want := range []string{"Daniel K. Inouye International Airport", "PHNL", "HNL"} {
		if !strings.Contains(body, want) {
			t.Errorf("the page does not carry %q", want)
		}
	}
}

func TestRenderPageRefusesBothOrNeitherCode(t *testing.T) {
	for name, params := range map[string]url.Values{
		"both":    {ParamValue: {"PHNL"}, ParamIATA: {"HNL"}},
		"neither": {},
	} {
		t.Run(name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			(&Decorator{}).RenderPage(rec, params)
			if rec.Code != 400 {
				t.Fatalf("status = %d, want 400", rec.Code)
			}
			if !strings.Contains(rec.Body.String(), "TF-17003") {
				t.Errorf("body does not carry the conflict code: %s", rec.Body.String())
			}
		})
	}
}

func TestRenderPageRefusesAMalformedIATACode(t *testing.T) {
	rec := httptest.NewRecorder()
	(&Decorator{}).RenderPage(rec, url.Values{ParamIATA: {"hnl"}})
	if rec.Code != 400 {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "TF-17002") {
		t.Errorf("body does not carry the invalid code: %s", rec.Body.String())
	}
}

func TestRenderPageAnswersForAnIATACodeThisBuildDoesNotHold(t *testing.T) {
	rec := httptest.NewRecorder()
	(&Decorator{}).RenderPage(rec, url.Values{ParamIATA: {"QQQ"}})

	if rec.Code != 200 {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "QQQ") || !strings.Contains(rec.Body.String(), "not in this build") {
		t.Errorf("the page does not name the code and say why: %s", rec.Body.String())
	}
}

func TestExpandMessageResolvesAnIATACode(t *testing.T) {
	table := (&Decorator{}).ExpandMessage("/x", "", url.Values{ParamIATA: {"HNL"}})
	if !strings.Contains(table, "| Code | PHNL |") {
		t.Errorf("the table does not carry the resolved ident:\n%s", table)
	}
}

func TestEveryIATACodeDeclinesInCapitalizedProse(t *testing.T) {
	tagger := newTagger(t)
	now := time.Now().UTC()

	rewritten := 0
	for code := range iataIndex {
		message := iataPrefix + ": " + code + " IS THE NEXT WORD"
		if tagger.Decorate(message, now) != message {
			rewritten++
		}
	}
	if rewritten != 0 {
		t.Errorf("%d capitalized sentences were rewritten", rewritten)
	}
}

func TestEveryIATACodeDecoratesBehindItsLabel(t *testing.T) {
	if testing.Short() {
		t.Skip("the corpus sweep is slow under -race and coverage")
	}

	tagger := newTagger(t)
	now := time.Now().UTC()

	for code := range iataIndex {
		labeled := iataPrefix + ":" + code
		if tagger.Decorate(labeled, now) == labeled {
			t.Fatalf("%s did not decorate behind its label", labeled)
		}
		if tagger.Decorate(code, now) != code {
			t.Fatalf("%s decorated bare", code)
		}
	}
}
