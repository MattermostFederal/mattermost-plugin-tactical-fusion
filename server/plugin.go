package main

import (
	"sync"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/plugin"
	"github.com/pkg/errors"

	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/decorators"
	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/decorators/dtg"
	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/decorators/location"
	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/errcode"
)

type Plugin struct {
	plugin.MattermostPlugin

	configurationLock sync.RWMutex
	configuration     *configuration

	// decorators is built once in OnActivate and only read afterwards, by the
	// message hook and by ServeHTTP.
	//
	// Owning it here rather than in a package level variable buys two things,
	// and it is worth being exact about which: activation is repeatable, since
	// a global would fail permanently on "already registered" the second time
	// round, and no reader can observe a half-populated registry, since the
	// whole thing is built before this field is assigned rather than being
	// filled in by Register calls a reader could interleave with.
	//
	// What it does NOT do is establish a happens-before edge between the write
	// here and those reads. That ordering is the plugin framework's to provide
	// by not dispatching hooks until OnActivate returns, and we have not
	// confirmed it does. The nil checks in http.go and api.go assume it might
	// not, and if that assumption is right then those checks are themselves
	// racing and this wants an atomic.Pointer with an accessor. Do not read the
	// paragraph above as a claim that the reads are synchronised.
	decorators *decorators.Registry

	// preferences is the per-reader view settings store, cached in memory.
	// Built in OnActivate alongside the registry, and read from the API
	// handlers. The same caveat about ordering applies.
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

// locationFormats reports which coordinate grammars the admin has left on.
//
// Read fresh for every message, and the parent is enforced here rather than in
// the manifest, because Mattermost's settings schema has no nesting.
func (p *Plugin) locationFormats() location.Formats {
	config := p.getConfiguration()
	if !config.EnableLocation {
		return location.Formats{}
	}

	return location.Formats{
		DDSigned: config.EnableLocationDDSigned,
		LatLon:   config.EnableLocationLatLon,
		USMTF:    config.EnableLocationUSMTF,
		MGRS:     config.EnableLocationGrid,
		UTM:      config.EnableLocationUTM,
		GEOREF:   config.EnableLocationGEOREF,
		GARS:     config.EnableLocationGARS,
		PlusCode: config.EnableLocationPlusCode,
		Moniker:  config.EnableLocationMoniker,
	}
}

// decorationEnabled reports whether any decorator would contribute a pattern.
//
// Asked of the registry rather than of one decorator's switches, so the
// examples command cannot claim decoration is off while another decorator is
// still running.
func (p *Plugin) decorationEnabled() bool {
	if p.decorators == nil {
		return false
	}

	for _, d := range p.decorators.All() {
		if len(d.Patterns()) > 0 {
			return true
		}
	}
	return false
}

func (p *Plugin) OnActivate() error {
	// Adding a decorator is one line here plus one directory.
	registry, err := decorators.NewDefaultRegistry(
		&dtg.Decorator{Enabled: p.dtgFormats},
		&location.Decorator{Enabled: p.locationFormats},
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
