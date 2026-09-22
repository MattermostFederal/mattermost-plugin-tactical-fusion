package airport

import (
	"html"
	"net/url"
	"strings"
)

const pageTitle = "Airfield"

const pageStyles = `
.ident { font-family: ui-monospace, SFMono-Regular, Menlo, monospace; font-size: 13px;
  color: var(--muted); margin: 0; letter-spacing: .08em; }
.name { font-size: 22px; font-weight: 600; margin: 0 0 6px; }
td.value { font-family: ui-monospace, SFMono-Regular, Menlo, monospace; text-align: right;
  word-break: break-all; }
td.value a { color: var(--accent); }
h2 { font-size: 13px; text-transform: uppercase; letter-spacing: .04em; margin: 18px 0 4px;
  color: var(--muted); }
td.plain { text-align: right; }
`

const coordinatePath = "location"

type row struct {
	label string
	value string
	href  string
}

func detailRows(d Details) []row {
	place := row{label: "Place", value: d.Place}
	if d.HasPosition {
		place.href = coordinateHref(d)
	}

	return []row{
		{label: "Code", value: d.Ident},
		place,
		{label: "Use", value: useText(d)},
		{label: "Type", value: d.Type},
		{label: "Elevation", value: d.Elevation},
		{label: "IATA", value: d.IATA},
	}
}

func useText(d Details) string {
	if d.Military == "" {
		return ""
	}
	return "Military (" + d.Military + ")"
}

func coordinateHref(d Details) string {
	query := url.Values{"f": {d.Format}, "v": {d.Token}}
	return coordinatePath + "?" + query.Encode()
}

func renderBody(code string, d Details, found bool) string {
	var b strings.Builder

	if !found {
		b.WriteString(`<p class="name">` + html.EscapeString(code) + `</p>`)
		b.WriteString(`<p class="note">This airfield code is not in this build's ` +
			`airfield database. The database is refreshed with the plugin, so a code ` +
			`that was recognized when the message was written may have been retired since.</p>`)
		return b.String()
	}

	name := d.Name
	if name == "" {
		name = d.Ident
	}

	b.WriteString(`<p class="name">` + html.EscapeString(name) + `</p>`)
	b.WriteString(`<p class="ident">` + html.EscapeString(d.Ident) + `</p>`)

	b.WriteString(`<table><tbody>`)
	for _, r := range detailRows(d) {
		if r.value == "" {
			continue
		}

		value := html.EscapeString(r.value)
		if r.href != "" {
			value = `<a href="` + html.EscapeString(r.href) + `">` + value + `</a>`
		}

		b.WriteString(`<tr><td>` + html.EscapeString(r.label) +
			`</td><td class="value">` + value + `</td></tr>`)
	}
	b.WriteString(`</tbody></table>`)

	if !d.HasPosition {
		b.WriteString(`<p class="note">This airfield has no position in the database, ` +
			`so there are no coordinate readings for it.</p>`)
	}

	renderRunways(&b, d.Runways)
	renderFrequencies(&b, d.Frequencies)

	return b.String()
}

func renderRunways(b *strings.Builder, runways []Runway) {
	if len(runways) == 0 {
		return
	}

	b.WriteString(`<h2>Runways</h2><table><tbody>`)
	for _, r := range runways {
		b.WriteString(`<tr><td>` + html.EscapeString(r.Designation) +
			`</td><td class="plain">` + html.EscapeString(RunwayLine(r)) + `</td></tr>`)
	}
	b.WriteString(`</tbody></table>`)
}

func renderFrequencies(b *strings.Builder, frequencies []Frequency) {
	if len(frequencies) == 0 {
		return
	}

	b.WriteString(`<h2>Frequencies</h2><table><tbody>`)
	for _, f := range frequencies {
		label := f.Type
		if f.Description != "" {
			label += " (" + f.Description + ")"
		}
		b.WriteString(`<tr><td>` + html.EscapeString(label) +
			`</td><td class="value">` + html.EscapeString(f.MHz) + `</td></tr>`)
	}
	b.WriteString(`</tbody></table>`)
}
