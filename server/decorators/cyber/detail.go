package cyber

import (
	"errors"
	"net/url"
	"slices"
	"strconv"
	"strings"

	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/decorators/cyber/intel"
)

type Reference struct {
	URL  string
	Tags string
}

const (
	maxPlatformsNamed = 5
	nvdSource         = "nvd@nist.gov"
)

func describeCVEDetail(d *Details, set *intel.Set, recordFound bool) {
	detail, err := set.CVEDetail(d.Value)
	switch {
	case errors.Is(err, intel.ErrNoDataset):
		return
	case errors.Is(err, intel.ErrNotFound):
		if recordFound {
			addRow(d, "Details", datasetSentence(set, intel.NameCVEDetail, err))
		}
		return
	case err != nil:
		addRow(d, "Details", datasetSentence(set, intel.NameCVEDetail, err))
		return
	}

	addRow(d, "Weakness sources", weaknessSourcesText(detail.Weaknesses))
	d.Affected = affectedLines(detail.Affected)
	d.Configurations = configurationLines(detail.Configurations)
	d.References = referenceLinks(detail.References)
}

func weaknessSourcesText(sources []intel.WeaknessSource) string {
	parts := make([]string, 0, len(sources))
	for _, source := range sources {
		name := source.Source
		if name == nvdSource {
			name = "NVD"
		}
		parts = append(parts, name+": "+strings.Join(source.CWE, ", "))
	}
	return strings.Join(parts, "; ")
}

func cpeFields(cpe string) []string {
	var fields []string
	var field strings.Builder
	escaped := false

	for _, r := range cpe {
		switch {
		case escaped:
			field.WriteRune(r)
			escaped = false
		case r == '\\':
			escaped = true
		case r == ':':
			fields = append(fields, field.String())
			field.Reset()
		default:
			field.WriteRune(r)
		}
	}

	return append(fields, field.String())
}

func unescapeCPE(value string) string {
	var unescaped strings.Builder
	escaped := false

	for _, r := range value {
		if !escaped && r == '\\' {
			escaped = true
			continue
		}
		unescaped.WriteRune(r)
		escaped = false
	}

	return unescaped.String()
}

func cpeField(fields []string, at int) string {
	if at >= len(fields) {
		return ""
	}
	if value := fields[at]; value != "*" && value != "-" {
		return value
	}
	return ""
}

func cpeName(fields []string) string {
	words := make([]string, 0, 2)
	for _, at := range []int{1, 2} {
		if value := cpeField(fields, at); value != "" {
			words = append(words, strings.ReplaceAll(value, "_", " "))
		}
	}
	return strings.Join(words, " ")
}

func cpeVersionText(match intel.CPEMatch, fields []string) string {
	var parts []string

	if version := cpeField(fields, 3); version != "" {
		if update := cpeField(fields, 4); update != "" {
			version += " " + update
		}
		parts = append(parts, version)
	}

	for _, bound := range []struct{ word, value string }{
		{"from", match.From},
		{"after", match.After},
		{"through", match.Through},
		{"before", match.Before},
	} {
		if bound.value != "" {
			parts = append(parts, bound.word+" "+unescapeCPE(bound.value))
		}
	}

	return strings.Join(parts, " ")
}

func appendDistinct(values []string, value string) []string {
	if slices.Contains(values, value) {
		return values
	}
	return append(values, value)
}

func configurationLines(groups []intel.Configuration) []string {
	var lines []string

	for _, group := range groups {
		platforms := platformText(group.On)

		var names []string
		specs := map[string][]string{}

		for _, match := range group.Vulnerable {
			fields := cpeFields(match.CPE)
			name := cpeName(fields)
			if name == "" {
				continue
			}
			if _, known := specs[name]; !known {
				names = append(names, name)
				specs[name] = nil
			}
			if spec := cpeVersionText(match, fields); spec != "" {
				specs[name] = appendDistinct(specs[name], spec)
			}
		}

		for _, name := range names {
			line := name + ": all versions"
			if len(specs[name]) > 0 {
				line = name + ": " + strings.Join(specs[name], ", ")
			}
			if platforms != "" {
				line += " (on " + platforms + ")"
			}
			lines = appendDistinct(lines, line)
		}
	}

	return lines
}

func platformText(matches []intel.CPEMatch) string {
	var names []string
	seen := map[string]bool{}

	for _, match := range matches {
		fields := cpeFields(match.CPE)
		if cpeName(fields) == "" {
			continue
		}
		name := strings.TrimSpace(cpeName(fields) + " " + cpeVersionText(match, fields))
		if !seen[name] {
			seen[name] = true
			names = append(names, name)
		}
	}

	if len(names) <= maxPlatformsNamed {
		return strings.Join(names, ", ")
	}

	return strings.Join(names[:maxPlatformsNamed], ", ") + " and " + strconv.Itoa(len(names)-maxPlatformsNamed) + " more"
}

func isOpenStart(version string) bool {
	switch version {
	case "", "0", "*":
		return true
	}
	return false
}

func affectedVersionText(version intel.AffectedVersion) string {
	var spec string
	switch {
	case version.LessThan != "" && isOpenStart(version.Version):
		spec = "before " + version.LessThan
	case version.LessThan != "":
		spec = "from " + version.Version + " before " + version.LessThan
	case version.LessThanOrEqual != "" && isOpenStart(version.Version):
		spec = "through " + version.LessThanOrEqual
	case version.LessThanOrEqual != "":
		spec = "from " + version.Version + " through " + version.LessThanOrEqual
	default:
		spec = version.Version
	}

	if len(version.Changes) > 0 {
		changes := make([]string, 0, len(version.Changes))
		for _, change := range version.Changes {
			changes = append(changes, change.Status+" from "+change.At)
		}
		spec += " (" + strings.Join(changes, ", ") + ")"
	}

	if version.Status != "" && version.Status != "affected" {
		spec = version.Status + ": " + spec
	}

	return spec
}

func affectedLines(products []intel.AffectedProduct) []string {
	lines := make([]string, 0, len(products))
	seen := map[string]bool{}
	add := func(line string) {
		if !seen[line] {
			seen[line] = true
			lines = append(lines, line)
		}
	}

	for _, product := range products {
		name := strings.TrimSpace(product.Vendor + " " + product.Product)

		entries := make([]string, 0, len(product.Versions)+1)
		for _, version := range product.Versions {
			entries = append(entries, affectedVersionText(version))
		}
		if product.DefaultStatus == "affected" {
			entries = append(entries, "all other versions affected")
		}

		if len(entries) == 0 {
			add(name)
			continue
		}
		add(name + ": " + strings.Join(entries, "; "))
	}

	return lines
}

func isWebURL(raw string) bool {
	parsed, err := url.Parse(raw)
	if err != nil {
		return false
	}
	return (parsed.Scheme == "http" || parsed.Scheme == "https") && parsed.Host != ""
}

func referenceLinks(refs []intel.Reference) []Reference {
	links := make([]Reference, 0, len(refs))
	seen := map[string]bool{}

	for _, ref := range refs {
		if !isWebURL(ref.URL) || seen[ref.URL] {
			continue
		}
		seen[ref.URL] = true
		links = append(links, Reference{URL: ref.URL, Tags: strings.Join(ref.Tags, ", ")})
	}

	return links
}
