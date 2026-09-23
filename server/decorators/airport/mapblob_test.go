package airport

import (
	"encoding/json"
	"testing"
)

func TestMapBlobCarriesThePositionAndTheDrawableRunways(t *testing.T) {
	blob, ok := MapBlob("PHNL")
	if !ok {
		t.Fatal("PHNL has no map blob")
	}

	var got mapBlob
	if err := json.Unmarshal([]byte(blob), &got); err != nil {
		t.Fatalf("not JSON: %v", err)
	}
	if got.Ident != "PHNL" || got.Name == "" {
		t.Errorf("blob names %q %q", got.Ident, got.Name)
	}
	if got.Coordinate.Format != "dd" || got.Coordinate.Value != "21.3184,-157.9257" {
		t.Errorf("coordinate = %+v", got.Coordinate)
	}
	if len(got.Runways) != 6 {
		t.Errorf("%d runways drawn, want 6", len(got.Runways))
	}
	for _, r := range got.Runways {
		if r.Designation == "" || r.Ends[0].Value == "" || r.Ends[1].Value == "" {
			t.Errorf("runway %+v is not drawable", r)
		}
	}
}

func TestMapBlobOmitsRunwaysWithoutEnds(t *testing.T) {
	blob, ok := MapBlob("AGAF")
	if !ok {
		t.Fatal("AGAF has no map blob")
	}
	var got mapBlob
	if err := json.Unmarshal([]byte(blob), &got); err != nil {
		t.Fatal(err)
	}
	if got.Runways == nil || len(got.Runways) != 0 {
		t.Errorf("runways = %v, want an empty list", got.Runways)
	}
}

func TestMapBlobRefusesAnUnknownIdent(t *testing.T) {
	if _, ok := MapBlob("QZQZ"); ok {
		t.Fatal("an unknown ident produced a blob")
	}
}

func TestEveryAirfieldHasAMapBlob(t *testing.T) {
	for ident := range airfields {
		if _, ok := MapBlob(ident); !ok {
			t.Fatalf("%s has no map blob", ident)
		}
	}
}
