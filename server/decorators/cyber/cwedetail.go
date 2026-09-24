package cyber

import (
	"errors"
	"strings"

	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/decorators/cyber/intel"
)

type Section struct {
	Title string
	Items []Item
}

type Item struct {
	Head string
	Text string
	Link *Link
}

func describeCWEDetail(d *Details, set *intel.Set) {
	detail, err := set.CWEDetail(d.Value)
	switch {
	case errors.Is(err, intel.ErrNoDataset):
		return
	case err != nil:
		addRow(d, "Details", datasetSentence(set, intel.NameCWEDetail, err))
		return
	}

	if detail.Description != "" {
		d.Summary = detail.Description
	}

	addSection(d, "Background", backgroundItems(detail.Extended))
	addSection(d, "Consequences", consequenceItems(detail.Consequences))
	addSection(d, "Mitigations", mitigationItems(detail.Mitigations))
	addSection(d, "Detection methods", detectionItems(detail.Detections))
	addSection(d, "Observed examples", exampleItems(detail.Examples))
}

func addSection(d *Details, title string, items []Item) {
	if len(items) > 0 {
		d.Sections = append(d.Sections, Section{Title: title, Items: items})
	}
}

func withQualifier(head, label, value string) string {
	if value == "" {
		return head
	}
	return head + " (" + label + " " + strings.ToLower(value) + ")"
}

func joinNonEmpty(separator string, values ...string) string {
	kept := make([]string, 0, len(values))
	for _, value := range values {
		if value != "" {
			kept = append(kept, value)
		}
	}
	return strings.Join(kept, separator)
}

func backgroundItems(extended string) []Item {
	if extended == "" {
		return nil
	}
	return []Item{{Text: extended}}
}

func consequenceItems(consequences []intel.Consequence) []Item {
	items := make([]Item, 0, len(consequences))
	for _, c := range consequences {
		head := joinNonEmpty(": ", strings.Join(c.Scopes, ", "), strings.Join(c.Impacts, "; "))
		items = append(items, Item{Head: withQualifier(head, "likelihood", c.Likelihood), Text: c.Note})
	}
	return items
}

func mitigationItems(mitigations []intel.Mitigation) []Item {
	items := make([]Item, 0, len(mitigations))
	for _, m := range mitigations {
		head := joinNonEmpty(", ", m.Phase, m.Strategy)
		items = append(items, Item{Head: withQualifier(head, "effectiveness", m.Effectiveness), Text: m.Description})
	}
	return items
}

func detectionItems(detections []intel.Detection) []Item {
	items := make([]Item, 0, len(detections))
	for _, detection := range detections {
		items = append(items, Item{
			Head: withQualifier(detection.Method, "effectiveness", detection.Effectiveness),
			Text: detection.Description,
		})
	}
	return items
}

func exampleItems(examples []intel.Example) []Item {
	items := make([]Item, 0, len(examples))
	for _, example := range examples {
		item := Item{Head: example.ID, Text: example.Description}
		if RecognizeAs(KindCVE, example.ID) {
			item.Link = &Link{Kind: KindCVE, Value: example.ID, Label: example.ID}
		}
		items = append(items, item)
	}
	return items
}
