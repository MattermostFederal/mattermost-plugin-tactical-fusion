package note

import "html"

const pageTitle = "Note"

const provenance = "Written by whoever made this link. Tactical Fusion has not checked it."

const pageStyles = `
.provenance { font-size: 13px; margin: 0 0 12px; padding: 8px 12px;
  border-left: 4px solid #d24b4e; background: rgba(210, 75, 78, 0.08); }
.source { font-family: ui-monospace, SFMono-Regular, Menlo, monospace; font-size: 13px;
  white-space: pre-wrap; overflow-wrap: anywhere; margin: 0; }
`

func renderBody(markdown string) string {
	return `<p class="provenance">` + provenance + `</p><pre class="source">` + html.EscapeString(markdown) + `</pre>`
}
