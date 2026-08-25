package main

import (
	"fmt"
	"strings"

	"github.com/mattermost/mattermost/server/public/model"

	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/cot"
	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/errcode"
)

// The event both commands demonstrate.
//
// A hostile contact rather than a friendly position report, because that is
// what somebody pastes into a channel. A unit's own position goes over the
// network to the people who need it; a target is the thing worth showing other
// people and arguing about, so it is the card a reader should meet first.
//
// Every field it carries is there because the card has a row for it: a callsign,
// a stated accuracy, a source, remarks, and a link naming who reported it. Every
// near miss and refusal belongs in the help pages, which can be as long as they
// need to be.
//
// It carries no `__group`. That element is the SENDER's team, and ATAK puts it on a
// self position report rather than on a placed marker. On a hostile contact it
// would render as "Team: Cyan" against the target, which reads as the target
// being on that team.
//
// `how` is `h-e`, estimated, and the accuracy is metres rather than the
// sub-metre of a GPS fix, because a contact somebody plotted by eye is not known
// to the precision of a device reporting itself.
//
// It sits on Hickam Air Force Base, inside the Hawaii detail map this plugin
// bundles, so the card a reader meets draws streets and coastline rather than
// the global tier's outline.
//
// On the FIELD rather than in the harbour beside it, and that is the whole
// reason for this coordinate rather than a rounder one nearby. Pearl Harbor is
// the obvious name to reach for and its middle is open water, which put a
// ground unit's crosshair in East Loch on every surface that draws one. The
// value is the airfield reference point PHIK carries in this plugin's own
// airfield database, so the pin and `ICAO:PHIK` cannot come to disagree.
const cotExampleEvent = `<event version="2.0" uid="TGT-9F2A1C7B" type="a-h-G-U-C-A" how="h-e" ` +
	`time="2026-08-09T16:30:00Z" start="2026-08-09T16:30:00Z" stale="2026-08-09T16:32:00Z">` +
	`<point lat="21.335300" lon="-157.948300" hae="4.0" ce="45.0" le="60.0"/>` +
	`<detail><contact callsign="TGT01"/>` +
	`<link uid="ANDROID-88" relation="p-p" parent_callsign="ALPHA"/>` +
	`<remarks>two vehicles, stationary</remarks></detail></event>`

// cotExampleLine is the row the one-post examples command shows.
//
// The event is INLINE CODE, and that is not decoration. This command writes a
// real post full of markdown, and an unprotected event anywhere in it would be
// recognized: the post would be stamped, the card would own the body, and every
// other row would render as the plain text of its own markup. A code span is a
// protected range, so the demonstration cannot eat the demonstration.
//
// It costs the row its live card, which is why it says where to get one.
func cotExampleLine() string {
	reading := "a card with the event's position on a map"
	if label := cotExampleTypeLabel(); label != "" {
		reading = "a card reading " + label + ", with the position on a map"
	}

	return "- **Cursor on Target:** " + inlineCode(cotExampleShort()) +
		" → " + reading +
		" - post one on its own, or run `/" + commandTrigger + " example-details`, to see it\n"
}

// cotExampleTypeLabel is what the card calls the example's type, read from the
// type table rather than written out here.
//
// The row's whole point is that `a-h-G-U-C-A` becomes readable English, so naming
// the reading in prose beside a token that produces a different one would be the
// one thing this row must not do. Empty if anything fails, which drops the row
// back to describing the card rather than quoting it.
func cotExampleTypeLabel() string {
	events, err := cot.Parse([]byte(cotExampleEvent))
	if err != nil || len(events) != 1 {
		return ""
	}

	rendered, ok := cot.Props(events, cot.Source{})["events"].([]any)
	if !ok || len(rendered) != 1 {
		return ""
	}

	row, ok := rendered[0].(map[string]any)
	if !ok {
		return ""
	}

	label, _ := row["type_label"].(string)
	return label
}

// cotExampleShort is the event with its detail dropped, which is as much as a
// single row can carry without becoming the post.
func cotExampleShort() string {
	if at := strings.Index(cotExampleEvent, "<detail>"); at > 0 {
		return cotExampleEvent[:at] + "</event>"
	}
	return cotExampleEvent
}

// postCotExample writes one post that IS an event, so the reader meets a real
// card rather than a description of one.
//
// Its own post, because a card owns the whole body: an event sharing a post with
// the other examples would render them as plain text. That is the same rule the
// examples row above works around, met from the other side.
//
// Stamped here rather than left to the hook. Whether a post this plugin creates
// through the API reaches its own MessageWillBePosted is recorded as unverified
// in docs/design/unverified.md, and a demonstration that renders as raw XML on
// half of installs is worse than none. Stamping twice is harmless: cotStamp
// strips a type it did not write and reads the message again, so the hook firing
// as well lands on the same answer.
func (p *Plugin) postCotExample(args *model.CommandArgs) bool {
	if !p.cotEnabled() {
		return true
	}

	post := &model.Post{
		UserId:    args.UserId,
		ChannelId: args.ChannelId,
		Message:   "A Cursor on Target event, posted as it would arrive:\n" + cotExampleEvent,
	}

	if card, stamped := p.cotStamp(post); stamped {
		post = card
	}

	if _, appErr := p.API.CreatePost(post); appErr != nil {
		p.API.LogError("tactical-fusion: could not post the Cursor on Target example",
			"error_code", errcode.CommandDetailsPostFailed, "error", appErr.Error())
		return false
	}

	return true
}

// cotExampleIsStampable reports whether the example would render as a card,
// which is what the tests assert rather than trusting the fixture.
func cotExampleIsStampable() bool {
	events, err := cot.Parse([]byte(cotExampleEvent))
	return err == nil && len(events) == 1
}

// cotDetailSetName names the Cursor on Target post, the way detailSet.name
// names each decorator's.
const cotDetailSetName = "Cursor on Target"

// cotDetailEvents are the sources the details section shows.
//
// Every one of them is a SINGLE LINE, because the packer's atomic unit is a
// line: a fenced block would let a set that needs two messages split in the
// middle of a fence and leave one post holding an unterminated one. It is also
// how an event arrives over the wire, so nothing is lost by writing it this way.
const (
	cotDetailMultiEvent = `<event version="2.0" uid="ANDROID-1" type="a-f-G-U-C" how="m-g" ` +
		`time="2026-08-09T16:30:00Z" start="2026-08-09T16:30:00Z" stale="2026-08-09T16:32:00Z">` +
		`<point lat="21.335300" lon="-157.948300" hae="4.0" ce="9.5" le="15.0"/>` +
		`<detail><contact callsign="DELTA1"/></detail></event>` +
		`<event version="2.0" uid="ANDROID-2" type="a-h-G-U-C-I" how="m-g" ` +
		`time="2026-08-09T16:30:04Z" start="2026-08-09T16:30:04Z" stale="2026-08-09T16:32:04Z">` +
		`<point lat="19.729700" lon="-155.090000" hae="9.0" ce="30.0" le="45.0"/>` +
		`<detail><contact callsign="BRAVO2"/></detail></event>`

	cotDetailMultiLink = `<event version="2.0" uid="ANDROID-4" type="a-f-G-U-C-R" how="m-g" ` +
		`time="2026-08-09T16:32:00Z" start="2026-08-09T16:32:00Z" stale="2026-08-09T16:34:00Z">` +
		`<point lat="21.483600" lon="-158.038600" hae="256.0" ce="10.0" le="15.0"/>` +
		`<detail><contact callsign="SCOUT1"/>` +
		`<link uid="ANDROID-88" relation="p-p" parent_callsign="ALPHA"/>` +
		`<link uid="TGT-9F2A1C7B" relation="p-c"/></detail></event>`

	cotDetailLinked = `<event version="2.0" uid="ANDROID-3" type="a-f-A-M-F-Q" how="m-g" ` +
		`time="2026-08-09T16:31:00Z" start="2026-08-09T16:31:00Z" stale="2026-08-09T16:33:00Z">` +
		`<point lat="13.583900" lon="144.930000" hae="188.0" ce="12.0" le="20.0"/>` +
		`<detail><contact callsign="REAPER1"/>` +
		`<link uid="ANDROID-1" relation="p-p" parent_callsign="DELTA1"/></detail></event>`
)

// cotDetailChunks is the Cursor on Target half of `example-details`.
//
// It does not go through detailSet. Those rows are decorator rows: each one is
// run through the tagger and asserted to decorate or not, and Cursor on Target
// is not a decorator. Running an event through the tagger would also find the
// RFC 3339 timestamps inside it and hang a date-time link off the row, which
// says something false about how an event is read.
//
// Every event here is written INSIDE INLINE CODE by detailLines' caller, which
// is what keeps this post from becoming the thing it describes: a code span is
// a code range, so SoleElementSpan does not see an event in one, and the set
// contains no fenced block for SoleFencedBlock to find.
// TestTheEventExampleIsAPostOfItsOwn holds every posted message to that by
// asking cotSource, rather than by looking for "<event" in the text.
func cotDetailChunks(enabled, filesEnabled bool) []detailChunk {
	chunks := []detailChunk{{
		heading: "Posting one",
		lines: cotDetailLines([]detailExample{
			{text: cotExampleShort(), note: "an event on its own, with nothing else in the message"},
			{text: "```cot", note: "or a fenced block labelled `cot`, with the event inside it"},
			{text: "```xml", note: "the same, read the same way; a block that turns out not to be an event is left alone silently, while a `cot` one tells you why"},
		}),
	}, {
		heading: "As an attached file",
		lines: append(cotDetailLines([]detailExample{
			{text: "track.cot", note: "attach exactly one file ending `.cot` or `.xml`, with nothing in the message"},
			{text: "track.xml", note: "the same; the card names the file and links to it, so the original stays one click away"},
			{text: cotDetailFile(), note: "what a `.cot` file holds: an event, optionally behind an XML declaration, and a file may carry several the same way a pasted source can"},
		}), cotFileLimitLines(filesEnabled)...),
	}, {
		heading: "Several events in one post",
		lines: cotDetailLines([]detailExample{
			{text: cotDetailMultiEvent, note: "sibling `event` elements, with no wrapper around them"},
		}),
	}, {
		heading: "Events that name other events",
		lines: append(cotDetailLines([]detailExample{
			{text: cotDetailLinked, note: "a `link` names who sent the event and what it relates to, which the card shows as Sent by and Relates to"},
			{text: cotDetailMultiLink, note: "several links: every `uid` joins Relates to, and the one carrying a `parent_callsign` is the one that becomes Sent by"},
		}), cotRelationLines()...),
	}, {
		heading: "Left as ordinary text",
		lines: append(cotDetailLines([]detailExample{
			{text: "<!DOCTYPE event SYSTEM \"cot.dtd\">", note: "a document type declaration, and any processing instruction other than the XML one"},
			{text: "<event/>", note: "an event with no uid, type or time"},
		}), cotSourceLimitLines()...),
	}}

	if !enabled {
		chunks = append(chunks, detailChunk{
			heading: "Cursor on Target is off",
			lines: []string{"- Cursor on Target rendering is currently **off**, so none of the " +
				"above draws a card and every event is posted as the text it was written in.\n"},
		})
	}

	return chunks
}

// cotDetailLines renders rows the same way detailLines does, minus the tagger.
func cotDetailLines(examples []detailExample) []string {
	lines := make([]string, 0, len(examples))

	for _, example := range examples {
		line := "- " + inlineCode(example.text)
		if example.note != "" {
			line += " (" + example.note + ")"
		}
		lines = append(lines, line+"\n")
	}

	return lines
}

// cotFileLimitLines are the limits on the file path, and whether it is on.
func cotFileLimitLines(filesEnabled bool) []string {
	if !filesEnabled {
		return []string{"- Reading attached files is currently **off**, so an attached event " +
			"stays an ordinary attachment (an admin turns it on with `EnableCotFile`).\n"}
	}

	return []string{fmt.Sprintf("- One file, and nothing else attached: a post carrying two "+
		"is left alone, since nothing chooses between them. Files over %d KB are left as "+
		"ordinary attachments.\n", cot.MaxSourceBytes/1024)}
}

// cotSourceLimitLines are the refusals with no example short enough to show.
func cotSourceLimitLines() []string {
	return []string{
		fmt.Sprintf("- More than %d events in one source, or a source over %d KB: both are "+
			"left as the text they were written in.\n", cot.MaxEvents, cot.MaxSourceBytes/1024),
		"- Two fenced blocks in one post, or two attached files: nothing chooses between " +
			"them, so neither is read.\n",
	}
}

// cotDetailFile is the example event as a file would hold it.
//
// Built from cotExampleEvent rather than written out again, so the file row and
// the card a reader met a moment earlier cannot come to describe different
// events.
func cotDetailFile() string {
	return `<?xml version="1.0" encoding="UTF-8"?>` + cotExampleEvent
}

// cotRelationLines say what a relation is worth here, which is nothing.
//
// The value is parsed and then not interpreted: addLinks reads parent_callsign
// and uid off every link whatever its relation says. A row showing `p-p` beside
// one showing `p-c`, with no note, would imply the card tells them apart.
func cotRelationLines() []string {
	return []string{
		"- The `relation` itself is **not** read. `p-p` is what ATAK writes on almost " +
			"every event, but every link is treated the same way whatever it says: the " +
			"`parent_callsign` becomes Sent by and the `uid` joins Relates to.\n",
		fmt.Sprintf("- Up to %d links an event. Past that the extra links are **dropped** and "+
			"the event still renders, which is the opposite of the event cap: losing a "+
			"relation costs a Relates to entry, while losing an event would leave a card "+
			"quietly wrong about what was posted.\n", cot.MaxLinks),
	}
}
