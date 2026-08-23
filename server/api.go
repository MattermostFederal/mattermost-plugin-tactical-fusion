package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/mattermost/mattermost/server/public/model"

	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/decorators/airport"
	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/decorators/location"
	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/errcode"
)

// apiPath is the route prefix for the authenticated JSON API, relative to the
// plugin's own base path.
//
// Deliberately a sibling of decoratePath rather than a branch inside it. Both
// require a session now, but they answer a missing one differently, and
// everything under here reads per-reader state while a decorator page reads
// none. Keeping them apart means neither can inherit the other's rules by
// accident.
const apiPath = "/api/v1"

// The resources this API has.
const (
	preferencesPath = apiPath + "/preferences"

	// convertPath turns a coordinate into every other notation.
	//
	// It exists because MGRS and UTM need a projection, the projection lives in
	// Go, and the sidebar panel is TypeScript. Everything the panel can work
	// out from the token by itself it still does, so this fills in the grid
	// rows and nothing else for most links; only a grid link depends on it for
	// its position.
	//
	// Authenticated, like everything under here, even though it reads no
	// workspace data. A caller with no session and a coordinate to convert is
	// not a reader of this workspace.
	convertPath = apiPath + "/convert"

	// featuresPath is which optional surfaces the admin has left on.
	//
	// The only channel from plugin configuration to the webapp. Mattermost hands
	// plugin settings to system admins alone, so a reader's browser has no other
	// way to learn that maps are off, and drawing one anyway would pull the
	// basemap archive on exactly the installs the switch exists for.
	//
	// It answers per surface rather than per setting, so the parent AND stays in
	// Go beside locationFormats and the webapp cannot come to a different
	// conclusion from the same switches. Nothing secret is on it: it says which
	// features are on, never how anything is configured.
	featuresPath = apiPath + "/features"

	// airportPath answers with one airfield and every reading of its position.
	//
	// It exists because the airfield database is compiled into the Go binary
	// and there is no way to carry a name to the browser through the link: the
	// link's stored text has to stay the author's own token.
	airportPath = apiPath + "/airport"

	// packagesPathAPI lists the detail map packages this install has, which is
	// what lets the panel add one vector source per covered area. Names only:
	// the client builds the archive URL the same way it builds the global one,
	// so no URL crosses the wire to be got wrong.
	packagesPathAPI = apiPath + "/packages"
)

// airportResponse is what the airfield endpoint answers.
//
// A discriminated shape rather than a flat record. An ident this build does not
// hold carries no airfield and no coordinate at all, rather than zero values a
// reader would take for real: 0,0 is a position like any other, and this plugin
// deliberately does not inherit the truthiness check that drops it.
//
// It carries the coordinate as an (f, v) PAIR rather than as readings. An
// airfield surface names the field and links to the coordinate, because the
// location decorator already renders that position eleven ways with a map and
// the reader's own row choices; a second table here would say the same thing
// worse and then drift from it.
//
// Held to webapp/src/decorators/airport/types.ts by TestWebappAirportShapeMatches.
type airportResponse struct {
	Found      bool               `json:"found"`
	Ident      string             `json:"ident"`
	Airport    *airportDetails    `json:"airport,omitempty"`
	Coordinate *airportCoordinate `json:"coordinate,omitempty"`
}

// airportCoordinate is the location decorator's own link identity.
//
// Built in Go rather than in the webapp because the two languages disagree
// about formatting a float in the last digit, and a token that differs by one
// digit is refused by the very page it points at.
type airportCoordinate struct {
	Format string `json:"format"`
	Value  string `json:"value"`

	// Region reaches the map's accessible label and nothing else. It is the
	// only place the country reaches a reader who is not looking at the map.
	Region string `json:"region"`
}

// airportDetails is the airfield itself, already rendered.
//
// Strings, for the reason Conversion is strings: the rendering rules live in Go
// and a second implementation of them in TypeScript is a second thing to get
// wrong. Elevation is a string for the same reason and is empty when the
// database states none, which is a different thing from sea level.
//
// It carries no citation for the airfield data. OurAirports is public domain,
// so nothing has to travel with the value, and the same judgment was already
// made for the Natural Earth basemap credit. The provenance is recorded where
// somebody would go looking for it, in server/decorators/airport/data/README.md
// and the help pages. The REGION's own citation is a different thing and stays:
// that says where a border lookup came from, and it is what keeps it from
// reading as a determination.
type airportDetails struct {
	Name      string `json:"name"`
	Type      string `json:"type"`
	Place     string `json:"place"`
	Elevation string `json:"elevation"`
	IATA      string `json:"iata"`
}

// featuresResponse is what a reader's browser is told about this install.
//
// Held to webapp/src/features/types.ts by TestWebappFeatureShapeMatches, on
// names, types and order, for the reason the Conversion shape is: a TypeScript
// field that silently reads undefined is a map that silently stops drawing.
type featuresResponse struct {
	MapPanel  bool `json:"map_panel"`
	MapInline bool `json:"map_inline"`
	MapPage   bool `json:"map_page"`
}

// maxPreferencesBody caps a submitted blob. The largest legitimate one is a few
// hundred bytes, so this is only here to keep a hostile client from making the
// decoder do unbounded work.
const maxPreferencesBody = 8 * 1024

// serveAPI handles the JSON API routes.
//
// It refuses a request without a session rather than redirecting it, unlike the
// page routes: these callers are fetch, and they want a status they can branch
// on, not a login page to render into a table cell.
func (p *Plugin) serveAPI(w http.ResponseWriter, r *http.Request) {
	userID := sessionUserID(r)
	if userID == "" {
		writeAPIError(w, http.StatusUnauthorized,
			errcode.WithCode(errcode.APINotAuthorized, "Not authorized."))
		return
	}

	if r.URL.Path == convertPath {
		p.serveConvert(w, r)
		return
	}

	if r.URL.Path == featuresPath {
		p.serveFeatures(w, r)
		return
	}

	if r.URL.Path == airportPath {
		p.serveAirport(w, r)
		return
	}

	if r.URL.Path == packagesPathAPI {
		p.servePackageList(w, r)
		return
	}

	if strings.HasPrefix(r.URL.Path, packagesPathAPI+"/") {
		p.servePackageAdmin(w, r, userID)
		return
	}

	if r.URL.Path != preferencesPath {
		writeAPIError(w, http.StatusNotFound,
			errcode.WithCode(errcode.APINotFound, "Not found."))
		return
	}

	if p.preferences == nil {
		// Only reachable if a request lands between activation and OnActivate
		// finishing. Nothing to serve yet, and nothing worth failing over.
		writeAPIError(w, http.StatusServiceUnavailable,
			errcode.WithCode(errcode.APINotReady, "Not ready."))
		return
	}

	// These are per-reader and change the moment they are saved, so no shared
	// cache anywhere between here and the browser may hold on to them.
	w.Header().Set("Cache-Control", "no-store")

	switch r.Method {
	case http.MethodGet:
		p.handleGetPreferences(w, userID)
	case http.MethodPut:
		p.handleSetPreferences(w, r, userID)
	case http.MethodDelete:
		p.handleDeletePreferences(w, userID)
	default:
		writeAPIError(w, http.StatusMethodNotAllowed,
			errcode.WithCode(errcode.APIMethodNotAllowed, "Method not allowed."))
	}
}

// serveConvert answers with every derived reading of one coordinate.
//
// A pure function of its two parameters, with no store behind it and no reader
// to consult, which is why it needs no cache of its own: the work is a
// projection costing microseconds. The preferences cache exists because there
// is a KV round trip to avoid, and that precedent does not transfer. What it
// does set is an HTTP caching header, below, which is a different thing: the
// answer for a given token never changes, so the browser may keep it.
func (p *Plugin) serveConvert(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeAPIError(w, http.StatusMethodNotAllowed,
			errcode.WithCode(errcode.APIMethodNotAllowed, "Method not allowed."))
		return
	}

	query := r.URL.Query()

	// All three parameters, so this applies exactly the checks the page
	// applies. A conversion that accepted a link the page rejects would let the
	// sidebar render a coordinate the page refuses to.
	conversion, ok := location.Convert(
		location.Format(query.Get("f")), query.Get("v"), query.Get("r"))
	if !ok {
		writeAPIError(w, http.StatusBadRequest,
			errcode.WithCode(errcode.APIConvertInvalid, "That is not a coordinate this plugin issued."))
		return
	}

	// The same answer forever for the same pair, so it is worth caching, and
	// private because the URL and the body both carry a position. A shared
	// cache holding one is a leak rather than an optimisation.
	w.Header().Set("Cache-Control", "private, max-age=300")

	writeAPIJSON(w, http.StatusOK, conversion)
}

// serveFeatures answers which optional surfaces this install has left on.
//
// no-store, like preferences and unlike convert: an admin can change this at any
// moment, and a coordinate's conversion is true forever where this is only true
// now. The webapp's own cache is what stops a channel full of links asking
// repeatedly; a shared cache in between would leave a reader on an answer no
// reload could correct.
func (p *Plugin) serveFeatures(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeAPIError(w, http.StatusMethodNotAllowed,
			errcode.WithCode(errcode.APIMethodNotAllowed, "Method not allowed."))
		return
	}

	// An unloaded configuration is every switch false, which here is
	// indistinguishable from an admin turning everything off, and the client
	// stamps its cache on a 200 and keeps the answer for half an hour. So this
	// says "not ready" rather than answering, the way the preferences route does:
	// the store treats a non-2xx as a failure, which degrades to every surface on
	// WITHOUT stamping the clock, so the next panel to open asks again.
	//
	// Not only a startup race. OnConfigurationChange returns before
	// setConfiguration when LoadPluginConfiguration fails, so the configuration
	// stays nil until a later change succeeds.
	if !p.configurationLoaded() {
		writeAPIError(w, http.StatusServiceUnavailable,
			errcode.WithCode(errcode.APINotReady, "Not ready."))
		return
	}

	maps := p.locationMaps()

	w.Header().Set("Cache-Control", "no-store")

	writeAPIJSON(w, http.StatusOK, featuresResponse{
		MapPanel:  maps.Panel,
		MapInline: maps.Inline,
		MapPage:   maps.Page,
	})
}

// serveAirport answers with one airfield and every reading of its position.
//
// It goes through airport.Describe, the same function the public page uses, so
// a lookup that the panel renders is one the page renders too.
//
// It does NOT consult EnableAirport. A format switch governs decoration only,
// so a link written while the decorator was on must keep resolving after it is
// turned off, exactly as a coordinate link does.
//
// No configurationLoaded gate either, unlike /features: that answer is stamped
// into a client cache and kept for half an hour, where this one is discarded.
func (p *Plugin) serveAirport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeAPIError(w, http.StatusMethodNotAllowed,
			errcode.WithCode(errcode.APIMethodNotAllowed, "Method not allowed."))
		return
	}

	// The shape check, before anything echoes the value back. An ident this
	// build does not hold is a different answer entirely, below.
	ident := r.URL.Query().Get("v")
	if !airport.MatchesIdentShape(ident) {
		writeAPIError(w, http.StatusBadRequest,
			errcode.WithCode(errcode.APIAirportInvalid, "That is not an airfield code this plugin issued."))
		return
	}

	// Constant for the life of a build, and private because the URL and the
	// body both name a place somebody looked up. The five minutes is not about
	// the answer changing; it bounds how long a browser keeps one across a
	// plugin upgrade.
	w.Header().Set("Cache-Control", "private, max-age=300")

	details, found := airport.Describe(ident)
	if !found {
		writeAPIJSON(w, http.StatusOK, airportResponse{Found: false, Ident: ident})
		return
	}

	body := airportResponse{
		Found: true,
		Ident: details.Ident,
		Airport: &airportDetails{
			Name:      details.Name,
			Type:      details.Type,
			Place:     details.Place,
			Elevation: details.Elevation,
			IATA:      details.IATA,
		},
	}
	if details.HasPosition {
		body.Coordinate = &airportCoordinate{
			Format: details.Format,
			Value:  details.Token,
			Region: details.Region,
		}
	}

	writeAPIJSON(w, http.StatusOK, body)
}

func (p *Plugin) handleGetPreferences(w http.ResponseWriter, userID string) {
	prefs, err := p.preferences.Get(userID)
	if err != nil {
		p.API.LogError("Failed to read preferences",
			"error_code", errcode.APIPreferencesReadFailed, "user_id", userID, "error", err.Error())
		writeAPIError(w, http.StatusInternalServerError,
			errcode.WithCode(errcode.APIPreferencesReadFailed, "Could not read your settings."))
		return
	}

	writeAPIJSON(w, http.StatusOK, forWire(prefs))
}

func (p *Plugin) handleSetPreferences(w http.ResponseWriter, r *http.Request, userID string) {
	var prefs UserPreferences
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxPreferencesBody))
	if err := decoder.Decode(&prefs); err != nil {
		writeAPIError(w, http.StatusBadRequest,
			errcode.WithCode(errcode.APIPreferencesInvalidBody, "That is not a valid settings payload."))
		return
	}

	// The message is the validator's, not a generic one: a rejected timezone or
	// an out-of-range threshold is something the reader can act on, and burying
	// it would leave the panel able only to say "something went wrong". It
	// arrives already carrying its own code, so nothing is added here.
	if err := prefs.validate(); err != nil {
		writeAPIError(w, http.StatusBadRequest, err.Error())
		return
	}

	if err := p.preferences.Set(userID, prefs); err != nil {
		p.API.LogError("Failed to save preferences",
			"error_code", errcode.APIPreferencesSaveFailed, "user_id", userID, "error", err.Error())
		writeAPIError(w, http.StatusInternalServerError,
			errcode.WithCode(errcode.APIPreferencesSaveFailed, "Could not save your settings."))
		return
	}

	// The saved blob rather than an empty body, so the panel renders exactly
	// what was stored instead of what it hoped was stored.
	writeAPIJSON(w, http.StatusOK, forWire(prefs))
}

// handleDeletePreferences is "Restore defaults". It removes the blob rather
// than writing today's defaults into it, so the reader goes back to tracking
// whatever the defaults become.
func (p *Plugin) handleDeletePreferences(w http.ResponseWriter, userID string) {
	if err := p.preferences.Delete(userID); err != nil {
		p.API.LogError("Failed to clear preferences",
			"error_code", errcode.APIPreferencesClearFailed, "user_id", userID, "error", err.Error())
		writeAPIError(w, http.StatusInternalServerError,
			errcode.WithCode(errcode.APIPreferencesClearFailed, "Could not restore the defaults."))
		return
	}

	writeAPIJSON(w, http.StatusOK, forWire(UserPreferences{}))
}

// forWire fills in the empty slice JSON would otherwise render as null, so the
// client never has to treat "no zones chosen" as two different values.
func forWire(prefs UserPreferences) UserPreferences {
	if prefs.DTG.Zones == nil {
		prefs.DTG.Zones = []ZoneSelection{}
	}
	return prefs
}

func writeAPIJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	// Nothing useful to do if this fails: the status line is already sent, so
	// the client sees a truncated body either way.
	_ = json.NewEncoder(w).Encode(body)
}

func writeAPIError(w http.ResponseWriter, status int, message string) {
	writeAPIJSON(w, status, map[string]string{"message": message})
}

const packageListCacheSeconds = 60

type packagesResponse struct {
	Packages []string `json:"packages"`

	// The subset the System Console may remove, which is the drop-in directory
	// alone. Sent as its own list rather than folded into Packages so the
	// reader-facing shape stays a plain array of names.
	Removable []string `json:"removable"`
}

func (p *Plugin) packagesPayload() packagesResponse {
	return packagesResponse{Packages: p.packageNames(), Removable: p.removablePackages()}
}

/*
 * The detail map packages this install has, by name.
 *
 * Names rather than URLs, for the same reason basemapUrl() is built in the
 * webapp rather than sent to it: a URL on the wire is a second place a subpath
 * install can be got wrong, and the client already knows how to address this
 * plugin.
 *
 * Never 503 while the configuration is loading, unlike /features. There the
 * zero value is a real answer that a client caches for half an hour, so an
 * unloaded configuration would be stored as an admin decision. Here the zero
 * value is "the bundled packages", which is exactly what an install with no
 * directory configured has, so answering it early costs nothing and a reader
 * gets a map rather than a failure.
 */
func (p *Plugin) servePackageList(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeAPIError(w, http.StatusMethodNotAllowed,
			errcode.WithCode(errcode.APIMethodNotAllowed, "Method not allowed."))
		return
	}

	// Short rather than absent: a package dropped into the directory should
	// appear without a restart, and a reader should not have to reload the tab
	// to see it. Named because the webapp caches the same list for the same
	// span; TestWebappPackageCacheLifetimeMatches holds the pair together.
	w.Header().Set("Cache-Control", fmt.Sprintf("private, max-age=%d", packageListCacheSeconds))

	writeAPIJSON(w, http.StatusOK, p.packagesPayload())
}

/*
 * Installing and removing a detail map package from the System Console.
 *
 * Restricted to system administrators, and checked here rather than trusted
 * from the fact that the request came from the console: the console is a
 * client, and a client is where a request claims to be from rather than where
 * it is from.
 *
 * The body is streamed to disk rather than read, which is the whole reason this
 * is a plain route and not something built on plugin.API: an archive is read by
 * byte range forever afterwards, so it has to land somewhere os.Open can reach.
 */
func (p *Plugin) servePackageAdmin(w http.ResponseWriter, r *http.Request, userID string) {
	if !p.API.HasPermissionTo(userID, model.PermissionManageSystem) {
		writeAPIError(w, http.StatusForbidden,
			errcode.WithCode(errcode.APINotAuthorized, "Not authorized."))
		return
	}

	name, _ := strings.CutPrefix(r.URL.Path, packagesPathAPI+"/")

	switch r.Method {
	case http.MethodPost:
		if err := p.installPackage(name, r.Body); err != nil {
			p.API.LogWarn("a map package upload was refused",
				"error_code", errcode.PackagesUploadWriteFailed, "name", name, "error", err.Error())
			writeAPIError(w, http.StatusBadRequest, err.Error())
			return
		}

		p.API.LogInfo("a map package was installed from the System Console",
			"name", name, "user_id", userID)
		writeAPIJSON(w, http.StatusOK, p.packagesPayload())

	case http.MethodDelete:
		if err := p.removePackage(name); err != nil {
			writeAPIError(w, http.StatusBadRequest, err.Error())
			return
		}

		p.API.LogInfo("a map package was removed from the System Console",
			"name", name, "user_id", userID)
		writeAPIJSON(w, http.StatusOK, p.packagesPayload())

	default:
		writeAPIError(w, http.StatusMethodNotAllowed,
			errcode.WithCode(errcode.APIMethodNotAllowed, "Method not allowed."))
	}
}
