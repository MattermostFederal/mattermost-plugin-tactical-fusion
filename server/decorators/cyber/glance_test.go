package cyber

import (
	"reflect"
	"strings"
	"testing"

	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/decorators/cyber/intel"
)

func TestAWeaknessGlanceNamesItsKindAndWhatItAffects(t *testing.T) {
	set := datasets(t, map[string][]string{intel.NameCWEDetail: {cwe79Detail}})

	g := Describe(KindCWE, "CWE-79", set).Glance

	if g.Subtitle != "CWE-79 · Base · Stable" {
		t.Errorf("subtitle %q", g.Subtitle)
	}
	weakness, _ := LookupWeakness("CWE-79")
	if g.Summary != weakness.Summary {
		t.Errorf("summary %q, want the catalog's first sentence rather than the whole description", g.Summary)
	}
	if !reflect.DeepEqual(g.Tags, []string{"Confidentiality", "Integrity"}) {
		t.Errorf("tags %v", g.Tags)
	}
	if !reflect.DeepEqual(g.Facts, []string{"2 mitigations", "2 observed examples"}) {
		t.Errorf("facts %v", g.Facts)
	}
}

func TestASubTechniqueGlanceNamesItsParentTacticsAndCounts(t *testing.T) {
	set := datasets(t, map[string][]string{intel.NameAttackDetail: {t1059Detail}})

	g := Describe(KindAttack, "T1059.001", set).Glance

	if g.Subtitle != "T1059.001 · Sub-technique of T1059 Command and Scripting Interpreter" {
		t.Errorf("subtitle %q", g.Subtitle)
	}
	if !reflect.DeepEqual(g.Tags, []string{"Execution"}) {
		t.Errorf("tags %v", g.Tags)
	}
	if !reflect.DeepEqual(g.Facts, []string{"Windows", "1 procedure example", "1 mitigation", "2 detection strategies"}) {
		t.Errorf("facts %v", g.Facts)
	}
	if g.Status != "" {
		t.Errorf("an active technique carries a status %q", g.Status)
	}
}

func TestARevokedTechniqueGlanceNamesItsReplacement(t *testing.T) {
	g := Describe(KindAttack, "T1562", nil).Glance

	if g.Status != "Revoked by MITRE, replaced by T1685 Disable or Modify Tools" {
		t.Fatalf("status %q", g.Status)
	}
}

func TestATacticGlanceCountsItsTechniques(t *testing.T) {
	g := Describe(KindAttack, "TA0002", nil).Glance

	if g.Subtitle != "TA0002 · Tactic" {
		t.Errorf("subtitle %q", g.Subtitle)
	}
	if len(g.Facts) != 1 || !strings.HasSuffix(g.Facts[0], " techniques") {
		t.Errorf("facts %v", g.Facts)
	}
}

func TestAnAddressGlanceGivesItsScopeEvenWithNoDataset(t *testing.T) {
	g := Describe(KindIP, "10.0.0.1", nil).Glance

	if !reflect.DeepEqual(g.Tags, []string{"Private"}) || g.Subtitle != "" || len(g.Facts) != 0 {
		t.Fatalf("glance %+v", g)
	}
}

func TestAHashGlanceSaysWhetherTheWatchlistKnowsIt(t *testing.T) {
	digest := strings.Repeat("a", 64)

	if g := Describe(KindHash, digest, nil).Glance; g.Subtitle != "SHA-256 · 32 bytes" || !reflect.DeepEqual(g.Facts, []string{"No watchlist is installed"}) {
		t.Errorf("no watchlist: %+v", g)
	}

	set := datasets(t, map[string][]string{intel.NameWatchlist: {strings.Repeat("b", 64) + "\tmalicious\tinternal\t\t2026-09-01\t"}})
	if g := Describe(KindHash, digest, set).Glance; !reflect.DeepEqual(g.Facts, []string{"Not on the watchlist"}) {
		t.Errorf("absent from the watchlist: %+v", g)
	}
}

func TestCountedSaysOneInTheSingular(t *testing.T) {
	if counted(1, "mitigation", "mitigations") != "1 mitigation" || counted(3, "mitigation", "mitigations") != "3 mitigations" {
		t.Fatal("counted does not agree in number")
	}
}

func TestAPlaceNamesACityAndRegionOfTheSameNameOnce(t *testing.T) {
	cases := map[[3]string]string{
		{"Kaunas", "Kaunas", "LT"}:            "Kaunas, LT",
		{"Mountain View", "California", "US"}: "Mountain View, California, US",
		{"", "", "US"}:                        "US",
		{"Singapore", "singapore", "SG"}:      "Singapore, SG",
		{"", "", ""}:                          "",
	}
	for parts, want := range cases {
		if got := placeText(parts[0], parts[1], parts[2]); got != want {
			t.Errorf("placeText(%q) = %q, want %q", parts, got, want)
		}
	}
}

func TestAVulnerabilityGlanceGathersItsDatesExposureAndSignals(t *testing.T) {
	set := datasets(t, map[string][]string{
		intel.NameCVE: {strings.Join([]string{
			"CVE-2025-55182", "2025-12-03T16:15:56.463", "2025-12-10", "10.0", "Critical",
			"CVSS:3.1/AV:N/AC:L/PR:N/UI:N/S:C/C:H/I:H/A:H", "CWE-502", "Remote code execution in an invented framework.",
		}, "\t")},
		intel.NameEPSS: {"CVE-2025-55182\t0.99802\t0.99958\t2026-09-23"},
		intel.NameKEV:  {strings.Join([]string{"CVE-2025-55182", "2025-12-05", "2025-12-12", "Known", "Invented Framework", "Patch."}, "\t")},
		intel.NameCVEDetail: {"CVE-2025-55182\t\t\t" +
			`[{"vendor":"Invented","product":"Framework","versions":[{"version":"19.0.0","status":"affected"}]}]` + "\t" +
			`[{"url":"https://example.org/a","tags":["Patch"]},{"url":"https://example.org/b"}]`},
	})

	g := Describe(KindCVE, "CVE-2025-55182", set).Glance

	if g.Subtitle != "Published 2025-12-03 · CWE-502" {
		t.Errorf("subtitle %q", g.Subtitle)
	}
	if g.Summary != "Remote code execution in an invented framework." {
		t.Errorf("summary %q", g.Summary)
	}
	if !reflect.DeepEqual(g.Tags, []string{"Network", "No privileges", "No user interaction"}) {
		t.Errorf("tags %v", g.Tags)
	}
	want := []string{"EPSS 99.802%", "KEV due 2025-12-12", "Ransomware use known", "1 affected product", "2 references"}
	if !reflect.DeepEqual(g.Facts, want) {
		t.Errorf("facts %v, want %v", g.Facts, want)
	}
}

func TestACVSS2VectorGivesItsAuthenticationAsExposure(t *testing.T) {
	if got := exposureTags(DescribeVector("AV:L/AC:M/Au:S/C:P/I:N/A:N")); !reflect.DeepEqual(got, []string{"Local", "Single authentication"}) {
		t.Fatalf("tags %v", got)
	}
	if got := exposureTags(DescribeVector("CVSS:4.0/AV:A/AC:L/AT:N/PR:L/UI:A/VC:H/VI:N/VA:N/SC:N/SI:N/SA:N")); !reflect.DeepEqual(got, []string{"Adjacent network", "Low privileges", "Active user interaction"}) {
		t.Fatalf("tags %v", got)
	}
}

func TestAVulnerabilityNoDatasetHoldsHasAnEmptyGlance(t *testing.T) {
	g := Describe(KindCVE, "CVE-2099-0001", nil).Glance

	if g.Subtitle != "" || g.Summary != "" || len(g.Tags) != 0 || len(g.Facts) != 0 {
		t.Fatalf("glance %+v, want nothing invented", g)
	}
}

func TestAKEVListingWithoutKnownRansomwareSaysNothingAboutIt(t *testing.T) {
	var g Glance
	addKEVGlance(&g, intel.KEVRecord{DueDate: "2025-12-12", Ransomware: "Unknown"})

	if !reflect.DeepEqual(g.Facts, []string{"KEV due 2025-12-12"}) {
		t.Fatalf("facts %v", g.Facts)
	}
}
