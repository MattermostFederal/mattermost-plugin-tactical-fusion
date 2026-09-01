package main

import (
	"github.com/mattermost/mattermost/server/public/model"

	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/cot"
	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/errcode"
)

const cotExampleTarget = `<event version="2.0" uid="TGT-9F2A1C7B" type="a-h-G-U-C-A" how="h-e"
       time="2026-08-09T16:30:00Z" start="2026-08-09T16:30:00Z" stale="2026-08-09T16:32:00Z">
  <point lat="21.335300" lon="-157.948300" hae="4.0" ce="45.0" le="60.0"/>
  <detail>
    <contact callsign="TGT01"/>
    <link uid="ANDROID-88" relation="p-p" parent_callsign="ALPHA"/>
    <remarks>two vehicles, stationary</remarks>
  </detail>
</event>`

const cotExampleRich = `<event version="2.0" uid="ANDROID-88" type="a-f-G-U-C" how="m-g"
       time="2026-08-09T16:30:00Z" start="2026-08-09T16:30:00Z" stale="2026-08-09T16:40:00Z">
  <point lat="21.318700" lon="-157.922500" hae="12.0" ce="9.5" le="15.0"/>
  <detail>
    <contact callsign="ALPHA" endpoint="192.168.1.20:4242:tcp"/>
    <__group name="Cyan" role="Team Lead"/>
    <takv platform="ATAK-CIV" device="SAMSUNG SM-G950U" os="29" version="4.10.0"/>
    <uid Droid="ALPHA"/>
    <status battery="87" readiness="true"/>
    <precisionlocation geopointsrc="GPS" altsrc="DTED2" pdop="1.4" hdop="0.9" vdop="1.1"/>
    <track speed="1.4" course="271.5"/>
    <remarks>on foot, moving west</remarks>
  </detail>
</event>
<event version="2.0" uid="REAPER1" type="a-f-A-M-F-Q" how="m-g"
       time="2026-08-09T16:30:05Z" start="2026-08-09T16:30:05Z" stale="2026-08-09T16:35:05Z">
  <point lat="21.402000" lon="-157.880000" hae="3200.0" ce="12.0" le="20.0"/>
  <detail>
    <contact callsign="REAPER1"/>
    <link uid="ANDROID-88" relation="p-p" parent_callsign="ALPHA"/>
    <track speed="58.0" course="94.0" slope="-2.5"/>
    <Attitude roll="1.5" pitch="-3.0" yaw="94.0"/>
    <sensor azimuth="94.0" elevation="-18.0" range="4200.0" fov="12.0" vfov="7.0" model="MTS-B"/>
    <remarks>orbiting, sensor on the coast road</remarks>
  </detail>
</event>
<event version="2.0" uid="TGT-9F2A1C7B" type="a-h-G-U-C-A" how="h-e"
       time="2026-08-09T16:30:10Z" start="2026-08-09T16:30:10Z" stale="2026-08-09T16:32:10Z">
  <point lat="21.335300" lon="-157.948300" hae="4.0" ce="45.0" le="60.0"/>
  <detail>
    <contact callsign="TGT01"/>
    <color argb="-65536"/>
    <usericon iconsetpath="COT_MAPPING_2525C/a-h/a-h-G-U-C-A"/>
    <remarks>two vehicles, stationary</remarks>
  </detail>
</event>
<event version="2.0" uid="AREA-7C3" type="u-d-f" how="h-e"
       time="2026-08-09T16:30:12Z" start="2026-08-09T16:30:12Z" stale="2026-08-09T17:30:12Z">
  <point lat="21.337800" lon="-157.945000" hae="9999999.0" ce="9999999.0" le="9999999.0"/>
  <detail>
    <contact callsign="SUSPECTED AREA"/>
    <color argb="-65536"/>
    <shape>
      <polyline closed="true">
        <vertex lat="21.337800" lon="-157.932000"/>
        <vertex lat="21.341634" lon="-157.936452"/>
        <vertex lat="21.342828" lon="-157.940695"/>
        <vertex lat="21.348787" lon="-157.942307"/>
        <vertex lat="21.349455" lon="-157.947856"/>
        <vertex lat="21.345316" lon="-157.951436"/>
        <vertex lat="21.341716" lon="-157.953732"/>
        <vertex lat="21.337800" lon="-157.953044"/>
        <vertex lat="21.333130" lon="-157.955412"/>
        <vertex lat="21.327504" lon="-157.953816"/>
        <vertex lat="21.329679" lon="-157.946990"/>
        <vertex lat="21.330346" lon="-157.943173"/>
        <vertex lat="21.329992" lon="-157.938315"/>
        <vertex lat="21.333212" lon="-157.934771"/>
      </polyline>
    </shape>
    <remarks>suspected hostile area, fourteen point outline</remarks>
  </detail>
</event>`

const cotExampleAttachment = `<?xml version="1.0" encoding="UTF-8"?>
<event version="2.0" uid="MED-4C1E" type="b-r-f-h-c" how="h-g-i-g-o"
       time="2026-08-09T16:31:00Z" start="2026-08-09T16:31:00Z" stale="2026-08-09T17:31:00Z">
  <point lat="21.330100" lon="-157.939900" hae="6.0" ce="20.0" le="30.0"/>
  <detail>
    <contact callsign="MEDEVAC-1"/>
    <_medevac_ title="MED.1.281630" urgent="1" priority="2" litter="1" ambulatory="2"
               equipment_detail="hoist" security="1" hlz_marking="3" freq="42.5"
               nationality="1" zone_prot_selection="2"/>
    <remarks>one urgent, two priority, hoist required</remarks>
  </detail>
</event>`

type cotExample struct {
	lead   string
	source string

	// file is the attachment name when this example is posted as a file rather
	// than as a fenced block. Empty means a fence.
	file string
}

var cotExampleOrder = []cotExample{
	{
		lead: "A hostile contact, posted as it would arrive. One event, and the card " +
			"names what it is, where it is and how long it is good for:",
		source: cotExampleTarget,
	},
	{
		lead: "Three events in one message, carrying most of what a `<detail>` block can " +
			"say: team and role, device and battery, position quality, track and " +
			"attitude, a sensor, a stated display color, and an event that names its " +
			"parent. The card lists them and the map draws all three:",
		source: cotExampleRich,
	},
	{
		lead: "The same reader, from an attachment. An `.xml` or `.cot` file is read exactly " +
			"as a fenced block is, which is how an event exported from ATAK arrives, and " +
			"the card replaces the attachment rather than sitting beside it:",
		source: cotExampleAttachment,
		file:   "medevac.xml",
	},
}

// cotExampleCount is how many events this install will actually post, which is
// what the caller counts a failure against.
func (p *Plugin) cotExampleMessages() []string {
	if !p.cotEnabled() {
		return nil
	}

	messages := make([]string, 0, len(cotExampleOrder))
	for _, example := range cotExampleOrder {
		if example.file == "" {
			messages = append(messages, example.lead+"\n"+cotFenced(example.source, cotFenceInfo))
			continue
		}
		if p.cotFilesEnabled() {
			messages = append(messages, example.lead)
		}
	}

	return messages
}

func (p *Plugin) cotExampleCount() int {
	if !p.cotEnabled() {
		return 0
	}

	count := 0
	for _, example := range cotExampleOrder {
		if example.file == "" || p.cotFilesEnabled() {
			count++
		}
	}
	return count
}

func cotFenced(source, info string) string {
	return "```" + info + "\n" + source + "\n```\n"
}

// postCotExamples returns how many of the events could not be posted.
func (p *Plugin) postCotExamples(args *model.CommandArgs) int {
	if !p.cotEnabled() {
		return 0
	}

	failed := 0

	// One post each, because a card owns the whole post body: an event sharing
	// a post with anything else would render that as plain text underneath it.
	for _, example := range cotExampleOrder {
		post, ok := p.cotExamplePost(args, example)
		if !ok {
			continue
		}

		if card, stamped := p.cotStamp(post); stamped {
			post = card
		}

		if _, appErr := p.API.CreatePost(post); appErr != nil {
			failed++
			p.API.LogError("tactical-fusion: could not post a Cursor on Target example",
				"error_code", errcode.CommandExamplesPostFailed, "error", appErr.Error())
		}
	}

	return failed
}

// cotExamplePost builds one example, uploading its attachment first when it has
// one. A file example on an install with attachments switched off is skipped
// rather than posted as a fence: the point of it is the attachment.
func (p *Plugin) cotExamplePost(args *model.CommandArgs, example cotExample) (*model.Post, bool) {
	if example.file == "" {
		return &model.Post{
			UserId:    args.UserId,
			ChannelId: args.ChannelId,
			Message:   example.lead + "\n" + cotFenced(example.source, cotFenceInfo),
		}, true
	}

	if !p.cotFilesEnabled() {
		return nil, false
	}

	info, appErr := p.API.UploadFile([]byte(example.source), args.ChannelId, example.file)
	if appErr != nil || info == nil {
		reason := "the server returned no file info"
		if appErr != nil {
			reason = appErr.Error()
		}

		p.API.LogError("tactical-fusion: could not upload the Cursor on Target example attachment",
			"error_code", errcode.CommandExamplesPostFailed, "error", reason)
		return nil, false
	}

	return &model.Post{
		UserId:    args.UserId,
		ChannelId: args.ChannelId,
		Message:   example.lead,
		FileIds:   []string{info.Id},
	}, true
}

func cotExampleEvents(source string) int {
	events, err := cot.Parse([]byte(source))
	if err != nil {
		return 0
	}
	return len(events)
}
