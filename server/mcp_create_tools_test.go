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

func TestMCPCreatesACotEventThatReadsBack(t *testing.T) {
	got := decodeMCPResult[CreatedCot](t, callMCPTool(t, agentsSession(t, mcpPlugin(t)), "create_cot", CreateCotArgs{
		Callsign: `TGT <01> & "B"`, Lat: 37.7749, Lon: -122.4194, Affiliation: "hostile", Dimension: "ground",
		Remarks: "two vehicles, stationary", StaleMinutes: 30,
	}))

	events, err := cot.Parse([]byte(got.XML))
	if err != nil || len(events) != 1 {
		t.Fatalf("the XML does not read back: %v\n%s", err, got.XML)
	}
	event := events[0]
	if event.Type != "a-h-G" || event.Detail.Callsign != `TGT <01> & "B"` || event.Point.Lat != "37.774900" || event.Point.Lon != "-122.419400" {
		t.Errorf("the event is %+v", event)
	}
	start, _ := time.Parse(time.RFC3339, event.Start)
	stale, _ := time.Parse(time.RFC3339, event.Stale)
	if stale.Sub(start) != 30*time.Minute {
		t.Errorf("the event is current for %v", stale.Sub(start))
	}
	if got.Event["affiliation"] != "hostile" || got.Event["callsign"] != `TGT <01> & "B"` {
		t.Errorf("the event reads back as %v", got.Event)
	}
	if !strings.HasPrefix(got.Message, "```cot\n<event") || !strings.HasSuffix(got.Message, "</event>\n```") || !got.FitsPost {
		t.Errorf("the message is not a postable cot fence:\n%s", got.Message)
	}
}

func TestMCPCreatedCotMessageIsStampedAsACard(t *testing.T) {
	p := mcpPlugin(t)
	got := decodeMCPResult[CreatedCot](t, callMCPTool(t, agentsSession(t, p), "create_cot", CreateCotArgs{Callsign: "ALPHA", Lat: 21.3353, Lon: -157.9483, Affiliation: "friend"}))

	stamped := p.decoratePost(&model.Post{Message: got.Message, UserId: testUserID}, time.Now())
	if stamped == nil || stamped.Type != cot.PostType {
		t.Fatalf("the created message was not stamped as a card: %+v", stamped)
	}
}

func TestMCPCreateCotTakesAnExactType(t *testing.T) {
	got := decodeMCPResult[CreatedCot](t, callMCPTool(t, agentsSession(t, mcpPlugin(t)), "create_cot",
		CreateCotArgs{Callsign: "ALPHA", Lat: 1, Lon: 2, Type: "a-f-G-U-C"}))
	if !strings.Contains(got.XML, `type="a-f-G-U-C"`) {
		t.Errorf("the exact type was not kept:\n%s", got.XML)
	}
}

func TestMCPCreateCotRefusesWhatItCannotBuild(t *testing.T) {
	for name, in := range map[string]CreateCotArgs{
		"no callsign":         {Lat: 1, Lon: 2},
		"latitude past 90":    {Callsign: "A", Lat: 91, Lon: 2},
		"longitude past 180":  {Callsign: "A", Lat: 1, Lon: 181},
		"unknown affiliation": {Callsign: "A", Lat: 1, Lon: 2, Affiliation: "ally"},
		"malformed type":      {Callsign: "A", Lat: 1, Lon: 2, Type: `a"><x`},
		"stale past a week":   {Callsign: "A", Lat: 1, Lon: 2, StaleMinutes: maxCotStaleMinutes + 1},
		"long remarks":        {Callsign: "A", Lat: 1, Lon: 2, Remarks: strings.Repeat("x", maxCreatedTextRunes+1)},
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
