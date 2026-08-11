// Package errcode holds the stable numeric catalogue of error codes this
// plugin emits, in logs and in user-facing failure messages alike.
//
// The point is that a reader who sees "(TF-13004)" in the sidebar and an
// operator grepping the server log are looking at the same identifier, and that
// public/help/troubleshooting.html can be organised around something that does
// not change when the wording does.
//
// Allocation, one thousand-wide range per source file:
//
//	10000-10999   server/plugin.go              activation
//	11000-11999   server/hooks.go               message decoration
//	12000-12999   server/http.go                public routing
//	13000-13999   server/api.go                 authenticated API
//	14000-14999   server/preferences.go         validation and storage
//	15000-15999   server/preferences_cache.go   cache and cluster events
//	16000-16999   server/command.go and server/command_examples.go
//	17000-17999   server/decorators/            framework and decorator pages
//
// Within a range codes are allocated in source order the first time a file is
// instrumented; a site added later takes the next free number in its range, so
// codes drift out of source order as a file grows. That is fine and expected.
//
// What is not: **never renumber a committed code, and never reuse a retired
// one.** Both are quoted in support tickets and neither can be taken back.
// Retire a code by leaving a comment in its place.
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

	// PreferencesBlobUnreadable is a warn recording that a stored blob could
	// not be parsed and the reader was served the defaults instead. View
	// settings must never be able to take the panel down with them.
	PreferencesBlobUnreadable = 14007

	// server/preferences_cache.go (15000-15999)

	// PreferencesCachePublishFailed is a warn recording that a cache
	// invalidation could not be published to the other nodes. Delivery is best
	// effort: the cost is one reader seeing a stale table until the TTL runs
	// out, which does not justify failing their save.
	PreferencesCachePublishFailed = 15000

	// server/command.go and server/command_examples.go (16000-16999)

	// CommandUnknownSubcommand is returned for a subcommand this plugin does
	// not have.
	CommandUnknownSubcommand = 16000

	// CommandExamplesNotReady is returned when /tactical-fusion examples runs
	// before OnActivate has built the registry.
	CommandExamplesNotReady = 16001

	// server/decorators/ (17000-17999)

	// DTGPageParamsInvalid is returned by the date-time group page for a link
	// whose query string carries no usable instant.
	DTGPageParamsInvalid = 17000
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

	HTTPMethodNotAllowed,
	HTTPDecoratePathInvalid,
	HTTPDecoratorsNotReady,
	HTTPDecoratorUnknown,

	APINotAuthorized,
	APINotFound,
	APINotReady,
	APIMethodNotAllowed,
	APIPreferencesReadFailed,
	APIPreferencesInvalidBody,
	APIPreferencesSaveFailed,
	APIPreferencesClearFailed,

	PreferencesZoneNameTooLong,
	PreferencesZoneNameControlCharacters,
	PreferencesZoneIDMalformed,
	PreferencesZoneIDLocal,
	PreferencesZoneIDUnknown,
	PreferencesTooManyZones,
	PreferencesThresholdOutOfRange,
	PreferencesBlobUnreadable,

	PreferencesCachePublishFailed,

	CommandUnknownSubcommand,
	CommandExamplesNotReady,

	DTGPageParamsInvalid,
}
