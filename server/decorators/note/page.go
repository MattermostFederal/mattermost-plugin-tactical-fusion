package note

import "html"

const pageTitle = "Note"

const pageStyles = `
.source { font-family: ui-monospace, SFMono-Regular, Menlo, monospace; font-size: 13px;
  white-space: pre-wrap; overflow-wrap: anywhere; margin: 0; }
`

func renderBody(markdown string) string {
	return `<pre class="source">` + html.EscapeString(markdown) + `</pre>`
}
