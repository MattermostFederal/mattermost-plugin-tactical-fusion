package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/plugin"

	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/decorators"
	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/decorators/dtg"
	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/decorators/location"
)

// detailMessageFor is the part of the output covering one decorator.
//
// Sliced out of the joined text rather than picked out by message, because the
// whole set now lands in a single post with each decorator as a heading inside
// it. A section runs from its own "#### Name" to the next heading that names a
// different decorator, so a "(continued)" one keeps its rows with the section
// they belong to.
func detailMessageFor(t *testing.T, p *Plugin, typ string) string {
	t.Helper()

	want := capitalize(detailSets[typ].name)

	var found []string
	for section := range strings.SplitSeq(strings.Join(runDetails(t, p), "\n"), "#### ") {
		if strings.HasPrefix(section, want) {
			found = append(found, section)
		}
	}

	if len(found) == 0 {
		t.Fatalf("no examples for the %q decorator", typ)
	}

	return strings.Join(found, "")
}

// Every row does what the group it sits under says it does, and the whole list
// is checked rather than a sample of it.
//
// This is the test the example lists exist to be held to. A row is a claim
// about the decorator made in a place nothing else verifies, and the two ways
// it can be wrong are opposites: a "recognized" row the tagger silently leaves
// alone (which is how an entire format once sat in that list decorating
// nothing), and a "declined" row it happily rewrites (which would advertise a
// near miss as safe when it corrupts a message).
//
// Run against the posted output rather than against the tagger directly, so a
// row that decorates but is dropped before it reaches the reader still fails.
func TestEveryDetailDoesWhatItsGroupClaims(t *testing.T) {
	p := newTestPlugin(t, "https://example.com", true)

	for _, typ := range detailSetOrder {
		text := detailMessageFor(t, p, typ)

		for _, group := range detailSets[typ].groups {
			// A live group's rows come from the command's own clock, which this
			// test cannot reproduce without racing a minute boundary. They are
			// generated through the decorator's own formatter and are checked
			// against the posted output by TestDetailsIncludeLiveTimes.
			if group.live != nil {
				continue
			}

			for _, example := range group.examples {
				t.Run(typ+"/"+example.text, func(t *testing.T) {
					// The typed text is always shown, inline, on its own row.
					row, found := detailRowFor(text, example)
					if !found {
						t.Fatalf("typed text for %q is missing from the output", example.text)
					}

					// The arrow is only written when decoration changed
					// something, so its presence is the outcome. Read off the
					// row rather than the whole message: several rows are the
					// labeled form of a row that is declined bare, and
					// "LATD:35N079W" contains "35N079W".
					changed := strings.Contains(row, " → ")

					if group.decorates && !changed {
						t.Fatalf("%q is under %q but the tagger left it alone",
							example.text, group.heading)
					}
					if !group.decorates && changed {
						t.Fatalf("%q is under %q but the tagger rewrote it",
							example.text, group.heading)
					}

					// A declined row carries no link at all, which catches a
					// decoration that landed somewhere the arrow does not.
					if !group.decorates && strings.Contains(row, "](/plugins/") {
						t.Fatalf("%q was decorated, but %q says it is declined:\n%s",
							example.text, group.heading, row)
					}
				})
			}
		}
	}
}

// detailRowFor is the single output line an example was rendered on.
//
// Scoped to the row because the output deliberately contains an example beside
// its own labeled variant, and a whole-message substring search cannot tell
// "35N079W was decorated" from "LATD:35N079W was, and contains it".
func detailRowFor(text string, example detailExample) (string, bool) {
	prefix := "- " + inlineCode(example.text)

	for line := range strings.SplitSeq(text, "\n") {
		if strings.HasPrefix(line, prefix) {
			return line, true
		}
	}

	return "", false
}

// Every group is worth having: an empty one would render a heading with nothing
// under it, and a heading is a claim about rows that are not there.
func TestEveryDetailGroupHasRows(t *testing.T) {
	ref := time.Now().UTC()

	for _, typ := range detailSetOrder {
		set := detailSets[typ]
		if len(set.groups) == 0 {
			t.Errorf("the %q set has no groups", typ)
		}

		for _, group := range set.groups {
			if group.heading == "" {
				t.Errorf("a group in the %q set has no heading", typ)
			}
			if len(groupDetailExamples(group, ref)) == 0 {
				t.Errorf("group %q in the %q set has no rows", group.heading, typ)
			}
		}
	}
}

func TestDetailsLinkToADecoratorPage(t *testing.T) {
	p := newTestPlugin(t, "https://example.com", true)

	text := detailMessageFor(t, p, dtg.Type)

	if !strings.Contains(text, "](/plugins/"+manifest.Id+"/decorate/dtg?") {
		t.Fatal("no decorated link in the output")
	}
}

// The live examples are built from the moment the command runs, so the panel
// opens on a countdown that is actually counting.
func TestDetailsIncludeLiveTimes(t *testing.T) {
	p := newTestPlugin(t, "https://example.com", true)

	before := time.Now().UTC()
	text := detailMessageFor(t, p, dtg.Type)
	after := time.Now().UTC()

	// Each must appear as a real decorated link, which only happens if the
	// generated text is something the decorator actually recognizes.
	//
	// The command reads its own clock, so a minute boundary can fall between
	// that and this test's. Accepting either token keeps this from failing once
	// an hour for no reason.
	for _, live := range liveDetailOffsets {
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
func TestLiveDetailsComeFirst(t *testing.T) {
	p := newTestPlugin(t, "https://example.com", true)

	text := detailMessageFor(t, p, dtg.Type)

	live := strings.Index(text, "("+liveDetailOffsets[0].note+")")
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
func TestLiveDetailsLandOnTheirOffset(t *testing.T) {
	ref := time.Date(2026, time.August, 9, 12, 0, 45, 0, time.UTC)
	examples := liveDetailExamples(ref)

	if len(examples) != len(liveDetailOffsets) {
		t.Fatalf("got %d examples for %d offsets", len(examples), len(liveDetailOffsets))
	}

	for i, live := range liveDetailOffsets {
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
func TestDetailsLinkToTheStandalonePage(t *testing.T) {
	p := newTestPlugin(t, "https://example.com", true)

	text := detailMessageFor(t, p, dtg.Type)

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

	link, ok := standalonePageLink(registry, tagger, hookRef, setDetailExamples(detailSets[dtg.Type], true, hookRef))
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

	from := setDetailExamples(detailSets[dtg.Type], true, hookRef)
	if link, ok := standalonePageLink(registry, tagger, hookRef, from); ok {
		t.Fatalf("standalonePageLink = %q, want no link when nothing parses", link)
	}

	// And the section it would have gone in is dropped with it, rather than
	// posting a heading over nothing.
	p := newTestPlugin(t, "https://example.com", true)
	p.decorators = registry

	for _, chunk := range p.detailChunks(detailSets[dtg.Type], tagger, hookRef) {
		if strings.Contains(chunk.heading, "Standalone page") {
			t.Fatalf("the standalone page section was written with no link in it: %+v", chunk)
		}
	}
}

// The page renders the same whether or not the flag is present: it is a webapp
// side instruction, and the server has no reason to care.
func TestForcePageParamIsIgnoredByThePage(t *testing.T) {
	p := newTestPlugin(t, "https://example.com", true)

	withFlag := httptest.NewRecorder()
	p.ServeHTTP(&plugin.Context{}, withFlag, withSession(httptest.NewRequest(http.MethodGet,
		"/decorate/dtg?t=1786293000000&dtg=091630ZAUG26&z=Z&a=&"+decorators.ForcePageParam+"=1", nil)))

	if withFlag.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 with the force-page flag present", withFlag.Code)
	}
	if !strings.Contains(withFlag.Body.String(), "091630ZAUG26") {
		t.Fatal("the page did not render with the force-page flag present")
	}
}

func TestDetailsBeforeActivation(t *testing.T) {
	p := newTestPlugin(t, "https://example.com", true)
	p.decorators = nil

	response, appErr := p.ExecuteCommand(&plugin.Context{}, &model.CommandArgs{
		Command: "/tactical-fusion example-details",
	})
	if appErr != nil {
		t.Fatalf("ExecuteCommand returned an error: %v", appErr)
	}
	if !strings.Contains(response.Text, "not registered yet") {
		t.Fatalf("expected a clear message before activation, got %q", response.Text)
	}
}

func TestDetailsNoteWhenDecorationIsOff(t *testing.T) {
	p := newTestPlugin(t, "https://example.com", false)

	// Unlike the short examples post, this one still goes out with every row
	// shown as typed: it is the reference, and a reference that vanishes when
	// decoration is off cannot tell anybody that decoration is off.
	all := strings.Join(runDetails(t, p), "\n")

	if !strings.Contains(all, "currently **off**") {
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

// The whole set goes to the channel, and nothing goes out ephemerally.
func TestDetailsPostToTheChannel(t *testing.T) {
	p := newTestPlugin(t, "https://example.com", true)

	messages := runDetails(t, p)
	if len(messages) == 0 {
		t.Fatal("nothing was posted")
	}

	api := p.API.(*fakeAPI)
	if len(api.ephemeral) != 0 {
		t.Errorf("sent %d ephemeral posts; the posts are the whole output", len(api.ephemeral))
	}
}

// The bug this fell over on: a server that refuses the packing.
//
// The post size limit cannot be read from the plugin API, so the budget is a
// guess, and the first guess was wrong by enough to be refused outright with
// "Post Message property is longer than the maximum permitted length". The
// command has to survive that rather than report it, because a reader cannot
// act on it and the fix is entirely on this side.
//
// Pinned at Mattermost's real default rather than at whatever this build
// guesses, so tightening the guess cannot make this pass for the wrong reason.
func TestDetailsPostWhateverTheServerAccepts(t *testing.T) {
	// V2 is the ordinary default and the first budget is sized for it. V1 is the
	// floor, and the only other rung: Mattermost's own stores never report less,
	// so a ladder below it would be inventing servers that do not exist.
	for _, limit := range []int{model.PostMessageMaxRunesV2, 5000, model.PostMessageMaxRunesV1} {
		t.Run(fmt.Sprintf("limit %d", limit), func(t *testing.T) {
			p := newTestPlugin(t, "https://example.com", true)
			api := p.API.(*fakeAPI)
			api.postSizeLimit = limit

			messages := runDetails(t, p)

			// runDetails reads api.created, and CreatePost appends to `refused`
			// and returns BEFORE appending there, so an over-limit message can
			// never appear in that slice. Checking its length was therefore a
			// tautology. What actually has to hold is that everything the
			// command tried to post was accepted.
			if len(api.refused) > 1 {
				t.Errorf("the server refused %d posts (%v); at most the first budget's "+
					"first post may be refused before the retry", len(api.refused), api.refused)
			}
			for i, message := range messages {
				if runes := len([]rune(message)); runes > limit {
					t.Errorf("message %d of %d is %d runes, over the server's %d",
						i+1, len(messages), runes, limit)
				}
			}

			// Nothing is dropped in the retry: the smaller packing is a
			// repacking of the same set, not a truncation of it.
			all := strings.Join(messages, "")
			for _, typ := range detailSetOrder {
				if !strings.Contains(all, "#### "+capitalize(detailSets[typ].name)) {
					t.Errorf("the %q section went missing", typ)
				}
			}
		})
	}
}

// A refused first post costs nothing, because nothing has been written yet.
//
// Every message in a packing shares one budget, so the first is the canary: if
// it fits the rest do. That is what makes repacking smaller and starting again
// safe, and why the retry is there rather than around the ones that follow.
func TestDetailsRetryLeavesNothingHalfPosted(t *testing.T) {
	p := newTestPlugin(t, "https://example.com", true)
	api := p.API.(*fakeAPI)
	api.postSizeLimit = 5000

	runDetails(t, p)

	if len(api.refused) == 0 {
		t.Fatal("the first packing was accepted, so this test proved nothing")
	}

	// Exactly one refusal: the first budget's first post. Nothing else was attempted
	// at that size, so no partial set was left behind.
	if len(api.refused) != 1 {
		t.Errorf("%d posts were refused, want only the first: %v", len(api.refused), api.refused)
	}

	// And an operator can see which budget the server took.
	if len(api.warnings) == 0 {
		t.Error("the refusal was not logged")
	}
}

// A first post that cannot be created has written nothing, so the reader gets
// the reason rather than silence.
func TestDetailsReportWhenNothingCanBePosted(t *testing.T) {
	p := newTestPlugin(t, "https://example.com", true)
	api := p.API.(*fakeAPI)
	api.createPostErr = model.NewAppError("CreatePost", "id", nil, "channel is archived", 403)

	response, appErr := p.ExecuteCommand(&plugin.Context{}, &model.CommandArgs{
		Command: "/tactical-fusion example-details",
	})
	if appErr != nil {
		t.Fatalf("ExecuteCommand returned an error: %v", appErr)
	}

	if response.ResponseType != model.CommandResponseTypeEphemeral {
		t.Fatalf("ResponseType = %q, want the failure to stay private", response.ResponseType)
	}
	if !strings.Contains(response.Text, "TF-16006") {
		t.Fatalf("the failure does not carry its code:\n%s", response.Text)
	}
	if len(api.warnings) == 0 {
		t.Error("nothing was logged, so an operator has no record of it")
	}
}

// A post that fails costs its own examples and nothing else.
//
// The first one is already in the channel by then, so abandoning the rest would
// leave the set stopping without saying why. Every remaining message is still
// attempted, and the reader is told how much is missing.
//
// Driven through postRemainingDetails with messages of its own, because the
// real set is two messages and the loop would otherwise barely run. Deleting it
// instead is the wrong trade: the limit it exists for is a guess.
func TestDetailsKeepGoingWhenAPostFails(t *testing.T) {
	p := newTestPlugin(t, "https://example.com", true)
	api := p.API.(*fakeAPI)

	messages := []string{"first", "second", "third", "fourth"}
	if _, appErr := p.API.CreatePost(&model.Post{
		UserId: "user1", ChannelId: "channel1", Message: messages[0],
	}); appErr != nil {
		t.Fatalf("the first post failed: %v", appErr)
	}

	// Refuse from here on, so the first has landed and only the rest fail.
	api.createPostErr = model.NewAppError("CreatePost", "id", nil, "rate limited", 429)
	api.createPostFailFrom = 2

	response := p.postRemainingDetails(
		&model.CommandArgs{UserId: "user1", ChannelId: "channel1"}, messages)

	if len(api.created) != 2 {
		t.Fatalf("created %d posts, want the first and one more", len(api.created))
	}
	if response.ResponseType != model.CommandResponseTypeEphemeral {
		t.Fatalf("ResponseType = %q, want the warning to stay private", response.ResponseType)
	}
	if !strings.Contains(response.Text, "TF-16006") {
		t.Fatalf("the warning does not carry its code:\n%s", response.Text)
	}
	if !strings.Contains(response.Text, "2 of 4") {
		t.Fatalf("the warning does not say how much is missing:\n%s", response.Text)
	}

	// One log line per refused post, not one for the batch: an operator chasing
	// a rate limit wants to know how many times it fired.
	if len(api.errors) != 2 {
		t.Errorf("logged %d errors for 2 refused posts", len(api.errors))
	}
}

// Every message is a post of its own, and none of them is a reply.
//
// Each is a reference somebody comes back to. A reply is filed under the post
// above it and read as a remark about it, which the coordinate examples are not,
// and it buries them a level down for no gain. Pinned because RootId is one
// field away from being set and nothing else in the suite would notice.
func TestDetailsAreSeparatePostsNotAThread(t *testing.T) {
	p := newTestPlugin(t, "https://example.com", true)

	runDetails(t, p)

	api := p.API.(*fakeAPI)
	if len(api.created) < 2 {
		t.Fatalf("created %d posts, want one per decorator", len(api.created))
	}

	for i, post := range api.created {
		if post.RootId != "" {
			t.Errorf("post %d has RootId %q, so it is a reply", i+1, post.RootId)
		}
	}
}

// Everything these posts carry is already decorated, so running the message
// hook over any of it must change nothing. Whether an API-created post reaches
// MessageWillBePosted is not something this plugin controls, and the failure it
// would cause is a nested link inside a real one.
func TestDetailsSurviveTheMessageHook(t *testing.T) {
	p := newTestPlugin(t, "https://example.com", true)

	for i, message := range runDetails(t, p) {
		if again := p.decoratePost(&model.Post{Message: message}, hookRef); again != nil {
			t.Fatalf("the hook rewrote post %d:\n%s", i+1, again.Message)
		}
	}
}

// Every row in the inline group does what it claims, in both directions.
//
// Scoped to that group because it is the only one making the claim: most single
// coordinates elsewhere in the list would also draw a map, and marking each of
// them would turn an incidental property into thirty claims nobody reads. The
// second loop is what keeps that scoping honest, by refusing the flag anywhere
// the test does not check it.
//
// Checked through decoratePost, the function that actually stamps a post,
// rather than by re-reading the rule out of the tagger: a row saying "posting
// this draws a map" should fail if anything on that path stops stamping.
func TestEveryInlineDetailDrawsWhatItClaims(t *testing.T) {
	p := newTestPlugin(t, "https://example.com", true)

	claims := 0
	for _, group := range locationDetailGroups {
		if group.heading != inlineDetailHeading {
			continue
		}

		for _, example := range group.examples {
			t.Run(example.text, func(t *testing.T) {
				got := p.decoratePost(&model.Post{Message: example.text}, hookRef)

				stamped := got != nil && got.Type != ""
				if stamped != example.inline {
					t.Fatalf("posting %q on its own stamps=%v, the row claims inline=%v",
						example.text, stamped, example.inline)
				}
			})

			if example.inline {
				claims++
			}
		}
	}

	if claims == 0 {
		t.Fatalf("no row under %q claims an inline rendering, so this asserts nothing",
			inlineDetailHeading)
	}

	for _, typ := range detailSetOrder {
		for _, group := range detailSets[typ].groups {
			if group.heading == inlineDetailHeading {
				continue
			}
			for _, example := range group.examples {
				if example.inline {
					t.Errorf("%q under %q sets inline, which nothing checks outside %q",
						example.text, group.heading, inlineDetailHeading)
				}
			}
		}
	}
}

// The group exists, is reachable in the posted output, and says which way round
// each of its rows goes.
func TestDetailsExplainTheInlineMap(t *testing.T) {
	p := newTestPlugin(t, "https://example.com", true)

	text := detailMessageFor(t, p, location.Type)

	if !strings.Contains(text, inlineDetailHeading) {
		t.Fatalf("the posted coordinates examples carry no %q group", inlineDetailHeading)
	}

	var drawn, notDrawn int
	for _, group := range locationDetailGroups {
		if group.heading != inlineDetailHeading {
			continue
		}
		for _, example := range group.examples {
			if _, found := detailRowFor(text, example); !found {
				t.Fatalf("typed text for %q is missing from the output", example.text)
			}
			if example.inline {
				drawn++
			} else {
				notDrawn++
			}
		}
	}

	// Both halves, or the group teaches a rule without showing its edge.
	if drawn == 0 || notDrawn == 0 {
		t.Fatalf("the %q group has %d drawn and %d not; it needs both to be a distinction",
			inlineDetailHeading, drawn, notDrawn)
	}
}

// capitalize raises a set name for its heading, and an empty name has no first
// byte to raise. Every set has a name, so only a direct call reaches the guard.
func TestCapitalize(t *testing.T) {
	for _, tc := range []struct {
		name string
		in   string
		want string
	}{
		{"a lower-case name", "coordinates", "Coordinates"},
		{"one already raised", "Coordinates", "Coordinates"},
		{"a single letter", "c", "C"},
		{"a name that starts with a digit", "3d", "3d"},
		{"empty", "", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := capitalize(tc.in); got != tc.want {
				t.Fatalf("capitalize(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestPackChunksProducesNoMessageWhenThereIsNothingToSay(t *testing.T) {
	for _, tc := range []struct {
		name   string
		chunks []detailChunk
	}{
		{"no chunks at all", nil},
		{"an empty slice", []detailChunk{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if messages := packChunks(tc.chunks, defaultPostRunes); len(messages) != 0 {
				t.Errorf("packed %d messages, want none", len(messages))
			}
		})
	}
}
