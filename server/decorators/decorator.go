// Package decorators provides a generic framework for turning domain tokens in
// post messages into markdown links whose query string carries already-parsed
// data.
//
// # Adding a decorator
//
// A decorator is one directory implementing Decorator, plus one line in
// OnActivate registering it. No file in this package needs to change.
//
//  1. Create server/decorators/<type>/ with a type implementing Decorator.
//  2. Register it: decorators.Register(&mytype.Decorator{}).
//  3. Add the matching TypeScript decorator under webapp/src/decorators/<type>/
//     and register it in registerBuiltinDecorators().
//
// The webapp side never parses the token. It reads the query params this side
// produced, so the grammar lives in Go only.
package decorators

import (
	"net/http"
	"net/url"
	"regexp"
	"time"
)

// ForcePageParam makes the webapp leave a decorator link alone, so the browser
// navigates to the standalone page instead of opening the right-hand sidebar.
//
// It exists for testing: the page is normally only reachable from a client that
// does not run the webapp bundle, such as the mobile app, which makes it awkward
// to check from a desktop browser.
//
// The leading underscore marks it as belonging to the framework. Decorators own
// every other parameter name, so reserving a prefix keeps a future decorator
// from colliding with this.
const ForcePageParam = "_page"

// ThemeParam carries the reader's Mattermost theme to a decorator page.
//
// The page is a separate document, so it cannot read the webapp's CSS
// variables the way a sidebar panel can. Without a hint it can only follow the
// operating system's light or dark preference, which is a different setting: a
// light Mattermost on a dark laptop would open a dark page.
//
// The webapp adds this when it hands a click off to the page. Clients that
// cannot know the theme, such as the mobile app's in-app browser, simply omit
// it and the page falls back to the operating system preference.
//
// Only "light" and "dark" are accepted. Anything else is ignored, because this
// value reaches a stylesheet and must never be interpolated freely.
const ThemeParam = "_theme"

// ThemeFromParams returns the requested theme, or "" to follow the operating
// system preference.
func ThemeFromParams(params url.Values) string {
	switch theme := params.Get(ThemeParam); theme {
	case "light", "dark":
		return theme
	default:
		return ""
	}
}

// Pattern is one regular expression a decorator matches against message text.
type Pattern struct {
	// Regexp must not be anchored to the start of the string; it is run over
	// the whole message.
	Regexp *regexp.Regexp

	// Extract pulls the canonical value out of a submatch set. Nil means
	// "use submatch 1", which is what almost every pattern wants.
	Extract func(m []string) string
}

// Value returns the canonical value for a submatch set.
//
// This is both what Parse is given and what the link is labelled with, which is
// what lets a pattern match more than it claims: a pattern that matches a
// labelled token such as "DTG: 091630ZAUG26" and captures only the time is
// replaced by a link reading "091630ZAUG26", with the label consumed.
//
// A pattern with no capture group gets the whole match, so this is the identity
// for every pattern that matches exactly what it means to link.
func (p Pattern) Value(m []string) string {
	if p.Extract != nil {
		return p.Extract(m)
	}
	if len(m) > 1 {
		return m[1]
	}
	return m[0]
}

// Decorator turns one kind of token into a link and renders the page behind it.
type Decorator interface {
	// Type is the URL path segment for this decorator and must be unique
	// across all registered decorators. It must be URL-safe.
	Type() string

	// Patterns are tried in registration order. Where two matches overlap the
	// longest wins, and registration order only breaks ties.
	Patterns() []Pattern

	// Parse validates a matched value against a reference time and returns the
	// query params encoding it. ok=false means "not really one of ours"; the
	// tagger then leaves the text alone and does not claim the range, so a
	// shorter valid match at the same span can still win.
	//
	// Parse must be pure, must not panic, and must be cheap: it runs inline on
	// the post path.
	Parse(value string, ref time.Time) (params url.Values, ok bool)

	// RenderPage writes the standalone page for these params.
	//
	// The /decorate/* route is PUBLIC. Params arrive from an untrusted URL and
	// must be re-validated here before anything is echoed.
	//
	// A decorator page must be a pure function of its query string: no post,
	// channel, user, team or config lookup, and it must never read or trust the
	// Mattermost-User-Id header. A decorator needing workspace data must not use
	// this route; it needs its own authenticated one.
	//
	// RenderPage owns its own Cache-Control header. The shell defaults to
	// no-store, so a decorator carrying anything non-public can simply say
	// nothing.
	RenderPage(w http.ResponseWriter, params url.Values)
}
