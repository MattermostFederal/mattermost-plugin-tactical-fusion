package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/mattermost/mattermost/server/public/plugin"

	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/bridgeclient"
	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/decorators/dtg"
	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/errcode"
)

const callerPluginID = "com.example.caller"

func bridgeCall(p *Plugin, method, path, pluginID, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if pluginID != "" {
		req.Header.Set("Mattermost-Plugin-ID", pluginID)
	}

	rec := httptest.NewRecorder()
	p.ServeHTTP(&plugin.Context{}, rec, req)

	return rec
}

func bridgeJSON(t *testing.T, value any) string {
	t.Helper()

	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("could not encode %T: %v", value, err)
	}
	return string(raw)
}

func decodeBridge[T any](t *testing.T, rec *httptest.ResponseRecorder) T {
	t.Helper()

	var out T
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("response is not a %T: %v (%s)", out, err, rec.Body.String())
	}
	return out
}

func bridgeLink(t *testing.T, p *Plugin, req bridgeclient.LinkRequest) *httptest.ResponseRecorder {
	t.Helper()
	return bridgeCall(p, http.MethodPost, bridgeLinkPath, callerPluginID, bridgeJSON(t, req))
}

func mustLink(t *testing.T, p *Plugin, req bridgeclient.LinkRequest) bridgeclient.LinkResponse {
	t.Helper()

	rec := bridgeLink(t, p, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("link %+v: status = %d, want 200 (%s)", req, rec.Code, rec.Body.String())
	}
	return decodeBridge[bridgeclient.LinkResponse](t, rec)
}

func assertDeclined(t *testing.T, rec *httptest.ResponseRecorder, code int, reason string) {
	t.Helper()

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422 (%s)", rec.Code, rec.Body.String())
	}
	body := decodeBridge[bridgeclient.ErrorResponse](t, rec)
	if body.Code != code || body.Reason != reason {
		t.Fatalf("declined with code %d reason %q, want %d %q", body.Code, body.Reason, code, reason)
	}
	assertCode(t, body.Message, code)
}

func withConfiguration(p *Plugin, change func(*configuration)) {
	updated := *p.getConfiguration()
	change(&updated)
	p.setConfiguration(&updated)
}

func TestBridgeRefusesARequestNotFromAPlugin(t *testing.T) {
	p := newTestPlugin(t, "https://example.com", true)

	for _, path := range []string{bridgeDecoratePath, bridgeLinkPath, bridgeInfoPath, bridgeAirportPath + "?ident=PHIK", bridgePath + "/nope"} {
		t.Run(path, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, path, strings.NewReader("{}"))
			req.Header.Set("Mattermost-User-Id", "reader")
			rec := httptest.NewRecorder()
			p.ServeHTTP(&plugin.Context{}, rec, req)

			if rec.Code != http.StatusUnauthorized {
				t.Fatalf("status = %d, want 401", rec.Code)
			}
			assertCode(t, rec.Body.String(), errcode.BridgeNotAuthorized)
		})
	}
}

func TestBridgeUnknownPathIsNotFound(t *testing.T) {
	p := newTestPlugin(t, "https://example.com", true)

	rec := bridgeCall(p, http.MethodPost, bridgePath+"/nope", callerPluginID, "{}")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
	assertCode(t, rec.Body.String(), errcode.BridgeNotFound)
}

func TestBridgeOperationsRefuseTheWrongMethod(t *testing.T) {
	p := newTestPlugin(t, "https://example.com", true)

	cases := []struct{ method, path string }{
		{http.MethodGet, bridgeDecoratePath},
		{http.MethodGet, bridgeLinkPath},
		{http.MethodPost, bridgeInfoPath},
		{http.MethodPost, bridgeAirportPath + "?ident=PHIK"},
		{http.MethodPut, decoratePathAPI},
	}
	for _, c := range cases {
		t.Run(c.method+" "+c.path, func(t *testing.T) {
			req := httptest.NewRequest(c.method, c.path, strings.NewReader("{}"))
			req.Header.Set("Mattermost-Plugin-ID", callerPluginID)
			req.Header.Set("Mattermost-User-Id", "reader")
			rec := httptest.NewRecorder()
			p.ServeHTTP(&plugin.Context{}, rec, req)

			if rec.Code != http.StatusMethodNotAllowed {
				t.Fatalf("status = %d, want 405", rec.Code)
			}
			assertCode(t, rec.Body.String(), errcode.BridgeMethodNotAllowed)
		})
	}
}

func TestBridgeRefusesABodyItCannotRead(t *testing.T) {
	p := newTestPlugin(t, "https://example.com", true)

	oversized := bridgeJSON(t, bridgeclient.DecorateRequest{Message: strings.Repeat("a", maxBridgeBody)})
	for name, body := range map[string]string{"malformed": "{", "oversized": oversized} {
		t.Run(name, func(t *testing.T) {
			rec := bridgeCall(p, http.MethodPost, bridgeDecoratePath, callerPluginID, body)
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400", rec.Code)
			}
			assertCode(t, rec.Body.String(), errcode.BridgeInvalidBody)
		})
	}
}

func TestBridgeIsNotReadyWithoutARegistry(t *testing.T) {
	p := newTestPlugin(t, "https://example.com", true)
	p.decorators = nil

	rec := bridgeCall(p, http.MethodGet, bridgeInfoPath, callerPluginID, "")
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", rec.Code)
	}
	assertCode(t, rec.Body.String(), errcode.BridgeNotReady)
}

func bridgeAirportCall(p *Plugin, query string) *httptest.ResponseRecorder {
	return bridgeCall(p, http.MethodGet, bridgeAirportPath+query, callerPluginID, "")
}

func TestBridgeAirportAnswersWithTheNameAndPosition(t *testing.T) {
	p := newTestPlugin(t, "https://example.com", true)

	rec := bridgeAirportCall(p, "?ident=PHIK")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (%s)", rec.Code, rec.Body.String())
	}
	if got := rec.Header().Get("Cache-Control"); got != "no-store" {
		t.Errorf("Cache-Control = %q, want no-store", got)
	}

	want := bridgeclient.AirportResponse{Found: true, Ident: "PHIK", Name: "Hickam Air Force Base", Lat: 21.3353, Lon: -157.9483}
	if got := decodeBridge[bridgeclient.AirportResponse](t, rec); got != want {
		t.Fatalf("airport = %+v, want %+v", got, want)
	}
}

func TestBridgeAirportReadsLowerCaseAndSurroundingSpace(t *testing.T) {
	p := newTestPlugin(t, "https://example.com", true)

	want := decodeBridge[bridgeclient.AirportResponse](t, bridgeAirportCall(p, "?ident=PHIK"))
	for _, query := range []string{"?ident=phik", "?ident=Phik", "?ident=%20phik%09"} {
		t.Run(query, func(t *testing.T) {
			rec := bridgeAirportCall(p, query)
			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200 (%s)", rec.Code, rec.Body.String())
			}
			if got := decodeBridge[bridgeclient.AirportResponse](t, rec); got != want || !got.Found {
				t.Fatalf("airport = %+v, want %+v", got, want)
			}
		})
	}
}

func TestBridgeAirportSaysWhenTheDatabaseDoesNotHoldTheIdent(t *testing.T) {
	p := newTestPlugin(t, "https://example.com", true)

	for _, query := range []string{"?ident=QQQQ", "?ident=qqqq", "?ident=ZZZZ"} {
		t.Run(query, func(t *testing.T) {
			rec := bridgeAirportCall(p, query)
			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200 (%s)", rec.Code, rec.Body.String())
			}

			want := bridgeclient.AirportResponse{Ident: strings.ToUpper(strings.TrimPrefix(query, "?ident="))}
			if got := decodeBridge[bridgeclient.AirportResponse](t, rec); got != want {
				t.Fatalf("airport = %+v, want %+v", got, want)
			}
		})
	}
}

func TestBridgeAirportAlwaysCarriesItsFiveFields(t *testing.T) {
	p := newTestPlugin(t, "https://example.com", true)

	for _, query := range []string{"?ident=PHIK", "?ident=QQQQ"} {
		t.Run(query, func(t *testing.T) {
			fields := decodeBridge[map[string]any](t, bridgeAirportCall(p, query))
			for _, name := range []string{"found", "ident", "name", "lat", "lon"} {
				if _, ok := fields[name]; !ok {
					t.Errorf("the answer has no %q: %v", name, fields)
				}
			}
		})
	}
}

func TestBridgeAirportRefusesAnIdentThatIsNotFourLetters(t *testing.T) {
	p := newTestPlugin(t, "https://example.com", true)

	for _, query := range []string{"", "?ident=", "?ident=PHI", "?ident=PHIKK", "?ident=PH1K", "?ident=PH%20K", "?v=PHIK", "?ident=HNL"} {
		t.Run(query, func(t *testing.T) {
			rec := bridgeAirportCall(p, query)
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400 (%s)", rec.Code, rec.Body.String())
			}

			body := decodeBridge[bridgeclient.ErrorResponse](t, rec)
			if body.Code != errcode.BridgeAirportInvalid || body.Reason != "" {
				t.Fatalf("refused with code %d reason %q, want %d and no reason", body.Code, body.Reason, errcode.BridgeAirportInvalid)
			}
			assertCode(t, body.Message, errcode.BridgeAirportInvalid)
		})
	}
}

func TestBridgeAirportAnswersWithTheAirfieldSwitchesOff(t *testing.T) {
	p := newTestPlugin(t, "https://example.com", true)
	want := decodeBridge[bridgeclient.AirportResponse](t, bridgeAirportCall(p, "?ident=PHIK"))

	withConfiguration(p, func(c *configuration) {
		c.EnableAirport = false
		c.EnableAirportIATA = false
	})

	if got := decodeBridge[bridgeclient.AirportResponse](t, bridgeAirportCall(p, "?ident=PHIK")); got != want || !got.Found {
		t.Fatalf("airport = %+v with the switches off, want %+v", got, want)
	}
}

func TestBridgeAirportAnswersTheSameForEveryCaller(t *testing.T) {
	p := newTestPlugin(t, "https://example.com", true)
	want := bridgeAirportCall(p, "?ident=PHIK").Body.String()

	req := httptest.NewRequest(http.MethodGet, bridgeAirportPath+"?ident=PHIK", http.NoBody)
	req.Header.Set("Mattermost-Plugin-ID", "com.example.other")
	req.Header.Set("Mattermost-User-Id", "reader")
	rec := httptest.NewRecorder()
	p.ServeHTTP(&plugin.Context{}, rec, req)

	if rec.Body.String() != want {
		t.Fatalf("answered %s to another caller, want %s", rec.Body.String(), want)
	}
}

func TestBridgeAirportIsNotReadyWithoutARegistry(t *testing.T) {
	p := newTestPlugin(t, "https://example.com", true)
	p.decorators = nil

	rec := bridgeAirportCall(p, "?ident=PHIK")
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", rec.Code)
	}
	assertCode(t, rec.Body.String(), errcode.BridgeNotReady)
}

func TestBridgeAirportAgreesWithTheSessionRoute(t *testing.T) {
	p := newTestPlugin(t, "https://example.com", true)

	for _, ident := range []string{"PHIK", "PNTF", "QQQQ"} {
		t.Run(ident, func(t *testing.T) {
			fromPlugin := decodeBridge[bridgeclient.AirportResponse](t, bridgeAirportCall(p, "?ident="+ident))
			fromSession := decodeBridge[airportResponse](t, call(p, http.MethodGet, airportPath+"?v="+ident, "reader", ""))

			if fromPlugin.Found != fromSession.Found || fromPlugin.Ident != fromSession.Ident {
				t.Fatalf("the bridge answered %+v and the session route %+v", fromPlugin, fromSession)
			}
			if fromSession.Found && fromPlugin.Name != fromSession.Airport.Name {
				t.Fatalf("the bridge names it %q and the session route %q", fromPlugin.Name, fromSession.Airport.Name)
			}
		})
	}
}

func TestBridgeRecoversAPanicAndLogsItsCode(t *testing.T) {
	p := newTestPlugin(t, "https://example.com", true)

	req := httptest.NewRequest(http.MethodPost, bridgeLinkPath, strings.NewReader("{}"))
	rec := httptest.NewRecorder()
	p.serveBridgeOperation(rec, req, func(http.ResponseWriter, *http.Request) { panic("boom") })

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
	assertCode(t, rec.Body.String(), errcode.BridgePanic)
}

func TestBridgeAnswersAreNeverCached(t *testing.T) {
	p := newTestPlugin(t, "https://example.com", true)

	rec := bridgeCall(p, http.MethodGet, bridgeInfoPath, callerPluginID, "")
	if got := rec.Header().Get("Cache-Control"); got != "no-store" {
		t.Fatalf("Cache-Control = %q, want no-store", got)
	}
}

func TestBridgeDecorateMatchesTheTagger(t *testing.T) {
	p := newTestPlugin(t, "https://example.com", true)
	ref := time.Date(2026, time.March, 1, 12, 0, 0, 0, time.UTC)

	messages := []string{
		"ARCT 091630ZAUG26 confirmed",
		"Rally at 34.0561, -118.2500 then ICAO:PHIK",
		"`091630ZAUG26` stays code, 091630ZAUG26 does not",
		"[the plan](https://example.com/091630ZAUG26) says 091630Z",
		"nothing to see here",
	}
	for _, message := range messages {
		t.Run(message, func(t *testing.T) {
			rec := bridgeCall(p, http.MethodPost, bridgeDecoratePath, callerPluginID,
				bridgeJSON(t, bridgeclient.DecorateRequest{Message: message, ReferenceTime: ref.UnixMilli()}))
			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d (%s)", rec.Code, rec.Body.String())
			}

			got := decodeBridge[bridgeclient.DecorateResponse](t, rec)
			want := p.bridgeTagger().Decorate(message, ref)
			if got.Message != want {
				t.Fatalf("message\n got: %s\nwant: %s", got.Message, want)
			}
			if got.Changed != (want != message) {
				t.Errorf("changed = %t for %q", got.Changed, message)
			}
			if !got.FitsPost {
				t.Errorf("fits_post = false for a short message")
			}
		})
	}
}

func TestBridgeDecorateIsIdempotent(t *testing.T) {
	p := newTestPlugin(t, "https://example.com", true)

	first := p.decorateText(bridgeclient.DecorateRequest{Message: "ARCT 091630ZAUG26 at ICAO:PHIK"})
	second := p.decorateText(bridgeclient.DecorateRequest{Message: first.Message})

	if !first.Changed || second.Changed || second.Message != first.Message {
		t.Fatalf("decorating twice changed the text:\n first: %s\nsecond: %s", first.Message, second.Message)
	}
}

func TestBridgeDecorateReportsWhetherTheResultFitsAPost(t *testing.T) {
	p := newTestPlugin(t, "https://example.com", true)

	fits := p.decorateText(bridgeclient.DecorateRequest{Message: strings.Repeat("é", safePostRunes)})
	if !fits.FitsPost {
		t.Error("a message of exactly the floor does not fit")
	}

	over := p.decorateText(bridgeclient.DecorateRequest{Message: strings.Repeat("é", safePostRunes+1)})
	if over.FitsPost {
		t.Error("a message one rune over the floor fits")
	}
}

func TestBridgeDecorateHonorsTheFormatSwitches(t *testing.T) {
	p := newTestPlugin(t, "https://example.com", true)
	withConfiguration(p, func(c *configuration) { c.EnableDTG = false })

	got := p.decorateText(bridgeclient.DecorateRequest{Message: "ARCT 091630ZAUG26"})
	if got.Changed {
		t.Fatalf("decorated a date-time group with EnableDTG off: %s", got.Message)
	}
}

func TestBridgeLinkIsTheLinkTheTaggerWrites(t *testing.T) {
	p := newTestPlugin(t, "https://example.com", true)
	ref := time.Date(2026, time.March, 1, 12, 0, 0, 0, time.UTC)

	cases := []struct {
		typ, token, message, prefix string
	}{
		{dtg.Type, "091630ZAUG26", "091630ZAUG26", ""},
		{dtg.Type, "2026-08-09T16:30:00Z", "2026-08-09T16:30:00Z", ""},
		{"location", "34.0561, -118.2500", "34.0561, -118.2500", ""},
		{"location", "18S UJ 23478 06483", "18S UJ 23478 06483", ""},
		{"airport", "PHIK", "ICAO:PHIK", "ICAO:"},
	}
	for _, c := range cases {
		t.Run(c.typ+" "+c.token, func(t *testing.T) {
			got := mustLink(t, p, bridgeclient.LinkRequest{Type: c.typ, Token: c.token, ReferenceTime: ref.UnixMilli()})

			want := strings.TrimPrefix(p.bridgeTagger().Decorate(c.message, ref), c.prefix)
			if got.Markdown != want {
				t.Fatalf("markdown\n got: %s\nwant: %s", got.Markdown, want)
			}
			if got.Markdown != "["+c.token+"]("+got.URL+")" {
				t.Errorf("markdown %q does not wrap url %q", got.Markdown, got.URL)
			}
			if got.Type != c.typ || got.Label != c.token {
				t.Errorf("type %q label %q", got.Type, got.Label)
			}
		})
	}
}

func TestBridgeLinkOpensAPageThatRenders(t *testing.T) {
	p := newTestPlugin(t, "https://example.com", true)

	for _, req := range []bridgeclient.LinkRequest{
		{Type: dtg.Type, Token: "091630ZAUG26"},
		{Type: "location", Token: "18S UJ 23478 06483"},
		{Type: "airport", Token: "PHIK"},
		{Type: "note", Token: "| Tail | Fuel |\n|:--|--:|\n| 101 | 12,400 lb |", Label: "fuel"},
	} {
		t.Run(req.Type, func(t *testing.T) {
			link := mustLink(t, p, req)

			path, ok := strings.CutPrefix(link.URL, "/plugins/"+manifest.Id)
			if !ok {
				t.Fatalf("url %q is not under this plugin", link.URL)
			}

			page := httptest.NewRequest(http.MethodGet, path, nil)
			page.Header.Set("Mattermost-User-Id", "reader")
			rec := httptest.NewRecorder()
			p.ServeHTTP(&plugin.Context{}, rec, page)

			if rec.Code != http.StatusOK {
				t.Fatalf("GET %s = %d, want 200", path, rec.Code)
			}
		})
	}
}

func TestBridgeLinkCarriesTheSiteSubpath(t *testing.T) {
	p := newTestPlugin(t, "https://example.com/mattermost", true)

	link := mustLink(t, p, bridgeclient.LinkRequest{Type: "airport", Token: "PHIK"})
	if want := "/mattermost/plugins/" + manifest.Id + "/decorate/airport?"; !strings.HasPrefix(link.URL, want) {
		t.Fatalf("url = %q, want prefix %q", link.URL, want)
	}
}

func TestBridgeLinkEscapesTheLabel(t *testing.T) {
	p := newTestPlugin(t, "https://example.com", true)

	link := mustLink(t, p, bridgeclient.LinkRequest{Type: "airport", Token: "PHIK", Label: "Hickam [*base_*]"})
	if want := `[Hickam \[\*base\_\*\]](`; !strings.HasPrefix(link.Markdown, want) {
		t.Fatalf("markdown = %q, want prefix %q", link.Markdown, want)
	}
	if link.Label != "Hickam [*base_*]" {
		t.Errorf("label = %q, want it unescaped", link.Label)
	}
}

func TestBridgeLinkRefusesALabelWithALineBreak(t *testing.T) {
	p := newTestPlugin(t, "https://example.com", true)

	rec := bridgeLink(t, p, bridgeclient.LinkRequest{Type: "airport", Token: "PHIK", Label: "Hickam\n\nbase"})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
	assertCode(t, rec.Body.String(), errcode.BridgeInvalidBody)
}

func TestBridgeLinkRefusesAMultiLineNoteWithNoLabel(t *testing.T) {
	p := newTestPlugin(t, "https://example.com", true)

	rec := bridgeLink(t, p, bridgeclient.LinkRequest{Type: "note", Token: "**DCA**\n\nDefensive Counter Air"})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
	assertCode(t, rec.Body.String(), errcode.BridgeInvalidBody)

	link := mustLink(t, p, bridgeclient.LinkRequest{Type: "note", Token: "**DCA**: Defensive Counter Air"})
	if !strings.HasPrefix(link.Markdown, `[\*\*DCA\*\*: Defensive Counter Air](`) {
		t.Errorf("a one-line note is its own label: %q", link.Markdown)
	}
}

func TestBridgeLinkSaysWhenANoteOutgrowsAPost(t *testing.T) {
	p := newTestPlugin(t, "https://example.com", true)

	short := mustLink(t, p, bridgeclient.LinkRequest{Type: "note", Token: "**DCA**", Label: "DCA"})
	if !short.FitsPost {
		t.Error("a short note does not fit a post")
	}

	long := mustLink(t, p, bridgeclient.LinkRequest{Type: "note", Token: strings.Repeat("é", 1000), Label: "long"})
	if long.FitsPost || utf8.RuneCountInString(long.Markdown) <= safePostRunes {
		t.Errorf("a %d-rune link reported fits_post %v", utf8.RuneCountInString(long.Markdown), long.FitsPost)
	}
}

func TestBridgeLinkTrimsTheToken(t *testing.T) {
	p := newTestPlugin(t, "https://example.com", true)

	link := mustLink(t, p, bridgeclient.LinkRequest{Type: "airport", Token: "  PHIK\t"})
	if link.Label != "PHIK" {
		t.Fatalf("label = %q, want PHIK", link.Label)
	}
}

func TestBridgeLinkDeclinesAnUnknownType(t *testing.T) {
	p := newTestPlugin(t, "https://example.com", true)

	assertDeclined(t, bridgeLink(t, p, bridgeclient.LinkRequest{Type: "cve", Token: "CVE-2024-3094"}),
		errcode.BridgeUnknownType, bridgeclient.ReasonUnknownType)
}

func TestBridgeLinkDeclinesATokenItCannotRead(t *testing.T) {
	p := newTestPlugin(t, "https://example.com", true)

	for _, req := range []bridgeclient.LinkRequest{
		{Type: dtg.Type, Token: "091630J"},
		{Type: "location", Token: "34.05, -118.25x"},
		{Type: "airport", Token: "ZZZZ"},
		{Type: "airport", Token: ""},
	} {
		t.Run(req.Type+" "+req.Token, func(t *testing.T) {
			assertDeclined(t, bridgeLink(t, p, req), errcode.BridgeTokenNotRecognized, bridgeclient.ReasonNotRecognized)
		})
	}
}

func TestBridgeLinkReportsAFormatSwitchedOff(t *testing.T) {
	cases := []struct {
		name   string
		change func(*configuration)
		req    bridgeclient.LinkRequest
	}{
		{"dtg parent", func(c *configuration) { c.EnableDTG = false }, bridgeclient.LinkRequest{Type: dtg.Type, Token: "091630ZAUG26"}},
		{"dtg military", func(c *configuration) { c.EnableDTGMilitary = false }, bridgeclient.LinkRequest{Type: dtg.Type, Token: "091630ZAUG26"}},
		{"dtg timestamp", func(c *configuration) { c.EnableDTGTimestamp = false }, bridgeclient.LinkRequest{Type: dtg.Type, Token: "2026-08-09T16:30:00Z"}},
		{"location parent", func(c *configuration) { c.EnableLocation = false }, bridgeclient.LinkRequest{Type: "location", Token: "34.0561, -118.2500"}},
		{"location utm", func(c *configuration) { c.EnableLocationUTM = false }, bridgeclient.LinkRequest{Type: "location", Token: "11S 384640E 3769080N"}},
		{"airport", func(c *configuration) { c.EnableAirport = false }, bridgeclient.LinkRequest{Type: "airport", Token: "PHIK"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			p := newTestPlugin(t, "https://example.com", true)
			mustLink(t, p, c.req)

			withConfiguration(p, c.change)
			assertDeclined(t, bridgeLink(t, p, c.req), errcode.BridgeFormatDisabled, bridgeclient.ReasonDisabled)
		})
	}
}

func TestBridgeLinkLeavesTheOtherDTGGrammarOn(t *testing.T) {
	p := newTestPlugin(t, "https://example.com", true)
	withConfiguration(p, func(c *configuration) { c.EnableDTGTimestamp = false })

	mustLink(t, p, bridgeclient.LinkRequest{Type: dtg.Type, Token: "091630ZAUG26"})
}

func TestBridgeLinkInfersAShortDTGFromTheReferenceTime(t *testing.T) {
	p := newTestPlugin(t, "https://example.com", true)

	march := time.Date(2026, time.March, 1, 12, 0, 0, 0, time.UTC)
	june := time.Date(2026, time.June, 1, 12, 0, 0, 0, time.UTC)

	for _, ref := range []time.Time{march, june} {
		link := mustLink(t, p, bridgeclient.LinkRequest{Type: dtg.Type, Token: "091630Z", ReferenceTime: ref.UnixMilli()})

		want, ok := (&dtg.Decorator{}).Parse("091630Z", ref)
		if !ok {
			t.Fatal("the fixture token does not parse")
		}
		if got := queryOf(t, link.URL); got.Encode() != want.Encode() {
			t.Errorf("reference %s: query = %s, want %s", ref.Month(), got.Encode(), want.Encode())
		}
	}
}

func queryOf(t *testing.T, raw string) url.Values {
	t.Helper()

	parsed, err := url.Parse(raw)
	if err != nil {
		t.Fatalf("url %q does not parse: %v", raw, err)
	}
	return parsed.Query()
}

func TestBridgeInfoListsTypesAndWhichAreOn(t *testing.T) {
	p := newTestPlugin(t, "https://example.com", true)
	withConfiguration(p, func(c *configuration) { c.EnableLocation = false })

	rec := bridgeCall(p, http.MethodGet, bridgeInfoPath, callerPluginID, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d (%s)", rec.Code, rec.Body.String())
	}

	info := decodeBridge[bridgeclient.InfoResponse](t, rec)
	want := bridgeclient.InfoResponse{
		PluginVersion: manifest.Version,
		APIVersion:    bridgeclient.APIVersion,
		Types:         []string{"dtg", "location", "airport", "avreport", "frequency", "note", "cyber"},
		EnabledTypes:  []string{"dtg", "airport", "avreport", "frequency", "note", "cyber"},
	}
	if !reflect.DeepEqual(info, want) {
		t.Fatalf("info = %+v, want %+v", info, want)
	}
}

func TestSessionRoutesServeTheSameOperations(t *testing.T) {
	p := newTestPlugin(t, "https://example.com", true)

	decorateBody := bridgeJSON(t, bridgeclient.DecorateRequest{Message: "ICAO:PHIK"})
	linkBody := bridgeJSON(t, bridgeclient.LinkRequest{Type: "airport", Token: "PHIK"})

	for path, body := range map[string]string{decoratePathAPI: decorateBody, linkPathAPI: linkBody} {
		t.Run(path, func(t *testing.T) {
			if rec := call(p, http.MethodPost, path, "", body); rec.Code != http.StatusUnauthorized {
				t.Fatalf("without a session: status = %d, want 401", rec.Code)
			}
			if rec := call(p, http.MethodPost, path, "reader", body); rec.Code != http.StatusOK {
				t.Fatalf("with a session: status = %d, want 200 (%s)", rec.Code, rec.Body.String())
			}
		})
	}

	fromSession := decodeBridge[bridgeclient.LinkResponse](t, call(p, http.MethodPost, linkPathAPI, "reader", linkBody))
	fromPlugin := decodeBridge[bridgeclient.LinkResponse](t, bridgeCall(p, http.MethodPost, bridgeLinkPath, callerPluginID, linkBody))
	if fromSession != fromPlugin {
		t.Fatalf("the session route answered %+v and the bridge %+v", fromSession, fromPlugin)
	}
}

type inProcessPluginHTTP struct {
	p *Plugin
}

func (b inProcessPluginHTTP) PluginHTTP(r *http.Request) *http.Response {
	path, ok := strings.CutPrefix(r.URL.Path, "/"+manifest.Id)
	if !ok {
		return nil
	}

	forwarded := r.Clone(r.Context())
	forwarded.URL.Path = path
	forwarded.Header.Set("Mattermost-Plugin-ID", callerPluginID)

	rec := httptest.NewRecorder()
	b.p.ServeHTTP(&plugin.Context{}, rec, forwarded)
	return rec.Result()
}

func TestTheGoClientRoundTripsThroughTheBridge(t *testing.T) {
	p := newTestPlugin(t, "https://example.com", true)
	client := bridgeclient.NewClient(inProcessPluginHTTP{p})
	ctx := context.Background()

	link, err := client.Link(ctx, bridgeclient.LinkRequest{Type: bridgeclient.TypeAirport, Token: "PHIK"})
	if err != nil {
		t.Fatalf("Link returned %v", err)
	}
	if want := mustLink(t, p, bridgeclient.LinkRequest{Type: "airport", Token: "PHIK"}); link != want {
		t.Fatalf("client link = %+v, want %+v", link, want)
	}

	decorated, err := client.Decorate(ctx, bridgeclient.DecorateRequest{Message: "ARCT 091630ZAUG26"})
	if err != nil || !decorated.Changed {
		t.Fatalf("Decorate = %+v, %v", decorated, err)
	}

	info, err := client.Info(ctx)
	if err != nil || info.APIVersion != bridgeclient.APIVersion {
		t.Fatalf("Info = %+v, %v", info, err)
	}

	field, err := client.Airport(ctx, "phik")
	if err != nil || field != bridgeAirport("PHIK") || !field.Found {
		t.Fatalf("Airport = %+v, %v", field, err)
	}

	var refused *bridgeclient.Error
	if _, err = client.Airport(ctx, "PHI"); !errors.As(err, &refused) || refused.Code != errcode.BridgeAirportInvalid {
		t.Fatalf("malformed ident error = %v, want TF-%d", err, errcode.BridgeAirportInvalid)
	}

	_, err = client.Link(ctx, bridgeclient.LinkRequest{Type: bridgeclient.TypeAirport, Token: "ZZZZ"})
	if !errors.Is(err, bridgeclient.ErrNotRecognized) || errors.Is(err, bridgeclient.ErrPluginNotActive) {
		t.Fatalf("declined link error = %v, want ErrNotRecognized alone", err)
	}
}

func TestBridgeClientTypesAreTheRegisteredDecorators(t *testing.T) {
	p := newTestPlugin(t, "https://example.com", true)

	named := []string{bridgeclient.TypeDTG, bridgeclient.TypeLocation, bridgeclient.TypeAirport, bridgeclient.TypeAvReport, bridgeclient.TypeFrequency, bridgeclient.TypeNote, bridgeclient.TypeCyber}
	for _, typ := range named {
		if p.decorators.Get(typ) == nil {
			t.Errorf("bridgeclient names type %q, which is not registered", typ)
		}
	}
	if got := len(p.decorators.All()); got != len(named) {
		t.Errorf("%d decorators are registered and bridgeclient names %d", got, len(named))
	}
}

func TestBridgeClientPluginIDMatchesManifest(t *testing.T) {
	if bridgeclient.PluginID != manifest.Id {
		t.Fatalf("bridgeclient.PluginID = %q, manifest id = %q", bridgeclient.PluginID, manifest.Id)
	}
}

func TestBridgeClientPathMatchesTheRoute(t *testing.T) {
	if bridgePath != "/bridge/v1" {
		t.Fatalf("bridgePath = %q; a bridge version change must be a new path, not an edit", bridgePath)
	}
}

func readWebappBridgeTypes(t *testing.T) string {
	t.Helper()

	path := filepath.Join("..", "webapp", "src", "bridge", "types.ts")
	raw, err := os.ReadFile(path) // #nosec G304 -- fixed, repo-relative source path
	if err != nil {
		t.Fatalf("could not read %s: %v", path, err)
	}
	return string(raw)
}

func TestWebappBridgeShapeMatches(t *testing.T) {
	source := readWebappBridgeTypes(t)

	shapes := map[string]reflect.Type{
		"DecorateRequest":  reflect.TypeFor[bridgeclient.DecorateRequest](),
		"DecorateResponse": reflect.TypeFor[bridgeclient.DecorateResponse](),
		"LinkRequest":      reflect.TypeFor[bridgeclient.LinkRequest](),
		"LinkResponse":     reflect.TypeFor[bridgeclient.LinkResponse](),
		"ErrorResponse":    reflect.TypeFor[bridgeclient.ErrorResponse](),
	}

	for name, goType := range shapes {
		t.Run(name, func(t *testing.T) {
			block := regexp.MustCompile(`(?s)export interface ` + name + ` \{(.*?)\n\}`).FindStringSubmatch(source)
			if block == nil {
				t.Fatalf("no `export interface %s` in webapp/src/bridge/types.ts", name)
			}

			var webapp []string
			for _, m := range regexp.MustCompile(`(?m)^\s+(\w+)(\??):\s*([\w\[\]]+);`).FindAllStringSubmatch(block[1], -1) {
				webapp = append(webapp, m[1]+m[2]+" "+m[3])
			}

			var server []string
			for field := range goType.Fields() {
				tag := strings.Split(field.Tag.Get("json"), ",")
				optional := ""
				if len(tag) > 1 && tag[1] == "omitempty" {
					optional = "?"
				}
				server = append(server, tag[0]+optional+" "+typescriptTypeOf(t, field.Type))
			}

			if strings.Join(webapp, "; ") != strings.Join(server, "; ") {
				t.Fatalf("shape drifted\nwebapp: %s\nserver: %s", strings.Join(webapp, "; "), strings.Join(server, "; "))
			}
		})
	}
}

func typescriptTypeOf(t *testing.T, goType reflect.Type) string {
	t.Helper()

	switch goType.Kind() {
	case reflect.String:
		return "string"
	case reflect.Bool:
		return "boolean"
	case reflect.Int, reflect.Int64:
		return "number"
	default:
		t.Fatalf("no TypeScript spelling for %s", goType)
		return ""
	}
}

func TestWebappBridgeDeclineReasonsMatch(t *testing.T) {
	source := readWebappBridgeTypes(t)

	union := regexp.MustCompile(`export type DeclineReason =([^;]*);`).FindStringSubmatch(source)
	if union == nil {
		t.Fatal("no `export type DeclineReason` in webapp/src/bridge/types.ts")
	}

	var webapp []string
	for _, m := range regexp.MustCompile(`'([a-z_]+)'`).FindAllStringSubmatch(union[1], -1) {
		webapp = append(webapp, m[1])
	}

	server := []string{bridgeclient.ReasonUnknownType, bridgeclient.ReasonNotRecognized, bridgeclient.ReasonDisabled}
	if strings.Join(webapp, " ") != strings.Join(server, " ") {
		t.Fatalf("webapp reasons [%s], server reasons [%s]", strings.Join(webapp, " "), strings.Join(server, " "))
	}
}
