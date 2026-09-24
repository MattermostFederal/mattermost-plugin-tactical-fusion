package cyber

import (
	"html"
	"reflect"
	"strings"
	"testing"

	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/decorators/cyber/intel"
)

const t1059Detail = "T1059.001\tAn invented full description. It has a second sentence.\t" +
	`[{"source":"Invented Report","url":"https://example.org/report","description":"Invented, A. (2026)."},{"source":"Bad Scheme","url":"javascript:alert(1)","description":"Never a link."}]` + "\t" +
	`[{"id":"M1026","name":"Privileged Account Management","url":"https://attack.mitre.org/mitigations/M1026","description":"Restrict it."}]` + "\t" +
	`[{"id":"DET0455","name":"Abuse of PowerShell","url":"https://attack.mitre.org/detectionstrategies/DET0455","analytics":[{"id":"AN1","platforms":["Windows"],"description":"Watch it.","logSources":[{"name":"WinEventLog:Sysmon","channel":"EventCode=1"}],"tunables":[{"field":"Threshold","description":"How many."}]}]},{"id":"DET0999","name":"Bare Strategy"}]` + "\t" +
	`[{"id":"G0007","name":"APT28","kind":"group","url":"https://attack.mitre.org/groups/G0007","description":"APT28 used PowerShell."}]`

func TestATechniqueShowsItsDetailInSections(t *testing.T) {
	set := datasets(t, map[string][]string{intel.NameAttackDetail: {t1059Detail}})

	d := Describe(KindAttack, "T1059.001", set)

	if d.Summary != "An invented full description. It has a second sentence." {
		t.Fatalf("summary %q, want the whole description", d.Summary)
	}
	want := []string{"Mitigations", "Detection", "Procedure examples", "References"}
	if got := sectionTitles(d); !reflect.DeepEqual(got, want) {
		t.Fatalf("sections %v, want %v", got, want)
	}

	if got := d.Sections[0].Items[0]; got.Head != "M1026 Privileged Account Management" || got.URL != "https://attack.mitre.org/mitigations/M1026" || got.Text != "Restrict it." {
		t.Errorf("mitigation %+v", got)
	}

	detection := d.Sections[1].Items
	if detection[0].Head != "DET0455 Abuse of PowerShell (Windows)" ||
		detection[0].Text != "Watch it.\nLog sources: WinEventLog:Sysmon (EventCode=1).\nTunable: Threshold (How many)." {
		t.Errorf("analytic %+v", detection[0])
	}
	if detection[1].Head != "DET0999 Bare Strategy" || detection[1].Text != "" {
		t.Errorf("a strategy with no analytics %+v", detection[1])
	}

	if got := d.Sections[2].Items[0]; got.Head != "G0007 APT28 (group)" || got.Text != "APT28 used PowerShell." {
		t.Errorf("procedure %+v", got)
	}
}

func TestAnAttackItemLinksOnlyAWebAddress(t *testing.T) {
	set := datasets(t, map[string][]string{intel.NameAttackDetail: {t1059Detail}})

	references := Describe(KindAttack, "T1059.001", set).Sections[3].Items

	if references[0].URL != "https://example.org/report" {
		t.Errorf("a web reference lost its link: %+v", references[0])
	}
	if references[1].URL != "" || references[1].Head != "Bad Scheme" {
		t.Errorf("a javascript: reference kept its link: %+v", references[1])
	}
}

func TestATacticListsItsActiveTechniques(t *testing.T) {
	d := Describe(KindAttack, "TA0002", nil)

	if got := sectionTitles(d); !reflect.DeepEqual(got, []string{"Techniques"}) {
		t.Fatalf("sections %v", got)
	}
	items := d.Sections[0].Items
	found := false
	for i, item := range items {
		if item.Link == nil || item.Link.Kind != KindAttack || !RecognizeAs(item.Link.Kind, item.Link.Value) {
			t.Fatalf("item %d does not link a technique: %+v", i, item)
		}
		technique, _ := LookupTechnique(item.Link.Value)
		if technique.Kind != techniqueKindTechnique || technique.Status != StatusActive {
			t.Errorf("%s is a %s %s", technique.ID, technique.Status, technique.Kind)
		}
		if i > 0 && items[i-1].Link.Value >= item.Link.Value {
			t.Errorf("techniques are not in id order at %s", item.Link.Value)
		}
		if item.Link.Value == "T1059" {
			found = true
		}
	}
	if !found {
		t.Fatalf("Execution does not list T1059")
	}
}

func TestATechniqueWithNoDetailDatasetKeepsTheCatalogAlone(t *testing.T) {
	d := Describe(KindAttack, "T1059.001", nil)

	if len(d.Sections) != 0 || rowValue(d, "Details") != "" {
		t.Fatalf("sections %+v, details %q", d.Sections, rowValue(d, "Details"))
	}
}

func TestThePageLinksAnAttackItemOutward(t *testing.T) {
	set := datasets(t, map[string][]string{intel.NameAttackDetail: {t1059Detail}})

	body := renderBody(Describe(KindAttack, "T1059.001", set))

	want := `<a href="` + html.EscapeString("https://attack.mitre.org/groups/G0007") + `" rel="noopener noreferrer" target="_blank"><strong>G0007 APT28 (group)</strong></a>`
	if !strings.Contains(body, want) {
		t.Fatalf("the page does not link the procedure's group outward")
	}
	if strings.Contains(body, "javascript:") {
		t.Fatalf("the page wrote a javascript: address")
	}
}

func TestThePageBreaksAnAnalyticsLinesWithoutUnescapingIt(t *testing.T) {
	set := datasets(t, map[string][]string{intel.NameAttackDetail: {t1059Detail}})

	body := renderBody(Describe(KindAttack, "T1059.001", set))

	if !strings.Contains(body, "Watch it.<br>Log sources: WinEventLog:Sysmon (EventCode=1).<br>Tunable: Threshold (How many).") {
		t.Fatalf("the analytic's lines are not broken on the page")
	}
}
