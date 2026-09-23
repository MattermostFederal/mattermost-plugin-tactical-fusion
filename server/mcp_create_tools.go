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
	Callsign     string  `json:"callsign" jsonschema:"the name the event is shown under, such as ALPHA or TGT01"`
	Lat          float64 `json:"lat" jsonschema:"latitude in decimal degrees, -90 to 90"`
	Lon          float64 `json:"lon" jsonschema:"longitude in decimal degrees, -180 to 180"`
	Type         string  `json:"type,omitempty" jsonschema:"an exact CoT type such as a-h-G-U-C-A; when empty the type is built from affiliation and dimension"`
	Affiliation  string  `json:"affiliation,omitempty" jsonschema:"friend, hostile, neutral, unknown, suspect, assumed-friend or pending; default unknown"`
	Dimension    string  `json:"dimension,omitempty" jsonschema:"ground, air, sea, subsurface or space; default ground"`
	Remarks      string  `json:"remarks,omitempty" jsonschema:"free text remarks, up to 256 characters"`
	StaleMinutes int     `json:"stale_minutes,omitempty" jsonschema:"how long the event stays current, in minutes; default 10, at most one week"`
	HAE          float64 `json:"hae,omitempty" jsonschema:"height above the ellipsoid in meters; zero when unknown"`
	CE           float64 `json:"ce,omitempty" jsonschema:"circular error in meters; zero when unknown"`
}

type CreatedCot struct {
	XML      string         `json:"xml" jsonschema:"the event as CoT XML"`
	Message  string         `json:"message" jsonschema:"the event in a cot fenced code block; posted as a message on its own it renders as a Tactical Fusion card"`
	FitsPost bool           `json:"fits_post"`
	Event    map[string]any `json:"event" jsonschema:"the event read back, as decode_cot describes it"`
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
		if err == nil && len(described) == 1 {
			message := fenced(cotFenceInfo, source)
			return nil, CreatedCot{
				XML:      source,
				Message:  message,
				FitsPost: utf8.RuneCountInString(message) <= safePostRunes,
				Event:    described[0],
			}, nil
		}
		reason = "the event it built did not read back."
	}
	return toolRefusal(errcode.MCPCreateCotInvalid, "Could not build that event: "+reason), CreatedCot{Event: map[string]any{}}, nil
}

func cotSource(in CreateCotArgs, now time.Time) (string, string) {
	callsign := strings.TrimSpace(in.Callsign)
	switch {
	case callsign == "" || utf8.RuneCountInString(callsign) > maxCreatedTextRunes:
		return "", "a callsign is required, up to 256 characters."
	case !validLatLon(in.Lat, in.Lon):
		return "", "lat must be -90 to 90 and lon -180 to 180."
	case utf8.RuneCountInString(in.Remarks) > maxCreatedTextRunes:
		return "", "remarks are at most 256 characters."
	case in.StaleMinutes < 0 || in.StaleMinutes > maxCotStaleMinutes:
		return "", "stale_minutes must be 1 to 10080."
	}

	cotType, reason := cotTypeFor(in)
	if reason != "" {
		return "", reason
	}

	stale := in.StaleMinutes
	if stale == 0 {
		stale = defaultCotStaleMinutes
	}
	at := now.Truncate(time.Second)

	var detail strings.Builder
	detail.WriteString(`<contact callsign="` + xmlText(callsign) + `"/>`)
	if remarks := strings.TrimSpace(in.Remarks); remarks != "" {
		detail.WriteString(`<remarks>` + xmlText(remarks) + `</remarks>`)
	}

	return `<event version="2.0" uid="TF-` + model.NewId() + `" type="` + cotType + `" how="` + createdCotHow + `"` +
		` time="` + at.Format(time.RFC3339) + `" start="` + at.Format(time.RFC3339) + `"` +
		` stale="` + at.Add(time.Duration(stale)*time.Minute).Format(time.RFC3339) + `">` +
		`<point lat="` + decimal(in.Lat) + `" lon="` + decimal(in.Lon) + `" hae="` + measure(in.HAE) + `" ce="` + measure(in.CE) + `" le="` + cotUnknownMeasure + `"/>` +
		`<detail>` + detail.String() + `</detail></event>`, ""
}

func cotTypeFor(in CreateCotArgs) (string, string) {
	if raw := strings.TrimSpace(in.Type); raw != "" {
		if !cotTypeShape.MatchString(raw) {
			return "", "type must be a CoT type such as a-h-G-U-C-A."
		}
		return raw, ""
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
