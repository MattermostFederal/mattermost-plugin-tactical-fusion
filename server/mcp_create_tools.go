package main

import (
	"bytes"
	"context"
	"encoding/json"
	"encoding/xml"
	"math"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/cot"
	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/errcode"
	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/geojson"
)

const (
	defaultCotStaleMinutes = 10
	maxCotStaleMinutes     = 7 * 24 * 60
	maxCreatedTextRunes    = 256
	createdCotHow          = "h-e"
	cotUnknownMeasure      = "9999999.0"
	cotDrawnCircleType     = "u-d-c-c"
	cotDrawnShapeType      = "u-d-f"
)

var (
	cotAffiliationCodes = map[string]string{
		"pending": "p", "unknown": "u", "assumed-friend": "a", "friend": "f", "friendly": "f",
		"neutral": "n", "suspect": "s", "hostile": "h", "enemy": "h", "joker": "j", "faker": "k", "none": "o",
	}
	cotDimensionCodes = map[string]string{
		"space": "P", "air": "A", "ground": "G", "land": "G", "sea": "S", "surface": "S", "subsurface": "U",
	}
	cotTypeShape = regexp.MustCompile(`^[a-z](?:-[A-Za-z0-9]{1,8}){1,12}$`)
	colorShape   = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)
)

type CreateCotArgs struct {
	Events       []CreateCotEvent `json:"events" jsonschema:"1 to 32 events, written in order into one message"`
	StaleMinutes int              `json:"stale_minutes,omitempty" jsonschema:"how long the events stay current, in minutes; default 10, at most one week"`
}

type CreateCotEvent struct {
	Callsign    string          `json:"callsign" jsonschema:"the name the event is shown under, such as ALPHA, TGT01 or OBJ AREA"`
	Lat         *float64        `json:"lat,omitempty" jsonschema:"latitude in decimal degrees, -90 to 90; required for a point or a circle, and a line or polygon defaults to its first position"`
	Lon         *float64        `json:"lon,omitempty" jsonschema:"longitude in decimal degrees, -180 to 180; required with lat"`
	Type        string          `json:"type,omitempty" jsonschema:"an exact CoT type such as a-h-G-U-C-A; when empty a point is built from affiliation and dimension, a circle is u-d-c-c and a line or polygon u-d-f"`
	Affiliation string          `json:"affiliation,omitempty" jsonschema:"friend, hostile, neutral, unknown, suspect, assumed-friend or pending; default unknown"`
	Dimension   string          `json:"dimension,omitempty" jsonschema:"ground, air, sea, subsurface or space; default ground"`
	Remarks     string          `json:"remarks,omitempty" jsonschema:"free text remarks, up to 256 characters"`
	HAE         float64         `json:"hae,omitempty" jsonschema:"height above the ellipsoid in meters; zero when unknown"`
	CE          float64         `json:"ce,omitempty" jsonschema:"circular error in meters; zero when unknown"`
	Color       string          `json:"color,omitempty" jsonschema:"a #rrggbb color the event and its shape are drawn in"`
	Shape       *CreateCotShape `json:"shape,omitempty" jsonschema:"a shape to draw instead of a plain point"`
}

type CreateCotShape struct {
	Kind         string            `json:"kind" jsonschema:"circle, line or polygon"`
	RadiusMeters float64           `json:"radius_meters,omitempty" jsonschema:"a circle's radius in meters, centered on the event's lat and lon"`
	Positions    []GeoJSONPosition `json:"positions,omitempty" jsonschema:"two or more positions in order for a line, three or more around a polygon (it is closed for you)"`
}

type CreatedCot struct {
	XML      string           `json:"xml" jsonschema:"the events as CoT XML"`
	Message  string           `json:"message" jsonschema:"the events in a cot fenced code block; posted as a message on its own it renders as a Tactical Fusion card with a map"`
	FitsPost bool             `json:"fits_post"`
	Events   []map[string]any `json:"events" jsonschema:"the events read back, as decode_cot describes them"`
}

type CreateGeoJSONArgs struct {
	Name        string                 `json:"name,omitempty" jsonschema:"a name for the whole document"`
	Description string                 `json:"description,omitempty" jsonschema:"a description for the whole document"`
	Features    []CreateGeoJSONFeature `json:"features" jsonschema:"the points, lines and polygons to draw"`
}

type CreateGeoJSONFeature struct {
	Name        string            `json:"name" jsonschema:"what the feature is called"`
	Description string            `json:"description,omitempty"`
	Kind        string            `json:"kind" jsonschema:"point, line or polygon"`
	Positions   []GeoJSONPosition `json:"positions" jsonschema:"one position for a point, two or more in order for a line, three or more around a polygon (it is closed for you)"`
	Color       string            `json:"color,omitempty" jsonschema:"a #rrggbb color for the stroke, fill or marker"`
	Properties  map[string]string `json:"properties,omitempty" jsonschema:"extra key and value pairs shown in the sidebar"`
}

type GeoJSONPosition struct {
	Lat float64 `json:"lat"`
	Lon float64 `json:"lon"`
}

type CreatedGeoJSON struct {
	Document string         `json:"document" jsonschema:"the GeoJSON document"`
	Message  string         `json:"message" jsonschema:"the document in a geojson fenced code block; posted as a message on its own it renders as a Tactical Fusion card with a map"`
	FitsPost bool           `json:"fits_post"`
	Summary  GeoJSONSummary `json:"summary" jsonschema:"the document read back, as summarize_geojson describes it"`
}

func (p *Plugin) createCotTool(_ context.Context, _ *mcp.CallToolRequest, in CreateCotArgs) (*mcp.CallToolResult, CreatedCot, error) {
	source, reason := cotSource(in, time.Now().UTC())
	if reason == "" {
		events, err := cot.Parse([]byte(source))
		described := objects(cot.Props(events, cot.Source{Kind: cot.SourceFence})["events"])
		if err == nil && len(described) == len(in.Events) {
			message := fenced(cotFenceInfo, source)
			return nil, CreatedCot{
				XML:      source,
				Message:  message,
				FitsPost: utf8.RuneCountInString(message) <= safePostRunes,
				Events:   described,
			}, nil
		}
		reason = "the events it built did not read back."
	}
	return toolRefusal(errcode.MCPCreateCotInvalid, "Could not build that event: "+reason), CreatedCot{Events: []map[string]any{}}, nil
}

func cotSource(in CreateCotArgs, now time.Time) (string, string) {
	switch {
	case len(in.Events) == 0 || len(in.Events) > cot.MaxEvents:
		return "", "give 1 to " + strconv.Itoa(cot.MaxEvents) + " events."
	case in.StaleMinutes < 0 || in.StaleMinutes > maxCotStaleMinutes:
		return "", "stale_minutes must be 1 to 10080."
	}

	stale := in.StaleMinutes
	if stale == 0 {
		stale = defaultCotStaleMinutes
	}
	at := now.Truncate(time.Second)
	timing := ` time="` + at.Format(time.RFC3339) + `" start="` + at.Format(time.RFC3339) + `"` +
		` stale="` + at.Add(time.Duration(stale)*time.Minute).Format(time.RFC3339) + `"`

	var source strings.Builder
	vertices := 0
	for i, event := range in.Events {
		written, count, reason := cotEventSource(event, timing)
		if reason != "" {
			return "", "event " + strconv.Itoa(i+1) + ": " + reason
		}
		vertices += count
		if vertices > cot.MaxVertices {
			return "", "the shapes have more than " + strconv.Itoa(cot.MaxVertices) + " positions between them."
		}
		if i > 0 {
			source.WriteString("\n")
		}
		source.WriteString(written)
	}
	if source.Len() > cot.MaxSourceBytes {
		return "", "the events are larger than " + strconv.Itoa(cot.MaxSourceBytes/1024) + " KB."
	}
	return source.String(), ""
}

func cotEventSource(in CreateCotEvent, timing string) (string, int, string) {
	callsign := strings.TrimSpace(in.Callsign)
	switch {
	case callsign == "" || utf8.RuneCountInString(callsign) > maxCreatedTextRunes:
		return "", 0, "a callsign is required, up to 256 characters."
	case utf8.RuneCountInString(in.Remarks) > maxCreatedTextRunes:
		return "", 0, "remarks are at most 256 characters."
	}

	shape, count, reason := cotShapeSource(in.Shape)
	if reason != "" {
		return "", 0, reason
	}
	lat, lon, reason := cotEventPosition(in)
	if reason != "" {
		return "", 0, reason
	}
	cotType, reason := cotTypeFor(in)
	if reason != "" {
		return "", 0, reason
	}
	color, reason := cotColor(in.Color)
	if reason != "" {
		return "", 0, reason
	}

	var detail strings.Builder
	detail.WriteString(`<contact callsign="` + xmlText(callsign) + `"/>`)
	if color != "" {
		detail.WriteString(`<color argb="` + color + `"/>`)
	}
	detail.WriteString(shape)
	if remarks := strings.TrimSpace(in.Remarks); remarks != "" {
		detail.WriteString(`<remarks>` + xmlText(remarks) + `</remarks>`)
	}

	return `<event version="2.0" uid="TF-` + model.NewId() + `" type="` + cotType + `" how="` + createdCotHow + `"` + timing + `>` +
		`<point lat="` + decimal(lat) + `" lon="` + decimal(lon) + `" hae="` + measure(in.HAE) + `" ce="` + measure(in.CE) + `" le="` + cotUnknownMeasure + `"/>` +
		`<detail>` + detail.String() + `</detail></event>`, count, ""
}

func cotEventPosition(in CreateCotEvent) (float64, float64, string) {
	if in.Lat != nil && in.Lon != nil {
		if !validLatLon(*in.Lat, *in.Lon) {
			return 0, 0, "lat must be -90 to 90 and lon -180 to 180."
		}
		return *in.Lat, *in.Lon, ""
	}
	if in.Lat == nil && in.Lon == nil && in.Shape != nil && len(in.Shape.Positions) > 0 && cotShapeKind(in.Shape) != cotShapeCircle {
		first := in.Shape.Positions[0]
		return first.Lat, first.Lon, ""
	}
	return 0, 0, "lat and lon are required for a point or a circle."
}

const (
	cotShapeCircle  = "circle"
	cotShapeLine    = "line"
	cotShapePolygon = "polygon"

	maxCotCircleMeters = 20_000_000
	opaqueArgbOffset   = 1 << 24
)

func cotShapeKind(shape *CreateCotShape) string {
	return strings.ToLower(strings.TrimSpace(shape.Kind))
}

func cotShapeSource(shape *CreateCotShape) (string, int, string) {
	if shape == nil {
		return "", 0, ""
	}

	switch cotShapeKind(shape) {
	case cotShapeCircle:
		radius := shape.RadiusMeters
		if math.IsNaN(radius) || radius <= 0 || radius > maxCotCircleMeters {
			return "", 0, "a circle takes a radius_meters above 0 and at most 20000000."
		}
		axis := strconv.FormatFloat(radius, 'f', 1, 64)
		return `<shape><ellipse major="` + axis + `" minor="` + axis + `" angle="0"/></shape>`, 0, ""

	case cotShapeLine, cotShapePolygon:
		closed := cotShapeKind(shape) == cotShapePolygon
		positions := shape.Positions
		switch {
		case !closed && len(positions) < 2:
			return "", 0, "a line takes at least two positions."
		case closed && len(positions) < 3:
			return "", 0, "a polygon takes at least three positions."
		}

		var polyline strings.Builder
		polyline.WriteString(`<shape><polyline closed="` + strconv.FormatBool(closed) + `">`)
		for _, position := range positions {
			if !validLatLon(position.Lat, position.Lon) {
				return "", 0, "lat must be -90 to 90 and lon -180 to 180."
			}
			polyline.WriteString(`<vertex lat="` + decimal(position.Lat) + `" lon="` + decimal(position.Lon) + `"/>`)
		}
		polyline.WriteString(`</polyline></shape>`)
		return polyline.String(), len(positions), ""
	}
	return "", 0, "a shape's kind must be circle, line or polygon."
}

func cotColor(raw string) (string, string) {
	color := strings.TrimSpace(raw)
	if color == "" {
		return "", ""
	}
	if !colorShape.MatchString(color) {
		return "", "color must be #rrggbb."
	}
	rgb, _ := strconv.ParseInt(color[1:], 16, 64)
	return strconv.FormatInt(rgb-opaqueArgbOffset, 10), ""
}

func cotTypeFor(in CreateCotEvent) (string, string) {
	if raw := strings.TrimSpace(in.Type); raw != "" {
		if !cotTypeShape.MatchString(raw) {
			return "", "type must be a CoT type such as a-h-G-U-C-A."
		}
		return raw, ""
	}
	if in.Shape != nil {
		if cotShapeKind(in.Shape) == cotShapeCircle {
			return cotDrawnCircleType, ""
		}
		return cotDrawnShapeType, ""
	}

	affiliation := strings.ToLower(strings.TrimSpace(in.Affiliation))
	if affiliation == "" {
		affiliation = "unknown"
	}
	dimension := strings.ToLower(strings.TrimSpace(in.Dimension))
	if dimension == "" {
		dimension = "ground"
	}
	a, okA := cotAffiliationCodes[affiliation]
	d, okD := cotDimensionCodes[dimension]
	if !okA || !okD {
		return "", "affiliation must be friend, hostile, neutral, unknown, suspect, assumed-friend or pending, and dimension ground, air, sea, subsurface or space."
	}
	return "a-" + a + "-" + d, ""
}

func (p *Plugin) createGeoJSONTool(_ context.Context, _ *mcp.CallToolRequest, in CreateGeoJSONArgs) (*mcp.CallToolResult, CreatedGeoJSON, error) {
	document, reason := geoJSONSource(in)
	if reason == "" {
		parsed, err := geojson.Parse([]byte(document))
		if err == nil {
			message := fenced(geoJSONFenceInfo, document)
			return nil, CreatedGeoJSON{
				Document: document,
				Message:  message,
				FitsPost: utf8.RuneCountInString(message) <= safePostRunes,
				Summary:  geoJSONSummaryOf(parsed),
			}, nil
		}
		reason = "the document it built did not read back: " + errorReason(err)
	}
	return toolRefusal(errcode.MCPCreateGeoJSONInvalid, "Could not build that document: "+reason), CreatedGeoJSON{}, nil
}

func geoJSONSource(in CreateGeoJSONArgs) (string, string) {
	if len(in.Features) == 0 || len(in.Features) > geojson.MaxFeatures {
		return "", "give 1 to " + strconv.Itoa(geojson.MaxFeatures) + " features."
	}
	if utf8.RuneCountInString(in.Name) > maxCreatedTextRunes || utf8.RuneCountInString(in.Description) > maxCreatedTextRunes {
		return "", "the name and description are at most 256 characters each."
	}

	features := make([]map[string]any, 0, len(in.Features))
	vertices := 0
	for i, feature := range in.Features {
		geometry, count, reason := geoJSONGeometry(feature)
		if reason != "" {
			return "", "feature " + strconv.Itoa(i+1) + ": " + reason
		}
		vertices += count
		if vertices > geojson.MaxVertices {
			return "", "the features have more than " + strconv.Itoa(geojson.MaxVertices) + " positions between them."
		}

		properties := map[string]any{}
		for key, value := range feature.Properties {
			properties[key] = value
		}
		properties["name"] = strings.TrimSpace(feature.Name)
		if description := strings.TrimSpace(feature.Description); description != "" {
			properties["description"] = description
		}
		if color := strings.TrimSpace(feature.Color); color != "" {
			if !colorShape.MatchString(color) {
				return "", "feature " + strconv.Itoa(i+1) + ": color must be #rrggbb."
			}
			for _, key := range []string{"stroke", "fill", "marker-color"} {
				properties[key] = color
			}
		}

		features = append(features, map[string]any{"type": "Feature", "geometry": geometry, "properties": properties})
	}

	document := map[string]any{"type": "FeatureCollection", "features": features}
	if name := strings.TrimSpace(in.Name); name != "" {
		document["name"] = name
	}
	if description := strings.TrimSpace(in.Description); description != "" {
		document["description"] = description
	}

	var out bytes.Buffer
	encoder := json.NewEncoder(&out)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(document); err != nil {
		return "", "it could not be written as JSON."
	}
	if out.Len() > geojson.MaxSourceBytes {
		return "", "the document is larger than " + strconv.Itoa(geojson.MaxSourceBytes/1024) + " KB."
	}
	return strings.TrimSpace(out.String()), ""
}

func geoJSONGeometry(feature CreateGeoJSONFeature) (map[string]any, int, string) {
	if strings.TrimSpace(feature.Name) == "" || utf8.RuneCountInString(feature.Name) > maxCreatedTextRunes {
		return nil, 0, "a name is required, up to 256 characters."
	}
	positions := make([][]float64, 0, len(feature.Positions)+1)
	for _, position := range feature.Positions {
		if !validLatLon(position.Lat, position.Lon) {
			return nil, 0, "lat must be -90 to 90 and lon -180 to 180."
		}
		positions = append(positions, []float64{position.Lon, position.Lat})
	}

	switch strings.ToLower(strings.TrimSpace(feature.Kind)) {
	case "point":
		if len(positions) != 1 {
			return nil, 0, "a point takes exactly one position."
		}
		return map[string]any{"type": "Point", "coordinates": positions[0]}, 1, ""
	case "line":
		if len(positions) < 2 {
			return nil, 0, "a line takes at least two positions."
		}
		return map[string]any{"type": "LineString", "coordinates": positions}, len(positions), ""
	case "polygon":
		if len(positions) < 3 {
			return nil, 0, "a polygon takes at least three positions."
		}
		first, last := positions[0], positions[len(positions)-1]
		if first[0] != last[0] || first[1] != last[1] {
			positions = append(positions, first)
		}
		return map[string]any{"type": "Polygon", "coordinates": [][][]float64{positions}}, len(positions), ""
	}
	return nil, 0, "kind must be point, line or polygon."
}

func validLatLon(lat, lon float64) bool {
	return !math.IsNaN(lat) && !math.IsNaN(lon) && lat >= -90 && lat <= 90 && lon >= -180 && lon <= 180
}

func fenced(info, source string) string {
	return "```" + info + "\n" + source + "\n```"
}

func decimal(value float64) string {
	return strconv.FormatFloat(value, 'f', 6, 64)
}

func measure(value float64) string {
	if value == 0 {
		return cotUnknownMeasure
	}
	return strconv.FormatFloat(value, 'f', 1, 64)
}

func xmlText(value string) string {
	var out strings.Builder
	_ = xml.EscapeText(&out, []byte(value))
	return out.String()
}
