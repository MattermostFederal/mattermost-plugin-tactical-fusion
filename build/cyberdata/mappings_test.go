package main

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func TestCAPECRelatedWeaknessesBecomeCWEIds(t *testing.T) {
	got := capecWeaknesses("::276::285::434::")
	if want := []string{"CWE-276", "CWE-285", "CWE-434"}; !slices.Equal(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestCAPECReadsOnlyItsATTACKTaxonomyEntries(t *testing.T) {
	field := "TAXONOMY NAME:ATTACK:ENTRY ID:1574.010:ENTRY NAME:Hijack Execution Flow: ServicesFile Permissions Weakness::" +
		"::TAXONOMY NAME:WASC:ENTRY ID:07:ENTRY NAME:Buffer Overflow::"
	if got := capecTechniques(field); !slices.Equal(got, []string{"T1574.010"}) {
		t.Errorf("got %v", got)
	}
}

func TestCAPECLeavesRetiredPatternsOut(t *testing.T) {
	dir := t.TempDir()
	csv := "'ID,Name,Abstraction,Status,Description,Likelihood Of Attack,Typical Severity,Related Weaknesses,Taxonomy Mappings\n" +
		"63,Cross-Site Scripting (XSS),Standard,Stable,Scripts run. More text.,High,Very High,::79::,\n" +
		"999,Old,Standard,Deprecated,Gone.,,,::79::,\n"
	path := filepath.Join(dir, "capec.csv")
	if err := os.WriteFile(path, []byte(csv), 0o600); err != nil {
		t.Fatal(err)
	}

	rows, err := buildCAPEC(path)
	if err != nil {
		t.Fatal(err)
	}
	want := `[{"id":"CAPEC-63","name":"Cross-Site Scripting (XSS)","abstraction":"Standard","severity":"Very High","likelihood":"High","summary":"Scripts run."}]`
	if len(rows) != 1 || rows[0][0] != "CWE-79" || rows[0][1] != want {
		t.Errorf("rows are %q", rows)
	}
}

func TestTheCAPECRowsMatchTheGoldenFileTheReaderParses(t *testing.T) {
	dir := t.TempDir()
	csv := "'ID,Name,Abstraction,Status,Description,Likelihood Of Attack,Typical Severity,Related Weaknesses,Taxonomy Mappings\n" +
		"63,Cross-Site Scripting (XSS),Standard,Stable,An adversary embeds scripts. More.,High,Very High,::79::,\n" +
		"9001,Invented Pattern,Detailed,Draft,An invented pattern.,Low,Medium,::79::20::,TAXONOMY NAME:ATTACK:ENTRY ID:1059.001:ENTRY NAME:PowerShell::\n"
	path := filepath.Join(dir, "capec.csv")
	if err := os.WriteFile(path, []byte(csv), 0o600); err != nil {
		t.Fatal(err)
	}
	rows, err := buildCAPEC(path)
	if err != nil {
		t.Fatal(err)
	}
	matchGolden(t, "capec", rows)
}

func TestACVEMappedTwiceToOneTechniqueIsOneEntryWithBothTypes(t *testing.T) {
	dir := t.TempDir()
	enterprise := `{"mapping_objects":[
		{"capability_id":"CVE-2022-41082","mapping_type":"primary_impact","attack_object_id":"T1059.001"},
		{"capability_id":"CVE-2022-41082","mapping_type":"exploitation_technique","attack_object_id":"T1059.001"},
		{"capability_id":"CVE-2021-44228","mapping_type":"exploitation_technique","attack_object_id":"T1059.001"}]}`
	for name, body := range map[string]string{"kev-attack-enterprise.json": enterprise, "kev-attack-mobile.json": `{"mapping_objects":[]}`} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	rows, err := buildCVEAttack(filepath.Join(dir, "kev-attack-enterprise.json"))
	if err != nil {
		t.Fatal(err)
	}
	matchGolden(t, "cveattack", rows)
	for _, row := range rows {
		if row[0] == "T1059.001" {
			want := `[{"id":"CVE-2022-41082","types":["exploitation technique","primary impact"]},{"id":"CVE-2021-44228","types":["exploitation technique"]}]`
			if row[1] != want {
				t.Errorf("T1059.001 maps to %s, want %s", row[1], want)
			}
			return
		}
	}
	t.Fatalf("no row for T1059.001 in %q", rows)
}
