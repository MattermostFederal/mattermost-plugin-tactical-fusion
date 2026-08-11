// Package dtg decorates military Date-Time Groups.
package dtg

import (
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"time"

	"github.com/MattermostFederal/mattermost-plugin-mission-context/server/decorators"
)

// Type is the URL path segment and the key shared with the webapp decorator.
const Type = "dtg"

// pageTitle is the browser tab name and the page heading.
//
// A category rather than the value itself: the canonical DTG is already the
// first and largest line of the body, so repeating it in the heading would
// spend the space saying the same thing twice. Matches the sidebar header in
// webapp/src/decorators/dtg/index.ts.
const pageTitle = "Date/Time"

// Formats selects which token grammars are matched.
//
// This governs decoration only. RenderPage never consults it, because a link
// already written into a message must keep working after an admin turns its
// format off: switching one off stops new messages being decorated, it does not
// break the history.
type Formats struct {
	// Military is 091630ZAUG26 and its short form.
	Military bool

	// Moniker is the "DTG:" label in front of either of the others.
	Moniker bool

	// Timestamp is RFC 3339.
	Timestamp bool
}

// AllFormats is what a decorator with no selector matches.
var AllFormats = Formats{Military: true, Moniker: true, Timestamp: true}

// Decorator recognises DTGs and renders the timezone conversion page.
type Decorator struct {
	// Enabled reports which formats to match, read fresh for every message so
	// an admin toggle takes effect without a restart. Nil means all of them,
	// which is what any caller with no configuration wants.
	Enabled func() Formats
}

var _ decorators.Decorator = (*Decorator)(nil)

func (d *Decorator) Type() string { return Type }

// The token shapes, as sub-expressions.
//
// The bare patterns and the labelled one are both built from these, so a change
// to what a token looks like cannot reach one and miss the other.
const (
	// DDHHMM<Z>MMMYYYY and DDHHMM<Z>MMMYY.
	dtgLongExpr  = `\d{6}[A-Z][A-Za-z]{3}\d{4}`
	dtgShortYear = `\d{6}[A-Z][A-Za-z]{3}\d{2}`

	// DDHHMMZ, literal Z only.
	dtgBareExpr = `\d{6}Z`

	// RFC 3339, extended form only. The T and the mandatory zone make this
	// almost impossible to hit by accident, which is the whole reason it is
	// safe to claim in general chat. The basic form is deliberately absent:
	// see parseISO.
	isoExpr = `\d{4}-\d{2}-\d{2}[Tt]\d{2}:\d{2}(?::\d{2}(?:\.\d+)?)?(?:[Zz]|[+-]\d{2}:\d{2})`
)

// militaryExpr is every date-time group shape, longest first.
//
// Order matters: Go's regexp is leftmost-first, so the bare six-digit form goes
// last or it would match the head of a longer token and stop there.
const militaryExpr = dtgLongExpr + `|` + dtgShortYear + `|` + dtgBareExpr

// anyTokenExpr is every shape, of either grammar, on the same rule.
const anyTokenExpr = dtgLongExpr + `|` + dtgShortYear + `|` + isoExpr + `|` + dtgBareExpr

// Compiled once at start-up rather than per call. Patterns() runs for every
// message, and rebuilding these each time was pure waste.
//
// Go's regexp is RE2 and has no lookahead, so the trailing "not followed by an
// alphanumeric" guard is expressed with \b. That is equivalent here and also
// treats "_" as a word character, which is what we want: 091630Z_foo should not
// match either.
//
// Every group is non-capturing except the one in a moniker pattern, and has to
// be: Pattern.Value hands the first capture group to Parse and uses it as the
// link text, so a stray capturing group would silently pass a fragment along
// instead of the token.
var (
	militaryPatterns = []decorators.Pattern{
		{Regexp: regexp.MustCompile(`\b` + dtgLongExpr + `\b`)},
		{Regexp: regexp.MustCompile(`\b` + dtgShortYear + `\b`)},
		{Regexp: regexp.MustCompile(`\b` + dtgBareExpr + `\b`)},
	}

	timestampPattern = decorators.Pattern{Regexp: regexp.MustCompile(`\b` + isoExpr + `\b`)}

	// One per combination of enabled grammars, so the moniker only ever claims
	// a shape that is switched on in its own right. Turning military formats
	// off has to stop "DTG: 091630ZAUG26" too, or it would be half a switch.
	monikerAny       = monikerPattern(anyTokenExpr)
	monikerMilitary  = monikerPattern(militaryExpr)
	monikerTimestamp = monikerPattern(isoExpr)
)

// monikerPattern matches the label some military formats use to mark where a
// time starts, and captures only the time, so the link reads as the time alone
// and the label is consumed.
//
// Only the moniker is case-insensitive: the token keeps the casing rules it has
// on its own.
func monikerPattern(tokens string) decorators.Pattern {
	return decorators.Pattern{
		Regexp: regexp.MustCompile(`\b(?i:DTG)[ \t]*:[ \t]*(` + tokens + `)\b`),
	}
}

// Patterns returns the enabled patterns. The tagger resolves overlaps by match
// length, so their order here is only a tiebreak.
func (d *Decorator) Patterns() []decorators.Pattern {
	formats := AllFormats
	if d.Enabled != nil {
		formats = d.Enabled()
	}

	patterns := make([]decorators.Pattern, 0, len(militaryPatterns)+2)

	if moniker, ok := monikerFor(formats); ok {
		patterns = append(patterns, moniker)
	}
	if formats.Military {
		patterns = append(patterns, militaryPatterns...)
	}
	if formats.Timestamp {
		patterns = append(patterns, timestampPattern)
	}

	return patterns
}

// monikerFor picks the labelled pattern matching exactly the grammars that are
// switched on. ok=false when the moniker is off, or on but with nothing left to
// label.
func monikerFor(formats Formats) (decorators.Pattern, bool) {
	switch {
	case !formats.Moniker:
		return decorators.Pattern{}, false
	case formats.Military && formats.Timestamp:
		return monikerAny, true
	case formats.Military:
		return monikerMilitary, true
	case formats.Timestamp:
		return monikerTimestamp, true
	default:
		return decorators.Pattern{}, false
	}
}

// Parse validates a candidate and returns the params encoding it.
//
// The resolved instant travels in "t" so that neither the RHS panel nor the
// standalone page repeats the zone arithmetic, and neither can disagree with
// this side about what the DTG means.
func (d *Decorator) Parse(value string, ref time.Time) (url.Values, bool) {
	if parsed, ok := parseDTG(value, ref); ok {
		return url.Values{
			"t":   {strconv.FormatInt(parsed.resolveInstant().UnixMilli(), 10)},
			"dtg": {parsed.canonical()},
			"z":   {string(parsed.Zone)},
			"a":   {parsed.assumedCode()},
		}, true
	}

	// A timestamp carries an offset rather than a zone letter, so it says so in
	// "o" instead of "z". Exactly one of the two is ever present, which is what
	// tells the two forms apart on the way back in, and it leaves every link
	// already written into a message reading exactly as it did.
	if parsed, ok := parseISO(value); ok {
		return url.Values{
			"t":   {strconv.FormatInt(parsed.Instant.UnixMilli(), 10)},
			"dtg": {parsed.canonical()},
			"o":   {strconv.Itoa(parsed.OffsetMinutes)},
			"a":   {""},
		}, true
	}

	return nil, false
}

// canonicalRe matches the canonical "dtg" parameter grammar for a date-time
// group. Params arrive from an untrusted URL, so nothing is echoed before it
// matches this.
var canonicalRe = regexp.MustCompile(`^\d{6}[A-Z]([A-Z]{3}\d{2})?$`)

// isoCanonicalRe matches the canonical form parseISO produces: always seconds,
// never a fraction, and a zero offset written as Z.
var isoCanonicalRe = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(Z|[+-]\d{2}:\d{2})$`)

// Sane bounds for the "t" parameter: 1970 through 2200. Wide enough for any
// real DTG, narrow enough that a garbage value cannot reach time formatting.
const (
	minInstantMillis int64 = 0
	maxInstantMillis int64 = 7_258_118_400_000
)

// RenderPage renders the timezone table for these params.
//
// This route is public and this page is a pure function of its query string: it
// reads no workspace data and never looks at Mattermost-User-Id. See the
// Decorator interface documentation before copying this pattern.
func (d *Decorator) RenderPage(w http.ResponseWriter, params url.Values) {
	page, ok := validateParams(params)
	if !ok {
		decorators.WriteError(w, http.StatusBadRequest, "That link is missing or has an invalid date-time group.")
		return
	}

	// Not cached: the body carries "in 3h 20m" and a wall clock, so it is not a
	// function of the params alone and a shared cache would serve a stale
	// offset for as long as it held the entry. Cache policy is decorator-owned
	// by design, and this decorator cannot honestly claim to be cacheable.
	w.Header().Set("Cache-Control", "no-store")

	decorators.WritePage(w, decorators.Page{
		Title:    pageTitle,
		BodyHTML: renderBody(page),
		Theme:    decorators.ThemeFromParams(params),
	})
}

// pageData is one validated payload, whichever form it arrived in.
//
// Downstream of here a date-time group and a timestamp are the same thing: an
// instant, the offset it was written in, and a label for that offset.
type pageData struct {
	instant   time.Time
	canonical string

	// offsetMinutes is the token's own offset from UTC, which the reading is
	// rendered in and every row's day badge is measured against.
	offsetMinutes int

	// zoneLabel is how that offset is written: a military letter for a
	// date-time group, "Z" or "+05:30" for a timestamp.
	zoneLabel string

	assumed string
}

// validateParams re-derives the whole payload from the query string and rejects
// anything self-inconsistent.
//
// Validating each parameter in isolation is not enough on a public route where
// the URL is user-supplied: a crafted link could pair an arbitrary token with an
// unrelated instant and a third zone, and the page would render all three side
// by side as though they agreed. Re-parsing the canonical form and requiring it
// to reproduce the instant, the offset and the assumed flags removes that whole
// class rather than the individual combinations.
func validateParams(params url.Values) (pageData, bool) {
	millis, err := strconv.ParseInt(params.Get("t"), 10, 64)
	if err != nil || millis < minInstantMillis || millis > maxInstantMillis {
		return pageData{}, false
	}
	instant := time.UnixMilli(millis).UTC()

	canonical := params.Get("dtg")

	// Exactly one of the two zone parameters, which is what tells a date-time
	// group from a timestamp. A link carrying both is not one this plugin wrote.
	hasZone := params.Has("z")
	hasOffset := params.Has("o")
	if hasZone == hasOffset {
		return pageData{}, false
	}

	if hasOffset {
		return validateISOParams(params, instant, canonical)
	}

	return validateDTGParams(params, instant, canonical)
}

func validateDTGParams(params url.Values, instant time.Time, canonical string) (pageData, bool) {
	if !canonicalRe.MatchString(canonical) {
		return pageData{}, false
	}

	// Resolving against the claimed instant is what makes the short form
	// checkable: its month and year come from the reference, so a self
	// consistent pair reproduces the instant exactly.
	parsed, ok := parseDTG(canonical, instant)
	if !ok || !parsed.resolveInstant().Equal(instant) {
		return pageData{}, false
	}
	if params.Get("z") != string(parsed.Zone) {
		return pageData{}, false
	}
	if params.Get("a") != parsed.assumedCode() {
		return pageData{}, false
	}

	offset, _ := zoneOffsetHours(parsed.Zone)

	return pageData{
		instant:       instant,
		canonical:     canonical,
		offsetMinutes: offset * 60,
		zoneLabel:     string(parsed.Zone),
		assumed:       parsed.assumedCode(),
	}, true
}

func validateISOParams(params url.Values, instant time.Time, canonical string) (pageData, bool) {
	if !isoCanonicalRe.MatchString(canonical) {
		return pageData{}, false
	}

	parsed, ok := parseISO(canonical)
	if !ok || !parsed.Instant.Equal(instant) {
		return pageData{}, false
	}

	offset, err := strconv.Atoi(params.Get("o"))
	if err != nil || offset != parsed.OffsetMinutes {
		return pageData{}, false
	}

	// A timestamp carries its own date, so nothing about it was ever assumed.
	if params.Get("a") != "" {
		return pageData{}, false
	}

	return pageData{
		instant:       instant,
		canonical:     canonical,
		offsetMinutes: offset,
		zoneLabel:     FormatOffset(offset),
		assumed:       "",
	}, true
}
