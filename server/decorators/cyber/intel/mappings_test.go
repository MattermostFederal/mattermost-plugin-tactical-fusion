package intel

import (
	"errors"
	"slices"
	"testing"
)

func TestAttackPatternsAreReadByWeaknessAndByTechnique(t *testing.T) {
	set := openIn(t, "testdata")

	byWeakness, err := set.AttackPatterns("CWE-79")
	if err != nil || len(byWeakness) != 2 || byWeakness[0].ID != "CAPEC-63" || byWeakness[0].Severity != "Very High" {
		t.Fatalf("CWE-79 reads as %+v, %v", byWeakness, err)
	}
	byTechnique, err := set.AttackPatterns("T1059.001")
	if err != nil || len(byTechnique) != 1 || byTechnique[0].Name != "Invented Pattern" {
		t.Fatalf("T1059.001 reads as %+v, %v", byTechnique, err)
	}
	if _, err := set.AttackPatterns("CWE-1"); !errors.Is(err, ErrNotFound) {
		t.Errorf("an unmapped weakness reads as %v", err)
	}
}

func TestATTACKMappingsAreReadInBothDirections(t *testing.T) {
	set := openIn(t, "testdata")

	techniques, err := set.ATTACKMappings("CVE-2022-41082")
	if err != nil || len(techniques) != 1 || techniques[0].ID != "T1059.001" ||
		!slices.Equal(techniques[0].Types, []string{"exploitation technique", "primary impact"}) {
		t.Fatalf("CVE-2022-41082 reads as %+v, %v", techniques, err)
	}
	cves, err := set.ATTACKMappings("T1059.001")
	if err != nil || len(cves) != 2 || cves[0].ID != "CVE-2022-41082" {
		t.Fatalf("T1059.001 reads as %+v, %v", cves, err)
	}
}
