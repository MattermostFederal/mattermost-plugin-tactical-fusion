package main

import (
	"strings"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/plugin"
)

const commandTrigger = "mission-context"

const subcommandList = "examples"

func getCommand() *model.Command {
	autocomplete := model.NewAutocompleteData(commandTrigger, "[command]", "Mission Context commands")
	autocomplete.AddCommand(model.NewAutocompleteData("examples", "", "Show what the decorators do, with live examples"))

	return &model.Command{
		Trigger:          commandTrigger,
		AutoComplete:     true,
		AutoCompleteDesc: "Mission Context commands",
		AutoCompleteHint: "[command]",
		DisplayName:      "Mission Context",
		AutocompleteData: autocomplete,
	}
}

func (p *Plugin) ExecuteCommand(_ *plugin.Context, args *model.CommandArgs) (*model.CommandResponse, *model.AppError) {
	fields := strings.Fields(args.Command)
	if len(fields) < 2 {
		return ephemeralResponse("Available subcommands: " + subcommandList), nil
	}

	switch fields[1] {
	case "examples":
		return p.examplesResponse(), nil
	default:
		return ephemeralResponse("Unknown subcommand. Available: " + subcommandList), nil
	}
}

func ephemeralResponse(text string) *model.CommandResponse {
	return &model.CommandResponse{
		ResponseType: model.CommandResponseTypeEphemeral,
		Text:         text,
	}
}
