package main

import (
	"regexp"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/plugin"

	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/decorators/airport"
)

func runMCPExamples(t *testing.T, p *Plugin) (*model.CommandResponse, *fakeAPI) {
	t.Helper()

	api, ok := p.API.(*fakeAPI)
	if !ok {
		t.Fatal("runMCPExamples needs the fake API")
	}
	api.created = nil

	response, appErr := p.ExecuteCommand(&plugin.Context{}, &model.CommandArgs{
		Command:   "/tactical-fusion examples " + mcpExamplesOption,
		UserId:    "user1",
		ChannelId: testCommandChannel,
	})
	if appErr != nil {
		t.Fatalf("ExecuteCommand returned an error: %v", appErr)
	}
	return response, api
}

func TestMCPExamplesPostOnePromptPerBlockForTheAgent(t *testing.T) {
	_, api := runMCPExamples(t, newTestPlugin(t, "https://example.com", true))

	if len(api.created) != 1 {
		t.Fatalf("posted %d messages, want one", len(api.created))
	}
	message := api.created[0].Message
	if !strings.HasPrefix(message, "#### Ask "+mcpExamplesAgent+"\n") {
		t.Fatalf("the message does not open with its heading:\n%s", message)
	}

	blocks := codeBlocks(t, message)
	if len(blocks) != len(agentPrompts) {
		t.Fatalf("%d blocks for %d prompts", len(blocks), len(agentPrompts))
	}
	for i, block := range blocks {
		if block != agentPromptText(agentPrompts[i]) || !strings.HasPrefix(block, mcpExamplesAgent+" ") {
			t.Errorf("block %d is %q, want the prompt addressed to %s", i, block, mcpExamplesAgent)
		}
		if strings.Contains(block, "\n") {
			t.Errorf("prompt %d spans lines, so it would not paste as one message: %q", i, block)
		}
	}
	if utf8.RuneCountInString(message) > safePostRunes {
		t.Fatalf("the message is %d runes, over the post limit", utf8.RuneCountInString(message))
	}
}

func TestEveryPostedExampleSetHasAnAgentPrompt(t *testing.T) {
	labels := map[string]bool{}
	for _, prompt := range agentPrompts {
		labels[prompt.label] = true
	}
	for _, key := range exampleSetOrder {
		if !labels[exampleSets[key].name] {
			t.Errorf("the %q examples have no prompt for the agent", exampleSets[key].name)
		}
	}
}

func TestEveryAirfieldAnAgentPromptNamesIsOneThePluginKnows(t *testing.T) {
	ident := regexp.MustCompile(`\b[A-Z]{4}\b`)
	for _, prompt := range agentPrompts {
		for _, code := range ident.FindAllString(prompt.prompt, -1) {
			if _, ok := airport.Lookup(code); !ok {
				t.Errorf("%s: %s is not an airfield the plugin knows", prompt.label, code)
			}
		}
	}
}

func TestTheUnknownOptionRefusalNamesEveryOption(t *testing.T) {
	p := newTestPlugin(t, "https://example.com", true)

	response, _ := p.ExecuteCommand(&plugin.Context{}, &model.CommandArgs{
		Command: "/tactical-fusion examples --nope", UserId: "user1", ChannelId: testCommandChannel,
	})

	for _, option := range []string{rawExamplesOption, mcpExamplesOption} {
		if !strings.Contains(response.Text, option) {
			t.Errorf("the refusal does not mention %s: %q", option, response.Text)
		}
	}
}
