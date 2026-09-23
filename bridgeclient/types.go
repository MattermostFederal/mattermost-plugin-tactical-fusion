package bridgeclient

// PluginID is the Tactical Fusion plugin id, which is the first path segment of
// every PluginHTTP request this package sends.
const PluginID = "com.mattermost.plugin-tactical-fusion"

// APIVersion is the version of the bridge contract this package speaks. Within
// a version, changes are additive only: new optional request fields, new
// response fields and new types.
const APIVersion = 1

// BridgePath is the route prefix the bridge is served under, relative to the
// plugin's own base path.
const BridgePath = "/bridge/v1"

// Decorator types accepted by Link.
const (
	// TypeDTG is a date-time group or an RFC 3339 timestamp, such as
	// "141200ZSEP26" or "2026-08-09T16:30:00Z".
	TypeDTG = "dtg"

	// TypeLocation is a coordinate in any grammar Tactical Fusion reads, such
	// as "18S UJ 23478 06483" or "34.0561, -118.2500".
	TypeLocation = "location"

	// TypeAirport is a four-letter ICAO airfield ident, such as "PHIK".
	TypeAirport = "airport"

	// TypeAvReport is a METAR, SPECI, TAF or FAA-format NOTAM on one line,
	// exactly as written.
	TypeAvReport = "avreport"

	// TypeFrequency is a radio frequency as an author writes it behind FREQ:,
	// such as "121.5", "118.300 MHZ" or "8992 KHZ".
	TypeFrequency = "frequency"

	// TypeNote is markdown, up to 1,000 characters, that the link's hover card
	// and sidebar render with Mattermost's own markdown renderer.
	TypeNote = "note"
)

// Reasons a Link request is declined, carried in ErrorResponse.Reason.
const (
	// ReasonUnknownType means the type names no decorator this build has.
	ReasonUnknownType = "unknown_type"

	// ReasonNotRecognized means the token is not one the type reads.
	ReasonNotRecognized = "not_recognized"

	// ReasonDisabled means the token is well formed but an administrator has
	// switched its format off.
	ReasonDisabled = "disabled"
)

// DecorateRequest asks for every recognized token in Message to be rewritten
// as a decorator link.
type DecorateRequest struct {
	// Message is markdown text. Tokens inside code, links, URLs and other
	// protected spans are left exactly as written.
	Message string `json:"message"`

	// ReferenceTime, in Unix milliseconds, supplies the month and year for a
	// short date-time group such as "091630Z". Zero means now.
	ReferenceTime int64 `json:"reference_time,omitempty"`
}

// DecorateResponse is the decorated text.
type DecorateResponse struct {
	// Message is the input with decorator links written in. It is markdown and
	// must be rendered as markdown, never escaped.
	Message string `json:"message"`

	// Changed reports whether any token was linked.
	Changed bool `json:"changed"`

	// FitsPost reports whether Message fits the smallest post size limit any
	// Mattermost server enforces (4,000 runes). A message that does not fit may
	// still post on a server with a larger limit.
	FitsPost bool `json:"fits_post"`
}

// LinkRequest asks for one decorator link for a token of a known type.
type LinkRequest struct {
	// Type is TypeDTG, TypeLocation, TypeAirport, TypeAvReport or TypeFrequency.
	Type string `json:"type"`

	// Token is the value alone, with no field label: "PHIK" rather than
	// "ICAO:PHIK", and "18S UJ 23478 06483" rather than "MGRS:18S UJ 23478 06483".
	Token string `json:"token"`

	// Label is the link text. Empty means the token as written. Markdown
	// metacharacters in it are escaped.
	Label string `json:"label,omitempty"`

	// ReferenceTime, in Unix milliseconds, supplies the month and year for a
	// short date-time group. Zero means now.
	ReferenceTime int64 `json:"reference_time,omitempty"`
}

// LinkResponse is one decorator link.
type LinkResponse struct {
	// Markdown is the complete link, "[label](url)", ready to embed in a
	// message.
	Markdown string `json:"markdown"`

	// URL is the root-relative link destination. It carries no scheme or host,
	// so it follows whichever server the reader is on.
	URL string `json:"url"`

	// Type is the decorator type that built the link.
	Type string `json:"type"`

	// Label is the link text before markdown escaping.
	Label string `json:"label"`

	// FitsPost reports whether Markdown alone fits the smallest post size limit
	// any Mattermost server enforces (4,000 runes). A note's markdown travels
	// URL-encoded, so its link can outgrow a post.
	FitsPost bool `json:"fits_post"`
}

// InfoResponse describes what the installed plugin offers.
type InfoResponse struct {
	// PluginVersion is the installed Tactical Fusion version.
	PluginVersion string `json:"plugin_version"`

	// APIVersion is the bridge contract version the plugin serves.
	APIVersion int `json:"api_version"`

	// Types lists every decorator type the plugin has.
	Types []string `json:"types"`

	// EnabledTypes lists the types whose decoration an administrator has left
	// on. A type can be enabled while some of its individual formats are off.
	EnabledTypes []string `json:"enabled_types"`
}

// ErrorResponse is the body of every non-2xx bridge response.
type ErrorResponse struct {
	// Message is human readable and ends with a "(TF-NNNNN)" code.
	Message string `json:"message"`

	// Code is the numeric TF code, documented on the plugin's error codes page.
	Code int `json:"code"`

	// Reason is set on a declined Link request: ReasonUnknownType,
	// ReasonNotRecognized or ReasonDisabled.
	Reason string `json:"reason,omitempty"`
}
