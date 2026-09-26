package main

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/bridgeclient"
	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/decorators/airport"
	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/decorators/dtg"
	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/decorators/location"
	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/errcode"
)

const mcpToolBudget = 12

func callMCPTool(t *testing.T, session *mcp.ClientSession, name string, args any) *mcp.CallToolResult {
	t.Helper()

	result, err := session.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      mcpToolName(name),
		Arguments: args,
	})
	if err != nil {
		t.Fatalf("CallTool %s failed: %v", name, err)
	}

	return result
}

func mcpToolName(name string) string {
	return strings.ReplaceAll(manifest.Id, ".", "_") + "__" + name
}

func decodeMCPResult[T any](t *testing.T, result *mcp.CallToolResult) T {
	t.Helper()

	if result.IsError {
		t.Fatalf("the tool refused: %s", mcpResultText(result))
	}

	var out T
	raw, err := json.Marshal(result.StructuredContent)
	if err != nil {
		t.Fatalf("could not re-encode the structured content: %v", err)
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("structured content is not a %T: %v (%s)", out, err, raw)
	}

	return out
}

func mcpResultText(result *mcp.CallToolResult) string {
	var parts []string
	for _, content := range result.Content {
		if text, ok := content.(*mcp.TextContent); ok {
			parts = append(parts, text.Text)
		}
	}

	return strings.Join(parts, "\n")
}

func TestMCPExposesEveryToolWithinTheBudget(t *testing.T) {
	names := toolNames(t, agentsSession(t, mcpPlugin(t)))

	if len(names) > mcpToolBudget {
		t.Errorf("%d tools registered, which is over the %d the design note budgets: %v",
			len(names), mcpToolBudget, names)
	}
}

func TestMCPDecorateMatchesTheTaggersOutput(t *testing.T) {
	p := mcpPlugin(t)
	session := agentsSession(t, p)

	for _, message := range []string{
		"ARCT 091630ZAUG26 at ICAO:PHIK",
		"MGRS:18S UJ 23478 06483",
		"nothing to see here",
		"`091630ZAUG26` in a code span",
	} {
		want := p.decorateText(bridgeclient.DecorateRequest{Message: message})

		got := decodeMCPResult[bridgeclient.DecorateResponse](t,
			callMCPTool(t, session, "decorate_text", DecorateTextArgs{Message: message}))

		if !reflect.DeepEqual(got, want) {
			t.Errorf("decorate_text(%q) = %+v, want %+v", message, got, want)
		}
	}
}

func TestMCPDecorateCarriesFitsPost(t *testing.T) {
	session := agentsSession(t, mcpPlugin(t))

	over := decodeMCPResult[bridgeclient.DecorateResponse](t,
		callMCPTool(t, session, "decorate_text", DecorateTextArgs{
			Message: strings.Repeat("é", safePostRunes+1),
		}))
	if over.FitsPost {
		t.Error("a message over the floor reported fits_post true, so an agent would post something the server refuses")
	}

	fits := decodeMCPResult[bridgeclient.DecorateResponse](t,
		callMCPTool(t, session, "decorate_text", DecorateTextArgs{Message: "ARCT 091630ZAUG26"}))
	if !fits.FitsPost {
		t.Error("a short message reported fits_post false")
	}
}

func TestMCPLinkIsTheLinkTheTaggerWrites(t *testing.T) {
	p := mcpPlugin(t)
	session := agentsSession(t, p)

	for _, c := range []struct {
		typ   string
		token string
	}{
		{dtg.Type, "091630ZAUG26"},
		{location.Type, "18S UJ 23478 06483"},
		{airport.Type, "PHIK"},
	} {
		want, refusal := p.buildLink(bridgeclient.LinkRequest{Type: c.typ, Token: c.token})
		if refusal != nil {
			t.Fatalf("buildLink refused %s %q: %s", c.typ, c.token, refusal.message)
		}

		got := decodeMCPResult[bridgeclient.LinkResponse](t,
			callMCPTool(t, session, "link_token", LinkTokenArgs{Type: c.typ, Token: c.token}))

		if !reflect.DeepEqual(got, want) {
			t.Errorf("link_token(%s, %q) = %+v, want %+v", c.typ, c.token, got, want)
		}
	}
}

func TestMCPLinkDeclinesWhatTheBridgeDeclines(t *testing.T) {
	session := agentsSession(t, mcpPlugin(t))

	for _, c := range []struct {
		typ   string
		token string
	}{
		{"cve", "CVE-2024-3094"},
		{dtg.Type, "091630J"},
		{airport.Type, "ZZZZ"},
		{location.Type, "34.05, -118.25x"},
	} {
		result := callMCPTool(t, session, "link_token", LinkTokenArgs{Type: c.typ, Token: c.token})

		if !result.IsError {
			t.Errorf("link_token(%s, %q) answered with a link rather than a refusal", c.typ, c.token)
			continue
		}
		if want := errcode.WithCode(errcode.MCPLinkDeclined, ""); !strings.Contains(mcpResultText(result), strings.TrimSpace(want)) {
			t.Errorf("link_token(%s, %q) refused with %q, which carries no %d",
				c.typ, c.token, mcpResultText(result), errcode.MCPLinkDeclined)
		}
	}
}

func TestMCPDecorateRefusesAMessageOverTheBridgeLimit(t *testing.T) {
	session := agentsSession(t, mcpPlugin(t))

	atLimit := callMCPTool(t, session, "decorate_text", DecorateTextArgs{Message: strings.Repeat("x", maxBridgeBody)})
	if atLimit.IsError {
		t.Fatalf("a message of exactly %d bytes was refused: %s", maxBridgeBody, mcpResultText(atLimit))
	}

	over := callMCPTool(t, session, "decorate_text", DecorateTextArgs{Message: strings.Repeat("x", maxBridgeBody+1)})
	if !over.IsError {
		t.Fatalf("a message of %d bytes was decorated rather than refused", maxBridgeBody+1)
	}
	if want := strings.TrimSpace(errcode.WithCode(errcode.MCPDecorateTooLong, "")); !strings.Contains(mcpResultText(over), want) {
		t.Fatalf("refused with %q, which carries no %d", mcpResultText(over), errcode.MCPDecorateTooLong)
	}
}

func TestMCPConvertMatchesTheConversionTheAPIServes(t *testing.T) {
	session := agentsSession(t, mcpPlugin(t))

	want, ok := location.Convert("mgrs", "18SUJ2347806483", "")
	if !ok {
		t.Fatal("location.Convert refused a coordinate the suite treats as valid")
	}

	got := decodeMCPResult[location.Conversion](t,
		callMCPTool(t, session, "convert_coordinate", ConvertCoordinateArgs{
			Format: "mgrs",
			Value:  "18SUJ2347806483",
		}))

	if !reflect.DeepEqual(got, want) {
		t.Errorf("convert_coordinate = %+v, want %+v", got, want)
	}
}

func TestMCPConvertDeclinesACoordinateThisPluginDidNotIssue(t *testing.T) {
	session := agentsSession(t, mcpPlugin(t))

	result := callMCPTool(t, session, "convert_coordinate", ConvertCoordinateArgs{
		Format: "mgrs",
		Value:  "not a coordinate",
	})

	if !result.IsError {
		t.Fatal("convert_coordinate answered a bad token with a conversion")
	}
}

func TestMCPAirfieldMatchesTheLookupTheAPIServes(t *testing.T) {
	session := agentsSession(t, mcpPlugin(t))

	want := describeAirport("PHIK")

	got := decodeMCPResult[airportResponse](t,
		callMCPTool(t, session, "lookup_airfield", LookupAirfieldArgs{Ident: "PHIK"}))

	if !reflect.DeepEqual(got, want) {
		t.Errorf("lookup_airfield = %+v, want %+v", got, want)
	}
	if !got.Found {
		t.Error("PHIK was not found, so the embedded database is not reaching the tool")
	}
}

func TestMCPAirfieldAnswersNotFoundRatherThanRefusingAWellShapedIdent(t *testing.T) {
	session := agentsSession(t, mcpPlugin(t))

	got := decodeMCPResult[airportResponse](t,
		callMCPTool(t, session, "lookup_airfield", LookupAirfieldArgs{Ident: "ZZZZ"}))

	if got.Found {
		t.Error("ZZZZ reported found")
	}
	if got.Ident != "ZZZZ" {
		t.Errorf("ident = %q, want ZZZZ echoed back", got.Ident)
	}
}

func TestMCPAirfieldDeclinesAnIdentThatIsNotTheShape(t *testing.T) {
	session := agentsSession(t, mcpPlugin(t))

	result := callMCPTool(t, session, "lookup_airfield", LookupAirfieldArgs{Ident: "not an ident"})

	if !result.IsError {
		t.Fatal("lookup_airfield echoed a value that is not an ident shape")
	}
}

func TestMCPToolsAreNotGovernedByTheFormatSwitches(t *testing.T) {
	p := mcpPlugin(t)
	withConfiguration(p, func(c *configuration) { c.EnableAirport = false })

	session := agentsSession(t, p)

	got := decodeMCPResult[airportResponse](t,
		callMCPTool(t, session, "lookup_airfield", LookupAirfieldArgs{Ident: "PHIK"}))

	if !got.Found {
		t.Error("lookup_airfield stopped resolving with EnableAirport off, unlike /api/v1/airport")
	}
}

func TestMCPLinkHonorsAFormatSwitch(t *testing.T) {
	p := mcpPlugin(t)
	withConfiguration(p, func(c *configuration) { c.EnableAirport = false })

	session := agentsSession(t, p)

	result := callMCPTool(t, session, "link_token", LinkTokenArgs{Type: airport.Type, Token: "PHIK"})

	if !result.IsError {
		t.Error("link_token wrote an airfield link with EnableAirport off, so MCP has a decoration rule of its own")
	}
}

func TestEveryMCPToolAnswersWithoutAReader(t *testing.T) {
	p := mcpPlugin(t)
	session := mcpSession(t, p, map[string]string{"Mattermost-Plugin-ID": agentsPluginID})

	args := map[string]any{
		"decorate_text":      DecorateTextArgs{Message: "ARCT 091630ZAUG26"},
		"link_token":         LinkTokenArgs{Type: airport.Type, Token: "PHIK"},
		"convert_coordinate": ConvertCoordinateArgs{Format: "mgrs", Value: "18SUJ2347806483"},
		"lookup_airfield":    LookupAirfieldArgs{Ident: "PHIK"},

		"decode_aviation_report": DecodeAviationReportArgs{Text: mcpMETAR},
		"decode_cot":             DecodeCotArgs{XML: cotExampleTarget},
		"summarize_geojson":      SummarizeGeoJSONArgs{Document: geoJSONExample},
		"describe_frequency":     DescribeFrequencyArgs{Frequency: "121.5"},
		"read_date_time":         ReadDateTimeArgs{Text: "141200ZSEP26"},
		"create_cot":             CreateCotArgs{Events: []CreateCotEvent{pointEvent("ALPHA", 21.3353, -157.9483)}},
		"create_geojson":         CreateGeoJSONArgs{Features: []CreateGeoJSONFeature{{Name: "Supply point", Kind: "point", Positions: []GeoJSONPosition{{Lat: 21.3353, Lon: -157.9483}}}}},
		"lookup_cyber_indicator": LookupCyberIndicatorArgs{Indicators: []string{"CVE-2021-44228"}},
	}

	names := toolNames(t, session)
	if len(names) != len(args) {
		t.Fatalf("the registry holds %d tools and this test knows arguments for %d; add the new one here: %v",
			len(names), len(args), names)
	}

	for _, full := range names {
		bare := strings.TrimPrefix(full, strings.ReplaceAll(manifest.Id, ".", "_")+"__")

		in, ok := args[bare]
		if !ok {
			t.Errorf("tool %q has no arguments in this test", bare)
			continue
		}

		result := callMCPTool(t, session, bare, in)
		if result.IsError {
			t.Errorf("tool %q refused a request that carried no user id: %s", bare, mcpResultText(result))
		}
	}
}

func TestMCPToolPanicIsRecoveredRatherThanTakingThePluginDown(t *testing.T) {
	p := mcpPlugin(t)

	handler := guardTool(p, "exploding", func(context.Context, *mcp.CallToolRequest, DecorateTextArgs) (*mcp.CallToolResult, bridgeclient.DecorateResponse, error) {
		panic("boom")
	})

	_, _, err := handler(context.Background(), nil, DecorateTextArgs{})
	if err == nil {
		t.Fatal("a panicking tool returned no error, so the panic escaped the handler")
	}

	api, ok := p.API.(*fakeAPI)
	if !ok {
		t.Fatal("the test plugin is not backed by fakeAPI")
	}
	var logged bool
	for _, msg := range api.errors {
		if strings.Contains(msg, "MCP tool panicked") {
			logged = true
		}
	}
	if !logged {
		t.Errorf("the recovered panic was not logged; errors were %v", api.errors)
	}
	if !strings.Contains(err.Error(), fmt.Sprintf("%s-%d", errcode.Prefix, errcode.MCPToolPanic)) {
		t.Errorf("the error %q carries no %d", err.Error(), errcode.MCPToolPanic)
	}
}
