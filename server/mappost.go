package main

import (
	"encoding/json"
	"net/http"
	"slices"

	"github.com/mattermost/mattermost/server/public/model"

	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/decorators"
	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/decorators/location"
	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/errcode"
)

// postParam addresses the map page by the post whose overlay it draws.
//
// A block of Cursor on Target events and a GeoJSON document have no canonical
// token, which is what /map?f=&v= is addressed by and what "a link may never
// disagree with itself" is about. A post id is the only name either one has.
// The invariant survives the change of address: nothing derived travels here
// either, and everything the page draws is re-read from stored props at render.
const postParam = "post"

// overlay is what the map page needs to draw a stamped post, and nothing else.
type overlay struct {
	kind string
	blob string
}

/*
 * serveOverlayMapPage answers /map?post=<id>.
 *
 * Every refusal is the same 404 and the same code. A reader with a session can
 * put any id here, so a route that distinguished "no such post" from "not
 * yours" from "not a stamped post" would answer, one id at a time, questions
 * this plugin has no business answering.
 *
 * It reads no format switch. EnableCot and EnableGeoJSON govern STAMPING, and a
 * post already stamped keeps its card when either is turned off; a page that
 * went dark at the same moment would disagree with the card the reader is
 * looking at. The map page's own switch is checked before this is reached,
 * because that one is about whether this page exists at all.
 */
func (p *Plugin) serveOverlayMapPage(w http.ResponseWriter, r *http.Request, postID string) {
	found, ok := p.overlayForPost(sessionUserID(r), postID)
	if !ok {
		decorators.WriteError(w, http.StatusNotFound,
			errcode.WithCode(errcode.HTTPMapPostUnavailable, "There is no map here."))
		return
	}

	location.RenderOverlayPage(w, r.URL.Query(), p.packageNames(), found.kind, found.blob)
}

/*
 * overlayForPost is the whole gate, and the order of it is the point.
 *
 * The permission check comes before anything is read out of the post, so a
 * reader who may not see the channel cannot learn what kind of thing is in it
 * from how the route behaves.
 *
 * The last three checks are the card's own, restated. CotPostBody and
 * GeoJsonPostBody stand a card down for a post that has been edited, and for a
 * file source whose file is no longer attached, because Post.Type survives an
 * edit and Props may not. A page that drew what the card had already refused to
 * draw would be the one surface still claiming something nobody can check.
 */
func (p *Plugin) overlayForPost(userID, postID string) (overlay, bool) {
	if !model.IsValidId(postID) {
		return overlay{}, false
	}

	post, appErr := p.API.GetPost(postID)
	if appErr != nil || post == nil {
		if appErr != nil {
			p.API.LogWarn("tactical-fusion: a map page could not read its post",
				"error_code", errcode.HTTPMapPostUnavailable, "error", appErr.Error())
		}

		return overlay{}, false
	}

	// Before DeleteAt, before the type, before the props. Nothing about the
	// post may be read until the reader has been shown to be allowed to see it,
	// and a field test ahead of this is a post field deciding the route's
	// behavior for somebody who may not be looking at that channel at all.
	// BOTH, because this route serves post CONTENT. read_channel is "may see
	// that this channel exists"; read_channel_content is the one Mattermost's
	// own post reads gate on, and the two are separately grantable through a
	// custom scheme or channel moderation. The whole stamped document, `src`
	// included, comes back here, so the weaker permission alone would serve a
	// post body out of a channel a reader may see but not read.
	for _, permission := range []*model.Permission{
		model.PermissionReadChannel,
		model.PermissionReadChannelContent,
	} {
		if !p.API.HasPermissionToChannel(userID, post.ChannelId, permission) {
			return overlay{}, false
		}
	}

	if post.DeleteAt != 0 {
		return overlay{}, false
	}

	propsKey, ok := stampedPropsKey(post.Type)
	if !ok || post.EditAt != 0 {
		return overlay{}, false
	}

	blob, ok := post.GetProps()[propsKey].(map[string]any)
	if !ok {
		return overlay{}, false
	}

	if fileID, named := blob["file_id"].(string); named && !slices.Contains(post.FileIds, fileID) {
		return overlay{}, false
	}

	// The one key, never the whole props map. Everything else on a post belongs
	// to Mattermost or to another plugin, and this page has no business
	// republishing any of it into a document.
	encoded, err := json.Marshal(map[string]any{propsKey: blob})
	if err != nil {
		// The reader can do nothing about this and is told nothing, but the
		// props came off a post this plugin wrote: an operator should see it.
		p.API.LogWarn("tactical-fusion: a stamped post's props could not be encoded",
			"error_code", errcode.HTTPMapPostUnavailable, "post_id", post.Id, "error", err.Error())

		return overlay{}, false
	}

	return overlay{kind: post.Type, blob: string(encoded)}, true
}

func stampedPropsKey(postType string) (string, bool) {
	for _, stamped := range stampedTypes {
		if postType == stamped.postType {
			return stamped.propsKey, true
		}
	}

	return "", false
}
