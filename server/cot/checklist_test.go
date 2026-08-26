package cot

import (
	"fmt"
	"strings"
	"testing"
)

func checklistProp(t *testing.T, detail string) map[string]any {
	t.Helper()

	props := detailProps(t, detail)
	list, ok := props["checklist"].(map[string]any)
	if !ok {
		t.Fatalf("no checklist in %v", props)
	}
	return list
}

func kindCounts(t *testing.T, list map[string]any) map[string]string {
	t.Helper()

	counts := map[string]string{}
	kinds, ok := list["kinds"].([]any)
	if !ok {
		return counts
	}
	for _, entry := range kinds {
		kind, ok := entry.(map[string]any)
		if !ok {
			t.Fatalf("a kind is %T, not a map", entry)
		}
		counts[kind["name"].(string)] = kind["count"].(string)
	}
	return counts
}

func TestAChecklistIsCountedRatherThanUnrecognised(t *testing.T) {
	props := detailProps(t, `<checklist/>`)

	if props["detail_unknown"] != nil {
		t.Errorf("detail_unknown is %v; the checklist was read, not missed", props["detail_unknown"])
	}
	if props["checklist"] == nil {
		t.Error("the event carried a checklist and the props say nothing about it")
	}
}

func TestAChecklistCountsItsContentsByTheNameTheEventUsed(t *testing.T) {
	list := checklistProp(t, `<checklist>`+
		`<checklistColumn/><checklistColumn/><checklistTask/>`+
		`</checklist>`)

	counts := kindCounts(t, list)
	if counts["checklistColumn"] != "2" {
		t.Errorf("checklistColumn counted %q, want 2", counts["checklistColumn"])
	}
	if counts["checklistTask"] != "1" {
		t.Errorf("checklistTask counted %q, want 1", counts["checklistTask"])
	}
	if list["count"] != "3" {
		t.Errorf("the total is %v, want 3", list["count"])
	}
}

// The nesting is the part no source pins down, so the count must not depend on
// it: a column inside a task is counted with the columns beside it.
func TestAChecklistCountsDescendantsAtAnyDepth(t *testing.T) {
	shallow := kindCounts(t, checklistProp(t, `<checklist>`+
		`<checklistColumn/><checklistColumn/>`+
		`</checklist>`))

	nested := kindCounts(t, checklistProp(t, `<checklist>`+
		`<checklistColumn/><checklistTask><checklistColumn/></checklistTask>`+
		`</checklist>`))

	if shallow["checklistColumn"] != nested["checklistColumn"] {
		t.Errorf("a nested column counted %q where a flat one counted %q",
			nested["checklistColumn"], shallow["checklistColumn"])
	}
}

// Nothing inside a checklist is read as something this build understands. A
// shape there is contents, not geometry, because the shape of a checklist is
// exactly what is unverified.
func TestAChecklistsContentsAreNotReadAsSomethingElse(t *testing.T) {
	props := detailProps(t, `<checklist><shape><polyline closed="true">`+
		`<vertex lat="34.0561" lon="-118.2500"/><vertex lat="34.0600" lon="-118.2400"/>`+
		`</polyline></shape></checklist>`)

	if props["geometry"] != nil {
		t.Errorf("a shape inside a checklist was drawn as geometry: %v", props["geometry"])
	}

	counts := kindCounts(t, props["checklist"].(map[string]any))
	if counts["shape"] != "1" || counts["vertex"] != "2" {
		t.Errorf("the contents were counted as %v", counts)
	}
}

// First-wins, as shape and the registry are, so a second checklist cannot add
// contents to the first one's tally.
func TestASecondChecklistIsNotRead(t *testing.T) {
	list := checklistProp(t, `<checklist><checklistTask/></checklist>`+
		`<checklist><checklistTask/><checklistTask/></checklist>`)

	if list["count"] != "1" {
		t.Errorf("the total is %v, want the first checklist's 1", list["count"])
	}
}

// Seen counts everything, including what the cap refused to name. A counter
// that stops when the list stops growing reports the cap as the measurement,
// which is the bug the vertex cap already shipped once.
func TestAChecklistKeepsCountingPastTheKindCap(t *testing.T) {
	var detail strings.Builder
	detail.WriteString(`<checklist>`)
	for i := range maxChecklistKinds + 4 {
		fmt.Fprintf(&detail, `<kind%d/>`, i)
	}
	detail.WriteString(`</checklist>`)

	list := checklistProp(t, detail.String())

	kinds, _ := list["kinds"].([]any)
	if len(kinds) != maxChecklistKinds {
		t.Errorf("named %d kinds, want the cap of %d", len(kinds), maxChecklistKinds)
	}
	if want := fmt.Sprint(maxChecklistKinds + 4); list["count"] != want {
		t.Errorf("the total is %v, want the %s the event carried", list["count"], want)
	}
}

// A kind already named keeps counting past the cap; only a new name is refused.
func TestACappedChecklistStillCountsAKindItAlreadyNamed(t *testing.T) {
	var detail strings.Builder
	detail.WriteString(`<checklist><early/>`)
	for i := range maxChecklistKinds {
		fmt.Fprintf(&detail, `<kind%d/>`, i)
	}
	detail.WriteString(`<early/></checklist>`)

	counts := kindCounts(t, checklistProp(t, detail.String()))

	if counts["early"] != "2" {
		t.Errorf("early counted %q, want 2", counts["early"])
	}
}

func TestAnEventWithNoChecklistWritesNoChecklistKey(t *testing.T) {
	props := detailProps(t, `<takv platform="ATAK"/>`)

	if _, held := props["checklist"]; held {
		t.Error("an event carrying no checklist still wrote a checklist key")
	}
}

func TestTheChecklistFixtureIsCounted(t *testing.T) {
	counts := kindCounts(t, checklistProp(t, FixtureChecklist()))

	if counts["checklistColumn"] != "3" || counts["checklistTask"] != "1" {
		t.Errorf("the fixture counted %v", counts)
	}
}
