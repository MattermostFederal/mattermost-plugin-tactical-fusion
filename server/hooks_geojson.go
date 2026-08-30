package main

import (
	"strings"

	"github.com/mattermost/mattermost/server/public/model"

	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/decorators"
	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/errcode"
	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/geojson"
)

// geoJSONFenceInfo is the only fence spelling this build recognizes.
//
// There is deliberately no "json". Stamping a post costs it its search matches,
// its embeds permanently, an author cannot undo it, and
// ordinary JSON is pasted into chat constantly. An author who wants the card
// can spell the fence geojson at no cost, so the ambiguous spelling buys
// convenience at a price this plugin should not charge by default.
const geoJSONFenceInfo = "geojson"

// geoJSONLooseFenceInfo and the .json suffix are the UNLABELED spellings, and
// they are read only while EnableGeoJSONUnlabeled is on.
//
// Off by default, and the default is the argument. Stamping a post costs it its
// search matches and its embeds permanently, an author
// cannot undo it, and ordinary JSON is pasted into chat constantly. An install
// that knows its channels carry overlays rather than API payloads can turn it
// on; nobody gets it by accident.
const geoJSONLooseFenceInfo = "json"

var geoJSONFileSuffixes = []string{".geojson"}

var geoJSONLooseFileSuffixes = []string{".json"}

func (p *Plugin) geoJSONStamp(post *model.Post) (result *model.Post, stamped bool) {
	api := p.API

	var stripped *model.Post

	// First statement, for the reason cotStamp's is: the span has to cover
	// geoJSONSource, which calls the filestore, and geojson.Parse. There is no
	// other recover on this path.
	defer func() {
		if r := recover(); r != nil {
			result, stamped = stripped, false
			if api != nil {
				api.LogWarn("tactical-fusion: recovered from panic while reading a GeoJSON document; post left unmodified",
					"error_code", errcode.HooksGeoJSONPanic, "panic", r)
			}
		}
	}()

	if stripped = stripStampedTypes(post); stripped != nil {
		post = stripped
	}

	if !p.geoJSONEnabled() || post.Type != "" {
		return stripped, false
	}

	source, found := p.geoJSONSource(post)
	if !found {
		return stripped, false
	}

	document, err := geojson.Parse([]byte(source.Text))
	if err != nil {
		// Logged only for a source the author LABELED. With the unlabeled
		// switch on, SoleObjectSpan matches any brace pair, so "Set it to {5}"
		// and every pasted snippet reached this and wrote a warn: the same
		// "ambiguous, so stay silent" argument that gates the ephemeral below
		// has to gate the log, or the log stops being worth reading.
		if api != nil && labeledGeoJSONFence(post.Message) {
			api.LogWarn("tactical-fusion: a GeoJSON source could not be read; posting unstamped",
				"error_code", errcode.HooksGeoJSONUnreadable, "channel_id", post.ChannelId, "error", err)
		}
		p.reportGeoJSONRefusal(post, source, errcode.HooksGeoJSONUnreadable,
			"The GeoJSON document you just posted could not be read, so it was left as ordinary text.")
		return stripped, false
	}

	rungs := []stampRung{
		{geojson.Props(document, source), false},
		{geojson.PropsWithoutProperties(document, source), true},
	}

	updated, code := p.commitStamped(post.Clone(), geojson.PostType, geojson.PropsKey, rungs, stampCodes{
		propsUnmeasurable: errcode.HooksGeoJSONPropsUnmeasurable,
		propsTooLarge:     errcode.HooksGeoJSONPropsTooLarge,
		degraded:          errcode.HooksGeoJSONPropertiesDropped,
		degradedMessage:   "tactical-fusion: the parsed document carried more properties than the post props map has room for; stamping without them",
	})
	if updated == nil {
		p.reportGeoJSONRefusal(post, source, code, geoJSONRefusalMessage(code))
		return stripped, false
	}

	return updated, true
}

func geoJSONRefusalMessage(code int) string {
	if code == errcode.HooksGeoJSONPropsUnmeasurable {
		return "The GeoJSON document you just posted could not be rendered, because something else attached to the post could not be read. It was left as ordinary text."
	}

	return "The GeoJSON document you just posted carries too much detail to render, so it was left as ordinary text."
}

func (p *Plugin) geoJSONSource(post *model.Post) (geojson.Source, bool) {
	if source, ok := p.messageGeoJSONSource(post); ok {
		return source, true
	}

	return p.geoJSONFileSource(post)
}

// messageGeoJSONSource is everything geoJSONSource reads out of the VISIBLE
// message, split out so the cross-format collision gate asks exactly the
// question the reader will ask.
func (p *Plugin) messageGeoJSONSource(post *model.Post) (geojson.Source, bool) {
	if block, ok := decorators.SoleFencedBlock(post.Message); ok && p.geoJSONFenceInfo(block.Info) {
		return geojson.Source{
			Kind:  geojson.SourceFence,
			Lead:  block.Lead,
			Trail: block.Trail,
			Text:  block.Body,
		}, true
	}

	// A bare document, with no fence around it. Tried after the fence so an
	// author who wrapped one still gets the fence's reading, and before the file
	// so the visible message always wins over an attachment, which is the order
	// cotSource records and the reason for it.
	//
	// Unlabeled, so it is behind the same switch the ambiguous spellings are:
	// with nothing naming the format, the parse is the only thing saying this is
	// a document rather than a payload.
	if p.geoJSONUnlabeledEnabled() {
		if block, ok := decorators.SoleObjectSpan(post.Message); ok {
			return geojson.Source{
				Kind:  geojson.SourceFence,
				Lead:  block.Lead,
				Trail: block.Trail,
				Text:  block.Body,
			}, true
		}
	}

	return geojson.Source{}, false
}

// geoJSONFenceInfo reports whether a fence label names a document this build
// reads.
func (p *Plugin) geoJSONFenceInfo(info string) bool {
	lowered := strings.ToLower(strings.TrimSpace(info))

	if lowered == geoJSONFenceInfo {
		return true
	}

	return lowered == geoJSONLooseFenceInfo && p.geoJSONUnlabeledEnabled()
}

func (p *Plugin) geoJSONFileSource(post *model.Post) (geojson.Source, bool) {
	// The switch first, before any API call, exactly as cotFileSource gates.
	// Reading on the union of the formats' switches would make an install with
	// both file switches off ask the store about every one-attachment post.
	if p.API == nil || !p.geoJSONFilesEnabled() || len(post.FileIds) != 1 {
		return geojson.Source{}, false
	}

	if !model.IsValidId(post.FileIds[0]) {
		return geojson.Source{}, false
	}

	info, appErr := p.API.GetFileInfo(post.FileIds[0])
	if appErr != nil || info == nil {
		p.logGeoJSONFileUnreadable(post, appErr)
		return geojson.Source{}, false
	}
	if !attachmentOwnedBy(info, post) {
		p.API.LogWarn("tactical-fusion: a GeoJSON attachment does not belong to this post; leaving it alone",
			"error_code", errcode.HooksGeoJSONFileNotOwned, "channel_id", post.ChannelId,
			"file_id", info.Id, "user_id", post.UserId)
		return geojson.Source{}, false
	}
	if !p.geoJSONFileName(info.Name) || info.Size > geojson.MaxSourceBytes {
		return geojson.Source{}, false
	}

	content, appErr := p.API.GetFile(info.Id)
	if appErr != nil {
		p.logGeoJSONFileUnreadable(post, appErr)
		return geojson.Source{}, false
	}

	return geojson.Source{
		Kind:     geojson.SourceFile,
		Lead:     post.Message,
		Text:     string(content),
		FileID:   info.Id,
		FileName: info.Name,
	}, true
}

func (p *Plugin) logGeoJSONFileUnreadable(post *model.Post, appErr *model.AppError) {
	if p.API == nil {
		return
	}

	p.API.LogWarn("tactical-fusion: an attached file could not be read; posting unstamped",
		"error_code", errcode.HooksGeoJSONFileUnreadable, "channel_id", post.ChannelId, "error", appErr)
}

// reportGeoJSONRefusal answers an author who labeled a fence geojson and got
// nothing.
//
// Fences only, matching reportCotRefusal: an ephemeral for an attachment would
// be answering a file rather than something the author wrote.
func (p *Plugin) reportGeoJSONRefusal(post *model.Post, source geojson.Source, code int, message string) {
	if p.API == nil || source.Kind != geojson.SourceFence || post.UserId == "" {
		return
	}

	// Only the geojson spelling. A json fence and a bare object are ambiguous,
	// and silence is right for both: a message on every one of those would fire
	// constantly on ordinary posts, which is the argument reportCotRefusal makes
	// about an xml fence. Labeling a fence geojson is how an author says they
	// meant it, and asking is what earns them the answer.
	if !labeledGeoJSONFence(post.Message) {
		return
	}

	p.API.SendEphemeralPost(post.UserId, &model.Post{
		ChannelId: post.ChannelId,
		RootId:    post.RootId,
		Message:   errcode.WithCode(code, message),
	})
}

// labeledGeoJSONFence reports whether the author named the format themselves,
// which is what earns both the log line and the ephemeral.
func labeledGeoJSONFence(message string) bool {
	block, ok := decorators.SoleFencedBlock(message)
	return ok && geoJSONInfoString(block.Info)
}

func geoJSONInfoString(info string) bool {
	return strings.EqualFold(strings.TrimSpace(info), geoJSONFenceInfo)
}

func (p *Plugin) geoJSONFileName(name string) bool {
	lowered := strings.ToLower(name)

	suffixes := geoJSONFileSuffixes
	if p.geoJSONUnlabeledEnabled() {
		suffixes = append(append([]string(nil), suffixes...), geoJSONLooseFileSuffixes...)
	}

	for _, suffix := range suffixes {
		if strings.HasSuffix(lowered, suffix) {
			return true
		}
	}
	return false
}

func (p *Plugin) messageShowsGeoJSON(post *model.Post) bool {
	if !p.geoJSONEnabled() {
		return false
	}

	source, ok := p.messageGeoJSONSource(post)
	if !ok || source.Kind != geojson.SourceFence {
		return false
	}

	if labeledGeoJSONFence(post.Message) {
		return true
	}

	_, err := geojson.Parse([]byte(source.Text))

	return err == nil
}
