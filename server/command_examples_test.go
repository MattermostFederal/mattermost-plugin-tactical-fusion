package main

import (
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/plugin"

	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/decorators"
	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/decorators/dtg"
)

func examplesResponse(t *testing.T, p *Plugin) *model.CommandResponse {
	t.Helper()

	response, appErr := p.ExecuteCommand(&plugin.Context{}, &model.CommandArgs{
		Command:   "/tactical-fusion examples",
		UserId:    "user1",
		ChannelId: "channel1",
	})
	if appErr != nil {
		t.Fatalf("ExecuteCommand returned an error: %v", appErr)
	}
	return response
}

// One post per set, and none of them a reply.
//
// The unit a reader thinks in is one format at a time, and a reply would file
// each under the one above it and read as a remark about it, which coordinates
// are not to date-time groups.
func TestExamplesPostOneMessagePerSet(t *testing.T) {
	p := newTestPlugin(t, "https://example.com", true)

	messages := runExamplePosts(t, p)

	want := len(exampleSetOrder) + len(cotExampleOrder)
	if len(messages) != want {
		t.Fatalf("got %d messages for %d sets plus %d Cursor on Target events",
			len(messages), len(exampleSetOrder), len(cotExampleOrder))
	}

	for i, key := range exampleSetOrder {
		heading := "#### " + exampleSets[key].name
		if !strings.HasPrefix(messages[i], heading) {
			t.Errorf("message %d does not start %q:\n%s", i+1, heading, first(messages[i]))
		}

		// Nothing else belongs in it. A set sharing a post with the next one is
		// the thing this shape exists to prevent.
		for _, other := range exampleSetOrder {
			if other == key {
				continue
			}
			if strings.Contains(messages[i], "#### "+exampleSets[other].name) {
				t.Errorf("message %d carries the %q section as well", i+1, exampleSets[other].name)
			}
		}
	}
}

// Read from the registry rather than listed, so a decorator added without a set
// fails here rather than being quietly left out of the demonstration.
func TestExamplesCoverEveryRegisteredDecorator(t *testing.T) {
	p := newTestPlugin(t, "https://example.com", true)
	joined := strings.Join(runExamplePosts(t, p), "\n")

	for _, d := range p.decorators.All() {
		if !strings.Contains(joined, "/decorate/"+d.Type()+"?") {
			t.Errorf("no example links to the %q decorator", d.Type())
		}
		if _, ok := exampleSets[d.Type()]; !ok {
			t.Errorf("the %q decorator has no example set", d.Type())
		}
	}

	for key := range exampleSets {
		if p.decorators.Get(key) == nil {
			t.Errorf("there is an example set for %q, which is not a registered decorator", key)
		}
	}
}

// Every row is read from its set rather than listed here, so a row added to a
// set is covered without touching this test.
func TestEveryExampleRowIsDecorated(t *testing.T) {
	p := newTestPlugin(t, "https://example.com", true)
	joined := strings.Join(runExamplePosts(t, p), "\n")

	checked := 0
	for _, key := range exampleSetOrder {
		for _, row := range exampleSets[key].rows {
			// UTM ships off, so its row is legitimately absent.
			if row.label == "UTM" {
				continue
			}
			checked++

			want := "**" + row.label + ":** " + inlineCode(row.text) + " → "
			at := strings.Index(joined, want)
			if at < 0 {
				t.Errorf("no decorated row for %q", row.text)
				continue
			}
			if !strings.Contains(joined[at:at+len(want)+400], "](/plugins/") {
				t.Errorf("the row for %q carries no link", row.text)
			}
		}
	}

	if checked == 0 {
		t.Fatal("checked no rows; the sets are not being read")
	}
}

// The two live rows straddle the flash threshold on purpose: one opens the
// countdown already warning, and one counts up rather than down.
func TestExamplesLiveRowsStraddleTheThreshold(t *testing.T) {
	p := newTestPlugin(t, "https://example.com", true)
	tagger := &decorators.Tagger{Registry: p.decorators, URLPrefix: p.decorateURLPrefix()}
	ref := time.Now().UTC()

	set := exampleSets[dtg.Type]
	if len(set.live) == 0 {
		t.Fatal("the date-time set has no live rows")
	}

	joined := strings.Join(exampleSetLines(tagger, ref, set), "")

	future, past := false, false
	for _, live := range set.live {
		if !strings.Contains(joined, inlineCode(dtg.FormatZulu(ref.Add(live.offset)))) {
			t.Errorf("no row for the %s offset", live.offset)
		}
		if live.offset > 0 {
			future = true
		}
		if live.offset < 0 {
			past = true
		}
	}

	if !future || !past {
		t.Error("the live rows do not straddle now; one has to count down and one up")
	}
}

// A message that does not fit is refused before anything is written, so a
// long install subpath cannot leave half a demonstration in the channel.
func TestExamplesFitInTheirPosts(t *testing.T) {
	for _, siteURL := range []string{
		"https://example.com",
		"https://example.com/mattermost",
		"https://example.com/a/rather/long/subpath/for/mattermost",
	} {
		p := newTestPlugin(t, siteURL, true)

		for i, message := range runExamplePosts(t, p) {
			if n := utf8.RuneCountInString(message); n > safePostRunes {
				t.Errorf("%s: message %d is %d runes, over the %d floor", siteURL, i+1, n, safePostRunes)
			}
		}
	}
}

func TestExamplesRefuseRatherThanOverflowing(t *testing.T) {
	p := newTestPlugin(t, "https://example.com/"+strings.Repeat("subpath/", 400), true)
	api := p.API.(*fakeAPI)
	api.created = nil

	response := examplesResponse(t, p)

	if response.ResponseType != model.CommandResponseTypeEphemeral {
		t.Fatalf("the refusal is %q, want ephemeral so a misfire never reaches the channel", response.ResponseType)
	}
	if !strings.Contains(response.Text, "TF-16004") {
		t.Errorf("the refusal carries no code: %s", response.Text)
	}
	if len(api.created) != 0 {
		t.Errorf("it posted %d message(s) before refusing; nothing should be written", len(api.created))
	}
}

// A row whose format is switched off is dropped rather than posted undecorated.
// A bare token beside rows that became links is a permanent post advertising
// that the plugin does nothing.
func TestExamplesDropRowsThatDoNotDecorate(t *testing.T) {
	p := newTestPlugin(t, "https://example.com", true)
	p.setConfiguration(&configuration{
		EnableLocation:         true,
		EnableLocationLatLon:   true,
		EnableLocationDDSigned: true,
		EnableLocationUSMTF:    true,
		EnableLocationMGRS:     true,
		EnableLocationMoniker:  true,
	})

	joined := strings.Join(runExamplePosts(t, p), "\n")

	if strings.Contains(joined, "/decorate/dtg?") {
		t.Error("a date-time row was posted although every date-time format is off")
	}
	for line := range strings.SplitSeq(joined, "\n") {
		if strings.HasPrefix(line, "- **") && !strings.Contains(line, "](/plugins/") {
			t.Errorf("a row was posted with no link: %s", line)
		}
	}
}

// A set with nothing left to show is left out entirely rather than posted as a
// heading over an empty list.
func TestAnEmptySetIsNotPosted(t *testing.T) {
	p := newTestPlugin(t, "https://example.com", true)
	p.setConfiguration(&configuration{
		EnableDTG:         true,
		EnableDTGMilitary: true,
	})

	for _, message := range runExamplePosts(t, p) {
		if strings.HasPrefix(message, "#### "+exampleSets["airport"].name) {
			t.Error("the airfields set was posted although its only format is off")
		}
	}
}

func TestExamplesWithEverythingOffRefusePrivately(t *testing.T) {
	p := newTestPlugin(t, "https://example.com", false)

	response := examplesResponse(t, p)

	if response.ResponseType != model.CommandResponseTypeEphemeral {
		t.Fatalf("the refusal is %q, want ephemeral", response.ResponseType)
	}
	if !strings.Contains(response.Text, "TF-16003") {
		t.Errorf("the refusal carries no code: %s", response.Text)
	}
}

func TestExamplesBeforeActivationSaySo(t *testing.T) {
	p := newTestPlugin(t, "https://example.com", true)
	p.decorators = nil

	response := examplesResponse(t, p)

	if !strings.Contains(response.Text, "TF-16001") {
		t.Errorf("the refusal carries no code: %s", response.Text)
	}
}

// The examples are already decorated, so running the hook over them must not
// write a link inside a link.
func TestExamplesSurviveTheMessageHook(t *testing.T) {
	p := newTestPlugin(t, "https://example.com", true)

	for i, message := range runExamplePosts(t, p) {
		decorated := p.decoratePost(&model.Post{Message: message}, time.Now().UTC())
		if decorated != nil && decorated.Message != message {
			t.Errorf("message %d was rewritten by the hook, so a link was written inside a link", i+1)
		}
	}
}

// Reported rather than silent, and the count is the reader's only way to know
// how much of the demonstration is missing.
func TestExamplesReportWhenAPostIsRefused(t *testing.T) {
	p := newTestPlugin(t, "https://example.com", true)
	api := p.API.(*fakeAPI)
	api.createPostErr = &model.AppError{Message: "channel is read only"}

	response := examplesResponse(t, p)

	if response.ResponseType != model.CommandResponseTypeEphemeral {
		t.Fatalf("the report is %q, want ephemeral", response.ResponseType)
	}
	if !strings.Contains(response.Text, "TF-16006") {
		t.Errorf("the report carries no code: %s", response.Text)
	}
}
