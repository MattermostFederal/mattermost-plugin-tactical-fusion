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

var webappFieldPattern = regexp.MustCompile(`(?m)^\s+(\w+)(\??):\s*([^;]+);`)

func TestWebappAirportShapeMatches(t *testing.T) {
	assertWebappInterfaceMatches(t, reflect.TypeFor[airportResponse](), map[reflect.Type]bool{})
}

func assertWebappInterfaceMatches(t *testing.T, goType reflect.Type, seen map[reflect.Type]bool) {
	t.Helper()

	if seen[goType] {
		return
	}
	seen[goType] = true

	name := webappInterfaceName(goType)
	block := webappInterface(t, name)

	var webapp []string
	for _, m := range webappFieldPattern.FindAllStringSubmatch(block, -1) {
		webapp = append(webapp, m[1]+m[2]+" "+strings.Join(strings.Fields(m[3]), " "))
	}

	var server []string
	for field := range goType.Fields() {
		tag, ok := field.Tag.Lookup("json")
		if !ok {
			t.Fatalf("%s.%s has no json tag, so the webapp cannot read it", goType.Name(), field.Name)
		}
		key := strings.Split(tag, ",")[0]
		optional := ""
		if strings.Contains(tag, ",omitempty") {
			optional = "?"
		}
		tsType, nested := webappTypeOf(t, goType.Name()+"."+field.Name, field.Type)
		server = append(server, key+optional+" "+tsType)
		for _, n := range nested {
			assertWebappInterfaceMatches(t, n, seen)
		}
	}

	if !slices.Equal(server, webapp) {
		t.Errorf("%s is %v here and %v in the webapp.\n"+
			"They must agree field for field, type for type, optionality for optionality, and in order.",
			name, server, webapp)
	}
}

func webappInterfaceName(goType reflect.Type) string {
	name := goType.Name()
	return strings.ToUpper(name[:1]) + name[1:]
}

func webappTypeOf(t *testing.T, where string, goType reflect.Type) (string, []reflect.Type) {
	t.Helper()

	switch goType.Kind() {
	case reflect.String:
		return "string", nil
	case reflect.Bool:
		return "boolean", nil
	case reflect.Struct:
		return webappInterfaceName(goType), []reflect.Type{goType}
	case reflect.Pointer:
		return webappTypeOf(t, where, goType.Elem())
	case reflect.Slice:
		inner, nested := webappTypeOf(t, where, goType.Elem())
		return inner + "[]", nested
	case reflect.Array:
		inner, nested := webappTypeOf(t, where, goType.Elem())
		parts := make([]string, goType.Len())
		for i := range parts {
			parts[i] = inner
		}
		return "[" + strings.Join(parts, ", ") + "]", nested
	}

	t.Fatalf("%s is a %s, which the webapp has no reading for", where, goType.Kind())
	return "", nil
}

func TestTheDerivedHalvesOfTheAirportResponseCanBeAbsent(t *testing.T) {
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

	ends, _ := reflect.TypeFor[airportRunway]().FieldByName("Ends")
	if ends.Type.Kind() != reflect.Pointer || !strings.Contains(ends.Tag.Get("json"), ",omitempty") {
		t.Error("airportRunway.Ends must be absent rather than zeroed when an end is unknown")
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

func TestWebappAirportIdentShapeMatches(t *testing.T) {
	raw := readWebappFile(t, "decorators", "airport", "airport.ts")

	for name, want := range map[string]string{"IDENT": airport.IdentBodyExpr(), "IATA": airport.IATABodyExpr()} {
		found := regexp.MustCompile(`export const ` + name + ` = /\^(.+)\$/;`).FindStringSubmatch(raw)
		if found == nil {
			t.Fatalf("no `export const %s = /^...$/;` in the webapp's airport/airport.ts; if it "+
				"was renamed, point this test at the new name rather than deleting it", name)
		}
		if found[1] != want {
			t.Errorf("the %s shape is %q here and %q in the webapp", name, want, found[1])
		}
	}
}

func TestWebappAirportMapKindMatches(t *testing.T) {
	raw := readWebappFile(t, "decorators", "airport", "map.ts")

	if want := "AIRPORT_MAP_KIND = '" + airport.MapKind + "'"; !strings.Contains(raw, want) {
		t.Errorf("the webapp does not declare %s", want)
	}
}
