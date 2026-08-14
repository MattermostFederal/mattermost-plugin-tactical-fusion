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
// Deliberately a sibling of decoratePath rather than a branch inside it. The
// two have opposite security postures: /decorate is public and reads no
// workspace data, while everything under here is per-reader and refuses a
// request without a session. Keeping them apart means neither can inherit the
// other's rules by accident.
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
	// workspace data and could in principle sit beside the public page. That is
	// deliberate: the public route exists because clients without a session
	// have no other way to see a decorator, and nothing else should be added to
	// it. A caller with no session and a coordinate to convert is not a reader
	// of this workspace.
	convertPath = apiPath + "/convert"
)

// maxPreferencesBody caps a submitted blob. The largest legitimate one is a few
// hundred bytes, so this is only here to keep a hostile client from making the
// decoder do unbounded work.
const maxPreferencesBody = 8 * 1024

// serveAPI handles the authenticated routes.
//
// Mattermost sets Mattermost-User-Id itself and strips any copy a client tries
// to send, so an empty value here means there is no session, and a non-empty
// one is trustworthy. This is the only place in the plugin that reads it.
func (p *Plugin) serveAPI(w http.ResponseWriter, r *http.Request) {
	userID := r.Header.Get("Mattermost-User-Id")
	if userID == "" {
		writeAPIError(w, http.StatusUnauthorized,
			errcode.WithCode(errcode.APINotAuthorized, "Not authorized."))
		return
	}

	if r.URL.Path == convertPath {
		p.serveConvert(w, r)
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

	// All three parameters, so this applies exactly the checks the public page
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
