package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/mattermost/mattermost/server/public/plugin"

	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/decorators/airport"
	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/decorators/location"
	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/errcode"
)

func airportMapRequest(t *testing.T, p *Plugin, query string) *httptest.ResponseRecorder {
	t.Helper()

	req := withSession(httptest.NewRequest(http.MethodGet, "/map?"+query, nil))
	rec := httptest.NewRecorder()
	p.ServeHTTP(&plugin.Context{}, rec, req)
	return rec
}

func TestTheAirfieldMapPageServesAShellTheBundleCanRead(t *testing.T) {
	p := newTestPlugin(t, "https://example.com", true)

	rec := airportMapRequest(t, p, "airport=PHNL")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (%s)", rec.Code, rec.Body.String())
	}

	body := rec.Body.String()
	for _, want := range []string{
		`data-mode="overlay"`,
		location.OverlayKindAttr + `="` + airport.MapKind + `"`,
		location.OverlayAttr + `="`,
		"PHNL",
		"08L/26R",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("the shell does not carry %s", want)
		}
	}
	if got := rec.Header().Get("Cache-Control"); got != "private, max-age=300" {
		t.Errorf("Cache-Control = %q", got)
	}
}

func TestTheAirfieldMapPageRefusesWithOneCode(t *testing.T) {
	p := newTestPlugin(t, "https://example.com", true)

	for _, query := range []string{"airport=QZQZ", "airport=phnl", "airport=PHN", "airport=%3Cscript%3E"} {
		t.Run(query, func(t *testing.T) {
			rec := airportMapRequest(t, p, query)
			if rec.Code != http.StatusNotFound {
				t.Fatalf("status = %d, want 404", rec.Code)
			}
			assertCode(t, rec.Body.String(), errcode.HTTPMapAirportUnavailable)
			if strings.Contains(rec.Body.String(), "<script>") {
				t.Error("the ident reached the page as markup")
			}
		})
	}
}

func TestTheAirfieldMapPageIsGoneWhenTheMapPageIsOff(t *testing.T) {
	p := newTestPlugin(t, "https://example.com", true)
	config := p.getConfiguration().Clone()
	config.EnableLocationMapPage = false
	p.setConfiguration(config)

	rec := airportMapRequest(t, p, "airport=PHNL")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
	assertCode(t, rec.Body.String(), errcode.HTTPMapDisabled)
}

func TestAPostIdWinsOverAnAirfield(t *testing.T) {
	p, _ := overlayPlugin(t, geoJSONPost(t))

	rec := airportMapRequest(t, p, "airport=PHNL&post="+overlayPost)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), location.OverlayKindAttr+`="custom_tf_geojson"`) {
		t.Error("the post did not win over the airfield")
	}
}

func TestAnAirfieldWinsOverACoordinate(t *testing.T) {
	p := newTestPlugin(t, "https://example.com", true)

	rec := airportMapRequest(t, p, "f=dd&v=21.3184,-157.9257&airport=PHNL")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), location.OverlayKindAttr+`="`+airport.MapKind+`"`) {
		t.Error("the coordinate won over the airfield")
	}
}

func TestTheAirfieldMapPageRequiresASession(t *testing.T) {
	p := newTestPlugin(t, "https://example.com", true)

	req := httptest.NewRequest(http.MethodGet, "/map?airport=PHNL", nil)
	rec := httptest.NewRecorder()
	p.ServeHTTP(&plugin.Context{}, rec, req)

	if rec.Code != http.StatusFound {
		t.Fatalf("status = %d, want a redirect to the login", rec.Code)
	}
}
