package main

import (
	"reflect"

	"github.com/pkg/errors"
)

type configuration struct {
	// EnableDTG is the switch for the date-time group decorator, and the three
	// below select which of its formats are matched.
	//
	// This is the admin's way to stop decoration without uninstalling.
	// Disabling the plugin is not the same thing: that also stops /decorate
	// serving, so every link already written into a message would 404. Turning
	// this off stops new decoration and leaves the history working.
	//
	// All four are false at zero on purpose: if the configuration has not
	// loaded yet we would rather decorate nothing than rewrite somebody's
	// message on a guess. plugin.json defaults every one of them to true.
	//
	// They govern decoration only. A link already written into a message keeps
	// working when its format is switched off, because the page that renders it
	// never consults any of this.
	EnableDTG          bool
	EnableDTGMilitary  bool
	EnableDTGMoniker   bool
	EnableDTGTimestamp bool
}

func (c *configuration) Clone() *configuration {
	clone := *c
	return &clone
}

func (p *Plugin) getConfiguration() *configuration {
	p.configurationLock.RLock()
	defer p.configurationLock.RUnlock()

	if p.configuration == nil {
		return &configuration{}
	}

	return p.configuration
}

func (p *Plugin) setConfiguration(configuration *configuration) {
	p.configurationLock.Lock()
	defer p.configurationLock.Unlock()

	if configuration != nil && p.configuration == configuration {
		// Boilerplate from the plugin starter template, for a configuration with
		// no fields at all: re-setting one is harmless because there is nothing
		// to change. Unreachable here and expected to stay uncovered, since
		// configuration has four fields.
		if reflect.ValueOf(*configuration).NumField() == 0 {
			return
		}

		panic("setConfiguration called with the existing configuration")
	}

	p.configuration = configuration
}

func (p *Plugin) OnConfigurationChange() error {
	configuration := new(configuration)

	if err := p.API.LoadPluginConfiguration(configuration); err != nil {
		return errors.Wrap(err, "failed to load plugin configuration")
	}

	p.setConfiguration(configuration)

	return nil
}
