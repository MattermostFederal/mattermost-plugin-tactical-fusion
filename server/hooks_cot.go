package main

import (
	"encoding/json"
	"maps"
	"strings"
	"unicode/utf8"

	"github.com/mattermost/mattermost/server/public/model"

	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/cot"
	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/decorators"
	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/errcode"
)

const cotPropsBudgetRunes = model.PostPropsMaxUserRunes

var cotFileSuffixes = []string{".cot", ".xml"}

func (p *Plugin) cotStamp(post *model.Post) (result *model.Post, stamped bool) {
	api := p.API

	// Declared before the defer so the recover can return it. A panic must not
	// hand back a post still wearing a type this hook did not write, which is
	// what returning nil from here would do.
	var stripped *model.Post

	defer func() {
		if r := recover(); r != nil {
			result, stamped = stripped, false
			if api != nil {
				api.LogWarn("tactical-fusion: recovered from panic while reading a Cursor on Target event; post left unmodified",
					"error_code", errcode.HooksCotPanic, "panic", r)
			}
		}
	}()

	// A post arriving already wearing this type was not written by this hook:
	// Post.IsValid accepts any custom_ type from an ordinary client, so anyone
	// who can post can otherwise hand a reader a card whose fields and whose
	// own XML pane were both authored to agree with each other. Strip it and
	// let recognition decide again from the message. Never refuse the post.
	if post.Type == cot.PostType {
		stripped = post.Clone()
		stripped.Type = ""
		stripped.DelProp(cot.PropsKey)
		post = stripped
	}

	if !p.cotEnabled() || post.Type != "" {
		return stripped, false
	}

	source, found := p.cotSource(post)
	if !found {
		return stripped, false
	}

	events, err := cot.Parse([]byte(source.Text))
	if err != nil {
		if api != nil {
			api.LogWarn("tactical-fusion: a Cursor on Target source could not be read; posting unstamped",
				"error_code", errcode.HooksCotUnreadable, "channel_id", post.ChannelId, "error", err)
		}
		p.reportCotRefusal(post, source, errcode.HooksCotUnreadable,
			"The Cursor on Target event you just posted could not be read, so it was left as ordinary text.")
		return stripped, false
	}

	updated := post.Clone()

	// The ladder, widest rung first. Dropping the extension keys leaves exactly
	// the card this feature shipped with, which is a better answer than raw XML
	// for a post that would have stamped before the registry existed.
	rungs := []struct {
		blob     map[string]any
		degraded bool
	}{
		{cot.Props(events, source), false},
		{cot.PropsWithoutDetail(events, source), true},
	}

	for _, rung := range rungs {
		props := cotProps(updated, rung.blob)

		// Marshalled here rather than through model.StringInterfaceToJSON, which
		// discards the error and answers "". An unmeasurable props map would score
		// zero runes and sail through the one gate that exists to stop the server
		// refusing the post.
		encoded, err := json.Marshal(props)
		if err != nil {
			// Not the author's problem and not a size problem: the value that
			// will not marshal came from elsewhere on the post, and no rung of
			// this ladder can shed it. Say so once and stop.
			if api != nil {
				api.LogWarn("tactical-fusion: the post's props could not be measured; posting unstamped",
					"error_code", errcode.HooksCotPropsUnmeasurable, "channel_id", post.ChannelId, "error", err)
			}
			p.reportCotRefusal(post, source, errcode.HooksCotPropsUnmeasurable,
				"The Cursor on Target event you just posted could not be rendered, because something else attached to the post could not be read. It was left as ordinary text.")
			return stripped, false
		}

		if utf8.RuneCountInString(string(encoded)) > cotPropsBudgetRunes {
			continue
		}

		if rung.degraded && api != nil {
			api.LogWarn("tactical-fusion: the parsed event carried more detail than the post props map has room for; stamping without it",
				"error_code", errcode.HooksCotDetailDropped, "channel_id", post.ChannelId)
		}

		updated.Type = cot.PostType
		updated.SetProps(props)

		return updated, true
	}

	if api != nil {
		api.LogWarn("tactical-fusion: the parsed event would exceed the maximum post props size; posting unstamped",
			"error_code", errcode.HooksCotPropsTooLarge, "channel_id", post.ChannelId)
	}
	p.reportCotRefusal(post, source, errcode.HooksCotPropsTooLarge,
		"The Cursor on Target event you just posted carries too much detail to render, so it was left as ordinary text.")

	return stripped, false
}

func cotProps(post *model.Post, blob map[string]any) model.StringInterface {
	props := make(model.StringInterface, len(post.GetProps())+1)
	maps.Copy(props, post.GetProps())
	props[cot.PropsKey] = blob

	return props
}

func (p *Plugin) cotSource(post *model.Post) (cot.Source, bool) {
	if block, ok := decorators.SoleFencedBlock(post.Message); ok && cotInfoString(block.Info) {
		return cot.Source{
			Kind:  cot.SourceFence,
			Lead:  block.Lead,
			Trail: block.Trail,
			Text:  block.Body,
		}, true
	}

	// A bare event, with no fence around it. Tried after the fence so an author
	// who wrapped one still gets the fence's stricter reading, and before the
	// file so the visible message always wins over an attachment.
	//
	// decorators.SoleElementSpan is what keeps this off text inside a code
	// block: an author who fenced an event without labelling the fence has said
	// it is code, and rewriting it anyway would be the corruption protected
	// ranges exist to stop.
	if block, ok := decorators.SoleElementSpan(post.Message, cotElementName); ok {
		return cot.Source{
			Kind:  cot.SourceFence,
			Lead:  block.Lead,
			Trail: block.Trail,
			Text:  block.Body,
		}, true
	}

	return p.cotFileSource(post)
}

func (p *Plugin) cotFileSource(post *model.Post) (cot.Source, bool) {
	if p.API == nil || !p.cotFilesEnabled() || len(post.FileIds) != 1 {
		return cot.Source{}, false
	}

	if !model.IsValidId(post.FileIds[0]) {
		return cot.Source{}, false
	}

	info, appErr := p.API.GetFileInfo(post.FileIds[0])
	if appErr != nil || info == nil {
		p.logCotFileUnreadable(post, appErr)
		return cot.Source{}, false
	}
	if !cotFileOwnedBy(info, post) {
		p.API.LogWarn("tactical-fusion: a Cursor on Target attachment does not belong to this post; leaving it alone",
			"error_code", errcode.HooksCotFileNotOwned, "channel_id", post.ChannelId,
			"file_id", info.Id, "user_id", post.UserId)
		return cot.Source{}, false
	}
	if !cotFileName(info.Name) || info.Size > cot.MaxSourceBytes {
		return cot.Source{}, false
	}

	content, appErr := p.API.GetFile(info.Id)
	if appErr != nil {
		p.logCotFileUnreadable(post, appErr)
		return cot.Source{}, false
	}

	return cot.Source{
		Kind:     cot.SourceFile,
		Lead:     post.Message,
		Text:     string(content),
		FileID:   info.Id,
		FileName: info.Name,
	}, true
}

func (p *Plugin) logCotFileUnreadable(post *model.Post, appErr *model.AppError) {
	if p.API == nil {
		return
	}

	p.API.LogWarn("tactical-fusion: an attached file could not be read; posting unstamped",
		"error_code", errcode.HooksCotFileUnreadable, "channel_id", post.ChannelId, "error", appErr)
}

// reportCotRefusal answers an author who labeled a fence cot and got nothing.
//
// Only for that spelling: an xml fence is ambiguous and silence is right for
// it. The text names the event rather than the post, because an ephemeral sent
// from this hook carries no PostId and cannot point at anything.
func (p *Plugin) reportCotRefusal(post *model.Post, source cot.Source, code int, message string) {
	if p.API == nil || source.Kind != cot.SourceFence || post.UserId == "" {
		return
	}

	block, ok := decorators.SoleFencedBlock(post.Message)
	if !ok || !strings.EqualFold(block.Info, cotFenceInfo) {
		return
	}

	p.API.SendEphemeralPost(post.UserId, &model.Post{
		ChannelId: post.ChannelId,
		RootId:    post.RootId,
		Message:   errcode.WithCode(code, message),
	})
}

const (
	// cotElementName is the root a Cursor on Target event is written in, and is
	// what a bare event is recognized by.
	cotElementName = "event"

	cotFenceInfo    = "cot"
	cotXMLFenceInfo = "xml"
)

func cotInfoString(info string) bool {
	lowered := strings.ToLower(strings.TrimSpace(info))
	return lowered == cotFenceInfo || lowered == cotXMLFenceInfo
}

func cotFileName(name string) bool {
	lowered := strings.ToLower(name)
	for _, suffix := range cotFileSuffixes {
		if strings.HasSuffix(lowered, suffix) {
			return true
		}
	}
	return false
}

// cotFileOwnedBy reports whether this attachment is the poster's own, unattached
// and live.
//
// post.FileIds arrives from the client and this hook runs BEFORE Mattermost
// binds files to posts, so at this point nothing else has checked that the id
// names a file its sender is allowed to read. Without this, quoting somebody
// else's file id copies up to maxInlineSrcRunes of that file's text into props
// that everyone who can read the attacker's post can read.
func cotFileOwnedBy(info *model.FileInfo, post *model.Post) bool {
	if info.PostId != "" || info.DeleteAt != 0 {
		return false
	}

	return cotFileCreator(info.CreatorId, post.UserId)
}

// cotFileCreator reports whether a file's creator may be read by this post.
//
// Two answers, not one. The poster's own upload is the ordinary case. The other
// is model.UploadNoUserID: plugin.API.UploadFile takes no user id at all, so a
// file an integration uploaded is credited to "nouser" and could never equal
// any post's UserId. Refusing it silently ignored every attachment a companion
// plugin posted, which for a Cursor on Target plugin is the likeliest producer
// of .cot files there is.
//
// It does not reopen what the check closed. A "nouser" file can only be created
// by a plugin or by local mode on the host; a remote client cannot make one and
// cannot learn the id of one while it is unattached.
func cotFileCreator(creator, poster string) bool {
	if creator == model.UploadNoUserID {
		return true
	}

	return model.IsValidId(poster) && creator == poster
}
