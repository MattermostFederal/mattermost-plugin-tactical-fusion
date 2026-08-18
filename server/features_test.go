package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"slices"
	"strings"
	"testing"
)

// The features payload has to mean the same thing on both sides of the wire.
//
// The same seam the Conversion shape is held across, and a worse one to get
// wrong. Every field here is read as a boolean, so a name that drifts reads
// `undefined`, which is falsy, which the webapp takes as "the admin turned this
// off". The symptom is a map that silently stops being drawn on an install that
// never touched the switch, with nothing logged on either side.
//
// Types as well as names, because `mapPanel: string` against a Go bool
// type-checks in TypeScript and is then truthy for "false".
func TestWebappFeatureShapeMatches(t *testing.T) {
	path := filepath.Join("..", "webapp", "src", "features", "types.ts")
	raw, err := os.ReadFile(path) // #nosec G304 -- fixed, repo-relative source path
	if err != nil {
		t.Fatalf("could not read %s: %v", path, err)
	}

	block := regexp.MustCompile(`(?s)export interface Features \{(.*?)\n\}`).FindStringSubmatch(string(raw))
	if block == nil {
		t.Fatal("no `export interface Features` in the webapp's features/types.ts; if it " +
			"was renamed, point this test at the new name rather than deleting it")
	}

	// Name AND type per field, in order. Names matter here in a way they did not
	// for Conversion: fromWire reads each key by its literal spelling, so a
	// drifted name reads `undefined`, which is falsy, which the webapp takes as
	// "the admin turned this surface off".
	var webapp []string
	for _, m := range regexp.MustCompile(`(?m)^\s+(\w+):\s*(\w+);`).FindAllStringSubmatch(block[1], -1) {
		webapp = append(webapp, camelToWire(m[1])+" "+m[2])
	}

	var server []string
	for field := range reflect.TypeFor[featuresResponse]().Fields() {
		tag, ok := field.Tag.Lookup("json")
		if !ok {
			t.Fatalf("featuresResponse.%s has no json tag, so the webapp cannot read it", field.Name)
		}
		if field.Type.Kind() != reflect.Bool {
			t.Fatalf("featuresResponse.%s is a %s; every field here is a switch and the "+
				"webapp reads them all as booleans", field.Name, field.Type.Kind())
		}
		server = append(server, strings.Split(tag, ",")[0]+" boolean")
	}

	// The wire spelling is snake_case and the TypeScript one camelCase, so the
	// webapp names are converted rather than compared raw. That conversion is
	// what makes this a real check: comparing counts and types alone compared
	// [boolean boolean boolean] against itself and could only ever fail if a
	// TypeScript field were annotated a non-boolean. A Go json tag could be
	// renamed on its own and every test in this repository stayed green.
	if !slices.Equal(server, webapp) {
		t.Errorf("the features payload is %v here and %v in the webapp.\n"+
			"They must agree field for field, type for type, and in order. A rename on "+
			"either side alone is silent: fromWire reads undefined, the store degrades to "+
			"every surface on, and maps draw on an install that switched them off.",
			server, webapp)
	}

	// And the wire keys as fromWire actually spells them, since the interface
	// above carries the camelCase half and only the reader carries the other.
	for _, field := range server {
		want := strings.Fields(field)[0]
		if !strings.Contains(string(raw), "'"+want+"'") {
			t.Errorf("the webapp never reads %q out of the payload; a field it does not "+
				"read is a surface that silently stops drawing", want)
		}
	}
}

// camelToWire turns the webapp's mapPanel into the wire's map_panel.
//
// Written here rather than taken from a library because it exists only to let
// the two field lists be compared as one vocabulary, and because a rule this
// small is easier to read than a dependency.
func camelToWire(name string) string {
	var out strings.Builder
	for _, r := range name {
		if r >= 'A' && r <= 'Z' {
			out.WriteByte('_')
			out.WriteRune(r - 'A' + 'a')
			continue
		}
		out.WriteRune(r)
	}

	return out.String()
}

// The route answers what locationMaps decided, so the parent AND lives in one
// place rather than being reimplemented in TypeScript.
func TestFeaturesRouteAnswersTheConfiguredSurfaces(t *testing.T) {
	for _, tc := range []struct {
		name   string
		config configuration
		want   featuresResponse
	}{
		{
			name: "everything on",
			config: configuration{
				EnableLocation: true, EnableLocationMap: true,
				EnableLocationMapPanel: true, EnableLocationMapInline: true, EnableLocationMapPage: true,
			},
			want: featuresResponse{MapPanel: true, MapInline: true, MapPage: true},
		},
		{
			name: "one surface off",
			config: configuration{
				EnableLocation: true, EnableLocationMap: true,
				EnableLocationMapPanel: true, EnableLocationMapInline: false, EnableLocationMapPage: true,
			},
			want: featuresResponse{MapPanel: true, MapInline: false, MapPage: true},
		},
		{
			// Asymmetric on purpose. Every other case here has Panel and Page
			// equal, so a transposition in locationMaps passed the whole table.
			name: "the panel on and the page off",
			config: configuration{
				EnableLocation: true, EnableLocationMap: true,
				EnableLocationMapPanel: true, EnableLocationMapInline: false, EnableLocationMapPage: false,
			},
			want: featuresResponse{MapPanel: true},
		},
		{
			// And the mirror, so neither field can be answering for the other.
			name: "the page on and the panel off",
			config: configuration{
				EnableLocation: true, EnableLocationMap: true,
				EnableLocationMapPanel: false, EnableLocationMapInline: false, EnableLocationMapPage: true,
			},
			want: featuresResponse{MapPage: true},
		},
		{
			// The map parent, which has to take every surface with it however
			// the three below it are set.
			name: "the map parent off",
			config: configuration{
				EnableLocation: true, EnableLocationMap: false,
				EnableLocationMapPanel: true, EnableLocationMapInline: true, EnableLocationMapPage: true,
			},
			want: featuresResponse{},
		},
		{
			// And the coordinate parent, because a map only ever appears behind
			// a coordinate this plugin decorated.
			name: "coordinates off entirely",
			config: configuration{
				EnableLocation: false, EnableLocationMap: true,
				EnableLocationMapPanel: true, EnableLocationMapInline: true, EnableLocationMapPage: true,
			},
			want: featuresResponse{},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := &Plugin{}
			p.setConfiguration(&tc.config)

			rec := httptest.NewRecorder()
			r := httptest.NewRequest(http.MethodGet, apiPath+"/features", nil)
			r.Header.Set("Mattermost-User-Id", "reader")

			p.serveAPI(rec, r)

			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want %d: %s", rec.Code, http.StatusOK, rec.Body.String())
			}

			var got featuresResponse
			if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
				t.Fatalf("body is not the features payload: %v", err)
			}
			if got != tc.want {
				t.Fatalf("features = %+v, want %+v", got, tc.want)
			}
		})
	}
}

// A plugin whose configuration has never loaded says so, rather than answering
// "every surface off" and having the client cache that for half an hour.
//
// Reachable beyond a startup race: OnConfigurationChange returns before
// setConfiguration when LoadPluginConfiguration fails, so the configuration
// stays nil until a later change succeeds.
func TestFeaturesRouteRefusesBeforeTheConfigurationLoads(t *testing.T) {
	p := &Plugin{}

	rec := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, apiPath+"/features", nil)
	r.Header.Set("Mattermost-User-Id", "reader")

	p.serveAPI(rec, r)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d: an unloaded configuration must not be reported "+
			"as an admin decision", rec.Code, http.StatusServiceUnavailable)
	}
}

// An admin can change this at any moment, so nothing between here and the
// browser may keep a copy. The webapp's own cache is bounded and refreshes; a
// shared HTTP cache would leave a reader on an answer no reload could correct.
func TestFeaturesRouteIsNotCached(t *testing.T) {
	p := &Plugin{}
	p.setConfiguration(&configuration{EnableLocation: true, EnableLocationMap: true})

	rec := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, apiPath+"/features", nil)
	r.Header.Set("Mattermost-User-Id", "reader")

	p.serveAPI(rec, r)

	if got := rec.Header().Get("Cache-Control"); got != "no-store" {
		t.Fatalf("Cache-Control = %q, want %q", got, "no-store")
	}
}

// The same session gate as everything else under /api/v1. A caller with no
// session is not a reader of this workspace, whatever they are asking about.
func TestFeaturesRouteRequiresASession(t *testing.T) {
	p := &Plugin{}
	p.setConfiguration(&configuration{EnableLocation: true, EnableLocationMap: true})

	rec := httptest.NewRecorder()
	p.serveAPI(rec, httptest.NewRequest(http.MethodGet, apiPath+"/features", nil))

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestFeaturesRouteRefusesAnythingButGet(t *testing.T) {
	p := &Plugin{}
	p.setConfiguration(&configuration{EnableLocation: true, EnableLocationMap: true})

	rec := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPut, apiPath+"/features", nil)
	r.Header.Set("Mattermost-User-Id", "reader")

	p.serveAPI(rec, r)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusMethodNotAllowed)
	}
}
