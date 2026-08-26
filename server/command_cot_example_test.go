package main

import (
	"math"
	"strconv"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/decorators/airport"
	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/decorators/location"

	"github.com/mattermost/mattermost/server/public/model"

	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/cot"
)

func TestTheExampleEventIsOneThisBuildReads(t *testing.T) {
	if !cotExampleIsStampable() {
		t.Fatal("the example event does not parse, so both commands demonstrate nothing")
	}
}

func TestExamplesMentionCursorOnTarget(t *testing.T) {
	p := newTestPlugin(t, "https://example.com", true)

	response := p.examplesResponse()
	if !strings.Contains(response.Text, "Cursor on Target") {
		t.Errorf("the examples post never mentions the feature:\n%s", response.Text)
	}
}

// The hazard the inline code exists for.
//
// This command writes a real post full of markdown. An unprotected event
// anywhere in it would be recognized, the post would be stamped, the card would
// own the body, and every other row would reach the reader as the plain text of
// its own markup. A code span is a protected range, which is what stops the
// demonstration eating the demonstration.
func TestTheExamplesPostIsNeverItselfStamped(t *testing.T) {
	p := newTestPlugin(t, "https://example.com", true)

	response := p.examplesResponse()

	stamped := p.decoratePost(&model.Post{Message: response.Text, UserId: "u1"}, hookRef)
	if stamped != nil && stamped.Type == cot.PostType {
		t.Fatal("the examples post was stamped as an event, so every other row " +
			"renders as plain text")
	}
}

func TestTheExamplesRowKeepsTheEventInCode(t *testing.T) {
	line := cotExampleLine()

	if !strings.Contains(line, "`") {
		t.Error("the event is not in a code span, so it can stamp the post it is shown in")
	}
	if strings.Contains(line, "```") {
		t.Error("a fenced block would be read as the post's event")
	}
}

func TestExamplesSayNothingAboutCotWhenItIsOff(t *testing.T) {
	p := newTestPlugin(t, "https://example.com", true)
	config := *p.getConfiguration()
	config.EnableCot = false
	p.setConfiguration(&config)

	if strings.Contains(p.examplesResponse().Text, "Cursor on Target") {
		t.Error("the examples post offers a feature the admin switched off")
	}
}

// The details command posts a real event, so a reader meets a card rather than
// a description of one.
func TestDetailsPostTheEventAsACard(t *testing.T) {
	p := newTestPlugin(t, "https://example.com", true)
	api := p.API.(*fakeAPI)

	p.exampleDetailsResponse(&model.CommandArgs{UserId: "u1", ChannelId: "c1"})

	var found *model.Post
	for _, post := range api.created {
		if post.Type == cot.PostType {
			found = post
		}
	}

	if found == nil {
		t.Fatal("no post was stamped, so the details show no card")
	}
	if !strings.Contains(found.Message, "<event") {
		t.Error("the stamped post does not carry the event")
	}
	if _, ok := found.GetProps()[cot.PropsKey]; !ok {
		t.Error("the stamped post carries no event props, so the card cannot render")
	}
}

// Its own post, because a card owns the whole body: an event sharing a post
// with the other examples would render them as plain text.
//
// The question asked is RECOGNITION, not whether the text holds "<event".
// The Cursor on Target examples put real events in the details posts, inside
// inline code where nothing reads them, so a substring is no longer evidence of
// anything. cotSource is the function that decides, so it is the one asked.
func TestTheEventExampleIsAPostOfItsOwn(t *testing.T) {
	p := newTestPlugin(t, "https://example.com", true)
	api := p.API.(*fakeAPI)

	p.exampleDetailsResponse(&model.CommandArgs{UserId: "u1", ChannelId: "c1"})

	for _, post := range api.created {
		if post.Type != cot.PostType {
			if _, found := p.cotSource(post); found {
				t.Errorf("an unstamped post would be read as an event:\n%s", first(post.Message))
			}
			continue
		}

		if strings.Contains(post.Message, "####") {
			t.Error("the event shares its post with a details heading")
		}
	}
}

// With the feature off nothing can be stamped, so the thing worth asserting is
// that the command still posts nothing that WOULD be a card were it on: a bare
// event in a channel with no renderer behind it is raw XML in somebody's feed.
func TestDetailsPostNoEventWhenCotIsOff(t *testing.T) {
	p := newTestPlugin(t, "https://example.com", true)
	config := *p.getConfiguration()
	config.EnableCot = false
	p.setConfiguration(&config)

	api := p.API.(*fakeAPI)
	p.exampleDetailsResponse(&model.CommandArgs{UserId: "u1", ChannelId: "c1"})

	for _, post := range api.created {
		if _, found := p.cotSource(post); found {
			t.Errorf("a post that would be a card was written with the feature off:\n%s",
				first(post.Message))
		}
	}
}

// The events the details section shows are real, and are what their notes say
// they are.
//
// A row promising "several events" that carries one, or a link row whose link
// does not parse, teaches the wrong thing and cannot be caught by reading it.
func TestTheDetailEventsAreWhatTheirRowsClaim(t *testing.T) {
	multi, err := cot.Parse([]byte(cotDetailMultiEvent))
	if err != nil {
		t.Fatalf("the multi-event example does not parse: %v", err)
	}
	if len(multi) < 2 {
		t.Errorf("the multi-event example carries %d events, so it shows nothing", len(multi))
	}

	linked, err := cot.Parse([]byte(cotDetailLinked))
	if err != nil {
		t.Fatalf("the linked example does not parse: %v", err)
	}
	if len(linked) != 1 {
		t.Fatalf("the linked example carries %d events, not one", len(linked))
	}
	if len(linked[0].Detail.Links) == 0 {
		t.Error("the linked example declares no link, which is the whole of what its row shows")
	}
	if linked[0].Detail.Links[0].ParentCallsign == "" {
		t.Error("the linked example's link names no parent, so the card has no Sent by row")
	}
}

// Every event in the details section resolves to a named type, since the row
// beside it is what a reader compares their own event against.
func TestTheDetailEventsCarryNamedTypes(t *testing.T) {
	for _, source := range []string{cotExampleEvent, cotDetailMultiEvent, cotDetailLinked} {
		events, err := cot.Parse([]byte(source))
		if err != nil {
			t.Fatalf("an example does not parse: %v", err)
		}

		for _, event := range events {
			props := cot.Props([]cot.Event{event}, cot.Source{})
			rows, _ := props["events"].([]any)
			if len(rows) != 1 {
				t.Fatalf("props carried %d events for one", len(rows))
			}
			row, _ := rows[0].(map[string]any)
			if row["type_label"] == "" {
				t.Errorf("%q resolves to no type name", event.Type)
			}
		}
	}
}

// The examples row quotes the label the card would show, and cannot drift from
// it: the row exists to demonstrate that a raw type becomes readable English,
// so a row naming a different reading than the card is worse than no row.
func TestTheExampleRowQuotesTheCardsOwnLabel(t *testing.T) {
	label := cotExampleTypeLabel()
	if label == "" {
		t.Fatal("the example's type resolves to no label, so the row cannot quote one")
	}

	if !strings.Contains(cotExampleLine(), label) {
		t.Errorf("the examples row does not carry %q:\n%s", label, cotExampleLine())
	}

	events, err := cot.Parse([]byte(cotExampleEvent))
	if err != nil {
		t.Fatalf("the example does not parse: %v", err)
	}

	props := cot.Props(events, cot.Source{})
	row := props["events"].([]any)[0].(map[string]any)
	if row["type_label"] != label {
		t.Errorf("the row says %q and the card says %q", label, row["type_label"])
	}
}

// The example is a hostile contact, which is what the crosshair's colour and
// the whole point of showing a card in a channel rest on.
//
// It also carries no __group: that element is the sender's team, and on a
// hostile contact a Team row reads as the target being on it.
func TestTheExampleIsAHostileContact(t *testing.T) {
	events, err := cot.Parse([]byte(cotExampleEvent))
	if err != nil {
		t.Fatalf("the example does not parse: %v", err)
	}

	row := cot.Props(events, cot.Source{})["events"].([]any)[0].(map[string]any)

	if row["affiliation"] != "hostile" {
		t.Errorf("the example is %q, not hostile", row["affiliation"])
	}
	if row["group"] != nil || row["role"] != nil {
		t.Errorf("the example carries a sender team (%v/%v), which a contact should not",
			row["group"], row["role"])
	}
	if row["parent"] == "" || row["parent"] == nil {
		t.Error("the example names nobody who reported it, so the card has no Sent by row")
	}
}

// The file row shows the event the reader just met, not a second one that could
// drift away from it.
func TestTheFileExampleIsTheCardsOwnEvent(t *testing.T) {
	file, err := cot.Parse([]byte(cotDetailFile()))
	if err != nil {
		t.Fatalf("the file example does not parse: %v", err)
	}
	card, err := cot.Parse([]byte(cotExampleEvent))
	if err != nil {
		t.Fatalf("the card example does not parse: %v", err)
	}

	if len(file) != 1 || len(card) != 1 {
		t.Fatalf("expected one event each, got %d and %d", len(file), len(card))
	}
	if file[0].UID != card[0].UID || file[0].Type != card[0].Type {
		t.Errorf("the file row shows %s/%s and the card shows %s/%s",
			file[0].UID, file[0].Type, card[0].UID, card[0].Type)
	}
	if !strings.HasPrefix(cotDetailFile(), "<?xml") {
		t.Error("the file row does not show the XML declaration it claims to")
	}
}

// The multi-link row's claim: every uid joins Relates to, and only the link
// carrying a parent_callsign becomes Sent by.
func TestTheMultiLinkExampleIsWhatItsRowClaims(t *testing.T) {
	events, err := cot.Parse([]byte(cotDetailMultiLink))
	if err != nil {
		t.Fatalf("the multi-link example does not parse: %v", err)
	}
	if len(events) != 1 || len(events[0].Detail.Links) < 2 {
		t.Fatalf("the row promises several links and carries %d", len(events[0].Detail.Links))
	}

	row := cot.Props(events, cot.Source{})["events"].([]any)[0].(map[string]any)

	for _, link := range events[0].Detail.Links {
		if !strings.Contains(row["related"].(string), link.UID) {
			t.Errorf("link %q is missing from Relates to (%v)", link.UID, row["related"])
		}
	}

	// The row says the parent comes from the link that carries one, so a link
	// without a parent_callsign must not be able to supply it.
	if row["parent"] != "ALPHA" {
		t.Errorf("Sent by is %v, not the callsign the only parent-bearing link names", row["parent"])
	}
}

// Links past the cap are DROPPED and the event still renders, which is the
// opposite of the event cap and is what an examples row tells readers.
func TestTheLinkCapDropsRatherThanRefuses(t *testing.T) {
	links := strings.Repeat(`<link uid="X" relation="p-p"/>`, cot.MaxLinks+4)
	source := `<event version="2.0" uid="U" type="a-f-G" how="m-g" time="2026-08-09T16:30:00Z">` +
		`<point lat="21.0" lon="-157.0"/><detail>` + links + `</detail></event>`

	events, err := cot.Parse([]byte(source))
	if err != nil {
		t.Fatalf("a source past the link cap was refused, so the examples row is wrong: %v", err)
	}
	if len(events[0].Detail.Links) != cot.MaxLinks {
		t.Errorf("kept %d links, not the %d the row states", len(events[0].Detail.Links), cot.MaxLinks)
	}
}

// The example sits on Hickam, at the coordinate this plugin's own airfield
// database gives for PHIK.
//
// Pearl Harbor is the name that comes to mind first and the middle of it is
// open water, which put the crosshair in East Loch. Reading the value back out
// of the airfield database rather than restating it is what keeps the pin and
// the ICAO:PHIK example naming one place.
func TestTheExampleSitsOnTheAirfieldItClaims(t *testing.T) {
	field, ok := airport.Describe("PHIK")
	if !ok {
		t.Fatal("PHIK is missing from the airfield database")
	}

	events, err := cot.Parse([]byte(cotExampleEvent))
	if err != nil {
		t.Fatalf("the example does not parse: %v", err)
	}

	lat, err := strconv.ParseFloat(events[0].Point.Lat, 64)
	if err != nil {
		t.Fatalf("the example's latitude does not read as a number: %v", err)
	}
	lon, err := strconv.ParseFloat(events[0].Point.Lon, 64)
	if err != nil {
		t.Fatalf("the example's longitude does not read as a number: %v", err)
	}

	fieldLat, fieldLon, ok := positionOf(t, field)
	if !ok {
		t.Fatal("PHIK carries no position, so the example cannot be held to it")
	}

	if math.Abs(lat-fieldLat) > 0.0001 || math.Abs(lon-fieldLon) > 0.0001 {
		t.Errorf("the example is at %.4f,%.4f and PHIK is at %.4f,%.4f",
			lat, lon, fieldLat, fieldLon)
	}
}

func positionOf(t *testing.T, field airport.Details) (lat, lon float64, ok bool) {
	t.Helper()

	if !field.HasPosition {
		return 0, 0, false
	}

	parsed, ok := location.Parse(location.Format(field.Format), field.Token)
	if !ok {
		return 0, 0, false
	}

	return parsed.Point()
}

func TestTheDetailExamplesAreWhatTheirRowsClaim(t *testing.T) {
	cases := []struct {
		name   string
		source string
		want   map[string]string
	}{{
		name:   "chat",
		source: cotDetailChat,
		want: map[string]string{
			"class":       cot.ClassChat,
			"chat_sender": "ALPHA-1",
			"chat_room":   "Operations",
			"chatgrp_id":  "t1",
			"remarks":     "Moving to checkpoint Bravo.",
		},
	}, {
		name:   "medevac",
		source: cotDetailMedevac,
		want: map[string]string{
			"class":            cot.ClassMedevac,
			"medevac_urgent":   "0",
			"medevac_priority": "1",
			"medevac_routine":  "0",
			"medevac_litter":   "2",
		},
	}, {
		name:   "device",
		source: cotDetailDevice,
		want: map[string]string{
			"takv_platform":    "ATAK-CIV",
			"status_battery":   "87%",
			"contact_endpoint": "192.168.1.40:4242:tcp",
			"uid_extra_droid":  "ALPHA1",
			"archive":          "stated",
		},
	}, {
		name:   "precision",
		source: cotDetailPrecision,
		want: map[string]string{
			"precision_geopointsrc": "GPS",
			"precision_hdop":        "0.8",
		},
	}, {
		name:   "attitude",
		source: cotDetailAttitude,
		want:   map[string]string{"attitude_yaw": "271.5°", "attitude_roll": "-4.5°"},
	}, {
		name:   "track",
		source: cotDetailTrack,
		want:   map[string]string{"track_slope": "-2.5°", "speed": "13.4 m/s", "course": "72°"},
	}, {
		name:   "sensor",
		source: cotDetailSensor,
		want: map[string]string{
			"class":        cot.ClassSensor,
			"sensor_fov":   "42°",
			"sensor_range": "1500 m",
			"sensor_model": "MX-10",
		},
	}, {
		name:   "video",
		source: cotDetailVideo,
		want: map[string]string{
			"class":              cot.ClassVideo,
			"video_uid":          "VID-1",
			"video_conn_address": "198.51.100.20",
			"video_conn_path":    "/tower",
		},
	}, {
		name:   "appearance",
		source: cotDetailAppearance,
		want: map[string]string{
			"usericon_iconsetpath": "COT_MAPPING_2525C/a-f/a-f-G-U-C",
			"color_argb":           "#ff0000",
		},
	}, {
		name:   "group",
		source: cotDetailGroup,
		want:   map[string]string{"group": "Cyan", "role": "Team Lead"},
	}}

	for _, c := range cases {
		props := detailExampleProps(t, c.source)

		for key, want := range c.want {
			if props[key] != want {
				t.Errorf("%s: %s is %v, want %q", c.name, key, props[key], want)
			}
		}
	}
}

func TestTheFlowExampleIsAProcessingPath(t *testing.T) {
	props := detailExampleProps(t, cotDetailFlow)

	flow, ok := props["flow"].([]any)
	if !ok || len(flow) != 2 {
		t.Fatalf("the flow example produced %v, want two hops", props["flow"])
	}

	first := flow[0].(map[string]any)["system"]
	if first != "TAK-Server-Prod" {
		t.Errorf("the first hop is %v, want TAK-Server-Prod; the row promises document order", first)
	}
	for _, entry := range flow {
		if entry.(map[string]any)["system"] == "version" {
			t.Error("version was rendered as a system, which the row says it is not")
		}
	}
}

func TestTheUnknownExampleIsCountedRatherThanDropped(t *testing.T) {
	props := detailExampleProps(t, cotDetailUnknown)

	if props["detail_unknown"] != "2" {
		t.Fatalf("the unrecognised example counted %v elements, want the two it carries",
			props["detail_unknown"])
	}
	if props["callsign"] != "GOLF1" {
		t.Errorf("the event lost its callsign to the elements it does not read: %v", props["callsign"])
	}
}

func TestTheDetailNotesAreTrueAboutAnAtom(t *testing.T) {
	source := strings.Replace(cotDetailChat, `type="b-t-f"`, `type="a-h-G-U-C-A"`, 1)

	if held, ok := detailExampleProps(t, source)["class"]; ok {
		t.Errorf("a hostile contact carrying the chat example classified as %v", held)
	}
}

func detailExampleProps(t *testing.T, source string) map[string]any {
	t.Helper()

	events, err := cot.Parse([]byte(source))
	if err != nil {
		t.Fatalf("the example does not parse: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("the example carries %d events, not one", len(events))
	}

	blob := cot.Props(events, cot.Source{Kind: cot.SourceFence, Text: source})
	return blob["events"].([]any)[0].(map[string]any)
}

func cotExampleSources() map[string]string {
	return map[string]string{
		"example":    cotExampleEvent,
		"one line":   cotExampleOneLine,
		"multi":      cotDetailMultiEvent,
		"batch":      cotDetailBatch,
		"multi link": cotDetailMultiLink,
		"linked":     cotDetailLinked,
		"chat":       cotDetailChat,
		"medevac":    cotDetailMedevac,
		"device":     cotDetailDevice,
		"precision":  cotDetailPrecision,
		"attitude":   cotDetailAttitude,
		"track":      cotDetailTrack,
		"sensor":     cotDetailSensor,
		"video":      cotDetailVideo,
		"appearance": cotDetailAppearance,
		"group":      cotDetailGroup,
		"flow":       cotDetailFlow,
		"checklist":  cotDetailChecklist,
		"unknown":    cotDetailUnknown,
		"area":       cotDetailArea,
		"circle":     cotDetailCircle,
		"route":      cotDetailRoute,
		"bad shape":  cotDetailBadShape,
		"routing":    cotDetailRouting,
		"fence":      cotDetailFence,
		"file":       cotDetailFile(),
	}
}

func TestEveryCotExampleParsesToNamedTypes(t *testing.T) {
	for name, source := range cotExampleSources() {
		events, err := cot.Parse([]byte(source))
		if err != nil {
			t.Errorf("%s does not parse: %v", name, err)
			continue
		}

		for _, event := range events {
			row := cot.Props([]cot.Event{event}, cot.Source{})["events"].([]any)[0].(map[string]any)
			if row["type_label"] == "" || row["type_label"] == nil {
				t.Errorf("%s: %q resolves to no type name", name, event.Type)
			}
		}
	}
}

func TestEveryRegisteredExtensionIsDemonstrated(t *testing.T) {
	written := map[string]bool{}

	for _, source := range cotExampleSources() {
		events, err := cot.Parse([]byte(source))
		if err != nil {
			continue
		}

		for _, raw := range cot.Props(events, cot.Source{})["events"].([]any) {
			for key := range raw.(map[string]any) {
				written[key] = true
			}
		}
	}

	for _, key := range cot.PropKeys() {
		if !written[key] {
			t.Errorf("no example writes %q, so nothing demonstrates it", key)
		}
	}
}

func TestNoCotAtomCarriesExactlyOneFence(t *testing.T) {
	for _, enabled := range []bool{true, false} {
		for _, files := range []bool{true, false} {
			for _, chunk := range append(cotDetailChunks(enabled, files), cotExtensionChunks()...) {
				for i, atom := range chunk.lines {
					if fences := countFences(atom); fences == 1 {
						t.Errorf("%q atom %d carries exactly one fence, so a post holding "+
							"only it would be stamped as a card", chunk.heading, i)
					}
				}
			}
		}
	}
}

func TestEveryCotAtomFitsTheFloorBudget(t *testing.T) {
	floor := safePostRunes - headingBudget

	for _, chunk := range append(cotDetailChunks(true, true), cotExtensionChunks()...) {
		for i, atom := range chunk.lines {
			if runes := utf8.RuneCountInString(atom); runes > floor {
				t.Errorf("%q atom %d is %d runes, past the %d floor the retry ladder "+
					"falls back to", chunk.heading, i, runes, floor)
			}
		}
	}
}

func TestTheFlatExampleIsRecognizedWithoutAFence(t *testing.T) {
	p := newTestPlugin(t, "https://example.com", true)

	source, found := p.cotSource(&model.Post{Message: cotExampleOneLine, UserId: "u1"})
	if !found {
		t.Fatal("the flat event is not recognized when posted bare, so the row promising " +
			"no fence is needed is false")
	}
	if _, err := cot.Parse([]byte(source.Text)); err != nil {
		t.Errorf("the recognized span does not parse: %v", err)
	}

	if _, found := p.cotSource(&model.Post{Message: cotExampleEvent, UserId: "u1"}); found {
		t.Error("the indented event was recognized bare, so the row telling readers to " +
			"fence an indented event is stating a rule that does not exist")
	}
}

func TestNoCotExampleIsTabIndented(t *testing.T) {
	for name, source := range cotExampleSources() {
		for line := range strings.SplitSeq(source, "\n") {
			if strings.HasPrefix(line, "\t") {
				t.Errorf("%s has a tab-indented line: %q", name, line)
			}
		}
	}
}

func TestTheFlatExampleIsTheCardsOwnEvent(t *testing.T) {
	short, err := cot.Parse([]byte(cotExampleOneLine))
	if err != nil {
		t.Fatalf("the compact row's event does not parse: %v", err)
	}
	card, err := cot.Parse([]byte(cotExampleEvent))
	if err != nil {
		t.Fatalf("the card example does not parse: %v", err)
	}

	if len(short) != 1 || len(card) != 1 {
		t.Fatalf("expected one event each, got %d and %d", len(short), len(card))
	}
	if short[0].UID != card[0].UID || short[0].Type != card[0].Type ||
		short[0].Time != card[0].Time || short[0].Point != card[0].Point {
		t.Errorf("the flat example shows %s/%s and the card shows %s/%s",
			short[0].UID, short[0].Type, card[0].UID, card[0].Type)
	}
	if strings.Contains(cotExampleOneLine, "\n") {
		t.Error("the flat example is not one line, so it is not recognized without a fence")
	}
	if strings.Contains(cotExampleShort(), "\n") {
		t.Error("the compact row is not one line, so it cannot sit in the examples list")
	}
}

func countFences(atom string) int {
	fences := 0
	for line := range strings.SplitSeq(atom, "\n") {
		if strings.HasPrefix(line, "```") {
			fences++
		}
	}

	return fences / 2
}

// The shape rows make four specific claims a reader cannot check by looking.
func TestTheShapeExamplesAreWhatTheirRowsClaim(t *testing.T) {
	area := detailExampleProps(t, cotDetailArea)
	geometry, ok := area["geometry"].(map[string]any)
	if !ok {
		t.Fatalf("the area example describes no shape: %v", area["geometry"])
	}
	if geometry["kind"] != cot.GeometryPolyline || geometry["closed"] == nil {
		t.Errorf("the area example is %v; its row says a closed outline", geometry)
	}
	if geometry["count"] != "4" {
		t.Errorf("the area example has %v points, and its row shows four", geometry["count"])
	}

	circle := detailExampleProps(t, cotDetailCircle)["geometry"].(map[string]any)
	if circle["kind"] != cot.GeometryEllipse {
		t.Errorf("the circle example is %v, not an ellipse", circle["kind"])
	}
	if circle["major"] != "400 m" || circle["angle"] != "30°" {
		t.Errorf("the circle example reads %v; its row names the axes and the bearing", circle)
	}
	if _, held := circle["points"]; held {
		t.Error("the circle example carries a vertex list, which its row says it does not")
	}

	// The row's whole point: the two kinds of link do not cost each other.
	route := detailExampleProps(t, cotDetailRoute)
	shape := route["geometry"].(map[string]any)
	if shape["kind"] != cot.GeometryRoute || shape["count"] != "3" {
		t.Errorf("the route example is %v; its row says three route points", shape)
	}
	if route["parent"] != "ALPHA" {
		t.Errorf("the route example lost its relation: %v", route["parent"])
	}
	if route["route_type"] != "Infil" {
		t.Errorf("the route example states no link_attr: %v", route["route_type"])
	}

	// And the one that says a shape is NOT drawn rather than drawn wrong.
	bad := detailExampleProps(t, cotDetailBadShape)
	undrawn := bad["geometry"].(map[string]any)
	if _, held := undrawn["points"]; held {
		t.Error("the bad shape example was drawn, which is what its row says cannot happen")
	}
	if undrawn["note"] != cot.GeometryUnusableNote {
		t.Errorf("the bad shape example's note is %v, want the refusal", undrawn["note"])
	}
	if bad["callsign"] != "BAD CORNER" {
		t.Errorf("the bad shape example lost its callsign: %v", bad["callsign"])
	}
}
