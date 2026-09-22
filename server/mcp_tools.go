package main

import (
	"context"
	"fmt"

	"github.com/mattermost/mattermost-plugin-agents/v2/external/pluginmcp"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/pkg/errors"

	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/bridgeclient"
	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/decorators/airport"
	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/decorators/location"
	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/errcode"
)

type DecorateTextArgs struct {
	Message       string `json:"message" jsonschema:"markdown text to run the tagger over; tokens inside code spans, fences, links and URLs are left as written"`
	ReferenceTime int64  `json:"reference_time,omitempty" jsonschema:"Unix milliseconds supplying the month and year for a short date-time group such as 091630Z; zero means now"`
}

type LinkTokenArgs struct {
	Type          string `json:"type" jsonschema:"the decorator type that reads the token: dtg, location, airport, avreport or frequency"`
	Token         string `json:"token" jsonschema:"the value alone with no field label: PHIK rather than ICAO:PHIK"`
	Label         string `json:"label,omitempty" jsonschema:"link text; empty means the token as written"`
	ReferenceTime int64  `json:"reference_time,omitempty" jsonschema:"Unix milliseconds supplying the month and year for a short date-time group; zero means now"`
}

type ConvertCoordinateArgs struct {
	Format string `json:"format" jsonschema:"the format of value, as a Tactical Fusion location link carries it: dd, latlon, usmtf, mgrs, utm, georef, gars or pluscode"`
	Value  string `json:"value" jsonschema:"the canonical coordinate token in that format"`
	Region string `json:"region,omitempty" jsonschema:"optional raw token the author wrote, when it differs from value"`
}

type LookupAirfieldArgs struct {
	Ident string `json:"ident" jsonschema:"an ICAO airfield identifier such as PHIK or EGLL"`
}

func (p *Plugin) registerMCPTools(server *pluginmcp.Server) {
	pluginmcp.AddTool(server, &mcp.Tool{
		Name:        "decorate_text",
		Description: "Rewrite recognized coordinates, date-time groups, airfield codes, aviation reports and radio frequencies in a message as Tactical Fusion links. Returns the message unchanged when it carries no recognized token.",
	}, guardTool(p, "decorate_text", p.decorateTextTool))

	pluginmcp.AddTool(server, &mcp.Tool{
		Name:        "link_token",
		Description: "Build one Tactical Fusion link for a token whose decorator type is already known. Use decorate_text instead when the type is not known or the token sits in prose.",
	}, guardTool(p, "link_token", p.linkTokenTool))

	pluginmcp.AddTool(server, &mcp.Tool{
		Name:        "convert_coordinate",
		Description: "Derive every other reading of a coordinate: MGRS, UTM, decimal degrees, DMS, DDM, USMTF, GEOREF, GARS, Plus Code, the country, and decimal latitude and longitude.",
	}, guardTool(p, "convert_coordinate", p.convertCoordinateTool))

	pluginmcp.AddTool(server, &mcp.Tool{
		Name:        "lookup_airfield",
		Description: "Look up an ICAO airfield by identifier and return its name, type, place, elevation, IATA code and position.",
	}, guardTool(p, "lookup_airfield", p.lookupAirfieldTool))
}

func (p *Plugin) decorateTextTool(_ context.Context, _ *mcp.CallToolRequest, in DecorateTextArgs) (*mcp.CallToolResult, bridgeclient.DecorateResponse, error) {
	return nil, p.decorateText(bridgeclient.DecorateRequest{
		Message:       in.Message,
		ReferenceTime: in.ReferenceTime,
	}), nil
}

func (p *Plugin) linkTokenTool(_ context.Context, _ *mcp.CallToolRequest, in LinkTokenArgs) (*mcp.CallToolResult, bridgeclient.LinkResponse, error) {
	resp, refusal := p.buildLink(bridgeclient.LinkRequest{
		Type:          in.Type,
		Token:         in.Token,
		Label:         in.Label,
		ReferenceTime: in.ReferenceTime,
	})
	if refusal != nil {
		return toolRefusal(errcode.MCPLinkDeclined, refusal.message), bridgeclient.LinkResponse{}, nil
	}

	return nil, resp, nil
}

func (p *Plugin) convertCoordinateTool(_ context.Context, _ *mcp.CallToolRequest, in ConvertCoordinateArgs) (*mcp.CallToolResult, location.Conversion, error) {
	conversion, ok := location.Convert(location.Format(in.Format), in.Value, in.Region)
	if !ok {
		return toolRefusal(errcode.MCPConvertInvalid,
			"That is not a coordinate this plugin issued."), location.Conversion{}, nil
	}

	return nil, conversion, nil
}

func (p *Plugin) lookupAirfieldTool(_ context.Context, _ *mcp.CallToolRequest, in LookupAirfieldArgs) (*mcp.CallToolResult, airportResponse, error) {
	if !airport.MatchesIdentShape(in.Ident) {
		return toolRefusal(errcode.MCPAirportInvalid,
			"That is not an airfield code this plugin issued."), airportResponse{}, nil
	}

	return nil, describeAirport(in.Ident), nil
}

func toolRefusal(code int, message string) *mcp.CallToolResult {
	return &mcp.CallToolResult{
		IsError: true,
		Content: []mcp.Content{&mcp.TextContent{Text: errcode.WithCode(code, message)}},
	}
}

func guardTool[In, Out any](p *Plugin, name string, handler mcp.ToolHandlerFor[In, Out]) mcp.ToolHandlerFor[In, Out] {
	return func(ctx context.Context, req *mcp.CallToolRequest, in In) (result *mcp.CallToolResult, out Out, err error) {
		api := p.API
		defer func() {
			if recovered := recover(); recovered != nil {
				api.LogError("MCP tool panicked",
					"error_code", errcode.MCPToolPanic,
					"tool", name,
					"panic", fmt.Sprint(recovered))

				var zero Out
				result, out, err = nil, zero, errors.New(errcode.WithCode(errcode.MCPToolPanic, "Internal error."))
			}
		}()

		return handler(ctx, req, in)
	}
}
