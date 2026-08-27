package main

import (
	"strings"
	"testing"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/plugin"
)

// runExamplePosts is every message the examples subcommand posts, in order.
//
// The command posts them rather than replying with them, so this reads the
// posts the fake API recorded and asserts their shape on the way past: every
// one a top-level post in the right channel, none of them a reply. Nothing else
// in the suite would notice a post that quietly became one.
func runExamplePosts(t *testing.T, p *Plugin) []string {
	t.Helper()

	api, ok := p.API.(*fakeAPI)
	if !ok {
		t.Fatal("runExamplePosts needs the fake API")
	}
	api.created = nil

	response, appErr := p.ExecuteCommand(&plugin.Context{}, &model.CommandArgs{
		Command:   "/tactical-fusion examples",
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
		t.Fatal("examples posted nothing")
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

func first(s string) string {
	if line, _, found := strings.Cut(s, "\n"); found {
		return line
	}
	return s
}
