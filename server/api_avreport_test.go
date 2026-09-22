package main

import (
	"encoding/json"
	"net/http"
	"net/url"
	"testing"

	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/avreport"
	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/errcode"
)

func avreportURL(t *testing.T, text string) string {
	t.Helper()

	params, ok := (&avreport.Decorator{}).Parse(text, hookRef)
	if !ok {
		t.Fatalf("the fixture does not parse: %q", text)
	}
	return avreportPath + "?" + params.Encode()
}

func TestAvReportRequiresASession(t *testing.T) {
	p, _ := newAPIPlugin(t)

	rec := call(p, http.MethodGet, avreportURL(t, reportMETAR), "", "")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
	assertCode(t, rec.Body.String(), errcode.APINotAuthorized)
}

func TestAvReportRefusesAnythingButGet(t *testing.T) {
	p, _ := newAPIPlugin(t)

	for _, method := range []string{http.MethodPost, http.MethodPut, http.MethodDelete} {
		rec := call(p, method, avreportURL(t, reportMETAR), testUserID, "")
		if rec.Code != http.StatusMethodNotAllowed {
			t.Errorf("%s: status = %d, want 405", method, rec.Code)
		}
	}
}

func TestAvReportReturnsTheBlobForALinkTheServerIssued(t *testing.T) {
	p, _ := newAPIPlugin(t)

	rec := call(p, http.MethodGet, avreportURL(t, reportMETAR), testUserID, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (%s)", rec.Code, rec.Body.String())
	}
	if got := rec.Header().Get("Cache-Control"); got != "private, max-age=300" {
		t.Errorf("Cache-Control = %q", got)
	}

	var got map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("not a report: %v (%s)", err, rec.Body.String())
	}
	if got["kind"] != avreport.KindMETAR || got["station"] != "PHNL" || got["src"] != reportMETAR {
		t.Errorf("body = %v", got)
	}
	if _, present := got["summary"]; present {
		t.Error("the API answer still carries a summary")
	}
	if _, stamped := got["version"]; stamped {
		t.Error("the API answer carries stamp-only keys")
	}
}

func TestAvReportRefusesAHandEditedLink(t *testing.T) {
	p, _ := newAPIPlugin(t)

	for name, query := range map[string]url.Values{
		"another instant": {avreport.ParamValue: {reportMETAR}, avreport.ParamInstant: {"1700000000000"}},
		"no instant":      {avreport.ParamValue: {reportMETAR}},
		"not a report":    {avreport.ParamValue: {"hello there"}, avreport.ParamInstant: {"0"}},
		"nothing":         {},
		"a punctuation-only notam word": {
			avreport.ParamValue: {"!HNL 09/123 HNL RWY 08L/26R CLSD ."}, avreport.ParamInstant: {"0"},
		},
	} {
		t.Run(name, func(t *testing.T) {
			rec := call(p, http.MethodGet, avreportPath+"?"+query.Encode(), testUserID, "")
			if name == "a punctuation-only notam word" {
				if rec.Code != http.StatusOK {
					t.Fatalf("status = %d, want 200 for a report that decodes", rec.Code)
				}
				return
			}
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400 (%s)", rec.Code, rec.Body.String())
			}
			assertCode(t, rec.Body.String(), errcode.APIAvReportInvalid)
		})
	}
}

func TestAvReportAnswersTheMultiLineLinkTheTableWrites(t *testing.T) {
	p, _ := newAPIPlugin(t)

	report, err := avreport.Decode(reportTAF, hookRef)
	if err != nil {
		t.Fatal(err)
	}
	params := url.Values{avreport.ParamValue: {report.Raw}, avreport.ParamInstant: {"1787419200000"}}

	rec := call(p, http.MethodGet, avreportPath+"?"+params.Encode(), testUserID, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (%s)", rec.Code, rec.Body.String())
	}
	var got map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("not a report: %v", err)
	}
	if got["kind"] != avreport.KindTAF || got["src"] != reportTAF {
		t.Errorf("body = %v", got)
	}
}
