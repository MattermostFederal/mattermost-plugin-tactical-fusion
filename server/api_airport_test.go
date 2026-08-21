package main

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/errcode"
)

func airportURL(v string) string {
	return airportPath + "?v=" + v
}

func TestAirportRequiresASession(t *testing.T) {
	p, _ := newAPIPlugin(t)

	rec := call(p, http.MethodGet, airportURL("KIND"), "", "")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
	assertCode(t, rec.Body.String(), errcode.APINotAuthorized)
}

func TestAirportRefusesAnythingButGet(t *testing.T) {
	p, _ := newAPIPlugin(t)

	for _, method := range []string{http.MethodPost, http.MethodPut, http.MethodDelete} {
		rec := call(p, method, airportURL("KIND"), testUserID, "")
		if rec.Code != http.StatusMethodNotAllowed {
			t.Errorf("%s: status = %d, want 405", method, rec.Code)
		}
	}
}

func TestAirportReturnsTheAirfieldAndItsPosition(t *testing.T) {
	p, _ := newAPIPlugin(t)

	rec := call(p, http.MethodGet, airportURL("KIND"), testUserID, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (%s)", rec.Code, rec.Body.String())
	}

	var got airportResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("not an airport response: %v (%s)", err, rec.Body.String())
	}

	if !got.Found || got.Ident != "KIND" {
		t.Fatalf("found = %v, ident = %q", got.Found, got.Ident)
	}
	if got.Airport == nil {
		t.Fatal("no airfield in the body")
	}
	if got.Airport.Name != "Indianapolis International Airport" {
		t.Errorf("name = %q", got.Airport.Name)
	}
	if got.Airport.Place != "Indianapolis, IN, US" {
		t.Errorf("place = %q", got.Airport.Place)
	}
	if got.Airport.Elevation != "797 ft" {
		t.Errorf("elevation = %q", got.Airport.Elevation)
	}
	// The link identity, not the readings: an airfield surface names the field
	// and links to the coordinate rather than reprinting it.
	if got.Coordinate == nil {
		t.Fatal("no coordinate in the body")
	}
	if got.Coordinate.Format != "dd" {
		t.Errorf("format = %q, want dd", got.Coordinate.Format)
	}
	if got.Coordinate.Value != "39.7173,-86.2944" {
		t.Errorf("value = %q", got.Coordinate.Value)
	}

	// The map's accessible label, and the only place the country reaches a
	// reader who is not looking at the map.
	if got.Coordinate.Region != "United States of America (Natural Earth 110m)" {
		t.Errorf("region = %q", got.Coordinate.Region)
	}
}

// An ident this build does not hold is an answer, not a failure: a refreshed
// database must not turn every link naming a retired code into a 400.
func TestAirportAnswersForAnUnknownIdent(t *testing.T) {
	p, _ := newAPIPlugin(t)

	rec := call(p, http.MethodGet, airportURL("QZQZ"), testUserID, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}

	var got airportResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("not an airport response: %v", err)
	}
	if got.Found {
		t.Error("found = true for an ident this build does not hold")
	}
	if got.Ident != "QZQZ" {
		t.Errorf("ident = %q", got.Ident)
	}
	if got.Airport != nil || got.Coordinate != nil {
		t.Error("an unknown ident carries an airfield or a coordinate")
	}
}

// The whole reason the shape is discriminated: a flat record would put an
// empty token on the wire, and a link built from one opens a view that refuses
// it.
func TestAnUnknownIdentCarriesNoPositionAtAll(t *testing.T) {
	p, _ := newAPIPlugin(t)

	rec := call(p, http.MethodGet, airportURL("QZQZ"), testUserID, "")

	var raw map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &raw); err != nil {
		t.Fatalf("not JSON: %v", err)
	}
	for _, key := range []string{"coordinate", "position", "lat", "lon", "airport"} {
		if _, present := raw[key]; present {
			t.Errorf("the body carries %q for an unknown ident", key)
		}
	}
}

func TestAirportRefusesAMalformedIdent(t *testing.T) {
	p, _ := newAPIPlugin(t)

	for _, v := range []string{"", "KIN", "KINDX", "kind", "K1ND", "%2e%2e"} {
		rec := call(p, http.MethodGet, airportURL(v), testUserID, "")
		if rec.Code != http.StatusBadRequest {
			t.Errorf("v=%q: status = %d, want 400", v, rec.Code)
			continue
		}
		assertCode(t, rec.Body.String(), errcode.APIAirportInvalid)
	}
}

func TestAirportIsCachedPrivately(t *testing.T) {
	p, _ := newAPIPlugin(t)

	rec := call(p, http.MethodGet, airportURL("KIND"), testUserID, "")
	if got := rec.Header().Get("Cache-Control"); got != "private, max-age=300" {
		t.Fatalf("Cache-Control = %q", got)
	}
}

// A refusal must never carry a cache lifetime.
func TestARefusedAirportIsNotCached(t *testing.T) {
	p, _ := newAPIPlugin(t)

	rec := call(p, http.MethodGet, airportURL("KIN"), testUserID, "")
	if got := rec.Header().Get("Cache-Control"); got == "private, max-age=300" {
		t.Fatalf("a 400 was given a cache lifetime")
	}
}

// A format switch governs decoration only. A link written while the decorator
// was on has to keep resolving after an admin turns it off, exactly as a
// coordinate link does.
func TestAirportAnswersWhileTheDecoratorIsOff(t *testing.T) {
	p, _ := newAPIPlugin(t)

	config := *p.getConfiguration()
	config.EnableAirport = false
	p.setConfiguration(&config)

	rec := call(p, http.MethodGet, airportURL("KIND"), testUserID, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 with the decorator switched off", rec.Code)
	}
}
