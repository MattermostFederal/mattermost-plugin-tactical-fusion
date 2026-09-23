package main

import (
	"context"
	"strconv"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/avreport"
	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/cot"
	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/decorators/airport"
	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/decorators/dtg"
	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/decorators/frequency"
	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/errcode"
	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/geojson"
)

type DecodeAviationReportArgs struct {
	Text          string `json:"text" jsonschema:"one METAR, SPECI, TAF, FAA NOTAM (including a temporary flight restriction) or ICAO NOTAM, exactly as written; line breaks are allowed"`
	ReferenceTime int64  `json:"reference_time,omitempty" jsonschema:"Unix milliseconds supplying the month and year the report's day-of-month groups belong to; zero means now"`
}

type DecodeCotArgs struct {
	XML string `json:"xml" jsonschema:"one Cursor on Target event, or several, as XML"`
}

type SummarizeGeoJSONArgs struct {
	Document string `json:"document" jsonschema:"a GeoJSON document: a FeatureCollection, a Feature or a bare geometry"`
}

type DescribeFrequencyArgs struct {
	Frequency string `json:"frequency" jsonschema:"a radio frequency such as 121.5, 243.0 MHZ or 8992 KHZ; a decimal is megahertz and a whole number kilohertz; a leading FREQ: label is ignored"`
}

type ReadDateTimeArgs struct {
	Text          string `json:"text" jsonschema:"a military date-time group such as 141200ZSEP26 or 141200Z, or an RFC 3339 timestamp"`
	ReferenceTime int64  `json:"reference_time,omitempty" jsonschema:"Unix milliseconds supplying the month and year for a short date-time group such as 141200Z; zero means now"`
}

type AviationReportPoint struct {
	Lat float64 `json:"lat"`
	Lon float64 `json:"lon"`
}

type AviationReportRow struct {
	Label string `json:"label"`
	Value string `json:"value"`
	At    string `json:"at,omitempty" jsonschema:"the instant this row states, RFC 3339 in UTC"`
}

type AviationReportPeriod struct {
	Period string              `json:"period"`
	Rows   []AviationReportRow `json:"rows"`
}

type AviationReport struct {
	Kind        string                 `json:"kind" jsonschema:"METAR, SPECI, TAF or NOTAM"`
	Station     string                 `json:"station,omitempty"`
	StationName string                 `json:"station_name,omitempty"`
	IssuedAt    string                 `json:"issued_at,omitempty" jsonschema:"when a METAR or TAF was issued, or when a NOTAM takes effect, RFC 3339 in UTC"`
	Inferred    bool                   `json:"inferred" jsonschema:"true when the month and year were taken from reference_time rather than the report"`
	Restriction bool                   `json:"restriction" jsonschema:"true for a temporary flight restriction"`
	Flags       []string               `json:"flags"`
	Rows        []AviationReportRow    `json:"rows"`
	Periods     []AviationReportPeriod `json:"periods"`
	Remarks     []AviationReportRow    `json:"remarks"`
	NotDecoded  []string               `json:"not_decoded" jsonschema:"groups this build could not read, as written"`
	Center      *AviationReportPoint   `json:"center,omitempty" jsonschema:"the station, a NOTAM's circle center, or a restricted area's centroid"`
	RadiusNm    string                 `json:"radius_nm,omitempty" jsonschema:"the radius of a NOTAM's circle in nautical miles"`
	Area        []AviationReportPoint  `json:"area,omitempty" jsonschema:"the vertices of a restricted area's polygon"`
	Truncated   bool                   `json:"truncated" jsonschema:"true when the report was longer than this build lists"`
}

type CotEvents struct {
	Events []map[string]any `json:"events"`
}

type GeoJSONSummary struct {
	Name        string           `json:"name,omitempty"`
	Description string           `json:"description,omitempty"`
	Note        string           `json:"note,omitempty"`
	Counts      GeoJSONCounts    `json:"counts"`
	Features    []map[string]any `json:"features"`
}

type GeoJSONCounts struct {
	Features    int `json:"features"`
	Points      int `json:"points"`
	Lines       int `json:"lines"`
	Polygons    int `json:"polygons"`
	Collections int `json:"collections"`
	Unlocated   int `json:"unlocated" jsonschema:"features the document gives no position"`
	Undrawable  int `json:"undrawable" jsonschema:"features whose geometry could not be drawn"`
}

type FrequencyDescription struct {
	Token   string `json:"token"`
	MHz     string `json:"mhz"`
	KHz     string `json:"khz"`
	Band    string `json:"band"`
	Channel string `json:"channel,omitempty"`
	Use     string `json:"use,omitempty"`
}

type DateTime struct {
	Canonical     string `json:"canonical"`
	UTC           string `json:"utc" jsonschema:"RFC 3339 in UTC"`
	UnixMillis    int64  `json:"unix_ms"`
	ZoneLetter    string `json:"zone_letter,omitempty" jsonschema:"the military time zone letter a date-time group carries"`
	OffsetMinutes *int   `json:"offset_minutes,omitempty" jsonschema:"the UTC offset an RFC 3339 timestamp carries"`
	Assumed       string `json:"assumed,omitempty" jsonschema:"which parts were taken from reference_time: m for the month, y for the year"`
}

func (p *Plugin) decodeAviationReportTool(_ context.Context, _ *mcp.CallToolRequest, in DecodeAviationReportArgs) (*mcp.CallToolResult, AviationReport, error) {
	report, err := avreport.Decode(in.Text, bridgeReferenceTime(in.ReferenceTime))
	if err != nil {
		return toolRefusal(errcode.MCPAvReportInvalid,
			"That is not a METAR, SPECI, TAF or NOTAM this plugin reads."), AviationReport{}, nil
	}

	return nil, aviationReportOf(report), nil
}

func aviationReportOf(report avreport.Report) AviationReport {
	out := AviationReport{
		Kind:        report.Kind,
		Station:     report.Station,
		StationName: report.StationName,
		IssuedAt:    rfc3339(report.IssuedAt),
		Inferred:    report.Inferred,
		Restriction: report.IsRestriction(),
		Flags:       append([]string{}, report.Flags...),
		Rows:        aviationRows(report.Rows),
		Periods:     []AviationReportPeriod{},
		Remarks:     aviationRows(report.Remarks),
		NotDecoded:  append([]string{}, report.Unknown...),
		RadiusNm:    report.RadiusNm,
		Truncated:   report.Truncated,
	}
	for _, period := range report.Periods {
		out.Periods = append(out.Periods, AviationReportPeriod{Period: period.Period, Rows: aviationRows(period.Rows)})
	}
	if report.Center != nil {
		out.Center = &AviationReportPoint{Lat: report.Center.Lat, Lon: report.Center.Lon}
	} else if details, ok := airport.Describe(report.Station); ok && details.HasPosition {
		field, _ := airport.Lookup(report.Station)
		out.Center = &AviationReportPoint{Lat: field.Lat, Lon: field.Lon}
	}
	for _, vertex := range report.Area {
		out.Area = append(out.Area, AviationReportPoint{Lat: vertex.Lat, Lon: vertex.Lon})
	}
	return out
}

func aviationRows(rows []avreport.Row) []AviationReportRow {
	out := make([]AviationReportRow, 0, len(rows))
	for _, row := range rows {
		out = append(out, AviationReportRow{Label: row.Label, Value: row.Value, At: rfc3339(row.At)})
	}
	return out
}

func rfc3339(at time.Time) string {
	if at.IsZero() {
		return ""
	}
	return at.UTC().Format(time.RFC3339)
}

func (p *Plugin) decodeCotTool(_ context.Context, _ *mcp.CallToolRequest, in DecodeCotArgs) (*mcp.CallToolResult, CotEvents, error) {
	events, err := cot.Parse([]byte(in.XML))
	if err != nil {
		return toolRefusal(errcode.MCPCotInvalid, "That is not a Cursor on Target event this plugin reads: "+errorReason(err)), CotEvents{}, nil
	}

	blob := cot.Props(events, cot.Source{Kind: cot.SourceFence})
	return nil, CotEvents{Events: objects(blob["events"])}, nil
}

func (p *Plugin) summarizeGeoJSONTool(_ context.Context, _ *mcp.CallToolRequest, in SummarizeGeoJSONArgs) (*mcp.CallToolResult, GeoJSONSummary, error) {
	document, err := geojson.Parse([]byte(in.Document))
	if err != nil {
		return toolRefusal(errcode.MCPGeoJSONInvalid, "That is not a GeoJSON document this plugin reads: "+errorReason(err)), GeoJSONSummary{}, nil
	}

	return nil, geoJSONSummaryOf(document), nil
}

func geoJSONSummaryOf(document *geojson.Document) GeoJSONSummary {
	blob := geojson.Props(document, geojson.Source{Kind: geojson.SourceFence})
	counts := document.Counts()
	return GeoJSONSummary{
		Name:        document.Name,
		Description: document.Description,
		Note:        document.Note,
		Counts: GeoJSONCounts{
			Features:    counts.Features,
			Points:      counts.Points,
			Lines:       counts.Lines,
			Polygons:    counts.Polygons,
			Collections: counts.Collections,
			Unlocated:   counts.Unlocated,
			Undrawable:  counts.Undrawable,
		},
		Features: objects(blob["features"]),
	}
}

func objects(value any) []map[string]any {
	items, _ := value.([]any)
	out := make([]map[string]any, 0, len(items))
	for _, item := range items {
		if object, ok := item.(map[string]any); ok {
			out = append(out, object)
		}
	}
	return out
}

func errorReason(err error) string {
	text := err.Error()
	if _, after, found := strings.Cut(text, ": "); found {
		return after + "."
	}
	return text + "."
}

func (p *Plugin) describeFrequencyTool(_ context.Context, _ *mcp.CallToolRequest, in DescribeFrequencyArgs) (*mcp.CallToolResult, FrequencyDescription, error) {
	token := strings.TrimSpace(in.Frequency)
	if label, rest, found := strings.Cut(token, ":"); found && strings.EqualFold(strings.TrimSpace(label), "FREQ") {
		token = strings.TrimSpace(rest)
	}

	parsed, ok := frequency.ParseToken(strings.ToUpper(token))
	if !ok {
		return toolRefusal(errcode.MCPFrequencyInvalid,
			"That is not a frequency between 2 MHz and 1,300 MHz this plugin reads."), FrequencyDescription{}, nil
	}

	details := frequency.Describe(parsed)
	return nil, FrequencyDescription{
		Token:   details.Token,
		MHz:     details.MHz,
		KHz:     details.KHz,
		Band:    details.Band,
		Channel: details.Channel,
		Use:     details.Use,
	}, nil
}

func (p *Plugin) readDateTimeTool(_ context.Context, _ *mcp.CallToolRequest, in ReadDateTimeArgs) (*mcp.CallToolResult, DateTime, error) {
	params, ok := (&dtg.Decorator{}).Parse(strings.TrimSpace(in.Text), bridgeReferenceTime(in.ReferenceTime))
	millis, err := strconv.ParseInt(params.Get("t"), 10, 64)
	if !ok || err != nil {
		return toolRefusal(errcode.MCPDateTimeInvalid,
			"That is not a date-time group or RFC 3339 timestamp this plugin reads."), DateTime{}, nil
	}

	out := DateTime{
		Canonical:  params.Get("dtg"),
		UTC:        time.UnixMilli(millis).UTC().Format(time.RFC3339),
		UnixMillis: millis,
		ZoneLetter: params.Get("z"),
		Assumed:    params.Get("a"),
	}
	if params.Has("o") {
		if offset, err := strconv.Atoi(params.Get("o")); err == nil {
			out.OffsetMinutes = &offset
		}
	}
	return nil, out, nil
}
