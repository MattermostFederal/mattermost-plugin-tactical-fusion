package main

import (
	"net/http"
	"strings"

	"github.com/mattermost/mattermost/server/public/plugin"

	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/decorators"
)

// decoratePath is the route prefix for decorator pages, relative to the
// plugin's own base path.
const decoratePath = "/decorate"

// ServeHTTP routes GET /decorate/<type> to the matching decorator's page, and
// everything under /api/v1 to the authenticated JSON API.
//
// The decorator route is intentionally PUBLIC. Mattermost sets
// Mattermost-User-Id only for requests carrying a valid session, and the
// clients this page exists for are precisely the ones without one: the mobile
// app opening a link in an in-app browser. Requiring auth would break the
// feature it was added for.
//
// That is only safe because a decorator page is a pure function of its query
// string. See the Decorator interface documentation before adding a page that
// needs workspace data. Anything that reads per-reader state belongs under
// /api/v1 instead, which refuses a request without a session.
func (p *Plugin) ServeHTTP(_ *plugin.Context, w http.ResponseWriter, r *http.Request) {
	w.Header().Set("X-Content-Type-Options", "nosniff")

	if strings.HasPrefix(r.URL.Path, apiPath+"/") {
		p.serveAPI(w, r)
		return
	}

	if r.Method != http.MethodGet {
		decorators.WriteError(w, http.StatusMethodNotAllowed, "Method not allowed.")
		return
	}

	typ, ok := strings.CutPrefix(r.URL.Path, decoratePath+"/")
	if !ok || typ == "" || strings.Contains(typ, "/") {
		decorators.WriteError(w, http.StatusNotFound, "Not found.")
		return
	}

	if p.decorators == nil {
		decorators.WriteError(w, http.StatusNotFound, "Not found.")
		return
	}

	decorator := p.decorators.Get(typ)
	if decorator == nil {
		decorators.WriteError(w, http.StatusNotFound, "Not found.")
		return
	}

	decorator.RenderPage(w, r.URL.Query())
}
