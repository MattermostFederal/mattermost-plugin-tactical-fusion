package main

import (
	"strings"
	"time"
	"unicode/utf8"

	"github.com/mattermost/mattermost/server/public/model"

	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/decorators"
	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/decorators/dtg"
	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/errcode"
)

// exampleRow is one line of the examples post: a label naming the grammar, the
// text an author would type, and a short reason where the row is showing more
// than itself.
//
// The label is what separates this from example-details. There, the rows are
// grouped under a heading and the reader is working through them; here there is
// one row per grammar and the label is how somebody scrolling past learns which
// is which without a heading for every line.
type exampleRow struct {
	label string
	text  string
	note  string
}

// exampleLiveRows are built from the moment the command runs.
//
// Both offsets earn their place. Five minutes is inside the flash threshold, so
// the countdown opens already warning, which is the state most worth seeing and
// the one a fixed date can never show. Four hours back counts up rather than
// down, which is the half of the behavior readers do not expect until they meet
// it.
var exampleLiveRows = []struct {
	label  string
	offset time.Duration
	note   string
}{
	{"Date/Time", 5 * time.Minute, "5 minutes out, so the countdown opens flagged as imminent"},
	{"Date/Time", -4 * time.Hour, "4 hours ago, so it counts up instead of down"},
}

// exampleFixedRows are the ordinary shape of each grammar, and deliberately
// nothing exotic.
//
// Every near miss, boundary and protected span belongs in example-details,
// which can be as long as it needs to be. This one is a single post that
// everybody in the channel reads, so it answers one question only: what does
// this plugin do to a message?
//
// The positions are in Hawaii and Guam, which is where the detail map packages
// this plugin bundles cover. A reader clicking through meets streets and
// coastline rather than the global tier's outline, which is the difference
// between a map that looks finished and one that looks broken.
var exampleFixedRows = []exampleRow{
	{label: "Date/Time", text: "091630ZAUG26", note: "the military date-time group"},
	{label: "RFC 3339", text: "2026-08-09T16:30:00Z"},
	{label: "Lat/lon", text: "21.3353, -157.9483", note: "Hickam Air Force Base"},
	{label: "Lat/lon", text: "21.3353 N, 157.9483 W", note: "hemisphere letters instead of signs"},
	{label: "DMS", text: "21°20'07.1\"N 157°56'53.9\"W"},
	{label: "DDM", text: "21°20.118'N 157°56.898'W"},
	{label: "USMTF", text: "2120N15757W", note: "degrees and whole minutes"},
	{label: "MGRS", text: "4Q FJ 0906 5962"},
	{label: "UTM", text: "4Q 609060E 2359620N", note: "off by default; an admin turns it on"},
	{label: "GEOREF", text: "GEOREF:XGKP55803503", note: "the label is required, longitude comes first, and this one is on Guam"},
	{label: "GARS", text: "GARS:045KG14", note: "the label is required"},
	{label: "Plus Code", text: "73H483P2+4MG", note: "matched without a label"},
	{label: "Airfield", text: "ICAO:PHNL", note: "the label is required, in upper case"},
}

// examplesResponse posts a short live demonstration to the channel.
//
// One post, everything in it decorated, and it goes where other people can see
// it, so somebody introducing the plugin to a team runs it once and lets
// everybody click through the links.
//
// Being a real post is what shapes the failure modes. A row whose format is
// switched off is DROPPED rather than shown as a bare token, because a token
// with no link beside rows that became links is a permanent post advertising
// that the plugin does nothing. With nothing left it refuses ephemerally rather
// than posting an empty demonstration, and it measures itself against the post
// limit rather than letting the server reject it with an error nobody can act
// on.
//
// Decorated here rather than by the message hook, because the output is full of
// code and links, which is exactly what the tagger leaves alone. That also makes
// this correct whether or not an in-channel command response reaches
// MessageWillBePosted, since Decorate is idempotent and a link it already wrote
// is a protected span.
func (p *Plugin) examplesResponse() *model.CommandResponse {
	if p.decorators == nil {
		return ephemeralResponse(errcode.WithCode(errcode.CommandExamplesNotReady,
			"Decorators are not registered yet. Try again once the plugin has finished activating."))
	}

	tagger := &decorators.Tagger{Registry: p.decorators, URLPrefix: p.decorateURLPrefix()}
	ref := time.Now().UTC()

	lines := examplePostLines(tagger, ref)
	if len(lines) == 0 {
		return ephemeralResponse(errcode.WithCode(errcode.CommandExamplesNothingEnabled,
			"Nothing would be decorated, so there is nothing to show. "+
				"Every format is switched off in the System Console, under Plugins > Tactical Fusion."))
	}

	var b strings.Builder
	b.WriteString("#### Tactical Fusion\n\n")
	b.WriteString("What you type, and what gets stored. Click any link to open it in the sidebar.\n\n")

	for _, line := range lines {
		b.WriteString(line)
	}

	// Appended rather than packed into the rows above, because those are built
	// by running the tagger and keeping what it changed, and a Cursor on Target
	// event is not a token the tagger has ever seen. It is recognized by the
	// post hook instead, which is also why this row cannot show its own output.
	if p.cotEnabled() {
		b.WriteString(cotExampleLine())
	}

	b.WriteString("\nRun `/" + commandTrigger + " example-details` for every recognized format and " +
		"every near miss that is deliberately declined, or `/" + commandTrigger +
		" check <text>` to try your own.\n")

	// Measured against the floor rather than the usual limit, because unlike
	// example-details this command has nowhere to fall back to: it is one post
	// by definition, so the only choices are "fits everywhere" and "might be
	// refused". Every link carries the install's subpath, which is the one
	// thing that makes this reachable.
	if utf8.RuneCountInString(b.String()) > safePostRunes {
		return ephemeralResponse(errcode.WithCode(errcode.CommandExamplesTooLong,
			"The examples do not fit in one post on this server. "+
				"Run `/"+commandTrigger+" example-details` instead, which spreads itself over several posts."))
	}

	return &model.CommandResponse{
		ResponseType: model.CommandResponseTypeInChannel,
		Text:         b.String(),
	}
}

// examplePostLines renders the rows that decorate, and drops the rest.
func examplePostLines(tagger *decorators.Tagger, ref time.Time) []string {
	rows := make([]exampleRow, 0, len(exampleLiveRows)+len(exampleFixedRows))

	// Live first: they are the only rows whose panel opens on a countdown that
	// is actually moving, so they are what a reader should click.
	for _, live := range exampleLiveRows {
		rows = append(rows, exampleRow{
			label: live.label,
			text:  dtg.FormatZulu(ref.Add(live.offset)),
			note:  live.note,
		})
	}
	rows = append(rows, exampleFixedRows...)

	lines := make([]string, 0, len(rows))

	for _, row := range rows {
		decorated := tagger.Decorate(row.text, ref)
		if decorated == row.text {
			// The format is switched off, or this build no longer recognizes
			// the shape. Either way the row has nothing to show.
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
