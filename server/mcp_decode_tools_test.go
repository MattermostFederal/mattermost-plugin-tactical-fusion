package main

import (
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/avreport"
	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/decorators/location"
	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/errcode"
)

const mcpMETAR = "METAR PHNL 221653Z 07012KT 10SM FEW030 27/19 A3010"

var mcpReference = time.Date(2026, time.September, 22, 18, 0, 0, 0, time.UTC)

func assertMCPRefusal(t *testing.T, tool string, in any, code int) {
	t.Helper()

	result := callMCPTool(t, agentsSession(t, mcpPlugin(t)), tool, in)
	if !result.IsError || !strings.Contains(mcpResultText(result), "(TF-"+strconv.Itoa(code)+")") {
		t.Errorf("%s answered %q, want a TF-%d refusal", tool, mcpResultText(result), code)
	}
}

func TestMCPDecodesAMETARAsTheReportDecoderDoes(t *testing.T) {
	session := agentsSession(t, mcpPlugin(t))

	got := decodeMCPResult[AviationReport](t, callMCPTool(t, session, "decode_aviation_report",
		DecodeAviationReportArgs{Text: mcpMETAR, ReferenceTime: mcpReference.UnixMilli()}))

	report, err := avreport.Decode(mcpMETAR, mcpReference)
	if err != nil {
		t.Fatal(err)
	}
	if want := aviationReportOf(report); !reflect.DeepEqual(got, want) {
		t.Errorf("decode_aviation_report = %+v, want %+v", got, want)
	}
	if got.Kind != "METAR" || got.Station != "PHNL" || got.IssuedAt != "2026-09-22T16:53:00Z" || got.Center == nil || got.Restriction {
		t.Errorf("the METAR decoded as %+v", got)
	}
}

func TestMCPDecodesATFRWithItsCircleAndTimes(t *testing.T) {
	session := agentsSession(t, mcpPlugin(t))

	got := decodeMCPResult[AviationReport](t, callMCPTool(t, session, "decode_aviation_report",
		DecodeAviationReportArgs{Text: tfrExampleMessage(mcpReference), ReferenceTime: mcpReference.UnixMilli()}))

	if !got.Restriction || got.RadiusNm != "3" || got.Center == nil || got.IssuedAt != "2026-09-22T18:00:00Z" {
		t.Fatalf("the TFR decoded as %+v", got)
	}
	var expires string
	for _, row := range got.Rows {
		if row.Label == "Expires" {
			expires = row.At
		}
	}
	if expires != "2026-09-23T02:00:00Z" {
		t.Errorf("the Expires row carries %q", expires)
	}
}

func TestMCPDecodesATFRPolygon(t *testing.T) {
	text := "!FDC 6/5678 ZZZ AIRSPACE TEMPORARY FLIGHT RESTRICTIONS WI AN AREA DEFINED AS " +
		"433700N1161200W TO 434000N1160000W TO 432000N1160000W TO POINT OF ORIGIN SFC-FL180. EFFECTIVE IMMEDIATELY UNTIL FURTHER NOTICE."
	got := decodeMCPResult[AviationReport](t, callMCPTool(t, agentsSession(t, mcpPlugin(t)), "decode_aviation_report",
		DecodeAviationReportArgs{Text: text}))

	if !got.Restriction || len(got.Area) != 3 || got.Center == nil || got.RadiusNm != "" {
		t.Errorf("the polygon TFR decoded as %+v", got)
	}
}

func TestMCPDecodeAviationReportRefusesProse(t *testing.T) {
	assertMCPRefusal(t, "decode_aviation_report", DecodeAviationReportArgs{Text: "hello there"}, errcode.MCPAvReportInvalid)
}

func TestMCPDecodesACotEventInWords(t *testing.T) {
	got := decodeMCPResult[CotEvents](t, callMCPTool(t, agentsSession(t, mcpPlugin(t)), "decode_cot",
		DecodeCotArgs{XML: cotExampleTarget}))

	if len(got.Events) != 1 {
		t.Fatalf("decode_cot returned %d events", len(got.Events))
	}
	event := got.Events[0]
	for key, want := range map[string]any{"uid": "TGT-9F2A1C7B", "cot_type": "a-h-G-U-C-A", "callsign": "TGT01", "affiliation": "hostile"} {
		if event[key] != want {
			t.Errorf("%s = %v, want %v", key, event[key], want)
		}
	}
	if label, _ := event["type_label"].(string); label == "" {
		t.Error("the event carries no words for its type")
	}
}

func TestMCPDecodeCotRefusesWhatIsNotAnEvent(t *testing.T) {
	assertMCPRefusal(t, "decode_cot", DecodeCotArgs{XML: "<note/>"}, errcode.MCPCotInvalid)
}

func TestMCPSummarizesAGeoJSONDocument(t *testing.T) {
	got := decodeMCPResult[GeoJSONSummary](t, callMCPTool(t, agentsSession(t, mcpPlugin(t)), "summarize_geojson",
		SummarizeGeoJSONArgs{Document: geoJSONExample}))

	if got.Name != "Pearl Harbor resupply" || got.Description == "" {
		t.Errorf("name %q description %q", got.Name, got.Description)
	}
	if len(got.Features) == 0 || got.Counts.Features != len(got.Features) {
		t.Errorf("counts %v for %d features", got.Counts, len(got.Features))
	}
	if got.Features[0]["name"] != "Forward supply point" {
		t.Errorf("the first feature is %v", got.Features[0]["name"])
	}
}

func TestMCPSummarizeGeoJSONRefusesOrdinaryJSON(t *testing.T) {
	assertMCPRefusal(t, "summarize_geojson", SummarizeGeoJSONArgs{Document: `{"hello":"world"}`}, errcode.MCPGeoJSONInvalid)
}

func TestMCPDescribesAFrequencyWithOrWithoutItsLabel(t *testing.T) {
	session := agentsSession(t, mcpPlugin(t))

	for _, token := range []string{"121.5", "FREQ:121.5", "freq: 121.5 mhz"} {
		got := decodeMCPResult[FrequencyDescription](t, callMCPTool(t, session, "describe_frequency",
			DescribeFrequencyArgs{Frequency: token}))
		if got.MHz != "121.500" || got.Band == "" || got.Use == "" {
			t.Errorf("%q described as %+v", token, got)
		}
	}
}

func TestMCPDescribeFrequencyRefusesOutOfRange(t *testing.T) {
	assertMCPRefusal(t, "describe_frequency", DescribeFrequencyArgs{Frequency: "1.0"}, errcode.MCPFrequencyInvalid)
}

func TestMCPReadsADateTimeGroupToOneInstant(t *testing.T) {
	session := agentsSession(t, mcpPlugin(t))

	full := decodeMCPResult[DateTime](t, callMCPTool(t, session, "read_date_time", ReadDateTimeArgs{Text: "141200ZSEP26"}))
	if full.UTC != "2026-09-14T12:00:00Z" || full.ZoneLetter != "Z" || full.Assumed != "" {
		t.Errorf("141200ZSEP26 read as %+v", full)
	}

	short := decodeMCPResult[DateTime](t, callMCPTool(t, session, "read_date_time",
		ReadDateTimeArgs{Text: "141200Z", ReferenceTime: mcpReference.UnixMilli()}))
	if short.UTC != "2026-09-14T12:00:00Z" || short.Assumed == "" {
		t.Errorf("141200Z read as %+v", short)
	}

	stamp := decodeMCPResult[DateTime](t, callMCPTool(t, session, "read_date_time", ReadDateTimeArgs{Text: "2026-08-09T20:30:00+04:00"}))
	if stamp.UTC != "2026-08-09T16:30:00Z" || stamp.OffsetMinutes == nil || *stamp.OffsetMinutes != 240 {
		t.Errorf("the timestamp read as %+v", stamp)
	}
}

func TestMCPReadDateTimeRefusesProse(t *testing.T) {
	assertMCPRefusal(t, "read_date_time", ReadDateTimeArgs{Text: "next tuesday"}, errcode.MCPDateTimeInvalid)
}

func TestMCPLooksUpAnAirfieldByIATACode(t *testing.T) {
	got := decodeMCPResult[airportResponse](t, callMCPTool(t, agentsSession(t, mcpPlugin(t)), "lookup_airfield",
		LookupAirfieldArgs{Ident: "hnl"}))
	if !got.Found || got.Ident != "PHNL" {
		t.Errorf("HNL resolved to %+v", got)
	}
}

func TestMCPConvertNamesEveryFormatIDAndNoOther(t *testing.T) {
	field, _ := reflect.TypeFor[ConvertCoordinateArgs]().FieldByName("Format")
	schema := field.Tag.Get("jsonschema")
	_, list, _ := strings.Cut(schema, ": ")

	named := strings.FieldsFunc(list, func(r rune) bool { return r == ',' || r == ' ' })
	named = slicesWithout(named, "or")

	want := make([]string, 0, len(location.AllFormatIDs))
	for _, id := range location.AllFormatIDs {
		want = append(want, string(id))
	}
	if !reflect.DeepEqual(named, want) {
		t.Errorf("the format description names %v, want %v", named, want)
	}
}

func slicesWithout(values []string, drop string) []string {
	out := values[:0]
	for _, value := range values {
		if value != drop {
			out = append(out, value)
		}
	}
	return out
}
