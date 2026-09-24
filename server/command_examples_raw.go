package main

import (
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/mattermost/mattermost/server/public/model"

	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/decorators"
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
		if texts := rawExampleSetTexts(tagger, ref, set); len(texts) > 0 {
			messages = append(messages, rawExampleBlocks(set.name, texts))
		}
	}

	for _, message := range p.tfrExampleMessages() {
		messages = append(messages, rawExampleBlocks("Temporary flight restriction", []string{message}))
	}
	if p.cotEnabled() {
		for _, example := range cotExampleOrder {
			if example.file == "" {
				messages = append(messages, rawExampleBlocks("Cursor on Target", []string{cotFenced(example.source, cotFenceInfo)}))
			}
		}
	}
	if p.geoJSONEnabled() {
		messages = append(messages, rawExampleBlocks("GeoJSON", []string{fenced(geoJSONFenceInfo, geoJSONExample)}))
	}

	return messages
}

func (p *Plugin) examplesCommand(args *model.CommandArgs, option string) *model.CommandResponse {
	if option != "" && option != rawExamplesOption {
		return ephemeralResponse(errcode.WithCode(errcode.CommandExamplesUnknownOption,
			"Unknown option. Run examples on its own to post them, or examples "+rawExamplesOption+" to post them as text to copy."))
	}
	if refusal := p.refuseUnlessCanPost(args); refusal != nil {
		return refusal
	}
	if option == rawExamplesOption {
		return p.rawExamplesResponse(args)
	}
	return p.examplesResponse(args)
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

	failed := 0
	for _, message := range messages {
		if _, appErr := p.API.CreatePost(examplePost(args, message)); appErr != nil {
			failed++
			p.API.LogError("tactical-fusion: could not post a raw examples message",
				"error_code", errcode.CommandExamplesPostFailed, "error", appErr.Error())
		}
	}

	switch {
	case failed == len(messages):
		return ephemeralResponse(errcode.WithCode(errcode.CommandExamplesPostFailed,
			"Could not post the examples to this channel."))
	case failed > 0:
		return ephemeralResponse(errcode.WithCode(errcode.CommandExamplesPostFailed,
			plural(failed, "message")+" of "+strconv.Itoa(len(messages))+" could not be posted, so the examples are incomplete."))
	}
	return &model.CommandResponse{}
}
