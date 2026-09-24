package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/decorators/cyber"
	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/decorators/cyber/intel"
)

const (
	mcpCVE     = "CVE-2025-55182"
	mcpAddress = "203.0.113.10"
	mcpMD5     = "22222222222222222222222222222222"
	mcpSHA256  = "1111111111111111111111111111111111111111111111111111111111111111"
)

func writeCyberDataset(t *testing.T, dir, name string, rows ...string) {
	t.Helper()

	body := fmt.Sprintf("%s%d\t%s\t2026-09-01T00:00:00Z\ttest\n", intel.SchemaPrefix, intel.SchemaVersion, name) + strings.Join(rows, "\n") + "\n"
	if err := os.WriteFile(filepath.Join(dir, name+intel.Suffix), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

func referenceRows(n int) string {
	refs := make([]string, 0, n)
	for i := range n {
		refs = append(refs, fmt.Sprintf(`{"url":"https://example.org/%d","tags":["Patch"]}`, i))
	}
	return "[" + strings.Join(refs, ",") + "]"
}

func mcpCyberPlugin(t *testing.T) *Plugin {
	t.Helper()

	dir := t.TempDir()
	writeCyberDataset(t, dir, intel.NameCVE, strings.Join([]string{
		mcpCVE, "2025-12-03T16:15:56.463", "2025-12-10", "10.0", "Critical",
		"CVSS:3.1/AV:N/AC:L/PR:N/UI:N/S:C/C:H/I:H/A:H", "CWE-502", "Remote code execution in an invented framework.",
	}, "\t"))
	writeCyberDataset(t, dir, intel.NameEPSS, mcpCVE+"\t0.99802\t0.99958\t2026-09-23")
	writeCyberDataset(t, dir, intel.NameKEV, strings.Join([]string{mcpCVE, "2025-12-05", "2025-12-12", "Known", "Invented", "Patch."}, "\t"))
	writeCyberDataset(t, dir, intel.NameCVEDetail, mcpCVE+"\t\t\t\t"+referenceRows(12))
	writeCyberDataset(t, dir, intel.NameThreat, mcpAddress+"\t"+
		`[{"source":"abuse.ch ThreatFox","category":"malicious","threat":"Botnet C2","malware":"InventedBot","ports":"443"},`+
		`{"source":"Tor Project","category":"context","threat":"Tor exit node"}]`)
	writeCyberDataset(t, dir, intel.NameMalware,
		mcpSHA256+"\t\t2026-09-20\tinvoice.exe\texe\tInventedBot",
		mcpMD5+"\t"+mcpSHA256+"\t\t\t\t",
	)

	p := newTestPlugin(t, "https://example.com", true)
	config := p.getConfiguration().Clone()
	config.CyberDatasetsDir = dir
	p.setConfiguration(config)
	if err := p.ensureMCPServer(); err != nil {
		t.Fatalf("ensureMCPServer: %v", err)
	}
	return p
}

func lookupCyber(t *testing.T, p *Plugin, indicators ...string) CyberIndicatorLookup {
	t.Helper()

	session := mcpSession(t, p, map[string]string{"Mattermost-Plugin-ID": agentsPluginID})
	return decodeMCPResult[CyberIndicatorLookup](t, callMCPTool(t, session, "lookup_cyber_indicator", LookupCyberIndicatorArgs{Indicators: indicators}))
}

func factValue(result CyberIndicatorResult, label string) string {
	for _, fact := range result.Facts {
		if fact.Label == label {
			return fact.Value
		}
	}
	return ""
}

func TestTheCyberToolAnswersAVulnerabilityWithItsSignals(t *testing.T) {
	got := lookupCyber(t, mcpCyberPlugin(t), mcpCVE).Results[0]

	if !got.Recognized || got.Kind != "cve" || got.Severity != "critical" || got.Score != "10.0" || !got.Exploited {
		t.Fatalf("result %+v", got)
	}
	if !strings.HasPrefix(factValue(got, "EPSS"), "99.802%") || factValue(got, "Action due") != "2025-12-12" {
		t.Errorf("facts %+v", got.Facts)
	}
	if len(got.Vector) == 0 || got.Vector[0] != (CyberVectorEntry{Metric: "Attack vector", Value: "Network", Severe: true}) {
		t.Errorf("vector %+v", got.Vector)
	}
	if !strings.Contains(got.Link, "/decorate/cyber?") || !strings.HasPrefix(got.Link, "["+mcpCVE+"](") {
		t.Errorf("link %q", got.Link)
	}
}

func TestTheCyberToolCutsALongListAndSaysHowLongItWas(t *testing.T) {
	got := lookupCyber(t, mcpCyberPlugin(t), mcpCVE).Results[0]

	var references *CyberToolSection
	for i := range got.Sections {
		if got.Sections[i].Title == "References" {
			references = &got.Sections[i]
		}
	}
	if references == nil || references.Total != 12 || !references.Truncated || len(references.Items) != maxCyberToolItems {
		t.Fatalf("references %+v", references)
	}
}

func TestTheCyberToolDoesNotMarkAShortListTruncated(t *testing.T) {
	section := toolSection("s", lineItems([]string{"a", "b"}))
	if section.Truncated || section.Total != 2 || len(section.Items) != 2 {
		t.Fatalf("section %+v", section)
	}
}

func TestTheCyberToolAnswersAnAddressWithItsReports(t *testing.T) {
	got := lookupCyber(t, mcpCyberPlugin(t), mcpAddress).Results[0]

	if len(got.Reports) != 2 || !got.Reports[0].Malicious || got.Reports[1].Malicious || got.Reports[1].Threat != "Tor exit node" {
		t.Fatalf("reports %+v", got.Reports)
	}
	if !strings.HasSuffix(got.Headline, "reported malicious") {
		t.Errorf("headline %q", got.Headline)
	}
}

func TestTheCyberToolFindsAMalwareSampleByItsMD5(t *testing.T) {
	got := lookupCyber(t, mcpCyberPlugin(t), strings.ToUpper(mcpMD5)).Results[0]

	if len(got.Reports) != 1 || got.Reports[0].Source != "abuse.ch MalwareBazaar" || !strings.Contains(got.Reports[0].Detail, "invoice.exe") {
		t.Fatalf("reports %+v", got.Reports)
	}
}

func TestTheCyberToolAnswersACatalogEntryWithNoDatasets(t *testing.T) {
	got := lookupCyber(t, mcpPlugin(t), "T1562").Results[0]

	if got.Title != "Impair Defenses" || factValue(got, "Replaced by") != "T1685 Disable or Modify Tools" || len(got.Related) == 0 {
		t.Fatalf("result %+v", got)
	}
}

func TestTheCyberToolAnswersEachItemOfABatchInOrder(t *testing.T) {
	lookup := lookupCyber(t, mcpCyberPlugin(t), mcpCVE, "hello", "cve-2025-55182", mcpAddress, "CVE-12")

	var inputs []string
	for _, result := range lookup.Results {
		inputs = append(inputs, fmt.Sprintf("%s:%v", result.Input, result.Recognized))
	}
	want := mcpCVE + ":true|hello:false|" + mcpAddress + ":true|CVE-12:false"
	if strings.Join(inputs, "|") != want {
		t.Fatalf("results %s, want %s with the repeated CVE answered once", strings.Join(inputs, "|"), want)
	}
	if lookup.Results[1].Reason != notAnIndicator {
		t.Errorf("reason %q", lookup.Results[1].Reason)
	}
}

func TestTheCyberToolRefusesAnEmptyOrOversizedBatch(t *testing.T) {
	p := mcpCyberPlugin(t)
	session := mcpSession(t, p, map[string]string{"Mattermost-Plugin-ID": agentsPluginID})

	for _, indicators := range [][]string{{}, make([]string, maxCyberToolIndicators+1)} {
		result := callMCPTool(t, session, "lookup_cyber_indicator", LookupCyberIndicatorArgs{Indicators: indicators})
		if !result.IsError || !strings.Contains(mcpResultText(result), "TF-20016") {
			t.Errorf("%d indicators: %+v", len(indicators), result)
		}
	}
}

func TestTheCyberToolNamesTheDatasetsThatAreMissing(t *testing.T) {
	lookup := lookupCyber(t, mcpCyberPlugin(t), mcpCVE)

	missing := strings.Join(lookup.DatasetsMissing, "|")
	if !strings.Contains(missing, cyber.DatasetLabel(intel.NameWatchlist)) || strings.Contains(missing, cyber.DatasetLabel(intel.NameKEV)) {
		t.Fatalf("missing %q", missing)
	}
}

func TestTheCyberToolAnswersWithTheCyberDecoratorOff(t *testing.T) {
	p := mcpCyberPlugin(t)
	config := p.getConfiguration().Clone()
	config.EnableCyber = false
	p.setConfiguration(config)

	got := lookupCyber(t, p, mcpCVE).Results[0]

	if !got.Exploited || got.Link != "" {
		t.Fatalf("result %+v, want the answer without a link, since links follow the switches", got)
	}
}

func TestTheCyberToolCarriesACreditThroughToItsResult(t *testing.T) {
	got := cyberIndicatorResult("8.8.8.8", cyber.Details{Kind: cyber.KindIP, Value: "8.8.8.8", Credits: []cyber.Credit{{Text: "IP Geolocation by DB-IP", URL: "https://db-ip.com"}}})

	if len(got.Credits) != 1 || got.Credits[0].URL != "https://db-ip.com" {
		t.Fatalf("credits %+v", got.Credits)
	}
}
