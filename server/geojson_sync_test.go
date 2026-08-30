package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/geojson"
)

func readGeoJSONTypes(t *testing.T) string {
	t.Helper()

	path := filepath.Join("..", "webapp", "src", "geojson", "types.ts")
	raw, err := os.ReadFile(path) // #nosec G304 -- fixed, repo-relative source path
	if err != nil {
		t.Fatalf("could not read %s: %v", path, err)
	}
	return string(raw)
}

func readGeoJSONSections(t *testing.T) string {
	t.Helper()

	path := filepath.Join("..", "webapp", "src", "geojson", "sections.ts")
	raw, err := os.ReadFile(path) // #nosec G304 -- fixed, repo-relative source path
	if err != nil {
		t.Fatalf("could not read %s: %v", path, err)
	}
	return string(raw)
}

// The hideable sections are the same catalog in the same order on both sides.
//
// Two things have to agree about what they are: the server refuses a
// hidden-section id it does not know, and the panel renders the sections and
// the editor lists them. A section the webapp has and this package does not
// cannot be hidden at all, because the save is refused; a section this package
// has and the webapp does not is a tickbox that changes nothing. Neither is an
// error at runtime, which is why it is one here.
//
// The ORDER matters too, since it is the order the panel draws and the editor
// lists, and the LABEL is what a reader matches a checkbox against a heading.
func TestWebappGeoJSONSectionCatalogMatches(t *testing.T) {
	source := readGeoJSONSections(t)

	found := regexp.MustCompile(
		`\{id: '([a-z]+)', label: '([^']*)'`).FindAllStringSubmatch(source, -1)
	if len(found) == 0 {
		t.Fatal("no sections parsed out of the webapp's geojson/sections.ts; if the shape " +
			"changed, point this test at the new one rather than deleting it")
	}

	if len(found) != len(geojson.Sections) {
		t.Fatalf("the webapp has %d sections and this package has %d", len(found), len(geojson.Sections))
	}

	for i, m := range found {
		if m[1] != geojson.Sections[i].ID {
			t.Errorf("section %d is %q in the webapp and %q here; the two must be the same list "+
				"in the same order", i+1, m[1], geojson.Sections[i].ID)
		}
		if m[2] != geojson.Sections[i].Label {
			t.Errorf("section %q is labeled %q in the webapp and %q here", m[1], m[2], geojson.Sections[i].Label)
		}
	}
}

// The post type, props key, props version and source kinds are the same on both
// sides.
//
// Folded into one test the way TestWebappCotPostTypeMatches folds the source
// kinds in: fromProps refuses a blob naming neither source, so a drift on any
// of them falls every card back to the post's own text, which is one silent
// failure with one cause.
func TestWebappGeoJSONPostTypeMatches(t *testing.T) {
	source := readGeoJSONTypes(t)

	constants := map[string]string{
		"GEOJSON_POST_TYPE":     geojson.PostType,
		"GEOJSON_PROPS_KEY":     geojson.PropsKey,
		"GEOJSON_PROPS_VERSION": "1",

		"SOURCE_FENCE": geojson.SourceFence,
		"SOURCE_FILE":  geojson.SourceFile,
	}

	for name, want := range constants {
		pattern := regexp.MustCompile(`export const ` + name + ` = '?([^';]+)'?;`)
		m := pattern.FindStringSubmatch(source)
		if m == nil {
			t.Fatalf("no `export const %s` in the webapp's geojson/types.ts; if it was renamed, "+
				"point this test at the new name rather than deleting it", name)
		}
		if m[1] != want {
			t.Errorf("%s = %q in the webapp, %q in Go", name, m[1], want)
		}
	}

	if geojson.PropsVersion != 1 {
		t.Errorf("PropsVersion is %d; update the webapp constant and this test together", geojson.PropsVersion)
	}
}

// The geometry classes are the same closed vocabulary in the same order.
//
// Both the card and the map dispatch on this string, and an unknown one falls
// back to "none", which is a feature listed as unlocated rather than drawn. A
// kind Go can write and the webapp cannot name is therefore a feature silently
// missing from the map, with nothing reporting it.
func TestWebappGeoJSONKindsMatch(t *testing.T) {
	source := readGeoJSONTypes(t)

	block := regexp.MustCompile(`(?s)export const GEOJSON_KINDS = \[(.*?)\] as const;`).FindStringSubmatch(source)
	if block == nil {
		t.Fatal("no `export const GEOJSON_KINDS` in the webapp's geojson/types.ts; if it was " +
			"renamed, point this test at the new name rather than deleting it")
	}

	var webapp []string
	for _, m := range regexp.MustCompile(`'([A-Za-z]+)'`).FindAllStringSubmatch(block[1], -1) {
		webapp = append(webapp, m[1])
	}

	if !slices.Equal(webapp, geojson.Kinds) {
		t.Errorf("kinds differ:\n webapp %v\n Go     %v", webapp, geojson.Kinds)
	}
}

// Every key Go writes must be read by the webapp, and every key the webapp reads
// must be one Go can write.
//
// The blob is mostly nested, so this walks it rather than scraping flat reads
// the way the Cursor on Target guard does: counts, features, parts, rings,
// positions and properties are all below the top level, and a scraper would
// have gone quiet on every coordinate.
func TestWebappGeoJSONShapeMatches(t *testing.T) {
	source := readGeoJSONTypes(t)

	webappKeys := map[string]bool{}
	for _, m := range regexp.MustCompile(`(?:text|count)\((\w+), '([a-z0-9_]+)'\)`).FindAllStringSubmatch(source, -1) {
		webappKeys[m[2]] = true
	}
	// Only the RAW wire records count as a read. The reader names its
	// parameters rawBlob/rawFeature/rawPart precisely so this can tell them
	// apart from the typed values, where `feature.format` is an ordinary
	// property access on a decoded object and proves nothing about the wire.
	// Matching a bare `feature.` was the hole: a helper elsewhere in the module
	// satisfied the guard while the reader had stopped reading the key at all.
	for _, m := range regexp.MustCompile(`raw(?:Blob|Feature|Part)\.([a-z_]+)`).FindAllStringSubmatch(source, -1) {
		webappKeys[m[1]] = true
	}
	if len(webappKeys) == 0 {
		t.Fatal("no reads found in the webapp's geojson/types.ts; if the reader was rewritten, " +
			"point this test at the new shape rather than deleting it")
	}

	document, err := geojson.Parse([]byte(geojson.Fixture()))
	if err != nil {
		t.Fatalf("the fixture does not parse: %v", err)
	}

	blob := geojson.Props(document, geojson.Source{
		Kind: geojson.SourceFile, Lead: "note", Text: geojson.Fixture(),
		FileID: "abcdefghijklmnopqrstuvwxyz", FileName: "overlay.geojson",
	})

	goKeys := map[string]bool{}
	collectKeys(blob, goKeys)

	// properties_dropped is written only by the lower rung, so the widest one
	// cannot produce it and it is asserted separately below.
	optional := map[string]bool{"properties_dropped": true, "ring_counts": true, "alt": true}

	// unplaceable is a presence key the widest rung leaves empty.
	wireOnlyKeys := map[string]bool{"unplaceable": true}

	for key := range goKeys {
		if !webappKeys[key] && !optional[key] {
			t.Errorf("Go writes %q and the webapp never reads it", key)
		}
	}

	// The OTHER direction, which this test's own header claimed and did not
	// implement. Without it a key the webapp reads and Go never writes reads as
	// the empty string forever, which renders as a blank row rather than as an
	// error. cot_sync_test.go has always checked both.
	for key := range webappKeys {
		if !goKeys[key] && !optional[key] && !wireOnlyKeys[key] {
			t.Errorf("the webapp reads %q but Go never writes it", key)
		}
	}

	lower := geojson.PropsWithoutProperties(document, geojson.Source{Kind: geojson.SourceFence})
	if _, present := lower["properties_dropped"]; !present {
		t.Error("the lower rung wrote no properties_dropped marker")
	}
	if !strings.Contains(source, "properties_dropped") {
		t.Error("the webapp never reads properties_dropped, so a degraded card cannot say so")
	}

	// Every optional key has to be reachable from the fixture, or the guard is
	// passing on a shape it never saw.
	for key := range optional {
		if key == "properties_dropped" {
			continue
		}
		if !goKeys[key] {
			t.Errorf("the fixture produced no %q, so the guard never covered it", key)
		}
	}
}

func collectKeys(value any, into map[string]bool) {
	switch typed := value.(type) {
	case map[string]any:
		for key, item := range typed {
			into[key] = true
			collectKeys(item, into)
		}
	case []any:
		for _, item := range typed {
			collectKeys(item, into)
		}
	}
}

// TestWebappGeoJSONTruncatesNothing holds the reader to rendering exactly what
// Go produced.
//
// The server refuses every document past its own caps rather than truncating
// one, so a cap repeated on the webapp side could only disagree with it: a ring
// cut short there would close onto the wrong vertex and draw a polygon nobody
// posted. This is why there is no caps sync row.
func TestWebappGeoJSONTruncatesNothing(t *testing.T) {
	source := readGeoJSONTypes(t)

	for _, forbidden := range []string{".slice(", ".substring(", "MAX_"} {
		if strings.Contains(source, forbidden) {
			t.Errorf("the webapp's geojson/types.ts contains %q; it must render what Go produced "+
				"rather than re-capping it, because the two sides do not fail alike", forbidden)
		}
	}
}

// The GeoJSON card reads the location decorator's map switches rather than
// declaring its own, and this is what keeps that decision from being quietly
// reversed.
func TestGeoJSONHasNoMapSettingOfItsOwn(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "plugin.json"))
	if err != nil {
		t.Fatalf("could not read plugin.json: %v", err)
	}

	var manifest struct {
		SettingsSchema struct {
			Sections []struct {
				Settings []struct {
					Key string `json:"key"`
				} `json:"settings"`
			} `json:"sections"`
		} `json:"settings_schema"`
	}
	if err := json.Unmarshal(raw, &manifest); err != nil {
		t.Fatalf("plugin.json does not parse: %v", err)
	}

	checked := 0
	for _, section := range manifest.SettingsSchema.Sections {
		for _, setting := range section.Settings {
			checked++
			if strings.Contains(setting.Key, "GeoJSON") && strings.Contains(setting.Key, "Map") {
				t.Errorf("%s declares a GeoJSON map switch; the card reads the location "+
					"decorator's switches so the parent ANDs cannot be re-implemented differently", setting.Key)
			}
		}
	}

	// Without this the test passes just as happily against a manifest whose
	// settings moved somewhere this loop does not look.
	if checked == 0 {
		t.Fatal("no settings parsed out of plugin.json; point this test at the new shape " +
			"rather than leaving it to guard nothing")
	}
}

/*
 * The marker-size vocabulary, both halves.
 *
 * Go refuses anything outside it and the webapp scales the reticle by it, and
 * until now nothing tied the two lists together. Every other Go/TypeScript
 * duplicate in this repo carries a guard; this one did not, so a size added on
 * one side would have been silently refused or silently undrawn on the other.
 */
func TestWebappMarkerSizesMatch(t *testing.T) {
	source := readWebappFile(t, "decorators", "location", "map", "overlay.ts")

	match := regexp.MustCompile(`MARKER_SIZES = \[([^\]]*)\]`).FindStringSubmatch(source)
	if match == nil {
		t.Fatal("no MARKER_SIZES in map/overlay.ts; if it was renamed, point this " +
			"test at the new name rather than deleting it")
	}

	webapp := map[string]bool{}
	for _, quoted := range regexp.MustCompile(`'([a-z]+)'`).FindAllStringSubmatch(match[1], -1) {
		webapp[quoted[1]] = true
	}

	for size := range geojson.MarkerSizes() {
		if !webapp[size] {
			t.Errorf("Go accepts marker-size %q and the webapp does not scale it", size)
		}
		delete(webapp, size)
	}

	for size := range webapp {
		t.Errorf("the webapp scales marker-size %q, which Go refuses", size)
	}
}
