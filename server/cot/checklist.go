package cot

const (
	checklistElement = "checklist"

	maxChecklistKinds = 8
)

type ChecklistKind struct {
	Name  string
	Count int
}

type Checklist struct {
	Present bool

	Kinds []ChecklistKind

	Seen int
}

func (c *Checklist) empty() bool { return !c.Present }

func (c *Checklist) add(name string) {
	c.Seen++

	for i := range c.Kinds {
		if c.Kinds[i].Name == name {
			c.Kinds[i].Count++
			return
		}
	}

	if len(c.Kinds) >= maxChecklistKinds {
		return
	}

	c.Kinds = append(c.Kinds, ChecklistKind{Name: name, Count: 1})
}

// FixtureChecklist is a checklist, for the cross-language guard's event.
func FixtureChecklist() string {
	return `<checklist>` +
		`<checklistColumn/><checklistColumn/>` +
		`<checklistTask><checklistColumn/></checklistTask>` +
		`</checklist>`
}
