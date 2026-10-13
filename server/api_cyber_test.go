package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/decorators/cyber"
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

func TestCyberListsEachDataSourceWithWhenItWasCompiled(t *testing.T) {
	p, _ := newAPIPlugin(t)

	rec := call(p, http.MethodGet, cyberURL("attack", "T1059.001"), testUserID, "")
	var got cyberResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("not a cyber response: %v (%s)", err, rec.Body.String())
	}

	if len(got.Compiled) != 1 {
		t.Fatalf("compiled = %+v", got.Compiled)
	}
	source := got.Compiled[0]
	if source.Label != "MITRE ATT&CK catalog" || source.File != "attack.tsv (built in)" ||
		!strings.HasSuffix(source.Date, " UTC") || !strings.Contains(source.Query, "dtg=") {
		t.Errorf("source = %+v", source)
	}
}

func TestCyberWithNothingCompiledListsNoSources(t *testing.T) {
	p, _ := newAPIPlugin(t)

	rec := call(p, http.MethodGet, cyberURL("hash", strings.Repeat("a", 64)), testUserID, "")
	if !strings.Contains(rec.Body.String(), `"compiled":[]`) {
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

func TestCyberBodyCarriesEveryCollection(t *testing.T) {
	at := time.Date(2026, time.September, 22, 16, 51, 0, 0, time.UTC)
	details := cyber.Details{
		Kind:           cyber.KindCVE,
		Value:          "CVE-2021-44228",
		Title:          "Log4Shell",
		Headline:       "Critical",
		Summary:        "Remote code execution",
		Status:         "listed",
		Rows:           []cyber.Row{{Label: "Published", Value: "2021-12-10", At: at}, {Label: "Vendor", Value: "Apache"}},
		Related:        []cyber.Link{{Kind: cyber.KindCWE, Value: "CWE-502", Label: "Deserialization"}},
		Watchlist:      []cyber.WatchEntry{{Verdict: "listed", Source: "KEV", Note: "ransomware", Updated: "2021-12-10", Known: true}},
		Datasets:       []cyber.DatasetStatus{{Name: "kev", Label: "known exploited", Present: true, Generated: "2026-09-01"}},
		Score:          "10.0",
		Severity:       "Critical",
		Exploited:      true,
		Vector:         []cyber.VectorMetric{{Metric: "Attack vector", Value: "Network", Severe: true}},
		Affected:       []string{"log4j-core"},
		Configurations: []string{"cpe:2.3:a:apache:log4j"},
		References:     []cyber.Reference{{URL: "https://example.test/advisory", Tags: "Vendor Advisory"}},
		Sections: []cyber.Section{{Title: "Weaknesses", Items: []cyber.Item{
			{Head: "CWE-502", Text: "Deserialization", Link: &cyber.Link{Kind: cyber.KindCWE, Value: "CWE-502"}},
			{Head: "Note", Text: "plain", URL: "https://example.test/note"},
		}}},
		Credits:   []cyber.Credit{{Text: "NVD", URL: "https://nvd.example.test"}},
		Glance:    cyber.Glance{Subtitle: "sub", Summary: "sum", Tags: []string{"rce"}, Facts: []string{"fact"}, Status: "st"},
		Reports:   []cyber.Report{{Source: "feed", Malicious: true, Threat: "botnet", Detail: "seen", URL: "https://example.test/r"}},
		Freshness: []cyber.Freshness{{Label: "KEV", File: "kev.tsv", Compiled: at}},
	}

	got := cyberBody(details)

	want := cyberResponse{
		Kind: "cve", Value: "CVE-2021-44228", Title: "Log4Shell", Headline: "Critical", Summary: "Remote code execution", Status: "listed",
		Rows: []cyberRow{
			{Label: "Published", Value: "2021-12-10", Query: dtg.QueryForZulu(at)},
			{Label: "Vendor", Value: "Apache"},
		},
		Related:        []cyberLink{{Kind: "cwe", Value: "CWE-502", Label: "Deserialization"}},
		Watchlist:      []cyberWatchEntry{{Verdict: "listed", Source: "KEV", Note: "ransomware", Updated: "2021-12-10", Known: true}},
		Datasets:       []cyberDataset{{Name: "kev", Label: "known exploited", Present: true, Generated: "2026-09-01"}},
		Score:          "10.0",
		Severity:       "Critical",
		Exploited:      true,
		Vector:         []cyberVectorMetric{{Metric: "Attack vector", Value: "Network", Severe: true}},
		Affected:       []string{"log4j-core"},
		Configurations: []string{"cpe:2.3:a:apache:log4j"},
		References:     []cyberReference{{URL: "https://example.test/advisory", Tags: "Vendor Advisory"}},
		Sections: []cyberSection{{Title: "Weaknesses", Items: []cyberItem{
			{Head: "CWE-502", Text: "Deserialization", Kind: "cwe", Value: "CWE-502"},
			{Head: "Note", Text: "plain", URL: "https://example.test/note"},
		}}},
		Credits:  []cyberCredit{{Text: "NVD", URL: "https://nvd.example.test"}},
		Glance:   cyberGlance{Subtitle: "sub", Summary: "sum", Tags: []string{"rce"}, Facts: []string{"fact"}, Status: "st"},
		Reports:  []cyberReport{{Source: "feed", Malicious: true, Threat: "botnet", Detail: "seen", URL: "https://example.test/r"}},
		Compiled: []cyberCompiled{{Label: "KEV", File: "kev.tsv", Date: cyber.CompiledText(at), Query: dtg.QueryForZulu(at)}},
	}
	if !reflect.DeepEqual(got, want) {
		gotJSON, _ := json.MarshalIndent(got, "", " ")
		wantJSON, _ := json.MarshalIndent(want, "", " ")
		t.Errorf("body =\n%s\nwant\n%s", gotJSON, wantJSON)
	}
	if want.Rows[0].Query == "" {
		t.Error("a dated row carries no DTG query")
	}
}

func TestCyberBodyMarshalsEmptyCollectionsAsArrays(t *testing.T) {
	raw, err := json.Marshal(cyberBody(cyber.Details{Kind: cyber.KindIP, Value: "192.0.2.1"}))
	if err != nil {
		t.Fatal(err)
	}

	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"rows", "related", "watchlist", "datasets", "vector", "affected", "configurations", "references", "sections", "credits", "reports", "compiled"} {
		if got := string(fields[key]); got != "[]" {
			t.Errorf("%s = %s, want []", key, got)
		}
	}
	var glance map[string]json.RawMessage
	if err := json.Unmarshal(fields["glance"], &glance); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"tags", "facts"} {
		if got := string(glance[key]); got != "[]" {
			t.Errorf("glance.%s = %s, want []", key, got)
		}
	}
}

func TestCyberSectionOfAnEmptySectionIsAnEmptyArray(t *testing.T) {
	got := cyberSectionOf(cyber.Section{Title: "Empty"})
	if got.Items == nil || len(got.Items) != 0 {
		t.Errorf("items = %#v, want an empty slice", got.Items)
	}
}
