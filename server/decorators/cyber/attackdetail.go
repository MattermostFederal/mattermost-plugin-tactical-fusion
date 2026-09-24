package cyber

import (
	"errors"
	"slices"
	"sort"
	"strings"

	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/decorators/cyber/intel"
)

func describeAttackDetail(d *Details, set *intel.Set) {
	detail, err := set.AttackDetail(d.Value)
	switch {
	case errors.Is(err, intel.ErrNoDataset):
		return
	case err != nil:
		addRow(d, "Details", datasetSentence(set, intel.NameAttackDetail, err))
		return
	}

	if detail.Description != "" {
		d.Summary = detail.Description
	}
	addTechniqueDetailGlance(&d.Glance, detail)

	addSection(d, "Mitigations", attackMitigationItems(detail.Mitigations))
	addSection(d, "Detection", attackDetectionItems(detail.Detections))
	addSection(d, "Procedure examples", procedureItems(detail.Procedures))
	addSection(d, "References", attackReferenceItems(detail.References))
}

func webURL(raw string) string {
	if isWebURL(raw) {
		return raw
	}
	return ""
}

func attackMitigationItems(mitigations []intel.AttackMitigation) []Item {
	items := make([]Item, 0, len(mitigations))
	for _, m := range mitigations {
		items = append(items, Item{Head: joinNonEmpty(" ", m.ID, m.Name), Text: m.Description, URL: webURL(m.URL)})
	}
	return items
}

func analyticText(analytic intel.Analytic) string {
	sources := make([]string, 0, len(analytic.LogSources))
	for _, source := range analytic.LogSources {
		sources = append(sources, withParenthetical(source.Name, source.Channel))
	}
	tunables := make([]string, 0, len(analytic.Tunables))
	for _, tunable := range analytic.Tunables {
		tunables = append(tunables, withParenthetical(tunable.Field, tunable.Description))
	}

	parts := []string{analytic.Description}
	if len(sources) > 0 {
		parts = append(parts, "Log sources: "+strings.Join(sources, "; ")+".")
	}
	if len(tunables) > 0 {
		parts = append(parts, "Tunable: "+strings.Join(tunables, "; ")+".")
	}
	return joinNonEmpty("\n", parts...)
}

func withParenthetical(value, detail string) string {
	detail = strings.TrimSuffix(detail, ".")
	if detail == "" {
		return value
	}
	return value + " (" + detail + ")"
}

func attackDetectionItems(detections []intel.AttackDetection) []Item {
	var items []Item
	for _, detection := range detections {
		head := joinNonEmpty(" ", detection.ID, detection.Name)
		if len(detection.Analytics) == 0 {
			items = append(items, Item{Head: head, URL: webURL(detection.URL)})
			continue
		}
		for _, analytic := range detection.Analytics {
			items = append(items, Item{
				Head: withParenthetical(head, strings.Join(analytic.Platforms, ", ")),
				Text: analyticText(analytic),
				URL:  webURL(detection.URL),
			})
		}
	}
	return items
}

func procedureItems(procedures []intel.Procedure) []Item {
	items := make([]Item, 0, len(procedures))
	for _, p := range procedures {
		items = append(items, Item{
			Head: withParenthetical(joinNonEmpty(" ", p.ID, p.Name), p.Kind),
			Text: p.Description,
			URL:  webURL(p.URL),
		})
	}
	return items
}

func attackReferenceItems(references []intel.AttackReference) []Item {
	items := make([]Item, 0, len(references))
	for _, ref := range references {
		items = append(items, Item{Head: ref.Source, Text: ref.Description, URL: webURL(ref.URL)})
	}
	return items
}

func tacticTechniqueItems(tactic string) []Item {
	var found []Technique
	for _, technique := range techniques {
		if technique.Kind == techniqueKindTechnique && technique.Status == StatusActive && slices.Contains(technique.Tactics, tactic) {
			found = append(found, technique)
		}
	}
	sort.Slice(found, func(i, j int) bool { return found[i].ID < found[j].ID })

	items := make([]Item, 0, len(found))
	for _, technique := range found {
		link := techniqueLink(technique.ID)
		items = append(items, Item{Head: link.Label, Link: &link})
	}
	return items
}
