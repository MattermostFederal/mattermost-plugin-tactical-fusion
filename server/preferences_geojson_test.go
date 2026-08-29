package main

import (
	"testing"

	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/geojson"
)

// An unknown id is refused rather than dropped, for the reason validHiddenRows
// records: it can only come from a hand-written request or a bug, and quietly
// storing something that will never do anything reports success for a setting
// that silently does not exist.
func TestAnUnknownGeoJSONSectionIsRefused(t *testing.T) {
	if _, err := validGeoJSONSections([]string{"not-a-section"}); err == nil {
		t.Fatal("an unknown section id was accepted")
	}
}

// The two panels keep separate vocabularies, so a Cursor on Target section is
// not a GeoJSON one. Sharing a key would mean hiding "Map" on one hid it on the
// other.
func TestACotSectionIsNotAGeoJSONSection(t *testing.T) {
	if _, err := validGeoJSONSections([]string{"payload"}); err == nil {
		t.Fatal("a Cursor on Target section id was accepted for the GeoJSON panel")
	}
}

func TestEveryGeoJSONSectionInTheCatalogIsAccepted(t *testing.T) {
	ids := make([]string, 0, len(geojson.Sections))
	for _, section := range geojson.Sections {
		ids = append(ids, section.ID)
	}

	stored, err := validGeoJSONSections(ids)
	if err != nil {
		t.Fatalf("the catalog's own ids were refused: %v", err)
	}
	if len(stored) != len(ids) {
		t.Fatalf("stored %d of %d ids", len(stored), len(ids))
	}
}

func TestGeoJSONSectionsAreDeduplicated(t *testing.T) {
	stored, err := validGeoJSONSections([]string{"map", "map", " map ", "source"})
	if err != nil {
		t.Fatalf("validGeoJSONSections: %v", err)
	}
	if len(stored) != 2 {
		t.Fatalf("stored %v, want two entries", stored)
	}
}

// The blob is handed to every caller from one cache, so a caller that appends
// to its own copy must not be editing what the next reader gets.
func TestClonedPreferencesShareNoGeoJSONSlice(t *testing.T) {
	original := UserPreferences{GeoJSON: GeoJSONPreferences{HiddenSections: []string{"map"}}}

	clone := original.clone()
	clone.GeoJSON.HiddenSections[0] = "source"

	if original.GeoJSON.HiddenSections[0] != "map" {
		t.Fatal("the clone shares its slice with the original")
	}
}

/*
 * Through validate(), not through validGeoJSONSections.
 *
 * Every other test in this file calls the validator directly, so deleting the
 * call from UserPreferences.validate() left all of them green while a
 * hand-written PUT of an unknown id was stored and reported successful.
 */
func TestValidateRefusesAnUnknownGeoJSONSection(t *testing.T) {
	prefs := UserPreferences{GeoJSON: GeoJSONPreferences{HiddenSections: []string{"telepathy"}}}

	if err := prefs.validate(); err == nil {
		t.Fatal("validate() accepted a section this build does not render")
	}
}

func TestValidateNormalizesTheGeoJSONSections(t *testing.T) {
	prefs := UserPreferences{
		GeoJSON: GeoJSONPreferences{HiddenSections: []string{"map", " map ", "source"}},
	}

	if err := prefs.validate(); err != nil {
		t.Fatalf("validate(): %v", err)
	}
	if len(prefs.GeoJSON.HiddenSections) != 2 {
		t.Fatalf("stored %v, want the duplicate removed", prefs.GeoJSON.HiddenSections)
	}
}
