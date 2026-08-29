package main

import (
	"strings"

	"github.com/mattermost/mattermost/server/public/model"

	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/cot"
	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/decorators"
	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/errcode"
)

var cotFileSuffixes = []string{".xml", ".cot"}

func (p *Plugin) cotStamp(post *model.Post) (result *model.Post, stamped bool) {
	api := p.API

	// Declared before the defer so the recover can return it. A panic must not
	// hand back a post still wearing a type this hook did not write, which is
	// what returning nil from here would do.
	var stripped *model.Post

	// First statement on purpose. The span has to cover cotSource, which calls
	// the filestore, and cot.Parse, not just the commit: decoratePost calls
	// this from outside decorateMessage's recover, so there is no other one on
	// the hook path.
	defer func() {
		if r := recover(); r != nil {
			result, stamped = stripped, false
			if api != nil {
				api.LogWarn("tactical-fusion: recovered from panic while reading a Cursor on Target event; post left unmodified",
					"error_code", errcode.HooksCotPanic, "panic", r)
			}
		}
	}()

	if stripped = stripStampedTypes(post); stripped != nil {
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

	// The ladder, widest rung first. Dropping the extension keys leaves exactly
	// the card this feature shipped with, which is a better answer than raw XML
	// for a post that would have stamped before the registry existed.
	rungs := []stampRung{
		{cot.Props(events, source), false},
		{cot.PropsWithoutDetail(events, source), true},
	}

	updated, code := p.commitStamped(post.Clone(), cot.PostType, cot.PropsKey, rungs, stampCodes{
		propsUnmeasurable: errcode.HooksCotPropsUnmeasurable,
		propsTooLarge:     errcode.HooksCotPropsTooLarge,
		degraded:          errcode.HooksCotDetailDropped,
		degradedMessage:   "tactical-fusion: the parsed event carried more detail than the post props map has room for; stamping without it",
	})
	if updated == nil {
		p.reportCotRefusal(post, source, code, cotRefusalMessage(code))
		return stripped, false
	}

	return updated, true
}

func cotRefusalMessage(code int) string {
	if code == errcode.HooksCotPropsUnmeasurable {
		return "The Cursor on Target event you just posted could not be rendered, because something else attached to the post could not be read. It was left as ordinary text."
	}

	return "The Cursor on Target event you just posted carries too much detail to render, so it was left as ordinary text."
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
	// block: an author who fenced an event without labeling the fence has said
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

	// The visible message wins. See messageShowsGeoJSON.
	if p.messageShowsGeoJSON(post) {
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
	if !attachmentOwnedBy(info, post) {
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
