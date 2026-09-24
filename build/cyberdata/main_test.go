package main

import (
	"flag"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
)

const nvdFixture = `{"vulnerabilities":[
{"cve":{"id":"CVE-2026-0001","published":"2026-09-20T10:00:00.000","lastModified":"2026-09-21T10:00:00.000",
 "descriptions":[{"lang":"en","value":"Only a 3.1 score."}],
 "metrics":{"cvssMetricV31":[{"cvssData":{"baseScore":9.8,"baseSeverity":"CRITICAL","vectorString":"CVSS:3.1/AV:N"}}]}}},
{"cve":{"id":"CVE-2026-0002","published":"2026-09-20T10:00:00.000","lastModified":"2026-09-21T10:00:00.000",
 "descriptions":[{"lang":"en","value":"Only a 4.0 score."}],
 "metrics":{"cvssMetricV40":[{"cvssData":{"baseScore":6.9,"baseSeverity":"MEDIUM","vectorString":"CVSS:4.0/AV:N"}}]}}},
{"cve":{"id":"CVE-2026-0003","published":"2026-09-20T10:00:00.000","lastModified":"2026-09-21T10:00:00.000",
 "descriptions":[{"lang":"en","value":"Both scores."}],
 "metrics":{"cvssMetricV40":[{"cvssData":{"baseScore":2.0,"baseSeverity":"LOW","vectorString":"CVSS:4.0/AV:L"}}],
            "cvssMetricV31":[{"cvssData":{"baseScore":7.5,"baseSeverity":"HIGH","vectorString":"CVSS:3.1/AV:N"}}]}}},
{"cve":{"id":"CVE-2026-0004","published":"2026-09-20T10:00:00.000","lastModified":"2026-09-21T10:00:00.000",
 "descriptions":[{"lang":"en","value":"Not yet analyzed."}],"metrics":{}}},
{"cve":{"id":"CVE-2026-0005","published":"2026-09-20T10:00:00.000","lastModified":"2026-09-21T10:00:00.000",
 "descriptions":[{"lang":"es","value":"Texto en otro idioma."},
  {"lang":"en","value":"Buffer overflow in Example, Inc. HyperTerminal before 2.0.\nAn attacker who can\tcontrol input can execute arbitrary code. ` + longTail + `"}],"metrics":{}}},
{"cve":{"id":"CVE-2026-0006","published":"2026-09-20T10:00:00.000","lastModified":"2026-09-21T10:00:00.000",
 "descriptions":[{"lang":"en","value":"Every detail block."}],"metrics":{},
 "weaknesses":[
  {"source":"security@example.com","type":"Primary","description":[{"lang":"en","value":"CWE-20"},{"lang":"en","value":"CWE-502"}]},
  {"source":"nvd@nist.gov","type":"Secondary","description":[{"lang":"en","value":"NVD-CWE-Other"},{"lang":"en","value":"CWE-917"}]}],
 "configurations":[
  {"operator":"AND","nodes":[
   {"operator":"OR","negate":false,"cpeMatch":[{"vulnerable":true,"criteria":"cpe:2.3:o:example:router_firmware:*:*:*:*:*:*:*:*","versionEndExcluding":"2.7.0","matchCriteriaId":"A"}]},
   {"operator":"OR","negate":false,"cpeMatch":[{"vulnerable":false,"criteria":"cpe:2.3:h:example:router:-:*:*:*:*:*:*:*","matchCriteriaId":"B"}]}]},
  {"nodes":[{"operator":"OR","negate":false,"cpeMatch":[
   {"vulnerable":true,"criteria":"cpe:2.3:a:example:log\\:lib:*:*:*:*:*:*:*:*","versionStartIncluding":"2.0","versionEndExcluding":"2.3.1","matchCriteriaId":"C"},
   {"vulnerable":true,"criteria":"cpe:2.3:a:example:log\\:lib:2.13.0:beta1:*:*:*:*:*:*","matchCriteriaId":"D"}]}]},
  {"nodes":[{"operator":"OR","negate":false,"cpeMatch":[{"vulnerable":false,"criteria":"cpe:2.3:o:example:os:-:*:*:*:*:*:*:*","matchCriteriaId":"E"}]}]}],
 "affected":[{"source":"security@example.com","affectedData":[
  {"vendor":"Example Corp","product":"Log Lib","defaultStatus":"unaffected","versions":[
   {"version":"2.0","lessThan":"2.3.1","versionType":"semver","status":"affected","changes":[{"at":"2.1","status":"unaffected"},{"at":"2.2","status":"affected"}]},
   {"version":"n/a","status":"affected"},
   {"version":"2.15.0","status":"unaffected"}]},
  {"vendor":"n/a","product":"n/a","versions":[{"version":"n/a","status":"affected"}]}]}],
 "references":[
  {"url":"https://example.com/advisory?id=1&lang=en","source":"security@example.com","tags":["Vendor Advisory","Patch"]},
  {"url":"http://example.org/exploit","source":"nvd@nist.gov","tags":["Exploit"]},
  {"url":"https://example.net/untagged","source":"nvd@nist.gov"},
  {"url":"  ","source":"nvd@nist.gov"},
  {"url":"http://example.org/exploit","source":"security@example.com","tags":["Exploit","Third Party Advisory"]}]}},
{"cve":{"id":"CVE-2026-0007","published":"2026-09-20T10:00:00.000","lastModified":"2026-09-21T10:00:00.000",
 "descriptions":[{"lang":"en","value":"Several products in one configuration."}],"metrics":{},
 "configurations":[{"nodes":[{"operator":"OR","negate":false,"cpeMatch":[
  {"vulnerable":true,"criteria":"cpe:2.3:a:cisco:unified_communications_manager:*:*:*:*:*:*:*:*","versionEndExcluding":"11.5\\(1\\)","matchCriteriaId":"F"},
  {"vulnerable":true,"criteria":"cpe:2.3:a:cisco:unified_communications_manager:*:*:*:*:session_management:*:*:*","versionEndExcluding":"11.5\\(1\\)","matchCriteriaId":"G"},
  {"vulnerable":true,"criteria":"cpe:2.3:a:cisco:unified_communications_manager:11.5\\(1\\):*:*:*:*:*:*:*","matchCriteriaId":"H"},
  {"vulnerable":true,"criteria":"cpe:2.3:a:cisco:dna_spaces\\:_connector:*:*:*:*:*:*:*:*","versionEndExcluding":"2.5","matchCriteriaId":"I"}]}]}]}}
]}`

const longTail = "Fixed in 2.0, which removes the feature entirely and is the only supported release for this product line going forward. Earlier releases remain vulnerable when the optional remote console is enabled, which it is by default on the server edition, and no configuration change mitigates it."

const fullDescription = "Buffer overflow in Example, Inc. HyperTerminal before 2.0. An attacker who can control input can execute arbitrary code. " + longTail

func writeNVDFixture(t *testing.T) string {
	t.Helper()

	dir := filepath.Join(t.TempDir(), "nvd")
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "page-0000.json"), []byte(nvdFixture), 0o600); err != nil {
		t.Fatal(err)
	}

	return dir
}

func scoresByID(t *testing.T, rows [][]string) map[string][]string {
	t.Helper()

	byID := map[string][]string{}
	for _, row := range rows {
		byID[row[0]] = []string{row[3], row[4], row[5]}
	}

	return byID
}

func TestACVEScoredOnlyUnderCVSS4StillCarriesItsScore(t *testing.T) {
	rows, err := buildCVE(writeNVDFixture(t))
	if err != nil {
		t.Fatal(err)
	}

	got := scoresByID(t, rows)["CVE-2026-0002"]
	if got[0] != "6.9" || got[1] != "Medium" || !strings.HasPrefix(got[2], "CVSS:4.0/") {
		t.Fatalf("a 4.0-only record read as %q", got)
	}
}

func TestCVSS31IsPreferredWhenARecordCarriesBoth(t *testing.T) {
	rows, err := buildCVE(writeNVDFixture(t))
	if err != nil {
		t.Fatal(err)
	}

	got := scoresByID(t, rows)["CVE-2026-0003"]
	if got[0] != "7.5" || !strings.HasPrefix(got[2], "CVSS:3.1/") {
		t.Fatalf("a record with 3.1 and 4.0 read as %q, want the 3.1 score it read before 4.0 was parsed", got)
	}
}

func TestAnUnscoredCVEIsKeptWithEmptyScoreFields(t *testing.T) {
	rows, err := buildCVE(writeNVDFixture(t))
	if err != nil {
		t.Fatal(err)
	}

	got, ok := scoresByID(t, rows)["CVE-2026-0004"]
	if !ok {
		t.Fatal("a record NVD has not analyzed yet was dropped")
	}
	if got[0] != "" || got[1] != "" || got[2] != "" {
		t.Fatalf("an unscored record read as %q", got)
	}
}

func TestTheLabelReplacesTheSourceInTheStamp(t *testing.T) {
	nvd := writeNVDFixture(t)
	out := t.TempDir()

	savedSource, savedOut, savedLabel := *sourceDir, *outDir, *label
	t.Cleanup(func() { *sourceDir, *outDir, *label = savedSource, savedOut, savedLabel })

	*sourceDir = filepath.Dir(nvd)
	*outDir = out
	*label = "NVD API, published 2026-09-16T00:00:00Z to 2026-09-23T00:00:00Z, 4 CVEs"

	err := run(builder{
		name:   "cve",
		source: "nvd",
		build:  buildCVE,
		target: func() string { return filepath.Join(out, "cve.tsv") },
		stamp:  true,
	})
	if err != nil {
		t.Fatal(err)
	}

	raw, err := os.ReadFile(filepath.Join(out, "cve.tsv"))
	if err != nil {
		t.Fatal(err)
	}

	stamp := strings.SplitN(string(raw), "\n", 2)[0]
	fields := strings.Split(stamp, "\t")
	if len(fields) != 4 {
		t.Fatalf("the stamp has %d fields, want the 4 the reader requires: %q", len(fields), stamp)
	}
	if fields[3] != *label {
		t.Fatalf("the stamp names %q as its source, want the label", fields[3])
	}
}

func TestACVEDescriptionIsKeptWholeInEnglish(t *testing.T) {
	rows, err := buildCVE(writeNVDFixture(t))
	if err != nil {
		t.Fatal(err)
	}

	for _, row := range rows {
		if row[0] != "CVE-2026-0005" {
			continue
		}
		if len([]rune(fullDescription)) <= 300 {
			t.Fatalf("the fixture is %d runes, too short to prove the old 300 cap is gone", len([]rune(fullDescription)))
		}
		if row[7] != fullDescription {
			t.Fatalf("the description was not kept whole\n got: %q\nwant: %q", row[7], fullDescription)
		}
		return
	}
	t.Fatal("CVE-2026-0005 was not built")
}

var updateGolden = flag.Bool("update", false, "rewrite the detail golden files the reader's tests parse")

const goldenDir = "../../server/decorators/cyber/intel/testdata/"

func matchGolden(t *testing.T, name string, rows [][]string) {
	t.Helper()

	if invalid := checkRows(rows); invalid != nil {
		t.Fatal(invalid)
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i][0] < rows[j][0] })

	var body strings.Builder
	body.WriteString("#tactical-fusion-cyber/1\t" + name + "\t2026-09-01T00:00:00Z\tgolden\n")
	for _, row := range rows {
		body.WriteString(strings.Join(row, "\t") + "\n")
	}

	path := goldenDir + name + ".tsv"
	if *updateGolden {
		if written := os.WriteFile(path, []byte(body.String()), 0o600); written != nil {
			t.Fatal(written)
		}
	}

	golden, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("%v; run go test ./build/cyberdata -run Golden -update", err)
	}
	if string(golden) != body.String() {
		t.Fatalf("the generator's %s rows no longer match %s, which the reader's tests parse; if the change is intended, rerun with -update and fix the reader to match\n got: %s\nwant: %s", name, path, body.String(), golden)
	}
}

func TestTheDetailRowsMatchTheGoldenFileTheReaderParses(t *testing.T) {
	rows, err := buildCVEDetail(writeNVDFixture(t))
	if err != nil {
		t.Fatal(err)
	}
	matchGolden(t, "cvedetail", rows)
}

const cweFixture = `CWE-ID,Name,Weakness Abstraction,Status,Description,Extended Description,Related Weaknesses,Common Consequences,Potential Mitigations,Detection Methods,Observed Examples
9001,"Invented Weakness One",Base,Draft,"The product mishandles an invented input. A second sentence follows.","Background about the invented weakness, with a colon: here.","::NATURE:ChildOf:CWE ID:9002:VIEW ID:1000:ORDINAL:Primary::","::SCOPE:Confidentiality:SCOPE:Integrity:IMPACT:Read Application Data:IMPACT:Modify Memory:LIKELIHOOD:High:NOTE:An invented note.::SCOPE:Availability:IMPACT:DoS: Crash, Exit, or Restart::","::PHASE:Implementation:STRATEGY:Input Validation:DESCRIPTION:Hold every value in a smart pointer class such as std::auto_ptr before use.:EFFECTIVENESS:High::PHASE:Architecture and Design:DESCRIPTION:Pick a design that avoids it.::","::METHOD:Automated Static Analysis:DESCRIPTION:Run an invented analyzer over the code.:EFFECTIVENESS:Moderate::","::REFERENCE:CVE-2099-0001:DESCRIPTION:Invented product mishandles the invented input.:LINK:https://www.cve.org/CVERecord?id=CVE-2099-0001::REFERENCE:[REF-1]:DESCRIPTION:An invented citation.:LINK:https://example.org/ref::"
9002,"Invented Class",Class,Stable,"The product has an invented class of problem.",,,,,,
`

func writeCWEFixture(t *testing.T) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "cwe-1000.csv")
	if err := os.WriteFile(path, []byte(cweFixture), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestTheCWEDetailRowsMatchTheGoldenFileTheReaderParses(t *testing.T) {
	rows, err := buildCWEDetail(writeCWEFixture(t))
	if err != nil {
		t.Fatal(err)
	}
	matchGolden(t, "cwedetail", rows)
}

func TestAPackedFieldSplitsOnlyWhereAKnownKeyFollows(t *testing.T) {
	got := mitigationsOf("::PHASE:Implementation:DESCRIPTION:Use std::auto_ptr and std::unique_ptr, see http://x.example/a:b.::PHASE:Testing:DESCRIPTION:Test it.::")

	want := []cweMitigation{
		{Phase: "Implementation", Description: "Use std::auto_ptr and std::unique_ptr, see http://x.example/a:b."},
		{Phase: "Testing", Description: "Test it."},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v\nwant %+v", got, want)
	}
}

func TestAConsequenceKeepsEveryScopeAndImpactItNames(t *testing.T) {
	got := consequencesOf("::SCOPE:Confidentiality:SCOPE:Integrity:IMPACT:Read Application Data:IMPACT:DoS: Crash, Exit, or Restart:LIKELIHOOD:High::")

	want := []cweConsequence{{
		Scopes:     []string{"Confidentiality", "Integrity"},
		Impacts:    []string{"Read Application Data", "DoS: Crash, Exit, or Restart"},
		Likelihood: "High",
	}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v\nwant %+v", got, want)
	}
}

func TestTheCWECatalogAndItsDetailReadTheSameRows(t *testing.T) {
	source := writeCWEFixture(t)

	catalog, err := buildCWE(source)
	if err != nil {
		t.Fatal(err)
	}
	detail, err := buildCWEDetail(source)
	if err != nil {
		t.Fatal(err)
	}

	if len(catalog) != len(detail) {
		t.Fatalf("catalog %d rows, detail %d", len(catalog), len(detail))
	}
	for i := range catalog {
		if catalog[i][0] != detail[i][0] {
			t.Errorf("row %d: catalog %s, detail %s", i, catalog[i][0], detail[i][0])
		}
	}
	if catalog[0][4] != "The product mishandles an invented input." || detail[0][1] != "The product mishandles an invented input. A second sentence follows." {
		t.Fatalf("the catalog keeps the first sentence and the detail the whole description: %q / %q", catalog[0][4], detail[0][1])
	}
}

func TestParentsOfNamesEachParentOnceAcrossViews(t *testing.T) {
	related := "::NATURE:ChildOf:CWE ID:74:VIEW ID:1000:ORDINAL:Primary::NATURE:ChildOf:CWE ID:74:VIEW ID:1003:ORDINAL:Primary::NATURE:ChildOf:CWE ID:20:VIEW ID:1000::NATURE:PeerOf:CWE ID:352:VIEW ID:1000::"

	if got := parentsOf(related); got != "CWE-74,CWE-20" {
		t.Fatalf("parentsOf = %q, want each ChildOf parent once, in order", got)
	}
}

func TestAnAttackSummaryDropsCitationsAndMarkup(t *testing.T) {
	cases := map[string]string{
		"Adversaries may obfuscate traffic.(Citation: Invented Report 2026) More text.": "Adversaries may obfuscate traffic.",
		"Adversaries may read `/proc` and <code>/sys</code>. More.":                     "Adversaries may read /proc and /sys.",
		"Adversaries may use [Invented Tool](https://attack.example/S9999). More.":      "Adversaries may use Invented Tool.",
		"Adversaries may “pass the hash” – the malware’s way. More.":                    "Adversaries may \"pass the hash\" - the malware's way.",
		"Adversaries may pivot\u2014quietly\u2014onward. More.":                         "Adversaries may pivot - quietly - onward.",
	}

	for description, want := range cases {
		if got := attackSummary(description); got != want {
			t.Errorf("attackSummary(%q) = %q, want %q", description, got, want)
		}
	}
}

const stixFixture = `{"objects":[
{"id":"x-mitre-tactic--1","type":"x-mitre-tactic","name":"Invented Tactic B","description":"B.","x_mitre_shortname":"b","external_references":[{"source_name":"mitre-attack","external_id":"TA9002"}]},
{"id":"x-mitre-tactic--2","type":"x-mitre-tactic","name":"Invented Tactic A","description":"A.","x_mitre_shortname":"a","external_references":[{"source_name":"mitre-attack","external_id":"TA9001"}]},
{"id":"attack-pattern--old","type":"attack-pattern","name":"Old Technique","description":"Old.","revoked":true,"external_references":[{"source_name":"mitre-attack","external_id":"T9001"}]},
{"id":"attack-pattern--new","type":"attack-pattern","name":"New Technique","description":"New.","kill_chain_phases":[{"kill_chain_name":"mitre-attack","phase_name":"b"},{"kill_chain_name":"mitre-attack","phase_name":"a"}],"external_references":[{"source_name":"mitre-attack","external_id":"T9002"}]},
{"id":"attack-pattern--gone","type":"attack-pattern","name":"Gone","description":"Gone.","x_mitre_deprecated":true,"external_references":[{"source_name":"mitre-attack","external_id":"T9003"}]},
{"id":"attack-pattern--orphan","type":"attack-pattern","name":"Orphan","description":"Orphan.","x_mitre_is_subtechnique":true,"revoked":true,"external_references":[{"source_name":"mitre-attack","external_id":"T9999.001"}]},
{"id":"relationship--1","type":"relationship","relationship_type":"revoked-by","source_ref":"attack-pattern--old","target_ref":"attack-pattern--new"}
]}`

func TestTheAttackCatalogKeepsRetiredEntriesWithTheirReplacement(t *testing.T) {
	path := filepath.Join(t.TempDir(), "enterprise-attack.json")
	if err := os.WriteFile(path, []byte(stixFixture), 0o600); err != nil {
		t.Fatal(err)
	}

	rows, err := buildAttack(path)
	if err != nil {
		t.Fatal(err)
	}

	byID := map[string][]string{}
	for _, row := range rows {
		if len(row) != len(headers["attack"]) {
			t.Fatalf("%s has %d fields, want %d", row[0], len(row), len(headers["attack"]))
		}
		byID[row[0]] = row
	}

	if got := byID["T9001"]; got[7] != "revoked" || got[8] != "T9002" {
		t.Errorf("revoked row %q", got)
	}
	if got := byID["T9003"]; got[7] != "deprecated" || got[8] != "" {
		t.Errorf("deprecated row %q", got)
	}
	if got := byID["T9002"]; got[3] != "TA9001,TA9002" || got[7] != "active" {
		t.Errorf("active row %q, want its tactics sorted by id", got)
	}
	if got := byID["T9999.001"]; got[4] != "" {
		t.Errorf("a sub-technique whose parent the catalog lacks kept the dangling parent %q", got[4])
	}
}

const stixDetailFixture = `{"objects":[
{"id":"x-mitre-tactic--1","type":"x-mitre-tactic","name":"Invented Tactic","description":"The adversary is inventing.","x_mitre_shortname":"a","external_references":[{"source_name":"mitre-attack","external_id":"TA9001","url":"https://attack.example/tactics/TA9001"}]},
{"id":"attack-pattern--1","type":"attack-pattern","name":"Invented Technique","description":"Adversaries may invent things.(Citation: Invented Report) See [Invented Tool](https://attack.example/software/S9001).","kill_chain_phases":[{"kill_chain_name":"mitre-attack","phase_name":"a"}],"external_references":[{"source_name":"mitre-attack","external_id":"T9001","url":"https://attack.example/techniques/T9001"},{"source_name":"Invented Report","url":"https://example.org/report","description":"Invented, A. (2026). A report. Retrieved September 1, 2026."},{"source_name":"Alias Only","description":"(Citation: Invented Report)"}]},
{"id":"course-of-action--1","type":"course-of-action","name":"Invented Mitigation","external_references":[{"source_name":"mitre-attack","external_id":"M9001","url":"https://attack.example/mitigations/M9001"}]},
{"id":"x-mitre-analytic--1","type":"x-mitre-analytic","name":"Analytic 9001","description":"Watch for invented events.","x_mitre_platforms":["Linux"],"x_mitre_log_source_references":[{"name":"auditd:SYSCALL","channel":"execve"}],"x_mitre_mutable_elements":[{"field":"Threshold","description":"How many."}],"external_references":[{"source_name":"mitre-attack","external_id":"AN9001"}]},
{"id":"x-mitre-detection-strategy--1","type":"x-mitre-detection-strategy","name":"Invented Detection","x_mitre_analytic_refs":["x-mitre-analytic--1"],"external_references":[{"source_name":"mitre-attack","external_id":"DET9001","url":"https://attack.example/detectionstrategies/DET9001"}]},
{"id":"intrusion-set--1","type":"intrusion-set","name":"Invented Group","external_references":[{"source_name":"mitre-attack","external_id":"G9001","url":"https://attack.example/groups/G9001"}]},
{"id":"intrusion-set--gone","type":"intrusion-set","name":"Retired Group","revoked":true,"external_references":[{"source_name":"mitre-attack","external_id":"G9002"}]},
{"id":"relationship--1","type":"relationship","relationship_type":"mitigates","source_ref":"course-of-action--1","target_ref":"attack-pattern--1","description":"Turn the invention off.(Citation: Invented Report)"},
{"id":"relationship--2","type":"relationship","relationship_type":"detects","source_ref":"x-mitre-detection-strategy--1","target_ref":"attack-pattern--1"},
{"id":"relationship--3","type":"relationship","relationship_type":"uses","source_ref":"intrusion-set--1","target_ref":"attack-pattern--1","description":"[Invented Group](https://attack.example/groups/G9001) has invented things."},
{"id":"relationship--4","type":"relationship","relationship_type":"uses","source_ref":"intrusion-set--gone","target_ref":"attack-pattern--1","description":"A retired group's procedure."}
]}`

func TestTheAttackDetailRowsMatchTheGoldenFileTheReaderParses(t *testing.T) {
	path := filepath.Join(t.TempDir(), "enterprise-attack.json")
	if err := os.WriteFile(path, []byte(stixDetailFixture), 0o600); err != nil {
		t.Fatal(err)
	}

	rows, err := buildAttackDetail(path)
	if err != nil {
		t.Fatal(err)
	}
	matchGolden(t, "attackdetail", rows)
}

func TestAnAttackNameCarriesNoTypographicDash(t *testing.T) {
	if got := attackName("Drive-by Compromise \u2014 Behavior-based \u2013 multi-signal"); got != "Drive-by Compromise - Behavior-based - multi-signal" {
		t.Fatalf("attackName = %q", got)
	}
}

func TestAnEPSSScoreCarriesTheDateFIRSTScoredIt(t *testing.T) {
	path := filepath.Join(t.TempDir(), "epss.csv")
	body := "#model_version:v2026.06.15,score_date:2026-09-20T12:00:20Z\ncve,epss,percentile\nCVE-2099-0001,0.5,0.9\n"
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}

	rows, err := buildEPSS(path)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"CVE-2099-0001", "0.5", "0.9", "2026-09-20"}; !reflect.DeepEqual(rows[0], want) {
		t.Fatalf("row %q, want %q", rows[0], want)
	}
}

func TestAnEPSSExportWithNoScoreDateIsRefused(t *testing.T) {
	for _, header := range []string{"cve,epss,percentile", "#model_version:v2026.06.15", "#score_date:yesterday"} {
		if _, err := epssScoreDate(header); err == nil {
			t.Errorf("accepted %q", header)
		}
	}
}

func TestAnEPSSValueInExponentFormIsWrittenAsAPlainDecimal(t *testing.T) {
	cases := map[string]string{"4e-05": "0.00004", "1e-05": "0.00001", "0.03351": "0.03351", "1.0": "1.0", "9.9E-4": "0.00099"}
	for value, want := range cases {
		if got := plainDecimal(value); got != want {
			t.Errorf("plainDecimal(%q) = %q, want %q", value, got, want)
		}
	}
}
