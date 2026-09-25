package cyber

import (
	"html"
	"net/url"
	"strconv"
	"strings"
)

const pageStyles = `
.value-id { font-family: ui-monospace, SFMono-Regular, Menlo, monospace; font-size: 13px;
  color: var(--muted); margin: 0; letter-spacing: .04em; word-break: break-all; }
.name { font-size: 22px; font-weight: 600; margin: 0 0 6px; }
.summary { margin: 12px 0 0; }
td.value { font-family: ui-monospace, SFMono-Regular, Menlo, monospace; text-align: right;
  word-break: break-all; }
td.value a { color: var(--accent); }
h2 { font-size: 13px; text-transform: uppercase; letter-spacing: .08em; color: var(--muted);
  margin: 22px 0 8px; }
ul.links { list-style: none; padding: 0; margin: 0; }
ul.links li { margin: 0 0 6px; }
ul.links a { color: var(--accent); }
ul.datasets { list-style: none; padding: 0; margin: 0; font-size: 13px; color: var(--muted); }
ul.datasets li { margin: 0 0 4px; }
.verdict { font-weight: 600; }
table.vector td.value { font-family: inherit; }
table.vector .severe, ul.lines .severe { color: var(--urgent); font-weight: 600; }
details { margin: 22px 0 0; }
summary { font-size: 13px; text-transform: uppercase; letter-spacing: .08em; color: var(--muted);
  cursor: pointer; }
ul.lines { padding-left: 18px; margin: 8px 0 0; }
ul.lines li { margin: 0 0 4px; overflow-wrap: anywhere; }
ul.lines a { color: var(--accent); }
.tags { color: var(--muted); font-size: 12px; }
`

const selfPath = "cyber"

func linkHref(link Link) string {
	query := url.Values{ParamKind: {string(link.Kind)}, ParamValue: {link.Value}}
	return selfPath + "?" + query.Encode()
}

func renderBody(d Details) string {
	var b strings.Builder

	b.WriteString(`<p class="name">` + html.EscapeString(d.Title) + `</p>`)
	if d.Title != d.Value {
		b.WriteString(`<p class="value-id">` + html.EscapeString(d.Value) + `</p>`)
	}

	if d.Summary != "" {
		b.WriteString(`<p class="summary">` + html.EscapeString(d.Summary) + `</p>`)
	}

	if len(d.Rows) > 0 {
		b.WriteString(`<table><tbody>`)
		for _, row := range d.Rows {
			b.WriteString(`<tr><td>` + html.EscapeString(row.Label) +
				`</td><td class="value">` + html.EscapeString(row.Value) + `</td></tr>`)
		}
		b.WriteString(`</tbody></table>`)
	}

	writeReports(&b, d.Reports)
	writeVector(&b, d.Vector)

	if d.Status != "" {
		b.WriteString(`<p class="note">` + html.EscapeString(d.Status) + `</p>`)
	}

	writeWatchlist(&b, d)
	writeRelated(&b, d)
	writeLines(&b, "Affected, as reported", d.Affected)
	writeLines(&b, "Affected, per NVD", d.Configurations)
	writeReferences(&b, d.References)
	writeSections(&b, d.Sections)
	writeCredits(&b, d.Credits)
	writeSources(&b, d.Freshness)
	writeDatasets(&b, d)

	return b.String()
}

func writeWatchlist(b *strings.Builder, d Details) {
	if len(d.Watchlist) == 0 {
		return
	}

	b.WriteString(`<h2>Watchlist</h2><table><tbody>`)
	for _, entry := range d.Watchlist {
		label := entry.Source
		if label == "" {
			label = "Entry"
		}

		value := `<span class="verdict">` + html.EscapeString(entry.Verdict) + `</span>`
		if entry.Note != "" {
			value += ` ` + html.EscapeString(entry.Note)
		}
		if entry.Updated != "" {
			value += ` (` + html.EscapeString(entry.Updated) + `)`
		}

		b.WriteString(`<tr><td>` + html.EscapeString(label) + `</td><td class="value">` + value + `</td></tr>`)
	}
	b.WriteString(`</tbody></table>`)
}

func writeReports(b *strings.Builder, reports []Report) {
	if len(reports) == 0 {
		return
	}

	b.WriteString(`<h2>Threat reports</h2><ul class="lines">`)
	for _, report := range reports {
		source := html.EscapeString(report.Source)
		if report.URL != "" {
			source = `<a href="` + html.EscapeString(report.URL) + `" rel="noopener noreferrer" target="_blank">` + source + `</a>`
		}
		class := "context"
		if report.Malicious {
			class = "severe"
		}
		b.WriteString(`<li><strong>` + source + `</strong>: <span class="` + class + `">` + html.EscapeString(report.Threat) + `</span>`)
		if report.Detail != "" {
			b.WriteString(`<br>` + html.EscapeString(report.Detail))
		}
		b.WriteString(`</li>`)
	}
	b.WriteString(`</ul>`)
}

func writeVector(b *strings.Builder, metrics []VectorMetric) {
	if len(metrics) == 0 {
		return
	}

	b.WriteString(`<h2>Vector, decoded</h2><table class="vector"><tbody>`)
	for _, metric := range metrics {
		value := html.EscapeString(metric.Value)
		if metric.Severe {
			value = `<span class="severe">` + value + `</span>`
		}
		b.WriteString(`<tr><td>` + html.EscapeString(metric.Metric) + `</td><td class="value">` + value + `</td></tr>`)
	}
	b.WriteString(`</tbody></table>`)
}

func writeRelated(b *strings.Builder, d Details) {
	if len(d.Related) == 0 {
		return
	}

	b.WriteString(`<h2>Related</h2><ul class="links">`)
	for _, link := range d.Related {
		b.WriteString(`<li><a href="` + html.EscapeString(linkHref(link)) + `">` +
			html.EscapeString(link.Label) + `</a></li>`)
	}
	b.WriteString(`</ul>`)
}

func writeLines(b *strings.Builder, title string, lines []string) {
	if len(lines) == 0 {
		return
	}

	b.WriteString(`<details><summary>` + html.EscapeString(title) + ` (` + strconv.Itoa(len(lines)) + `)</summary><ul class="lines">`)
	for _, line := range lines {
		b.WriteString(`<li>` + html.EscapeString(line) + `</li>`)
	}
	b.WriteString(`</ul></details>`)
}

func writeCredits(b *strings.Builder, credits []Credit) {
	for _, credit := range credits {
		text := html.EscapeString(credit.Text)
		if credit.URL != "" {
			text = `<a href="` + html.EscapeString(credit.URL) + `" rel="noopener noreferrer" target="_blank">` + text + `</a>`
		}
		b.WriteString(`<p class="note credit">` + text + `</p>`)
	}
}

func writeSections(b *strings.Builder, sections []Section) {
	for _, section := range sections {
		b.WriteString(`<details><summary>` + html.EscapeString(section.Title) + ` (` + strconv.Itoa(len(section.Items)) + `)</summary><ul class="lines">`)
		for _, item := range section.Items {
			b.WriteString(`<li>`)
			switch {
			case item.Link != nil:
				b.WriteString(`<a href="` + html.EscapeString(linkHref(*item.Link)) + `"><strong>` + html.EscapeString(item.Head) + `</strong></a>`)
			case item.URL != "":
				b.WriteString(`<a href="` + html.EscapeString(item.URL) + `" rel="noopener noreferrer" target="_blank"><strong>` + html.EscapeString(item.Head) + `</strong></a>`)
			case item.Head != "":
				b.WriteString(`<strong>` + html.EscapeString(item.Head) + `</strong>`)
			}
			if item.Text != "" {
				if item.Head != "" {
					b.WriteString(`<br>`)
				}
				b.WriteString(strings.ReplaceAll(html.EscapeString(item.Text), "\n", "<br>"))
			}
			b.WriteString(`</li>`)
		}
		b.WriteString(`</ul></details>`)
	}
}

func writeReferences(b *strings.Builder, refs []Reference) {
	if len(refs) == 0 {
		return
	}

	b.WriteString(`<details><summary>References (` + strconv.Itoa(len(refs)) + `)</summary><ul class="lines">`)
	for _, ref := range refs {
		b.WriteString(`<li><a href="` + html.EscapeString(ref.URL) + `" rel="noopener noreferrer" target="_blank">` +
			html.EscapeString(ref.URL) + `</a>`)
		if ref.Tags != "" {
			b.WriteString(` <span class="tags">` + html.EscapeString(ref.Tags) + `</span>`)
		}
		b.WriteString(`</li>`)
	}
	b.WriteString(`</ul></details>`)
}

func writeDatasets(b *strings.Builder, d Details) {
	if len(d.Datasets) == 0 {
		return
	}

	b.WriteString(`<h2>Datasets</h2><ul class="datasets">`)
	for _, dataset := range d.Datasets {
		line := dataset.Label
		if dataset.Present {
			line += ": installed"
			if dataset.Generated != "" {
				line += ", generated " + dataset.Generated
			}
		} else {
			line += ": not installed"
		}

		b.WriteString(`<li>` + html.EscapeString(line) + `</li>`)
	}
	b.WriteString(`</ul>`)
}

func writeSources(b *strings.Builder, sources []Freshness) {
	if len(sources) == 0 {
		return
	}
	b.WriteString(`<h2>Data sources</h2><table class="sources"><tbody>`)
	for _, source := range sources {
		b.WriteString(`<tr><td>` + html.EscapeString(source.Label) + `</td><td class="value">` +
			html.EscapeString(source.File) + `, compiled ` + html.EscapeString(CompiledText(source.Compiled)) + `</td></tr>`)
	}
	b.WriteString(`</tbody></table>`)
}
