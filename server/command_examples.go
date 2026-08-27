package main

import (
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/mattermost/mattermost/server/public/model"

	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/decorators"
	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/decorators/airport"
	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/decorators/dtg"
	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/decorators/location"
	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/errcode"
)

type exampleRow struct {
	label string
	text  string
	note  string
}

type exampleLiveRow struct {
	label  string
	offset time.Duration
	note   string
}

type exampleSet struct {
	decorator string
	name      string
	live      []exampleLiveRow
	rows      []exampleRow
}

var exampleSetOrder = []string{dtg.Type, location.Type, airport.Type}

var exampleSets = map[string]exampleSet{
	dtg.Type: {
		decorator: dtg.Type,
		name:      "Date-time groups",
		live: []exampleLiveRow{
			{"Live", 5 * time.Minute, "5 minutes out, so the countdown opens flagged as imminent"},
			{"Live", -4 * time.Hour, "4 hours ago, so it counts up instead of down"},
		},
		rows: []exampleRow{
			{label: "Military", text: "091630ZAUG26", note: "the ordinary date-time group"},
			{label: "Short form", text: "091630Z", note: "month and year come from the date the message was posted"},
			{label: "Zone letter", text: "091630RAUG26", note: "any letter except I and J"},
			{label: "RFC 3339", text: "2026-08-09T16:30:00Z"},
			{label: "Moniker", text: "DTG: 091630ZAUG26", note: "the label is consumed into the link"},
		},
	},

	location.Type: {
		decorator: location.Type,
		name:      "Coordinates",
		rows: []exampleRow{
			{label: "Lat/lon", text: "21.3353, -157.9483", note: "Hickam Air Force Base"},
			{label: "Lat/lon", text: "21.3353 N, 157.9483 W", note: "hemisphere letters instead of signs"},
			{label: "DMS", text: "21°21'53\"N 157°57'00\"W"},
			{label: "DDM", text: "21°20.118'N 157°56.898'W"},
			{label: "USMTF", text: "2120N15757W", note: "degrees and whole minutes"},
			{label: "MGRS", text: "4Q FJ 0906 5962"},
			{label: "UTM", text: "4Q 609060 2359620", note: "off by default; an admin turns it on"},
			{label: "GEOREF", text: "GEOREF:XGKP55803503", note: "the label is required, longitude comes first, and this one is on Guam"},
			{label: "GARS", text: "GARS:045KG14", note: "the label is required"},
			{label: "Plus Code", text: "73H483P2+4MG", note: "matched without a label"},
		},
	},

	airport.Type: {
		decorator: airport.Type,
		name:      "Airfields",
		rows: []exampleRow{
			{label: "ICAO", text: "ICAO:PHNL", note: "the label is required, in upper case"},
			{label: "Location", text: "LOC:PGUM"},
			{label: "Departure", text: "DEPLOC:PHTO"},
		},
	},
}

const examplesHeader = "What you type, and what gets stored. Click any link to open it in the sidebar.\n\n"

const examplesFooter = "\nEvery format, every boundary and every near miss that is deliberately " +
	"declined is in the built-in help. Run `/" + commandTrigger + " check <text>` to try your own.\n"

func (p *Plugin) examplesResponse(args *model.CommandArgs) *model.CommandResponse {
	if p.decorators == nil {
		return ephemeralResponse(errcode.WithCode(errcode.CommandExamplesNotReady,
			"Decorators are not registered yet. Try again once the plugin has finished activating."))
	}

	tagger := &decorators.Tagger{Registry: p.decorators, URLPrefix: p.decorateURLPrefix()}
	messages := examplePosts(tagger, time.Now().UTC())

	if len(messages) == 0 && !p.cotEnabled() {
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

	return p.postExamples(args, messages)
}

func (p *Plugin) postExamples(args *model.CommandArgs, messages []string) *model.CommandResponse {
	failed, total := 0, len(messages)+p.cotExampleCount()

	for _, message := range messages {
		if _, appErr := p.API.CreatePost(examplePost(args, message)); appErr != nil {
			failed++
			p.API.LogError("tactical-fusion: could not post an examples message",
				"error_code", errcode.CommandExamplesPostFailed, "error", appErr.Error())
		}
	}

	failed += p.postCotExamples(args)

	if failed == total {
		return ephemeralResponse(errcode.WithCode(errcode.CommandExamplesPostFailed,
			"Could not post the examples to this channel."))
	}
	if failed > 0 {
		return ephemeralResponse(errcode.WithCode(errcode.CommandExamplesPostFailed,
			plural(failed, "message")+" of "+strconv.Itoa(total)+" could not be posted, so the examples are incomplete."))
	}

	return &model.CommandResponse{}
}

func examplePost(args *model.CommandArgs, message string) *model.Post {
	return &model.Post{
		UserId:    args.UserId,
		ChannelId: args.ChannelId,
		Message:   message,
	}
}

func examplePosts(tagger *decorators.Tagger, ref time.Time) []string {
	messages := make([]string, 0, len(exampleSetOrder))

	for _, key := range exampleSetOrder {
		set := exampleSets[key]

		lines := exampleSetLines(tagger, ref, set)
		if len(lines) == 0 {
			continue
		}

		var b strings.Builder
		b.WriteString("#### " + set.name + "\n\n")
		b.WriteString(examplesHeader)
		for _, line := range lines {
			b.WriteString(line)
		}
		b.WriteString(examplesFooter)

		messages = append(messages, b.String())
	}

	return messages
}

func exampleSetLines(tagger *decorators.Tagger, ref time.Time, set exampleSet) []string {
	rows := make([]exampleRow, 0, len(set.live)+len(set.rows))

	for _, live := range set.live {
		rows = append(rows, exampleRow{
			label: live.label,
			text:  dtg.FormatZulu(ref.Add(live.offset)),
			note:  live.note,
		})
	}
	rows = append(rows, set.rows...)

	lines := make([]string, 0, len(rows))

	for _, row := range rows {
		decorated := tagger.Decorate(row.text, ref)
		if decorated == row.text {
			continue
		}

		var b strings.Builder
		b.WriteString("- **" + row.label + ":** " + inlineCode(row.text) + " → " + decorated)
		if row.note != "" {
			b.WriteString(" - " + row.note)
		}
		b.WriteString("\n")

		lines = append(lines, b.String())
	}

	return lines
}

func inlineCode(text string) string {
	longest, run := 0, 0
	for i := range len(text) {
		if text[i] != '`' {
			run = 0
			continue
		}
		run++
		longest = max(longest, run)
	}

	delimiter := strings.Repeat("`", longest+1)

	if strings.HasPrefix(text, "`") || strings.HasSuffix(text, "`") {
		return delimiter + " " + text + " " + delimiter
	}
	return delimiter + text + delimiter
}

func plural(n int, noun string) string {
	if n == 1 {
		return "1 " + noun
	}
	return strconv.Itoa(n) + " " + noun + "s"
}
