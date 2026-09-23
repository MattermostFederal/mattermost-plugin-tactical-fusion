package main

import (
	"time"

	"github.com/mattermost/mattermost/server/public/model"

	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/errcode"
)

const (
	tfrExampleLayout   = "0601021504"
	tfrExampleDuration = 8 * time.Hour
)

func tfrExampleMessage(now time.Time) string {
	start := now.UTC().Truncate(time.Hour)
	end := start.Add(tfrExampleDuration)

	return "!FDC 6/4321 ZZZ AIRSPACE HONOLULU, HI. TEMPORARY FLIGHT\n" +
		"RESTRICTIONS.\n\n" +
		"PURSUANT TO 14 CFR SECTION 91.137, TEMPORARY FLIGHT\n" +
		"RESTRICTIONS ARE IN EFFECT WI AN AREA DEFINED AS\n" +
		"3 NM RADIUS OF 211900N1575500W\n" +
		"(HNL VORTAC 045 DEG RADIAL AT 2.1 NM)\n" +
		"SFC-3000FT MSL\n\n" +
		"EFFECTIVE " + start.Format(tfrExampleLayout) + " UTC UNTIL " + end.Format(tfrExampleLayout) + " UTC.\n\n" +
		"EXC AS SPECIFIED BLW AND/OR UNLESS AUTH BY ATC:\n" +
		"NO ACFT OPS ARE AUTH IN THE AREA EXCEPT DISASTER RELIEF."
}

func (p *Plugin) tfrExampleEnabled() bool {
	return p.avreportRendersEnabled() && p.avreportFormats().NOTAM
}

func (p *Plugin) tfrExampleMessages() []string {
	if !p.tfrExampleEnabled() {
		return nil
	}
	return []string{tfrExampleMessage(time.Now())}
}

func (p *Plugin) tfrExampleCount() int {
	if !p.tfrExampleEnabled() {
		return 0
	}
	return 1
}

func (p *Plugin) postTFRExample(args *model.CommandArgs) int {
	if !p.tfrExampleEnabled() {
		return 0
	}

	now := time.Now().UTC()
	post := &model.Post{
		UserId:    args.UserId,
		ChannelId: args.ChannelId,
		RootId:    args.RootId,
		Message:   tfrExampleMessage(now),
	}

	if rendered, ok := p.avreportStamp(post, now); ok {
		post = rendered
	}

	if _, appErr := p.API.CreatePost(post); appErr != nil {
		p.API.LogError("tactical-fusion: could not post a TFR example",
			"error_code", errcode.CommandExamplesPostFailed, "error", appErr.Error())
		return 1
	}

	return 0
}
