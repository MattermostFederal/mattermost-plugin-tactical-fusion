package main

import (
	"strings"
	"time"
	"unicode/utf8"

	"github.com/mattermost/mattermost/server/public/model"

	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/decorators"
	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/decorators/dtg"
	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/decorators/location"
	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/errcode"
)

const (
	rawExamplesOption = "--raw"
	minimumFence      = 3
)

func outerFenced(body string) string {
	longest, run := 0, 0
	for i := range len(body) {
		if body[i] != '`' {
			run = 0
			continue
		}
		run++
		longest = max(longest, run)
	}
	fence := strings.Repeat("`", max(minimumFence, longest+1))
	return fence + "\n" + strings.TrimRight(body, "\n") + "\n" + fence
}

var rawExampleSetsOnePerBlock = map[string]bool{dtg.Type: true, location.Type: true}

func rawExampleBlock(heading, body string) string {
	return "#### " + heading + "\n\n" + outerFenced(body) + "\n"
}

func rawExampleBlocks(heading string, bodies []string) string {
	blocks := make([]string, 0, len(bodies))
	for _, body := range bodies {
		blocks = append(blocks, outerFenced(body))
	}
	return "#### " + heading + "\n\n" + strings.Join(blocks, "\n\n") + "\n"
}

func rawExampleSetTexts(tagger *decorators.Tagger, ref time.Time, set exampleSet) []string {
	texts := make([]string, 0, len(set.live)+len(set.rows))
	for _, live := range set.live {
		texts = append(texts, exampleLiveText(ref, live))
	}
	for _, row := range set.rows {
		texts = append(texts, row.text)
	}

	decorating := texts[:0]
	for _, text := range texts {
		if tagger.Decorate(text, ref) != text {
			decorating = append(decorating, text)
		}
	}
	return decorating
}

func (p *Plugin) rawExampleMessages(ref time.Time) []string {
	tagger := &decorators.Tagger{Registry: p.decorators, URLPrefix: p.decorateURLPrefix()}

	var messages []string
	for _, key := range exampleSetOrder {
		set := exampleSets[key]
		texts := rawExampleSetTexts(tagger, ref, set)
		switch {
		case len(texts) == 0:
		case rawExampleSetsOnePerBlock[key]:
			messages = append(messages, rawExampleBlocks(set.name, texts))
		default:
			messages = append(messages, rawExampleBlock(set.name, strings.Join(texts, "\n")))
		}
	}

	for _, message := range p.tfrExampleMessages() {
		messages = append(messages, rawExampleBlock("Temporary flight restriction", message))
	}
	if p.cotEnabled() {
		for _, example := range cotExampleOrder {
			if example.file == "" {
				messages = append(messages, rawExampleBlock("Cursor on Target", cotFenced(example.source, cotFenceInfo)))
			}
		}
	}
	if p.geoJSONEnabled() {
		messages = append(messages, rawExampleBlock("GeoJSON", fenced(geoJSONFenceInfo, geoJSONExample)))
	}

	return messages
}

func (p *Plugin) examplesCommand(args *model.CommandArgs, option string) *model.CommandResponse {
	switch option {
	case rawExamplesOption:
		return p.rawExamplesResponse(args)
	case "":
		if refusal := p.refuseUnlessCanPost(args); refusal != nil {
			return refusal
		}
		return p.examplesResponse(args)
	}
	return ephemeralResponse(errcode.WithCode(errcode.CommandExamplesUnknownOption,
		"Unknown option. Run examples on its own to post them, or examples "+rawExamplesOption+" to see them as text to copy."))
}

func (p *Plugin) rawExamplesResponse(args *model.CommandArgs) *model.CommandResponse {
	if p.decorators == nil {
		return ephemeralResponse(errcode.WithCode(errcode.CommandExamplesNotReady,
			"Decorators are not registered yet. Try again once the plugin has finished activating."))
	}

	messages := p.rawExampleMessages(time.Now().UTC())
	if len(messages) == 0 {
		return ephemeralResponse(errcode.WithCode(errcode.CommandExamplesNothingEnabled,
			"Nothing would be decorated, so there is nothing to show. "+
				"Every format is switched off in the System Console, under Plugins > Tactical Fusion."))
	}
	for _, message := range messages {
		if utf8.RuneCountInString(message) > safePostRunes {
			return ephemeralResponse(errcode.WithCode(errcode.CommandExamplesTooLong,
				"The examples do not fit in a post on this server. "+
					"The built-in help carries the same examples, under Formats."))
		}
	}

	for _, message := range messages {
		p.API.SendEphemeralPost(args.UserId, &model.Post{
			ChannelId: args.ChannelId,
			RootId:    args.RootId,
			Message:   message,
		})
	}
	return &model.CommandResponse{}
}
