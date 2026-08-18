package main

import (
	"encoding/json"
	"net/http"

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
)

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
