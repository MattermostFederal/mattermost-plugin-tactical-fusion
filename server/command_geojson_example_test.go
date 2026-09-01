package main

import (
	"strings"
	"testing"

	"github.com/mattermost/mattermost/server/public/model"

	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/geojson"
)

// The example has to arrive as a card, not as raw JSON.
//
// CreatePost from a plugin does not run MessageWillBePosted, so an example that
// relied on the hook would post the document as a wall of coordinates, which is
// exactly what the feature exists to stop.
func TestTheGeoJSONExampleIsPostedAsACard(t *testing.T) {
	p := newTestPlugin(t, "https://example.com", true)
	api := p.API.(*fakeAPI)

	runExamplePosts(t, p)

	var card *model.Post
	for _, post := range api.created {
		if post.Type == geojson.PostType {
			card = post
		}
	}

	if card == nil {
		t.Fatal("the GeoJSON example was not stamped")
	}

	blob, ok := card.GetProps()[geojson.PropsKey].(map[string]any)
	if !ok {
		t.Fatalf("the example carries no %s blob", geojson.PropsKey)
	}

	counts, ok := blob["counts"].(map[string]any)
	if !ok {
		t.Fatal("the example's blob carries no counts")
	}
	if counts["features"] != 3 {
		t.Errorf("features = %v, want 3", counts["features"])
	}
}

// The example is measured against the same floor every other message is, so a
// server with the smaller limit gets the whole run or none of it.
func TestTheGeoJSONExampleFitsTheSafePostSize(t *testing.T) {
	message := geoJSONExampleLead + "\n\n```geojson\n" + geoJSONExample + "\n```\n"

	if runes := len([]rune(message)); runes > safePostRunes {
		t.Fatalf("the GeoJSON example is %d runes, over the %d floor", runes, safePostRunes)
	}
}

// Only the geojson spelling. An example that used a json fence would teach the
// opposite of what recognition does.
func TestTheGeoJSONExampleUsesTheGeoJSONFence(t *testing.T) {
	// On the MESSAGE, not on the lead with the needle glued to it. This once
	// read strings.Contains(lead+"```geojson", "```geojson"), which cannot
	// fail: the needle was concatenated onto the haystack inside the assertion,
	// and it never touched the function that writes the fence.
	message := geoJSONExampleMessage()

	if !strings.Contains(message, "```geojson\n") {
		t.Fatalf("the example does not use a geojson fence:\n%s", message)
	}

	// And not the ambiguous spelling. A json fence is refused out of the box,
	// so an example written that way would teach the one spelling that does not
	// work while every other assertion here stayed green.
	if strings.Contains(message, "```json\n") {
		t.Errorf("the example uses a json fence, which this build does not read:\n%s", message)
	}
}

/*
 * The area is a BLOB and the route bends, and both survive the parse whole.
 *
 * A three point line and an axis-aligned rectangle are what a hand-typed
 * document looks like, not what a GIS export produces, and the rectangle proved
 * nothing about the rendering either: it is the one polygon that still looks
 * right if the renderer swapped its axis order or kept only the corners of its
 * bounding box.
 *
 * The ring is asserted closed because RFC 7946 requires it and a reader that
 * silently closed it for us would hide an example that did not.
 */
func TestTheExampleDrawsIrregularShapes(t *testing.T) {
	document, err := geojson.Parse([]byte(geoJSONExample))
	if err != nil {
		t.Fatalf("the example does not parse: %v", err)
	}

	var ring geojson.Ring
	var route geojson.Ring
	for _, feature := range document.Features {
		for _, part := range feature.Geometry.Parts {
			switch part.Kind {
			case geojson.KindPolygon:
				ring = part.Rings[0]
			case geojson.KindLineString:
				route = part.Rings[0]
			}
		}
	}

	if len(ring) < 16 {
		t.Errorf("the area has %d positions; a blob needs enough of them to read as one", len(ring))
	}
	if len(route) < 6 {
		t.Errorf("the route has %d positions, so it is still a straight line", len(route))
	}

	if len(ring) > 0 && ring[0] != ring[len(ring)-1] {
		t.Error("the area's ring does not close, which RFC 7946 requires")
	}

	// Not a rectangle: a rectangle's positions take only two distinct values on
	// each axis, which is exactly the shape whose rendering proves nothing.
	lats, lons := map[string]bool{}, map[string]bool{}
	for _, position := range ring {
		lats[position.Lat] = true
		lons[position.Lon] = true
	}
	if len(lats) < 8 || len(lons) < 8 {
		t.Errorf("the area spans %d latitudes and %d longitudes, which is a box, not a blob",
			len(lats), len(lons))
	}
}

// The example is styled deliberately, and the styling is the feature it
// demonstrates. Reading the parse rather than the source text, so a key renamed
// to one the build ignores fails here rather than shipping an example that
// quietly demonstrates the theme's colors.
func TestTheExampleCarriesTheStyleItDemonstrates(t *testing.T) {
	document, err := geojson.Parse([]byte(geoJSONExample))
	if err != nil {
		t.Fatalf("the example does not parse: %v", err)
	}

	var area, point geojson.Style
	for _, feature := range document.Features {
		switch feature.Geometry.Kind {
		case geojson.KindPolygon:
			area = feature.Style
		case geojson.KindPoint:
			point = feature.Style
		}
	}

	if area.Color != "#ff0000" || area.FillOpacity == "" || area.Width == "" || area.StrokeOpacity == "" {
		t.Errorf("the operating area is not the styled red blob the lead describes: %+v", area)
	}
	if point.Color == "" || point.MarkerSize == "" {
		t.Errorf("the supply point states no marker style: %+v", point)
	}
}

// A fence-sourced post carries NO file_id. overlayForPost keys the map page's
// file gate on the key's presence, so a props writer that ever emitted it
// unconditionally would 404 every fenced post's map page, permanently.
func TestAFenceSourcedBlobNamesNoFile(t *testing.T) {
	document, err := geojson.Parse([]byte(geoJSONExample))
	if err != nil {
		t.Fatalf("the example does not parse: %v", err)
	}

	blob := geojson.Props(document, geojson.Source{Kind: geojson.SourceFence, Text: geoJSONExample})
	if _, named := blob["file_id"]; named {
		t.Error("a fence-sourced blob names a file, which would 404 its map page")
	}
}
