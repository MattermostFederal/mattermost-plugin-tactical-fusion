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
	// EnableLocationMGRS and EnableLocationUTM are separate for a different
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
	EnableLocationMGRS     bool
	EnableLocationUTM      bool
	EnableLocationGEOREF   bool
	EnableLocationGARS     bool
	EnableLocationPlusCode bool
	EnableLocationMoniker  bool

	// EnableAirport is the switch for the airfield decorator, which recognizes
	// an ICAO airfield code behind one of the four USMTF field labels that
	// introduce one. False at zero exactly as the switches above are.
	//
	// One switch rather than two: the grammar is label-only by construction,
	// because four letters is what ordinary prose is made of and 343 of the
	// idents this plugin holds are English dictionary words. A separate moniker
	// switch would only turn the whole decorator off under a second name.
	//
	// It governs decoration only. A link already written into a message keeps
	// working when this is off, because the page that renders it never consults
	// this.
	EnableAirport bool

	// EnableAirportTable governs whether a message that is nothing but an
	// airfield code is expanded with a markdown table of the field's details.
	//
	// Separate from EnableAirport because it rewrites much more of what the
	// author wrote. The table goes into the STORED message, so it is in every
	// export, an author editing the post sees the markdown, and the values are
	// frozen at the moment of posting: the database ships with the plugin, so a
	// table written today keeps today's elevation after an upgrade corrects it.
	//
	// Turning it off stops new messages being expanded. It cannot un-expand the
	// ones already posted, for the same reason no decoration is ever undone:
	// the text is what the author's message now says.
	EnableAirportTable bool

	// EnableLocationMap is the switch for drawing a coordinate on a map, and
	// the three below select which surfaces draw one. They are ANDed with
	// EnableLocation as well as with each other, because a map only ever
	// appears behind a coordinate this plugin decorated.
	//
	// Unlike a format switch these are read at render rather than only at
	// decoration: a map is drawn live every time somebody looks, so turning one
	// off has to reach links already written into messages. See the Maps
	// documentation in server/decorators/location.
	//
	// EnableLocationMapInline is the one with a cost beyond bytes on the wire.
	// Drawing under a post means stamping it with a custom Type, and
	// Elasticsearch and OpenSearch build an allow list of `default` and
	// `slack_attachment`, so such a post is indexed and then never matches:
	// it is absent from search and from Recent Mentions. Turning it off leaves
	// those posts ordinary and gets all of that back.
	EnableLocationMap       bool
	EnableLocationMapPanel  bool
	EnableLocationMapInline bool
	EnableLocationMapPage   bool

	// LocationMapPackagesDir is a directory on the server's filesystem that
	// detail map packages are read from, beside the ones the bundle ships.
	// Empty means bundled packages only.
	//
	// It is a real path rather than anything routed through plugin.API because
	// PMTiles is read by byte range and neither ReadFile nor GetFile can serve
	// one: both return the whole file, so a request for a single tile would
	// load an entire archive into memory. os.Open with http.ServeContent is the
	// only reader that answers a range without doing that.
	LocationMapPackagesDir string

	// EnableCot is the switch for rendering a Cursor on Target event, and
	// EnableCotFile decides whether an attached .xml or .cot file is read at
	// post time. Both are false at zero, exactly as every switch above is.
	//
	// EnableCot governs stamping only. A post already stamped keeps rendering
	// after this is turned off, for the same reason a decorator link already
	// written into a message keeps working.
	//
	// EnableCotFile is one of the two settings that put a filestore
	// read on the post path.
	//
	// There is deliberately no EnableCotMap. EnableLocationMapInline already
	// means "the map under a post" and the CoT card reads the same answer, so
	// the parent ANDs cannot be re-implemented differently here.
	EnableCot     bool
	EnableCotFile bool

	// EnableGeoJSON is the switch for rendering a GeoJSON document, and
	// EnableGeoJSONFile decides whether an attached .geojson file is read at
	// post time. Both are false at zero, exactly as every switch above is, and
	// both mean for GeoJSON what the pair above means for Cursor on Target.
	//
	// There is deliberately no EnableGeoJSONMap, for the reason the CoT block
	// above gives: EnableLocationMapInline already means "the map under a
	// post".
	EnableGeoJSON     bool
	EnableGeoJSONFile bool

	// EnableGeoJSONUnlabeled reads the spellings that do not name the format: a
	// fence labeled json, a .json attachment, and a document with no fence at
	// all. False at zero, and unlike every other switch here it also ships off.
	//
	// Stamping a post is permanent and costs it its search matches, and ordinary
	// JSON is pasted into chat constantly, so the ambiguous spellings are opt-in
	// rather than opt-out. An install whose channels carry overlays rather than
	// API payloads can turn it on.
	EnableGeoJSONUnlabeled bool
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

// configurationLoaded reports whether the settings have ever been read.
//
// getConfiguration deliberately substitutes a zero value for a nil one, so every
// caller that only wants to decorate can treat "not loaded" as "everything off"
// and carry on. That is the safe reading where the answer is discarded straight
// away, and the wrong one where it is handed to a client that will cache it: an
// unloaded configuration would then be reported as an admin decision. This is
// how the two are told apart.
func (p *Plugin) configurationLoaded() bool {
	p.configurationLock.RLock()
	defer p.configurationLock.RUnlock()

	return p.configuration != nil
}

func (p *Plugin) setConfiguration(configuration *configuration) {
	p.configurationLock.Lock()
	defer p.configurationLock.Unlock()

	if configuration != nil && p.configuration == configuration {
		// Boilerplate from the plugin starter template, for a configuration with
		// no fields at all: re-setting one is harmless because there is nothing
		// to change. Unreachable here and expected to stay uncovered, since
		// configuration has fifteen fields.
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
