package main

import (
	"strings"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/plugin"

	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/errcode"
)

const commandTrigger = "tactical-fusion"

const subcommandList = "examples, check, note"

func getCommand() *model.Command {
	autocomplete := model.NewAutocompleteData(commandTrigger, "[command]", "Tactical Fusion commands")
	autocomplete.AddCommand(model.NewAutocompleteData("examples", "["+rawExamplesOption+" | "+mcpExamplesOption+"]", "Post a demonstration to this channel, one message per format, for everybody to see; with "+rawExamplesOption+", post the example text as code blocks to copy and paste; with "+mcpExamplesOption+", post a question for the "+mcpExamplesAgent+" agent for each format"))
	autocomplete.AddCommand(model.NewAutocompleteData("check", "[text]", "Show what would be decorated in some text, and what would not"))
	autocomplete.AddCommand(model.NewAutocompleteData("note", "[label] | [markdown]", "Post a link whose hover card renders your markdown"))

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
		return p.examplesCommand(args, strings.Join(fields[2:], " ")), nil
	case "check":
		return p.checkResponse(argumentText(args.Command, fields[1])), nil
	case "note":
		if refusal := p.refuseUnlessCanPost(args); refusal != nil {
			return refusal, nil
		}
		return p.noteResponse(args, argumentText(args.Command, fields[1])), nil
	default:
		return ephemeralResponse(errcode.WithCode(errcode.CommandUnknownSubcommand,
			"Unknown subcommand. Available: "+subcommandList)), nil
	}
}

// argumentText is everything after the subcommand, with its spacing intact.
//
// Recovered by finding the subcommand rather than by trimming a fixed prefix,
// because dispatch splits on Fields and so tolerates repeated spaces that a
// prefix trim would not. Getting that wrong made "/tactical-fusion  check X"
// echo the whole command line back as the text under test.
func argumentText(command, subcommand string) string {
	_, rest, found := strings.Cut(command, subcommand)
	if !found {
		return ""
	}
	return strings.TrimSpace(rest)
}

func (p *Plugin) refuseUnlessCanPost(args *model.CommandArgs) *model.CommandResponse {
	if p.API.HasPermissionToChannel(args.UserId, args.ChannelId, model.PermissionCreatePost) {
		return nil
	}
	return ephemeralResponse(errcode.WithCode(errcode.CommandPostNotPermitted,
		"You cannot post in this channel, so this command cannot post for you."))
}

func ephemeralResponse(text string) *model.CommandResponse {
	return &model.CommandResponse{
		ResponseType: model.CommandResponseTypeEphemeral,
		Text:         text,
	}
}
