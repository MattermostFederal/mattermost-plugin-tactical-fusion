package airport

import (
	"html"
	"net/url"
	"strings"
)

// pageTitle is the browser tab name and the page heading. A category rather
// than the value, matching the sidebar header and the other decorator pages.
const pageTitle = "Airfield"

const pageStyles = `
.ident { font-family: ui-monospace, SFMono-Regular, Menlo, monospace; font-size: 13px;
  color: var(--muted); margin: 0; letter-spacing: .08em; }
.name { font-size: 22px; font-weight: 600; margin: 0 0 6px; }
td.value { font-family: ui-monospace, SFMono-Regular, Menlo, monospace; text-align: right;
  word-break: break-all; }
td.value a { color: var(--accent); }
`

// coordinatePath is the location decorator's page, which is a sibling of this
// one: both are served from /decorate/, so a bare relative reference resolves
// without this page knowing the install's subpath. The page renderers are pure
// functions of a query string and cannot see SiteURL, which is the same reason
// ScriptSrc has to be relative.
const coordinatePath = "location"

// row is one line of the rendered airfield.
//
// The page carries no copy controls, because it has no script to hang a
// delegated listener on and this page deliberately stays under PageStatic. The
// values are still selectable.
//
// href turns the VALUE into a link rather than adding a line below the table.
// The place is where the airfield is, so the place is the thing to follow to
// see where that is, and the panel does exactly the same.
type row struct {
	label string
	value string
	href  string
}

// detailRows is the airfield and nothing else.
//
// The coordinate readings are deliberately absent. An airfield resolves to a
// position, and the location decorator already renders that position eleven
// ways with a map, copy buttons and the reader's own hidden-row choices;
// reprinting those rows here would be a second table saying the same thing,
// worse, and it would drift. The place links to the one that already exists.
func detailRows(d Details) []row {
	place := row{label: "Place", value: d.Place}
	if d.HasPosition {
		place.href = coordinateHref(d)
	}

	return []row{
		{label: "Code", value: d.Ident},
		place,
		{label: "Type", value: d.Type},
		{label: "Elevation", value: d.Elevation},
		{label: "IATA", value: d.IATA},
	}
}

// coordinateHref points at the location page for this airfield's position.
//
// Relative, so it works on a subpath install, and built from the same (format,
// token) pair the panel hands to its own location view, so the two surfaces
// cannot send a reader to different coordinates. Labeled with that page's own
// title for the same reason the panel's link is.
func coordinateHref(d Details) string {
	query := url.Values{"f": {d.Format}, "v": {d.Token}}
	return coordinatePath + "?" + query.Encode()
}

// renderBody is the whole page.
//
// Everything interpolated is escaped. The page declares PageStatic and carries
// no script at all, so script-src is 'none' and an escaping mistake cannot
// become a running script however it is spelled. The escaping is still what
// stops it becoming markup.
func renderBody(ident string, d Details, found bool) string {
	var b strings.Builder

	if !found {
		b.WriteString(`<p class="name">` + html.EscapeString(ident) + `</p>`)
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

	return b.String()
}
