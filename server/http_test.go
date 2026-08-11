package main

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/mattermost/mattermost/server/public/plugin"

	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/decorators"
)

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

	cases := []struct {
		name string
		path string
	}{
		{"unknown decorator type", "/decorate/nope"},
		{"no type", "/decorate/"},
		{"decorate root", "/decorate"},
		{"nested path", "/decorate/dtg/extra"},
		{"unrelated path", "/something-else"},
		{"root", "/"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			p.ServeHTTP(&plugin.Context{}, rec, httptest.NewRequest(http.MethodGet, tc.path, nil))

			if rec.Code != http.StatusNotFound {
				t.Fatalf("status = %d, want 404 for %s", rec.Code, tc.path)
			}
		})
	}
}

func TestServeHTTPRejectsNonGET(t *testing.T) {
	p := newTestPlugin(t, "https://example.com", true)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/decorate/dtg", nil)

	p.ServeHTTP(&plugin.Context{}, rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want 405", rec.Code)
	}
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
}
