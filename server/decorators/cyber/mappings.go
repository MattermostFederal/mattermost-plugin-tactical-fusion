package cyber

import (
	"errors"
	"strings"

	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/decorators/cyber/intel"
)

const (
	capecURLPrefix    = "https://capec.mitre.org/data/definitions/"
	capecIDPrefix     = "CAPEC-"
	attackPatternsKey = "Attack patterns"
	exploitedByKey    = "Known exploited vulnerabilities"
)

func mappingMissing(err error) bool {
	return errors.Is(err, intel.ErrNoDataset) || errors.Is(err, intel.ErrNotFound)
}

func describeAttackPatterns(d *Details, set *intel.Set) {
	patterns, err := set.AttackPatterns(d.Value)
	if err != nil {
		if !mappingMissing(err) {
			addRow(d, attackPatternsKey, datasetSentence(set, intel.NameCAPEC, err))
		}
		return
	}

	items := make([]Item, 0, len(patterns))
	for _, pattern := range patterns {
		items = append(items, Item{
			Head: withParenthetical(joinNonEmpty(" ", pattern.ID, pattern.Name), severityPhrase(pattern.Severity)),
			Text: pattern.Summary,
			URL:  capecURL(pattern.ID),
		})
	}
	addSection(d, attackPatternsKey, items)
}

func severityPhrase(severity string) string {
	if severity == "" {
		return ""
	}
	return strings.ToLower(severity) + " severity"
}

func capecURL(id string) string {
	number, ok := strings.CutPrefix(id, capecIDPrefix)
	if !ok || number == "" || strings.Trim(number, "0123456789") != "" {
		return ""
	}
	return capecURLPrefix + number + ".html"
}

func describeMappedTechniques(d *Details, set *intel.Set) {
	mappings, err := set.ATTACKMappings(d.Value)
	if err != nil {
		if !mappingMissing(err) {
			addRow(d, "ATT&CK", datasetSentence(set, intel.NameCVEAttack, err))
		}
		return
	}

	for _, mapping := range mappings {
		if _, known := LookupTechnique(mapping.ID); !known {
			continue
		}
		link := techniqueLink(mapping.ID)
		link.Label = withParenthetical(link.Label, strings.Join(mapping.Types, ", "))
		d.Related = append(d.Related, link)
	}
}

func describeExploitedVulnerabilities(d *Details, set *intel.Set) {
	mappings, err := set.ATTACKMappings(d.Value)
	if err != nil {
		if !mappingMissing(err) {
			addRow(d, exploitedByKey, datasetSentence(set, intel.NameCVEAttack, err))
		}
		return
	}

	items := make([]Item, 0, len(mappings))
	for _, mapping := range mappings {
		item := Item{Head: mapping.ID, Text: capitalize(strings.Join(mapping.Types, ", "))}
		if RecognizeAs(KindCVE, mapping.ID) {
			item.Link = &Link{Kind: KindCVE, Value: mapping.ID, Label: mapping.ID}
		}
		items = append(items, item)
	}
	addSection(d, exploitedByKey, items)
}

func capitalize(text string) string {
	if text == "" {
		return ""
	}
	return strings.ToUpper(text[:1]) + text[1:]
}
