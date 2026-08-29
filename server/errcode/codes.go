// Package errcode holds the numeric catalog of error codes this plugin emits,
// in logs and in user-facing failure messages alike.
//
// The point is that a reader who sees "(TF-13004)" in the sidebar and an
// operator grepping the server log are looking at the same identifier, and that
// public/help/troubleshooting.html can be organized around something that does
// not change when the wording does. That holds within a build; see below for
// what does not hold across them.
//
// Allocation, one thousand-wide range per source file:
//
//	10000-10999   server/plugin.go              activation
//	11000-11999   server/hooks.go               message decoration
//	12000-12999   server/http.go                public routing
//	13000-13999   server/api.go                 authenticated API
//	14000-14999   server/preferences.go         validation and storage
//	15000-15999   server/preferences_cache.go   cache and cluster events
//	16000-16999   server/command*.go            the slash command
//	17000-17999   server/decorators/            framework and decorator pages
//	18000-18999   server/packages.go            detail map packages
//
// Within a range codes are allocated in source order the first time a file is
// instrumented; a site added later takes the next free number in its range, so
// codes drift out of source order as a file grows.
//
// Numbers may be renumbered and reused. Nothing here treats a code as
// permanent, and a retired one goes back in the pool rather than leaving a gap.
// The cost is real and is accepted rather than overlooked: a code quoted in an
// old log line or support ticket can come to mean something else, so read one
// against the release it came from.
//
// Adding a code means four edits that go together: a constant below, an entry
// in AllCodes, the call site, and a row in public/help/error-codes.html.
// TestAllCodesComplete enforces the first two and TestEveryCodeIsDocumented the
// fourth.
package errcode

const (
	// server/plugin.go (10000-10999)

	// PluginRegistryFailed is an activation failure: a decorator could not be
	// registered, so the plugin has no registry and refuses to start.
	PluginRegistryFailed = 10000

	// PluginCommandRegistrationFailed is an activation failure: the slash
	// command could not be registered, so the plugin refuses to start.
	PluginCommandRegistrationFailed = 10001

	// server/hooks.go (11000-11999)

	// HooksDecoratePanic is a warn recording that a decorator panicked while
	// decorating a message. The post was left exactly as written; nothing in
	// here may ever stop somebody from posting.
	HooksDecoratePanic = 11000

	// HooksDecorationTooLong is a warn recording that decorating a message
	// would have pushed it over the maximum post size, so it was posted
	// undecorated rather than rejected.
	HooksDecorationTooLong = 11001

	// HooksCotPanic is a warn recording that recognizing a Cursor on Target
	// event panicked. The post was left exactly as it arrived, and decoration
	// still had its turn.
	HooksCotPanic = 11002

	// HooksCotFileUnreadable is a warn recording that an attached file could
	// not be read or described. The post was left as an ordinary one.
	HooksCotFileUnreadable = 11003

	// HooksCotPropsTooLarge is a warn recording that the parsed event would
	// have pushed the post's whole props map over what the server accepts, so
	// nothing was stamped rather than risking a post the server refuses.
	HooksCotPropsTooLarge = 11004

	// HooksCotFileNotOwned is a warn recording that an attached file was not
	// the poster's own, so it was left unread.
	//
	// Its own code rather than HooksCotFileUnreadable, because the two say
	// opposite things to an operator: that one means the filestore did not
	// answer, and this one means it answered and the answer was somebody else's
	// file. Folding them together filed the only line that would ever betray an
	// attempted disclosure under a code whose guidance is to check that storage
	// is reachable.
	HooksCotFileNotOwned = 11006

	// HooksCotUnreadable is the code an author is given when a fence they
	// explicitly labeled cot could not be read as a Cursor on Target event.
	HooksCotUnreadable = 11005

	// HooksCotDetailDropped is a warn recording that the parsed event carried
	// more <detail> than the post's props map had room for, so the card was
	// stamped without them rather than not stamped at all.
	HooksCotDetailDropped = 11007

	// HooksCotPropsUnmeasurable is a warn recording that the post's props map
	// could not be marshalled, so its size could not be checked and nothing was
	// stamped.
	//
	// Its own code rather than HooksCotPropsTooLarge, because the two say
	// different things to an operator and only one of them is the author's to
	// act on. The value that cannot be marshalled came from somewhere else on
	// the post, so telling the author their event was too big is both false and
	// useless advice.
	HooksCotPropsUnmeasurable = 11008

	// HooksGeoJSONPanic is a warn recording that recognizing a GeoJSON
	// document panicked and the post was left unmodified.
	HooksGeoJSONPanic = 11009

	// HooksGeoJSONUnreadable is the code an author is given when a fence they
	// labeled geojson could not be read.
	HooksGeoJSONUnreadable = 11010

	// HooksGeoJSONPropsTooLarge is a warn recording that the parsed document
	// would exceed the maximum post props size, so the post was left as text.
	HooksGeoJSONPropsTooLarge = 11011

	// HooksGeoJSONPropsUnmeasurable is a warn recording that the post's props
	// map could not be measured.
	//
	// Its own code rather than HooksGeoJSONPropsTooLarge, because the two say
	// different things to whoever reads the log: one is this document being
	// large, the other is something else on the post being unreadable.
	HooksGeoJSONPropsUnmeasurable = 11012

	// HooksGeoJSONPropertiesDropped is a warn recording that the parsed
	// document carried more properties than the post props map has room for,
	// so it was stamped without them.
	HooksGeoJSONPropertiesDropped = 11013

	// HooksGeoJSONFileUnreadable is a warn recording that an attached file
	// could not be read while looking for a GeoJSON document.
	//
	// Its own code rather than the Cursor on Target one it used to borrow: an
	// operator looking a number up must not be handed another format's
	// explanation for their failure.
	HooksGeoJSONFileUnreadable = 11014

	// HooksGeoJSONFileNotOwned is a warn recording that an attachment offered
	// to the GeoJSON reader was not the poster's own.
	HooksGeoJSONFileNotOwned = 11015

	// server/http.go (12000-12999)

	// HTTPMethodNotAllowed is returned for anything other than GET on the
	// public decorator route.
	HTTPMethodNotAllowed = 12000

	// HTTPDecoratePathInvalid is returned when the path carries no decorator
	// type, or carries more than one path segment.
	HTTPDecoratePathInvalid = 12001

	// HTTPDecoratorsNotReady is returned when a request lands between
	// activation starting and OnActivate building the registry.
	HTTPDecoratorsNotReady = 12002

	// HTTPDecoratorUnknown is returned for a decorator type no build of this
	// plugin has registered.
	HTTPDecoratorUnknown = 12003

	// HTTPMapDisabled is returned for /map when the admin has turned the
	// full-window map page off. A 404 rather than a 403, because to a reader the
	// route does not exist on this install.
	HTTPMapDisabled = 12004

	// HTTPPackagePathInvalid rejects a /packages request whose path is not
	// one package archive.
	HTTPPackagePathInvalid = 12005

	// HTTPPackageUnknown reports a /packages request for an area this install
	// does not have.
	HTTPPackageUnknown = 12006

	// HTTPPackageUnreadable reports a package that was discovered and then
	// could not be opened, which usually means it moved between the two.
	HTTPPackageUnreadable = 12007

	// HTTPMapPostUnavailable is the ONE answer /map?post= gives for every way
	// it can decline: no such post, a post in a channel this reader may not
	// read, a post this plugin never stamped, and a stamped post whose card has
	// since stood down. They are one code and one 404 deliberately. Separating
	// them would let anybody with a session ask this route whether a given post
	// id exists and what kind of thing it is, one id at a time.
	HTTPMapPostUnavailable = 12008

	// server/api.go (13000-13999)

	// APINotAuthorized is returned when a request to the authenticated API
	// carries no Mattermost session.
	APINotAuthorized = 13000

	// APINotFound is returned for a path under /api/v1 that names no resource.
	APINotFound = 13001

	// APINotReady is returned when a request lands before OnActivate has built
	// the preferences store.
	APINotReady = 13002

	// APIMethodNotAllowed is returned for a method the preferences resource
	// does not support.
	APIMethodNotAllowed = 13003

	// APIPreferencesReadFailed covers both the log line and the reply when the
	// reader's stored settings could not be read.
	APIPreferencesReadFailed = 13004

	// APIPreferencesInvalidBody is returned when a submitted settings payload
	// is not decodable JSON, or is larger than the cap.
	APIPreferencesInvalidBody = 13005

	// APIPreferencesSaveFailed covers both the log line and the reply when the
	// reader's settings could not be written.
	APIPreferencesSaveFailed = 13006

	// APIPreferencesClearFailed covers both the log line and the reply when
	// "Restore defaults" could not delete the stored blob.
	APIPreferencesClearFailed = 13007

	// APIConvertInvalid is returned when the coordinate conversion endpoint is
	// given a format and token that do not name a coordinate this plugin would
	// have issued. It runs the same checks the public page does, so a link that
	// renders there converts here and one that does not fails in both places.
	APIConvertInvalid = 13008

	// APIAirportInvalid is returned when the airfield endpoint is given
	// something that is not a well-formed ICAO ident. An ident this build
	// simply does not hold is not this: that answers 200 saying so, exactly as
	// the page does, because a refreshed database must not turn every link
	// naming a retired code into a permanent failure.
	APIAirportInvalid = 13009

	// server/preferences.go (14000-14999)

	// PreferencesZoneNameTooLong rejects a row label longer than the cap.
	PreferencesZoneNameTooLong = 14000

	// PreferencesZoneNameControlCharacters rejects a row label containing a
	// control character, which would disturb the layout of its row.
	PreferencesZoneNameControlCharacters = 14001

	// PreferencesZoneIDMalformed rejects a string that does not look like an
	// IANA timezone identifier at all.
	PreferencesZoneIDMalformed = 14002

	// PreferencesZoneIDLocal rejects "Local", which resolves to whatever zone
	// the server process runs in. That is not a place, and two nodes of the
	// same cluster can disagree about it.
	PreferencesZoneIDLocal = 14003

	// PreferencesZoneIDUnknown rejects an identifier the embedded tzdata cannot
	// resolve.
	PreferencesZoneIDUnknown = 14004

	// PreferencesTooManyZones rejects a selection above the row cap.
	PreferencesTooManyZones = 14005

	// PreferencesThresholdOutOfRange rejects a countdown flash threshold
	// outside the supported range.
	PreferencesThresholdOutOfRange = 14006

	// PreferencesRowUnknown rejects a hidden-row id the location panel does not
	// have a row for.
	PreferencesRowUnknown = 14008

	// PreferencesBlobUnreadable is a warn recording that a stored blob could
	// not be parsed and the reader was served the defaults instead. View
	// settings must never be able to take the panel down with them.
	PreferencesBlobUnreadable = 14007

	// PreferencesSectionUnknown rejects a hidden-section id the Cursor on
	// Target panel does not have a section for.
	PreferencesSectionUnknown = 14009

	// server/preferences_cache.go (15000-15999)

	// PreferencesCachePublishFailed is a warn recording that a cache
	// invalidation could not be published to the other nodes. Delivery is best
	// effort: the cost is one reader seeing a stale table until the TTL runs
	// out, which does not justify failing their save.
	PreferencesCachePublishFailed = 15000

	// server/command.go, server/command_examples.go,
	// server/command_example_details.go and server/command_check.go
	// (16000-16999)

	// CommandUnknownSubcommand is returned for a subcommand this plugin does
	// not have.
	CommandUnknownSubcommand = 16000

	// CommandExamplesNotReady is returned when /tactical-fusion examples runs
	// before OnActivate has built the registry.
	CommandExamplesNotReady = 16001

	// CommandCheckNotReady is returned when /tactical-fusion check runs before
	// OnActivate has built the registry.
	CommandCheckNotReady = 16002

	// CommandExamplesNothingEnabled is returned when every format is switched
	// off, so the examples post has no row that would decorate and declines to
	// post one.
	CommandExamplesNothingEnabled = 16003

	// CommandExamplesTooLong is returned when one of the examples messages would
	// not fit in a post, which needs a long enough install subpath to reach.
	CommandExamplesTooLong = 16004

	// 16005 was CommandDetailsNotReady, for the example-details subcommand.
	// That command was folded into examples, which reports the same condition
	// through CommandExamplesNotReady. Retired rather than reused.

	// CommandExamplesPostFailed is returned, and logged, when one of the
	// examples messages could not be posted to the channel.
	CommandExamplesPostFailed = 16006

	// server/decorators/ (17000-17999)

	// DTGPageParamsInvalid is returned by the date-time group page for a link
	// whose query string carries no usable instant.
	DTGPageParamsInvalid = 17000

	// LocationPageParamsInvalid is returned by the location page for a link
	// whose query string carries no usable coordinate, whose token does not
	// reproduce itself, or whose "r" parameter is not something this plugin
	// would have written.
	LocationPageParamsInvalid = 17001

	// AirportPageInvalid is returned by the airfield page for a link whose "v"
	// parameter is not four upper-case letters. An ident this build does not
	// hold renders at 200 with a note instead.
	AirportPageInvalid = 17002

	// server/packages.go (18000-18999)

	// PackagesNoBundlePath reports that the plugin cannot locate its own
	// bundle, so no bundled map package can be served. Dropped-in packages
	// are unaffected.
	PackagesNoBundlePath = 18000

	// PackagesBadName rejects a file in a package directory whose name is not
	// <command>-<area>. The name reaches a URL, so it is whitelisted.
	PackagesBadName = 18001

	// PackagesBadArchive rejects a file that is not the PMTiles archive it
	// claims to be, or is built to a depth the seam cannot use.
	PackagesBadArchive = 18002

	// PackagesUploadBadName rejects an upload whose name is not <command>-<area>.
	PackagesUploadBadName = 18003

	// PackagesUploadNoDir rejects an upload when no package directory is
	// configured, because there is nowhere a range-readable file could go.
	PackagesUploadNoDir = 18004

	// PackagesUploadTooLarge rejects an upload past what this route carries.
	// Larger areas are copied into the package directory instead.
	PackagesUploadTooLarge = 18005

	// PackagesUploadNotAnArchive rejects an upload that is not a map archive
	// built for the seam, before it is put in place rather than after.
	PackagesUploadNotAnArchive = 18006

	// PackagesUploadWriteFailed reports a package directory that cannot be
	// written to.
	PackagesUploadWriteFailed = 18007

	// PackagesSchemaMismatch rejects an archive built for a different map
	// schema. Distinguished from PackagesBadArchive because the file is
	// well formed and the fix is to re-download the area or upgrade the
	// plugin, not to rebuild a corrupt one.
	PackagesSchemaMismatch = 18008
)

// AllCodes lists every code declared above. TestAllCodesComplete enforces that
// this slice exactly matches the declared constants, in both directions. Keep
// the entries in the same order as the const block so the slice reads as an
// index.
var AllCodes = []int{
	PluginRegistryFailed,
	PluginCommandRegistrationFailed,

	HooksDecoratePanic,
	HooksDecorationTooLong,
	HooksCotPanic,
	HooksCotFileUnreadable,
	HooksCotFileNotOwned,
	HooksCotPropsTooLarge,
	HooksCotUnreadable,
	HooksCotDetailDropped,
	HooksCotPropsUnmeasurable,
	HooksGeoJSONPanic,
	HooksGeoJSONUnreadable,
	HooksGeoJSONPropsTooLarge,
	HooksGeoJSONPropsUnmeasurable,
	HooksGeoJSONPropertiesDropped,
	HooksGeoJSONFileUnreadable,
	HooksGeoJSONFileNotOwned,

	HTTPMethodNotAllowed,
	HTTPDecoratePathInvalid,
	HTTPDecoratorsNotReady,
	HTTPDecoratorUnknown,
	HTTPMapDisabled,
	HTTPPackagePathInvalid,
	HTTPPackageUnknown,
	HTTPPackageUnreadable,
	HTTPMapPostUnavailable,

	APINotAuthorized,
	APINotFound,
	APINotReady,
	APIMethodNotAllowed,
	APIPreferencesReadFailed,
	APIPreferencesInvalidBody,
	APIPreferencesSaveFailed,
	APIPreferencesClearFailed,
	APIConvertInvalid,
	APIAirportInvalid,

	PreferencesZoneNameTooLong,
	PreferencesZoneNameControlCharacters,
	PreferencesZoneIDMalformed,
	PreferencesZoneIDLocal,
	PreferencesZoneIDUnknown,
	PreferencesTooManyZones,
	PreferencesThresholdOutOfRange,
	PreferencesRowUnknown,
	PreferencesBlobUnreadable,
	PreferencesSectionUnknown,

	PreferencesCachePublishFailed,

	CommandUnknownSubcommand,
	CommandExamplesNotReady,
	CommandCheckNotReady,
	CommandExamplesNothingEnabled,
	CommandExamplesTooLong,
	CommandExamplesPostFailed,

	DTGPageParamsInvalid,
	LocationPageParamsInvalid,
	AirportPageInvalid,

	PackagesNoBundlePath,
	PackagesBadName,
	PackagesBadArchive,
	PackagesUploadBadName,
	PackagesUploadNoDir,
	PackagesUploadTooLarge,
	PackagesUploadNotAnArchive,
	PackagesUploadWriteFailed,
	PackagesSchemaMismatch,
}
