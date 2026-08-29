package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/plugin"

	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/cot"
	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/decorators/location"
	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/errcode"
	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/geojson"
)

const (
	overlayReader  = "reader00000000000000000000"
	overlayPost    = "post0000000000000000000000"
	overlayChannel = "channel000000000000000000a"
)

// overlayPlugin is a reader who may read one channel holding one stamped post.
func overlayPlugin(t *testing.T, post *model.Post) (*Plugin, *fakeAPI) {
	t.Helper()

	p := newTestPlugin(t, "https://example.com", true)
	api := p.API.(*fakeAPI)

	post.Id = overlayPost
	post.ChannelId = overlayChannel

	api.posts = map[string]*model.Post{overlayPost: post}
	api.channelsPermitted = map[string]bool{overlayChannel: true}

	return p, api
}

func geoJSONPost(t *testing.T) *model.Post {
	t.Helper()

	document, err := geojson.Parse([]byte(geojson.Fixture()))
	if err != nil {
		t.Fatalf("the fixture does not parse: %v", err)
	}

	return &model.Post{
		Type: geojson.PostType,
		Props: model.StringInterface{
			geojson.PropsKey: geojson.Props(document, geojson.Source{
				Kind: geojson.SourceFence, Text: geojson.Fixture(),
			}),
		},
	}
}

func TestAnOverlayPageIsServedForAStampedPost(t *testing.T) {
	p, _ := overlayPlugin(t, geoJSONPost(t))

	found, ok := p.overlayForPost(overlayReader, overlayPost)
	if !ok {
		t.Fatal("a stamped post in a readable channel was refused")
	}
	if found.kind != geojson.PostType {
		t.Errorf("kind = %q, want the post type", found.kind)
	}
	if !strings.Contains(found.blob, geojson.PropsKey) {
		t.Errorf("the blob does not carry the props key: %s", found.blob)
	}
}

// The gate has to ASK, not merely answer no. A refusal that came from
// somewhere else would look identical from outside.
func TestAnOverlayPageChecksTheChannelBeforeReadingThePost(t *testing.T) {
	p, api := overlayPlugin(t, geoJSONPost(t))
	api.channelsPermitted = map[string]bool{}

	if _, ok := p.overlayForPost(overlayReader, overlayPost); ok {
		t.Fatal("a post in a channel this reader may not read was served")
	}
	if len(api.channelsAsked) != 1 || api.channelsAsked[0] != overlayChannel {
		t.Errorf("channels asked about = %v, want the post's own", api.channelsAsked)
	}
}

// Everything else on a post belongs to Mattermost or to another plugin. The
// page republishes the document, so it must carry the one key and no more.
func TestAnOverlayPageCarriesOnlyItsOwnPropsKey(t *testing.T) {
	post := geoJSONPost(t)
	post.AddProp("from_webhook", "true")
	post.AddProp("override_username", "somebody-elses-secret")
	post.AddProp(cot.PropsKey, map[string]any{"version": 2})

	p, _ := overlayPlugin(t, post)

	found, ok := p.overlayForPost(overlayReader, overlayPost)
	if !ok {
		t.Fatal("the post was refused")
	}

	for _, leaked := range []string{"from_webhook", "override_username", cot.PropsKey} {
		if strings.Contains(found.blob, leaked) {
			t.Errorf("the page carries %q, which is not its own: %s", leaked, found.blob)
		}
	}
}

/*
 * The card's own stand-downs, restated.
 *
 * CotPostBody and GeoJsonPostBody refuse an edited post and a file source whose
 * file is gone. A page that drew what the card had already refused would be the
 * one surface still claiming something nobody can check.
 */
func TestAnOverlayPageStandsDownWhereTheCardDoes(t *testing.T) {
	for _, tc := range []struct {
		name string
		post func(*model.Post)
	}{
		{"an edited post", func(post *model.Post) { post.EditAt = 1 }},
		{"a deleted post", func(post *model.Post) { post.DeleteAt = 1 }},
		{"a post this plugin never stamped", func(post *model.Post) { post.Type = "" }},
		{"a type with no props", func(post *model.Post) { post.Props = nil }},
		{"a file source whose file is gone", func(post *model.Post) {
			blob := post.GetProps()[geojson.PropsKey].(map[string]any)
			blob["file_id"] = "filegone00000000000000000a"
			post.FileIds = nil
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			post := geoJSONPost(t)
			tc.post(post)

			p, _ := overlayPlugin(t, post)

			if _, ok := p.overlayForPost(overlayReader, overlayPost); ok {
				t.Fatal("the page was served where the card would have stood down")
			}
		})
	}
}

func TestAnOverlayPageIsRefusedForAPostThatIsNotThere(t *testing.T) {
	p, _ := overlayPlugin(t, geoJSONPost(t))

	for _, postID := range []string{"", "not-an-id", "absent0000000000000000000a"} {
		if _, ok := p.overlayForPost(overlayReader, postID); ok {
			t.Errorf("%q was served", postID)
		}
	}
}

// The route, end to end: the shell the bundle reads, and the one 404.
func TestTheOverlayRouteServesAShellTheBundleCanRead(t *testing.T) {
	p, _ := overlayPlugin(t, geoJSONPost(t))

	req := withSession(httptest.NewRequest(http.MethodGet, "/map?post="+overlayPost, nil))
	rec := httptest.NewRecorder()
	p.ServeHTTP(&plugin.Context{}, rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (%s)", rec.Code, rec.Body.String())
	}

	body := rec.Body.String()
	for _, want := range []string{
		`data-mode="overlay"`,
		location.OverlayKindAttr + `="` + geojson.PostType + `"`,
		location.OverlayAttr + `="`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("the shell does not carry %s", want)
		}
	}
}

// One 404 and one code for every way this can be declined. Anything finer
// would answer, one id at a time, whether a post exists and what it holds.
func TestTheOverlayRouteRefusesWithOneCode(t *testing.T) {
	p, api := overlayPlugin(t, geoJSONPost(t))
	api.channelsPermitted = map[string]bool{}

	req := withSession(httptest.NewRequest(http.MethodGet, "/map?post="+overlayPost, nil))
	rec := httptest.NewRecorder()
	p.ServeHTTP(&plugin.Context{}, rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
	assertCode(t, rec.Body.String(), errcode.HTTPMapPostUnavailable)
}

// A request carrying both is answered as the overlay. The two draw different
// pictures and only one of them was asked for.
func TestAPostIdWinsOverACoordinate(t *testing.T) {
	p, _ := overlayPlugin(t, geoJSONPost(t))

	req := withSession(httptest.NewRequest(http.MethodGet,
		"/map?f=mgrs&v=18SUJ2347806483&post="+overlayPost, nil))
	rec := httptest.NewRecorder()
	p.ServeHTTP(&plugin.Context{}, rec, req)

	if !strings.Contains(rec.Body.String(), `data-mode="overlay"`) {
		t.Error("the coordinate was drawn instead of the overlay")
	}
}

// BOTH permissions, and each one load-bearing. This route returns the whole
// stamped document, so read_channel ("may see this channel exists") alone would
// serve a post body out of a channel a reader may see but not read.
func TestAnOverlayPageRequiresBothChannelPermissions(t *testing.T) {
	for _, denied := range []*model.Permission{
		model.PermissionReadChannel,
		model.PermissionReadChannelContent,
	} {
		t.Run(denied.Id, func(t *testing.T) {
			p, api := overlayPlugin(t, geoJSONPost(t))
			api.channelDenied = map[string]bool{denied.Id: true}

			if _, ok := p.overlayForPost(overlayReader, overlayPost); ok {
				t.Fatalf("the page was served without %s", denied.Id)
			}
		})
	}
}

// Nothing about a post may be read before the reader is shown to be allowed to
// see it. A deleted post is refused, but only after the permission check.
func TestTheChannelIsCheckedBeforeThePostIsRead(t *testing.T) {
	post := geoJSONPost(t)
	post.DeleteAt = 1

	p, api := overlayPlugin(t, post)
	api.channelsPermitted = map[string]bool{}

	if _, ok := p.overlayForPost(overlayReader, overlayPost); ok {
		t.Fatal("a deleted post in an unreadable channel was served")
	}
	if len(api.channelsAsked) == 0 {
		t.Error("the deleted check short-circuited ahead of the permission check")
	}
}

// The other stamped format through the same gate. Every fixture here is GeoJSON,
// so the CoT arm of stampedPropsKey was never exercised server-side.
func TestTheOverlayRouteServesACotPost(t *testing.T) {
	events, err := cot.Parse([]byte(cotExampleTarget))
	if err != nil {
		t.Fatalf("the fixture does not parse: %v", err)
	}

	p, _ := overlayPlugin(t, &model.Post{
		Type: cot.PostType,
		Props: model.StringInterface{
			cot.PropsKey: cot.Props(events, cot.Source{Kind: cot.SourceFence, Text: cotExampleTarget}),
		},
	})

	found, ok := p.overlayForPost(overlayReader, overlayPost)
	if !ok {
		t.Fatal("a stamped Cursor on Target post was refused")
	}
	if found.kind != cot.PostType || !strings.Contains(found.blob, cot.PropsKey) {
		t.Errorf("the overlay does not carry the CoT post's own kind and key: %+v", found)
	}
}
