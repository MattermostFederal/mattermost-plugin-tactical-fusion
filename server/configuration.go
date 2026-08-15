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

	// EnableLocation is the switch for the coordinate decorator, and the nine
	// below select which of its grammars are matched. They behave exactly as
	// the date-time group switches above do, including being false at zero.
	//
	// EnableLocationDDSigned is separate from the rest because signed decimal
	// degrees is the weakest grammar here: it is two decimal numbers and a
	// comma, held apart from ordinary text by a four-decimal rule. A workspace
	// bitten by it must be able to kill exactly that and keep the others.
	//
	// EnableLocationUSMTF plus EnableLocationMoniker on their own reproduce the
	// posture of mattermost-plugin-aocanywhere, which decorates a coordinate
	// only when the author labeled it. That is a supported configuration.
	//
	// EnableLocationGrid and EnableLocationUTM are separate for a different
	// reason again: MGRS and UTM are the only grammars whose position is
	// computed rather than read off the token, so turning both off removes
	// every row this plugin derives from a hand-written projection rather than
	// from arithmetic on what the author typed. A workspace that wants only
	// what the message literally says can have exactly that.
	//
	// EnableLocationUTM is the one switch in this plugin that ships OFF, and it
	// is the only one whose default is about correctness rather than noise. A
	// UTM token is genuinely ambiguous: "11S" is band S here and "zone 11,
	// southern hemisphere" to a civilian, 90 degrees of latitude apart. Every
	// other switch trades a false positive against a missed decoration; this
	// one trades it against a decoration that is confidently wrong. See the
	// Formats.UTM documentation for the whole argument.
	//
	// EnableLocationGEOREF, EnableLocationGARS and EnableLocationPlusCode are
	// the area-reference systems, which name a cell of the graticule rather
	// than a point and need no projection to read. The first two are reachable
	// behind a field label only, so they also need EnableLocationMoniker; that
	// is not a nesting the manifest can express and is why their help text says
	// so in words.
	EnableLocation         bool
	EnableLocationDDSigned bool
	EnableLocationLatLon   bool
	EnableLocationUSMTF    bool
	EnableLocationGrid     bool
	EnableLocationUTM      bool
	EnableLocationGEOREF   bool
	EnableLocationGARS     bool
	EnableLocationPlusCode bool
	EnableLocationMoniker  bool
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
