package main

import (
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/mattermost/mattermost/server/public/model"

	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/avreport"
	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/decorators"
	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/decorators/airport"
	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/decorators/cyber"
	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/decorators/dtg"
	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/decorators/frequency"
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

var exampleSetOrder = []string{dtg.Type, location.Type, airport.Type, avreport.Type, frequency.Type, cyber.Type}

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
			{label: "IATA", text: "IATA:HNL", note: "the three-letter code behind its own label"},
		},
	},

	avreport.Type: {
		decorator: avreport.Type,
		name:      "Aviation reports",
		rows: []exampleRow{
			{label: "METAR", text: "METAR PHNL 221651Z 07012G18KT 10SM FEW025 SCT045 27/19 A3010", note: "hover for the plain-language summary; posted on its own it becomes a table"},
			{label: "TAF", text: "TAF PGUA 221720Z 2218/2324 07012KT P6SM SCT025", note: "one line; a multi-line forecast posted on its own gets a table too, and a fenced one a card"},
			{label: "NOTAM", text: "!HNL 09/123 HNL RWY 08L/26R CLSD 2609221200-2609232359", note: "the FAA domestic form"},
		},
	},

	frequency.Type: {
		decorator: frequency.Type,
		name:      "Frequencies",
		rows: []exampleRow{
			{label: "Guard", text: "FREQ:121.5", note: "the label is required; hover for the band and the allocation"},
			{label: "Tower", text: "FREQ:118.3", note: "VHF air band, no known allocation"},
			{label: "UHF guard", text: "FREQ:243.0", note: "the military emergency frequency"},
		},
	},

	cyber.Type: {
		decorator: cyber.Type,
		name:      "Cyber context",
		rows: []exampleRow{
			{label: "Vulnerability", text: "CVE-2025-55182", note: "React2Shell, critical and known exploited; recognized by shape, so it links with no dataset installed"},
			{label: "Weakness", text: "CWE-79", note: "only identifiers the built-in catalog holds"},
			{label: "Technique", text: "T1059.001", note: "a sub-technique, which carries its parent and its tactics"},
			{label: "Tactic", text: "TA0002"},
			{label: "IP address", text: "203.0.113.7", note: "a documentation address, so no dataset describes it"},
			{label: "File hash", text: "44d88612fea8a8f36de82e1278abb02f", note: "the EICAR test file, an MD5"},
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

	if len(messages) == 0 && !p.cotEnabled() && !p.geoJSONEnabled() {
		return ephemeralResponse(errcode.WithCode(errcode.CommandExamplesNothingEnabled,
			"Nothing would be decorated, so there is nothing to show. "+
				"Every format is switched off in the System Console, under Plugins > Tactical Fusion."))
	}

	// EVERY message, including the ones the format commands build for
	// themselves. They used to be written after this loop had already measured,
	// so the invariant ("measures every message against the same floor before it
	// writes any of them, refusing the whole run rather than posting some of
	// it") was not enforced for exactly the two that carry the largest bodies.
	for _, message := range append(append([]string(nil), messages...), p.formatExampleMessages()...) {
		if utf8.RuneCountInString(message) > safePostRunes {
			return ephemeralResponse(errcode.WithCode(errcode.CommandExamplesTooLong,
				"The examples do not fit in a post on this server. "+
					"The built-in help carries the same examples, under Formats."))
		}
	}

	return p.postExamples(args, messages)
}

// formatExampleMessages is every message the format commands will write, so the
// size gate above can measure them alongside the decorator sets.
func (p *Plugin) formatExampleMessages() []string {
	messages := append(append(p.cotExampleMessages(), p.geoJSONExampleMessages()...), p.tfrExampleMessages()...)
	return append(messages, p.noteExampleMessages()...)
}

func (p *Plugin) postExamples(args *model.CommandArgs, messages []string) *model.CommandResponse {
	failed, total := 0, len(messages)+p.cotExampleCount()+p.geoJSONExampleCount()+p.tfrExampleCount()+len(noteExamples)

	for _, message := range messages {
		if _, appErr := p.API.CreatePost(examplePost(args, message)); appErr != nil {
			failed++
			p.API.LogError("tactical-fusion: could not post an examples message",
				"error_code", errcode.CommandExamplesPostFailed, "error", appErr.Error())
		}
	}

	failed += p.postCotExamples(args)
	failed += p.postGeoJSONExamples(args)
	failed += p.postTFRExample(args)
	failed += p.postNoteExamples(args)

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
