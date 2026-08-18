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

func runExamples(t *testing.T, p *Plugin) *model.CommandResponse {
	t.Helper()

	response, appErr := p.ExecuteCommand(&plugin.Context{}, &model.CommandArgs{
		Command: "/tactical-fusion examples",
	})
	if appErr != nil {
		t.Fatalf("ExecuteCommand returned an error: %v", appErr)
	}

	return response
}

// It goes in the channel, which is the point of it.
//
// Pinned because it is a one-word change away from being ephemeral, and an
// ephemeral post is indistinguishable from a working one to whoever ran it.
func TestExamplesPostsToTheChannel(t *testing.T) {
	p := newTestPlugin(t, "https://example.com", true)

	response := runExamples(t, p)

	if response.ResponseType != model.CommandResponseTypeInChannel {
		t.Fatalf("ResponseType = %q, want in_channel", response.ResponseType)
	}
}

// Every row that survives is a real decorated link, in the label → link shape.
func TestExamplesRowsAreDecorated(t *testing.T) {
	p := newTestPlugin(t, "https://example.com", true)

	text := runExamples(t, p).Text

	for _, row := range exampleFixedRows {
		t.Run(row.text, func(t *testing.T) {
			// The link does not always begin at the arrow. A location field
			// label is matched but NOT consumed, so a labeled row renders
			// "→ GEOREF:[GJNJ5753](...)": the label stays in the message and
			// only the token is linked. Asserting the link starts at the arrow
			// would have declined that on the grounds it is working correctly.
			prefix := "**" + row.label + ":** " + inlineCode(row.text) + " → "

			_, rest, found := strings.Cut(text, prefix)
			if !found {
				t.Fatalf("row is missing, wanted a line starting %q:\n%s", prefix, text)
			}

			line, _, _ := strings.Cut(rest, "\n")
			if !strings.Contains(line, "](/plugins/"+manifest.Id+"/decorate/") {
				t.Fatalf("row is undecorated: %q", line)
			}
		})
	}

	if !strings.Contains(text, "](/plugins/"+manifest.Id+"/decorate/") {
		t.Fatal("no decorator link in the examples post")
	}
}

// Between them the rows have to cover every registered decorator, or the post
// would silently stop mentioning one the day it was added.
func TestExamplesCoversEveryRegisteredDecorator(t *testing.T) {
	p := newTestPlugin(t, "https://example.com", true)

	text := runExamples(t, p).Text

	for _, d := range p.decorators.All() {
		if !strings.Contains(text, "/decorate/"+d.Type()+"?") {
			t.Errorf("the examples post has no row for the %q decorator", d.Type())
		}
	}
}

// The two live rows are the reason this command is worth running rather than
// reading: one opens inside the flash threshold and one counts up.
func TestExamplesLiveRowsStraddleTheThreshold(t *testing.T) {
	p := newTestPlugin(t, "https://example.com", true)

	before := time.Now().UTC()
	text := runExamples(t, p).Text
	after := time.Now().UTC()

	for _, live := range exampleLiveRows {
		t.Run(live.note, func(t *testing.T) {
			if !strings.Contains(text, " - "+live.note) {
				t.Fatalf("the %q row is missing:\n%s", live.note, text)
			}

			// The command reads its own clock, so a minute boundary can fall
			// between that and this test's. Either token is correct.
			earliest := dtg.FormatZulu(before.Add(live.offset))
			latest := dtg.FormatZulu(after.Add(live.offset))

			if !strings.Contains(text, "["+earliest+"](/plugins/") &&
				!strings.Contains(text, "["+latest+"](/plugins/") {
				t.Fatalf("no decorated link for %q or %q", earliest, latest)
			}
		})
	}

	// One either side of zero, which is what makes the pair worth having: a
	// countdown and a count-up. Read off the offsets rather than the rendered
	// text, so this still holds if the wording changes.
	var ahead, behind bool
	for _, live := range exampleLiveRows {
		ahead = ahead || live.offset > 0
		behind = behind || live.offset < 0
	}
	if !ahead || !behind {
		t.Fatalf("exampleLiveRows has no future row (%v) or no past row (%v)", ahead, behind)
	}
}

// The examples post is one post, and it stays one however long the install's subpath makes
// the links.
func TestExamplesFitsInOnePost(t *testing.T) {
	for _, siteURL := range []string{
		"https://example.com",
		"https://example.com/mattermost",
		"https://example.com/apps/collaboration/mattermost",
	} {
		t.Run(siteURL, func(t *testing.T) {
			p := newTestPlugin(t, siteURL, true)

			response := runExamples(t, p)
			if response.ResponseType != model.CommandResponseTypeInChannel {
				t.Fatalf("the examples post was declined on %s: %s", siteURL, response.Text)
			}

			if runes := utf8.RuneCountInString(response.Text); runes > safePostRunes {
				t.Fatalf("the examples post is %d runes, over the %d limit", runes, safePostRunes)
			}
		})
	}
}

// Past the limit the command says so, ephemerally, rather than letting the
// server refuse the post with an error nobody can act on.
//
// The subpath here is absurd, and that is the finding rather than a flaw in the
// test: examples measures itself against safePostRunes, the 4,000-rune floor,
// so the subpath needed to overflow it is long but not absurd, which is why
// this drives it with one. It is kept because the real limit cannot be read
// from the plugin API and safePostRunes is an assumption tracked by hand, and
// the day it is wrong is the day an ordinary post starts crossing it.
func TestExamplesRefusesRatherThanOverflowing(t *testing.T) {
	p := newTestPlugin(t, "https://example.com/"+strings.Repeat("longsubpath/", 2200), true)

	response := runExamples(t, p)

	if response.ResponseType != model.CommandResponseTypeEphemeral {
		t.Fatalf("ResponseType = %q, want the refusal to stay private", response.ResponseType)
	}
	if !strings.Contains(response.Text, "TF-16004") {
		t.Fatalf("the refusal does not carry its code:\n%s", response.Text)
	}
	if !strings.Contains(response.Text, "example-details") {
		t.Fatalf("the refusal does not point anywhere useful:\n%s", response.Text)
	}
}

// A row whose format is switched off has nothing to demonstrate, so it is left
// out rather than posted as a bare token beside rows that did become links.
func TestExamplesDropsRowsThatDoNotDecorate(t *testing.T) {
	p := newTestPlugin(t, "https://example.com", true)
	p.setConfiguration(&configuration{
		EnableLocation:         true,
		EnableLocationLatLon:   true,
		EnableLocationDDSigned: true,
		EnableLocationUSMTF:    true,
		EnableLocationMGRS:     true,
		EnableLocationUTM:      true,
		EnableLocationMoniker:  true,
	})

	text := runExamples(t, p).Text

	if strings.Contains(text, "/decorate/dtg?") {
		t.Fatalf("date-time groups are off but the examples post still links one:\n%s", text)
	}
	if !strings.Contains(text, "/decorate/location?") {
		t.Fatalf("coordinates are on but the examples post shows none:\n%s", text)
	}

	// And no row is left showing a token that never became a link.
	for line := range strings.SplitSeq(text, "\n") {
		if strings.HasPrefix(line, "- **") && !strings.Contains(line, "](/plugins/") {
			t.Errorf("undecorated row left in the examples post: %q", line)
		}
	}
}

// With nothing enabled there is nothing to post, and the refusal stays private:
// a channel post saying the plugin does nothing is worse than no post at all.
func TestExamplesWithEverythingOffRefusesPrivately(t *testing.T) {
	p := newTestPlugin(t, "https://example.com", false)

	response := runExamples(t, p)

	if response.ResponseType != model.CommandResponseTypeEphemeral {
		t.Fatalf("ResponseType = %q, want the refusal to stay private", response.ResponseType)
	}
	if !strings.Contains(response.Text, "TF-16003") {
		t.Fatalf("the refusal does not carry its code:\n%s", response.Text)
	}
}

func TestExamplesBeforeActivationSaysSo(t *testing.T) {
	p := newTestPlugin(t, "https://example.com", true)
	p.decorators = nil

	response := runExamples(t, p)

	if response.ResponseType != model.CommandResponseTypeEphemeral {
		t.Fatalf("ResponseType = %q, want ephemeral before activation", response.ResponseType)
	}
	if !strings.Contains(response.Text, "TF-16001") {
		t.Fatalf("the reply does not carry its code:\n%s", response.Text)
	}
}

// The examples post is decorated by the command rather than by the message hook, so it
// is already a set of links by the time it is posted. If an in-channel command
// response does reach MessageWillBePosted, running the hook over it must change
// nothing: a decorator link is a protected span, so Decorate is idempotent.
//
// Pinned because the alternative failure is a nested link written inside a real
// one, which is corruption rather than a cosmetic problem, and because whether
// that hook fires for a command response is not something this plugin controls.
func TestExamplesSurvivesTheMessageHook(t *testing.T) {
	p := newTestPlugin(t, "https://example.com", true)

	text := runExamples(t, p).Text

	if again := p.decoratePost(&model.Post{Message: text}, hookRef); again != nil {
		t.Fatalf("the hook rewrote an already-decorated examples post:\n%s", again.Message)
	}
}

// The examples post shows the ordinary shape of each grammar and nothing else. A near
// miss in here would be a permanent post telling a channel that something works
// when it is deliberately declined.
func TestExamplesShowsNothingThatIsDeclinedElsewhere(t *testing.T) {
	p := newTestPlugin(t, "https://example.com", true)
	tagger := &decorators.Tagger{Registry: p.decorators, URLPrefix: p.decorateURLPrefix()}

	declined := map[string]string{}
	for _, typ := range detailSetOrder {
		for _, group := range detailSets[typ].groups {
			if group.decorates {
				continue
			}
			for _, example := range group.examples {
				declined[example.text] = group.heading
			}
		}
	}

	for _, row := range exampleFixedRows {
		if heading, found := declined[row.text]; found {
			t.Errorf("the examples post shows %q, which examples lists under %q", row.text, heading)
		}

		// And it is genuinely a decoration rather than a row that happens to
		// survive because nothing tried.
		if tagger.Decorate(row.text, hookRef) == row.text {
			t.Errorf("the examples row %q is not decorated at all", row.text)
		}
	}
}
