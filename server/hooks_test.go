package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/plugin"

	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/decorators"
	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/decorators/airport"
	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/decorators/dtg"
	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/decorators/location"
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

	// ephemeral records every ephemeral post, in the order it was sent, so a
	// test can read a multi-message command's output as the reader sees it.
	ephemeral []*model.Post

	// created records every real post, in order, so a test can read a threaded
	// command's output and check what hangs off what.
	created []*model.Post

	// refused records the rune count of every post turned away for length, so a
	// test can tell a command that got it right first time from one that had to
	// find out.
	refused []int

	// createPostErr forces CreatePost to fail. createPostFailFrom is the
	// zero-based index of the first call that fails, so a test can let the root
	// land and refuse a reply, which is the case where the command has already
	// written to the channel and still has to report.
	createPostErr      *model.AppError
	createPostFailFrom int

	// postSizeLimit makes CreatePost refuse an over-long message the way a real
	// server does, which is the only way to test code that has to discover the
	// limit rather than being told it. Zero means no limit.
	postSizeLimit int

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

func (a *fakeAPI) SendEphemeralPost(_ string, post *model.Post) *model.Post {
	a.ephemeral = append(a.ephemeral, post)
	return post
}

// CreatePost stamps an id, because the caller threads replies off the root's.
func (a *fakeAPI) CreatePost(post *model.Post) (*model.Post, *model.AppError) {
	if a.createPostErr != nil && len(a.created) >= a.createPostFailFrom {
		return nil, a.createPostErr
	}

	// The same refusal model.Post.IsValid produces, message and all, so a test
	// sees what the server would actually say.
	if a.postSizeLimit > 0 && utf8.RuneCountInString(post.Message) > a.postSizeLimit {
		a.refused = append(a.refused, utf8.RuneCountInString(post.Message))
		return nil, model.NewAppError("CreatePost", "model.post.is_valid.message_length.app_error",
			nil, "Post Message property is longer than the maximum permitted length.", 400)
	}

	stored := post.Clone()
	stored.Id = fmt.Sprintf("post%d", len(a.created))
	a.created = append(a.created, stored)

	return stored, nil
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

		EnableLocation:         enabled,
		EnableLocationDDSigned: true,
		EnableLocationLatLon:   true,
		EnableLocationUSMTF:    true,
		EnableLocationMGRS:     true,
		EnableLocationUTM:      true,
		EnableLocationGEOREF:   true,
		EnableLocationGARS:     true,
		EnableLocationPlusCode: true,
		EnableLocationMoniker:  true,

		EnableLocationMap:       true,
		EnableLocationMapPanel:  true,
		EnableLocationMapInline: enabled,
		EnableLocationMapPage:   true,

		EnableAirport: enabled,
	})

	registerDecoratorsForTest(t, p)

	return p
}

// registerDecoratorsForTest builds the plugin's decorator registry, mirroring
// what OnActivate does. Each plugin owns its own, so tests never share state.
func registerDecoratorsForTest(t *testing.T, p *Plugin) {
	t.Helper()
	registry, err := decorators.NewDefaultRegistry(
		&dtg.Decorator{Enabled: p.dtgFormats},
		&location.Decorator{Enabled: p.locationFormats, Maps: p.locationMaps},
		&airport.Decorator{Enabled: p.airportFormats},
	)
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

// A subpath is normalized the way Mattermost normalizes it, not refused.
//
// These URLs are what makes the difference between hardening and a regression.
// Mattermost derives its own subpath with path.Clean, so "https://host//mm" is
// served at /mm: a typo'd but WORKING install. Refusing it outright wrote
// root-relative links that 404 there, permanently, into stored post text that
// correcting SiteURL afterwards cannot repair.
//
// The escaped forms are the off-origin half of the same rule. A browser folds
// "\" to "/" and strips a tab, so both have to survive as %5C and %09 rather
// than as characters a URL parser can turn into an authority.
func TestDecoratePostNormalizesAnAwkwardSubpath(t *testing.T) {
	cases := []struct {
		name    string
		siteURL string
		want    string
	}{
		{"a doubled slash is cleaned, as Mattermost cleans it", "https://example.com//mm", "(/mm/plugins/"},
		{"a dot segment is cleaned", "https://example.com/a/../mm", "(/mm/plugins/"},
		{"a backslash cannot become an authority", "https://example.com/\\mm", "(/%5Cmm/plugins/"},
		{"a tab cannot become an authority", "https://example.com/%09mm", "(/%09mm/plugins/"},
		{"a space stays encoded, or the markdown link breaks", "https://example.com/my%20mm", "(/my%20mm/plugins/"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := newTestPlugin(t, tc.siteURL, true)

			got := p.decoratePost(&model.Post{Message: "091630ZAUG26"}, hookRef)
			if got == nil {
				t.Fatal("decoratePost skipped; an awkward subpath is not a reason to skip")
			}
			if !strings.Contains(got.Message, tc.want) {
				t.Fatalf("message = %q, want a link starting %q", got.Message, tc.want)
			}
		})
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
	//
	// The multiplier is what makes this reachable at all: a 12-character
	// date-time group becomes roughly 120 once linked, so it takes about a
	// tenth of the limit in tokens to cross it. Against safePostRunes, the
	// 4,000-rune floor this hook measures itself with, that is a long message
	// but not an impossible one, so the guard is reachable rather than
	// theoretical. It is worth having either way, because the real limit
	// cannot be read from the plugin API and safePostRunes is an assumption
	// tracked by hand: the day it is wrong is the day this fires.
	const dtg = "091630ZAUG26 "
	message := strings.TrimSpace(strings.Repeat(dtg, safePostRunes/len(dtg)))

	if len([]rune(message)) >= safePostRunes {
		t.Fatalf("test message is already %d runes, which does not exercise the guard", len([]rune(message)))
	}

	if got := p.decoratePost(&model.Post{Message: message}, hookRef); got != nil {
		t.Fatal("decoratePost returned a post that would exceed the maximum size")
	}

	// And it says so, because an author whose message silently stopped being
	// decorated has no other way to find out why.
	api := p.API.(*fakeAPI)
	if len(api.warnings) == 0 {
		t.Error("nothing was logged, so an operator has no record of the skip")
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

// A post can reach the hook before OnActivate has built the registry, and a
// deactivated plugin can still have the hook called. Nothing here may stop
// somebody from posting, so a missing registry means "decorate nothing" rather
// than a nil dereference.
func TestDecoratePostWithoutARegistry(t *testing.T) {
	config := &model.Config{}
	config.SetDefaults()
	config.ServiceSettings.SiteURL = model.NewPointer("https://example.com")

	p := &Plugin{}
	p.SetAPI(&fakeAPI{config: config})
	p.setConfiguration(&configuration{
		EnableDTG:          true,
		EnableDTGMilitary:  true,
		EnableDTGMoniker:   true,
		EnableDTGTimestamp: true,
	})

	if p.decorators != nil {
		t.Fatal("test plugin already has a registry, which does not exercise the guard")
	}

	if got := p.decoratePost(&model.Post{Message: "091630ZAUG26"}, hookRef); got != nil {
		t.Fatalf("decoratePost = %+v, want nil when no decorator is registered", got)
	}
}

// SiteURL is read straight off the server configuration, and both the config
// and the pointer inside it can be absent. Neither is a reason to skip
// decoration: no subpath simply means the link starts at the root.
func TestSiteURLPathToleratesAMissingConfiguration(t *testing.T) {
	withSiteURL := func(siteURL *string) *model.Config {
		config := &model.Config{}
		config.SetDefaults()
		config.ServiceSettings.SiteURL = siteURL
		return config
	}

	cases := []struct {
		name   string
		config *model.Config
	}{
		{"no configuration at all", nil},
		{"configuration with no SiteURL", withSiteURL(nil)},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := siteURLPath(tc.config); got != "" {
				t.Fatalf("siteURLPath() = %q, want an empty prefix", got)
			}
		})
	}
}

// The same absence seen from the post path: a plugin whose API reports no
// SiteURL still decorates, with a root-relative link.
func TestDecoratePostWorksWhenSiteURLIsAbsent(t *testing.T) {
	config := &model.Config{}
	config.SetDefaults()
	config.ServiceSettings.SiteURL = nil

	p := &Plugin{}
	p.SetAPI(&fakeAPI{config: config})
	p.setConfiguration(&configuration{
		EnableDTG:          true,
		EnableDTGMilitary:  true,
		EnableDTGMoniker:   true,
		EnableDTGTimestamp: true,
	})
	registerDecoratorsForTest(t, p)

	got := p.decoratePost(&model.Post{Message: "091630ZAUG26"}, hookRef)
	if got == nil {
		t.Fatal("decoratePost skipped; an absent SiteURL is not a reason to skip")
	}

	want := "(/plugins/" + manifest.Id + "/decorate/dtg?"
	if !strings.Contains(got.Message, want) {
		t.Fatalf("message = %q, want a root-relative link starting %q", got.Message, want)
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
		labeled   = "DTG: 091630ZAUG26"
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
			[]string{military, labeled, timestamp},
			nil,
		},
		{
			"military off",
			configuration{EnableDTG: true, EnableDTGMoniker: true, EnableDTGTimestamp: true},
			[]string{timestamp},

			// The moniker is on, but it has nothing to label here: turning a
			// format off has to stop the labeled form of it too.
			[]string{military, labeled},
		},
		{
			"timestamps off",
			configuration{EnableDTG: true, EnableDTGMilitary: true, EnableDTGMoniker: true},
			[]string{military, labeled},
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
			[]string{military, labeled, timestamp},
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
	req := withSession(httptest.NewRequest(http.MethodGet,
		"/decorate/dtg?t=1786293000000&dtg=091630ZAUG26&z=Z&a=", nil))

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

	t.Run("an existing CreateAt is honored", func(t *testing.T) {
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

// standaloneProps pulls this plugin's props off a post, or nil.
func standaloneProps(t *testing.T, post *model.Post) map[string]any {
	t.Helper()

	if post == nil {
		return nil
	}
	value, ok := post.GetProps()[decorators.PostPropsKey]
	if !ok {
		return nil
	}
	props, ok := value.(map[string]any)
	if !ok {
		t.Fatalf("props under %q are %T, want map[string]any", decorators.PostPropsKey, value)
	}
	return props
}

func TestDecoratePostStampsAMessageThatIsOnlyACoordinate(t *testing.T) {
	for _, tc := range []struct {
		name    string
		message string
		wantF   string
		wantV   string
	}{
		{"a bare coordinate", "34.0561N,118.2500W", "ddh", "34.0561N,118.2500W"},
		{"a labeled grid reference", "MGRS: 18SUJ2347806483", "mgrs", "18SUJ2347806483"},
		{"surrounding whitespace", "  34.0561N,118.2500W\n", "ddh", "34.0561N,118.2500W"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := newTestPlugin(t, "https://example.com", true)

			got := p.decoratePost(&model.Post{Message: tc.message}, hookRef)
			if got == nil {
				t.Fatal("decoratePost left the post alone, want it decorated")
			}
			if got.Type != location.PostType {
				t.Fatalf("Type = %q, want %q", got.Type, location.PostType)
			}

			props := standaloneProps(t, got)
			if props == nil {
				t.Fatal("no props were stamped on a standalone coordinate")
			}
			if props["type"] != location.Type {
				t.Fatalf("props[type] = %v, want %q", props["type"], location.Type)
			}
			if props["f"] != tc.wantF {
				t.Fatalf("props[f] = %v, want %q", props["f"], tc.wantF)
			}
			if props["v"] != tc.wantV {
				t.Fatalf("props[v] = %v, want %q", props["v"], tc.wantV)
			}
			if props["version"] != decorators.PostPropsVersion {
				t.Fatalf("props[version] = %v, want %d", props["version"], decorators.PostPropsVersion)
			}
		})
	}
}

// The author's own spelling travels in the props exactly as it travels in the
// URL, and is omitted when it would only repeat the canonical form.
func TestDecoratePostCarriesTheAuthorsTextOnlyWhenItDiffers(t *testing.T) {
	p := newTestPlugin(t, "https://example.com", true)

	differs := standaloneProps(t, p.decoratePost(&model.Post{Message: "34.0561N, 118.2500W"}, hookRef))
	if differs["r"] != "34.0561N, 118.2500W" {
		t.Fatalf("props[r] = %v, want the author's text", differs["r"])
	}

	same := standaloneProps(t, p.decoratePost(&model.Post{Message: "34.0561N,118.2500W"}, hookRef))
	if _, ok := same["r"]; ok {
		t.Fatalf("props carry r = %v when it only repeats v", same["r"])
	}
}

// With the inline map switched off the post is decorated and NOT stamped.
//
// The stamp is what costs the post its Elasticsearch and OpenSearch matches, so
// skipping it is the substance of the switch rather than a tidy-up: leaving the
// Type on and merely declining to draw would keep every one of those costs and
// buy nothing. Post.Type also survives every edit once it is set, and there is
// deliberately no MessageWillBeUpdated hook to clear one, so this is the only
// moment the decision can be made.
func TestDecoratePostDoesNotStampWhenTheInlineMapIsOff(t *testing.T) {
	p := newTestPlugin(t, "https://example.com", true)

	config := p.getConfiguration().Clone()
	config.EnableLocationMapInline = false
	p.setConfiguration(config)

	got := p.decoratePost(&model.Post{Message: "34.0561N,118.2500W"}, hookRef)
	if got == nil {
		t.Fatal("decoratePost left the post alone; the coordinate should still be decorated")
	}

	// The link is the point: turning the map off must not turn decoration off.
	if !strings.Contains(got.Message, "/decorate/location?") {
		t.Fatalf("the coordinate was not decorated: %q", got.Message)
	}

	if got.Type != "" {
		t.Fatalf("Type = %q, want it left empty so the post stays searchable", got.Type)
	}
	if props := standaloneProps(t, got); props != nil {
		t.Fatalf("props were stamped on a post nothing will render inline: %v", props)
	}
}

// And the map parent takes the inline map with it, whatever the surface switch
// under it says.
func TestDecoratePostDoesNotStampWhenMapsAreOffEntirely(t *testing.T) {
	p := newTestPlugin(t, "https://example.com", true)

	config := p.getConfiguration().Clone()
	config.EnableLocationMap = false
	p.setConfiguration(config)

	got := p.decoratePost(&model.Post{Message: "34.0561N,118.2500W"}, hookRef)
	if got == nil {
		t.Fatal("decoratePost left the post alone; the coordinate should still be decorated")
	}
	if got.Type != "" {
		t.Fatalf("Type = %q, want it left empty", got.Type)
	}
}

func TestDecoratePostDoesNotStampACoordinateInASentence(t *testing.T) {
	p := newTestPlugin(t, "https://example.com", true)

	got := p.decoratePost(&model.Post{Message: "target at 34.0561N,118.2500W"}, hookRef)
	if got == nil {
		t.Fatal("decoratePost left the post alone, want it decorated")
	}
	if got.Type != "" {
		t.Fatalf("Type = %q, want it left empty", got.Type)
	}
	if props := standaloneProps(t, got); props != nil {
		t.Fatalf("props were stamped on a coordinate inside a sentence: %v", props)
	}
}

func TestDecoratePostDoesNotStampTwoCoordinates(t *testing.T) {
	p := newTestPlugin(t, "https://example.com", true)

	got := p.decoratePost(&model.Post{Message: "34.0561N,118.2500W 35.0000N,119.0000W"}, hookRef)
	if got == nil {
		t.Fatal("decoratePost left the post alone, want it decorated")
	}
	if got.Type != "" {
		t.Fatalf("Type = %q, want it left empty", got.Type)
	}
}

// DTG declares no PostType, so a lone date-time group stays an ordinary post.
// Losing that is how every DTG post would silently acquire a custom type and,
// with it, the Elasticsearch and translation costs the location one accepts.
func TestDecoratePostDoesNotStampALoneDateTimeGroup(t *testing.T) {
	p := newTestPlugin(t, "https://example.com", true)

	got := p.decoratePost(&model.Post{Message: "091630ZAUG26"}, hookRef)
	if got == nil {
		t.Fatal("decoratePost left the post alone, want it decorated")
	}
	if got.Type != "" {
		t.Fatalf("Type = %q, want it left empty", got.Type)
	}
	if props := standaloneProps(t, got); props != nil {
		t.Fatalf("props were stamped on a date-time group: %v", props)
	}
}

// Another integration's custom type is real mission content. Clobbering it
// would be the same mistake isSystemPost's narrow deny list exists to avoid.
func TestDecoratePostKeepsAnotherIntegrationsCustomType(t *testing.T) {
	p := newTestPlugin(t, "https://example.com", true)

	got := p.decoratePost(&model.Post{Message: "34.0561N,118.2500W", Type: "custom_something"}, hookRef)
	if got == nil {
		t.Fatal("decoratePost skipped a custom post type, want it decorated")
	}
	if got.Type != "custom_something" {
		t.Fatalf("Type = %q, want it left as %q", got.Type, "custom_something")
	}
	if props := standaloneProps(t, got); props != nil {
		t.Fatalf("props were stamped over another integration's post: %v", props)
	}
}

// Clone carries whatever props arrived, and AddProp is what keeps them.
func TestDecoratePostKeepsAnotherIntegrationsProps(t *testing.T) {
	p := newTestPlugin(t, "https://example.com", true)

	post := &model.Post{Message: "34.0561N,118.2500W"}
	post.AddProp("attachments", "theirs")

	got := p.decoratePost(post, hookRef)
	if got == nil {
		t.Fatal("decoratePost left the post alone, want it decorated")
	}
	if got.GetProps()["attachments"] != "theirs" {
		t.Fatalf("another integration's prop was lost: %v", got.GetProps())
	}
	if standaloneProps(t, got) == nil {
		t.Fatal("our own props were not stamped alongside theirs")
	}
}
