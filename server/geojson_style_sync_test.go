package main

import (
	"regexp"
	"slices"
	"testing"

	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/geojson"
)

func TestWebappGeoJSONStyleKeysMatch(t *testing.T) {
	source := readWebappFile(t, "geojson", "GeoJsonCard.tsx")

	block := regexp.MustCompile(`(?s)const STYLE_KEYS = new Set\(\[(.*?)\]\);`).FindStringSubmatch(source)
	if block == nil {
		t.Fatal("no STYLE_KEYS set in the webapp's geojson/GeoJsonCard.tsx; if it moved, point this test at it rather than deleting it")
	}

	var webapp []string
	for _, m := range regexp.MustCompile(`'([a-z-]+)'`).FindAllStringSubmatch(block[1], -1) {
		webapp = append(webapp, m[1])
	}

	want := geojson.StyleKeys()
	slices.Sort(webapp)
	slices.Sort(want)
	if !slices.Equal(webapp, want) {
		t.Errorf("the webapp hides %v as style keys; the server reads %v", webapp, want)
	}
}
