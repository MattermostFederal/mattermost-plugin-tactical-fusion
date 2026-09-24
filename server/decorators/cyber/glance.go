package cyber

import (
	"slices"
	"strconv"
	"strings"

	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/decorators/cyber/intel"
)

type Glance struct {
	Subtitle string
	Summary  string
	Tags     []string
	Facts    []string
	Status   string
}

const glanceSeparator = " · "

func counted(n int, singular, plural string) string {
	if n == 1 {
		return "1 " + singular
	}
	return strconv.Itoa(n) + " " + plural
}

func addFact(g *Glance, fact string) {
	if fact != "" {
		g.Facts = append(g.Facts, fact)
	}
}

func addCount(g *Glance, n int, singular, plural string) {
	if n > 0 {
		g.Facts = append(g.Facts, counted(n, singular, plural))
	}
}

func appendTagOnce(tags []string, tag string) []string {
	if slices.Contains(tags, tag) {
		return tags
	}
	return append(tags, tag)
}

func weaknessGlance(weakness Weakness) Glance {
	return Glance{
		Subtitle: joinNonEmpty(glanceSeparator, weakness.ID, weakness.Abstraction, weakness.Status),
		Summary:  weakness.Summary,
	}
}

func addWeaknessDetailGlance(g *Glance, detail intel.CWEDetail) {
	for _, consequence := range detail.Consequences {
		for _, scope := range consequence.Scopes {
			g.Tags = appendTagOnce(g.Tags, scope)
		}
	}
	addCount(g, len(detail.Mitigations), "mitigation", "mitigations")
	addCount(g, len(detail.Examples), "observed example", "observed examples")
}

func techniqueGlance(technique Technique) Glance {
	g := Glance{Summary: technique.Summary}

	switch technique.Kind {
	case techniqueKindTactic:
		g.Subtitle = technique.ID + glanceSeparator + "Tactic"
		addCount(&g, len(tacticTechniqueItems(technique.ID)), "technique", "techniques")
	case techniqueKindSubTechnique:
		g.Subtitle = technique.ID + glanceSeparator + "Sub-technique"
		if parent, ok := LookupTechnique(technique.Parent); ok {
			g.Subtitle += " of " + parent.ID + " " + parent.Name
		}
	default:
		g.Subtitle = technique.ID + glanceSeparator + "Technique"
	}

	for _, tactic := range technique.Tactics {
		if parent, ok := LookupTechnique(tactic); ok {
			g.Tags = append(g.Tags, parent.Name)
		}
	}
	addFact(&g, strings.Join(technique.Platforms, ", "))

	if technique.Status != StatusActive && technique.Status != "" {
		g.Status = attackStatusText(technique.Status)
		if replacement, ok := LookupTechnique(technique.ReplacedBy); ok {
			g.Status += ", replaced by " + replacement.ID + " " + replacement.Name
		}
	}

	return g
}

func addTechniqueDetailGlance(g *Glance, detail intel.AttackDetail) {
	addCount(g, len(detail.Procedures), "procedure example", "procedure examples")
	addCount(g, len(detail.Mitigations), "mitigation", "mitigations")
	addCount(g, len(detail.Detections), "detection strategy", "detection strategies")
}

func addressGlance(scope string, record intel.IPRecord) Glance {
	return Glance{
		Subtitle: joinFields(record.ASN, record.ASName),
		Tags:     []string{scope},
		Facts:    nonEmpty(placeText(record.City, record.Region, record.Country)),
	}
}

func hashGlance(algorithm string, bytes int, set *intel.Set, onWatchlist bool) Glance {
	g := Glance{Subtitle: joinNonEmpty(glanceSeparator, algorithm, strconv.Itoa(bytes)+" bytes")}
	switch {
	case onWatchlist:
	case !set.Has(intel.NameWatchlist):
		addFact(&g, "No watchlist is installed")
	default:
		addFact(&g, "Not on the watchlist")
	}
	return g
}

func placeText(parts ...string) string {
	var kept []string
	for _, part := range parts {
		if part != "" && !slices.ContainsFunc(kept, func(k string) bool { return strings.EqualFold(k, part) }) {
			kept = append(kept, part)
		}
	}
	return strings.Join(kept, ", ")
}

func nonEmpty(value string) []string {
	if value == "" {
		return nil
	}
	return []string{value}
}
