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
	req := httptest.NewRequest(http.MethodGet,
		"/decorate/dtg?t=1786293000000&dtg=091630ZAUG26&z=Z&a=", nil)

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

// The route is public by design: the clients it exists for, notably the mobile
// app's in-app browser, have no Mattermost session.
func TestServeHTTPServesWithoutAuthenticationHeaders(t *testing.T) {
	p := newTestPlugin(t, "https://example.com", true)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet,
		"/decorate/dtg?t=1786293000000&dtg=091630ZAUG26&z=Z&a=", nil)
	// Explicitly no Mattermost-User-Id.

	p.ServeHTTP(&plugin.Context{}, rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 with no session", rec.Code)
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
			p.ServeHTTP(&plugin.Context{}, rec, httptest.NewRequest(http.MethodGet, tc.path, nil))

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
	p.ServeHTTP(&plugin.Context{}, rec, httptest.NewRequest(http.MethodGet, "/decorate/dtg", nil))

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
	assertCode(t, rec.Body.String(), errcode.HTTPDecoratorsNotReady)
}

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
	req := httptest.NewRequest(http.MethodGet, "/decorate/dtg?t=abc", nil)

	p.ServeHTTP(&plugin.Context{}, rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
	if got := rec.Header().Get("Cache-Control"); got != "no-store" {
		t.Fatalf("Cache-Control = %q, want no-store", got)
	}
	assertCode(t, rec.Body.String(), errcode.DTGPageParamsInvalid)
}

// The map page is a sibling of /decorate, not a mode of it, and it is public on
// the same terms: the clients it exists for have no session.
func TestMapRouteIsPublicAndServesTheMap(t *testing.T) {
	p := newTestPlugin(t, "https://example.com", true)

	req := httptest.NewRequest(http.MethodGet, "/map?f=mgrs&v=18SUJ2347806483", nil)
	rec := httptest.NewRecorder()
	p.ServeHTTP(&plugin.Context{}, rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 without a session (%s)", rec.Code, rec.Body.String())
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
