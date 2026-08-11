package decorators

import (
	"fmt"
	"html"
	"net/http"
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
//   - the left bar and the colour carry the signal on their own, so a reader
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
</style>
</head>
<body>
<main>
<h1>%s</h1>
%s
</main>
</body>
</html>`

// WritePage renders a decorator page inside the shared shell.
//
// It sets Content-Type and, if the decorator has not already chosen one, a
// conservative Cache-Control of no-store. Cache policy is decorator-owned: a
// framework-wide default of "public" would be wrong the moment a decorator
// carries anything that is not.
func WritePage(w http.ResponseWriter, p Page) {
	setPageHeaders(w)
	writePageBody(w, p)
}

// WriteError renders a minimal error page. Nothing from the request is echoed,
// because the request is what we just rejected.
func WriteError(w http.ResponseWriter, status int, message string) {
	setPageHeaders(w)
	w.WriteHeader(status)
	writePageBody(w, Page{
		Title:    "Tactical Fusion",
		BodyHTML: `<p class="sub">` + html.EscapeString(message) + `</p>`,
	})
}

func setPageHeaders(w http.ResponseWriter) {
	h := w.Header()
	h.Set("Content-Type", "text/html; charset=utf-8")

	// This route is public and echoes values from an untrusted query string.
	// Escaping is the real defence, but a restrictive policy means a mistake in
	// it cannot become script execution or an exfiltration channel. Inline
	// styles and the clock script are allowed because the page carries both and
	// loads nothing from anywhere else.
	h.Set("Content-Security-Policy",
		"default-src 'none'; style-src 'unsafe-inline'; script-src 'unsafe-inline'; base-uri 'none'; form-action 'none'; frame-ancestors 'none'")
	h.Set("Referrer-Policy", "no-referrer")

	if h.Get("Cache-Control") == "" {
		h.Set("Cache-Control", "no-store")
	}
}

func writePageBody(w http.ResponseWriter, p Page) {
	title := html.EscapeString(p.Title)
	_, _ = fmt.Fprintf(w, pageShell, p.themeAttribute(), title, title, p.BodyHTML)
}
