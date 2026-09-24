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

const dateLength = len("2006-01-02")

func dateOf(timestamp string) string {
	if len(timestamp) >= dateLength {
		return timestamp[:dateLength]
	}
	return timestamp
}

func vulnerabilityGlance(record intel.CVERecord, vector []VectorMetric) Glance {
	g := Glance{Summary: record.Summary}

	subtitle := []string{}
	if published := dateOf(record.Published); published != "" {
		subtitle = append(subtitle, "Published "+published)
	}
	subtitle = append(subtitle, record.Weaknesses...)
	g.Subtitle = strings.Join(subtitle, glanceSeparator)

	g.Tags = exposureTags(vector)
	return g
}

func exposureTags(vector []VectorMetric) []string {
	var tags []string
	for _, metric := range vector {
		switch metric.Metric {
		case "Attack vector":
			tags = append(tags, metric.Value)
		case "Privileges required":
			if metric.Value == "None" {
				tags = append(tags, "No privileges")
			} else {
				tags = append(tags, metric.Value+" privileges")
			}
		case "Authentication":
			if metric.Value == "None" {
				tags = append(tags, "No authentication")
			} else {
				tags = append(tags, metric.Value+" authentication")
			}
		case "User interaction":
			switch metric.Value {
			case "None":
				tags = append(tags, "No user interaction")
			case "Required":
				tags = append(tags, "User interaction required")
			default:
				tags = append(tags, metric.Value+" user interaction")
			}
		}
	}
	return tags
}

func addEPSSGlance(g *Glance, epss intel.EPSSRecord) {
	if probability, ok := asPercent(epss.Score); ok {
		addFact(g, "EPSS "+probability)
	}
}

func addKEVGlance(g *Glance, kev intel.KEVRecord) {
	if kev.DueDate != "" {
		addFact(g, "KEV due "+kev.DueDate)
	}
	if strings.EqualFold(kev.Ransomware, "known") {
		addFact(g, "Ransomware use known")
	}
}

func addVulnerabilityDetailGlance(g *Glance, d *Details) {
	addCount(g, len(d.Affected), "affected product", "affected products")
	addCount(g, len(d.References), "reference", "references")
}
