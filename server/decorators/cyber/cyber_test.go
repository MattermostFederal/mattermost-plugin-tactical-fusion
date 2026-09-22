package cyber

import (
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestKindsDecorateInOrdinaryProse(t *testing.T) {
	cases := []struct {
		message string
		link    string
		label   string
	}{
		{"Patched CVE-2021-44228 today", "k=cve&v=CVE-2021-44228", "CVE-2021-44228"},
		{"patched cve-2021-44228 today", "k=cve&v=CVE-2021-44228", "cve-2021-44228"},
		{"see CWE-79 for the class", "k=cwe&v=CWE-79", "CWE-79"},
		{"see cwe-79 for the class", "k=cwe&v=CWE-79", "cwe-79"},
		{"mapped to T1059 already", "k=attack&v=T1059", "T1059"},
		{"mapped to T1059.001 already", "k=attack&v=T1059.001", "T1059.001"},
		{"the TA0002 column", "k=attack&v=TA0002", "TA0002"},
		{"beacon to 203.0.113.7 hourly", "k=ip&v=203.0.113.7", "203.0.113.7"},
		{"beacon to 2001:db8::1 hourly", "k=ip&v=2001%3Adb8%3A%3A1", "2001:db8::1"},
		{
			"sample 44d88612fea8a8f36de82e1278abb02f matched",
			"k=hash&v=44d88612fea8a8f36de82e1278abb02f",
			"44d88612fea8a8f36de82e1278abb02f",
		},
	}

	for _, tc := range cases {
		t.Run(tc.message, func(t *testing.T) {
			out := decorate(t, tc.message)
			if out == tc.message {
				t.Fatalf("was not decorated")
			}
			if !strings.Contains(out, tc.link) {
				t.Fatalf("no %q in %q", tc.link, out)
			}
			if !strings.Contains(out, "["+tc.label+"]") {
				t.Fatalf("the label is not the author's token in %q", out)
			}
		})
	}
}

func TestTheLabelIsTheAuthorsToken(t *testing.T) {
	out := decorate(t, "cve-2021-44228")
	if !strings.HasPrefix(out, "[cve-2021-44228](") {
		t.Fatalf("the stored label was rewritten: %q", out)
	}
}

func TestNothingElseDecorates(t *testing.T) {
	for _, message := range []string{
		"T9999 is not a technique",
		"CWE-99999 is not a weakness",
		"CVE-1998-0001 predates the scheme",
		"1.2.3.4.5 is not an address",
		"01.2.3.4 has a leading zero",
		"999.1.1.1 is out of range",
		"v1.2.3.4 is a version",
		"the range 1.2.3.0-1.2.3.255",
		"0.0.0.0 is unspecified",
		"127.0.0.1 is loopback",
		"::1 is loopback",
		"::ffff:0102:0304 is a mapped address",
		"::ffff:1.2.3.4 is the same one written out",
		"deploy at 12:30:45 sharp",
		"the mac 00:1A:2B:3C:4D:5E responded",
		"use std::vector here",
		"a ratio of 1:2:3 overall",
		"deadbeefdeadbeefdeadbeefdeadbeefd is 33 digits",
		"md5:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855 is the wrong length",
	} {
		t.Run(message, func(t *testing.T) {
			if decorated(t, message) {
				t.Fatalf("decorated: %q", decorate(t, message))
			}
		})
	}
}

func TestCIDRIsDeclined(t *testing.T) {
	if decorated(t, "blocked 1.2.3.4/24 at the edge") {
		t.Fatalf("a CIDR was rewritten")
	}
}

func TestConsumedTailsStayInTheMessage(t *testing.T) {
	cases := []struct {
		message string
		want    string
	}{
		{"Patched CVE-2021-44228.", "."},
		{"seen at 203.0.113.7:443 twice", ":443"},
		{"connect 203.0.113.7: connection refused", ": connection refused"},
		{"fe80::a%eth0 on the wire", "%eth0"},
		{"md5:44d88612fea8a8f36de82e1278abb02f matched", "md5:"},
		{"sha256=e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855 matched", "sha256="},
	}

	for _, tc := range cases {
		t.Run(tc.message, func(t *testing.T) {
			out := decorate(t, tc.message)
			if out == tc.message {
				t.Fatalf("was not decorated")
			}
			if !strings.Contains(out, tc.want) {
				t.Fatalf("%q was consumed out of %q", tc.want, out)
			}
		})
	}
}

func TestTwoIndicatorsOnOneLineBothDecorate(t *testing.T) {
	cases := []string{
		"CVE-2021-44228, CVE-2021-45046",
		"203.0.113.7 and 198.51.100.9",
		"T1059.001 then T1021.001",
		"CWE-79 and CWE-89",
	}

	for _, message := range cases {
		t.Run(message, func(t *testing.T) {
			out := decorate(t, message)
			if got := strings.Count(out, "/decorate/cyber?"); got != 2 {
				t.Fatalf("%d links in %q, want 2", got, out)
			}
		})
	}
}

func TestProtectedSpansAreNotDecorated(t *testing.T) {
	for _, message := range []string{
		"#CVE-2021-44228",
		"~cve-2021-44228",
		"@CVE-2021-44228",
		"`CVE-2021-44228`",
		"```\nCVE-2021-44228\n```",
		"[CVE-2021-44228](https://example.com)",
		"https://example.com/CVE-2021-44228",
	} {
		t.Run(message, func(t *testing.T) {
			if decorated(t, message) {
				t.Fatalf("decorated: %q", decorate(t, message))
			}
		})
	}
}

func TestATokenEndingOnTheRuneBeforeAMentionStillDecorates(t *testing.T) {
	for _, message := range []string{"203.0.113.7,@bob", "203.0.113.7 @bob", "CVE-2021-44228,#log4shell"} {
		t.Run(message, func(t *testing.T) {
			if !decorated(t, message) {
				t.Fatalf("was not decorated")
			}
		})
	}
}

func TestATokenInsideAnEmailAddressIsNeverRewritten(t *testing.T) {
	for _, message := range []string{
		"root@203.0.113.7",
		"Bounce from d41d8cd98f00b204e9800998ecf8427e@lists.example.mil",
		"soc@203.0.113.7 is the relay",
		"reply-to: cve-2021-44228@vendor.example.com",
	} {
		t.Run(message, func(t *testing.T) {
			if decorated(t, message) {
				t.Fatalf("decorated inside an address: %q", decorate(t, message))
			}
		})
	}
}

func TestDecorationIsIdempotent(t *testing.T) {
	once := decorate(t, "Patched CVE-2021-44228 today")
	if twice := decorate(t, once); twice != once {
		t.Fatalf("a second pass changed the message\n got: %s\nwant: %s", twice, once)
	}
}

func TestPatternsFollowTheSwitches(t *testing.T) {
	if patterns := (&Decorator{Enabled: func() Formats { return Formats{} }}).Patterns(); patterns != nil {
		t.Fatalf("a decorator with everything off contributed %d patterns", len(patterns))
	}

	only := &Decorator{Enabled: func() Formats { return Formats{CVE: true} }}
	if len(only.Patterns()) != 1 {
		t.Fatalf("one kind on gave %d patterns", len(only.Patterns()))
	}

	if _, ok := only.Parse("203.0.113.7", time.Now()); ok {
		t.Fatalf("a disabled kind still parsed")
	}
	if _, ok := only.Parse("CVE-2021-44228", time.Now()); !ok {
		t.Fatalf("the enabled kind did not parse")
	}
}

func TestParseCarriesKindAndCanonicalValue(t *testing.T) {
	params, ok := (&Decorator{}).Parse("cve-2021-44228", time.Now())
	if !ok {
		t.Fatalf("did not parse")
	}
	if got := params.Get(ParamKind); got != string(KindCVE) {
		t.Fatalf("kind %q", got)
	}
	if got := params.Get(ParamValue); got != "CVE-2021-44228" {
		t.Fatalf("value %q", got)
	}
	if len(params) != 2 {
		t.Fatalf("the URL carries %d parameters, and nothing derived may travel in it", len(params))
	}
}

func TestRecognizeAsRequiresTheLinkToAgreeWithItself(t *testing.T) {
	cases := []struct {
		kind  Kind
		value string
		want  bool
	}{
		{KindCVE, "CVE-2021-44228", true},
		{KindCWE, "CWE-79", true},
		{KindAttack, "T1059.001", true},
		{KindIP, "203.0.113.7", true},
		{KindHash, "44d88612fea8a8f36de82e1278abb02f", true},

		{KindCVE, "CWE-79", false},
		{KindCWE, "CVE-2021-44228", false},
		{KindIP, "CVE-2021-44228", false},
		{KindCVE, "cve-2021-44228", false},
		{KindCWE, "CWE-079", false},
		{KindHash, "44D88612FEA8A8F36DE82E1278ABB02F", false},
		{KindAttack, "T9999", false},
		{"nonsense", "CVE-2021-44228", false},
		{KindIP, "1.2.3.4.5", false},
		{KindIP, "::ffff:1.2.3.4", false},
	}

	for _, tc := range cases {
		t.Run(string(tc.kind)+"/"+tc.value, func(t *testing.T) {
			if got := RecognizeAs(tc.kind, tc.value); got != tc.want {
				t.Fatalf("RecognizeAs(%q, %q) = %v", tc.kind, tc.value, got)
			}
		})
	}
}

func TestPageRefusesALinkThatDisagreesWithItself(t *testing.T) {
	for _, params := range []url.Values{
		{ParamKind: {"cve"}, ParamValue: {"CWE-79"}},
		{ParamKind: {"nonsense"}, ParamValue: {"CVE-2021-44228"}},
		{ParamKind: {"cve"}, ParamValue: {"<script>"}},
		{ParamValue: {"CVE-2021-44228"}},
		{},
	} {
		recorder := httptest.NewRecorder()
		(&Decorator{}).RenderPage(recorder, params)

		if recorder.Code != 400 {
			t.Fatalf("%v answered %d, want 400", params, recorder.Code)
		}
		if strings.Contains(recorder.Body.String(), "<script>") {
			t.Fatalf("the refusal echoed the value as markup")
		}
	}
}

func TestPageRendersWithNoDatasets(t *testing.T) {
	recorder := httptest.NewRecorder()
	(&Decorator{}).RenderPage(recorder, url.Values{
		ParamKind: {"cve"}, ParamValue: {"CVE-2021-44228"},
	})

	if recorder.Code != 200 {
		t.Fatalf("answered %d", recorder.Code)
	}

	body := recorder.Body.String()
	if !strings.Contains(body, "No vulnerability dataset is installed.") {
		t.Fatalf("the page does not say the dataset is absent:\n%s", body)
	}
	if !strings.Contains(body, "CVE-2021-44228") {
		t.Fatalf("the page does not name the indicator")
	}
}

func TestThePageCarriesNoScript(t *testing.T) {
	recorder := httptest.NewRecorder()
	(&Decorator{}).RenderPage(recorder, url.Values{
		ParamKind: {"attack"}, ParamValue: {"T1059.001"},
	})

	if strings.Contains(recorder.Body.String(), "<script") {
		t.Fatalf("the cyber page grew a script")
	}
	if got := recorder.Header().Get("Content-Security-Policy"); !strings.Contains(got, "script-src 'none'") {
		t.Fatalf("policy %q", got)
	}
}

func TestThePageCachesPrivatelyAndBriefly(t *testing.T) {
	recorder := httptest.NewRecorder()
	(&Decorator{}).RenderPage(recorder, url.Values{
		ParamKind: {"cwe"}, ParamValue: {"CWE-79"},
	})

	if got := recorder.Header().Get("Cache-Control"); got != "private, max-age=60" {
		t.Fatalf("Cache-Control %q", got)
	}
}

func TestEveryRecognizedTokenReproducesItself(t *testing.T) {
	tokens := []string{
		"CVE-2021-44228", "cve-2021-44228", "CVE-1999-0001",
		"CWE-79", "cwe-79", "CWE-502",
		"T1059", "T1059.001", "TA0002",
		"203.0.113.7", "8.8.8.8", "2001:db8::1", "2606:4700::1",
		"44d88612fea8a8f36de82e1278abb02f",
		"da39a3ee5e6b4b0d3255bfef95601890afd80709",
		strings.Repeat("a", 64),
		strings.ToUpper("da39a3ee5e6b4b0d3255bfef95601890afd80709"),
	}

	for _, token := range tokens {
		t.Run(token, func(t *testing.T) {
			kind, canonical, ok := Recognize(token)
			if !ok {
				t.Fatalf("%q is not recognized", token)
			}
			if !RecognizeAs(kind, canonical) {
				t.Fatalf("%q canonicalizes to %q, which does not reproduce itself", token, canonical)
			}
			if !MatchesShape(kind, canonical) {
				t.Fatalf("%q canonicalizes to %q, which its own shape refuses", token, canonical)
			}
		})
	}
}

func TestEveryScannedKindHasAShapeThatAdmitsIt(t *testing.T) {
	for _, kind := range Kinds {
		if ShapeExpr(kind) == "" {
			t.Errorf("%s has no shape expression", kind)
		}
	}
}
