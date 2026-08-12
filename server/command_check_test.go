package main

import (
	"fmt"
	"strings"
	"testing"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/plugin"

	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/decorators"
)

// runDetails is every message example-details posts, in order.
//
// The command posts them rather than replying with them, so this reads the
// posts the fake API recorded and asserts their shape on the way past: every
// one a top-level post in the right channel, none of them a reply. Nothing else
// in the suite would notice a post that quietly became one.
func runDetails(t *testing.T, p *Plugin) []string {
	t.Helper()

	api, ok := p.API.(*fakeAPI)
	if !ok {
		t.Fatal("runDetails needs the fake API")
	}
	api.created = nil

	response, appErr := p.ExecuteCommand(&plugin.Context{}, &model.CommandArgs{
		Command:   "/tactical-fusion example-details",
		UserId:    "user1",
		ChannelId: "channel1",
	})
	if appErr != nil {
		t.Fatalf("ExecuteCommand returned an error: %v", appErr)
	}
	if response.Text != "" {
		t.Fatalf("the command replied %q; the posts are meant to be the whole output", response.Text)
	}

	if len(api.created) == 0 {
		t.Fatal("example-details posted nothing")
	}

	messages := make([]string, 0, len(api.created))
	for i, post := range api.created {
		if post.RootId != "" {
			t.Errorf("post %d has RootId %q, so it is a reply rather than its own post",
				i+1, post.RootId)
		}
		if post.ChannelId != "channel1" || post.UserId != "user1" {
			t.Errorf("post %d was created as %q in %q", i+1, post.UserId, post.ChannelId)
		}
		messages = append(messages, post.Message)
	}

	return messages
}

func runCommand(t *testing.T, p *Plugin, command string) string {
	t.Helper()

	response, appErr := p.ExecuteCommand(&plugin.Context{}, &model.CommandArgs{Command: command})
	if appErr != nil {
		t.Fatalf("ExecuteCommand(%q) returned an error: %v", command, appErr)
	}
	if response.ResponseType != model.CommandResponseTypeEphemeral {
		t.Fatalf("ExecuteCommand(%q) replied %q, want ephemeral", command, response.ResponseType)
	}
	return response.Text
}

// check is the answer to the one question decoration cannot answer for itself:
// why nothing happened.
func TestCheckShowsWhatWouldBeDecorated(t *testing.T) {
	p := newTestPlugin(t, "https://example.com", true)

	text := runCommand(t, p, "/tactical-fusion check target at 34.0561, -118.2500")

	if !strings.Contains(text, "Would be stored as") {
		t.Fatalf("check did not report a decoration:\n%s", text)
	}
	if !strings.Contains(text, "/decorate/location?") {
		t.Fatalf("check did not show the decorated link:\n%s", text)
	}
}

// The case the whole subcommand exists for: a coordinate the author believes is
// one, declined by a rule they cannot see.
func TestCheckExplainsWhyNothingMatched(t *testing.T) {
	p := newTestPlugin(t, "https://example.com", true)

	text := runCommand(t, p, "/tactical-fusion check 34.05, -118.25")

	if !strings.Contains(text, "Nothing would be decorated") {
		t.Fatalf("check claimed something matched:\n%s", text)
	}
	if !strings.Contains(text, "four decimals") {
		t.Fatalf("check did not name the rule that declined it:\n%s", text)
	}
}

// Asking the question must never put anything in the channel.
func TestCheckWithNoTextExplainsItself(t *testing.T) {
	p := newTestPlugin(t, "https://example.com", true)

	text := runCommand(t, p, "/tactical-fusion check")

	if !strings.Contains(text, "Usage:") {
		t.Fatalf("check did not explain itself:\n%s", text)
	}
}

// Multi-word text survives, including the spaces a coordinate needs.
func TestCheckKeepsTheWholeText(t *testing.T) {
	p := newTestPlugin(t, "https://example.com", true)

	text := runCommand(t, p, "/tactical-fusion check DZ is 34°03'22\"N 118°15'00\"W over")

	if !strings.Contains(text, "Would be stored as") {
		t.Fatalf("check lost the coordinate in a multi-word message:\n%s", text)
	}
}

func TestCheckBeforeActivationSaysSo(t *testing.T) {
	p := newTestPlugin(t, "https://example.com", true)
	p.decorators = nil

	text := runCommand(t, p, "/tactical-fusion check 34.0561, -118.2500")

	if !strings.Contains(text, "TF-16002") {
		t.Fatalf("check did not carry its error code:\n%s", text)
	}
}

// One post per decorator, in order.
//
// The decorator is the unit a reader thinks in, so it is the unit a post is:
// one for date-time groups, one for coordinates. Both fit at the ordinary post
// limit, which is what makes this the shape rather than an aspiration.
func TestDetailsPostOnePostPerDecorator(t *testing.T) {
	p := newTestPlugin(t, "https://example.com", true)

	messages := runDetails(t, p)

	if len(messages) != len(detailSetOrder) {
		t.Fatalf("got %d messages for %d decorators", len(messages), len(detailSetOrder))
	}

	for i, typ := range detailSetOrder {
		want := "#### " + capitalize(detailSets[typ].name)

		if !strings.HasPrefix(messages[i], want) {
			t.Errorf("message %d does not start %q:\n%s", i+1, want, first(messages[i]))
		}

		// Nothing else belongs in it. A decorator sharing a post with the next
		// one is the thing this shape exists to prevent.
		for _, other := range detailSetOrder {
			if other == typ {
				continue
			}
			if strings.Contains(messages[i], "#### "+capitalize(detailSets[other].name)) {
				t.Errorf("message %d carries the %q section as well", i+1, other)
			}
		}
	}

	// A set that fits in one message is not numbered, because the heading
	// already says which decorator it is.
	for i, message := range messages {
		if strings.Contains(message, " of ") && strings.Contains(first(message), "(") {
			t.Errorf("message %d is numbered although its set fits in one: %s", i+1, first(message))
		}
	}
}

func first(s string) string {
	if line, _, found := strings.Cut(s, "\n"); found {
		return line
	}
	return s
}

// The splitting still works, and is exercised on purpose.
//
// It no longer happens by itself: each decorator's set fits in one post at the
// ordinary limit, so every branch in the packer that deals with a message
// running out of room would be dead code held only by the hope that nobody
// deletes it. So this drives the packer at budgets small enough to force the
// split, which is also what a server with a smaller limit would do, and what
// postDetails falls back to when one turns out to.
//
// The budgets are deliberately awkward rather than round: one that splits a
// group's rows across messages is the case that needs the "(continued)"
// heading, and one barely over a single row is the case where packing could
// stall.
func TestDetailsSplitWhenAMessageRunsOutOfRoom(t *testing.T) {
	p := newTestPlugin(t, "https://example.com", true)
	tagger := &decorators.Tagger{Registry: p.decorators, URLPrefix: p.decorateURLPrefix()}

	for _, budget := range []int{500, 1000, 3936, 20000} {
		t.Run(fmt.Sprintf("budget %d", budget), func(t *testing.T) {
			messages := p.packDetails(tagger, hookRef, budget)

			// Every message still belongs to exactly one decorator, however
			// finely it split, and every one of them OPENS with a heading.
			//
			// The second half is what stops a continuation being a bare run of
			// rows with nothing saying what they are, which the packer's own
			// comment calls out and which nothing used to assert: dropping the
			// repeated heading entirely left the whole suite green.
			counts := map[string]int{}
			for i, message := range messages {
				runes := len([]rune(message))

				// A single row always goes in, so a budget below one row
				// overflows by design. The allowance is that row, not a blanket
				// exemption for small budgets: an earlier version skipped the
				// check outright below 1000 and so asserted nothing there.
				if runes > budget+headingBudget+longestDetailLine(t, p, tagger) {
					t.Errorf("message %d of %d is %d runes, past the %d budget plus one row",
						i+1, len(messages), runes, budget)
				}

				if !strings.HasPrefix(message, "#### ") {
					t.Errorf("message %d of %d does not open with a heading: %s",
						i+1, len(messages), first(message))
				}

				owner := ""
				for _, typ := range detailSetOrder {
					if strings.HasPrefix(message, "#### "+capitalize(detailSets[typ].name)) {
						owner = typ
					}
				}
				if owner == "" {
					t.Errorf("message %d belongs to no decorator: %s", i+1, first(message))
					continue
				}
				counts[owner]++

				// The first line after the decorator heading is either a group
				// heading or a group heading marked as continuing one. Anything
				// else is rows with no label.
				body := strings.TrimSpace(strings.SplitN(message, "\n", 2)[1])
				if !strings.HasPrefix(body, "**") {
					t.Errorf("message %d of %d starts its rows with no group heading:\n%s",
						i+1, len(messages), first(body))
				}
			}

			// And a group whose rows were split says so, rather than repeating
			// a heading that reads as a fresh start.
			if len(messages) > len(detailSetOrder) {
				if !strings.Contains(strings.Join(messages, "\n"), "(continued)") {
					t.Error("rows were split across messages with no (continued) heading")
				}
			}

			// However it splits, nothing is dropped: not a row, not the heading
			// that says what a run of rows is showing, and not the decorator
			// name above them.
			all := strings.Join(messages, "")
			for _, typ := range detailSetOrder {
				if !strings.Contains(all, "#### "+capitalize(detailSets[typ].name)) {
					t.Errorf("the %q heading went missing in the split", typ)
				}

				for _, group := range detailSets[typ].groups {
					if !strings.Contains(all, group.heading) {
						t.Errorf("the %q heading went missing in the split", group.heading)
					}
					// A live group's rows are generated from the command's own
					// clock, so there is no fixed text to look for here.
					// TestDetailsIncludeLiveTimes covers those.
					if group.live != nil {
						continue
					}

					for _, example := range group.examples {
						if !strings.Contains(all, inlineCode(example.text)) {
							t.Errorf("%q went missing in the split", example.text)
						}
					}
				}
			}
		})
	}
}

// The examples for every decorator have to actually arrive, which for all but
// the last means going out as an ephemeral post rather than as the reply.
func TestDetailsCoverEveryRegisteredDecorator(t *testing.T) {
	p := newTestPlugin(t, "https://example.com", true)

	all := strings.Join(runDetails(t, p), "\n")

	for _, d := range p.decorators.All() {
		if !strings.Contains(all, "/decorate/"+d.Type()+"?") {
			t.Errorf("no examples for the %q decorator", d.Type())
		}
	}
}

// Each message has to fit in one post, since each IS one post. This is the
// constraint the command is shaped around, so it is checked per message rather
// than on the total.
func TestEveryDetailMessageFitsInOnePost(t *testing.T) {
	p := newTestPlugin(t, "https://example.com", true)

	for i, message := range runDetails(t, p) {
		if runes := len([]rune(message)); runes > defaultPostRunes {
			t.Errorf("message %d is %d runes, which exceeds the %d limit", i+1, runes, defaultPostRunes)
		}
	}
}

// longestDetailLine is the size of the biggest single row, which is the one
// thing a message is allowed to exceed its budget by.
func longestDetailLine(t *testing.T, p *Plugin, tagger *decorators.Tagger) int {
	t.Helper()

	longest := 0
	for _, typ := range detailSetOrder {
		for _, chunk := range p.detailChunks(detailSets[typ], tagger, hookRef) {
			for _, line := range chunk.lines {
				longest = max(longest, len([]rune(line)))
			}
		}
	}

	return longest
}
