package intel

import (
	"errors"
	"reflect"
	"strings"
	"testing"
)

func TestTheReaderParsesTheGoldenFileTheGeneratorWrites(t *testing.T) {
	set := openIn(t, "testdata")

	got, err := set.CVEDetail("CVE-2026-0006")
	if err != nil {
		t.Fatal(err)
	}

	want := CVEDetail{
		ID: "CVE-2026-0006",
		Weaknesses: []WeaknessSource{
			{Source: "security@example.com", CWE: []string{"CWE-20", "CWE-502"}},
			{Source: "nvd@nist.gov", CWE: []string{"CWE-917"}},
		},
		Configurations: []Configuration{
			{
				Vulnerable: []CPEMatch{{CPE: "o:example:router_firmware", Before: "2.7.0"}},
				On:         []CPEMatch{{CPE: "h:example:router:-"}},
			},
			{
				Vulnerable: []CPEMatch{
					{CPE: `a:example:log\:lib`, From: "2.0", Before: "2.3.1"},
					{CPE: `a:example:log\:lib:2.13.0:beta1`},
				},
			},
		},
		Affected: []AffectedProduct{{
			Vendor:        "Example Corp",
			Product:       "Log Lib",
			DefaultStatus: "unaffected",
			Versions: []AffectedVersion{
				{Version: "2.0", Status: "affected", LessThan: "2.3.1", Changes: []VersionChange{
					{At: "2.1", Status: "unaffected"},
					{At: "2.2", Status: "affected"},
				}},
				{Version: "2.15.0", Status: "unaffected"},
			},
		}},
		References: []Reference{
			{URL: "https://example.com/advisory?id=1&lang=en", Tags: []string{"Vendor Advisory", "Patch"}},
			{URL: "http://example.org/exploit", Tags: []string{"Exploit", "Third Party Advisory"}},
			{URL: "https://example.net/untagged"},
		},
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("the golden row read back as\n%+v\nwant\n%+v", got, want)
	}
}

func TestADetailRowWithNothingButAnIDReadsAsEmpty(t *testing.T) {
	set := openIn(t, "testdata")

	got, err := set.CVEDetail("CVE-2026-0001")
	if err != nil {
		t.Fatal(err)
	}
	if got.Weaknesses != nil || got.Configurations != nil || got.Affected != nil || got.References != nil {
		t.Fatalf("an ID-only row read back with detail: %+v", got)
	}
}

func TestAMalformedDetailFieldIsAReadFailureNotAMissingRow(t *testing.T) {
	dir := t.TempDir()
	writeDataset(t, dir, NameCVEDetail, strings.Join([]string{"CVE-2026-0001", "", "not json", "", ""}, "\t"))
	set := openIn(t, dir)

	_, err := set.CVEDetail("CVE-2026-0001")
	if err == nil || errors.Is(err, ErrNotFound) || errors.Is(err, ErrNoDataset) {
		t.Fatalf("a malformed field gave %v, want a read failure", err)
	}

	if _, err := set.CVEDetail("CVE-2026-9999"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("an absent row gave %v, want ErrNotFound", err)
	}
}

func TestTheDetailDatasetIsReportedAmongTheOthers(t *testing.T) {
	set := openIn(t, "testdata")

	for _, status := range set.Statuses() {
		if status.Name == NameCVEDetail {
			if !status.Present {
				t.Fatalf("the detail dataset in testdata was not reported installed")
			}
			return
		}
	}
	t.Fatalf("the detail dataset is missing from Statuses")
}
