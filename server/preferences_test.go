package main

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/mattermost/mattermost/server/public/model"
)

const testUserID = "user1234567890123456789012"

func newPreferenceAPI() *fakeAPI {
	return &fakeAPI{kv: map[string][]byte{}}
}

func TestValidateAcceptsAnEmptyBlob(t *testing.T) {
	// The whole "restore defaults" story rests on this: an empty blob is a
	// valid one, not a rejected one.
	prefs := UserPreferences{}
	if err := prefs.validate(); err != nil {
		t.Fatalf("validate() = %v, want nil", err)
	}
	if prefs.Version != preferencesVersion {
		t.Fatalf("Version = %d, want %d", prefs.Version, preferencesVersion)
	}
	if len(prefs.DTG.Zones) != 0 {
		t.Fatalf("Zones = %v, want empty", prefs.DTG.Zones)
	}
}

func TestValidateNormalisesZones(t *testing.T) {
	prefs := UserPreferences{DTG: DTGPreferences{
		Zones: []ZoneSelection{{IANA: "UTC"}, {IANA: "  Asia/Tokyo  "}, {IANA: ""}, {IANA: "UTC"}, {IANA: "   "}},
	}}

	if err := prefs.validate(); err != nil {
		t.Fatalf("validate() = %v, want nil", err)
	}

	want := []ZoneSelection{{IANA: "UTC"}, {IANA: "Asia/Tokyo"}}
	if !reflect.DeepEqual(prefs.DTG.Zones, want) {
		t.Fatalf("Zones = %v, want %v", prefs.DTG.Zones, want)
	}
}

func TestValidateRejectsBadZones(t *testing.T) {
	cases := map[string]string{
		"unknown identifier": "Mars/Olympus_Mons",
		// Resolves to whatever zone the server process runs in, which is not a
		// place and can differ between nodes of one cluster.
		"Local":            "Local",
		"path traversal":   "../../etc/passwd",
		"absolute path":    "/etc/passwd",
		"stray characters": "Asia/Tokyo; DROP",
		"far too long":     strings.Repeat("a", maxZoneIDLength+1),
	}

	for name, zone := range cases {
		t.Run(name, func(t *testing.T) {
			prefs := UserPreferences{DTG: DTGPreferences{Zones: []ZoneSelection{{IANA: zone}}}}
			if err := prefs.validate(); err == nil {
				t.Fatalf("validate() accepted %q", zone)
			}
		})
	}
}

// Distinct identifiers, since duplicates are collapsed before the count is
// taken. There must be more than maxZones of them.
var tooManyZones = []string{
	"UTC", "Asia/Tokyo", "Asia/Qatar", "Europe/Berlin", "Europe/Paris",
	"Europe/Rome", "Europe/Madrid", "Europe/Lisbon", "Europe/Dublin",
	"Europe/Athens", "Europe/Oslo", "Europe/Prague", "Europe/Vienna",
	"Europe/Warsaw", "Europe/Zurich", "Europe/Sofia", "Europe/Kyiv",
	"Africa/Cairo", "Africa/Lagos", "Africa/Nairobi", "Africa/Tunis",
	"Asia/Seoul", "Asia/Manila", "Asia/Kolkata", "Asia/Dubai",
	"Asia/Bangkok",
}

func TestValidateRejectsTooManyZones(t *testing.T) {
	if len(tooManyZones) <= maxZones {
		t.Fatalf("the fixture has %d zones, which no longer exceeds maxZones (%d)",
			len(tooManyZones), maxZones)
	}

	zones := make([]ZoneSelection, 0, len(tooManyZones))
	for _, id := range tooManyZones {
		zones = append(zones, ZoneSelection{IANA: id})
	}

	prefs := UserPreferences{DTG: DTGPreferences{Zones: zones}}
	if err := prefs.validate(); err == nil {
		t.Fatalf("validate() accepted %d zones, want a rejection above %d",
			len(tooManyZones), maxZones)
	}
}

func TestValidateThreshold(t *testing.T) {
	cases := []struct {
		name    string
		minutes int
		wantErr bool
	}{
		// Zero is "unset", which is how a reader goes back to the default.
		{"zero means default", 0, false},
		{"lowest allowed", minUrgentWithinMinutes, false},
		{"highest allowed", maxUrgentWithinMinutes, false},
		{"below the range", -1, true},
		{"above the range", maxUrgentWithinMinutes + 1, true},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			prefs := UserPreferences{DTG: DTGPreferences{UrgentWithinMinutes: c.minutes}}
			err := prefs.validate()
			if (err != nil) != c.wantErr {
				t.Fatalf("validate(%d) = %v, wantErr %v", c.minutes, err, c.wantErr)
			}
		})
	}
}

func TestCloneDoesNotShareZones(t *testing.T) {
	original := UserPreferences{DTG: DTGPreferences{Zones: []ZoneSelection{{IANA: "UTC"}}}}

	copied := original.clone()
	copied.DTG.Zones[0] = ZoneSelection{IANA: "Asia/Tokyo"}

	if original.DTG.Zones[0].IANA != "UTC" {
		t.Fatalf("clone shares its slice: original = %v", original.DTG.Zones)
	}
}

func TestKVStoreRoundTrip(t *testing.T) {
	api := newPreferenceAPI()
	store := &kvPreferenceStore{api: api}

	want := UserPreferences{Version: 1, DTG: DTGPreferences{
		Zones:               []ZoneSelection{{IANA: "UTC"}, {IANA: "Asia/Tokyo", Name: "Yokota"}},
		UrgentWithinMinutes: 15,
	}}
	if err := store.Set(testUserID, want); err != nil {
		t.Fatalf("Set() = %v", err)
	}

	got, err := store.Get(testUserID)
	if err != nil {
		t.Fatalf("Get() = %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Get() = %+v, want %+v", got, want)
	}
}

func TestKVStoreMissingKeyIsDefaults(t *testing.T) {
	store := &kvPreferenceStore{api: newPreferenceAPI()}

	got, err := store.Get(testUserID)
	if err != nil {
		t.Fatalf("Get() = %v, want no error for a reader who never saved", err)
	}
	if !reflect.DeepEqual(got, UserPreferences{}) {
		t.Fatalf("Get() = %+v, want the zero value", got)
	}
}

// A blob this build cannot read must not be able to take the panel down. The
// reader loses their settings either way; failing the request would lose the
// panel too.
func TestKVStoreUnreadableBlobFallsBackToDefaults(t *testing.T) {
	api := newPreferenceAPI()
	api.kv[preferenceKey(testUserID)] = []byte("{not json")
	store := &kvPreferenceStore{api: api}

	got, err := store.Get(testUserID)
	if err != nil {
		t.Fatalf("Get() = %v, want no error", err)
	}
	if !reflect.DeepEqual(got, UserPreferences{}) {
		t.Fatalf("Get() = %+v, want the zero value", got)
	}
	if len(api.warnings) == 0 {
		t.Fatal("an unreadable blob was discarded without a warning")
	}
}

func TestKVStoreDelete(t *testing.T) {
	api := newPreferenceAPI()
	store := &kvPreferenceStore{api: api}

	if err := store.Set(testUserID, UserPreferences{DTG: DTGPreferences{Zones: []ZoneSelection{{IANA: "UTC"}}}}); err != nil {
		t.Fatalf("Set() = %v", err)
	}
	if err := store.Delete(testUserID); err != nil {
		t.Fatalf("Delete() = %v", err)
	}

	if _, ok := api.kv[preferenceKey(testUserID)]; ok {
		t.Fatal("the key survived the delete")
	}
}

func TestKVStoreSurfacesReadFailures(t *testing.T) {
	api := newPreferenceAPI()
	api.kvGetErr = model.NewAppError("KVGet", "kv.read", nil, "boom", 500)
	store := &kvPreferenceStore{api: api}

	if _, err := store.Get(testUserID); err == nil {
		t.Fatal("Get() succeeded despite a KV failure")
	}
}

func TestKVStoreSurfacesWriteFailures(t *testing.T) {
	api := newPreferenceAPI()
	api.kvSetErr = model.NewAppError("KVSet", "kv.write", nil, "boom", 500)
	store := &kvPreferenceStore{api: api}

	if err := store.Set(testUserID, UserPreferences{}); err == nil {
		t.Fatal("Set() succeeded despite a KV failure")
	}
}

// The blob is stored as JSON, so the wire names are a compatibility surface:
// renaming one silently discards everybody's saved settings.
func TestStoredBlobKeepsItsWireNames(t *testing.T) {
	api := newPreferenceAPI()
	store := &kvPreferenceStore{api: api}

	err := store.Set(testUserID, UserPreferences{Version: 1, DTG: DTGPreferences{
		Zones:               []ZoneSelection{{IANA: "UTC"}},
		UrgentWithinMinutes: 5,
	}})
	if err != nil {
		t.Fatalf("Set() = %v", err)
	}

	var raw map[string]any
	if err := json.Unmarshal(api.kv[preferenceKey(testUserID)], &raw); err != nil {
		t.Fatalf("stored blob is not JSON: %v", err)
	}

	dtg, ok := raw["dtg"].(map[string]any)
	if !ok {
		t.Fatalf("stored blob has no dtg object: %v", raw)
	}
	if _, ok := dtg["zones"]; !ok {
		t.Fatalf("stored blob has no zones field: %v", dtg)
	}
	if _, ok := dtg["urgent_within_minutes"]; !ok {
		t.Fatalf("stored blob has no urgent_within_minutes field: %v", dtg)
	}
}
