package location

import (
	"net/http"
	"net/url"

	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/decorators"
	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/errcode"
)

const mapPageTitle = "Map"

// The map page gives the whole window to one picture, so the shell's own
// centred column and its heading are out of the way. The heading stays in the
// document for assistive technology.
const mapPageStyles = `
body { padding: 0; overflow: hidden; }
main { max-width: none; margin: 0; }
h1 { position: absolute; width: 1px; height: 1px; overflow: hidden; clip: rect(0 0 0 0); margin: 0; }
`

// RenderMapPage serves the coordinate as a page that is nothing but the map.
//
// A route of its own rather than a mode of the readings page. The readings page
// answers "what is this coordinate" and opens onto a table; this answers "where
// is it" and gives the whole window to one picture, which is what a reader
// following "Open larger" from a 300px sidebar is asking for.
//
// A pure function of its query string, exactly like /decorate, and behind the
// same session gate. It validates through validateParams, the same gate the
// readings page uses, so a link one refuses cannot be rendered by the other.
//
// It reads no map configuration of its own: this page IS the map, so reaching it
// at all means ServeHTTP found the surface switched on and refused it otherwise.
func RenderMapPage(w http.ResponseWriter, params url.Values) {
	page, ok := validateParams(params)
	if !ok {
		decorators.WriteError(w, http.StatusBadRequest,
			errcode.WithCode(errcode.LocationPageParamsInvalid, "That is not a coordinate this plugin issued."))
		return
	}

	decorators.WritePage(w, decorators.Page{
		Title:      mapPageTitle,
		Theme:      decorators.ThemeFromParams(params),
		StyleCSS:   pageStyles + mapPageStyles,
		BodyHTML:   renderRoot(page, pageModeMap, Maps{Page: true}),
		ScriptSrc:  pageAppFromRoot,
		Capability: decorators.PageMapping,
	})
}
