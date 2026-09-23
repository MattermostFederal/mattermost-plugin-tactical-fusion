package main

import (
	"regexp"
	"strconv"
	"testing"

	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/decorators/airport"
)

func TestWebappAirfieldsPostTypeMatches(t *testing.T) {
	source := readWebappFile(t, "decorators", "airport", "route.ts")

	for name, want := range map[string]string{
		"AIRFIELDS_POST_TYPE":     airport.PostType,
		"AIRFIELDS_PROPS_KEY":     airport.PropsKey,
		"AIRFIELDS_PROPS_VERSION": strconv.Itoa(airport.PropsVersion),
		"MAX_ROUTE_AIRFIELDS":     strconv.Itoa(airport.MaxRouteAirfields),
	} {
		pattern := regexp.MustCompile(`export const ` + name + ` = '?([^';]+)'?;`)
		m := pattern.FindStringSubmatch(source)
		if m == nil {
			t.Fatalf("no `export const %s` in the webapp's airport/route.ts; if it was renamed, "+
				"point this test at the new name rather than deleting it", name)
		}
		if m[1] != want {
			t.Errorf("%s = %q in the webapp, %q in Go", name, m[1], want)
		}
	}
}

func TestWebappAirfieldsShapeMatches(t *testing.T) {
	source := readWebappFile(t, "decorators", "airport", "route.ts")

	webappKeys := map[string]bool{}
	for _, m := range regexp.MustCompile(`readString\(\w+, '([a-z_]+)'\)`).FindAllStringSubmatch(source, -1) {
		webappKeys[m[1]] = true
	}
	for _, m := range regexp.MustCompile(`raw(?:Blob|Entry)\.([a-z_]+)`).FindAllStringSubmatch(source, -1) {
		webappKeys[m[1]] = true
	}
	if len(webappKeys) == 0 {
		t.Fatal("no reads found in the webapp's airport/route.ts")
	}

	d := &airport.Decorator{}
	tagger := newTestPlugin(t, "https://example.com", true).bridgeTagger()
	_, result := tagger.DecorateWithResult("DEPLOC:PHIK IATA:HNL", hookRef)
	blob, ok := d.MultiPostProps(result.Tokens)
	if !ok {
		t.Fatal("the fixture route was refused")
	}

	goKeys := map[string]bool{}
	collectKeys(blob, goKeys)

	for key := range goKeys {
		if !webappKeys[key] {
			t.Errorf("Go writes %q and the webapp never reads it", key)
		}
	}
	for key := range webappKeys {
		if !goKeys[key] {
			t.Errorf("the webapp reads %q but Go never writes it", key)
		}
	}
}
