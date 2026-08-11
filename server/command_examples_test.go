package main

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/plugin"

	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/decorators"
	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/decorators/dtg"
)

func TestExamplesCommand(t *testing.T) {
	p := newTestPlugin(t, "https://example.com", true)

	response, appErr := p.ExecuteCommand(&plugin.Context{}, &model.CommandArgs{
		Command: "/tactical-fusion examples",
	})
	if appErr != nil {
		t.Fatalf("ExecuteCommand returned an error: %v", appErr)
	}

	text := response.Text

	// Every example shows the typed text, inline, on its own line.
	for _, example := range decoratedExamples {
		if !strings.Contains(text, "- "+inlineCode(example.text)) {
			t.Fatalf("typed text for %q is missing", example.text)
		}
	}

	// ...and every one of them is followed by a real decorated link.
	//
	// Per example, not "somewhere in the output": a single passing row used to
	// cover for every other, which is how a whole format got into this list
	// while silently decorating into nothing.
	for _, example := range decoratedExamples {
		if !strings.Contains(text, inlineCode(example.text)+" → ") {
			t.Fatalf("%q is listed as decorated but the tagger left it alone", example.text)
		}
	}

	if !strings.Contains(text, "](/plugins/"+manifest.Id+"/decorate/dtg?") {
		t.Fatal("no decorated link in the output")
	}
}

// The declined rows have to be the tagger's genuine output. Hand-writing them
// would let the examples drift away from what the decorator actually does.
func TestExamplesShowRealTaggerOutput(t *testing.T) {
	p := newTestPlugin(t, "https://example.com", true)

	response, _ := p.ExecuteCommand(&plugin.Context{}, &model.CommandArgs{
		Command: "/tactical-fusion examples",
	})
	text := response.Text

	for _, example := range slices.Concat(rejectedExamples, skippedExamples) {
		t.Run(example.text, func(t *testing.T) {
			// No arrow, because nothing changed.
			if strings.Contains(text, inlineCode(example.text)+" →") {
				t.Fatalf("%q is shown as changed, but it is meant to be left alone", example.text)
			}
			// And it must not appear anywhere as a decorated link.
			if strings.Contains(text, "["+example.text+"](/plugins/") {
				t.Fatalf("%q was decorated, but this example is meant to be declined", example.text)
			}
		})
	}
}

// The response is a single message, so it has to fit inside one.
func TestExamplesFitInOneMessage(t *testing.T) {
	p := newTestPlugin(t, "https://example.com", true)

	response, _ := p.ExecuteCommand(&plugin.Context{}, &model.CommandArgs{
		Command: "/tactical-fusion examples",
	})

	if runes := utf8.RuneCountInString(response.Text); runes > maxPostRunes {
		t.Fatalf("examples output is %d runes, which exceeds the %d limit", runes, maxPostRunes)
	}
}

// The live examples are built from the moment the command runs, so the panel
// opens on a countdown that is actually counting.
func TestExamplesIncludeLiveTimes(t *testing.T) {
	p := newTestPlugin(t, "https://example.com", true)

	before := time.Now().UTC()
	response, _ := p.ExecuteCommand(&plugin.Context{}, &model.CommandArgs{
		Command: "/tactical-fusion examples",
	})
	after := time.Now().UTC()
	text := response.Text

	// Each must appear as a real decorated link, which only happens if the
	// generated text is something the decorator actually recognises.
	//
	// The command reads its own clock, so a minute boundary can fall between
	// that and this test's. Accepting either token keeps this from failing once
	// an hour for no reason.
	for _, live := range liveExampleOffsets {
		if !strings.Contains(text, "("+live.note+")") {
			t.Fatalf("output is missing the %q example", live.note)
		}

		earliest := dtg.FormatZulu(before.Add(live.offset))
		latest := dtg.FormatZulu(after.Add(live.offset))

		if !strings.Contains(text, "["+earliest+"](/plugins/") &&
			!strings.Contains(text, "["+latest+"](/plugins/") {
			t.Fatalf("no decorated link for %q or %q, generated from now%+s", earliest, latest, live.offset)
		}
	}
}

// The live examples lead, because they are the only ones whose panel opens on a
// countdown that is actually moving.
func TestLiveExamplesComeFirst(t *testing.T) {
	p := newTestPlugin(t, "https://example.com", true)

	response, _ := p.ExecuteCommand(&plugin.Context{}, &model.CommandArgs{
		Command: "/tactical-fusion examples",
	})
	text := response.Text

	live := strings.Index(text, "(5 minutes from now)")
	static := strings.Index(text, "(only the token is linked)")

	if live < 0 || static < 0 {
		t.Fatalf("missing an example: live=%d static=%d", live, static)
	}
	if live > static {
		t.Fatal("the static examples come before the live ones")
	}
}

// A DTG carries no seconds, so each example lands on the minute at or below its
// intended offset, and never further away than that.
//
// The reference is deliberately 45 seconds past the minute, which is the worst
// case for that truncation.
func TestLiveExamplesLandOnTheirOffset(t *testing.T) {
	ref := time.Date(2026, time.August, 9, 12, 0, 45, 0, time.UTC)
	examples := liveExamples(ref)

	if len(examples) != len(liveExampleOffsets) {
		t.Fatalf("got %d examples for %d offsets", len(examples), len(liveExampleOffsets))
	}

	for i, live := range liveExampleOffsets {
		t.Run(live.note, func(t *testing.T) {
			params, ok := (&dtg.Decorator{}).Parse(examples[i].text, ref)
			if !ok {
				t.Fatalf("the decorator declines the generated text %q", examples[i].text)
			}

			millis, err := strconv.ParseInt(params.Get("t"), 10, 64)
			if err != nil {
				t.Fatalf("unparsable instant: %v", err)
			}
			instant := time.UnixMilli(millis).UTC()

			target := ref.Add(live.offset)
			if instant.After(target) || !instant.After(target.Add(-time.Minute)) {
				t.Fatalf("instant %s is not within the minute below %s", instant, target)
			}

			// Truncation must never flip an example across the reference, or a
			// countdown would open counting the wrong way.
			if live.offset > 0 && !instant.After(ref) {
				t.Fatalf("instant %s should be ahead of %s", instant, ref)
			}
			if live.offset < 0 && !instant.Before(ref) {
				t.Fatalf("instant %s should be behind %s", instant, ref)
			}
		})
	}
}

// The standalone page is normally only reachable from a client that does not
// run the webapp, so the examples output ships a ready-made link that opts out
// of the sidebar.
func TestExamplesLinkToTheStandalonePage(t *testing.T) {
	p := newTestPlugin(t, "https://example.com", true)

	response, _ := p.ExecuteCommand(&plugin.Context{}, &model.CommandArgs{
		Command: "/tactical-fusion examples",
	})
	text := response.Text

	if !strings.Contains(text, decorators.ForcePageParam+"=1") {
		t.Fatalf("no force-page link in the output:\n%s", text)
	}
	if !strings.Contains(text, " as a page](/plugins/"+manifest.Id+"/decorate/dtg?") {
		t.Fatal("the force-page link does not point at a decorator page")
	}
}

// decliningDecorator matches an example but refuses to parse it. A decorator
// whose pattern is broader than its grammar is the normal case rather than a
// pathological one: the DTG decorator itself matches shapes that Parse then
// rejects, such as an impossible calendar date.
type decliningDecorator struct{}

func (*decliningDecorator) Type() string { return "declining" }

func (*decliningDecorator) Patterns() []decorators.Pattern {
	return []decorators.Pattern{{Regexp: regexp.MustCompile(`\d{6}[A-Z][A-Za-z]{3}\d{2}`)}}
}

func (*decliningDecorator) Parse(string, time.Time) (url.Values, bool) { return nil, false }

func (*decliningDecorator) RenderPage(http.ResponseWriter, url.Values) {}

// A decorator that matches an example but declines it must not consume the
// example: the search moves on to the next decorator rather than giving up and
// dropping the section. Otherwise registering a second decorator whose pattern
// happened to be broad would silently delete the standalone-page link from the
// examples output.
func TestStandalonePageLinkSkipsADecliningDecorator(t *testing.T) {
	registry, err := decorators.NewDefaultRegistry(&decliningDecorator{}, &dtg.Decorator{})
	if err != nil {
		t.Fatalf("failed to build the registry: %v", err)
	}
	tagger := &decorators.Tagger{Registry: registry, URLPrefix: "/plugins/test/decorate"}

	link, ok := standalonePageLink(registry, tagger, hookRef)
	if !ok {
		t.Fatal("standalonePageLink gave up after a declining decorator")
	}

	if !strings.Contains(link, "/decorate/dtg?") {
		t.Fatalf("link = %q, want it built by the decorator that accepted the example", link)
	}
	if strings.Contains(link, "/decorate/declining") {
		t.Fatalf("link = %q, want nothing from the declining decorator", link)
	}
	if !strings.Contains(link, decorators.ForcePageParam+"=1") {
		t.Fatalf("link = %q, want the force-page flag set", link)
	}
}

// With nothing able to parse an example there is no link to offer, and the
// section is left out rather than emitted empty.
func TestStandalonePageLinkGivesUpWhenNothingParses(t *testing.T) {
	registry, err := decorators.NewDefaultRegistry(&decliningDecorator{})
	if err != nil {
		t.Fatalf("failed to build the registry: %v", err)
	}
	tagger := &decorators.Tagger{Registry: registry, URLPrefix: "/plugins/test/decorate"}

	if link, ok := standalonePageLink(registry, tagger, hookRef); ok {
		t.Fatalf("standalonePageLink = %q, want no link when nothing parses", link)
	}

	var b strings.Builder
	writeStandalonePageSection(&b, registry, tagger, hookRef)
	if b.String() != "" {
		t.Fatalf("writeStandalonePageSection wrote %q, want nothing", b.String())
	}
}

// The page renders the same whether or not the flag is present: it is a webapp
// side instruction, and the server has no reason to care.
func TestForcePageParamIsIgnoredByThePage(t *testing.T) {
	p := newTestPlugin(t, "https://example.com", true)

	withFlag := httptest.NewRecorder()
	p.ServeHTTP(&plugin.Context{}, withFlag, httptest.NewRequest(http.MethodGet,
		"/decorate/dtg?t=1786293000000&dtg=091630ZAUG26&z=Z&a=&"+decorators.ForcePageParam+"=1", nil))

	if withFlag.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 with the force-page flag present", withFlag.Code)
	}
	if !strings.Contains(withFlag.Body.String(), "091630ZAUG26") {
		t.Fatal("the page did not render with the force-page flag present")
	}
}

func TestExamplesBeforeActivation(t *testing.T) {
	p := newTestPlugin(t, "https://example.com", true)
	p.decorators = nil

	response, appErr := p.ExecuteCommand(&plugin.Context{}, &model.CommandArgs{
		Command: "/tactical-fusion examples",
	})
	if appErr != nil {
		t.Fatalf("ExecuteCommand returned an error: %v", appErr)
	}
	if !strings.Contains(response.Text, "not registered yet") {
		t.Fatalf("expected a clear message before activation, got %q", response.Text)
	}
}

func TestExamplesNoteWhenDecorationIsOff(t *testing.T) {
	p := newTestPlugin(t, "https://example.com", false)

	response, _ := p.ExecuteCommand(&plugin.Context{}, &model.CommandArgs{
		Command: "/tactical-fusion examples",
	})

	if !strings.Contains(response.Text, "currently **off**") {
		t.Fatal("expected the output to say decoration is disabled")
	}
}

func TestUnknownSubcommandListsAvailable(t *testing.T) {
	p := newTestPlugin(t, "https://example.com", true)

	response, _ := p.ExecuteCommand(&plugin.Context{}, &model.CommandArgs{
		Command: "/tactical-fusion nope",
	})

	if !strings.Contains(response.Text, "examples") {
		t.Fatalf("unknown subcommand output does not mention examples: %q", response.Text)
	}
}
