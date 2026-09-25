package main

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/mattermost/mattermost/server/public/model"

	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/cot"
	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/errcode"
	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/geojson"
)

func at(lat, lon float64) (*float64, *float64) { return &lat, &lon }

func pointEvent(callsign string, lat, lon float64) CreateCotEvent {
	la, lo := at(lat, lon)
	return CreateCotEvent{Callsign: callsign, Lat: la, Lon: lo}
}

func TestMCPCreatesACotEventThatReadsBack(t *testing.T) {
	event := pointEvent(`TGT <01> & "B"`, 37.7749, -122.4194)
	event.Affiliation, event.Dimension, event.Remarks = "hostile", "ground", "two vehicles, stationary"
	got := decodeMCPResult[CreatedCot](t, callMCPTool(t, agentsSession(t, mcpPlugin(t)), "create_cot", CreateCotArgs{
		Events: []CreateCotEvent{event}, StaleMinutes: 30,
	}))

	events, err := cot.Parse([]byte(got.XML))
	if err != nil || len(events) != 1 {
		t.Fatalf("the XML does not read back: %v\n%s", err, got.XML)
	}
	parsed := events[0]
	if parsed.Type != "a-h-G" || parsed.Detail.Callsign != `TGT <01> & "B"` || parsed.Point.Lat != "37.774900" || parsed.Point.Lon != "-122.419400" {
		t.Errorf("the event is %+v", parsed)
	}
	start, _ := time.Parse(time.RFC3339, parsed.Start)
	stale, _ := time.Parse(time.RFC3339, parsed.Stale)
	if stale.Sub(start) != 30*time.Minute {
		t.Errorf("the event is current for %v", stale.Sub(start))
	}
	if len(got.Events) != 1 || got.Events[0]["affiliation"] != "hostile" || got.Events[0]["callsign"] != `TGT <01> & "B"` {
		t.Errorf("the event reads back as %v", got.Events)
	}
	if !strings.HasPrefix(got.Message, "```cot\n<event") || !strings.HasSuffix(got.Message, "</event>\n```") || !got.FitsPost {
		t.Errorf("the message is not a postable cot fence:\n%s", got.Message)
	}
}

func TestMCPCreatesARedCircleAGreenLineAndAFriendlyPointInOneMessage(t *testing.T) {
	p := mcpPlugin(t)
	circle := pointEvent("THREAT RING", 21.3187, -157.9225)
	circle.Color = "#ff0000"
	circle.Shape = &CreateCotShape{Kind: "circle", RadiusMeters: 40233.6}
	line := CreateCotEvent{Callsign: "PHIK TO PGUM", Color: "#00ff00", Shape: &CreateCotShape{
		Kind: "line", Positions: []GeoJSONPosition{{Lat: 21.3187, Lon: -157.9225}, {Lat: 13.4834, Lon: 144.796}},
	}}

	friendly := pointEvent("GUAM", 13.4834, 144.796)
	friendly.Affiliation = "friend"

	got := decodeMCPResult[CreatedCot](t, callMCPTool(t, agentsSession(t, p), "create_cot", CreateCotArgs{Events: []CreateCotEvent{circle, line, friendly}}))

	if len(got.Events) != 3 {
		t.Fatalf("the message carries %d events:\n%s", len(got.Events), got.XML)
	}
	ring, route, unit := got.Events[0], got.Events[1], got.Events[2]
	ringShape, _ := ring["geometry"].(map[string]any)
	routeShape, _ := route["geometry"].(map[string]any)
	if ring["cot_type"] != "u-d-c-c" || ringShape["kind"] != "ellipse" || ringShape["major_m"] != "40233.6" || ringShape["minor_m"] != "40233.6" {
		t.Errorf("the circle reads back as %v", ring)
	}
	if route["cot_type"] != "u-d-f" || routeShape["kind"] != "polyline" || routeShape["closed"] != nil || len(objects(routeShape["points"])) != 2 {
		t.Errorf("the line reads back as %v", route)
	}
	if route["lat"] != "21.318700" || route["lon"] != "-157.922500" {
		t.Errorf("a line with no position is not placed at its first vertex: %v", route)
	}
	if unit["cot_type"] != "a-f-G" || unit["affiliation"] != "friend" || unit["geometry"] != nil {
		t.Errorf("the friendly point reads back as %v", unit)
	}
	if !strings.Contains(got.XML, `<color argb="-65536"/>`) || !strings.Contains(got.XML, `<color argb="-16711936"/>`) {
		t.Errorf("the colors were not written as ATAK argb:\n%s", got.XML)
	}

	stamped := p.decoratePost(&model.Post{Message: got.Message, UserId: testUserID}, time.Now())
	if stamped == nil || stamped.Type != cot.PostType {
		t.Fatalf("the created message was not stamped as a card: %+v", stamped)
	}
}

func TestMCPCreatesAClosedPolygon(t *testing.T) {
	got := decodeMCPResult[CreatedCot](t, callMCPTool(t, agentsSession(t, mcpPlugin(t)), "create_cot", CreateCotArgs{Events: []CreateCotEvent{{
		Callsign: "OBJ AREA", Shape: &CreateCotShape{Kind: "polygon", Positions: []GeoJSONPosition{{Lat: 1, Lon: 1}, {Lat: 1, Lon: 2}, {Lat: 2, Lon: 2}}},
	}}}))

	geometry, _ := got.Events[0]["geometry"].(map[string]any)
	if geometry["kind"] != "polyline" || geometry["closed"] == nil {
		t.Errorf("the polygon reads back as %v", geometry)
	}
}

func TestMCPCreatedCotMessageIsStampedAsACard(t *testing.T) {
	p := mcpPlugin(t)
	event := pointEvent("ALPHA", 21.3353, -157.9483)
	event.Affiliation = "friend"
	got := decodeMCPResult[CreatedCot](t, callMCPTool(t, agentsSession(t, p), "create_cot", CreateCotArgs{Events: []CreateCotEvent{event}}))

	stamped := p.decoratePost(&model.Post{Message: got.Message, UserId: testUserID}, time.Now())
	if stamped == nil || stamped.Type != cot.PostType {
		t.Fatalf("the created message was not stamped as a card: %+v", stamped)
	}
}

func TestMCPCreateCotTakesAnExactType(t *testing.T) {
	event := pointEvent("ALPHA", 1, 2)
	event.Type = "a-f-G-U-C"
	got := decodeMCPResult[CreatedCot](t, callMCPTool(t, agentsSession(t, mcpPlugin(t)), "create_cot", CreateCotArgs{Events: []CreateCotEvent{event}}))
	if !strings.Contains(got.XML, `type="a-f-G-U-C"`) {
		t.Errorf("the exact type was not kept:\n%s", got.XML)
	}
}

func TestMCPCreateCotRefusesWhatItCannotBuild(t *testing.T) {
	with := func(change func(*CreateCotEvent)) []CreateCotEvent {
		event := pointEvent("A", 1, 2)
		change(&event)
		return []CreateCotEvent{event}
	}
	line := func(n int) *CreateCotShape {
		return &CreateCotShape{Kind: "line", Positions: make([]GeoJSONPosition, n)}
	}
	tooMany := make([]CreateCotEvent, cot.MaxEvents+1)
	for i := range tooMany {
		tooMany[i] = pointEvent("A", 1, 2)
	}
	tooManyVertices := []CreateCotEvent{
		{Callsign: "A", Shape: line(cot.MaxVertices)},
		{Callsign: "B", Shape: line(2)},
	}

	for name, in := range map[string]CreateCotArgs{
		"no events":             {},
		"too many events":       {Events: tooMany},
		"no callsign":           {Events: with(func(e *CreateCotEvent) { e.Callsign = "" })},
		"latitude past 90":      {Events: []CreateCotEvent{pointEvent("A", 91, 2)}},
		"longitude past 180":    {Events: []CreateCotEvent{pointEvent("A", 1, 181)}},
		"no position":           {Events: []CreateCotEvent{{Callsign: "A"}}},
		"unknown affiliation":   {Events: with(func(e *CreateCotEvent) { e.Affiliation = "ally" })},
		"malformed type":        {Events: with(func(e *CreateCotEvent) { e.Type = `a"><x` })},
		"stale past a week":     {Events: with(func(*CreateCotEvent) {}), StaleMinutes: maxCotStaleMinutes + 1},
		"long remarks":          {Events: with(func(e *CreateCotEvent) { e.Remarks = strings.Repeat("x", maxCreatedTextRunes+1) })},
		"malformed color":       {Events: with(func(e *CreateCotEvent) { e.Color = "red" })},
		"unknown shape":         {Events: with(func(e *CreateCotEvent) { e.Shape = &CreateCotShape{Kind: "star"} })},
		"circle with no radius": {Events: with(func(e *CreateCotEvent) { e.Shape = &CreateCotShape{Kind: "circle"} })},
		"circle with no center": {Events: []CreateCotEvent{{Callsign: "A", Shape: &CreateCotShape{Kind: "circle", RadiusMeters: 10}}}},
		"one point line":        {Events: with(func(e *CreateCotEvent) { e.Shape = line(1) })},
		"two point polygon": {Events: with(func(e *CreateCotEvent) {
			e.Shape = &CreateCotShape{Kind: "polygon", Positions: make([]GeoJSONPosition, 2)}
		})},
		"vertex off the globe": {Events: with(func(e *CreateCotEvent) {
			e.Shape = &CreateCotShape{Kind: "line", Positions: []GeoJSONPosition{{Lat: 1}, {Lat: 95}}}
		})},
		"too many vertices": {Events: tooManyVertices},
	} {
		t.Run(name, func(t *testing.T) {
			assertMCPRefusal(t, "create_cot", in, errcode.MCPCreateCotInvalid)
		})
	}
}

func TestMCPCreatesAGeoJSONDocumentThatReadsBack(t *testing.T) {
	got := decodeMCPResult[CreatedGeoJSON](t, callMCPTool(t, agentsSession(t, mcpPlugin(t)), "create_geojson", CreateGeoJSONArgs{
		Name: "Resupply", Description: "Exercise overlay",
		Features: []CreateGeoJSONFeature{
			{Name: "Supply point", Kind: "point", Positions: []GeoJSONPosition{{Lat: 21.3353, Lon: -157.9483}}, Color: "#d32f2f", Properties: map[string]string{"status": "open"}},
			{Name: "Route in", Kind: "line", Positions: []GeoJSONPosition{{Lat: 21.33, Lon: -157.95}, {Lat: 21.34, Lon: -157.94}}},
			{Name: "Area", Kind: "polygon", Positions: []GeoJSONPosition{{Lat: 21.33, Lon: -157.95}, {Lat: 21.34, Lon: -157.95}, {Lat: 21.34, Lon: -157.94}}},
		},
	}))

	document, err := geojson.Parse([]byte(got.Document))
	if err != nil {
		t.Fatalf("the document does not read back: %v\n%s", err, got.Document)
	}
	if document.Name != "Resupply" || len(document.Features) != 3 {
		t.Errorf("the document is %+v", document)
	}
	if got.Summary.Counts.Points != 1 || got.Summary.Counts.Lines != 1 || got.Summary.Counts.Polygons != 1 {
		t.Errorf("counts %+v", got.Summary.Counts)
	}
	var wire struct {
		Features []struct {
			Geometry struct {
				Coordinates json.RawMessage `json:"coordinates"`
			} `json:"geometry"`
		} `json:"features"`
	}
	var point []float64
	if err := json.Unmarshal([]byte(got.Document), &wire); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(wire.Features[0].Geometry.Coordinates, &point); err != nil || point[0] != -157.9483 || point[1] != 21.3353 {
		t.Errorf("the point is not written longitude first: %v %v", err, point)
	}
	if !strings.HasPrefix(got.Message, "```geojson\n{") || !got.FitsPost {
		t.Errorf("the message is not a postable geojson fence:\n%s", got.Message)
	}
}

func TestMCPCreatedGeoJSONMessageIsStampedAsACard(t *testing.T) {
	p := mcpPlugin(t)
	got := decodeMCPResult[CreatedGeoJSON](t, callMCPTool(t, agentsSession(t, p), "create_geojson", CreateGeoJSONArgs{
		Features: []CreateGeoJSONFeature{{Name: "Supply point", Kind: "point", Positions: []GeoJSONPosition{{Lat: 21.3353, Lon: -157.9483}}}},
	}))

	stamped := p.decoratePost(&model.Post{Message: got.Message, UserId: testUserID}, time.Now())
	if stamped == nil || stamped.Type != geojson.PostType {
		t.Fatalf("the created message was not stamped as a card: %+v", stamped)
	}
}

func TestMCPCreateGeoJSONRefusesWhatItCannotBuild(t *testing.T) {
	point := []GeoJSONPosition{{Lat: 1, Lon: 2}}
	for name, in := range map[string]CreateGeoJSONArgs{
		"no features":       {},
		"unknown kind":      {Features: []CreateGeoJSONFeature{{Name: "A", Kind: "circle", Positions: point}}},
		"point with two":    {Features: []CreateGeoJSONFeature{{Name: "A", Kind: "point", Positions: append(point, point...)}}},
		"line with one":     {Features: []CreateGeoJSONFeature{{Name: "A", Kind: "line", Positions: point}}},
		"polygon with two":  {Features: []CreateGeoJSONFeature{{Name: "A", Kind: "polygon", Positions: append(point, point...)}}},
		"latitude past 90":  {Features: []CreateGeoJSONFeature{{Name: "A", Kind: "point", Positions: []GeoJSONPosition{{Lat: 95, Lon: 2}}}}},
		"bad color":         {Features: []CreateGeoJSONFeature{{Name: "A", Kind: "point", Positions: point, Color: "red"}}},
		"unnamed feature":   {Features: []CreateGeoJSONFeature{{Kind: "point", Positions: point}}},
		"too many features": {Features: make([]CreateGeoJSONFeature, geojson.MaxFeatures+1)},
	} {
		t.Run(name, func(t *testing.T) {
			assertMCPRefusal(t, "create_geojson", in, errcode.MCPCreateGeoJSONInvalid)
		})
	}
}
