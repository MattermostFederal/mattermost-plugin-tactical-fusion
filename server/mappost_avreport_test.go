package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/plugin"

	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/avreport"
	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/decorators/location"
)

func avreportPost(t *testing.T, text string) *model.Post {
	t.Helper()

	report, err := avreport.Decode(text, hookRef)
	if err != nil {
		t.Fatalf("the fixture does not decode: %v", err)
	}

	return &model.Post{
		Type: avreport.PostType,
		Props: model.StringInterface{
			avreport.PropsKey: avreport.Props(report, avreport.Source{Kind: avreport.SourceFence, Text: text}),
		},
	}
}

func TestTheOverlayRouteServesAReportPost(t *testing.T) {
	p, _ := overlayPlugin(t, avreportPost(t, reportTAF))

	req := withSession(httptest.NewRequest(http.MethodGet, "/map?post="+overlayPost, nil))
	rec := httptest.NewRecorder()
	p.ServeHTTP(&plugin.Context{}, rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (%s)", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	for _, want := range []string{
		location.OverlayKindAttr + `="` + avreport.PostType + `"`,
		"PHNL",
		"21.3184",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("the shell does not carry %s", want)
		}
	}
}

func TestTheOverlayRouteStandsDownForAReportWithNoPosition(t *testing.T) {
	post := avreportPost(t, "METAR ZZZY 221651Z 07012KT 10SM CLR 27/19 A3010")
	if blob := post.GetProps()[avreport.PropsKey].(map[string]any); blob["format"] != "" {
		t.Fatalf("the fixture station was placed: %v", blob["format"])
	}
	p, _ := overlayPlugin(t, post)

	req := withSession(httptest.NewRequest(http.MethodGet, "/map?post="+overlayPost, nil))
	rec := httptest.NewRecorder()
	p.ServeHTTP(&plugin.Context{}, rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404 for a report with nowhere to draw", rec.Code)
	}
}
