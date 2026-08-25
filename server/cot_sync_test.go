package main

import (
	"os"
	"path/filepath"
	"regexp"
	"slices"
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
	for _, m := range regexp.MustCompile(`text\((?:blob|event), '([a-z_]+)'\)`).FindAllStringSubmatch(source, -1) {
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

	for _, key := range goKeys {
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
var cotOptionalKeys = map[string]bool{
	"position_note": true,
	"start":         true,
	"src":           true,
	"parent":        true,
	"related":       true,

	// Version 1 wrote one "event" where version 2 writes an "events" array. The
	// webapp still reads it, for posts stamped before the bump.
	"event": true,
}

const fullCotEvent = `<event version="2.0" uid="ANDROID-1" type="a-f-G-U-C" how="m-g" ` +
	`time="2026-08-23T11:43:38Z" start="2026-08-23T11:43:38Z" stale="2026-08-23T11:45:38Z">` +
	`<point lat="34.056100" lon="-118.250000" hae="-42.6" ce="45.3" le="99.5"/>` +
	`<detail><contact callsign="DELTA1"/><__group name="Cyan" role="Team Member"/>` +
	`<track speed="3.2" course="180.0"/><remarks>holding</remarks></detail></event>`

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
// The words are the map label's only channel: colour is what distinguishes one
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
