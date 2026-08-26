package main

import (
	"fmt"
	"strings"

	"github.com/mattermost/mattermost/server/public/model"

	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/cot"
	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/errcode"
)

const cotExampleEvent = `<event version="2.0" uid="TGT-9F2A1C7B" type="a-h-G-U-C-A" how="h-e"
       time="2026-08-09T16:30:00Z" start="2026-08-09T16:30:00Z" stale="2026-08-09T16:32:00Z">
  <point lat="21.335300" lon="-157.948300" hae="4.0" ce="45.0" le="60.0"/>
  <detail>
    <contact callsign="TGT01"/>
    <link uid="ANDROID-88" relation="p-p" parent_callsign="ALPHA"/>
    <remarks>two vehicles, stationary</remarks>
  </detail>
</event>`

const cotExampleOneLine = `<event version="2.0" uid="TGT-9F2A1C7B" type="a-h-G-U-C-A" how="h-e" ` +
	`time="2026-08-09T16:30:00Z" start="2026-08-09T16:30:00Z" stale="2026-08-09T16:32:00Z">` +
	`<point lat="21.335300" lon="-157.948300" hae="4.0" ce="45.0" le="60.0"/>` +
	`<detail><contact callsign="TGT01"/>` +
	`<link uid="ANDROID-88" relation="p-p" parent_callsign="ALPHA"/>` +
	`<remarks>two vehicles, stationary</remarks></detail></event>`

func cotExampleLine() string {
	reading := "a card with the event's position on a map"
	if label := cotExampleTypeLabel(); label != "" {
		reading = "a card reading " + label + ", with the position on a map"
	}

	return "- **Cursor on Target:** " + inlineCode(cotExampleShort()) +
		" → " + reading +
		" - post one on its own, or run `/" + commandTrigger + " example-details`, to see it\n"
}

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

func cotExampleShort() string {
	if at := strings.Index(cotExampleOneLine, "<detail>"); at > 0 {
		return cotExampleOneLine[:at] + "</event>"
	}
	return cotExampleOneLine
}

func (p *Plugin) postCotExample(args *model.CommandArgs) bool {
	if !p.cotEnabled() {
		return true
	}

	post := &model.Post{
		UserId:    args.UserId,
		ChannelId: args.ChannelId,
		Message: "A Cursor on Target event, posted as it would arrive:\n" +
			cotFenced(cotExampleEvent, cotFenceInfo),
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

func cotExampleIsStampable() bool {
	events, err := cot.Parse([]byte(cotExampleEvent))
	return err == nil && len(events) == 1
}

const cotDetailSetName = "Cursor on Target"

// cotExtensionSetName is the second Cursor on Target post.
//
// One set is one post, and the examples outgrew that: the detail extensions and
// the shapes together run past what a server accepts, so the packer was
// splitting the set and the heading only appeared on the first half. Two named
// sets say where the seam is instead of leaving the reader to find it.
const cotExtensionSetName = "Reading an event's detail block"

const (
	cotDetailMultiEvent = `<event version="2.0" uid="ANDROID-1" type="a-f-G-U-C" how="m-g"
       time="2026-08-09T16:30:00Z" start="2026-08-09T16:30:00Z" stale="2026-08-09T16:32:00Z">
  <point lat="21.335300" lon="-157.948300" hae="4.0" ce="9.5" le="15.0"/>
  <detail>
    <contact callsign="DELTA1"/>
  </detail>
</event>
<event version="2.0" uid="ANDROID-2" type="a-h-G-U-C-I" how="m-g"
       time="2026-08-09T16:30:04Z" start="2026-08-09T16:30:04Z" stale="2026-08-09T16:32:04Z">
  <point lat="19.729700" lon="-155.090000" hae="9.0" ce="30.0" le="45.0"/>
  <detail>
    <contact callsign="BRAVO2"/>
  </detail>
</event>`

	cotDetailBatch = `<event version="2.0" uid="ANDROID-5" type="a-f-G-U-C" how="m-g"
       time="2026-08-09T16:33:00Z" start="2026-08-09T16:33:00Z" stale="2026-08-09T16:35:00Z">
  <point lat="21.318700" lon="-157.922400" hae="6.0" ce="8.0" le="12.0"/>
  <detail>
    <contact callsign="ECHO1"/>
  </detail>
</event>
<event version="2.0" uid="ANDROID-6" type="a-f-G-U-C" how="m-g"
       time="2026-08-09T16:33:02Z" start="2026-08-09T16:33:02Z" stale="2026-08-09T16:35:02Z">
  <point lat="21.319900" lon="-157.921100" hae="6.0" ce="8.0" le="12.0"/>
  <detail>
    <contact callsign="ECHO2"/>
  </detail>
</event>`

	cotDetailMultiLink = `<event version="2.0" uid="ANDROID-4" type="a-f-G-U-C-R" how="m-g"
       time="2026-08-09T16:32:00Z" start="2026-08-09T16:32:00Z" stale="2026-08-09T16:34:00Z">
  <point lat="21.483600" lon="-158.038600" hae="256.0" ce="10.0" le="15.0"/>
  <detail>
    <contact callsign="SCOUT1"/>
    <link uid="ANDROID-88" relation="p-p" parent_callsign="ALPHA"/>
    <link uid="TGT-9F2A1C7B" relation="p-c"/>
  </detail>
</event>`

	cotDetailLinked = `<event version="2.0" uid="ANDROID-3" type="a-f-A-M-F-Q" how="m-g"
       time="2026-08-09T16:31:00Z" start="2026-08-09T16:31:00Z" stale="2026-08-09T16:33:00Z">
  <point lat="13.583900" lon="144.930000" hae="188.0" ce="12.0" le="20.0"/>
  <detail>
    <contact callsign="REAPER1"/>
    <link uid="ANDROID-1" relation="p-p" parent_callsign="DELTA1"/>
  </detail>
</event>`
)

const (
	cotDetailChat = `<event version="2.0" uid="GeoChat.ANDROID-1.Operations.t1" type="b-t-f" how="h-g-i-g-o"
       time="2026-08-09T16:34:00Z" start="2026-08-09T16:34:00Z" stale="2026-08-09T16:39:00Z">
  <point lat="21.335300" lon="-157.948300" hae="4.0" ce="45.0" le="60.0"/>
  <detail>
    <__chat senderCallsign="ALPHA-1" chatroom="Operations" id="t1"
            parent="RootContactGroup" groupOwner="false">
      <chatgrp uid0="ANDROID-1" uid1="ANDROID-2" id="t1"/>
    </__chat>
    <remarks>Moving to checkpoint Bravo.</remarks>
  </detail>
</event>`

	cotDetailMedevac = `<event version="2.0" uid="MED-4C1E" type="b-r-f-h-c" how="h-e"
       time="2026-08-09T16:35:00Z" start="2026-08-09T16:35:00Z" stale="2026-08-09T16:55:00Z">
  <point lat="21.341200" lon="-157.939800" hae="5.0" ce="30.0" le="45.0"/>
  <detail>
    <_medevac_ title="MEDEVAC 4C1E" urgent="0" priority="1" routine="0"
               litter="2" ambulatory="1" casevac="false" freq="38.90"
               security="Possible enemy in area" hlz_marking="Panels"
               terrain_none="false" equipment_detail="Hoist"
               equipment_none="false" zone_prot_selection="2"
               nationality="US Military" nbc="none"
               medline_remarks="Two casualties, one ambulatory"/>
  </detail>
</event>`

	cotDetailDevice = `<event version="2.0" uid="ANDROID-7" type="a-f-G-U-C" how="m-g"
       time="2026-08-09T16:36:00Z" start="2026-08-09T16:36:00Z" stale="2026-08-09T16:38:00Z">
  <point lat="21.335300" lon="-157.948300" hae="4.0" ce="9.5" le="15.0"/>
  <detail>
    <contact callsign="ALPHA1" endpoint="192.168.1.40:4242:tcp"/>
    <uid Droid="ALPHA1"/>
    <takv device="SAMSUNG SM-T870" platform="ATAK-CIV" os="34" version="5.1.0"/>
    <status battery="87" readiness="true"/>
    <archive/>
  </detail>
</event>`

	cotDetailPrecision = `<event version="2.0" uid="ANDROID-8" type="a-f-G-U-C" how="m-g"
       time="2026-08-09T16:36:30Z" start="2026-08-09T16:36:30Z" stale="2026-08-09T16:38:30Z">
  <point lat="21.335300" lon="-157.948300" hae="4.0" ce="2.5" le="4.0"/>
  <detail>
    <contact callsign="ALPHA2"/>
    <precisionlocation geopointsrc="GPS" altsrc="GPS"
                       pdop="1.4" hdop="0.8" vdop="1.1"/>
  </detail>
</event>`

	cotDetailAttitude = `<event version="2.0" uid="ANDROID-9" type="a-f-A-M-F-Q" how="m-g"
       time="2026-08-09T16:37:00Z" start="2026-08-09T16:37:00Z" stale="2026-08-09T16:39:00Z">
  <point lat="21.410000" lon="-157.990000" hae="1220.0" ce="12.0" le="20.0"/>
  <detail>
    <contact callsign="REAPER2"/>
    <Attitude roll="-4.5" pitch="2.0" yaw="271.5"/>
  </detail>
</event>`

	cotDetailTrack = `<event version="2.0" uid="ANDROID-10" type="a-f-G-E-V-C" how="m-g"
       time="2026-08-09T16:37:30Z" start="2026-08-09T16:37:30Z" stale="2026-08-09T16:39:30Z">
  <point lat="21.329000" lon="-157.930000" hae="12.0" ce="9.5" le="15.0"/>
  <detail>
    <contact callsign="CONVOY3"/>
    <track speed="13.4" course="72.0" slope="-2.5"/>
  </detail>
</event>`

	cotDetailSensor = `<event version="2.0" uid="SENSOR-1" type="b-l-p-c" how="m-g"
       time="2026-08-09T16:38:00Z" start="2026-08-09T16:38:00Z" stale="2026-08-09T16:40:00Z">
  <point lat="21.352000" lon="-157.960000" hae="30.0" ce="10.0" le="15.0"/>
  <detail>
    <contact callsign="TOWER-CAM"/>
    <sensor azimuth="135.0" elevation="-8.0" range="1500" fov="42.0"
            vfov="24.0" roll="0.0" model="MX-10"/>
  </detail>
</event>`

	cotDetailVideo = `<event version="2.0" uid="ANDROID-11" type="b-m-p-s-p-i" how="m-g"
       time="2026-08-09T16:38:30Z" start="2026-08-09T16:38:30Z" stale="2026-08-09T16:40:30Z">
  <point lat="21.352000" lon="-157.960000" hae="30.0" ce="10.0" le="15.0"/>
  <detail>
    <contact callsign="TOWER-CAM"/>
    <__video uid="VID-1" url="rtsp://198.51.100.20:554/tower">
      <ConnectionEntry address="198.51.100.20" port="554"
                       protocol="rtsp" path="/tower"/>
    </__video>
  </detail>
</event>`

	cotDetailAppearance = `<event version="2.0" uid="MARK-2B" type="b-m-p-s-m" how="h-e"
       time="2026-08-09T16:39:00Z" start="2026-08-09T16:39:00Z" stale="2026-08-09T16:59:00Z">
  <point lat="21.344000" lon="-157.945000" hae="4.0" ce="20.0" le="30.0"/>
  <detail>
    <contact callsign="RALLY-2B"/>
    <usericon iconsetpath="COT_MAPPING_2525C/a-f/a-f-G-U-C"/>
    <color argb="-65536"/>
    <remarks>rally point, confirm before use</remarks>
  </detail>
</event>`

	cotDetailGroup = `<event version="2.0" uid="ANDROID-88" type="a-f-G-U-C" how="m-g"
       time="2026-08-09T16:39:30Z" start="2026-08-09T16:39:30Z" stale="2026-08-09T16:41:30Z">
  <point lat="21.336900" lon="-157.950100" hae="4.0" ce="9.5" le="15.0"/>
  <detail>
    <contact callsign="ALPHA"/>
    <__group name="Cyan" role="Team Lead"/>
  </detail>
</event>`

	cotDetailFlow = `<event version="2.0" uid="ANDROID-12" type="a-f-G-U-C" how="m-g"
       time="2026-08-09T16:40:00Z" start="2026-08-09T16:40:00Z" stale="2026-08-09T16:42:00Z">
  <point lat="21.335300" lon="-157.948300" hae="4.0" ce="9.5" le="15.0"/>
  <detail>
    <contact callsign="FOXTROT1"/>
    <_flow-tags_ version="0.2" TAK-Server-Prod="2026-08-23T20:10:00Z"
                 Gateway-B="2026-08-23T20:10:02Z"/>
  </detail>
</event>`

	cotDetailArea = `<event version="2.0" uid="AREA-1" type="u-d-f" how="h-g"
       time="2026-08-09T16:41:00Z" start="2026-08-09T16:41:00Z" stale="2026-08-09T17:41:00Z">
  <point lat="21.335300" lon="-157.948300" hae="4.0" ce="9999999.0" le="9999999.0"/>
  <detail>
    <contact callsign="RAMP CLOSURE"/>
    <shape>
      <polyline closed="true">
        <vertex lat="21.336800" lon="-157.950100"/>
        <vertex lat="21.336800" lon="-157.946500"/>
        <vertex lat="21.333900" lon="-157.946500"/>
        <vertex lat="21.333900" lon="-157.950100"/>
      </polyline>
    </shape>
    <color argb="-65536"/>
    <remarks>ramp closed to all traffic</remarks>
  </detail>
</event>`

	cotDetailCircle = `<event version="2.0" uid="RING-1" type="u-d-c-c" how="h-g"
       time="2026-08-09T16:42:00Z" start="2026-08-09T16:42:00Z" stale="2026-08-09T17:42:00Z">
  <point lat="21.335300" lon="-157.948300" hae="4.0" ce="9999999.0" le="9999999.0"/>
  <detail>
    <contact callsign="KEEP OUT"/>
    <shape><ellipse major="400" minor="250" angle="30"/></shape>
  </detail>
</event>`

	cotDetailRoute = `<event version="2.0" uid="ROUTE-1" type="b-m-r" how="h-e"
       time="2026-08-09T16:43:00Z" start="2026-08-09T16:43:00Z" stale="2026-08-09T17:43:00Z">
  <point lat="21.335300" lon="-157.948300" hae="4.0" ce="9999999.0" le="9999999.0"/>
  <detail>
    <contact callsign="MSR AMBER"/>
    <link point="21.335300,-157.948300,4.0"/>
    <link point="21.339000,-157.943000,4.0"/>
    <link point="21.344000,-157.938000,4.0"/>
    <link_attr routetype="Infil" planningmethod="Infil" method="Driving"
               direction="Infil" order="Ascending Check Points"/>
    <link uid="ANDROID-88" relation="p-p" parent_callsign="ALPHA"/>
  </detail>
</event>`

	cotDetailBadShape = `<event version="2.0" uid="AREA-2" type="u-d-f" how="h-g"
       time="2026-08-09T16:44:00Z" start="2026-08-09T16:44:00Z" stale="2026-08-09T17:44:00Z">
  <point lat="21.335300" lon="-157.948300" hae="4.0" ce="9999999.0" le="9999999.0"/>
  <detail>
    <contact callsign="BAD CORNER"/>
    <shape>
      <polyline closed="true">
        <vertex lat="21.336800" lon="-157.950100"/>
        <vertex lat="0x1p+3" lon="-157.946500"/>
        <vertex lat="21.333900" lon="-157.946500"/>
      </polyline>
    </shape>
  </detail>
</event>`

	cotDetailRouting = `<event version="2.0" uid="ANDROID-21" type="b-t-f" how="m-g"
       time="2026-08-09T16:45:00Z" start="2026-08-09T16:45:00Z" stale="2026-08-09T16:47:00Z">
  <point lat="21.335300" lon="-157.948300" hae="4.0" ce="9.5" le="15.0"/>
  <detail>
    <contact callsign="ALPHA-1"/>
    <__chatReceipt id="t1" chatroom="Operations" ackuid="ANDROID-22" senderCallsign="BRAVO-2"/>
    <__serverdestination destinations="takserver-hi:8089:tcp"/>
    <_radio rssi="-71" gps="3"/>
    <attachment_list hashes="[&quot;9f2a1c7b&quot;,&quot;4e8d0a13&quot;]"/>
    <TakControl>
      <TakProtocolSupport version="1"/>
      <TakRequest version="1"/>
      <TakResponse status="true"/>
    </TakControl>
  </detail>
</event>`

	cotDetailFence = `<event version="2.0" uid="FENCE-1" type="u-d-c-c" how="h-g"
       time="2026-08-09T16:46:00Z" start="2026-08-09T16:46:00Z" stale="2026-08-09T17:46:00Z">
  <point lat="21.335300" lon="-157.948300" hae="4.0" ce="9999999.0" le="9999999.0"/>
  <detail>
    <contact callsign="RESTRICTED"/>
    <shape><ellipse major="500" minor="500" angle="0"/></shape>
    <__geofence monitor="All" trigger="Entry" tracking="true"
                elevationMonitored="true" minElevation="0" maxElevation="300"
                boundingSphere="500"/>
  </detail>
</event>`

	cotDetailChecklist = `<event version="2.0" uid="ANDROID-23" type="a-f-G-U-C" how="m-g"
       time="2026-08-09T16:48:00Z" start="2026-08-09T16:48:00Z" stale="2026-08-09T16:50:00Z">
  <point lat="21.335300" lon="-157.948300" hae="4.0" ce="9.5" le="15.0"/>
  <detail>
    <contact callsign="HOTEL1"/>
    <checklist name="Pre-flight">
      <checklistColumn name="Task"/>
      <checklistColumn name="Status"/>
      <checklistTask status="COMPLETE">
        <checklistColumn>Fuel checked</checklistColumn>
      </checklistTask>
      <checklistTask status="PENDING">
        <checklistColumn>Radios checked</checklistColumn>
      </checklistTask>
    </checklist>
  </detail>
</event>`

	cotDetailUnknown = `<event version="2.0" uid="ANDROID-13" type="a-f-G-U-C" how="m-g"
       time="2026-08-09T16:40:30Z" start="2026-08-09T16:40:30Z" stale="2026-08-09T16:42:30Z">
  <point lat="21.335300" lon="-157.948300" hae="4.0" ce="9.5" le="15.0"/>
  <detail>
    <contact callsign="GOLF1"/>
    <__network uid="ANDROID-13"/>
    <fileshare filename="brief.pdf"/>
  </detail>
</event>`
)

func cotDetailChunks(enabled, filesEnabled bool) []detailChunk {
	chunks := []detailChunk{{
		heading: "Posting one",
		lines: cotFenceAtom([]cotFencedExample{
			{
				note:   "**No fence is needed** for an event on one line, as it arrives over the wire, with nothing else in the message. It is drawn here in a plain block so that this post does not render it as a card.",
				source: cotExampleOneLine, info: cotPlainFenceInfo,
			},
			{
				note:   "**Indent it to read it and you need a fence**, labeled `cot`, which is the spelling to use when you mean it. Markdown reads four spaces of indent as a code block, so the same event pasted bare and indented is left as the text it was written in.",
				source: cotExampleEvent, info: cotFenceInfo,
			},
			{
				note:   "And labeled `xml`, read the same way. A block that turns out not to be an event is left alone silently, while a `cot` one tells you why.",
				source: cotExampleEvent, info: cotXMLFenceInfo,
			},
		}),
	}, {
		heading: "As an attached file",
		lines: append(cotFenceAtom([]cotFencedExample{
			{
				note:   "What a `.cot` file holds: an event, optionally behind an XML declaration. Attach exactly one file ending `.cot` or `.xml`, with nothing in the message, and the card names the file and links to it so the original stays one click away.",
				source: cotDetailFile(), info: cotXMLFenceInfo,
			},
			{
				note:   "A file may carry several events the same way a pasted source can.",
				source: cotDetailBatch, info: cotXMLFenceInfo,
			},
		}), cotFileLimitLines(filesEnabled)...),
	}, {
		heading: "Several events in one post",
		lines: cotFenceAtom([]cotFencedExample{
			{
				note:   "Sibling `event` elements, with no wrapper around them.",
				source: cotDetailMultiEvent, info: cotFenceInfo,
			},
			{
				note:   "A batch of position reports arrives the same way.",
				source: cotDetailBatch, info: cotFenceInfo,
			},
		}),
	}, {
		heading: "Events that name other events",
		lines: append(cotFenceAtom([]cotFencedExample{
			{
				note:   "A `link` names who sent the event and what it relates to, which the card shows as Sent by and Relates to.",
				source: cotDetailLinked, info: cotFenceInfo,
			},
			{
				note:   "Several links: every `uid` joins Relates to, and the one carrying a `parent_callsign` is the one that becomes Sent by.",
				source: cotDetailMultiLink, info: cotFenceInfo,
			},
		}), cotRelationLines()...),
	}, {
		heading: "GeoChat and MEDEVAC",
		lines: cotFenceAtom([]cotFencedExample{
			{
				note:   "A GeoChat event: the card names the sender the event **states**, which is not the Mattermost account that posted it, and shows the message once.",
				source: cotDetailChat, info: cotFenceInfo,
			},
			{
				note:   "A MEDEVAC request: patient counts by precedence, and a stated `0` is kept because it is not the same as saying nothing.",
				source: cotDetailMedevac, info: cotFenceInfo,
			},
		}),
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

// cotExtensionChunks is what an event's <detail> says, which is the half of
// these examples that grew.
func cotExtensionChunks() []detailChunk {
	return []detailChunk{{
		heading: "Device and position quality",
		lines: cotFenceAtom([]cotFencedExample{
			{
				note:   "Device, network endpoint, battery and readiness, and an `archive` element asking that the event be kept. None of this reaches the card; all of it is in the sidebar under Device.",
				source: cotDetailDevice, info: cotFenceInfo,
			},
			{
				note:   "Where the fix came from and how good it was, shown under Position quality.",
				source: cotDetailPrecision, info: cotFenceInfo,
			},
		}),
	}, {
		heading: "Orientation and movement",
		lines: cotFenceAtom([]cotFencedExample{
			{
				note:   "An aircraft's attitude: roll, pitch and yaw, shown under Orientation. Yaw is called yaw rather than heading, because that is what the event said.",
				source: cotDetailAttitude, info: cotFenceInfo,
			},
			{
				note:   "A ground vehicle's track: speed and course reach the card, and slope joins them under Orientation.",
				source: cotDetailTrack, info: cotFenceInfo,
			},
		}),
	}, {
		heading: "Sensor and video",
		lines: cotFenceAtom([]cotFencedExample{
			{
				note:   "A sensor's field of view, bearing and range, summarised on the card and given in full under Payload.",
				source: cotDetailSensor, info: cotFenceInfo,
			},
			{
				note:   "A video stream, with its connection nested inside `__video`. The card says a stream is associated with the event and the address is in the sidebar.",
				source: cotDetailVideo, info: cotFenceInfo,
			},
		}),
	}, {
		heading: "Appearance and team",
		lines: cotFenceAtom([]cotFencedExample{
			{
				note:   "A stated icon and color, both shown as values in the sidebar.",
				source: cotDetailAppearance, info: cotFenceInfo,
			},
			{
				note:   "A self position report naming the sender's own team and role, which is what `__group` says. It belongs on a unit reporting itself, not on a marker somebody placed.",
				source: cotDetailGroup, info: cotFenceInfo,
			},
		}),
	}, {
		heading: "Shapes and routes",
		lines: append(cotFenceAtom([]cotFencedExample{
			{
				note:   "A drawn area: a `polyline` inside a `shape`, closed, so it is drawn as an outline on the map rather than as a single crosshair at the event's `point`. The map frames the shape and the position together, so an area larger than its own point is not half off screen.",
				source: cotDetailArea, info: cotFenceInfo,
			},
			{
				note:   "A circle, drawn from its axes rather than from a vertex list. `major` and `minor` are read as semi-axes in meters and `angle` as a bearing clockwise from north, so an ellipse keeps its meters at every zoom.",
				source: cotDetailCircle, info: cotFenceInfo,
			},
			{
				note:   "A route. Its points are `link` elements carrying a `point`, which is how ATAK writes them, and the last `link` here is an ordinary relation carrying a `uid`. The two are told apart by what the element carries, so a long route never costs the reader the Sent by row. `link_attr` describes the route itself and is shown under Shape.",
				source: cotDetailRoute, info: cotFenceInfo,
			},
			{
				note:   "A shape that is **not** drawn. Its second corner is written in a notation this build will not stand behind, so the outline is left off the map entirely rather than drawn missing a corner, and the sidebar says which happened. The callsign, the position and the times are unaffected.",
				source: cotDetailBadShape, info: cotFenceInfo,
			},
		}), cotShapeLines()...),
	}, {
		heading: "Receipts, routing and protocol",
		lines: append(cotFenceAtom([]cotFencedExample{
			{
				note:   "What a TAK server adds on the way through: a GeoChat receipt naming the message it acknowledges, the servers the event was addressed to, a radio's signal strength, the attachments the event references, and a protocol exchange. All of it sits in the sidebar under Payload and Processing path, and none of it reaches the card.",
				source: cotDetailRouting, info: cotFenceInfo,
			},
			{
				note:   "A geofence, which is behavior attached to a shape rather than a shape of its own: the circle is what is drawn, and the fence says what crossing it means.",
				source: cotDetailFence, info: cotFenceInfo,
			},
			{
				note:   "A checklist, counted rather than decoded. The sidebar reports `checklistColumn` four times and `checklistTask` twice, which are the event's own element names, and none of the attributes here are read. A column nested inside a task is counted with its siblings, so the answer does not depend on a nesting this build has never seen.",
				source: cotDetailChecklist, info: cotFenceInfo,
			},
		}), cotRoutingLines()...),
	}, {
		heading: "Processing path, and what is not recognized",
		lines: append(cotFenceAtom([]cotFencedExample{
			{
				note:   "The systems that handled the event, shown in the sidebar as a collapsed Processing path in the order the event wrote them.",
				source: cotDetailFlow, info: cotFenceInfo,
			},
			{
				note:   "Elements this build does not read: the event still renders, the sidebar says how many there were, and the whole event is unchanged under **As posted**.",
				source: cotDetailUnknown, info: cotFenceInfo,
			},
		}), cotDetailNotes()...),
	}, {
		heading: "Left as ordinary text",
		lines: append(cotDetailLines([]detailExample{
			{text: "<!DOCTYPE event SYSTEM \"cot.dtd\">", note: "a document type declaration, and any processing instruction other than the XML one"},
			{text: "<event/>", note: "an event with no uid, type or time"},
		}), cotSourceLimitLines()...),
	}}
}

type cotFencedExample struct {
	note   string
	source string
	info   string
}

const cotPlainFenceInfo = "text"

func cotFenceAtom(examples []cotFencedExample) []string {
	var b strings.Builder

	for _, example := range examples {
		b.WriteString("- " + example.note + "\n\n")
		b.WriteString(cotFenced(example.source, example.info))
		b.WriteString("\n")
	}

	return []string{b.String()}
}

func cotFenced(source, info string) string {
	return "```" + info + "\n" + source + "\n```\n"
}

// cotRoutingLines are the rules the two examples above cannot show.
func cotRoutingLines() []string {
	return []string{
		"- An `attachment_list` is **counted, not printed**. A content hash is longer " +
			"than a field, so the list would truncate mid-hash into something that " +
			"looks like a hash and is not, and nothing here resolves a hash to a file: " +
			"Mattermost file ids bear no relationship to them.\n",
		"- A radio signal carries its unit. An unlabelled `-71` is a number a reader " +
			"has to guess the meaning of, which is the same failure as this plugin " +
			"guessing it.\n",
		"- A `checklist` is counted rather than decoded. Its contents are reported " +
			"by the element names the event itself used, because no specification " +
			"this build could check says what a checklist column or task is.\n",
	}
}

// cotShapeLines are the rules a reader cannot infer from the shapes above.
func cotShapeLines() []string {
	return []string{
		fmt.Sprintf("- Up to %d points a shape. Past that the shape is **not drawn** and the "+
			"event keeps everything else it stated. Drawing the first %d of a longer route "+
			"would put a line on the map that ends where the route does not, which is worse "+
			"than drawing nothing.\n", cot.MaxVertices, cot.MaxVertices),
		"- A point must be written as a plain decimal. One that is not costs the whole " +
			"shape rather than one corner, because a polygon missing a corner is a " +
			"different polygon and not a partial one.\n",
		"- A route's points and an event's relations share the `link` element and have " +
			"**separate budgets**. Neither can exhaust the other.\n",
		"- These shapes are read as ATAK is understood to write them and have not been " +
			"checked against a real device. A shape this build reads wrongly does not " +
			"draw and is counted as unrecognized, rather than drawn wrongly.\n",
	}
}

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

func cotFileLimitLines(filesEnabled bool) []string {
	if !filesEnabled {
		return []string{"- Reading attached files is currently **off**, so an attached event " +
			"stays an ordinary attachment (an admin turns it on with `EnableCotFile`).\n"}
	}

	return []string{fmt.Sprintf("- One file, and nothing else attached: a post carrying two "+
		"is left alone, since nothing chooses between them. Files over %d KB are left as "+
		"ordinary attachments.\n", cot.MaxSourceBytes/1024)}
}

func cotSourceLimitLines() []string {
	return []string{
		fmt.Sprintf("- More than %d events in one source, or a source over %d KB: both are "+
			"left as the text they were written in.\n", cot.MaxEvents, cot.MaxSourceBytes/1024),
		"- Two fenced blocks in one post, or two attached files: nothing chooses between " +
			"them, so neither is read.\n",
	}
}

func cotDetailNotes() []string {
	return []string{
		"- A `__chat` or `_medevac_` element does **not** change how an `a-` event is " +
			"drawn. The type code decides, so nobody can re-shape a contact report by " +
			"adding one element to it.\n",
		"- A stated `color` is shown as a value in the sidebar and never colors the " +
			"marker or the affiliation dot, which are what say whose track it is.\n",
		"- A video address is shown as text and is never a link.\n",
	}
}

func cotDetailFile() string {
	return `<?xml version="1.0" encoding="UTF-8"?>` + "\n" + cotExampleEvent
}

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
