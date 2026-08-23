package main

import (
	"fmt"
	"net/http"
	"net/url"
	"os"
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

/*
 * packagesPath serves one detail map package's archive.
 *
 * Deliberately OUTSIDE the session gate below, and a sibling of it rather than
 * a route under /decorate. Two reasons, and the second is the operational one.
 *
 * It carries the same posture as the bundle's own public/ directory, which
 * Mattermost serves without a session and which world.pmtiles is fetched from
 * today: a basemap is not reader-specific and there is nothing in one to
 * protect. And MapLibre fetches tiles from a worker through the pmtiles
 * protocol, so a route that could redirect to a login would answer a tile
 * request with an HTML page, which the reader would see as a map that half
 * drew.
 */
const packagesPath = "/packages"

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

	if strings.HasPrefix(r.URL.Path, packagesPath+"/") {
		p.servePackage(w, r)
		return
	}

	// After the method check, so a request that could never be resumed is
	// refused rather than sent to a login page it cannot come back from.
	if sessionUserID(r) == "" {
		p.redirectToLogin(w, r)
		return
	}

	if r.URL.Path == mapPath {
		// The one route an admin switch can remove outright, because this page is
		// the map: with nothing to draw there is no reduced version of it worth
		// serving, unlike /decorate/<type>, which still has every reading.
		if !p.locationMaps().Page {
			decorators.WriteError(w, http.StatusNotFound,
				errcode.WithCode(errcode.HTTPMapDisabled, "Not found."))
			return
		}

		location.RenderMapPage(w, r.URL.Query(), p.packageNames())
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

/*
 * Serves one detail package's archive, by byte range.
 *
 * http.ServeContent is what makes this viable at all: it answers Range from an
 * open file without reading the whole thing, where plugin.API.ReadFile would
 * pull an entire archive into memory for every tile a reader scrolls past.
 *
 * The name is whitelisted by mapPackage before it ever becomes a path, so no
 * traversal reaches here to be cleaned; a name that is not a package is a 404
 * rather than a 403, because which areas an install has is not a secret and
 * distinguishing the two would only tell a prober more.
 */
func (p *Plugin) servePackage(w http.ResponseWriter, r *http.Request) {
	name, ok := strings.CutPrefix(r.URL.Path, packagesPath+"/")
	if !ok {
		decorators.WriteError(w, http.StatusNotFound,
			errcode.WithCode(errcode.HTTPPackagePathInvalid, "Not found."))
		return
	}

	name, ok = strings.CutSuffix(name, packageSuffix)
	if !ok || strings.Contains(name, "/") {
		decorators.WriteError(w, http.StatusNotFound,
			errcode.WithCode(errcode.HTTPPackagePathInvalid, "Not found."))
		return
	}

	pkg, ok := p.mapPackage(name)
	if !ok {
		decorators.WriteError(w, http.StatusNotFound,
			errcode.WithCode(errcode.HTTPPackageUnknown, "Not found."))
		return
	}

	file, err := os.Open(pkg.path) // #nosec G304 -- a path built from a whitelisted name
	if err != nil {
		p.API.LogWarn("a discovered map package could not be opened",
			"error_code", errcode.HTTPPackageUnreadable, "file", pkg.path, "error", err.Error())
		decorators.WriteError(w, http.StatusNotFound,
			errcode.WithCode(errcode.HTTPPackageUnreadable, "Not found."))
		return
	}
	defer func() { _ = file.Close() }()

	info, err := file.Stat()
	if err != nil {
		decorators.WriteError(w, http.StatusNotFound,
			errcode.WithCode(errcode.HTTPPackageUnreadable, "Not found."))
		return
	}

	/*
	 * An ETag, because a package is NOT immutable.
	 *
	 * The upload route and the documented drop-in workflow both replace an
	 * archive in place, under a URL that carries only the plugin version and so
	 * does not change with it. PMTiles is read by byte offsets taken from a
	 * directory the client read earlier, so a reader whose tab is holding those
	 * offsets against a file that has since been replaced reads the new bytes at
	 * the old positions, which is garbage rather than an error. pmtiles.js
	 * watches the ETag across its own requests for exactly this and re-reads the
	 * directory when it moves; with no ETag it has nothing to compare and the
	 * recovery it already implements never fires.
	 *
	 * Size and modification time rather than a digest of the contents: this runs
	 * per tile request, and hashing a 500 MB archive to answer one is not a
	 * trade worth making. The pair moves whenever a file is replaced by any
	 * means an operator has, which is what it has to catch.
	 */
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("ETag", fmt.Sprintf("%q", fmt.Sprintf("%x-%x", info.Size(), info.ModTime().UnixNano())))

	// Bounded to the same minute the client caches the package list for, rather
	// than the five it was. A replaced archive is stale for at most that long,
	// and the ETag makes the revalidation at the end of it a 304 rather than a
	// re-download.
	w.Header().Set("Cache-Control", "private, max-age=60")
	http.ServeContent(w, r, pkg.Name+packageSuffix, info.ModTime(), file)
}
