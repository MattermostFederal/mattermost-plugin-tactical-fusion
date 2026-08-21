package main

import (
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/decorators/airport"
)

// The airfield payload is a worse seam to get wrong than the conversion one.
// Every field is read by its literal spelling, so a drifted name reads
// undefined and the panel renders a blank row where an airfield name should be,
// with nothing logged on either side.
func TestWebappAirportShapeMatches(t *testing.T) {
	block := webappInterface(t, "AirportDetails")

	var webapp []string
	for _, m := range regexp.MustCompile(`(?m)^\s+(\w+):\s*(\w+);`).FindAllStringSubmatch(block, -1) {
		webapp = append(webapp, m[1]+" "+m[2])
	}

	var server []string
	for field := range reflect.TypeFor[airportDetails]().Fields() {
		tag, ok := field.Tag.Lookup("json")
		if !ok {
			t.Fatalf("airportDetails.%s has no json tag, so the webapp cannot read it", field.Name)
		}
		if field.Type.Kind() != reflect.String {
			t.Fatalf("airportDetails.%s is a %s; every field here is already rendered and "+
				"the webapp reads them all as strings", field.Name, field.Type.Kind())
		}
		server = append(server, strings.Split(tag, ",")[0]+" string")
	}

	if !slices.Equal(server, webapp) {
		t.Errorf("the airfield payload is %v here and %v in the webapp.\n"+
			"They must agree field for field, type for type, and in order.",
			server, webapp)
	}
}

// The coordinate is what every airfield surface draws and links with, so a
// rename on either side alone would leave the map undrawn with nothing logged.
func TestWebappAirportCoordinateShapeMatches(t *testing.T) {
	block := webappInterface(t, "AirportCoordinate")

	var webapp []string
	for _, m := range regexp.MustCompile(`(?m)^\s+(\w+):\s*(\w+);`).FindAllStringSubmatch(block, -1) {
		webapp = append(webapp, m[1]+" "+m[2])
	}

	var server []string
	for field := range reflect.TypeFor[airportCoordinate]().Fields() {
		tag, ok := field.Tag.Lookup("json")
		if !ok {
			t.Fatalf("airportCoordinate.%s has no json tag", field.Name)
		}
		if field.Type.Kind() != reflect.String {
			t.Fatalf("airportCoordinate.%s is a %s, and the webapp reads them all as strings",
				field.Name, field.Type.Kind())
		}
		server = append(server, strings.Split(tag, ",")[0]+" string")
	}

	if !slices.Equal(server, webapp) {
		t.Errorf("the airfield coordinate is %v here and %v in the webapp", server, webapp)
	}
}

// The discriminated shape is what keeps an unknown ident from drawing at Null
// Island, so the discriminator and the two optional halves have to survive a
// rename on either side.
func TestWebappAirportResponseShapeMatches(t *testing.T) {
	block := webappInterface(t, "AirportResponse")

	var webapp []string
	for _, m := range regexp.MustCompile(`(?m)^\s+(\w+)\??:\s*(\w+);`).FindAllStringSubmatch(block, -1) {
		webapp = append(webapp, m[1])
	}

	var server []string
	for field := range reflect.TypeFor[airportResponse]().Fields() {
		tag, ok := field.Tag.Lookup("json")
		if !ok {
			t.Fatalf("airportResponse.%s has no json tag", field.Name)
		}
		server = append(server, strings.Split(tag, ",")[0])
	}

	if !slices.Equal(server, webapp) {
		t.Errorf("the airfield response is %v here and %v in the webapp", server, webapp)
	}

	// The two derived halves must be omitempty, or an ident this build does not
	// hold ships a zeroed coordinate and the webapp builds a link from it.
	for _, name := range []string{"Airport", "Coordinate"} {
		field, ok := reflect.TypeFor[airportResponse]().FieldByName(name)
		if !ok {
			t.Fatalf("airportResponse has no %s field", name)
		}
		if !strings.Contains(field.Tag.Get("json"), ",omitempty") {
			t.Errorf("airportResponse.%s is not omitempty, so an unknown ident would "+
				"carry a zero value the webapp reads as real", name)
		}
		if field.Type.Kind() != reflect.Pointer {
			t.Errorf("airportResponse.%s is a %s, not a pointer, so it cannot be absent",
				name, field.Type.Kind())
		}
	}
}

func webappInterface(t *testing.T, name string) string {
	t.Helper()

	path := filepath.Join("..", "webapp", "src", "decorators", "airport", "types.ts")
	raw, err := os.ReadFile(path) // #nosec G304 -- fixed, repo-relative source path
	if err != nil {
		t.Fatalf("could not read %s: %v", path, err)
	}

	block := regexp.MustCompile(`(?s)export interface ` + name + ` \{(.*?)\n\}`).
		FindStringSubmatch(string(raw))
	if block == nil {
		t.Fatalf("no `export interface %s` in the webapp's airport/types.ts; if it was "+
			"renamed, point this test at the new name rather than deleting it", name)
	}

	return block[1]
}

// The decorator type is the URL path segment, so the two sides cannot differ.
func TestWebappAirportTypeMatches(t *testing.T) {
	path := filepath.Join("..", "webapp", "src", "decorators", "airport", "index.ts")
	raw, err := os.ReadFile(path) // #nosec G304 -- fixed, repo-relative source path
	if err != nil {
		t.Fatalf("could not read %s: %v", path, err)
	}

	if want := "type: '" + airport.Type + "'"; !strings.Contains(string(raw), want) {
		t.Errorf("the webapp airport decorator does not declare %s", want)
	}
}

// The ident shape is written in both languages and nothing held the two
// together, which is the drift that is silent by construction: fromParams
// returns null, the click handler reads that as "not one of ours" and stands
// aside, the browser opens the standalone page, which renders correctly, and
// nothing is logged on either side. The only symptom is that clicking an
// airfield code opens a page instead of the sidebar.
func TestWebappAirportIdentShapeMatches(t *testing.T) {
	path := filepath.Join("..", "webapp", "src", "decorators", "airport", "airport.ts")
	raw, err := os.ReadFile(path) // #nosec G304 -- fixed, repo-relative source path
	if err != nil {
		t.Fatalf("could not read %s: %v", path, err)
	}

	found := regexp.MustCompile(`export const IDENT = /\^(.+)\$/;`).FindStringSubmatch(string(raw))
	if found == nil {
		t.Fatalf("no `export const IDENT = /^...$/;` in the webapp's airport/airport.ts; if it " +
			"was renamed, point this test at the new name rather than deleting it")
	}

	if found[1] != airport.IdentBodyExpr() {
		t.Errorf("the ident shape is %q here and %q in the webapp", airport.IdentBodyExpr(), found[1])
	}
}
