package main

import (
	"errors"
	"testing"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/plugin"

	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/decorators/dtg"
)

// newActivationPlugin returns a plugin that has not been activated yet, which
// is the one state newTestPlugin cannot give us: it builds the registry itself
// so that the other tests do not depend on activation having run.
func newActivationPlugin(t *testing.T) (*Plugin, *fakeAPI) {
	t.Helper()

	config := &model.Config{}
	config.SetDefaults()
	config.ServiceSettings.SiteURL = model.NewPointer("https://example.com")

	api := &fakeAPI{config: config, kv: map[string][]byte{}}

	p := &Plugin{}
	p.SetAPI(api)

	return p, api
}

// Activation is what turns a loaded plugin into a working one. Nothing else in
// this suite calls it, because newTestPlugin builds the registry by hand, so
// without this OnActivate could be deleted and every other test would still
// pass.
func TestOnActivateWiresThePlugin(t *testing.T) {
	p, api := newActivationPlugin(t)

	if err := p.OnActivate(); err != nil {
		t.Fatalf("OnActivate returned an error: %v", err)
	}

	if p.decorators == nil {
		t.Fatal("OnActivate left the decorator registry nil, so /decorate would 404 for everything")
	}
	if p.decorators.Get(dtg.Type) == nil {
		t.Fatalf("the registry does not resolve %q", dtg.Type)
	}

	if p.preferences == nil {
		t.Fatal("OnActivate left the preferences store nil, so the settings API would answer 503 forever")
	}

	if len(api.commands) != 1 {
		t.Fatalf("registered %d commands, want 1", len(api.commands))
	}
	if got := api.commands[0].Trigger; got != commandTrigger {
		t.Fatalf("registered trigger %q, want %q", got, commandTrigger)
	}
}

// The decorator is handed p.dtgFormats rather than the formats as they stand at
// activation. That indirection is the whole reason an admin console change
// takes effect without a restart, and it is invisible: freeze the value here
// and every existing test still passes, because they all set the configuration
// before building their registry.
func TestOnActivateReadsFormatsFreshRatherThanFreezingThem(t *testing.T) {
	p, _ := newActivationPlugin(t)

	// Activate with everything off, which is also the zero value a plugin has
	// before its configuration has loaded.
	p.setConfiguration(&configuration{})

	if err := p.OnActivate(); err != nil {
		t.Fatalf("OnActivate returned an error: %v", err)
	}

	decorator := p.decorators.Get(dtg.Type)
	typed, ok := decorator.(*dtg.Decorator)
	if !ok {
		t.Fatalf("registered decorator is %T, want *dtg.Decorator", decorator)
	}

	if got := typed.Enabled(); (got != dtg.Formats{}) {
		t.Fatalf("Enabled() = %+v before configuration, want every format off", got)
	}

	// Now the admin turns decoration on. No reactivation.
	p.setConfiguration(&configuration{
		EnableDTG:          true,
		EnableDTGMilitary:  true,
		EnableDTGMoniker:   true,
		EnableDTGTimestamp: true,
	})

	if got := typed.Enabled(); !got.Military {
		t.Fatalf("Enabled() = %+v after the configuration changed, want the new formats; "+
			"the decorator captured the formats at activation instead of reading them fresh", got)
	}
}

// A plugin can be activated again without being reloaded. Reusing one registry
// would fail on the duplicate type the second time round.
func TestOnActivateIsRepeatable(t *testing.T) {
	p, api := newActivationPlugin(t)

	if err := p.OnActivate(); err != nil {
		t.Fatalf("first OnActivate returned an error: %v", err)
	}
	first := p.decorators

	if err := p.OnActivate(); err != nil {
		t.Fatalf("second OnActivate returned an error: %v", err)
	}

	if p.decorators == first {
		t.Fatal("the second activation reused the first registry rather than building a fresh one")
	}
	if len(api.commands) != 2 {
		t.Fatalf("registered %d commands over two activations, want 2", len(api.commands))
	}
}

// Activation has to fail loudly. A plugin that reports success without its
// command registered looks installed and does nothing.
func TestOnActivateSurfacesACommandFailure(t *testing.T) {
	p, api := newActivationPlugin(t)
	api.registerErr = errors.New("command already exists")

	err := p.OnActivate()
	if err == nil {
		t.Fatal("OnActivate reported success despite the command failing to register")
	}
	if !errors.Is(err, api.registerErr) {
		t.Fatalf("OnActivate returned %v, which does not wrap the underlying failure", err)
	}
}

// Without this the reader who saves on one node keeps seeing their old table on
// every other node until the cache TTL runs out.
func TestOnPluginClusterEventDropsTheCachedReader(t *testing.T) {
	p, api := newActivationPlugin(t)
	if err := p.OnActivate(); err != nil {
		t.Fatalf("OnActivate returned an error: %v", err)
	}

	// Populate the cache, then change the stored blob behind its back. A cache
	// that has not been invalidated keeps serving the old value.
	if _, err := p.preferences.Get(testUserID); err != nil {
		t.Fatalf("priming the cache failed: %v", err)
	}
	api.kv[preferenceKey(testUserID)] = []byte(`{"dtg":{"urgent_within_minutes":7}}`)

	if prefs, _ := p.preferences.Get(testUserID); prefs.DTG.UrgentWithinMinutes != 0 {
		t.Fatal("the cache did not serve the primed value, so this test proves nothing")
	}

	p.OnPluginClusterEvent(&plugin.Context{}, model.PluginClusterEvent{
		Id:   clusterEventInvalidatePreferences,
		Data: []byte(testUserID),
	})

	prefs, err := p.preferences.Get(testUserID)
	if err != nil {
		t.Fatalf("reading after invalidation failed: %v", err)
	}
	if prefs.DTG.UrgentWithinMinutes != 7 {
		t.Fatalf("threshold = %d after invalidation, want 7; the entry was not dropped",
			prefs.DTG.UrgentWithinMinutes)
	}
}

// Events are broadcast to every plugin hook on the node, so one this plugin
// does not recognize must be ignored rather than acted on.
func TestOnPluginClusterEventIgnoresOtherEvents(t *testing.T) {
	p, api := newActivationPlugin(t)
	if err := p.OnActivate(); err != nil {
		t.Fatalf("OnActivate returned an error: %v", err)
	}

	if _, err := p.preferences.Get(testUserID); err != nil {
		t.Fatalf("priming the cache failed: %v", err)
	}
	api.kv[preferenceKey(testUserID)] = []byte(`{"dtg":{"urgent_within_minutes":7}}`)

	p.OnPluginClusterEvent(&plugin.Context{}, model.PluginClusterEvent{
		Id:   "something_else",
		Data: []byte(testUserID),
	})

	if prefs, _ := p.preferences.Get(testUserID); prefs.DTG.UrgentWithinMinutes != 0 {
		t.Fatal("an unrelated cluster event dropped the cached entry")
	}
}

// An event can arrive between the plugin loading and OnActivate finishing.
// Panicking there would take the node's plugin host down with it.
func TestOnPluginClusterEventBeforeActivation(t *testing.T) {
	p, _ := newActivationPlugin(t)

	p.OnPluginClusterEvent(&plugin.Context{}, model.PluginClusterEvent{
		Id:   clusterEventInvalidatePreferences,
		Data: []byte(testUserID),
	})
}

func TestDecorationIsOffBeforeActivationBuildsTheRegistry(t *testing.T) {
	p, _ := newActivationPlugin(t)

	if p.decorationEnabled() {
		t.Error("decoration reported on with no registry, which no branch may treat as usable")
	}

	p.setConfiguration(&configuration{EnableDTG: true, EnableDTGMilitary: true})
	if err := p.OnActivate(); err != nil {
		t.Fatalf("OnActivate returned an error: %v", err)
	}
	if !p.decorationEnabled() {
		t.Error("decoration reported off after activation with a format switched on")
	}
}

func TestDecorationIsOffWithEveryFormatSwitchedOff(t *testing.T) {
	p, _ := newActivationPlugin(t)
	p.setConfiguration(&configuration{})

	if err := p.OnActivate(); err != nil {
		t.Fatalf("OnActivate returned an error: %v", err)
	}
	if p.decorationEnabled() {
		t.Error("decoration reported on while every decorator contributes no patterns")
	}
}
