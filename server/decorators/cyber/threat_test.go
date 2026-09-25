package cyber

import (
	"strings"
	"testing"

	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/decorators/cyber/intel"
)

const (
	threatRow = "203.0.113.10\t" +
		`[{"source":"CISA AA99-002A","category":"malicious","threat":"Botnet C2","malware":"InventedBot","confidence":"100","ports":"443,8443","firstSeen":"2026-08-01","lastSeen":"2026-09-20","url":"https://www.cisa.gov/news-events/cybersecurity-advisories/aa99-002a"},` +
		`{"source":"Tor Project","category":"context","threat":"Tor exit node","url":"javascript:alert(1)"}]`
	advisoryReport = `{"source":"CISA AA99-001A","category":"malicious","threat":"Invented Ransomware Campaign","status":"Advisory published 2026-09-01","firstSeen":"2026-08-15","url":"https://www.cisa.gov/news-events/cybersecurity-advisories/aa99-001a"}`
)

func addressRow() string {
	return strings.Replace(threatRow, "\t[", "\t["+advisoryReport+",", 1)
}

func TestAnAddressCarriesEveryReportAndSaysItIsReportedMalicious(t *testing.T) {
	set := datasets(t, map[string][]string{intel.NameAdvisory: {addressRow()}})

	d := Describe(KindIP, "203.0.113.10", set)

	if len(d.Reports) != 3 {
		t.Fatalf("reports %+v", d.Reports)
	}
	if got := d.Reports[0]; got.Source != "CISA AA99-001A" || !got.Malicious || got.Detail != "first seen 2026-08-15 · Advisory published 2026-09-01" {
		t.Errorf("advisory report %+v", got)
	}
	if got := d.Reports[1]; got.Detail != "InventedBot · ports 443, 8443 · confidence 100% · seen 2026-08-01 to 2026-09-20" {
		t.Errorf("feed report detail %q", got.Detail)
	}
	if got := d.Reports[2]; got.Malicious || got.URL != "" {
		t.Errorf("a Tor exit was called malicious or kept a javascript: address: %+v", got)
	}
	if !strings.HasSuffix(d.Headline, "reported malicious") {
		t.Errorf("headline %q", d.Headline)
	}
}

func TestATorExitAloneIsNotReportedMalicious(t *testing.T) {
	row := "198.51.100.5\t" + `[{"source":"Tor Project","category":"context","threat":"Tor exit node"}]`
	set := datasets(t, map[string][]string{intel.NameAdvisory: {row}})

	d := Describe(KindIP, "198.51.100.5", set)

	if len(d.Reports) != 1 || strings.Contains(d.Headline, "malicious") {
		t.Fatalf("headline %q, reports %+v", d.Headline, d.Reports)
	}
}

func TestAHashCarriesItsReports(t *testing.T) {
	digest := strings.Repeat("ab", 16)
	set := datasets(t, map[string][]string{intel.NameAdvisory: {digest + "\t" + `[{"source":"CISA AA99-002A","category":"malicious","threat":"Malware payload","malware":"InventedBot"}]`}})

	d := Describe(KindHash, strings.ToUpper(digest), set)

	if len(d.Reports) != 1 || d.Reports[0].Detail != "InventedBot" {
		t.Fatalf("reports %+v", d.Reports)
	}
}

func TestAnIndicatorNoFeedNamesCarriesNoReportsAndNoNoise(t *testing.T) {
	set := datasets(t, map[string][]string{intel.NameAdvisory: {threatRow}})

	d := Describe(KindIP, "192.0.2.1", set)

	if len(d.Reports) != 0 || rowValue(d, "Threat reports") != "" || strings.Contains(d.Headline, "malicious") {
		t.Fatalf("reports %+v, row %q, headline %q", d.Reports, rowValue(d, "Threat reports"), d.Headline)
	}
}

func TestThePageListsTheReportsWithTheirSources(t *testing.T) {
	set := datasets(t, map[string][]string{intel.NameAdvisory: {addressRow()}})

	body := renderBody(Describe(KindIP, "203.0.113.10", set))

	for _, want := range []string{
		"<h2>Threat reports</h2>",
		`<a href="https://www.cisa.gov/news-events/cybersecurity-advisories/aa99-001a" rel="noopener noreferrer" target="_blank">CISA AA99-001A</a>`,
		`<span class="severe">Botnet C2</span>`,
		`<span class="context">Tor exit node</span>`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("the page lacks %q", want)
		}
	}
	if strings.Contains(body, "javascript:") {
		t.Fatalf("the page wrote a javascript: address")
	}
}

func TestAMalwareSampleReportNamesItsFile(t *testing.T) {
	detail := reportDetail(intel.ThreatReport{Malware: "InventedBot", File: "invoice.exe (exe)", FirstSeen: "2026-09-20"})

	if detail != "InventedBot · file invoice.exe (exe) · first seen 2026-09-20" {
		t.Fatalf("detail %q", detail)
	}
}
