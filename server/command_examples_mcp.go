package main

import (
	"strings"

	"github.com/mattermost/mattermost/server/public/model"

	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/errcode"
)

const (
	mcpExamplesAgent = "@fusion"
	minimumFence     = 3
)

type agentPrompt struct {
	label  string
	prompt string
}

var agentPrompts = []agentPrompt{
	{"Date-time groups", "When is 241205ZSEP26 in Honolulu, Guam and Washington local time, and how many hours from now is it?"},
	{"Coordinates", "Convert 4Q FJ 0906 5962 to decimal degrees, DMS and GEOREF, and tell me which country it is in."},
	{"Airfields", "Compare PHIK in Hawaii and PGUM in Guam: runway lengths, field elevation and tower frequencies."},
	{"Aviation reports", "Decode this METAR and tell me whether VFR flight is possible: METAR PHNL 241153Z 06012G20KT 10SM FEW025 SCT045 BKN080 27/19 A3002 RMK AO2"},
	{"Frequencies", "What are 121.5, 243.0 and 8992 kHz used for, and which bands and channel spacing are they in?"},
	{"Cyber context", "Triage these indicators from an alert and tell me which are known exploited or reported malicious, with the sources: CVE-2025-55182, 141.98.9.137, ed01ebfbc9eb5bbea545af4d01bf5f1071661840480439c6e5babe8e080e41aa, T1059.001"},
	{"Cursor on Target", "Create CoT with a red 25 mile threat circle centered on PHIK in Hawaii, a green line from PHIK to PGUM in Guam, and a friendly ground unit at PGUM, and post it."},
	{"GeoJSON", "Create GeoJSON with a red 25 mile circle centered on PHIK in Hawaii and a green line from PHIK to PGUM in Guam, and post it."},
	{"Notes", "Make a note link for ROE that explains the three weapons control statuses, free, tight and hold, in a small table."},
	{"Any message", "Rewrite this with Tactical Fusion links: TGT01 at 4Q FJ 0906 5962, TOT 241205ZSEP26, divert to PGUM, guard on 243.0."},
}

func outerFenced(body string) string {
	longest, run := 0, 0
	for i := range len(body) {
		if body[i] != '`' {
			run = 0
			continue
		}
		run++
		longest = max(longest, run)
	}
	fence := strings.Repeat("`", max(minimumFence, longest+1))
	return fence + "\n" + strings.TrimRight(body, "\n") + "\n" + fence
}

func agentPromptText(prompt agentPrompt) string {
	return mcpExamplesAgent + " " + prompt.prompt
}

func mcpExampleMessage() string {
	var b strings.Builder
	b.WriteString("#### Ask " + mcpExamplesAgent + "\n\n")
	b.WriteString("One question for each kind of thing the Tactical Fusion tools know. Copy one into a channel or a direct message with the agent.\n")
	for _, prompt := range agentPrompts {
		b.WriteString("\n**" + prompt.label + "**\n\n")
		b.WriteString(outerFenced(agentPromptText(prompt)) + "\n")
	}
	return b.String()
}

const mcpExampleCount = 1

func (p *Plugin) postMCPExample(args *model.CommandArgs) int {
	if _, appErr := p.API.CreatePost(examplePost(args, mcpExampleMessage())); appErr != nil {
		p.API.LogError("tactical-fusion: could not post the agent examples",
			"error_code", errcode.CommandExamplesPostFailed, "error", appErr.Error())
		return 1
	}
	return 0
}
