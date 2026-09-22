package avreport

import (
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/decorators"
	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/decorators/dtg"
	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/errcode"
)

const (
	ParamValue   = "v"
	ParamInstant = "t"
)

type Formats struct {
	METAR bool
	TAF   bool
	NOTAM bool
}

var AllFormats = Formats{METAR: true, TAF: true, NOTAM: true}

type Decorator struct {
	Enabled func() Formats
}

var _ decorators.Decorator = (*Decorator)(nil)

func (d *Decorator) Type() string { return Type }

func (d *Decorator) formats() Formats {
	if d.Enabled == nil {
		return AllFormats
	}
	return d.Enabled()
}

const (
	station   = `[A-Z][A-Z0-9]{3}`
	timeGroup = `\d{6}Z`
	rest      = `(?:[^\n=]*[^\s=])?`
	trail     = `[ \t]*=?`

	continuation = `(\n[ \t]*(?:FM\d{6}|TEMPO|BECMG|PROB\d{2}|RMK)\b)?`
)

var (
	metarKeywordPattern = regexp.MustCompile(`((?:METAR|SPECI)[ \t]+(?:COR[ \t]+)?` + station + `[ \t]+` + timeGroup + rest + `)` + trail)
	metarBarePattern    = regexp.MustCompile(`(` + station + `[ \t]+` + timeGroup + `[ \t]+(?:AUTO[ \t]+|COR[ \t]+)?(?:\d{3}|VRB)\d{2,3}(?:G\d{2,3})?(?:KT|MPS)` + rest + `)` + trail)
	tafPattern          = regexp.MustCompile(`(TAF[ \t]+(?:AMD[ \t]+|COR[ \t]+)?` + station + `[ \t]+` + timeGroup + `[ \t]+(?:\d{4}/\d{4}|NIL|CNL)` + rest + `)` + trail + continuation)
	notamPattern        = regexp.MustCompile(`(![A-Z]{3}[ \t]+\d{2}/\d{3,4}[ \t]+[A-Z]{3,4}[ \t]+[^\n=]*[^\s=])` + trail)
)

func (d *Decorator) Patterns() []decorators.Pattern {
	formats := d.formats()

	var patterns []decorators.Pattern
	add := func(re *regexp.Regexp, boundary func(before, after rune) bool) {
		patterns = append(patterns, decorators.Pattern{
			Regexp:       re,
			ReplaceGroup: 1,
			Extract:      extractReport,
			Boundary:     boundary,
		})
	}

	if formats.METAR {
		add(metarKeywordPattern, decorators.BoundaryOK)
		add(metarBarePattern, lineStartOK)
	}
	if formats.TAF {
		add(tafPattern, decorators.BoundaryOK)
	}
	if formats.NOTAM {
		add(notamPattern, decorators.BoundaryOK)
	}
	return patterns
}

func extractReport(m []string) string {
	if len(m) > 2 && m[2] != "" {
		return m[0]
	}
	return m[1]
}

func lineStartOK(before, after rune) bool {
	return (before == 0 || before == '\n') && !decorators.BadNeighbor(after)
}

func (d *Decorator) Parse(value string, ref time.Time) (url.Values, bool) {
	if strings.ContainsAny(value, "\r\n") {
		return nil, false
	}

	report, err := Decode(value, ref)
	if err != nil {
		return nil, false
	}
	if !d.kindEnabled(report.Kind) {
		return nil, false
	}

	return url.Values{
		ParamValue:   {report.Raw},
		ParamInstant: {strconv.FormatInt(report.Instant(), 10)},
	}, true
}

func (d *Decorator) kindEnabled(kind string) bool {
	return KindEnabled(d.formats(), kind)
}

func KindEnabled(formats Formats, kind string) bool {
	switch kind {
	case KindMETAR, KindSPECI:
		return formats.METAR
	case KindTAF:
		return formats.TAF
	case KindNOTAM:
		return formats.NOTAM
	}
	return false
}

func Validate(params url.Values) (Report, bool) {
	value := params.Get(ParamValue)
	if value == "" || strings.ContainsAny(value, "\r\n") || utf8Runes(value) > MaxSourceRunes {
		return Report{}, false
	}

	millis, err := strconv.ParseInt(params.Get(ParamInstant), 10, 64)
	if err != nil || millis < dtg.MinInstantMillis || millis > dtg.MaxInstantMillis {
		return Report{}, false
	}

	ref := time.UnixMilli(millis).UTC()
	if millis == 0 {
		ref = time.Now().UTC()
	}

	report, err := Decode(value, ref)
	if err != nil || report.Raw != value {
		return Report{}, false
	}
	if report.Instant() != millis {
		return Report{}, false
	}

	return report, true
}

func utf8Runes(s string) int {
	return len([]rune(s))
}

func (d *Decorator) RenderPage(w http.ResponseWriter, params url.Values) {
	report, ok := Validate(params)
	if !ok {
		decorators.WriteError(w, http.StatusBadRequest,
			errcode.WithCode(errcode.AvReportPageInvalid, "That is not an aviation report this plugin issued."))
		return
	}

	w.Header().Set("Cache-Control", "private, max-age=300")

	decorators.WritePage(w, decorators.Page{
		Title:    pageTitle,
		Theme:    decorators.ThemeFromParams(params),
		StyleCSS: pageStyles,
		BodyHTML: renderBody(report),
	})
}

func KindOf(text string, ref time.Time) string {
	report, err := Decode(text, ref)
	if err != nil {
		return ""
	}
	return report.Kind
}
