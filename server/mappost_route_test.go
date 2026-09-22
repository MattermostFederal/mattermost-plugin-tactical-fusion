package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/plugin"

	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/decorators/airport"
	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/decorators/location"
)

func routePost(t *testing.T) *model.Post {
	t.Helper()

	p := newTestPlugin(t, "https://example.com", true)
	got := p.decoratePost(&model.Post{Message: "DEPLOC:PHIK ARRLOC:PGUA//"}, hookRef)
	if got == nil || got.Type != airport.PostType {
		t.Fatal("the route was not stamped")
	}
	return got
}

func TestTheOverlayRouteServesAnAirfieldsPost(t *testing.T) {
	p, _ := overlayPlugin(t, routePost(t))

	req := withSession(httptest.NewRequest(http.MethodGet, "/map?post="+overlayPost, nil))
	rec := httptest.NewRecorder()
	p.ServeHTTP(&plugin.Context{}, rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (%s)", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	for _, want := range []string{
		location.OverlayKindAttr + `="` + airport.PostType + `"`,
		"PHIK",
		"PGUA",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("the shell does not carry %s", want)
		}
	}
}

func TestAnOverlayPageStandsDownForAnEditedRoute(t *testing.T) {
	post := routePost(t)
	post.EditAt = 1
	p, _ := overlayPlugin(t, post)

	req := withSession(httptest.NewRequest(http.MethodGet, "/map?post="+overlayPost, nil))
	rec := httptest.NewRecorder()
	p.ServeHTTP(&plugin.Context{}, rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404 for an edited route", rec.Code)
	}
}
