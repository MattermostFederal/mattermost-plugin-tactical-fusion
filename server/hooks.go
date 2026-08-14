package main

import (
	"net/url"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/plugin"

	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/decorators"
	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/errcode"
)

// The post size limits, and why there are two of them.
//
// There is no way to ask for the real one. `Post.IsValid` takes it as an
// argument, the server computes it, and neither `plugin.API` nor the
// `model.Config` it hands back exposes it. So this plugin works from the two
// constants the SDK does offer, and uses each where its failure mode is
// survivable.
//
// The two directions are not symmetric, which is the whole reason for the
// split. A limit set too high in `decoratePost` means a decorated message the
// server then refuses, so the AUTHOR CANNOT POST AT ALL. Too low there only
// means occasionally skipping decoration. In the slash commands it is the
// difference between a post that lands and one that is refused, which is
// reported and recoverable.
const (
	// safePostRunes is the floor every server accepts. PostMessageMaxRunesV1
	// is what the model validated against before the column widened, and no
	// store returns less, so a message under this is never refused for length.
	//
	// This is what `decoratePost` uses, because that is the call site where
	// being wrong stops somebody posting.
	safePostRunes = model.PostMessageMaxRunesV1

	// defaultPostRunes is what a server normally accepts: PostMessageMaxRunesV2
	// is PostMessageMaxBytesV2/4, the worst-case rune count for the TEXT column
	// the message is stored in, and is what the Postgres and MySQL stores
	// report by default.
	//
	// Used by the slash commands, which write their own posts and can be told
	// they were refused. It is a good guess and not a guarantee: an admin or a
	// store can report less, so anything using this has to survive being wrong.
	defaultPostRunes = model.PostMessageMaxRunesV2
)

// MessageWillBePosted decorates a message once, as it is created.
//
// There is deliberately no MessageWillBeUpdated. Edits are stored verbatim, so
// the plugin never transforms text a user has deliberately authored. Editing a
// decorator link is the supported way to change or remove it.
func (p *Plugin) MessageWillBePosted(_ *plugin.Context, post *model.Post) (*model.Post, string) {
	return p.decoratePost(post, referenceTime(post)), ""
}

// referenceTime is what an undated short-form token is resolved against.
//
// CreateAt is normally unset at this point and the server fills it in later, so
// "now" is the right answer. It is honored when present because an imported or
// scheduled post carries its real timestamp, and the page tells the reader the
// inferred month and year came from the post date.
func referenceTime(post *model.Post) time.Time {
	if post != nil && post.CreateAt > 0 {
		return model.GetTimeForMillis(post.CreateAt).UTC()
	}
	return time.Now().UTC()
}

// decoratePost returns a rewritten copy of the post, or nil to leave it alone.
//
// nil is the documented "allow without modification" value. The second return
// value of the hook is always the empty string: a bug in here must never stop
// somebody from posting.
func (p *Plugin) decoratePost(post *model.Post, ref time.Time) (result *model.Post) {
	// Capture the API before the deferred call. If a nil or broken API was what
	// panicked in the first place, logging through p.API inside the recover
	// would panic again from within the deferred function, which escapes the
	// hook entirely and defeats the point of recovering.
	api := p.API

	defer func() {
		if r := recover(); r != nil {
			result = nil
			if api != nil {
				api.LogWarn("tactical-fusion: recovered from panic while decorating; post left unmodified",
					"error_code", errcode.HooksDecoratePanic, "panic", r)
			}
		}
	}()

	if post == nil || post.Message == "" {
		return nil
	}
	if p.decorators == nil {
		return nil
	}
	if isSystemPost(post) {
		return nil
	}

	tagger := &decorators.Tagger{Registry: p.decorators, URLPrefix: p.decorateURLPrefix()}
	decorated := tagger.Decorate(post.Message, ref)
	if decorated == post.Message {
		return nil
	}

	if utf8.RuneCountInString(decorated) > safePostRunes {
		// A 12-character DTG becomes roughly 120 once linked, so a message that
		// is visibly well under the limit can cross it here. Rejecting the post
		// would show the author an opaque "too long" error for text they can
		// see fits, so drop the decoration instead.
		// Through the captured api, for the same reason the recover above uses
		// it: this function must not be the thing that stops somebody posting,
		// and that includes its own logging.
		if api != nil {
			api.LogWarn("tactical-fusion: decoration would exceed the maximum post size; posting undecorated",
				"error_code", errcode.HooksDecorationTooLong, "channel_id", post.ChannelId)
		}
		return nil
	}

	updated := post.Clone()
	updated.Message = decorated
	return updated
}

// isSystemPost reports whether a post is server-generated chrome.
//
// The deny list is deliberately narrow. Skipping every non-empty Type would
// also skip custom post types from integrations and other plugins, which may
// carry real mission content.
func isSystemPost(post *model.Post) bool {
	return strings.HasPrefix(post.Type, model.PostSystemMessagePrefix)
}

// decorateURLPrefix builds everything before the decorator type.
//
// The result is root-relative and never carries a scheme or host, so it follows
// whichever server the reader is on. That is the whole point: an absolute URL
// would freeze the hostname into every historical post and break them all the
// day the server moves.
//
// SiteURL is consulted for one thing only, its path component, which is what
// makes a subpath install like https://host/mattermost work. Everything else
// about it is irrelevant here, so an unset, empty or malformed SiteURL is not a
// reason to skip decoration: it simply means "no subpath", and "/plugins/..."
// resolves correctly against the server the reader already has open.
//
// A path that is not rooted is ignored rather than used. "example.com/mm" parses
// with the whole string in Path, and emitting that would produce a relative URL
// resolving against whatever page the reader happens to be on.
func (p *Plugin) decorateURLPrefix() string {
	return siteURLPath(p.API.GetConfig()) + "/plugins/" + manifest.Id + decoratePath
}

func siteURLPath(config *model.Config) string {
	if config == nil || config.ServiceSettings.SiteURL == nil {
		return ""
	}

	raw := strings.TrimSpace(*config.ServiceSettings.SiteURL)
	if raw == "" {
		return ""
	}

	parsed, err := url.Parse(raw)
	if err != nil || !strings.HasPrefix(parsed.Path, "/") {
		return ""
	}

	return strings.TrimSuffix(parsed.Path, "/")
}
