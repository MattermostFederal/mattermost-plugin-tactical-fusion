package main

import (
	"math"
	"strconv"
	"strings"
	"testing"

	"github.com/mattermost/mattermost/server/public/model"

	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/cot"
	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/decorators/airport"
	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/decorators/location"
)

// Read from the catalog rather than listed, so an example added without a
// working source fails here rather than posting as raw text.
func TestEveryCotExampleParsesToNamedTypes(t *testing.T) {
	if len(cotExampleOrder) < 2 {
		t.Fatal("there are fewer than two Cursor on Target examples")
	}

	for _, example := range cotExampleOrder {
		events, err := cot.Parse([]byte(example.source))
		if err != nil {
			t.Fatalf("an example does not parse: %v", err)
		}
		if len(events) == 0 {
			t.Fatal("an example parses to no events")
		}

		for _, event := range events {
			if event.Point.Lat == "" || event.Point.Lon == "" {
				t.Errorf("event %q has no position, so the map would draw nothing", event.UID)
			}
		}

		for _, entry := range renderedEvents(t, example.source) {
			if label, _ := entry["type_label"].(string); label == "" {
				t.Error("an event has no type label, so its card would say nothing")
			}
		}
	}
}

// The two examples are deliberately different shapes: one event to meet the
// card with, and several carrying most of what a detail block can say.
func TestTheExamplesAreASimpleOneAndARichOne(t *testing.T) {
	if n := cotExampleEvents(cotExampleTarget); n != 1 {
		t.Errorf("the simple example carries %d events, want exactly 1", n)
	}
	if n := cotExampleEvents(cotExampleRich); n < 3 {
		t.Errorf("the rich example carries %d events, want at least 3", n)
	}
}

func TestTheSimpleExampleIsAHostileContact(t *testing.T) {
	entries := renderedEvents(t, cotExampleTarget)
	if len(entries) != 1 {
		t.Fatalf("the simple example renders %d events, want 1", len(entries))
	}

	event := entries[0]
	if affiliation, _ := event["affiliation"].(string); affiliation != "hostile" {
		t.Errorf("affiliation is %q, want hostile: the example is meant to be a contact", affiliation)
	}
	if callsign, _ := event["callsign"].(string); callsign == "" {
		t.Error("the example has no callsign, so the card would be named by its uid")
	}
	if remarks, _ := event["remarks"].(string); remarks == "" {
		t.Error("the example carries no remarks, so the card shows none")
	}
}

// renderedEvents is the props the card actually reads, which is where a type
// label and an affiliation are derived rather than parsed.
func renderedEvents(t *testing.T, source string) []map[string]any {
	t.Helper()

	events, err := cot.Parse([]byte(source))
	if err != nil {
		t.Fatalf("the source does not parse: %v", err)
	}

	props := cot.Props(events, cot.Source{Kind: cot.SourceFence, Text: source})

	rendered, ok := props["events"].([]any)
	if !ok {
		t.Fatal("the props carry no events array")
	}

	entries := make([]map[string]any, 0, len(rendered))
	for _, entry := range rendered {
		entries = append(entries, entry.(map[string]any))
	}
	return entries
}

// The rich example earns its name: it has to reach enough of the registry to be
// worth posting beside the simple one, and it must still stamp in full rather
// than degrading, which is what a props blob over budget does.
func TestTheRichExampleFillsInMostOfTheDetailBlock(t *testing.T) {
	written := map[string]bool{}
	for _, entry := range renderedEvents(t, cotExampleRich) {
		for key, value := range entry {
			if text, isText := value.(string); isText && text != "" {
				written[key] = true
			}
		}
	}

	for _, key := range []string{
		"callsign", "type_label", "group", "role", "takv_platform", "status_battery",
		"precision_pdop", "speed", "course", "attitude_yaw", "sensor_range",
		"color_argb", "parent", "remarks",
	} {
		if !written[key] {
			t.Errorf("the rich example never writes %q", key)
		}
	}

	for _, entry := range renderedEvents(t, cotExampleRich) {
		if dropped, _ := entry["detail_dropped"].(string); dropped != "" {
			t.Error("the rich example is over the props budget, so its card degrades")
		}
	}
}

// Both examples are posted as cards, so each has to be stampable on its own.
func TestBothExamplesStamp(t *testing.T) {
	p := newTestPlugin(t, "https://example.com", true)

	for i, example := range cotExampleOrder {
		post := &model.Post{Message: example.lead + "\n" + cotFenced(example.source, cotFenceInfo)}

		card, stamped := p.cotStamp(post)
		if !stamped {
			t.Fatalf("example %d does not stamp, so it would post as raw XML", i+1)
		}
		if card.Type != cot.PostType {
			t.Errorf("example %d stamped to type %q", i+1, card.Type)
		}
	}
}

// A card owns the whole post body, so each event is alone in its post.
func TestEachEventIsAPostOfItsOwn(t *testing.T) {
	p := newTestPlugin(t, "https://example.com", true)

	messages := runExamplePosts(t, p)
	cards := messages[len(messages)-len(cotExampleOrder):]

	if len(cards) != len(cotExampleOrder) {
		t.Fatalf("got %d Cursor on Target posts, want %d", len(cards), len(cotExampleOrder))
	}

	for i, message := range cards {
		// A fenced example carries its source in the body; an attachment carries
		// none, because the file is the source.
		want := 2
		if cotExampleOrder[i].file != "" {
			want = 0
		}
		if got := strings.Count(message, "```"); got != want {
			t.Errorf("post %d has %d fence markers, want %d:\n%s", i+1, got, want, first(message))
		}
		if strings.Contains(message, "#### ") {
			t.Errorf("post %d shares its body with a decorator set", i+1)
		}
	}
}

// The attachment example exists to exercise the file path, so it has to
// actually be a file: uploaded first, named so the reader recognizes it, and
// stamped from the attachment rather than from anything in the message.
func TestTheAttachmentExampleIsPostedAsAFile(t *testing.T) {
	p := newTestPlugin(t, "https://example.com", true)
	api := p.API.(*fakeAPI)

	runExamplePosts(t, p)

	if len(api.uploaded) != 1 {
		t.Fatalf("uploaded %d file(s), want 1", len(api.uploaded))
	}
	if !cotFileName(api.uploaded[0].Name) {
		t.Errorf("the attachment is named %q, which the reader does not accept", api.uploaded[0].Name)
	}

	var attached *model.Post
	for _, post := range api.created {
		if len(post.FileIds) == 1 {
			attached = post
		}
	}
	if attached == nil {
		t.Fatal("no example was posted with an attachment")
	}
	if attached.Type != cot.PostType {
		t.Errorf("the attachment post is type %q, so the file was not read", attached.Type)
	}
	if strings.Contains(attached.Message, "<event") {
		t.Error("the attachment post also carries the event in its body")
	}
}

// With attachments switched off the file example is left out rather than posted
// as a fence. The point of it is the attachment.
func TestTheAttachmentExampleIsSkippedWhenFilesAreOff(t *testing.T) {
	p := newTestPlugin(t, "https://example.com", true)
	config := *p.getConfiguration()
	config.EnableCotFile = false
	p.setConfiguration(&config)

	api := p.API.(*fakeAPI)
	runExamplePosts(t, p)

	if len(api.uploaded) != 0 {
		t.Errorf("uploaded %d file(s) although attachments are off", len(api.uploaded))
	}
	for _, post := range api.created {
		if len(post.FileIds) != 0 {
			t.Error("an example was posted with an attachment although attachments are off")
		}
		if strings.Contains(post.Message, "MED-4C1E") {
			t.Error("the attachment example was posted as a fence instead")
		}
	}
}

func TestExamplesPostNoEventWhenCotIsOff(t *testing.T) {
	p := newTestPlugin(t, "https://example.com", true)
	config := *p.getConfiguration()
	config.EnableCot = false
	p.setConfiguration(&config)

	for _, message := range runExamplePosts(t, p) {
		if strings.Contains(message, "<event") {
			t.Errorf("a Cursor on Target event was posted although the feature is off:\n%s", first(message))
		}
	}
}

// The simple example sits on a real airfield, so the map it draws is somewhere
// the bundled detail package actually covers.
func TestTheExampleSitsOnTheAirfieldItClaims(t *testing.T) {
	field, ok := airport.Describe("PHIK")
	if !ok {
		t.Fatal("PHIK is missing from the airfield database")
	}

	events, err := cot.Parse([]byte(cotExampleTarget))
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

	if math.Abs(lat-fieldLat) > 0.05 || math.Abs(lon-fieldLon) > 0.05 {
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

// Tab-indented XML inside a fence renders as a code block within a code block.
func TestNoCotExampleIsTabIndented(t *testing.T) {
	for i, example := range cotExampleOrder {
		if strings.Contains(example.source, "\t") {
			t.Errorf("example %d is tab indented", i+1)
		}
	}
}
