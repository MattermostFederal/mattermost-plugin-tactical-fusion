package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/plugin"

	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/decorators"
	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/decorators/dtg"
)

var hookRef = time.Date(2026, time.August, 9, 12, 0, 0, 0, time.UTC)

// fakeAPI implements only the handful of plugin.API methods this plugin uses.
//
// The embedded interface is deliberately nil: any call the code makes that is
// not stubbed here panics, which surfaces unexpected API use instead of quietly
// returning a zero value. Hand-rolling this also keeps testify/mock, and the
// dependency it drags in, out of the module.
type fakeAPI struct {
	plugin.API

	config   *model.Config
	warnings []string
	errors   []string

	// kv is the fake plugin KV store. Nil until a test needs one, so a test
	// that does not touch preferences still panics on an unexpected KV call.
	kv map[string][]byte

	// kvGetErr, kvSetErr and kvDeleteErr force the corresponding failure.
	kvGetErr    *model.AppError
	kvSetErr    *model.AppError
	kvDeleteErr *model.AppError

	// published records every cluster event, so a test can prove that saving
	// tells the other nodes to drop their copy.
	published []model.PluginClusterEvent

	// publishErr forces publication to fail, which must not fail the save.
	publishErr error

	// commands records what OnActivate registered, and registerErr forces the
	// registration to fail.
	commands    []*model.Command
	registerErr error

	// loadedConfig is the JSON OnConfigurationChange unmarshals into the
	// destination it is handed, and loadErr forces the load to fail.
	loadedConfig string
	loadErr      error
}

func (a *fakeAPI) GetConfig() *model.Config { return a.config }

func (a *fakeAPI) LogWarn(msg string, _ ...any) { a.warnings = append(a.warnings, msg) }

func (a *fakeAPI) LogError(msg string, _ ...any) { a.errors = append(a.errors, msg) }

func (a *fakeAPI) KVGet(key string) ([]byte, *model.AppError) {
	if a.kvGetErr != nil {
		return nil, a.kvGetErr
	}
	return a.kv[key], nil
}

func (a *fakeAPI) KVSet(key string, value []byte) *model.AppError {
	if a.kvSetErr != nil {
		return a.kvSetErr
	}
	if a.kv == nil {
		a.kv = map[string][]byte{}
	}
	a.kv[key] = value
	return nil
}

func (a *fakeAPI) KVDelete(key string) *model.AppError {
	if a.kvDeleteErr != nil {
		return a.kvDeleteErr
	}
	delete(a.kv, key)
	return nil
}

func (a *fakeAPI) PublishPluginClusterEvent(ev model.PluginClusterEvent, _ model.PluginClusterEventSendOptions) error {
	a.published = append(a.published, ev)
	return a.publishErr
}

func (a *fakeAPI) RegisterCommand(command *model.Command) error {
	if a.registerErr != nil {
		return a.registerErr
	}
	a.commands = append(a.commands, command)
	return nil
}

// LoadPluginConfiguration fills the destination from loadedConfig.
//
// The real one unmarshals the admin console's saved JSON into whatever it is
// handed, so this does the same rather than assigning a prepared struct: that
// way a field the manifest declares but the struct cannot receive fails here
// exactly as it would on a server.
func (a *fakeAPI) LoadPluginConfiguration(dest any) error {
	if a.loadErr != nil {
		return a.loadErr
	}

	blob := a.loadedConfig
	if blob == "" {
		blob = "{}"
	}

	return json.Unmarshal([]byte(blob), dest)
}

// newTestPlugin returns a plugin wired to a fake API, with the DTG decorator
// registered so tests do not depend on OnActivate having run.
func newTestPlugin(t *testing.T, siteURL string, enabled bool) *Plugin {
	t.Helper()

	config := &model.Config{}
	config.SetDefaults()
	config.ServiceSettings.SiteURL = model.NewPointer(siteURL)

	p := &Plugin{}
	p.SetAPI(&fakeAPI{config: config})
	p.setConfiguration(&configuration{
		EnableDTG:          enabled,
		EnableDTGMilitary:  true,
		EnableDTGMoniker:   true,
		EnableDTGTimestamp: true,
	})

	registerDTGForTest(t, p)

	return p
}

// registerDTGForTest builds the plugin's decorator registry, mirroring what
// OnActivate does. Each plugin owns its own, so tests never share state.
func registerDTGForTest(t *testing.T, p *Plugin) {
	t.Helper()
	registry, err := decorators.NewDefaultRegistry(&dtg.Decorator{Enabled: p.dtgFormats})
	if err != nil {
		t.Fatalf("failed to build the decorator registry: %v", err)
	}
	p.decorators = registry
}

func TestDecoratePostRewritesMessage(t *testing.T) {
	p := newTestPlugin(t, "https://example.com", true)

	got := p.decoratePost(&model.Post{Message: "ARCT 091630ZAUG26 confirmed"}, hookRef)
	if got == nil {
		t.Fatal("decoratePost returned nil, want a rewritten post")
	}

	instant := time.Date(2026, time.August, 9, 16, 30, 0, 0, time.UTC).UnixMilli()
	want := "ARCT [091630ZAUG26](/plugins/" + manifest.Id +
		"/decorate/dtg?a=&dtg=091630ZAUG26&t=" + strconv.FormatInt(instant, 10) + "&z=Z) confirmed"
	if got.Message != want {
		t.Fatalf("message\n got: %s\nwant: %s", got.Message, want)
	}
}

func TestDecoratePostCarriesSubpathButNeverHost(t *testing.T) {
	p := newTestPlugin(t, "https://example.com/mattermost", true)

	got := p.decoratePost(&model.Post{Message: "091630ZAUG26"}, hookRef)
	if got == nil {
		t.Fatal("decoratePost returned nil, want a rewritten post")
	}

	if !strings.Contains(got.Message, "(/mattermost/plugins/") {
		t.Fatalf("message = %q, want the SiteURL subpath carried into the link", got.Message)
	}
	if strings.Contains(got.Message, "example.com") || strings.Contains(got.Message, "https://") {
		t.Fatalf("message = %q, want no scheme or host in a stored URL", got.Message)
	}
}

func TestDecoratePostReturnsNilWhenNothingMatches(t *testing.T) {
	p := newTestPlugin(t, "https://example.com", true)

	if got := p.decoratePost(&model.Post{Message: "nothing to decorate here"}, hookRef); got != nil {
		t.Fatalf("decoratePost = %+v, want nil when the message is unchanged", got)
	}
}

func TestDecoratePostSkippedWhenDisabled(t *testing.T) {
	p := newTestPlugin(t, "https://example.com", false)

	if got := p.decoratePost(&model.Post{Message: "091630ZAUG26"}, hookRef); got != nil {
		t.Fatal("decoratePost decorated while the decorator was switched off")
	}
}

// SiteURL is only consulted for a subpath. Since no host is stored, a
// root-relative link resolves against whatever server the reader is on, so an
// unset or malformed SiteURL just means "no subpath" and decoration continues.
func TestDecoratePostWorksWithoutUsableSiteURL(t *testing.T) {
	cases := []struct {
		name    string
		siteURL string
	}{
		{"unset", ""},
		{"whitespace only", "   "},
		{"unparsable", "://not a url"},
		{"no scheme, so the path is not rooted", "example.com/mm"},
		{"host only", "https://example.com"},
		{"host with a trailing slash", "https://example.com/"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := newTestPlugin(t, tc.siteURL, true)

			got := p.decoratePost(&model.Post{Message: "091630ZAUG26"}, hookRef)
			if got == nil {
				t.Fatal("decoratePost skipped; a missing subpath is not a reason to skip")
			}

			want := "(/plugins/" + manifest.Id + "/decorate/dtg?"
			if !strings.Contains(got.Message, want) {
				t.Fatalf("message = %q, want a root-relative link starting %q", got.Message, want)
			}
		})
	}
}

// A path that is not rooted must be ignored rather than emitted, or the stored
// URL becomes relative to whatever page the reader is on.
func TestDecoratePostNeverEmitsARelativePrefix(t *testing.T) {
	p := newTestPlugin(t, "example.com/mm", true)

	got := p.decoratePost(&model.Post{Message: "091630ZAUG26"}, hookRef)
	if got == nil {
		t.Fatal("decoratePost returned nil")
	}
	if strings.Contains(got.Message, "example.com") {
		t.Fatalf("message = %q, want no host fragment in the stored URL", got.Message)
	}
	if !strings.Contains(got.Message, "](/plugins/") {
		t.Fatalf("message = %q, want the link destination to start at the root", got.Message)
	}
}

func TestDecoratePostSkipsSystemMessages(t *testing.T) {
	p := newTestPlugin(t, "https://example.com", true)

	post := &model.Post{Message: "091630ZAUG26", Type: model.PostTypeJoinChannel}
	if got := p.decoratePost(post, hookRef); got != nil {
		t.Fatal("decoratePost decorated a system message")
	}
}

// The deny list is narrow on purpose: custom post types from integrations and
// other plugins may carry real mission content.
func TestDecoratePostDecoratesCustomPostTypes(t *testing.T) {
	p := newTestPlugin(t, "https://example.com", true)

	post := &model.Post{Message: "091630ZAUG26", Type: "custom_something"}
	if got := p.decoratePost(post, hookRef); got == nil {
		t.Fatal("decoratePost skipped a custom post type, want it decorated")
	}
}

// A short message can cross the limit once every token grows tenfold. Losing
// decoration is better than the author seeing an opaque "post too long".
func TestDecoratePostSkippedWhenItWouldExceedMaxPostSize(t *testing.T) {
	p := newTestPlugin(t, "https://example.com", true)

	// Well under the limit as typed, far over it once decorated.
	message := strings.TrimSpace(strings.Repeat("091630ZAUG26 ", 60))
	if len([]rune(message)) >= maxPostRunes {
		t.Fatalf("test message is already %d runes, which does not exercise the guard", len([]rune(message)))
	}

	if got := p.decoratePost(&model.Post{Message: message}, hookRef); got != nil {
		t.Fatal("decoratePost returned a post that would exceed the maximum size")
	}
}

func TestDecoratePostLeavesOriginalPostUntouched(t *testing.T) {
	p := newTestPlugin(t, "https://example.com", true)

	original := &model.Post{Message: "091630ZAUG26"}
	got := p.decoratePost(original, hookRef)

	if got == nil {
		t.Fatal("decoratePost returned nil, want a rewritten post")
	}
	if original.Message != "091630ZAUG26" {
		t.Fatalf("the input post was mutated: %q", original.Message)
	}
}

func TestDecoratePostHandlesNilAndEmpty(t *testing.T) {
	p := newTestPlugin(t, "https://example.com", true)

	if got := p.decoratePost(nil, hookRef); got != nil {
		t.Fatal("decoratePost(nil) returned a post")
	}
	if got := p.decoratePost(&model.Post{Message: ""}, hookRef); got != nil {
		t.Fatal("decoratePost on an empty message returned a post")
	}
}

func TestMessageWillBePostedNeverRejects(t *testing.T) {
	p := newTestPlugin(t, "https://example.com", true)

	_, reason := p.MessageWillBePosted(&plugin.Context{}, &model.Post{Message: "091630ZAUG26"})
	if reason != "" {
		t.Fatalf("MessageWillBePosted returned rejection reason %q, want none", reason)
	}
}

// A panic in the tagger must leave the post alone rather than stopping somebody
// from posting.
func TestDecoratePostRecoversFromPanic(t *testing.T) {
	p := newTestPlugin(t, "https://example.com", true)

	registry, err := decorators.NewDefaultRegistry(&panicDecorator{})
	if err != nil {
		t.Fatalf("failed to build the registry: %v", err)
	}
	p.decorators = registry

	got := p.decoratePost(&model.Post{Message: "boom"}, hookRef)
	if got != nil {
		t.Fatalf("decoratePost = %+v, want nil after a panic", got)
	}
}

// Edits are passed through verbatim. This asserts the decision structurally, so
// re-adding the hook cannot happen by accident: doing so would reintroduce the
// unwrap-and-re-decorate problem the design deliberately removed.
func TestPluginHasNoMessageWillBeUpdatedHook(t *testing.T) {
	if _, exists := reflect.TypeFor[*Plugin]().MethodByName("MessageWillBeUpdated"); exists {
		t.Fatal("Plugin implements MessageWillBeUpdated; edits must be stored verbatim, see the plan's revision 2c")
	}
}

// withFormats returns a plugin configured with exactly these switches, so a
// test controls every one of them including the parent.
func withFormats(t *testing.T, config configuration) *Plugin {
	t.Helper()

	p := newTestPlugin(t, "https://example.com", true)
	p.setConfiguration(&config)

	return p
}

func decorated(p *Plugin, message string) string {
	got := p.decoratePost(&model.Post{Message: message}, hookRef)
	if got == nil {
		return message
	}
	return got.Message
}

// Each format is switched independently, and the ones left on keep working.
func TestFormatSwitchesActIndependently(t *testing.T) {
	const (
		military  = "091630ZAUG26"
		labelled  = "DTG: 091630ZAUG26"
		timestamp = "2026-08-09T16:30:00Z"
	)

	cases := []struct {
		name   string
		config configuration
		linked []string
		left   []string
	}{
		{
			"all on",
			configuration{EnableDTG: true, EnableDTGMilitary: true, EnableDTGMoniker: true, EnableDTGTimestamp: true},
			[]string{military, labelled, timestamp},
			nil,
		},
		{
			"military off",
			configuration{EnableDTG: true, EnableDTGMoniker: true, EnableDTGTimestamp: true},
			[]string{timestamp},

			// The moniker is on, but it has nothing to label here: turning a
			// format off has to stop the labelled form of it too.
			[]string{military, labelled},
		},
		{
			"timestamps off",
			configuration{EnableDTG: true, EnableDTGMilitary: true, EnableDTGMoniker: true},
			[]string{military, labelled},
			[]string{timestamp},
		},
		{
			"moniker off",
			configuration{EnableDTG: true, EnableDTGMilitary: true, EnableDTGTimestamp: true},
			[]string{military, timestamp},
			nil,
		},
		{
			"everything below the parent off",
			configuration{EnableDTG: true},
			nil,
			[]string{military, labelled, timestamp},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := withFormats(t, tc.config)

			for _, message := range tc.linked {
				if got := decorated(p, message); got == message {
					t.Fatalf("%q was left alone, want a link", message)
				}
			}
			for _, message := range tc.left {
				if got := decorated(p, message); got != message {
					t.Fatalf("%q was rewritten to %q, want it left alone", message, got)
				}
			}
		})
	}
}

// With the moniker off the label stays as text, and the token inside it is
// still decorated in its own right.
func TestMonikerOffLeavesTheLabelButKeepsTheToken(t *testing.T) {
	p := withFormats(t, configuration{EnableDTG: true, EnableDTGMilitary: true})

	got := decorated(p, "DTG: 091630ZAUG26")

	if !strings.Contains(got, "DTG: [091630ZAUG26](") {
		t.Fatalf("message = %q, want the label kept and the token linked", got)
	}
}

// The parent switch overrides every format below it.
func TestParentSwitchOverridesTheFormats(t *testing.T) {
	p := withFormats(t, configuration{
		EnableDTGMilitary:  true,
		EnableDTGMoniker:   true,
		EnableDTGTimestamp: true,
	})

	for _, message := range []string{"091630ZAUG26", "DTG: 091630ZAUG26", "2026-08-09T16:30:00Z"} {
		if got := decorated(p, message); got != message {
			t.Fatalf("%q was rewritten to %q while the decorator was off", message, got)
		}
	}
}

// Switching a format off must not break the links already written into
// messages, which is why the page never consults the configuration.
func TestPagesStillRenderForADisabledFormat(t *testing.T) {
	p := withFormats(t, configuration{EnableDTG: false})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet,
		"/decorate/dtg?t=1786293000000&dtg=091630ZAUG26&z=Z&a=", nil)

	p.ServeHTTP(&plugin.Context{}, rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: an old link must keep working", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "091630ZAUG26") {
		t.Fatal("the page did not render the DTG")
	}
}

// referenceTime decides the month and year a short form such as 091630Z gets,
// so it is worth pinning rather than leaving to whichever branch a test happens
// to take.
func TestReferenceTime(t *testing.T) {
	created := time.Date(2026, time.March, 4, 9, 15, 0, 0, time.UTC)

	t.Run("an existing CreateAt is honoured", func(t *testing.T) {
		// An imported or scheduled post carries its real timestamp, and
		// resolving against "now" would date it wrong by however long ago it
		// was written.
		got := referenceTime(&model.Post{CreateAt: model.GetMillis()})
		if got.IsZero() {
			t.Fatal("referenceTime returned the zero time")
		}

		got = referenceTime(&model.Post{CreateAt: created.UnixMilli()})
		if !got.Equal(created) {
			t.Fatalf("referenceTime = %v, want %v", got, created)
		}
		if got.Location() != time.UTC {
			t.Fatalf("referenceTime returned %v, want a UTC time", got.Location())
		}
	})

	// CreateAt is normally unset at this point: the server fills it in after
	// the hook runs, so "now" is the right answer for an ordinary new post.
	t.Run("falls back to now", func(t *testing.T) {
		before := time.Now().UTC()

		for name, post := range map[string]*model.Post{
			"nil post":       nil,
			"unset CreateAt": {CreateAt: 0},
		} {
			got := referenceTime(post)
			if got.Before(before) || got.After(time.Now().UTC().Add(time.Second)) {
				t.Errorf("%s: referenceTime = %v, want approximately now", name, got)
			}
		}
	})
}
