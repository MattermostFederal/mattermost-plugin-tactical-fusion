package main

import (
	"strings"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/plugin"
)

const commandTrigger = "mission-context"

func getCommand() *model.Command {
	return &model.Command{
		Trigger:          commandTrigger,
		AutoComplete:     true,
		AutoCompleteDesc: "Mission Context commands",
		AutoCompleteHint: "[command]",
		DisplayName:      "Mission Context",
	}
}

func (p *Plugin) ExecuteCommand(_ *plugin.Context, args *model.CommandArgs) (*model.CommandResponse, *model.AppError) {
	fields := strings.Fields(args.Command)
	if len(fields) < 2 {
		return ephemeralResponse("Available subcommands: hello"), nil
	}

	switch fields[1] {
	case "hello":
		return ephemeralResponse("Hello from Mission Context!"), nil
	default:
		return ephemeralResponse("Unknown subcommand. Available: hello"), nil
	}
}

func ephemeralResponse(text string) *model.CommandResponse {
	return &model.CommandResponse{
		ResponseType: model.CommandResponseTypeEphemeral,
		Text:         text,
	}
}
