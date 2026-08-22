package location

import (
	"encoding/json"
	"html"
	"strings"
)

const (
	pageAppFromRoot     = "./public/app/page.js"
	pageAppFromDecorate = "../public/app/page.js"
)

const (
	pageModeLocation = "location"
	pageModeMap      = "map"
)

// MapSurfacesAttr is the shell attribute naming the surfaces that may draw a
// map, and SurfacePanel and SurfacePage are the names it can carry.
//
// Exported so webapp_sync_test.go can hold them against the webapp's reader
// rather than against a second copy written into a Go test. All three are the
// quiet kind of contract: rename one here and the standalone pages stop drawing
// a map, or start drawing one on an install that switched them off, with nothing
// logged on either side. Renaming the ATTRIBUTE is the worse of the two, because
// an absent attribute deliberately reads as every surface on.
const (
	MapSurfacesAttr = "data-maps"

	SurfacePanel = "panel"
	SurfacePage  = "page"

	mapSurfaceSeparator = ","

	// PackagesAttr carries the detail map packages this install has, by name.
	//
	// The pages need it in the shell rather than from /api/v1/packages because
	// they have no session and that route requires one. Written unconditionally,
	// empty included, for the same reason MapSurfacesAttr is: an absent
	// attribute has to mean "a shell older than this bundle" and nothing else.
	PackagesAttr = "data-packages"

	packageSeparator = ","
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
func renderRoot(page pageData, mode string, maps Maps, packages []string) string {
	loc := page.loc

	attrs := `<div id="root"` +
		` data-mode="` + html.EscapeString(mode) + `"` +
		` ` + MapSurfacesAttr + `="` + html.EscapeString(mapSurfaces(maps)) + `"` +
		` ` + PackagesAttr + `="` + html.EscapeString(strings.Join(packages, packageSeparator)) + `"` +
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

// mapSurfaces is the comma-separated list of surfaces that may draw a map.
//
// Always written, empty included, so the bundle can tell "the admin turned maps
// off" from "this shell predates the attribute" without either reading as the
// other. The bundle matches these against a closed set and ignores anything
// else, so adding a surface here cannot make an older bundle draw one it does
// not know.
func mapSurfaces(maps Maps) string {
	var on []string
	if maps.Panel {
		on = append(on, SurfacePanel)
	}
	if maps.Page {
		on = append(on, SurfacePage)
	}

	return strings.Join(on, mapSurfaceSeparator)
}
