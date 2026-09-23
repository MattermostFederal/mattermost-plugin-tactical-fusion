package main

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/errcode"
)

func decodeAirport(t *testing.T, body []byte) airportResponse {
	t.Helper()

	var got airportResponse
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("not an airport response: %v (%s)", err, body)
	}
	return got
}

func TestAirportResolvesAnIATACode(t *testing.T) {
	p, _ := newAPIPlugin(t)

	rec := call(p, http.MethodGet, airportPath+"?i=HNL", testUserID, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (%s)", rec.Code, rec.Body.String())
	}

	got := decodeAirport(t, rec.Body.Bytes())
	if !got.Found || got.Ident != "PHNL" || got.IATA != "HNL" {
		t.Fatalf("found = %v, ident = %q, iata = %q", got.Found, got.Ident, got.IATA)
	}
	if got.Airport == nil || got.Airport.IATA != "HNL" {
		t.Fatal("the airfield does not carry its IATA code")
	}
}

func TestAirportAnswersForAnIATACodeThisBuildDoesNotHold(t *testing.T) {
	p, _ := newAPIPlugin(t)

	rec := call(p, http.MethodGet, airportPath+"?i=QQQ", testUserID, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}

	got := decodeAirport(t, rec.Body.Bytes())
	if got.Found || got.Ident != "" || got.IATA != "QQQ" {
		t.Fatalf("found = %v, ident = %q, iata = %q; want an honest miss naming the code", got.Found, got.Ident, got.IATA)
	}
	if got.Airport != nil || got.Coordinate != nil {
		t.Error("an unknown code carried an airfield or a coordinate")
	}
}

func TestAirportRefusesBothOrNeitherCode(t *testing.T) {
	p, _ := newAPIPlugin(t)

	for _, query := range []string{"?v=PHNL&i=HNL", "", "?x=PHNL"} {
		t.Run(query, func(t *testing.T) {
			rec := call(p, http.MethodGet, airportPath+query, testUserID, "")
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400", rec.Code)
			}
			assertCode(t, rec.Body.String(), errcode.APIAirportParamsConflict)
		})
	}
}

func TestAirportRefusesAMalformedIATACode(t *testing.T) {
	p, _ := newAPIPlugin(t)

	for _, code := range []string{"hnl", "HN", "HNLL", "H1L"} {
		rec := call(p, http.MethodGet, airportPath+"?i="+code, testUserID, "")
		if rec.Code != http.StatusBadRequest {
			t.Errorf("%s: status = %d, want 400", code, rec.Code)
		}
		assertCode(t, rec.Body.String(), errcode.APIAirportInvalid)
	}
}

func TestAirportCarriesRunwaysFrequenciesAndTheDesignator(t *testing.T) {
	p, _ := newAPIPlugin(t)

	got := decodeAirport(t, call(p, http.MethodGet, airportURL("PHNL"), testUserID, "").Body.Bytes())
	if got.Airport == nil {
		t.Fatal("no airfield")
	}
	if len(got.Airport.Runways) != 6 {
		t.Errorf("%d runways, want 6", len(got.Airport.Runways))
	}
	var drawn int
	for _, r := range got.Airport.Runways {
		if r.Ends != nil {
			drawn++
			if r.Ends[0].Format != "dd" || r.Ends[1].Value == "" {
				t.Errorf("runway %s ends = %v", r.Designation, *r.Ends)
			}
		}
	}
	if drawn != 6 {
		t.Errorf("%d runways carry ends, want 6", drawn)
	}
	if len(got.Airport.Frequencies) == 0 || got.Airport.Frequencies[0].MHz == "" {
		t.Errorf("frequencies = %v", got.Airport.Frequencies)
	}

	hickam := decodeAirport(t, call(p, http.MethodGet, airportURL("PHIK"), testUserID, "").Body.Bytes())
	if hickam.Airport == nil || hickam.Airport.Military != "Air Force Base" {
		t.Errorf("PHIK military = %+v", hickam.Airport)
	}

	bare := decodeAirport(t, call(p, http.MethodGet, airportURL("AAXX"), testUserID, "").Body.Bytes())
	if bare.Airport == nil || bare.Airport.Runways == nil || bare.Airport.Frequencies == nil {
		t.Error("an airfield with nothing to list carries null rather than empty lists")
	}
	if len(bare.Airport.Runways) != 0 || len(bare.Airport.Frequencies) != 0 || bare.Airport.Military != "" {
		t.Errorf("AAXX carries %+v", bare.Airport)
	}
}

func TestAirportRefusesTheIATAParamAsAnIdent(t *testing.T) {
	p, _ := newAPIPlugin(t)

	rec := call(p, http.MethodGet, airportPath+"?i=PHNL", testUserID, "")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}
