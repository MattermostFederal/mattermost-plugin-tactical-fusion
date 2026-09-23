package main

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/errcode"
)

func cyberURL(kind, value string) string {
	return cyberPath + "?" + url.Values{"k": {kind}, "v": {value}}.Encode()
}

func TestCyberRequiresASession(t *testing.T) {
	p, _ := newAPIPlugin(t)

	rec := call(p, http.MethodGet, cyberURL("cve", "CVE-2021-44228"), "", "")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
	assertCode(t, rec.Body.String(), errcode.APINotAuthorized)
}

func TestCyberRefusesAnythingButGet(t *testing.T) {
	p, _ := newAPIPlugin(t)

	for _, method := range []string{http.MethodPost, http.MethodPut, http.MethodDelete} {
		rec := call(p, method, cyberURL("cve", "CVE-2021-44228"), testUserID, "")
		if rec.Code != http.StatusMethodNotAllowed {
			t.Errorf("%s: status = %d, want 405", method, rec.Code)
		}
	}
}

func TestCyberAnswersWithTheIndicator(t *testing.T) {
	p, _ := newAPIPlugin(t)

	rec := call(p, http.MethodGet, cyberURL("attack", "T1059.001"), testUserID, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d (%s)", rec.Code, rec.Body.String())
	}

	var got cyberResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("not a cyber response: %v (%s)", err, rec.Body.String())
	}

	if got.Kind != "attack" || got.Value != "T1059.001" {
		t.Fatalf("kind = %q, value = %q", got.Kind, got.Value)
	}
	if got.Title != "PowerShell" {
		t.Errorf("title = %q", got.Title)
	}
	if len(got.Rows) == 0 || len(got.Related) == 0 || len(got.Datasets) == 0 {
		t.Errorf("rows = %d, related = %d, datasets = %d", len(got.Rows), len(got.Related), len(got.Datasets))
	}
}

func TestCyberListsAreNeverNull(t *testing.T) {
	p, _ := newAPIPlugin(t)

	rec := call(p, http.MethodGet, cyberURL("hash", strings.Repeat("a", 64)), testUserID, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}

	for _, field := range []string{`"rows":[`, `"related":[`, `"watchlist":[`, `"datasets":[`} {
		if !strings.Contains(rec.Body.String(), field) {
			t.Errorf("%s is not an array in %s", field, rec.Body.String())
		}
	}
}

func TestCyberRefusesAPairThatDisagreesWithItself(t *testing.T) {
	p, _ := newAPIPlugin(t)

	for _, path := range []string{
		cyberURL("cve", "CWE-79"),
		cyberURL("nonsense", "CVE-2021-44228"),
		cyberURL("cve", "cve-2021-44228"),
		cyberURL("attack", "T9999"),
		cyberURL("", "CVE-2021-44228"),
		cyberURL("cve", ""),
		cyberURL("cve", "<script>"),
	} {
		rec := call(p, http.MethodGet, path, testUserID, "")
		if rec.Code != http.StatusBadRequest {
			t.Errorf("%s: status = %d, want 400", path, rec.Code)
			continue
		}
		assertCode(t, rec.Body.String(), errcode.APICyberInvalid)
	}
}

func TestCyberCachesPrivatelyAndBriefly(t *testing.T) {
	p, _ := newAPIPlugin(t)

	rec := call(p, http.MethodGet, cyberURL("cwe", "CWE-79"), testUserID, "")
	if got := rec.Header().Get("Cache-Control"); got != "private, max-age=60" {
		t.Fatalf("Cache-Control = %q", got)
	}
}

func TestCyberRefusalIsNotCached(t *testing.T) {
	p, _ := newAPIPlugin(t)

	rec := call(p, http.MethodGet, cyberURL("cve", "nonsense"), testUserID, "")
	if got := rec.Header().Get("Cache-Control"); got != "" {
		t.Fatalf("a refusal carried Cache-Control %q", got)
	}
}

func TestCyberAnswersWhileTheDecoratorIsOff(t *testing.T) {
	p, _ := newAPIPlugin(t)
	p.setConfiguration(&configuration{})

	rec := call(p, http.MethodGet, cyberURL("cwe", "CWE-79"), testUserID, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 with everything switched off", rec.Code)
	}
}
