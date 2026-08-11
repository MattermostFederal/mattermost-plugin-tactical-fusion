package main

import (
	"sync"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/plugin"
	"github.com/pkg/errors"

	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/decorators"
	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/decorators/dtg"
	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/errcode"
)

type Plugin struct {
	plugin.MattermostPlugin

	configurationLock sync.RWMutex
	configuration     *configuration

	// decorators is built once in OnActivate and only read afterwards, by the
	// message hook and by ServeHTTP. Owning it here rather than in a package
	// level variable keeps those concurrent readers race-free and makes
	// activation repeatable.
	decorators *decorators.Registry

	// preferences is the per-reader view settings store, cached in memory.
	// Built in OnActivate alongside the registry, and read from the API
	// handlers.
	preferences *cachingPreferenceStore
}

// dtgFormats reports which date-time group formats the admin has left on.
//
// Read fresh for every message rather than captured at activation, so a change
// in the admin console takes effect without a restart. The decorator stays
// registered whatever this returns, so the pages behind links already in
// existing messages keep rendering.
func (p *Plugin) dtgFormats() dtg.Formats {
	config := p.getConfiguration()
	if !config.EnableDTG {
		return dtg.Formats{}
	}

	return dtg.Formats{
		Military:  config.EnableDTGMilitary,
		Moniker:   config.EnableDTGMoniker,
		Timestamp: config.EnableDTGTimestamp,
	}
}

func (p *Plugin) OnActivate() error {
	// Adding a decorator is one line here plus one directory. Nothing in
	// server/decorators needs to change.
	registry, err := decorators.NewDefaultRegistry(
		&dtg.Decorator{Enabled: p.dtgFormats},
	)
	// Expected to stay uncovered: Register only rejects a duplicate or empty
	// type, and there is one decorator here with a constant one. It is what
	// turns a bad set into a failed activation once there are two, which is the
	// last moment an operator can still be told about it. The error path itself
	// is covered in server/decorators/registry_test.go.
	if err != nil {
		return errors.Wrap(err, errcode.WithCode(errcode.PluginRegistryFailed, "failed to register decorators"))
	}
	p.decorators = registry

	p.preferences = newCachingPreferenceStore(&kvPreferenceStore{api: p.API}, p.API)

	if err := p.API.RegisterCommand(getCommand()); err != nil {
		return errors.Wrap(err, errcode.WithCode(errcode.PluginCommandRegistrationFailed,
			"failed to register the slash command"))
	}

	return nil
}

// OnPluginClusterEvent keeps the preferences cache honest across nodes.
//
// Without it, a reader who changes their timezones on one node keeps seeing the
// old table on every other node for as long as the cache TTL lasts.
func (p *Plugin) OnPluginClusterEvent(_ *plugin.Context, ev model.PluginClusterEvent) {
	if p.preferences != nil {
		p.preferences.HandleClusterEvent(ev)
	}
}
