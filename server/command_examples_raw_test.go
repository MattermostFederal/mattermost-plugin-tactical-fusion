package main

import (
	"strings"
	"testing"
	"time"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/plugin"

	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/decorators"
)

func runRawExamples(t *testing.T, p *Plugin, channel string) (*model.CommandResponse, *fakeAPI) {
	t.Helper()

	api, ok := p.API.(*fakeAPI)
	if !ok {
		t.Fatal("runRawExamples needs the fake API")
	}
	api.created, api.ephemeral = nil, nil

	response, appErr := p.ExecuteCommand(&plugin.Context{}, &model.CommandArgs{
		Command:   "/tactical-fusion examples " + rawExamplesOption,
		UserId:    "user1",
		ChannelId: channel,
	})
	if appErr != nil {
		t.Fatalf("ExecuteCommand returned an error: %v", appErr)
	}
	return response, api
}

func codeBlockBody(t *testing.T, message string) string {
	t.Helper()

	lines := strings.Split(strings.TrimRight(message, "\n"), "\n")
	var open int
	for i, line := range lines {
		if strings.HasPrefix(line, "```") {
			open = i
			break
		}
	}
	fence := lines[open]
	if lines[len(lines)-1] != fence {
		t.Fatalf("the block does not close with its opening fence %q:\n%s", fence, message)
	}
	return strings.Join(lines[open+1:len(lines)-1], "\n")
}

func TestRawExamplesPostNothingAndTellOnlyTheCaller(t *testing.T) {
	p := newTestPlugin(t, "https://example.com", true)

	response, api := runRawExamples(t, p, "a-channel-nobody-may-post-in")

	if len(api.created) != 0 {
		t.Fatalf("--raw posted %d messages to the channel", len(api.created))
	}
	if len(api.ephemeral) == 0 {
		t.Fatal("--raw showed the caller nothing")
	}
	if response.Text != "" {
		t.Fatalf("the response carries text %q as well as the ephemeral posts", response.Text)
	}
}

func TestRawExamplesAreTheTextAPersonTypesAndEachStillDecorates(t *testing.T) {
	p := newTestPlugin(t, "https://example.com", true)
	_, api := runRawExamples(t, p, testCommandChannel)
	tagger := &decorators.Tagger{Registry: p.decorators, URLPrefix: p.decorateURLPrefix()}

	sets := 0
	for _, post := range api.ephemeral {
		for _, key := range exampleSetOrder {
			if !strings.HasPrefix(post.Message, "#### "+exampleSets[key].name+"\n") {
				continue
			}
			sets++
			body := codeBlockBody(t, post.Message)
			if strings.Contains(body, "](") || strings.Contains(body, "**") {
				t.Errorf("the %s block carries a link or a label rather than bare text:\n%s", key, body)
			}
			for line := range strings.SplitSeq(body, "\n") {
				if tagger.Decorate(line, time.Now().UTC()) == line {
					t.Errorf("the %s block holds %q, which does not decorate when pasted", key, line)
				}
			}
		}
	}
	if sets != len(exampleSetOrder) {
		t.Fatalf("%d decorator blocks, want one per set (%d)", sets, len(exampleSetOrder))
	}
}

func TestARawBlockHoldingAFenceIsFencedLongerSoItCopiesWhole(t *testing.T) {
	p := newTestPlugin(t, "https://example.com", true)
	_, api := runRawExamples(t, p, testCommandChannel)

	found := false
	for _, post := range api.ephemeral {
		if !strings.HasPrefix(post.Message, "#### Cursor on Target") {
			continue
		}
		found = true
		body := codeBlockBody(t, post.Message)
		if !strings.HasPrefix(body, "```"+cotFenceInfo+"\n") || !strings.HasSuffix(body, "\n```") {
			t.Errorf("the Cursor on Target block does not hold the fenced event a person posts:\n%s", body)
		}
	}
	if !found {
		t.Fatal("no Cursor on Target block")
	}
}

func TestOuterFencedOutrunsTheLongestFenceInside(t *testing.T) {
	cases := map[string]string{
		"plain":            "```",
		"```cot\nx\n```":   "````",
		"a ```` b":         "`````",
		"inline `code` ok": "```",
	}
	for body, want := range cases {
		got := outerFenced(body)
		if !strings.HasPrefix(got, want+"\n") || !strings.HasSuffix(got, "\n"+want) || strings.HasPrefix(got, want+"`") {
			t.Errorf("outerFenced(%q) = %q, want fences of %q", body, got, want)
		}
	}
}

func TestAnUnknownExamplesOptionIsRefusedWithItsCode(t *testing.T) {
	p := newTestPlugin(t, "https://example.com", true)

	response, _ := p.ExecuteCommand(&plugin.Context{}, &model.CommandArgs{
		Command: "/tactical-fusion examples --rwa", UserId: "user1", ChannelId: testCommandChannel,
	})

	if response.ResponseType != model.CommandResponseTypeEphemeral || !strings.Contains(response.Text, "TF-16012") {
		t.Fatalf("response %+v", response)
	}
}

func TestRawExamplesWithEverythingOffSaySo(t *testing.T) {
	p := newTestPlugin(t, "https://example.com", false)

	response, api := runRawExamples(t, p, testCommandChannel)

	if !strings.Contains(response.Text, "TF-16003") || len(api.ephemeral) != 0 {
		t.Fatalf("response %q, %d ephemeral posts", response.Text, len(api.ephemeral))
	}
}
