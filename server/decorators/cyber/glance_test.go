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
