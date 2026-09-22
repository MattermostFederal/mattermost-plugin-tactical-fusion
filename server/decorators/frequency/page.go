package frequency

import (
	"html"
	"strings"
)

const pageTitle = "Frequency"

const pageStyles = `
.token { font-family: ui-monospace, SFMono-Regular, Menlo, monospace; font-size: 22px;
  font-weight: 600; margin: 0 0 6px; }
.band { font-size: 15px; margin: 0 0 14px; }
td.value { font-family: ui-monospace, SFMono-Regular, Menlo, monospace; text-align: right; }
`

func renderBody(d Details) string {
	var b strings.Builder

	b.WriteString(`<p class="token">` + html.EscapeString(d.Token) + `</p>`)
	b.WriteString(`<p class="band">` + html.EscapeString(d.Band) + `</p>`)

	b.WriteString(`<table><tbody>`)
	for _, r := range [][2]string{
		{"MHz", d.MHz},
		{"kHz", d.KHz},
		{"Channel", d.Channel},
		{"Use", d.Use},
	} {
		if r[1] == "" {
			continue
		}
		b.WriteString(`<tr><td>` + html.EscapeString(r[0]) + `</td><td class="value">` + html.EscapeString(r[1]) + `</td></tr>`)
	}
	b.WriteString(`</tbody></table>`)

	return b.String()
}
