package main

import (
	"strings"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/plugin"

	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/errcode"
)

const commandTrigger = "tactical-fusion"

const subcommandList = "examples"

func getCommand() *model.Command {
	autocomplete := model.NewAutocompleteData(commandTrigger, "[command]", "Tactical Fusion commands")
	autocomplete.AddCommand(model.NewAutocompleteData("examples", "", "Show what the decorators do, with live examples"))

	return &model.Command{
		Trigger:          commandTrigger,
		AutoComplete:     true,
		AutoCompleteDesc: "Tactical Fusion commands",
		AutoCompleteHint: "[command]",
		DisplayName:      "Tactical Fusion",
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
		return ephemeralResponse(errcode.WithCode(errcode.CommandUnknownSubcommand,
			"Unknown subcommand. Available: "+subcommandList)), nil
	}
}

func ephemeralResponse(text string) *model.CommandResponse {
	return &model.CommandResponse{
		ResponseType: model.CommandResponseTypeEphemeral,
		Text:         text,
	}
}
