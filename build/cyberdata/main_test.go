package main

import (
	"os"
	"path/filepath"
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
 "descriptions":[{"lang":"es","value":"Descripcion en otro idioma."},
  {"lang":"en","value":"Buffer overflow in Example, Inc. HyperTerminal before 2.0.\nAn attacker who can\tcontrol input can execute arbitrary code. ` + longTail + `"}],"metrics":{}}}
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
