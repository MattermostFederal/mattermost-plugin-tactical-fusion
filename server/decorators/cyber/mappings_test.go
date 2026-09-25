package cyber

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/decorators/cyber/intel"
)

func goldenRows(t *testing.T, name string) []string {
	t.Helper()

	raw, err := os.ReadFile(filepath.Join("intel", "testdata", filepath.Base(name)+intel.Suffix)) // #nosec G304 -- a fixture name from this package's own tests
	if err != nil {
		t.Fatal(err)
	}
	var rows []string
	for line := range strings.SplitSeq(strings.TrimRight(string(raw), "\n"), "\n") {
		if !strings.HasPrefix(line, intel.SchemaPrefix) {
			rows = append(rows, line)
		}
	}
	return rows
}

func mappingSet(t *testing.T) *intel.Set {
	return datasets(t, map[string][]string{
		intel.NameCAPEC:     goldenRows(t, "capec"),
		intel.NameCVEAttack: goldenRows(t, "cveattack"),
	})
}

func sectionItems(d Details, title string) []Item {
	for _, section := range d.Sections {
		if section.Title == title {
			return section.Items
		}
	}
	return nil
}

func TestAWeaknessListsItsAttackPatternsLinkedToCAPEC(t *testing.T) {
	items := sectionItems(Describe(KindCWE, "CWE-79", mappingSet(t)), attackPatternsKey)

	if len(items) != 2 {
		t.Fatalf("attack patterns are %+v", items)
	}
	if items[0].Head != "CAPEC-63 Cross-Site Scripting (XSS) (very high severity)" ||
		items[0].URL != "https://capec.mitre.org/data/definitions/63.html" || items[0].Text != "An adversary embeds scripts." {
		t.Errorf("the first pattern is %+v", items[0])
	}
}

func TestATechniqueListsItsAttackPatternsAndItsKnownExploitedCVEs(t *testing.T) {
	d := Describe(KindAttack, "T1059.001", mappingSet(t))

	if patterns := sectionItems(d, attackPatternsKey); len(patterns) != 1 {
		t.Errorf("attack patterns are %+v", patterns)
	}
	cves := sectionItems(d, exploitedByKey)
	if len(cves) != 2 || cves[0].Head != "CVE-2022-41082" || cves[0].Text != "Exploitation technique, primary impact" ||
		cves[0].Link == nil || cves[0].Link.Kind != KindCVE {
		t.Errorf("known exploited vulnerabilities are %+v", cves)
	}
}

func TestACVELinksTheTechniquesItIsMappedTo(t *testing.T) {
	d := Describe(KindCVE, "CVE-2022-41082", mappingSet(t))

	for _, link := range d.Related {
		if link.Kind == KindAttack && link.Value == "T1059.001" {
			if link.Label != "T1059.001 PowerShell (exploitation technique, primary impact)" {
				t.Errorf("the link reads %q", link.Label)
			}
			return
		}
	}
	t.Errorf("CVE-2022-41082 does not link T1059.001: %+v", d.Related)
}

func TestNoMappingDatasetMeansNoMappingSections(t *testing.T) {
	d := Describe(KindAttack, "T1059.001", nil)

	if sectionItems(d, attackPatternsKey) != nil || sectionItems(d, exploitedByKey) != nil {
		t.Errorf("sections appeared with nothing installed: %+v", d.Sections)
	}
}

func TestCAPECURLIsBuiltOnlyFromANumericId(t *testing.T) {
	for id, want := range map[string]string{"CAPEC-63": "https://capec.mitre.org/data/definitions/63.html", "CAPEC-x": "", "63": ""} {
		if got := capecURL(id); got != want {
			t.Errorf("capecURL(%q) = %q, want %q", id, got, want)
		}
	}
}
