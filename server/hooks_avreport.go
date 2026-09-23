package main

import (
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/mattermost/mattermost/server/public/model"

	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/avreport"
	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/decorators"
	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/errcode"
)

var avreportFenceLabels = []string{"metar", "speci", "taf", "notam"}

func (p *Plugin) avreportStamp(post *model.Post, ref time.Time) (*model.Post, bool) {
	return p.runStamper(post, p.avreportRendersEnabled, errcode.HooksAvReportPanic,
		"tactical-fusion: recovered from panic while reading an aviation report; post left unmodified",
		func(post *model.Post) (*model.Post, bool) { return p.recognizeAvReport(post, ref) })
}

func (p *Plugin) recognizeAvReport(post *model.Post, ref time.Time) (*model.Post, bool) {
	api := p.API

	source, found := p.avreportSource(post)
	if !found || !p.avreportSurfaceEnabled(source) {
		return nil, false
	}

	report, err := avreport.Decode(source.Text, ref)
	if err != nil {
		if api != nil && labeledAvReportFence(post.Message) {
			api.LogWarn("tactical-fusion: an aviation report could not be read; posting unstamped",
				"error_code", errcode.HooksAvReportUnreadable, "channel_id", post.ChannelId, "error", err)
		}
		p.reportAvReportRefusal(post, source, errcode.HooksAvReportUnreadable,
			"The aviation report you just posted could not be read, so it was left as ordinary text.")
		return nil, false
	}
	if !avreport.KindEnabled(p.avreportFormats(), report.Kind) {
		return nil, false
	}

	prefersCard := report.IsRestriction() && p.avreportCardEnabled()
	if source.Kind == avreport.SourceMessage && p.avreportFormats().Table && !prefersCard {
		if expanded, ok := p.expandAvReport(post, report, ref); ok {
			return expanded, true
		}
	}
	if !p.avreportCardEnabled() {
		return nil, false
	}

	rungs := []stampRung{
		{avreport.Props(report, source), false},
		{avreport.PropsWithoutRows(report, source), true},
	}

	updated, code := p.commitStamped(post.Clone(), avreport.PostType, avreport.PropsKey, rungs, stampCodes{
		propsUnmeasurable: errcode.HooksAvReportPropsUnmeasurable,
		propsTooLarge:     errcode.HooksAvReportPropsTooLarge,
		degraded:          errcode.HooksAvReportRowsDropped,
		degradedMessage:   "tactical-fusion: the decoded report carried more rows than the post props map has room for; stamping without them",
	})
	if updated == nil {
		p.reportAvReportRefusal(post, source, code, avreportRefusalMessage(code))
		return nil, false
	}

	return updated, true
}

func (p *Plugin) expandAvReport(post *model.Post, report avreport.Report, ref time.Time) (*model.Post, bool) {
	if p.decorators == nil {
		return nil, false
	}

	tagger := &decorators.Tagger{Registry: p.decorators, URLPrefix: p.decorateURLPrefix()}
	href := tagger.URLFor(avreport.Type, url.Values{
		avreport.ParamValue:   {report.Raw},
		avreport.ParamInstant: {strconv.FormatInt(report.Instant(), 10)},
	})

	table, ok := avreport.Expanded(href, report)
	if !ok {
		return nil, false
	}
	message := tagger.Decorate(table, ref)
	if utf8.RuneCountInString(message) > safePostRunes {
		return nil, false
	}

	updated := post.Clone()
	updated.Message = message
	return updated, true
}

func avreportRefusalMessage(code int) string {
	if code == errcode.HooksAvReportPropsUnmeasurable {
		return "The aviation report you just posted could not be rendered, because something else attached to the post could not be read. It was left as ordinary text."
	}

	return "The aviation report you just posted carries too much detail to render, so it was left as ordinary text."
}

func (p *Plugin) avreportSource(post *model.Post) (avreport.Source, bool) {
	if block, ok := decorators.SoleFencedBlock(post.Message); ok && avreportInfoString(block.Info) {
		return avreport.Source{
			Kind:  avreport.SourceFence,
			Lead:  block.Lead,
			Trail: block.Trail,
			Text:  block.Body,
		}, true
	}

	text := strings.TrimSpace(strings.ReplaceAll(post.Message, "\r\n", "\n"))
	if strings.Count(text, "\n") == 0 {
		return avreport.Source{}, false
	}
	if decorators.HasCodeSpan(post.Message) {
		return avreport.Source{}, false
	}
	if !avreport.LooksLikeHeader(strings.SplitN(text, "\n", 2)[0]) {
		return avreport.Source{}, false
	}

	return avreport.Source{Kind: avreport.SourceMessage, Text: text}, true
}

func (p *Plugin) messageShowsAvReport(post *model.Post) bool {
	source, ok := p.avreportSource(post)
	if !ok || !p.avreportSurfaceEnabled(source) {
		return false
	}

	report, err := avreport.Decode(source.Text, referenceTime(post))
	if err != nil {
		return labeledAvReportFence(post.Message)
	}

	return avreport.KindEnabled(p.avreportFormats(), report.Kind)
}

func (p *Plugin) reportAvReportRefusal(post *model.Post, source avreport.Source, code int, message string) {
	if p.API == nil || source.Kind != avreport.SourceFence || post.UserId == "" {
		return
	}
	if !labeledAvReportFence(post.Message) {
		return
	}

	p.API.SendEphemeralPost(post.UserId, &model.Post{
		ChannelId: post.ChannelId,
		RootId:    post.RootId,
		Message:   errcode.WithCode(code, message),
	})
}

func labeledAvReportFence(message string) bool {
	block, ok := decorators.SoleFencedBlock(message)
	return ok && avreportInfoString(block.Info)
}

func avreportInfoString(info string) bool {
	return slices.Contains(avreportFenceLabels, strings.ToLower(strings.TrimSpace(info)))
}
