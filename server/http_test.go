package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/mattermost/mattermost/server/public/plugin"

	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/decorators"
	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/errcode"
)

// assertCode fails unless a rendered failure carries the given error code.
//
// A status alone cannot tell two failures apart: four different things answer
// 404 here. The code is what distinguishes them in a log or a support ticket,
// so it is worth asserting that each path emits its own.
//
// One helper serves both surfaces, because the HTML shell and the JSON error
// body each put the message into the payload verbatim.
func assertCode(t *testing.T, body string, code int) {
	t.Helper()

	if want := fmt.Sprintf("(%s-%d)", errcode.Prefix, code); !strings.Contains(body, want) {
		t.Fatalf("response does not carry %s: %s", want, body)
	}
}

// withSession stamps the header Mattermost sets for a request carrying a valid
// session. Every page route needs one now, so every test expecting a page has
// to build its request through here.
//
// The gate's own tests deliberately do not, and say so where they do not.
func withSession(req *http.Request) *http.Request {
	req.Header.Set("Mattermost-User-Id", "reader1")
	return req
}

// panicDecorator panics on Parse. Used by the hook tests to prove a bug in a
// decorator cannot stop somebody from posting.
type panicDecorator struct{}

func (*panicDecorator) Type() string { return "panic" }

func (*panicDecorator) Patterns() []decorators.Pattern {
	return []decorators.Pattern{{Regexp: regexp.MustCompile(`boom`)}}
}

func (*panicDecorator) Parse(string, time.Time) (url.Values, bool) {
	panic("decorator exploded")
}

func (*panicDecorator) RenderPage(http.ResponseWriter, url.Values) {}

func TestServeHTTPRendersKnownDecorator(t *testing.T) {
	p := newTestPlugin(t, "https://example.com", true)

	rec := httptest.NewRecorder()
	req := withSession(httptest.NewRequest(http.MethodGet,
		"/decorate/dtg?t=1786293000000&dtg=091630ZAUG26&z=Z&a=", nil))

	p.ServeHTTP(&plugin.Context{}, rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "091630ZAUG26") {
		t.Fatal("body does not contain the DTG")
	}
	if got := rec.Header().Get("X-Content-Type-Options"); got != "nosniff" {
		t.Fatalf("X-Content-Type-Options = %q, want nosniff", got)
	}
}

// Both page routes require a session, and a reader without one is sent to the
// login rather than refused: the usual case is somebody whose session expired
// between reading a channel and tapping a link in it.
func TestPageRoutesRedirectWithoutASession(t *testing.T) {
	p := newTestPlugin(t, "https://example.com", true)

	for _, path := range []string{
		"/decorate/dtg?t=1786293000000&dtg=091630ZAUG26&z=Z&a=",
		"/map?f=mgrs&v=18SUJ2347806483",
	} {
		t.Run(path, func(t *testing.T) {
			rec := httptest.NewRecorder()
			// Explicitly no Mattermost-User-Id.
			p.ServeHTTP(&plugin.Context{}, rec, httptest.NewRequest(http.MethodGet, path, nil))

			if rec.Code != http.StatusFound {
				t.Fatalf("status = %d, want 302 with no session", rec.Code)
			}
			if got := rec.Header().Get("Location"); !strings.HasPrefix(got, "/login?redirect_to=") {
				t.Fatalf("Location = %q, want a login redirect", got)
			}

			// A cached redirect keeps bouncing a reader who has since signed in.
			if got := rec.Header().Get("Cache-Control"); got != "no-store" {
				t.Fatalf("Cache-Control = %q, want no-store", got)
			}
		})
	}
}

// The reader has to come back to the coordinate they clicked. Without the query
// string they would sign in and land on a page that does not know what they
// asked for.
func TestLoginRedirectCarriesTheWholeRequestBack(t *testing.T) {
	p := newTestPlugin(t, "https://example.com", true)

	rec := httptest.NewRecorder()
	p.ServeHTTP(&plugin.Context{}, rec,
		httptest.NewRequest(http.MethodGet, "/map?f=mgrs&v=18SUJ2347806483", nil))

	got := rec.Header().Get("Location")
	want := "/login?redirect_to=" + url.QueryEscape(
		"/plugins/"+manifest.Id+"/map?f=mgrs&v=18SUJ2347806483")
	if got != want {
		t.Fatalf("Location = %q, want %q", got, want)
	}
}

// The subpath install, which is the case that breaks silently: the redirect
// renders, the reader signs in, and lands on a 404 at the server root.
func TestLoginRedirectKeepsTheSubpath(t *testing.T) {
	p := newTestPlugin(t, "https://example.com/mattermost", true)

	rec := httptest.NewRecorder()
	p.ServeHTTP(&plugin.Context{}, rec,
		httptest.NewRequest(http.MethodGet, "/decorate/dtg?dtg=091630ZAUG26", nil))

	got := rec.Header().Get("Location")
	want := "/mattermost/login?redirect_to=" + url.QueryEscape(
		"/mattermost/plugins/"+manifest.Id+"/decorate/dtg?dtg=091630ZAUG26")
	if got != want {
		t.Fatalf("Location = %q, want %q", got, want)
	}
}

// Root-relative, for the same reason the decorated links themselves are: an
// absolute URL bakes in a hostname, and this one bakes it into the address bar
// of somebody who is mid-login.
func TestLoginRedirectIsRootRelative(t *testing.T) {
	// The last four are the ways this value reaches off-origin, and the
	// assertion below already stated the property before any of them were fed
	// to it. A browser reads "//x" as scheme-relative, folds "\" to "/" for a
	// special scheme where Go does not, and strips TAB/LF/CR before parsing,
	// so all four resolve to another host if the path is emitted decoded.
	for _, siteURL := range []string{
		"https://example.com", "https://example.com/mattermost", "", "not a url",
		"https://example.com//elsewhere",
		"https://example.com/\\elsewhere",
		"https://example.com/%5Celsewhere",
		"https://example.com/%09/elsewhere",
	} {
		p := newTestPlugin(t, siteURL, true)

		rec := httptest.NewRecorder()
		p.ServeHTTP(&plugin.Context{}, rec,
			httptest.NewRequest(http.MethodGet, "/map?f=mgrs&v=18SUJ2347806483", nil))

		got := rec.Header().Get("Location")
		if !strings.HasPrefix(got, "/") || strings.HasPrefix(got, "//") {
			t.Fatalf("SiteURL %q gave Location %q, want a rooted relative path", siteURL, got)
		}
		if strings.Contains(got, "example.com") {
			t.Fatalf("SiteURL %q leaked a host into Location %q", siteURL, got)
		}

		// A browser folds a backslash to a slash and strips a tab, so a
		// Location a browser would read as "//host" is off-origin however Go
		// spells it. Checked on the second character, which is the only place
		// either can produce an authority.
		if len(got) > 1 && (got[1] == '\\' || got[1] == '\t') {
			t.Fatalf("SiteURL %q gave Location %q, which a browser reads as an authority",
				siteURL, got)
		}
	}
}

func TestServeHTTPNotFound(t *testing.T) {
	p := newTestPlugin(t, "https://example.com", true)

	// The code says which 404 this is. A path that names no decorator at all
	// and a path naming one that does not exist are different failures, and
	// only the code tells an operator which happened.
	cases := []struct {
		name string
		path string
		code int
	}{
		{"unknown decorator type", "/decorate/nope", errcode.HTTPDecoratorUnknown},
		{"no type", "/decorate/", errcode.HTTPDecoratePathInvalid},
		{"decorate root", "/decorate", errcode.HTTPDecoratePathInvalid},
		{"nested path", "/decorate/dtg/extra", errcode.HTTPDecoratePathInvalid},
		{"unrelated path", "/something-else", errcode.HTTPDecoratePathInvalid},
		{"root", "/", errcode.HTTPDecoratePathInvalid},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			p.ServeHTTP(&plugin.Context{}, rec,
				withSession(httptest.NewRequest(http.MethodGet, tc.path, nil)))

			if rec.Code != http.StatusNotFound {
				t.Fatalf("status = %d, want 404 for %s", rec.Code, tc.path)
			}
			assertCode(t, rec.Body.String(), tc.code)
		})
	}
}

// A request that lands before OnActivate has built the registry answers 404
// like an unknown type, but for an entirely different reason. Only the code
// separates the two, and telling an operator "the plugin was still starting"
// from "that decorator does not exist" is the whole point of having codes.
func TestServeHTTPBeforeActivationSaysSo(t *testing.T) {
	p := newTestPlugin(t, "https://example.com", true)
	p.decorators = nil

	rec := httptest.NewRecorder()
	p.ServeHTTP(&plugin.Context{}, rec,
		withSession(httptest.NewRequest(http.MethodGet, "/decorate/dtg", nil)))

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
	assertCode(t, rec.Body.String(), errcode.HTTPDecoratorsNotReady)
}

// Deliberately with no session, because the method check has to come first: a
// POST cannot be resumed after a login, so refusing it beats redirecting it.
func TestServeHTTPRejectsNonGET(t *testing.T) {
	p := newTestPlugin(t, "https://example.com", true)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/decorate/dtg", nil)

	p.ServeHTTP(&plugin.Context{}, rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want 405", rec.Code)
	}
	assertCode(t, rec.Body.String(), errcode.HTTPMethodNotAllowed)
}

// A page whose params fail validation must not be cached publicly.
func TestServeHTTPInvalidParamsAreNotCached(t *testing.T) {
	p := newTestPlugin(t, "https://example.com", true)

	rec := httptest.NewRecorder()
	req := withSession(httptest.NewRequest(http.MethodGet, "/decorate/dtg?t=abc", nil))

	p.ServeHTTP(&plugin.Context{}, rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
	if got := rec.Header().Get("Cache-Control"); got != "no-store" {
		t.Fatalf("Cache-Control = %q, want no-store", got)
	}
	assertCode(t, rec.Body.String(), errcode.DTGPageParamsInvalid)
}

// The map page is a sibling of /decorate, not a mode of it, and it is gated on
// the same terms.
func TestMapRouteServesTheMapToAReader(t *testing.T) {
	p := newTestPlugin(t, "https://example.com", true)

	req := withSession(httptest.NewRequest(http.MethodGet, "/map?f=mgrs&v=18SUJ2347806483", nil))
	rec := httptest.NewRecorder()
	p.ServeHTTP(&plugin.Context{}, rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 with a session (%s)", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `data-mode="map"`) {
		t.Fatal("the map route served no map")
	}
}

func TestMapRouteRejectsANonGet(t *testing.T) {
	p := newTestPlugin(t, "https://example.com", true)

	req := httptest.NewRequest(http.MethodPost, "/map?f=mgrs&v=18SUJ2347806483", nil)
	rec := httptest.NewRecorder()
	p.ServeHTTP(&plugin.Context{}, rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want 405", rec.Code)
	}
}
