package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/decorators/cyber/intel"
	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/decorators/dtg"
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

func TestCyberSaysWhenItsDataWasCompiled(t *testing.T) {
	p, _ := newAPIPlugin(t)

	rec := call(p, http.MethodGet, cyberURL("attack", "T1059.001"), testUserID, "")
	var got cyberResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("not a cyber response: %v (%s)", err, rec.Body.String())
	}

	if !strings.HasSuffix(got.Current.Date, " UTC") || !strings.Contains(got.Current.Query, "dtg=") {
		t.Errorf("current = %+v", got.Current)
	}
	if len(got.Current.Sources) != 1 || got.Current.Sources[0].Label != "MITRE ATT&CK catalog" || got.Current.Sources[0].Date != got.Current.Date {
		t.Errorf("sources = %+v", got.Current.Sources)
	}
}

func TestCyberWithNothingCompiledClaimsNoDate(t *testing.T) {
	p, _ := newAPIPlugin(t)

	rec := call(p, http.MethodGet, cyberURL("hash", strings.Repeat("a", 64)), testUserID, "")
	if !strings.Contains(rec.Body.String(), `"current":{"date":"","query":"","sources":[]}`) {
		t.Errorf("an undated answer is %s", rec.Body.String())
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

func TestCyberLinksATimestampRowToTheDateTimeGroupDecorator(t *testing.T) {
	dir := t.TempDir()
	body := fmt.Sprintf("%s%d\t%s\t2026-09-01T00:00:00Z\ttest\n", intel.SchemaPrefix, intel.SchemaVersion, intel.NameCVE) +
		strings.Join([]string{
			"CVE-2025-55182", "2025-12-03T16:15:56.463", "2025-12-10", "10.0", "Critical", "AV:N", "", "React2Shell",
		}, "\t") + "\n"
	if err := os.WriteFile(filepath.Join(dir, intel.NameCVE+intel.Suffix), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	p := withDatasetDir(t, dir)

	rec := call(p, http.MethodGet, cyberURL("cve", "CVE-2025-55182"), testUserID, "")
	var got cyberResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("not a cyber response: %v (%s)", err, rec.Body.String())
	}

	want := dtg.QueryForZulu(time.Date(2025, 12, 3, 16, 16, 0, 0, time.UTC))
	for _, row := range got.Rows {
		switch row.Label {
		case "Published":
			if row.Query != want {
				t.Errorf("published query = %q, want %q", row.Query, want)
			}
		default:
			if row.Query != "" {
				t.Errorf("%s carries the query %q although it names no instant", row.Label, row.Query)
			}
		}
	}
}
