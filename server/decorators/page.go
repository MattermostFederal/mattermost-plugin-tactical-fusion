package decorators

import (
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"html"
	"net/http"
	"strings"
)

// Page is the content a decorator supplies to the shared HTML shell.
type Page struct {
	// Title goes in <title> and the page heading. Escaped by WritePage.
	Title string

	// BodyHTML is the decorator's own markup. The decorator is responsible for
	// escaping anything interpolated into it.
	BodyHTML string

	// Theme is "light", "dark", or "" to follow the operating system. Use
	// ThemeFromParams to derive it; any other value is treated as "".
	Theme string

	// StyleCSS is appended to the shared stylesheet, for rules only this
	// decorator needs.
	//
	// It is emitted inside <style> without escaping, so it must be a constant
	// in the decorator's own source and must never carry anything derived from
	// a request. WritePage refuses a value containing "<" for that reason,
	// which stops the one mistake that would turn a stylesheet into markup.
	StyleCSS string

	// ScriptJS is the page's inline JavaScript, without the <script> tags.
	//
	// Empty, the zero value, emits script-src 'none', so a page with no script
	// cannot be made to run one by an escaping mistake.
	//
	// A page that supplies one is served under script-src 'sha256-...', pinned
	// to the digest of exactly this text, rather than under 'unsafe-inline'.
	// That distinction is the point rather than fastidiousness: these pages echo
	// author text from a message, on a route whose query string anybody
	// can write, so the property worth keeping is that an escaping mistake
	// stays inert. Under 'unsafe-inline' an injected <script> runs; under a
	// hash it does not match the digest and is blocked.
	//
	// It is emitted without escaping, so like StyleCSS it must be a constant in
	// the decorator's own source and must never carry anything from a request.
	ScriptJS string

	// ScriptSrc is a same-origin script for the page to load, or "", written
	// relative to the page's own route. Emitted only under PageMapping.
	ScriptSrc string

	// Capability is how much of default-src 'none' this page gives back. The
	// zero value gives back nothing. See CLAUDE.md, "The page content policy".
	Capability PageCapability
}

type PageCapability int

const (
	PageStatic PageCapability = iota
	PageMapping
)

// styleCSS returns the decorator's extra rules, or nothing when they are
// unusable.
//
// A "<" in a stylesheet can close the <style> element and start markup, so a
// value containing one is dropped entirely rather than escaped: this field is
// documented as a source constant, and a constant does not contain "<".
func (p Page) styleCSS() string {
	if strings.Contains(p.StyleCSS, "<") {
		return ""
	}
	return p.StyleCSS
}

func (p Page) scriptPolicy() string {
	sources := ""
	if p.Capability == PageMapping {
		sources = " 'self'"
	}

	js := p.scriptJS()
	if js == "" {
		if sources == "" {
			return "script-src 'none'"
		}
		return "script-src" + sources
	}

	sum := sha256.Sum256([]byte(js))
	return "script-src" + sources + " 'sha256-" + base64.StdEncoding.EncodeToString(sum[:]) + "'"
}

func (p Page) contentPolicy() string {
	directives := []string{
		"default-src 'none'",
		"style-src 'unsafe-inline'",
		p.scriptPolicy(),
	}

	// style-src keeps only 'unsafe-inline': MapLibre's stylesheet arrives through
	// style-loader as an injected <style>, and nothing emits a <link>. img-src
	// keeps only data:, which is the zoom control's glyphs; the map draws through
	// WebGL, not <img>, and the style has no sprite, no glyphs and no raster
	// source. Every source not listed here is one less exfiltration channel on a
	// route that echoes author text.
	if p.Capability == PageMapping {
		directives = append(directives,
			"worker-src 'self'",
			"img-src data:",
			"connect-src 'self'")
	}

	directives = append(directives,
		"base-uri 'none'", "form-action 'none'", "frame-ancestors 'none'")

	return strings.Join(directives, "; ")
}

// scriptJS returns the page's script, or nothing when it is unusable.
//
// A "</script" in the body would close the element early and let whatever
// follows be parsed as markup, which is the one mistake that turns a script
// block into an injection point. Dropped entirely rather than escaped, for the
// same reason styleCSS drops a "<": this field is documented as a source
// constant, and a constant does not contain one.
func (p Page) scriptJS() string {
	if strings.Contains(strings.ToLower(p.ScriptJS), "</script") {
		return ""
	}
	return p.ScriptJS
}

func (p Page) scriptTag() string {
	out := ""

	if src := p.scriptSrc(); src != "" {
		out += `<script src="` + html.EscapeString(src) + `" defer></script>`
	}

	if js := p.scriptJS(); js != "" {
		out += "<script>" + js + "</script>"
	}

	return out
}

func (p Page) scriptSrc() string {
	src := p.ScriptSrc
	if src == "" || p.Capability != PageMapping {
		return ""
	}
	if !strings.HasPrefix(src, "./") && !strings.HasPrefix(src, "../") {
		return ""
	}
	return src
}

// themeAttribute renders the theme as a root attribute, or nothing at all.
//
// The value reaches a stylesheet, so it is re-checked here rather than trusted
// from the caller. Only the two known keywords can ever be emitted.
func (p Page) themeAttribute() string {
	switch p.Theme {
	case "light", "dark":
		return ` data-theme="` + p.Theme + `"`
	default:
		return ""
	}
}

// pageShell is a self-contained, mobile-friendly, theme-aware document. It has
// no external assets: the clients this page exists for may have no Mattermost
// session, and some will be an in-app browser on a phone.
//
// # The urgent pulse
//
// .countdown.urgent animates for every reader, with no prefers-reduced-motion
// guard. That is a decision, not an oversight: an imminent DTG is operational
// information and was judged to be worth drawing the eye regardless of the
// reader's motion preference. Do not add the guard back without asking.
//
// Two mitigations stand in its place, and both are asserted by
// TestUrgentPulseIsAccessible:
//
//   - the rate is roughly 0.8Hz, against the 3Hz that can trigger
//     photosensitivity, so shortening the duration materially needs the same
//     conversation again;
//   - the left bar and the color carry the signal on their own, so a reader
//     who does not perceive the movement still sees an urgent countdown.
const pageShell = `<!DOCTYPE html>
<html lang="en"%s>
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<meta name="referrer" content="no-referrer">
<title>%s</title>
<style>
/*
 * Light is the base. Dark is applied twice: once for readers whose operating
 * system prefers it, and once for an explicit data-theme, which is how the
 * webapp makes this page match the sidebar it was opened from.
 *
 * The media query is guarded so an explicit light theme still wins on a dark
 * machine, which is the mismatch this exists to fix.
 */
:root { color-scheme: light; --fg: #3f4350; --muted: #5c6470; --bg: #fff; --line: rgba(61,60,64,.12); --accent: #1c58d9; --urgent: #d24b4e; }
@media (prefers-color-scheme: dark) {
  :root:not([data-theme="light"]) { color-scheme: dark; --fg: #dddfe4; --muted: #9aa2ad; --bg: #1b1d22; --line: rgba(255,255,255,.12); --accent: #7ba7ff; --urgent: #ff6b6b; }
}
:root[data-theme="dark"] { color-scheme: dark; --fg: #dddfe4; --muted: #9aa2ad; --bg: #1b1d22; --line: rgba(255,255,255,.12); --accent: #7ba7ff; --urgent: #ff6b6b; }
* { box-sizing: border-box; }
body { margin: 0; padding: 24px 16px 48px; background: var(--bg); color: var(--fg);
  font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif; -webkit-text-size-adjust: 100%%; }
main { max-width: 560px; margin: 0 auto; }
h1 { font-size: 15px; font-weight: 600; letter-spacing: .04em; text-transform: uppercase; color: var(--muted); margin: 0 0 16px; }
.countdown { font-family: ui-monospace, SFMono-Regular, Menlo, monospace; font-size: 30px; font-weight: 600; margin: 0 0 8px; }
.countdown.urgent { color: var(--urgent); border-left: 4px solid var(--urgent); padding-left: 10px;
  animation: mc-pulse 1.2s ease-in-out infinite; }
@keyframes mc-pulse { 50%% { opacity: .35; } }
.described { font-size: 16px; margin: 0 0 4px; }
.dtg { font-family: ui-monospace, SFMono-Regular, Menlo, monospace; font-size: 13px; color: var(--muted); margin: 0; word-break: break-all; }
.sub { color: var(--muted); font-size: 14px; margin: 0 0 4px; }
.note { color: var(--muted); font-size: 13px; margin: 12px 0 0; font-style: italic; }
table { width: 100%%; border-collapse: collapse; margin-top: 24px; font-size: 14px; }
th { text-align: left; font-size: 11px; text-transform: uppercase; letter-spacing: .04em; color: var(--muted);
  font-weight: 600; padding: 8px 10px; border-bottom: 1px solid var(--line); }
td { padding: 10px; border-bottom: 1px solid var(--line); vertical-align: middle; }
td.time { font-family: ui-monospace, SFMono-Regular, Menlo, monospace; text-align: right; white-space: nowrap; }
.abbr { color: var(--muted); font-size: 11px; margin-left: 6px; }
.badge { font-size: 10px; font-weight: 600; padding: 1px 5px; border-radius: 3px; margin-left: 6px;
  border: 1px solid var(--line); color: var(--muted); }
footer { margin-top: 32px; color: var(--muted); font-size: 12px; }
%s
</style>
</head>
<body>
<main>
<h1>%s</h1>
%s
</main>
%s
</body>
</html>`

// WritePage renders a decorator page inside the shared shell.
//
// It sets Content-Type and, if the decorator has not already chosen one, a
// conservative Cache-Control of no-store. Cache policy is decorator-owned: a
// framework-wide default of "public" would be wrong the moment a decorator
// carries anything that is not.
func WritePage(w http.ResponseWriter, p Page) {
	setPageHeaders(w, p)
	writePageBody(w, p)
}

// WriteError renders a minimal error page. Nothing from the request is echoed,
// because the request is what we just rejected.
//
// The error page carries no script, so it is served under script-src 'none'
// whatever the page that failed would have asked for.
func WriteError(w http.ResponseWriter, status int, message string) {
	p := Page{
		Title:    "Tactical Fusion",
		BodyHTML: `<p class="sub">` + html.EscapeString(message) + `</p>`,
	}

	setPageHeaders(w, p)
	w.WriteHeader(status)
	writePageBody(w, p)
}

func setPageHeaders(w http.ResponseWriter, p Page) {
	h := w.Header()
	h.Set("Content-Type", "text/html; charset=utf-8")

	// This route echoes values from an untrusted query string.
	// Escaping is the real defense, but a restrictive policy means a mistake in
	// it cannot become script execution or an exfiltration channel. Inline
	// styles are allowed because the shell carries its own and loads nothing
	// from anywhere else.
	//
	// Script is per page rather than blanket, and it is pinned by DIGEST. A
	// decorator whose page carries no JavaScript gets script-src 'none', so an
	// escaping mistake there is inert markup instead of execution; one that
	// carries a script gets 'sha256-...' over exactly the bytes served. Never
	// 'unsafe-inline', which is what this said before the digest existed: these
	// pages echo author text from a message on a route whose query string
	// anybody can write, so what has to survive an escaping mistake is that
	// injected markup cannot execute. A hash keeps that; 'unsafe-inline' would
	// hand it back.
	h.Set("Content-Security-Policy", p.contentPolicy())
	h.Set("Referrer-Policy", "no-referrer")

	if h.Get("Cache-Control") == "" {
		h.Set("Cache-Control", "no-store")
	}
}

func writePageBody(w http.ResponseWriter, p Page) {
	title := html.EscapeString(p.Title)
	_, _ = fmt.Fprintf(w, pageShell, p.themeAttribute(), title, p.styleCSS(), title, p.BodyHTML, p.scriptTag())
}
