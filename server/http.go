package main

import (
	"net/http"
	"net/url"
	"strings"

	"github.com/mattermost/mattermost/server/public/plugin"

	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/decorators"
	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/decorators/location"
	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/errcode"
)

// decoratePath is the route prefix for decorator pages, relative to the
// plugin's own base path.
const decoratePath = "/decorate"

// mapPath is the coordinate rendered as a page of its own, which is what
// "Open larger" in the sidebar opens.
//
// A sibling of /decorate rather than a mode of it: the decorator route answers
// "what is this token" and every page under it opens onto a table, where this
// answers "where is it" and gives the window to one picture. Authenticated and a
// pure function of its query string on the same terms.
const mapPath = "/map"

// ServeHTTP routes GET /decorate/<type> to the matching decorator's page, GET
// /map to the coordinate's own page, and everything under /api/v1 to the JSON
// API. Every one of them requires a session.
//
// The page routes were public until this gate was added, on the argument that
// the clients they exist for are the ones without a session: the mobile app
// opening a link in an in-app browser. Whether that is true of the in-app
// browser was never verified in either direction, and the cost of being wrong is
// bounded by redirecting rather than refusing, so a reader whose session is
// missing signs in and arrives where they were going.
//
// The pages stay pure functions of their query strings regardless. That is what
// keeps them testable and what keeps a page from growing a workspace lookup;
// this gate decides who may ask, not what the answer is built from.
func (p *Plugin) ServeHTTP(_ *plugin.Context, w http.ResponseWriter, r *http.Request) {
	w.Header().Set("X-Content-Type-Options", "nosniff")

	if strings.HasPrefix(r.URL.Path, apiPath+"/") {
		p.serveAPI(w, r)
		return
	}

	if r.Method != http.MethodGet {
		decorators.WriteError(w, http.StatusMethodNotAllowed,
			errcode.WithCode(errcode.HTTPMethodNotAllowed, "Method not allowed."))
		return
	}

	// After the method check, so a request that could never be resumed is
	// refused rather than sent to a login page it cannot come back from.
	if sessionUserID(r) == "" {
		p.redirectToLogin(w, r)
		return
	}

	if r.URL.Path == mapPath {
		location.RenderMapPage(w, r.URL.Query())
		return
	}

	typ, ok := strings.CutPrefix(r.URL.Path, decoratePath+"/")
	if !ok || typ == "" || strings.Contains(typ, "/") {
		decorators.WriteError(w, http.StatusNotFound,
			errcode.WithCode(errcode.HTTPDecoratePathInvalid, "Not found."))
		return
	}

	if p.decorators == nil {
		decorators.WriteError(w, http.StatusNotFound,
			errcode.WithCode(errcode.HTTPDecoratorsNotReady, "Not found."))
		return
	}

	decorator := p.decorators.Get(typ)
	if decorator == nil {
		decorators.WriteError(w, http.StatusNotFound,
			errcode.WithCode(errcode.HTTPDecoratorUnknown, "Not found."))
		return
	}

	decorator.RenderPage(w, r.URL.Query())
}

// sessionUserID is the reader Mattermost authenticated, or "" for no session.
//
// Mattermost sets this header itself and strips any copy a client tries to send,
// so an empty value means there is no session and a non-empty one is
// trustworthy. One function rather than one call site, so the two routes that
// ask can differ in what they do about the answer without differing in how they
// get it.
func sessionUserID(r *http.Request) string {
	return r.Header.Get("Mattermost-User-Id")
}

// redirectToLogin sends an unauthenticated reader to the server's login page and
// back again.
//
// A redirect rather than a refusal because the reader is almost always somebody
// whose session expired between reading a channel and tapping a link in it.
// Answering 401 would make them find their own way to a login and then back to a
// coordinate they no longer have the URL for.
//
// Root-relative, never absolute, for the same reason the decorated links
// themselves are: an absolute URL bakes in a hostname, and this one would bake it
// into the address bar of somebody who is mid-login.
//
// Mattermost strips /plugins/<id> before the plugin sees a request, so it has to
// go back on to build a URL a browser can return to. RequestURI carries the query
// string, which is the whole coordinate: without it the reader signs in and lands
// on a page that does not know what they clicked.
func (p *Plugin) redirectToLogin(w http.ResponseWriter, r *http.Request) {
	sitePath := siteURLPath(p.API.GetConfig())
	target := sitePath + "/login?redirect_to=" +
		url.QueryEscape(sitePath+"/plugins/"+manifest.Id+r.URL.RequestURI())

	// A cached redirect keeps bouncing a reader who has since signed in.
	w.Header().Set("Cache-Control", "no-store")
	http.Redirect(w, r, target, http.StatusFound)
}
