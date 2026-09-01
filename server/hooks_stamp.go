package main

import (
	"encoding/json"
	"maps"
	"unicode/utf8"

	"github.com/mattermost/mattermost/server/public/model"

	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/cot"
	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/geojson"
)

const stampPropsBudgetRunes = model.PostPropsMaxUserRunes

// stampedTypes is every post type this plugin stamps from a recognized source.
//
// custom_tf_location is deliberately absent. stampStandalonePost writes that
// one from decoration rather than from recognition, and sweeping it in here
// would change forged-type handling on a shipped path that nothing has asked to
// change.
var stampedTypes = []struct{ postType, propsKey string }{
	{cot.PostType, cot.PropsKey},
	{geojson.PostType, geojson.PropsKey},
}

// stripStampedTypes removes a type and props this hook did not write.
//
// Post.IsValid accepts any custom_ type from an ordinary client, so anyone who
// can post can otherwise hand a reader a card whose fields and whose own source
// pane were both authored to agree with each other. Strip it and let
// recognition decide again from the message. Never refuse the post.
//
// EVERY key in the table goes, on every post, not just the one belonging to a
// matching type. cotProps copies the post's existing props forward, so an
// author posting one format's props key with a real event in the other format
// would otherwise have that forged blob carried into stored props permanently,
// counted against the rune budget and readable by everyone who can read the
// post.
func stripStampedTypes(post *model.Post) *model.Post {
	var stripped *model.Post

	clone := func() *model.Post {
		if stripped == nil {
			stripped = post.Clone()
		}
		return stripped
	}

	for _, stamped := range stampedTypes {
		if post.Type == stamped.postType {
			clone().Type = ""
		}
		if _, carried := post.GetProps()[stamped.propsKey]; carried {
			clone().DelProp(stamped.propsKey)
		}
	}

	return stripped
}

// stampRung is one width of a props blob, widest first.
type stampRung struct {
	blob     map[string]any
	degraded bool
}

// stampCodes are the TF-NNNN values the calling format logs.
//
// Passed in rather than chosen here because a shared helper logging HooksCot
// codes for a GeoJSON document is a call site telling a lie.
type stampCodes struct {
	propsUnmeasurable int
	propsTooLarge     int
	degraded          int
	degradedMessage   string
}

// commitStamped walks the ladder and commits the widest rung that fits.
//
// It does NOT recover. Each format's stamper declares its own recover as its
// first statement so that the span covers source-finding and the filestore call
// as well, which are outside this function and outside every other recover on
// the hook path.
//
// The second return is the code the caller should report to the author, or zero
// when it committed.
func (p *Plugin) commitStamped(
	post *model.Post, postType, propsKey string, rungs []stampRung, codes stampCodes,
) (*model.Post, int) {
	api := p.API

	for _, rung := range rungs {
		props := stampProps(post, propsKey, rung.blob)

		// Marshalled here rather than through model.StringInterfaceToJSON,
		// which discards the error and answers "". An unmeasurable props map
		// would score zero runes and sail through the one gate that exists to
		// stop the server refusing the post.
		encoded, err := json.Marshal(props)
		if err != nil {
			// Not the author's problem and not a size problem: the value that
			// will not marshal came from elsewhere on the post, and no rung of
			// this ladder can shed it.
			if api != nil {
				api.LogWarn("tactical-fusion: the post's props could not be measured; posting unstamped",
					"error_code", codes.propsUnmeasurable, "channel_id", post.ChannelId, "error", err)
			}
			return nil, codes.propsUnmeasurable
		}

		if utf8.RuneCountInString(string(encoded)) > stampPropsBudgetRunes {
			continue
		}

		if rung.degraded && api != nil {
			api.LogWarn(codes.degradedMessage,
				"error_code", codes.degraded, "channel_id", post.ChannelId)
		}

		post.Type = postType
		post.SetProps(props)

		return post, 0
	}

	if api != nil {
		api.LogWarn("tactical-fusion: the parsed document would exceed the maximum post props size; posting unstamped",
			"error_code", codes.propsTooLarge, "channel_id", post.ChannelId)
	}

	return nil, codes.propsTooLarge
}

func stampProps(post *model.Post, propsKey string, blob map[string]any) model.StringInterface {
	props := make(model.StringInterface, len(post.GetProps())+1)
	maps.Copy(props, post.GetProps())
	props[propsKey] = blob

	return props
}

// attachmentOwnedBy reports whether this attachment is the poster's own,
// unattached and live.
//
// post.FileIds arrives from the client and this hook runs BEFORE Mattermost
// binds files to posts, so at this point nothing else has checked that the id
// names a file its sender is allowed to read. Without this, quoting somebody
// else's file id copies that file's text into props that everyone who can read
// the attacker's post can read.
func attachmentOwnedBy(info *model.FileInfo, post *model.Post) bool {
	if info.PostId != "" || info.DeleteAt != 0 {
		return false
	}

	return attachmentCreator(info.CreatorId, post.UserId)
}

// attachmentCreator reports whether a file's creator may be read by this post.
//
// Two answers, not one. The poster's own upload is the ordinary case. The other
// is model.UploadNoUserID: plugin.API.UploadFile takes no user id at all, so a
// file an integration uploaded is credited to "nouser" and could never equal
// any post's UserId. Refusing it silently ignored every attachment a companion
// plugin posted.
//
// It does not reopen what the check closed. A "nouser" file can only be created
// by a plugin or by local mode on the host; a remote client cannot make one and
// cannot learn the id of one while it is unattached.
func attachmentCreator(creator, poster string) bool {
	if creator == model.UploadNoUserID {
		return true
	}

	return model.IsValidId(poster) && creator == poster
}
