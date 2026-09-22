package airport

import (
	"strings"
)

const tableHeaderLabel = "Airfield"

var cellEscaper = strings.NewReplacer(
	`\`, `\\`,
	`|`, `\|`,
	"`", "\\`",
	`*`, `\*`,
	`_`, `\_`,
	`[`, `\[`,
	`]`, `\]`,
	`<`, `\<`,
	`>`, `\>`,
	`~`, `\~`,
	"\r", " ",
	"\n", " ",
)

func mdCell(value string) string {
	return cellEscaper.Replace(value)
}

func airfieldTable(href, trail string, d Details) string {
	name := d.Name
	if name == "" {
		name = d.Ident
	}

	var b strings.Builder
	b.WriteString("| " + tableHeaderLabel + " | [" + mdCell(name) + "](" + href + ") |\n")
	b.WriteString("|:--|:--|\n")

	for _, row := range []struct{ label, value string }{
		{"Code", d.Ident + trail},
		{"Place", d.Place},
		{"Use", useText(d)},
		{"Type", d.Type},
		{"Elevation", d.Elevation},
		{"IATA", d.IATA},
		{"Runways", runwaysCell(d.Runways)},
		{"Frequencies", frequenciesCell(d.Frequencies)},
	} {
		if row.value == "" {
			continue
		}
		b.WriteString("| " + row.label + " | " + mdCell(row.value) + " |\n")
	}

	return strings.TrimRight(b.String(), "\n")
}

func runwaysCell(runways []Runway) string {
	parts := make([]string, 0, len(runways))
	for _, r := range runways {
		line := r.Designation
		if detail := RunwayLine(r); detail != "" {
			line += " " + detail
		}
		parts = append(parts, line)
	}
	return strings.Join(parts, "; ")
}

func frequenciesCell(frequencies []Frequency) string {
	parts := make([]string, 0, len(frequencies))
	for _, f := range frequencies {
		parts = append(parts, f.Type+" "+f.MHz)
	}
	return strings.Join(parts, "; ")
}
