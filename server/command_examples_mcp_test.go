package main

import (
	"regexp"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/decorators/airport"
)

func codeBlocks(t *testing.T, message string) []string {
	t.Helper()

	var blocks []string
	lines := strings.Split(strings.TrimRight(message, "\n"), "\n")
	for i := 0; i < len(lines); i++ {
		fence := lines[i]
		if !strings.HasPrefix(fence, "```") || strings.Trim(fence, "`") != "" {
			continue
		}
		end := i + 1
		for end < len(lines) && lines[end] != fence {
			end++
		}
		if end == len(lines) {
			t.Fatalf("a block opened with %q never closes:\n%s", fence, message)
		}
		blocks = append(blocks, strings.Join(lines[i+1:end], "\n"))
		i = end
	}
	return blocks
}

func TestTheAgentExampleHoldsOnePromptPerBlock(t *testing.T) {
	message := mcpExampleMessage()
	if !strings.HasPrefix(message, "#### Ask the Fusion agent\n") {
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

func TestTheAgentExampleMentionsTheAgentOnlyInsideCode(t *testing.T) {
	message := mcpExampleMessage()
	for _, block := range codeBlocks(t, message) {
		message = strings.Replace(message, block, "", 1)
	}

	if strings.Contains(message, mcpExamplesAgent) {
		t.Errorf("the examples post mentions %s outside code, so posting it would summon the agent:\n%s", mcpExamplesAgent, message)
	}
}
