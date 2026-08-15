package location

import (
	"encoding/json"
	"html"
)

const (
	pageAppFromRoot     = "./public/app/page.js"
	pageAppFromDecorate = "../public/app/page.js"
)

const (
	pageModeLocation = "location"
	pageModeMap      = "map"
)

// renderRoot is the whole body of both standalone pages.
//
// The page carries no rendered readings of its own. It carries the link's
// already-validated parameters and the conversion this plugin would have served
// from /api/v1/convert to a reader with a session, and the bundle renders the
// same components the sidebar renders from them. That is what keeps one
// implementation of the table, the resolution rules and the map.
//
// Everything interpolated here is escaped. The page declares PageMapping, which
// admits same-origin script, so escaping is the only thing standing between the
// author's own text in `r` and execution.
func renderRoot(page pageData, mode string) string {
	loc := page.loc

	attrs := `<div id="root"` +
		` data-mode="` + html.EscapeString(mode) + `"` +
		` data-f="` + html.EscapeString(string(loc.Format)) + `"` +
		` data-v="` + html.EscapeString(loc.Canonical()) + `"`

	if page.raw != "" && page.raw != loc.Canonical() {
		attrs += ` data-r="` + html.EscapeString(page.raw) + `"`
	}

	// Through Convert, the same function /api/v1/convert calls, so a page and
	// the sidebar cannot come to disagree about what a token converts to.
	if conversion, ok := Convert(loc.Format, loc.Canonical(), page.raw); ok {
		if encoded, err := json.Marshal(conversion); err == nil {
			attrs += ` data-conversion="` + html.EscapeString(string(encoded)) + `"`
		}
	}

	return attrs + `></div>`
}
