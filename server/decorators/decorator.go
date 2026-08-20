// Package decorators provides a generic framework for turning domain tokens in
// post messages into markdown links whose query string carries already-parsed
// data.
//
// # Adding a decorator
//
// A decorator is one directory implementing Decorator, plus one argument in
// OnActivate registering it. No file in this package needs to change.
//
//  1. Create server/decorators/<type>/ with a type implementing Decorator.
//  2. Pass it to the NewDefaultRegistry call in OnActivate:
//     decorators.NewDefaultRegistry(&dtg.Decorator{...}, &mytype.Decorator{}).
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
	"strings"
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

	// ReplaceGroup is the submatch whose span is rewritten into a link. Zero,
	// the default, rewrites the whole match.
	//
	// The two differ only for a pattern that deliberately matches more than it
	// rewrites. Every moniker here wants the label consumed, so the DTG and
	// location monikers take the default and the whole match becomes the link.
	//
	// The airfield pattern is the one that needs this: it matches the "//" a
	// USMTF set line ends with, so that the boundary guard can look past it,
	// but that terminator has to stay in the message. Its ReplaceGroup names
	// the label and the ident together, leaving the "//" outside the rewrite.
	//
	// The whole match is still what protected ranges are tested against, so a
	// moniker inside a code span protects the token behind it. Only the
	// substitution and the overlap claim use this narrower span.
	ReplaceGroup int

	// Boundary reports whether a match is acceptable given the runes on either
	// side of it. Nil, the default, imposes no constraint.
	//
	// before and after are the runes immediately outside the match, or 0 at the
	// start or end of the message.
	//
	// This exists because \b is the wrong guard for a token that does not start
	// and end with a word character. A coordinate begins with "-" or a digit and
	// ends with a digit or a quote, so \b before "-118" asserts the opposite of
	// what is wanted and \b after "2500" is satisfied by a following "." -
	// which would let a link be written into the middle of "-118.2500..-118.2600".
	//
	// The guard deliberately lives here rather than in the expression. A pattern
	// that consumed its own guard characters would break the *next* match:
	// FindAllStringSubmatchIndex returns non-overlapping matches, so the first
	// token would eat the space that the second one needs as its leading guard,
	// and the second token would silently go undecorated.
	Boundary func(before, after rune) bool
}

// Value returns the canonical value for a submatch set.
//
// This is both what Parse is given and what the link is labeled with, which is
// what lets a pattern match more than it claims: a pattern that matches a
// labeled token such as "DTG: 091630ZAUG26" and captures only the time is
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
	// Params arrive from a URL anybody can write and must be re-validated here
	// before anything is echoed. ServeHTTP requires a session to reach this, but
	// a session says who is asking and nothing about what they asked for.
	//
	// A decorator page must be a pure function of its query string: no post,
	// channel, user, team or config lookup, and it is handed no reader. That is
	// what keeps a page renderable from a test with a url.Values and nothing
	// else, and what keeps a route served with a cache lifetime from growing a
	// per-reader answer. A decorator needing workspace data needs its own route
	// under /api/v1.
	//
	// RenderPage owns its own Cache-Control header. The shell defaults to
	// no-store, so a decorator carrying anything non-public can simply say
	// nothing.
	RenderPage(w http.ResponseWriter, params url.Values)
}

type PostRenderer interface {
	PostType() string
}

const (
	PostTypePrefix   = "custom_"
	PostTypeMaxLen   = 26
	PostPropsKey     = "tactical_fusion"
	PostPropsVersion = 1
)

func StandalonePostType(d Decorator) string {
	renderer, ok := d.(PostRenderer)
	if !ok {
		return ""
	}

	postType := renderer.PostType()
	if !strings.HasPrefix(postType, PostTypePrefix) || len(postType) > PostTypeMaxLen {
		return ""
	}
	return postType
}

func StandalonePostProps(r Result) map[string]any {
	props := map[string]any{
		"version": PostPropsVersion,
		"type":    r.Type,
	}
	for key := range r.Params {
		props[key] = r.Params.Get(key)
	}
	return props
}
