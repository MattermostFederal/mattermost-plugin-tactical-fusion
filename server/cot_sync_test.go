package main

import (
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/cot"
)

func readCotTypes(t *testing.T) string {
	t.Helper()

	path := filepath.Join("..", "webapp", "src", "cot", "types.ts")
	raw, err := os.ReadFile(path) // #nosec G304 -- fixed, repo-relative source path
	if err != nil {
		t.Fatalf("could not read %s: %v", path, err)
	}
	return string(raw)
}

func readCotSections(t *testing.T) string {
	t.Helper()

	path := filepath.Join("..", "webapp", "src", "cot", "sections.ts")
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
func TestWebappCotSectionCatalogMatches(t *testing.T) {
	source := readCotSections(t)

	found := regexp.MustCompile(
		`\{id: '([a-z]+)', label: '([^']*)'`).FindAllStringSubmatch(source, -1)
	if len(found) == 0 {
		t.Fatal("no sections parsed out of the webapp's sections.ts; if the shape changed, " +
			"point this test at the new one rather than deleting it")
	}

	if len(found) != len(cot.Sections) {
		t.Fatalf("the webapp has %d sections and this package has %d", len(found), len(cot.Sections))
	}

	for i, m := range found {
		if m[1] != cot.Sections[i].ID {
			t.Errorf("section %d is %q in the webapp and %q here; the two must be the same list "+
				"in the same order", i+1, m[1], cot.Sections[i].ID)
		}
		if m[2] != cot.Sections[i].Label {
			t.Errorf("section %q is labeled %q in the webapp and %q here", m[1], m[2], cot.Sections[i].Label)
		}
	}
}

// The post type, the props key and the props version are the three strings that
// decide whether a stamped post is rendered at all. A drift in any of them is a
// post whose body falls through to Mattermost's own markdown, silently, on
// exactly the posts this feature exists for.
func TestWebappCotPostTypeMatches(t *testing.T) {
	source := readCotTypes(t)

	constants := map[string]string{
		"COT_POST_TYPE":     cot.PostType,
		"COT_PROPS_KEY":     cot.PropsKey,
		"COT_PROPS_VERSION": "2",

		// Not decoration. fromProps refuses a source naming neither, so a
		// drift on either side falls every card back to the post's own text,
		// which is the same silent failure the three above are guarded for.
		"SOURCE_FENCE": cot.SourceFence,
		"SOURCE_FILE":  cot.SourceFile,
	}

	for name, want := range constants {
		pattern := regexp.MustCompile(`export const ` + name + ` = '?([^';]+)'?;`)
		m := pattern.FindStringSubmatch(source)
		if m == nil {
			t.Fatalf("no `export const %s` in the webapp's cot/types.ts; if it was renamed, "+
				"point this test at the new name rather than deleting it", name)
		}
		if m[1] != want {
			t.Errorf("%s = %q in the webapp, %q in Go", name, m[1], want)
		}
	}

	if cot.PropsVersion != 2 {
		t.Errorf("PropsVersion is %d; update the webapp constant and this test together", cot.PropsVersion)
	}
}

// Every key Go writes must be read by the webapp, and every key the webapp reads
// must be one Go can write. A name that drifts reads as the empty string, which
// renders as a blank row rather than as an error, so nothing would report it.
func TestWebappCotShapeMatches(t *testing.T) {
	source := readCotTypes(t)

	webappKeys := map[string]bool{}
	for _, m := range regexp.MustCompile(`text\((?:blob|event), '([a-z0-9_]+)'\)`).FindAllStringSubmatch(source, -1) {
		webappKeys[m[1]] = true
	}
	if len(webappKeys) == 0 {
		t.Fatal("no text(...) reads found in the webapp's cot/types.ts; if the reader was " +
			"rewritten, point this test at the new shape rather than deleting it")
	}

	events, err := cot.Parse([]byte(fullCotEvent))
	if err != nil {
		t.Fatalf("the fixture does not parse: %v", err)
	}

	props := cot.Props(events, cot.Source{
		Kind: cot.SourceFile, Lead: "note", Trail: "", Text: fullCotEvent,
		FileID: "f1", FileName: "event.cot",
	})

	var goKeys []string
	for key, value := range props {
		if key == "version" {
			continue
		}
		if rendered, ok := value.([]any); ok {
			for _, entry := range rendered {
				for innerKey := range entry.(map[string]any) {
					goKeys = append(goKeys, innerKey)
				}
			}
			continue
		}
		goKeys = append(goKeys, key)
	}
	slices.Sort(goKeys)

	// Every registry entry has to reach this comparison, or the guard narrows
	// silently as entries are added. The fixture is built from the registry for
	// exactly that reason, so assert it actually produced them.
	for _, key := range cot.PropKeys() {
		if !slices.Contains(goKeys, key) {
			t.Errorf("the registry declares %q and the fixture did not produce it", key)
		}
	}

	for _, key := range goKeys {
		if cotNonTextKeys[key] {
			continue
		}
		if !webappKeys[key] {
			t.Errorf("Go writes %q but the webapp never reads it", key)
		}
	}

	for key := range webappKeys {
		if !slices.Contains(goKeys, key) && !cotOptionalKeys[key] {
			t.Errorf("the webapp reads %q but Go never writes it", key)
		}
	}
}

// Keys Go omits when it has nothing to say. The webapp still reads them, since
// absent and empty mean the same thing to it, so they are not drift.
// Keys the webapp reads through something other than text(), which is the only
// shape the scraper above recognizes. TestWebappReadsTheProcessingPath is what
// holds this one instead.
var cotNonTextKeys = map[string]bool{"flow": true, "geometry": true, "checklist": true}

var cotOptionalKeys = map[string]bool{
	"position_note": true,

	// Written only by PropsWithoutDetail, the middle rung of the hook's budget
	// ladder, so the full blob this fixture builds never carries it.
	// TestTheDegradedBlobSaysSo is what holds it instead.
	"detail_dropped": true,

	"start":   true,
	"src":     true,
	"parent":  true,
	"related": true,

	// Version 1 wrote one "event" where version 2 writes an "events" array. The
	// webapp still reads it, for posts stamped before the bump.
	"event": true,
}

// The processing path is an ordered array rather than a rendered string, so it
// has a reader of its own rather than a text() call. It still has to have one.
func TestWebappReadsTheProcessingPath(t *testing.T) {
	source := readCotTypes(t)

	if !strings.Contains(source, "readFlow") {
		t.Error("the webapp has no readFlow; Go writes an ordered flow array that nothing reads")
	}
	if !strings.Contains(source, "event.flow") {
		t.Error("the webapp never reads the flow key")
	}
}

// Geometry is a shape rather than a rendered string, so it has its own reader
// too. Without one the map draws nothing and no other guard would say so.
func TestWebappReadsTheGeometry(t *testing.T) {
	source := readCotTypes(t)

	if !strings.Contains(source, "readGeometry") {
		t.Error("the webapp has no readGeometry; Go writes a shape that nothing reads")
	}
	if !strings.Contains(source, "event.geometry") {
		t.Error("the webapp never reads the geometry key")
	}
}

// A checklist is counted rather than decoded, so its key holds a nested object
// with an ordered array inside and the text() scraper cannot see it either.
func TestWebappReadsTheChecklist(t *testing.T) {
	source := readCotTypes(t)

	if !strings.Contains(source, "readChecklist") {
		t.Error("the webapp has no readChecklist; Go writes a checklist that nothing reads")
	}
	if !strings.Contains(source, "event.checklist") {
		t.Error("the webapp never reads the checklist key")
	}
}

// Every class the server can write needs a layout on the other side. A class the
// webapp does not know falls to the default, which is today's card, so the cost
// is a silent loss of the layout rather than a broken render. That is still a
// drift nothing else would report.
func TestWebappCotClassesMatch(t *testing.T) {
	source := readCotTypes(t)

	_, block, found := strings.Cut(source, "export const COT_CLASSES = [")
	if !found {
		t.Fatal("COT_CLASSES is not in cot/types.ts")
	}
	if end, _, closed := strings.Cut(block, "]"); closed {
		block = end
	}

	for _, class := range cot.Classes() {
		if !strings.Contains(block, "'"+class+"'") {
			t.Errorf("the server writes class %q and the webapp does not name it", class)
		}
	}

	for _, m := range regexp.MustCompile(`'([a-z_]+)'`).FindAllStringSubmatch(block, -1) {
		if !slices.Contains(cot.Classes(), m[1]) {
			t.Errorf("the webapp names class %q, which the server never writes", m[1])
		}
	}
}

// fullCotEvent carries every <detail> extension this build reads, because it is
// built from the registry rather than written by hand. A hand-written fixture
// stops covering the moment somebody adds an entry and forgets to extend it,
// and the guard would go quiet rather than fail.
//
// The type is b-t-f so the blob also carries a class, which is otherwise absent.
var fullCotEvent = `<event version="2.0" uid="ANDROID-1" type="b-t-f" how="m-g" ` +
	`time="2026-08-23T11:43:38Z" start="2026-08-23T11:43:38Z" stale="2026-08-23T11:45:38Z">` +
	`<point lat="34.056100" lon="-118.250000" hae="-42.6" ce="45.3" le="99.5"/>` +
	`<detail>` + cot.FixtureDetail() + cot.FixtureGeometry() + cot.FixtureChecklist() +
	`<mystery-element/></detail></event>`

// The card reads the map switch the location decorator already owns, so there is
// no CoT map setting and no new /features field. If somebody adds one, this test
// is where the decision gets revisited rather than silently reversed.
func TestCotHasNoMapSettingOfItsOwn(t *testing.T) {
	raw, err := os.ReadFile("../plugin.json")
	if err != nil {
		t.Fatalf("could not read plugin.json: %v", err)
	}

	if strings.Contains(string(raw), "EnableCotMap") {
		t.Error("plugin.json declares EnableCotMap; the CoT card reads EnableLocationMapInline, " +
			"whose parent ANDs already live in Go")
	}
}

// Every affiliation the decoder can name has a word in the webapp.
//
// The words are the map label's only channel: color is what distinguishes one
// marker from another, and a screen reader gets none of it. An affiliation the
// server decodes and the webapp cannot name falls through to "unstated", which
// describes a value this build is holding as though nothing were known about
// it. That is the ignorance the card refuses to claim everywhere else.
func TestWebappAffiliationWordsMatch(t *testing.T) {
	source := readCotTypes(t)

	_, block, found := strings.Cut(source, "const AFFILIATION_WORDS: Record<string, string> = {")
	if !found {
		t.Fatal("AFFILIATION_WORDS is not in cot/types.ts")
	}
	if end, _, closed := strings.Cut(block, "};"); closed {
		block = end
	}

	keys := map[string]bool{}
	for _, m := range regexp.MustCompile(`(?m)^\s*'?([a-z-]+)'?:`).FindAllStringSubmatch(block, -1) {
		keys[m[1]] = true
	}

	if len(keys) == 0 {
		t.Fatal("no affiliation words were read, so this test is checking nothing")
	}

	for _, id := range cot.AffiliationIDs() {
		if !keys[id] {
			t.Errorf("the server decodes affiliation %q and the webapp has no word for it", id)
		}
		delete(keys, id)
	}

	for extra := range keys {
		t.Errorf("the webapp names affiliation %q, which the server never produces", extra)
	}
}

// The three caps the webapp applies to a props blob it does not trust.
//
// Each carried a comment naming its Go counterpart and nothing held it to one.
// They do not fail alike, which is why all three are here: past MaxVertices the
// server REFUSES the shape and says so in a note while the webapp TRUNCATES and
// still draws, so a raise on the Go side alone draws a polygon missing corners
// as though it were the whole shape. The other two shorten a list.
func TestWebappBlobCapsMatch(t *testing.T) {
	source := readCotTypes(t)

	caps := map[string]int{
		"MAX_VERTICES":        cot.MaxVertices,
		"MAX_CHECKLIST_KINDS": cot.MaxChecklistKinds,
		"MAX_EVENTS":          cot.MaxEvents,
	}

	for name, want := range caps {
		pattern := regexp.MustCompile(`export const ` + name + ` = (\d+);`)
		m := pattern.FindStringSubmatch(source)
		if m == nil {
			t.Fatalf("no `export const %s` in the webapp's cot/types.ts; if it was renamed, "+
				"point this test at the new name rather than deleting it", name)
		}

		got, err := strconv.Atoi(m[1])
		if err != nil {
			t.Fatalf("%s is %q in the webapp, which is not a number", name, m[1])
		}
		if got != want {
			t.Errorf("%s = %d in the webapp, %d in Go", name, got, want)
		}
	}
}
