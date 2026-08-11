package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/mattermost/mattermost/server/public/plugin"
)

// newAPIPlugin returns a plugin with a working preferences store, wired the way
// OnActivate wires it.
func newAPIPlugin(t *testing.T) (*Plugin, *fakeAPI) {
	t.Helper()

	p := newTestPlugin(t, "https://example.com", true)
	api, ok := p.API.(*fakeAPI)
	if !ok {
		t.Fatalf("test plugin API is %T, want *fakeAPI", p.API)
	}
	api.kv = map[string][]byte{}
	p.preferences = newCachingPreferenceStore(&kvPreferenceStore{api: api}, api)

	return p, api
}

// call issues one request against the plugin's router. An empty userID means no
// session, which is what Mattermost sends for an unauthenticated request.
func call(p *Plugin, method, path, userID, body string) *httptest.ResponseRecorder {
	var reader *strings.Reader
	if body == "" {
		reader = strings.NewReader("")
	} else {
		reader = strings.NewReader(body)
	}

	req := httptest.NewRequest(method, path, reader)
	if userID != "" {
		req.Header.Set("Mattermost-User-Id", userID)
	}

	rec := httptest.NewRecorder()
	p.ServeHTTP(&plugin.Context{}, rec, req)

	return rec
}

func decodePreferences(t *testing.T, rec *httptest.ResponseRecorder) UserPreferences {
	t.Helper()

	var prefs UserPreferences
	if err := json.Unmarshal(rec.Body.Bytes(), &prefs); err != nil {
		t.Fatalf("response is not a preferences blob: %v (%s)", err, rec.Body.String())
	}

	return prefs
}

// The whole reason this lives under /api/v1 rather than beside /decorate: it
// reads per-reader state and so must refuse a request with no session.
func TestPreferencesRequireASession(t *testing.T) {
	p, _ := newAPIPlugin(t)

	for _, method := range []string{http.MethodGet, http.MethodPut, http.MethodDelete} {
		t.Run(method, func(t *testing.T) {
			rec := call(p, method, preferencesPath, "", "{}")
			if rec.Code != http.StatusUnauthorized {
				t.Fatalf("status = %d, want 401", rec.Code)
			}
		})
	}
}

// The sibling route stays public. A regression here would either lock the
// mobile app out of the decorator page or open this API to anonymous callers.
func TestDecoratePageStaysPublic(t *testing.T) {
	p, _ := newAPIPlugin(t)

	rec := call(p, http.MethodGet,
		"/decorate/dtg?t=1786293000000&dtg=091630ZAUG26&z=Z&a=", "", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 without a session", rec.Code)
	}
}

func TestGetPreferencesReturnsDefaultsForANewReader(t *testing.T) {
	p, _ := newAPIPlugin(t)

	rec := call(p, http.MethodGet, preferencesPath, testUserID, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}

	prefs := decodePreferences(t, rec)
	if len(prefs.DTG.Zones) != 0 || prefs.DTG.UrgentWithinMinutes != 0 {
		t.Fatalf("prefs = %+v, want the zero value", prefs)
	}

	// An empty array rather than null, so the client has one shape to handle.
	if !strings.Contains(rec.Body.String(), `"zones":[]`) {
		t.Fatalf("body = %s, want an empty zones array", rec.Body.String())
	}
}

func TestPreferencesAreNeverCached(t *testing.T) {
	p, _ := newAPIPlugin(t)

	rec := call(p, http.MethodGet, preferencesPath, testUserID, "")
	if got := rec.Header().Get("Cache-Control"); got != "no-store" {
		t.Fatalf("Cache-Control = %q, want no-store", got)
	}
}

func TestSavePreferencesRoundTrips(t *testing.T) {
	p, _ := newAPIPlugin(t)

	body := `{"dtg":{"zones":[{"iana":"UTC"},{"iana":"Asia/Tokyo","name":"Yokota"}],"urgent_within_minutes":15}}`
	rec := call(p, http.MethodPut, preferencesPath, testUserID, body)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (%s)", rec.Code, rec.Body.String())
	}

	// The response is the saved blob, so the panel renders what was stored
	// rather than what it hoped was stored.
	saved := decodePreferences(t, rec)
	want := []ZoneSelection{{IANA: "UTC"}, {IANA: "Asia/Tokyo", Name: "Yokota"}}
	if !reflect.DeepEqual(saved.DTG.Zones, want) {
		t.Fatalf("saved zones = %v, want %v", saved.DTG.Zones, want)
	}
	if saved.DTG.UrgentWithinMinutes != 15 {
		t.Fatalf("saved threshold = %d, want 15", saved.DTG.UrgentWithinMinutes)
	}

	fetched := decodePreferences(t, call(p, http.MethodGet, preferencesPath, testUserID, ""))
	if !reflect.DeepEqual(fetched, saved) {
		t.Fatalf("re-read %+v, want %+v", fetched, saved)
	}
}

func TestSavePreferencesRejectsBadInput(t *testing.T) {
	cases := map[string]string{
		"not json":       `{`,
		"unknown zone":   `{"dtg":{"zones":[{"iana":"Mars/Olympus_Mons"}]}}`,
		"local zone":     `{"dtg":{"zones":[{"iana":"Local"}]}}`,
		"threshold high": `{"dtg":{"urgent_within_minutes":100000}}`,
		"threshold low":  `{"dtg":{"urgent_within_minutes":-5}}`,
		"wrong type":     `{"dtg":{"zones":"UTC"}}`,
		"control label":  `{"dtg":{"zones":[{"iana":"UTC","name":"a\u0007b"}]}}`,
	}

	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			p, api := newAPIPlugin(t)

			rec := call(p, http.MethodPut, preferencesPath, testUserID, body)
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400 (%s)", rec.Code, rec.Body.String())
			}
			if len(api.kv) != 0 {
				t.Fatalf("a rejected payload still wrote to the store: %v", api.kv)
			}
		})
	}
}

// A hostile client must not be able to make the decoder do unbounded work.
func TestSavePreferencesRejectsAnOversizedBody(t *testing.T) {
	p, _ := newAPIPlugin(t)

	body := `{"dtg":{"zones":[{"iana":"` + strings.Repeat("A", maxPreferencesBody) + `"}]}}`
	rec := call(p, http.MethodPut, preferencesPath, testUserID, body)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

// "Restore defaults" removes the blob rather than writing today's defaults, so
// the reader goes back to tracking whatever the defaults become.
func TestDeletePreferencesRemovesTheBlob(t *testing.T) {
	p, api := newAPIPlugin(t)

	body := `{"dtg":{"zones":[{"iana":"UTC"}],"urgent_within_minutes":15}}`
	if rec := call(p, http.MethodPut, preferencesPath, testUserID, body); rec.Code != http.StatusOK {
		t.Fatalf("save failed: %d %s", rec.Code, rec.Body.String())
	}

	rec := call(p, http.MethodDelete, preferencesPath, testUserID, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if prefs := decodePreferences(t, rec); len(prefs.DTG.Zones) != 0 || prefs.DTG.UrgentWithinMinutes != 0 {
		t.Fatalf("delete returned %+v, want the defaults", prefs)
	}
	if _, ok := api.kv[preferenceKey(testUserID)]; ok {
		t.Fatal("the stored blob survived the delete")
	}

	fetched := decodePreferences(t, call(p, http.MethodGet, preferencesPath, testUserID, ""))
	if len(fetched.DTG.Zones) != 0 || fetched.DTG.UrgentWithinMinutes != 0 {
		t.Fatalf("re-read %+v, want the defaults", fetched)
	}
}

// Deleting settings that were never saved is how a reader who has customised
// nothing uses the button. It has to succeed.
func TestDeletePreferencesIsIdempotent(t *testing.T) {
	p, _ := newAPIPlugin(t)

	rec := call(p, http.MethodDelete, preferencesPath, testUserID, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
}

// Two readers must not see each other's settings.
func TestPreferencesAreScopedToTheReader(t *testing.T) {
	p, _ := newAPIPlugin(t)
	other := "user6543210987654321098765"

	body := `{"dtg":{"zones":[{"iana":"Asia/Tokyo"}]}}`
	if rec := call(p, http.MethodPut, preferencesPath, testUserID, body); rec.Code != http.StatusOK {
		t.Fatalf("save failed: %d %s", rec.Code, rec.Body.String())
	}

	fetched := decodePreferences(t, call(p, http.MethodGet, preferencesPath, other, ""))
	if len(fetched.DTG.Zones) != 0 {
		t.Fatalf("the second reader saw %v, want the defaults", fetched.DTG.Zones)
	}
}

func TestAPIRejectsUnknownRoutesAndMethods(t *testing.T) {
	p, _ := newAPIPlugin(t)

	if rec := call(p, http.MethodGet, apiPath+"/nope", testUserID, ""); rec.Code != http.StatusNotFound {
		t.Fatalf("unknown route status = %d, want 404", rec.Code)
	}
	if rec := call(p, http.MethodPost, preferencesPath, testUserID, "{}"); rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("POST status = %d, want 405", rec.Code)
	}
}

// A request that lands before OnActivate finishes has nothing to serve, and
// must say so rather than panicking on a nil store.
func TestAPIBeforeActivation(t *testing.T) {
	p := newTestPlugin(t, "https://example.com", true)
	p.preferences = nil

	rec := call(p, http.MethodGet, preferencesPath, testUserID, "")
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", rec.Code)
	}
}

// Two installations sharing a zone are two rows. Deduplicating on the zone
// would silently drop one and leave the reader wondering where it went.
func TestSaveKeepsTwoBasesInOneZone(t *testing.T) {
	p, _ := newAPIPlugin(t)

	body := `{"dtg":{"zones":[` +
		`{"iana":"Europe/Berlin","name":"Ramstein"},` +
		`{"iana":"Europe/Berlin","name":"USAG Stuttgart"}]}}`

	rec := call(p, http.MethodPut, preferencesPath, testUserID, body)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (%s)", rec.Code, rec.Body.String())
	}

	saved := decodePreferences(t, rec)
	want := []ZoneSelection{
		{IANA: "Europe/Berlin", Name: "Ramstein"},
		{IANA: "Europe/Berlin", Name: "USAG Stuttgart"},
	}
	if !reflect.DeepEqual(saved.DTG.Zones, want) {
		t.Fatalf("saved zones = %v, want %v", saved.DTG.Zones, want)
	}
}

// The same base twice is still one row, so the pair is what gets deduplicated.
func TestSaveDeduplicatesOnTheWholeEntry(t *testing.T) {
	p, _ := newAPIPlugin(t)

	body := `{"dtg":{"zones":[` +
		`{"iana":"Europe/Berlin","name":"Ramstein"},` +
		`{"iana":"Europe/Berlin","name":"Ramstein"}]}}`

	rec := call(p, http.MethodPut, preferencesPath, testUserID, body)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (%s)", rec.Code, rec.Body.String())
	}

	if saved := decodePreferences(t, rec); len(saved.DTG.Zones) != 1 {
		t.Fatalf("saved %d zones, want 1", len(saved.DTG.Zones))
	}
}

// Blobs written before base names existed hold bare identifiers. Discarding
// them would silently wipe a reader's settings on upgrade.
func TestReadsABlobWrittenBeforeNames(t *testing.T) {
	p, api := newAPIPlugin(t)
	api.kv[preferenceKey(testUserID)] = []byte(
		`{"version":1,"dtg":{"zones":["UTC","Asia/Tokyo"],"urgent_within_minutes":15}}`)

	fetched := decodePreferences(t, call(p, http.MethodGet, preferencesPath, testUserID, ""))

	want := []ZoneSelection{{IANA: "UTC"}, {IANA: "Asia/Tokyo"}}
	if !reflect.DeepEqual(fetched.DTG.Zones, want) {
		t.Fatalf("read %v, want %v", fetched.DTG.Zones, want)
	}
	if fetched.DTG.UrgentWithinMinutes != 15 {
		t.Fatalf("threshold = %d, want 15", fetched.DTG.UrgentWithinMinutes)
	}
}

// A bare identifier is still accepted on the wire, for a client running an
// older bundle than the server.
func TestSaveAcceptsABareIdentifier(t *testing.T) {
	p, _ := newAPIPlugin(t)

	rec := call(p, http.MethodPut, preferencesPath, testUserID, `{"dtg":{"zones":["UTC"]}}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (%s)", rec.Code, rec.Body.String())
	}

	if saved := decodePreferences(t, rec); !reflect.DeepEqual(saved.DTG.Zones, []ZoneSelection{{IANA: "UTC"}}) {
		t.Fatalf("saved zones = %v", saved.DTG.Zones)
	}
}
