package main

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/mattermost/mattermost/server/public/model"

	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/cot"
	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/decorators/location"
	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/errcode"
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

func TestValidateNormalizesZones(t *testing.T) {
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
	// The code separates "that is not shaped like an identifier" from "that is
	// shaped like one but names nowhere", which the reader sees and which are
	// two quite different mistakes to have made.
	cases := []struct {
		name string
		zone string
		code int
	}{
		{"unknown identifier", "Mars/Olympus_Mons", errcode.PreferencesZoneIDUnknown},
		// Resolves to whatever zone the server process runs in, which is not a
		// place and can differ between nodes of one cluster.
		{"Local", "Local", errcode.PreferencesZoneIDLocal},
		{"path traversal", "../../etc/passwd", errcode.PreferencesZoneIDMalformed},
		// Clears the shape check, because the regexp permits a leading slash.
		// time.LoadLocation is what stops it: Go refuses any name beginning
		// with a separator or containing "..", so the traversal defense holds
		// either way, just one step further along.
		{"absolute path", "/etc/passwd", errcode.PreferencesZoneIDUnknown},
		{"stray characters", "Asia/Tokyo; DROP", errcode.PreferencesZoneIDMalformed},
		{"far too long", strings.Repeat("a", maxZoneIDLength+1), errcode.PreferencesZoneIDMalformed},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			prefs := UserPreferences{DTG: DTGPreferences{Zones: []ZoneSelection{{IANA: tc.zone}}}}
			err := prefs.validate()
			if err == nil {
				t.Fatalf("validate() accepted %q", tc.zone)
			}
			assertCode(t, err.Error(), tc.code)
		})
	}
}

// A label is the reader's own free text, so the only thing worth enforcing is
// that it cannot be absurd. The limit counts runes rather than bytes, or a name
// in a non-Latin script would be rejected at a third of the length an ASCII one
// is allowed.
func TestValidateBoundsTheZoneLabel(t *testing.T) {
	cases := []struct {
		name     string
		label    string
		accepted bool
	}{
		{"empty label is an unnamed zone", "", true},
		{"ordinary label", "Ramstein", true},
		{"exactly at the limit", strings.Repeat("a", maxZoneNameLength), true},
		{"one rune over", strings.Repeat("a", maxZoneNameLength+1), false},
		// Well inside the limit by runes, well over it by bytes. Counting bytes
		// would reject this.
		{"multi-byte label inside the limit", strings.Repeat("東", maxZoneNameLength/2), true},
		{"multi-byte label over the limit", strings.Repeat("東", maxZoneNameLength+1), false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			prefs := UserPreferences{DTG: DTGPreferences{
				Zones: []ZoneSelection{{IANA: "UTC", Name: tc.label}},
			}}

			err := prefs.validate()

			if tc.accepted {
				if err != nil {
					t.Fatalf("validate() = %v, want the label accepted", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("validate() accepted a %d-rune label", len([]rune(tc.label)))
			}
			assertCode(t, err.Error(), errcode.PreferencesZoneNameTooLong)
		})
	}
}

// A zone entry is written as an object, but blobs stored before names existed
// hold a bare identifier. Both have to decode, and anything else has to fail
// rather than quietly yield a zone with no identifier, which validate() would
// then skip as empty and silently drop from the reader's table.
func TestZoneSelectionUnmarshalJSON(t *testing.T) {
	cases := []struct {
		name    string
		blob    string
		want    ZoneSelection
		wantErr bool
	}{
		{"object form", `{"iana":"Europe/Berlin","name":"Ramstein"}`, ZoneSelection{IANA: "Europe/Berlin", Name: "Ramstein"}, false},
		{"object with no name", `{"iana":"UTC"}`, ZoneSelection{IANA: "UTC"}, false},
		// Written before names existed. It reads as an unnamed zone, which is
		// exactly what it was.
		{"bare identifier", `"Asia/Tokyo"`, ZoneSelection{IANA: "Asia/Tokyo"}, false},

		{"number", `5`, ZoneSelection{}, true},
		{"array", `[1,2]`, ZoneSelection{}, true},
		{"iana of the wrong type", `{"iana":5}`, ZoneSelection{}, true},
		{"name of the wrong type", `{"iana":"UTC","name":[]}`, ZoneSelection{}, true},
		{"not json at all", `{`, ZoneSelection{}, true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var got ZoneSelection
			err := json.Unmarshal([]byte(tc.blob), &got)

			if tc.wantErr {
				if err == nil {
					t.Fatalf("Unmarshal(%s) = %+v, want an error", tc.blob, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("Unmarshal(%s) = %v, want nil", tc.blob, err)
			}
			if got != tc.want {
				t.Fatalf("Unmarshal(%s) = %+v, want %+v", tc.blob, got, tc.want)
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
	err := prefs.validate()
	if err == nil {
		t.Fatalf("validate() accepted %d zones, want a rejection above %d",
			len(tooManyZones), maxZones)
	}
	assertCode(t, err.Error(), errcode.PreferencesTooManyZones)
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
			if err != nil {
				assertCode(t, err.Error(), errcode.PreferencesThresholdOutOfRange)
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

	err := store.Set(testUserID, UserPreferences{
		Version: 1,
		DTG: DTGPreferences{
			Zones:               []ZoneSelection{{IANA: "UTC"}},
			UrgentWithinMinutes: 5,
		},
		Location: LocationPreferences{HiddenRows: []string{"ddm"}},
		Cot:      CotPreferences{HiddenSections: []string{"payload"}},
	})
	if err != nil {
		t.Fatalf("Set() = %v", err)
	}

	var raw map[string]any
	if err := json.Unmarshal(api.kv[preferenceKey(testUserID)], &raw); err != nil {
		t.Fatalf("stored blob is not JSON: %v", err)
	}

	// Every name the webapp reads or writes, checked by name. Renaming one
	// silently discards everybody's saved settings on upgrade, and nothing else
	// in either language would notice: webapp/src/preferences/types.ts is the
	// other half of this contract.
	for _, tc := range []struct{ section, field string }{
		{"dtg", "zones"},
		{"dtg", "urgent_within_minutes"},
		{"location", "hidden_rows"},
		{"cot", "hidden_sections"},
	} {
		t.Run(tc.section+"."+tc.field, func(t *testing.T) {
			section, ok := raw[tc.section].(map[string]any)
			if !ok {
				t.Fatalf("stored blob has no %q object: %v", tc.section, raw)
			}
			if _, ok := section[tc.field]; !ok {
				t.Fatalf("stored blob has no %s.%s field: %v", tc.section, tc.field, section)
			}
		})
	}
}

// The mirror of TestKVStoreSurfacesWriteFailures. "Restore defaults" is a
// delete, so a failure here is the one the reader sees as TF-13007.
func TestKVStoreSurfacesDeleteFailures(t *testing.T) {
	api := newPreferenceAPI()
	api.kvDeleteErr = model.NewAppError("KVDelete", "kv.delete", nil, "boom", 500)
	store := &kvPreferenceStore{api: api}

	err := store.Delete(testUserID)
	if err == nil {
		t.Fatal("Delete() succeeded despite the KV store failing")
	}
	if !strings.Contains(err.Error(), "failed to clear preferences") {
		t.Fatalf("Delete() = %v, want the failure wrapped with what was being attempted", err)
	}
}

// The hidden-row selection: what a reader may store, and what is refused.
func TestValidateHiddenRows(t *testing.T) {
	for _, tc := range []struct {
		name  string
		rows  []string
		want  []string
		fails bool
	}{
		{name: "nothing hidden is the default", rows: nil, want: []string{}},
		{name: "a row this build renders", rows: []string{"ddm"}, want: []string{"ddm"}},

		// The map is hideable and is NOT a row, so it travels by its own
		// constant. Revert the widening in KnownRow and every other test here
		// stays green while a reader can never hide the map.
		{name: "the map, which is hideable but is not a row", rows: []string{"map"}, want: []string{"map"}},

		// The map under a post, likewise. Revert the widening in KnownRow and
		// every other test here stays green while a reader can never hide it.
		{name: "the map under a post, hideable and not a row", rows: []string{"inline"}, want: []string{"inline"}},
		{
			name: "duplicates collapse, because they were never two rows",
			rows: []string{"ddm", "ddm", "datum"}, want: []string{"ddm", "datum"},
		},
		{name: "blanks are dropped rather than stored", rows: []string{"", "  ", "dms"}, want: []string{"dms"}},
		{name: "surrounding space is trimmed", rows: []string{" dms "}, want: []string{"dms"}},

		// Refused rather than dropped, for the same reason a bad timezone is:
		// it can only come from a hand-written request or a bug, and storing
		// something that will never do anything reports success for a setting
		// that silently does not exist.
		{name: "a row this build does not have", rows: []string{"sextant"}, fails: true},
		{name: "a row named by its label rather than its id", rows: []string{"Lat / lon"}, fails: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			prefs := UserPreferences{Location: LocationPreferences{HiddenRows: tc.rows}}

			err := prefs.validate()
			if tc.fails {
				if err == nil {
					t.Fatalf("validate() accepted %v", tc.rows)
				}
				// The code, not merely an error: it is what the reader quotes
				// and what public/help/error-codes.html is organized around.
				assertCode(t, err.Error(), errcode.PreferencesRowUnknown)
				return
			}
			if err != nil {
				t.Fatalf("validate() rejected %v: %v", tc.rows, err)
			}

			if len(prefs.Location.HiddenRows) != len(tc.want) {
				t.Fatalf("HiddenRows = %v, want %v", prefs.Location.HiddenRows, tc.want)
			}
			for i, id := range tc.want {
				if prefs.Location.HiddenRows[i] != id {
					t.Fatalf("HiddenRows = %v, want %v", prefs.Location.HiddenRows, tc.want)
				}
			}
		})
	}
}

// The same table for the Cursor on Target panel's sections.
//
// Section ids reach the KV store exactly as row ids do, so the two halves of
// the contract are the same: refuse an id this build cannot honor, and collapse
// what is only a difference in spelling.
func TestValidateNormalizesTheHiddenSections(t *testing.T) {
	for _, tc := range []struct {
		name     string
		sections []string
		want     []string
		fails    bool
	}{
		{name: "nothing hidden is the default", sections: nil, want: []string{}},
		{name: "a section this build renders", sections: []string{"payload"}, want: []string{"payload"}},

		// The panel's map travels by a section id of its own rather than by the
		// location decorator's "inline", because a sidebar map is not a map
		// under a post. Revert that and a reader can never hide this one while
		// every other test here stays green.
		{name: "the panel map, which is the cot section rather than the location one", sections: []string{"map"}, want: []string{"map"}},
		{
			name:     "duplicates collapse, because they were never two sections",
			sections: []string{"payload", "payload", "flow"}, want: []string{"payload", "flow"},
		},
		{name: "blanks are dropped rather than stored", sections: []string{"", "  ", "shape"}, want: []string{"shape"}},
		{name: "surrounding space is trimmed", sections: []string{" shape "}, want: []string{"shape"}},

		{name: "a section this build does not have", sections: []string{"telepathy"}, fails: true},
		{name: "a section named by its label rather than its id", sections: []string{"Processing path"}, fails: true},

		// A location row id is not a section id. The two catalogs are separate
		// namespaces and accepting one for the other would store a setting that
		// silently does nothing.
		{name: "a location row id", sections: []string{"ddm"}, fails: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			prefs := UserPreferences{Cot: CotPreferences{HiddenSections: tc.sections}}

			err := prefs.validate()
			if tc.fails {
				if err == nil {
					t.Fatalf("validate() accepted %v", tc.sections)
				}
				assertCode(t, err.Error(), errcode.PreferencesSectionUnknown)
				return
			}
			if err != nil {
				t.Fatalf("validate() rejected %v: %v", tc.sections, err)
			}

			if len(prefs.Cot.HiddenSections) != len(tc.want) {
				t.Fatalf("HiddenSections = %v, want %v", prefs.Cot.HiddenSections, tc.want)
			}
			for i, id := range tc.want {
				if prefs.Cot.HiddenSections[i] != id {
					t.Fatalf("HiddenSections = %v, want %v", prefs.Cot.HiddenSections, tc.want)
				}
			}
		})
	}
}

// Every section the panel renders can be hidden, read from the catalog so a
// section added later is covered without anybody remembering to come back here.
func TestEverySectionCanBeHidden(t *testing.T) {
	ids := make([]string, 0, len(cot.Sections))
	for _, section := range cot.Sections {
		ids = append(ids, section.ID)
	}

	prefs := UserPreferences{Cot: CotPreferences{HiddenSections: ids}}

	if err := prefs.validate(); err != nil {
		t.Fatalf("validate() refused to hide every section: %v", err)
	}
	if len(prefs.Cot.HiddenSections) != len(ids) {
		t.Fatalf("kept %d of %d sections", len(prefs.Cot.HiddenSections), len(ids))
	}
}

// The cache hands the same value to every caller, so a caller that appended to
// the hidden sections would be editing what the next reader gets.
func TestCloneCopiesTheHiddenSections(t *testing.T) {
	original := UserPreferences{Cot: CotPreferences{HiddenSections: []string{"payload"}}}

	clone := original.clone()
	clone.Cot.HiddenSections[0] = "flow"

	if original.Cot.HiddenSections[0] != "payload" {
		t.Fatalf("the original was modified through its clone: %v", original.Cot.HiddenSections)
	}
}

// Every row the panel renders can be hidden, and nothing else can.
//
// Written against the catalog rather than a hand-typed list, so a row added
// later is covered without anybody remembering to come back here.
func TestEveryRowCanBeHidden(t *testing.T) {
	prefs := UserPreferences{Location: LocationPreferences{HiddenRows: location.AllRowIDs}}

	if err := prefs.validate(); err != nil {
		t.Fatalf("validate() refused to hide every row: %v", err)
	}
	if len(prefs.Location.HiddenRows) != len(location.AllRowIDs) {
		t.Fatalf("kept %d of %d rows", len(prefs.Location.HiddenRows), len(location.AllRowIDs))
	}
}

// The cache hands the same value to every caller, so a caller that appended to
// the hidden rows would be editing what the next reader gets.
func TestCloneCopiesTheHiddenRows(t *testing.T) {
	original := UserPreferences{Location: LocationPreferences{HiddenRows: []string{"ddm"}}}

	clone := original.clone()
	clone.Location.HiddenRows[0] = "datum"

	if original.Location.HiddenRows[0] != "ddm" {
		t.Fatalf("the original was modified through its clone: %v", original.Location.HiddenRows)
	}
}

func TestValidZoneIDRefusesAShapeBeforeItResolvesAnything(t *testing.T) {
	for _, tc := range []struct {
		name string
		zone string
	}{
		{"empty", ""},
		{"not an identifier", "Europe/Berlin; DROP"},
		{"longer than any identifier", strings.Repeat("a", maxZoneIDLength+1)},
		{"the server's own zone", "Local"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			shapeErr := validZoneShape(tc.zone)
			if shapeErr == nil {
				t.Fatalf("validZoneShape(%q) accepted it, so this no longer exercises the shape gate", tc.zone)
			}

			err := validZoneID(tc.zone)
			if err == nil {
				t.Fatalf("validZoneID(%q) accepted it", tc.zone)
			}
			if err.Error() != shapeErr.Error() {
				t.Errorf("validZoneID returned %q, want the shape error %q", err, shapeErr)
			}
		})
	}
}

func TestValidZoneIDRefusesAZoneTheServerCannotResolve(t *testing.T) {
	if err := validZoneID("Mars/Olympus"); err == nil {
		t.Fatal("an unresolvable zone was accepted")
	}
	if err := validZoneID("Europe/Berlin"); err != nil {
		t.Fatalf("a real zone was refused: %v", err)
	}
}
