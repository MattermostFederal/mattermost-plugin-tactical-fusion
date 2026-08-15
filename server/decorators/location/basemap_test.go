package location

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"os"
	"testing"
)

const basemapPath = "../../../public/map/world.geo.json"

func TestBasemapFitsItsBudget(t *testing.T) {
	const (
		rawBudget  = 400 * 1024
		gzipBudget = 120 * 1024
	)

	raw, err := os.ReadFile(basemapPath)
	if err != nil {
		t.Fatalf("the browser basemap is missing: %v. Run 'make map-data'", err)
	}

	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	if _, err := zw.Write(raw); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}

	t.Logf("basemap = %d bytes raw, %d gzipped", len(raw), buf.Len())

	if len(raw) > rawBudget {
		t.Errorf("basemap is %d bytes raw, over the %d budget", len(raw), rawBudget)
	}
	if buf.Len() > gzipBudget {
		t.Errorf("basemap is %d bytes gzipped, over the %d budget. Every reader who "+
			"opens a coordinate fetches this", buf.Len(), gzipBudget)
	}
}

func TestBasemapIsWellFormedAndCarriesEveryLayer(t *testing.T) {
	raw, err := os.ReadFile(basemapPath)
	if err != nil {
		t.Fatalf("the browser basemap is missing: %v. Run 'make map-data'", err)
	}

	var fc struct {
		Type     string `json:"type"`
		Features []struct {
			Properties struct {
				Layer string `json:"layer"`
			} `json:"properties"`
			Geometry struct {
				Type string `json:"type"`
			} `json:"geometry"`
		} `json:"features"`
	}
	if err := json.Unmarshal(raw, &fc); err != nil {
		t.Fatalf("basemap is not valid JSON: %v", err)
	}
	if fc.Type != "FeatureCollection" {
		t.Fatalf("basemap type = %q, want FeatureCollection", fc.Type)
	}

	seen := map[string]int{}
	for _, f := range fc.Features {
		seen[f.Properties.Layer]++
	}
	for _, layer := range []string{"land", "lakes", "borders"} {
		if seen[layer] == 0 {
			t.Errorf("basemap carries no %q features. A layer dropped from the "+
				"generator is invisible until somebody opens a map", layer)
		}
	}
	t.Logf("layers: %v", seen)
}
