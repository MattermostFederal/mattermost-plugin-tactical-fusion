package main

import (
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/mattermost/mattermost/server/public/model"

	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/decorators"
	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/decorators/note"
	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/errcode"
)

const noteLabelSeparator = "|"

func (p *Plugin) noteResponse(args *model.CommandArgs, text string) *model.CommandResponse {
	label, markdown, found := strings.Cut(text, noteLabelSeparator)
	label, markdown = strings.TrimSpace(label), strings.TrimSpace(markdown)
	if !found || label == "" || markdown == "" || strings.ContainsAny(label, "\r\n") {
		return ephemeralResponse(errcode.WithCode(errcode.CommandNoteUsage,
			"Usage: `/"+commandTrigger+" note <label> | <markdown>`. The label is one line and may not contain `|`; "+
				"the markdown may span several lines."))
	}

	params, ok := (&note.Decorator{}).Parse(markdown, time.Time{})
	if !ok {
		return ephemeralResponse(errcode.WithCode(errcode.CommandNoteInvalid,
			"A note is up to "+strconv.Itoa(note.MaxNoteRunes)+" characters of markdown."))
	}

	tagger := &decorators.Tagger{URLPrefix: p.decorateURLPrefix()}
	message := tagger.LinkFor(note.Type, label, params)
	if utf8.RuneCountInString(message) > safePostRunes {
		return ephemeralResponse(errcode.WithCode(errcode.CommandNoteTooLong,
			"That note does not fit in a post once it is written into a link. Shorten the markdown and try again."))
	}

	post := &model.Post{
		UserId:    args.UserId,
		ChannelId: args.ChannelId,
		RootId:    args.RootId,
		Message:   message,
	}
	if _, appErr := p.API.CreatePost(post); appErr != nil {
		p.API.LogError("tactical-fusion: could not post a note",
			"error_code", errcode.CommandNotePostFailed, "error", appErr.Error())
		return ephemeralResponse(errcode.WithCode(errcode.CommandNotePostFailed,
			"Could not post the note to this channel."))
	}

	return &model.CommandResponse{}
}
