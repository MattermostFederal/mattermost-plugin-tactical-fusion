package avreport

import (
	"html"
	"net/url"
	"strings"
)

const pageTitle = "Aviation report"

const pageStyles = `
.kind { font-family: ui-monospace, SFMono-Regular, Menlo, monospace; font-size: 13px;
  color: var(--muted); margin: 0; letter-spacing: .08em; }
.name { font-size: 22px; font-weight: 600; margin: 0 0 6px; }
.summary { font-size: 15px; margin: 0 0 14px; }
pre { white-space: pre-wrap; word-break: break-word; font-size: 13px; padding: 10px;
  border: 1px solid var(--muted); border-radius: 4px; }
h2 { font-size: 13px; text-transform: uppercase; letter-spacing: .04em; margin: 18px 0 4px;
  color: var(--muted); }
td.value { text-align: right; }
td.value a { color: var(--accent); }
`

const airfieldPath = "airport"

const coordinatePath = "location"

func renderBody(report Report) string {
	var b strings.Builder

	heading := report.Kind
	if report.Station != "" {
		heading += " " + report.Station
	}
	b.WriteString(`<p class="name">` + html.EscapeString(heading) + `</p>`)

	if report.StationName != "" {
		href := airfieldPath + "?" + url.Values{"v": {report.Station}}.Encode()
		b.WriteString(`<p class="kind"><a href="` + html.EscapeString(href) + `">` + html.EscapeString(report.StationName) + `</a></p>`)
	}
	if report.Summary != "" {
		b.WriteString(`<p class="summary">` + html.EscapeString(report.Summary) + `</p>`)
	}

	b.WriteString(`<pre>` + html.EscapeString(report.Raw) + `</pre>`)

	b.WriteString(`<table><tbody>`)
	if issued := zuluText(report.IssuedAt); issued != "" {
		label := "Issued"
		if report.Kind == KindNOTAM {
			label = "Effective"
		}
		value := issued
		if report.Inferred {
			value += " (month and year taken from the post date)"
		}
		writeRow(&b, label, value, "")
	}
	if len(report.Flags) > 0 {
		writeRow(&b, "Flags", strings.Join(report.Flags, ", "), "")
	}
	for _, row := range report.Rows {
		if row.Label == "Effective" && report.Kind == KindNOTAM {
			continue
		}
		writeRow(&b, row.Label, row.Value, "")
	}
	if report.Format != "" {
		href := coordinatePath + "?" + url.Values{"f": {report.Format}, "v": {report.Value}}.Encode()
		writeRow(&b, "Position", report.Value, href)
	}
	b.WriteString(`</tbody></table>`)

	for _, period := range report.Periods {
		b.WriteString(`<h2>` + html.EscapeString(period.Period) + `</h2><table><tbody>`)
		for _, row := range period.Rows {
			writeRow(&b, row.Label, row.Value, "")
		}
		b.WriteString(`</tbody></table>`)
	}

	if len(report.Remarks) > 0 {
		b.WriteString(`<h2>Remarks</h2><table><tbody>`)
		for _, row := range report.Remarks {
			writeRow(&b, row.Label, row.Value, "")
		}
		b.WriteString(`</tbody></table>`)
	}

	if len(report.Unknown) > 0 {
		b.WriteString(`<h2>Not decoded</h2><p class="note">` + html.EscapeString(strings.Join(report.Unknown, " ")) + `</p>`)
	}

	return b.String()
}

func writeRow(b *strings.Builder, label, value, href string) {
	escaped := html.EscapeString(value)
	if href != "" {
		escaped = `<a href="` + html.EscapeString(href) + `">` + escaped + `</a>`
	}
	b.WriteString(`<tr><td>` + html.EscapeString(label) + `</td><td class="value">` + escaped + `</td></tr>`)
}
