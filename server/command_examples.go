package main

import (
	"slices"
	"strings"
	"time"

	"github.com/mattermost/mattermost/server/public/model"

	"github.com/MattermostFederal/mattermost-plugin-mission-context/server/decorators"
	"github.com/MattermostFederal/mattermost-plugin-mission-context/server/decorators/dtg"
)

// commandExample is one line of the /mission-context examples output: the text
// a user would type, and a short reason when the outcome is not self-evident.
type commandExample struct {
	text string
	note string
}

// decoratedExamples are tokens the decorator recognises.
var decoratedExamples = []commandExample{
	{text: "ARCT 091630ZAUG26 confirmed", note: "only the token is linked"},
	{text: "091630ZAUG2026", note: "normalised in the link"},
	{text: "091630Z", note: "month and year from the post date"},
	{text: "091630RAUG26", note: "R is UTC-5"},
	{text: "window opens 090600ZAUG26 and closes 091800ZAUG26"},
	{text: "[the plan](https://example.com) says 091630ZAUG26", note: "the link is untouched"},
	{text: "run `status --now` at 091630ZAUG26", note: "the code is untouched"},

	// RFC 3339, the other format the decorator claims.
	{text: "2026-08-09T16:30:00Z", note: "RFC 3339"},
	{text: "2026-08-09T16:30Z", note: "seconds optional"},
	{text: "2026-08-09T20:30:00+04:00", note: "the same instant, written in +04:00"},
	{text: "logged at 2026-08-09T22:00:00+05:30", note: "half-hour offsets a zone letter cannot say"},

	// The moniker some military formats use to mark where a time starts.
	{text: "DTG: 091630ZAUG26", note: "the moniker is consumed"},
	{text: "DTG:2026-08-09T16:30:00Z", note: "either format, spacing optional"},
}

// liveExampleOffsets are measured from the moment the command runs. A negative
// one counts up instead of down, which is the other half of the behaviour and
// easy to forget exists.
//
// Kept as data so the tests can check the same offsets the output uses.
var liveExampleOffsets = []struct {
	offset time.Duration
	note   string
}{
	{5 * time.Minute, "5 minutes from now"},
	{48 * time.Hour, "2 days from now"},
	{-6 * time.Hour, "6 hours ago, counting up"},
}

// liveExamples are built from the moment the command runs, so the panel opens
// on a countdown that is actually moving rather than a fixed date.
//
// Generated through the decorator's own formatter, so whatever appears here is
// by construction something the decorator recognises.
func liveExamples(ref time.Time) []commandExample {
	examples := make([]commandExample, 0, len(liveExampleOffsets))

	for _, live := range liveExampleOffsets {
		examples = append(examples, commandExample{
			text: dtg.FormatZulu(ref.Add(live.offset)),
			note: live.note,
		})
	}

	return examples
}

// rejectedExamples look like a DTG but are not one. Showing these matters as
// much as the accepted ones: the decorator is only trustworthy if it declines
// the near-misses.
var rejectedExamples = []commandExample{
	{text: "311200ZFEB26", note: "no 31 February"},
	{text: "091630JUL26", note: "JUL is the month, so no zone letter"},
	{text: "092400ZAUG26", note: "hour 24"},
	{text: "2026-08-09T16:30:00", note: "no zone, so no instant"},
	{text: "2026-08-09", note: "a date is not a time"},
	{text: "20260809T163000Z", note: "matches inside filenames"},
	{text: "2026-02-30T16:30:00Z", note: "no 30 February"},
	{text: "DTG: 091630R", note: "the moniker vouches for nothing"},
}

// skippedExamples sit inside a span that must be left exactly as written.
var skippedExamples = []commandExample{
	{text: "see https://example.com/logs/091630Z for detail", note: "inside a URL"},
	{text: "the format is `091630ZAUG26`", note: "inside code"},
	{text: "[091630ZAUG26](https://example.com/window)", note: "already a link"},
}

// examplesResponse renders the decorator's behaviour as a single message.
//
// The examples are decorated here rather than by the message hook, because the
// output is itself full of code and links, which is exactly what the tagger
// leaves alone. Running the tagger directly is also what keeps the declined
// rows honest: they are its real output, not hand-written.
func (p *Plugin) examplesResponse() *model.CommandResponse {
	if p.decorators == nil {
		return ephemeralResponse("Decorators are not registered yet. Try again once the plugin has finished activating.")
	}

	tagger := &decorators.Tagger{Registry: p.decorators, URLPrefix: p.decorateURLPrefix()}
	ref := time.Now().UTC()

	var b strings.Builder

	// Live ones first: they are the only examples whose panel opens on a
	// countdown that is actually moving, so they are what a reader should click.
	b.WriteString("**Recognised** (typed, then stored)\n")
	writeExamples(&b, tagger, ref, slices.Concat(liveExamples(ref), decoratedExamples))

	b.WriteString("\n**Declined**\n")
	writeExamples(&b, tagger, ref, rejectedExamples)

	b.WriteString("\n**Left as written**\n")
	writeExamples(&b, tagger, ref, skippedExamples)

	writeStandalonePageSection(&b, p.decorators, tagger, ref)

	if (p.dtgFormats() == dtg.Formats{}) {
		b.WriteString("\nDecoration is currently **off**, so new messages are not decorated.\n")
	}

	return ephemeralResponse(b.String())
}

// writeExamples renders one line per example: the typed text, and the stored
// result when decoration changed it.
func writeExamples(b *strings.Builder, tagger *decorators.Tagger, ref time.Time, examples []commandExample) {
	for _, example := range examples {
		b.WriteString("- " + inlineCode(example.text))

		if decorated := tagger.Decorate(example.text, ref); decorated != example.text {
			b.WriteString(" → " + decorated)
		}

		if example.note != "" {
			b.WriteString(" (" + example.note + ")")
		}

		b.WriteString("\n")
	}
}

// inlineCode wraps text in the shortest backtick delimiter that can hold it.
//
// Several examples contain backticks of their own, and a single-backtick
// delimiter would end early and break the rest of the line.
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

	// A leading or trailing backtick would otherwise merge into the delimiter.
	if strings.HasPrefix(text, "`") || strings.HasSuffix(text, "`") {
		return delimiter + " " + text + " " + delimiter
	}
	return delimiter + text + delimiter
}

// writeStandalonePageSection links to the page a link normally opens only on
// clients that do not run the webapp, such as the mobile app.
//
// Checking that page from a desktop browser otherwise means faking a phone, so
// the framework reserves a parameter that tells the webapp to stand aside.
func writeStandalonePageSection(b *strings.Builder, registry *decorators.Registry, tagger *decorators.Tagger, ref time.Time) {
	link, ok := standalonePageLink(registry, tagger, ref)
	if !ok {
		return
	}

	b.WriteString("\n**Standalone page** (what mobile gets, via `" + decorators.ForcePageParam + "`)\n")
	b.WriteString(link + "\n")
}

// standalonePageLink builds a demonstration link from whichever decorator can
// parse one of the recognised examples, so this stays correct if the first
// decorator in the registry changes.
func standalonePageLink(registry *decorators.Registry, tagger *decorators.Tagger, ref time.Time) (string, bool) {
	for _, example := range decoratedExamples {
		for _, decorator := range registry.All() {
			for _, pattern := range decorator.Patterns() {
				match := pattern.Regexp.FindStringSubmatch(example.text)
				if match == nil {
					continue
				}

				params, ok := decorator.Parse(pattern.Value(match), ref)
				if !ok {
					continue
				}

				params.Set(decorators.ForcePageParam, "1")
				return "[" + match[0] + " as a page](" + tagger.URLFor(decorator.Type(), params) + ")", true
			}
		}
	}

	return "", false
}
