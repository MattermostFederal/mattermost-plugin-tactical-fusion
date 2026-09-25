package cyber

import (
	"html"
	"reflect"
	"strings"
	"testing"

	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/decorators/cyber/intel"
)

const cwe79Detail = "CWE-79\tAn invented full description. It has a second sentence.\tInvented background.\t" +
	`[{"scopes":["Confidentiality","Integrity"],"impacts":["Read Application Data","Execute Unauthorized Code or Commands"],"likelihood":"High","note":"An invented note."}]` + "\t" +
	`[{"phase":"Implementation","strategy":"Output Encoding","description":"Encode it.","effectiveness":"High"},{"phase":"Architecture and Design","description":"Design it out."}]` + "\t" +
	`[{"method":"Black Box","description":"Probe it.","effectiveness":"Moderate"}]` + "\t" +
	`[{"id":"CVE-2021-44228","description":"An invented example."},{"id":"[REF-1]","description":"A citation."}]`

func sectionTitles(d Details) []string {
	titles := make([]string, 0, len(d.Sections))
	for _, section := range d.Sections {
		titles = append(titles, section.Title)
	}
	return titles
}

func TestAWeaknessShowsItsDetailInSections(t *testing.T) {
	set := datasets(t, map[string][]string{intel.NameCWEDetail: {cwe79Detail}})

	d := Describe(KindCWE, "CWE-79", set)

	if d.Summary != "An invented full description. It has a second sentence." {
		t.Fatalf("summary %q, want the whole description in place of the catalog's first sentence", d.Summary)
	}

	want := []string{"Background", "Consequences", "Mitigations", "Detection methods", "Observed examples"}
	if got := sectionTitles(d); !reflect.DeepEqual(got, want) {
		t.Fatalf("sections %v, want %v", got, want)
	}

	if got := d.Sections[1].Items[0]; got.Head != "Confidentiality, Integrity: Read Application Data; Execute Unauthorized Code or Commands (likelihood high)" || got.Text != "An invented note." {
		t.Errorf("consequence %+v", got)
	}
	if got := d.Sections[2].Items; got[0].Head != "Implementation, Output Encoding (effectiveness high)" || got[1].Head != "Architecture and Design" {
		t.Errorf("mitigations %+v", got)
	}
	if got := d.Sections[3].Items[0].Head; got != "Black Box (effectiveness moderate)" {
		t.Errorf("detection %q", got)
	}
}

func TestAnObservedExampleLinksOnlyWhenItIsACVE(t *testing.T) {
	set := datasets(t, map[string][]string{intel.NameCWEDetail: {cwe79Detail}})

	examples := Describe(KindCWE, "CWE-79", set).Sections[4].Items

	if examples[0].Link == nil || examples[0].Link.Kind != KindCVE || examples[0].Link.Value != "CVE-2021-44228" {
		t.Fatalf("the CVE example carries no CVE link: %+v", examples[0])
	}
	if !RecognizeAs(examples[0].Link.Kind, examples[0].Link.Value) {
		t.Fatalf("the example's link does not agree with itself")
	}
	if examples[1].Link != nil {
		t.Fatalf("a citation was linked as an indicator: %+v", examples[1])
	}
}

func TestAWeaknessWithNoDetailDatasetKeepsTheCatalogAlone(t *testing.T) {
	d := Describe(KindCWE, "CWE-79", nil)

	if len(d.Sections) != 0 {
		t.Fatalf("sections with no dataset: %+v", d.Sections)
	}
	weakness, _ := LookupWeakness("CWE-79")
	if d.Summary != weakness.Summary {
		t.Fatalf("summary %q, want the catalog's", d.Summary)
	}
	if rowValue(d, "Details") != "" {
		t.Fatalf("a missing bundled file was reported as a missing row")
	}
}

func TestAWeaknessTheDetailFileLacksSaysSo(t *testing.T) {
	set := datasets(t, map[string][]string{intel.NameCWEDetail: {cwe79Detail}})

	d := Describe(KindCWE, "CWE-89", set)

	if got := rowValue(d, "Details"); !strings.HasPrefix(got, "Not in the weakness detail dataset") {
		t.Fatalf("Details row %q", got)
	}
}

func TestThePageShowsTheWeaknessSections(t *testing.T) {
	set := datasets(t, map[string][]string{intel.NameCWEDetail: {cwe79Detail}})

	body := renderBody(Describe(KindCWE, "CWE-79", set))

	link := html.EscapeString(linkHref(Link{Kind: KindCVE, Value: "CVE-2021-44228"}))
	for _, want := range []string{"Mitigations (2)", "Observed examples (2)", "Encode it.", `href="` + link + `"`} {
		if !strings.Contains(body, want) {
			t.Errorf("the page lacks %q", want)
		}
	}
}
