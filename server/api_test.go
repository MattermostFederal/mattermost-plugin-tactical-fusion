package main

import (
	"encoding/json"
	"math"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strings"
	"testing"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/plugin"

	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/decorators/location"
	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/errcode"
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
			assertCode(t, rec.Body.String(), errcode.APINotAuthorized)
		})
	}
}

// Both routes require a session and answer a missing one differently. The API
// refuses, because its callers are fetch and want a status; the page redirects,
// because its caller is a person who can sign in and carry on.
//
// Asserted together, because the thing worth pinning is the difference: a
// regression that made the page 401 would be silent in a browser, and one that
// made the API redirect would put a login document through JSON.parse.
func TestTheTwoRoutesRefuseAnAnonymousCallerDifferently(t *testing.T) {
	p, _ := newAPIPlugin(t)

	page := call(p, http.MethodGet,
		"/decorate/dtg?t=1786293000000&dtg=091630ZAUG26&z=Z&a=", "", "")
	if page.Code != http.StatusFound {
		t.Fatalf("page status = %d, want 302 without a session", page.Code)
	}

	api := call(p, http.MethodGet, convertPath+"?f=mgrs&v=18SUJ2347806483", "", "")
	if api.Code != http.StatusUnauthorized {
		t.Fatalf("api status = %d, want 401 without a session", api.Code)
	}
	if got := api.Header().Get("Location"); got != "" {
		t.Fatalf("the API redirected to %q; it must answer a status", got)
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
	// Every one of these is a 400, so the status says nothing about which rule
	// the payload broke. The code does, and the reader sees it: the validator's
	// own message is what reaches the panel, deliberately, so a rejected
	// timezone is something they can act on.
	cases := []struct {
		name string
		body string
		code int
	}{
		{"not json", `{`, errcode.APIPreferencesInvalidBody},
		{"wrong type", `{"dtg":{"zones":"UTC"}}`, errcode.APIPreferencesInvalidBody},
		{"unknown zone", `{"dtg":{"zones":[{"iana":"Mars/Olympus_Mons"}]}}`, errcode.PreferencesZoneIDUnknown},
		{"local zone", `{"dtg":{"zones":[{"iana":"Local"}]}}`, errcode.PreferencesZoneIDLocal},
		{"threshold high", `{"dtg":{"urgent_within_minutes":100000}}`, errcode.PreferencesThresholdOutOfRange},
		{"threshold low", `{"dtg":{"urgent_within_minutes":-5}}`, errcode.PreferencesThresholdOutOfRange},
		{"control label", `{"dtg":{"zones":[{"iana":"UTC","name":"a\u0007b"}]}}`, errcode.PreferencesZoneNameControlCharacters},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p, api := newAPIPlugin(t)

			rec := call(p, http.MethodPut, preferencesPath, testUserID, tc.body)
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400 (%s)", rec.Code, rec.Body.String())
			}
			assertCode(t, rec.Body.String(), tc.code)
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

	rec := call(p, http.MethodGet, apiPath+"/nope", testUserID, "")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("unknown route status = %d, want 404", rec.Code)
	}
	assertCode(t, rec.Body.String(), errcode.APINotFound)

	rec = call(p, http.MethodPost, preferencesPath, testUserID, "{}")
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("POST status = %d, want 405", rec.Code)
	}
	assertCode(t, rec.Body.String(), errcode.APIMethodNotAllowed)
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
	assertCode(t, rec.Body.String(), errcode.APINotReady)
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

// The three 500s below carry TF-13004, TF-13006 and TF-13007, which
// public/help/troubleshooting.html tells readers they might see. Nothing
// previously proved any of them was reachable, which makes the documentation a
// claim rather than a description.

func TestGetPreferencesSurfacesAReadFailure(t *testing.T) {
	p, api := newAPIPlugin(t)
	api.kvGetErr = model.NewAppError("KVGet", "kv.read", nil, "boom", http.StatusInternalServerError)

	rec := call(p, http.MethodGet, preferencesPath, testUserID, "")

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500 (%s)", rec.Code, rec.Body.String())
	}
	assertCode(t, rec.Body.String(), errcode.APIPreferencesReadFailed)

	// Logged as well as returned, so an operator can find the cause behind the
	// message the reader saw.
	if len(api.errors) == 0 {
		t.Fatal("the read failure was returned to the reader but never logged")
	}
}

func TestSavePreferencesSurfacesAWriteFailure(t *testing.T) {
	p, api := newAPIPlugin(t)
	api.kvSetErr = model.NewAppError("KVSet", "kv.write", nil, "boom", http.StatusInternalServerError)

	body := `{"dtg":{"zones":[{"iana":"UTC"}],"urgent_within_minutes":15}}`
	rec := call(p, http.MethodPut, preferencesPath, testUserID, body)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500 (%s)", rec.Code, rec.Body.String())
	}
	assertCode(t, rec.Body.String(), errcode.APIPreferencesSaveFailed)
	if len(api.errors) == 0 {
		t.Fatal("the write failure was returned to the reader but never logged")
	}

	// The reader's stored settings are untouched. A failed save that had
	// partially applied would be worse than one that failed outright.
	if len(api.kv) != 0 {
		t.Fatalf("a failed save still wrote to the store: %v", api.kv)
	}
}

func TestDeletePreferencesSurfacesAClearFailure(t *testing.T) {
	p, api := newAPIPlugin(t)
	api.kvDeleteErr = model.NewAppError("KVDelete", "kv.delete", nil, "boom", http.StatusInternalServerError)

	rec := call(p, http.MethodDelete, preferencesPath, testUserID, "")

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500 (%s)", rec.Code, rec.Body.String())
	}
	assertCode(t, rec.Body.String(), errcode.APIPreferencesClearFailed)
	if len(api.errors) == 0 {
		t.Fatal("the clear failure was returned to the reader but never logged")
	}
}

// The conversion endpoint.
//
// location/convert_test.go already covers which links Convert accepts and which
// it refuses, so nothing below re-derives that matrix. What is tested here is
// the HTTP contract wrapped around it, which is a different thing and was
// untested: the route, the session gate, the method gate, the status codes, the
// error code and the caching header.

// convertURL builds a request to the conversion endpoint. Values go through
// url.Values because r legitimately carries spaces.
func convertURL(f, v, raw string) string {
	params := url.Values{"f": {f}, "v": {v}}
	if raw != "" {
		params.Set("r", raw)
	}

	return convertPath + "?" + params.Encode()
}

// The same session gate as the preferences resource, which is the whole reason
// both live under /api/v1 rather than beside the public page.
// TestDecoratePageStaysPublic pins the other half of that split.
func TestConvertRequiresASession(t *testing.T) {
	p, _ := newAPIPlugin(t)

	rec := call(p, http.MethodGet, convertURL("mgrs", "18SUJ2347806483", ""), "", "")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
	assertCode(t, rec.Body.String(), errcode.APINotAuthorized)
}

// The exact strings, because these are the wire contract: the panel renders
// every one of them verbatim and has no way to check them.
func TestConvertReturnsEveryDerivedReading(t *testing.T) {
	p, _ := newAPIPlugin(t)

	rec := call(p, http.MethodGet, convertURL("mgrs", "18SUJ2347806483", ""), testUserID, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (%s)", rec.Code, rec.Body.String())
	}

	var got location.Conversion
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("response is not a conversion: %v (%s)", err, rec.Body.String())
	}

	want := location.Conversion{
		MGRS:    "18S UJ 23478 06483",
		UTM:     "18S 323478E 4306483N",
		Decimal: "38.889502° N, 77.035295° W",
		DMS:     `38°53'22.21"N 77°02'07.06"W`,
		DDM:     "38°53.3701'N 77°02.1177'W",
		USMTF:   "385322.21N0770207.06W",
		Region:  "United States of America (Natural Earth 110m)",
		Lat:     got.Lat,
		Lon:     got.Lon,
	}
	if got != want {
		t.Fatalf("conversion =\n%+v\nwant\n%+v", got, want)
	}

	// The position is compared separately, and to a tolerance. Pinning a
	// seventeen-digit float in a golden struct breaks on any change to the
	// projection that is far smaller than the resolution the token carries.
	const wantLat, wantLon = 38.889502, -77.035295
	if math.Abs(got.Lat-wantLat) > 1e-5 || math.Abs(got.Lon-wantLon) > 1e-5 {
		t.Errorf("position = %v, %v, want %v, %v within 1e-5", got.Lat, got.Lon, wantLat, wantLon)
	}

	// Every field named, so a renamed JSON tag fails here rather than silently
	// leaving a row blank in the panel.
	for _, field := range []string{"mgrs", "utm", "decimal", "dms", "ddm", "usmtf", "region", "lat", "lon"} {
		if !strings.Contains(rec.Body.String(), `"`+field+`":`) {
			t.Errorf("body has no %q field: %s", field, rec.Body.String())
		}
	}
}

// A conversion is the same answer forever for the same token, so it is cached,
// unlike the preferences resource a few lines away in the same handler. Private
// because the URL and the body both carry a position.
func TestConvertIsCachedPrivately(t *testing.T) {
	p, _ := newAPIPlugin(t)

	rec := call(p, http.MethodGet, convertURL("mgrs", "18SUJ2347806483", ""), testUserID, "")
	if got := rec.Header().Get("Cache-Control"); got != "private, max-age=300" {
		t.Fatalf("Cache-Control = %q, want private, max-age=300", got)
	}
}

// A refusal must not be cached as though it were an answer, and must not
// inherit the preferences branch's no-store either.
func TestConvertDoesNotCacheARefusal(t *testing.T) {
	p, _ := newAPIPlugin(t)

	rec := call(p, http.MethodGet, convertURL("mgrs", "not a grid reference", ""), testUserID, "")
	if got := rec.Header().Get("Cache-Control"); got != "" {
		t.Fatalf("a refused conversion carries Cache-Control %q, want none", got)
	}
}

func TestConvertRejectsANonGet(t *testing.T) {
	p, _ := newAPIPlugin(t)

	for _, method := range []string{http.MethodPost, http.MethodPut, http.MethodDelete} {
		t.Run(method, func(t *testing.T) {
			rec := call(p, method, convertURL("mgrs", "18SUJ2347806483", ""), testUserID, "")
			if rec.Code != http.StatusMethodNotAllowed {
				t.Fatalf("status = %d, want 405 (%s)", rec.Code, rec.Body.String())
			}
			assertCode(t, rec.Body.String(), errcode.APIMethodNotAllowed)
		})
	}
}

// One case per gate, to prove the refusal reaches the right status and code.
// Which links are refused is convert_test.go's subject, not this one's.
func TestConvertRejectsALinkTheServerWouldNotHaveIssued(t *testing.T) {
	p, _ := newAPIPlugin(t)

	for _, tc := range refusedConvertLinks {
		t.Run(tc.name, func(t *testing.T) {
			rec := call(p, http.MethodGet, convertURL(tc.f, tc.v, tc.raw), testUserID, "")
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400 (%s)", rec.Code, rec.Body.String())
			}
			assertCode(t, rec.Body.String(), errcode.APIConvertInvalid)
		})
	}
}

// refusedConvertLinks is one link per gate on the way in, shared by the test
// above and the one below so the two surfaces are asked about the same links.
var refusedConvertLinks = []struct {
	name      string
	f, v, raw string
}{
	{"an unknown format id", "usng", "18SUJ2347806483", ""},
	{"no format id at all", "", "18SUJ2347806483", ""},
	{"a token that is not its own canonical form", "mgrs", "18S UJ 23478 06483", ""},
	{"a token of a different grammar", "mgrs", "3510N07901W", ""},
	{"no token at all", "mgrs", "", ""},
	{"a square that names nowhere", "mgrs", "32WDL123123", ""},

	// The gates on the author's own text. Two of them need the token grammar,
	// which is Go-only, so these are exactly the cases the webapp cannot check
	// for itself and asks about here instead.
	{"prose wrapped around a coordinate", "latm", "3510N07901W", "3510N07901W ALFA"},
	{"an r naming a different place", "latm", "3510N07901W", "3610N08001W"},
	{"markup in r", "latm", "3510N07901W", "3510N07901W<b>"},
}

// The invariant the whole validation layout exists for: /api/v1/convert and the
// public page are two doors onto the same coordinates, and neither may have
// weaker locks than the other.
//
// convert_test.go holds Convert to validateParams, which is the same check one
// level down. This holds the two ROUTES to each other, which is what a reader
// actually meets: a link accepted here and refused there would render a
// position in the sidebar that its own page will not show, and the click
// handler's fallback makes that look like a routing choice rather than a
// rejection.
func TestConvertAndThePageRefuseTheSameLinks(t *testing.T) {
	p, _ := newAPIPlugin(t)

	accepted := []struct {
		name      string
		f, v, raw string
	}{
		{"a grid reference", "mgrs", "18SUJ2347806483", ""},
		{"a UTM position with its author's spacing", "utm", "18S3234784306483", "18S 323478E 4306483N"},
		{"a USMTF token", "latm", "3510N07901W", ""},
	}

	for _, tc := range accepted {
		t.Run("accepted/"+tc.name, func(t *testing.T) {
			assertBothRoutes(t, p, tc.f, tc.v, tc.raw, http.StatusOK)
		})
	}

	for _, tc := range refusedConvertLinks {
		t.Run("refused/"+tc.name, func(t *testing.T) {
			assertBothRoutes(t, p, tc.f, tc.v, tc.raw, http.StatusBadRequest)
		})
	}
}

// assertBothRoutes requires the endpoint and the page to answer with the same
// status for one link. The error CODES differ by design, since they name two
// different surfaces; agreeing about accept-or-refuse is the property.
func assertBothRoutes(t *testing.T, p *Plugin, f, v, raw string, want int) {
	t.Helper()

	api := call(p, http.MethodGet, convertURL(f, v, raw), testUserID, "")

	params := url.Values{"f": {f}, "v": {v}}
	if raw != "" {
		params.Set("r", raw)
	}
	// With a session, or every page here answers 302 and the comparison is
	// between two routes that never reached their validation.
	page := call(p, http.MethodGet, decoratePath+"/location?"+params.Encode(), testUserID, "")

	if api.Code != want || page.Code != want {
		t.Fatalf("endpoint = %d and page = %d, want both %d\n  endpoint: %s\n  page carried %d bytes",
			api.Code, page.Code, want, api.Body.String(), page.Body.Len())
	}
}

// r is the reason the endpoint takes three parameters rather than two: it is
// the author's own spelling, and two of the four gates on it need the token
// grammar, which lives in Go only.
//
// It is display only and derives nothing, so sending it may decide whether the
// request is answered at all but may never change the answer. Asserted by
// converting the same token twice, since r legitimately reappears inside a row
// by coincidence: "18S 323478E 4306483N" is both what this author typed and how
// gridText spells that position, so looking for the string in the body would
// pin nothing.
func TestConvertCarriesTheAuthorsTextWithoutDerivingFromIt(t *testing.T) {
	p, _ := newAPIPlugin(t)

	const (
		token = "18S3234784306483"
		raw   = "18S 323478E 4306483N"
	)

	with := call(p, http.MethodGet, convertURL("utm", token, raw), testUserID, "")
	if with.Code != http.StatusOK {
		t.Fatalf("status = %d with an r the token round-trips to (%s)", with.Code, with.Body.String())
	}

	without := call(p, http.MethodGet, convertURL("utm", token, ""), testUserID, "")
	if without.Code != http.StatusOK {
		t.Fatalf("status = %d with no r at all (%s)", without.Code, without.Body.String())
	}

	if with.Body.String() != without.Body.String() {
		t.Fatalf("r changed the conversion:\n  with r: %s\n  without: %s",
			with.Body.String(), without.Body.String())
	}
}
