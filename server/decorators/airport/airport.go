package airport

import (
	"errors"
	"net/http"
	"net/url"
	"regexp"
	"time"

	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/decorators"
	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/errcode"
)

const Type = "airport"

const ParamValue = "v"

const ParamIATA = "i"

type Formats struct {
	Airfield bool
	IATA     bool
	Table    bool
	Route    bool
}

var AllFormats = Formats{Airfield: true, IATA: true, Table: true, Route: true}

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

func (d *Decorator) ExpandMessage(href, trail string, params url.Values) string {
	if !d.formats().Table || href == "" {
		return ""
	}

	ref, err := ReferenceFromParams(params)
	if err != nil {
		return ""
	}
	ident, ok := ref.Resolve()
	if !ok {
		return ""
	}

	fields, ok := DescribeFields(ident)
	if !ok {
		return ""
	}

	return airfieldTable(href, trail, fields)
}

var scanPattern = regexp.MustCompile(scanExpr)

var iataScanPattern = regexp.MustCompile(iataScanExpr)

func (d *Decorator) Patterns() []decorators.Pattern {
	formats := d.formats()

	var patterns []decorators.Pattern
	if formats.Airfield {
		patterns = append(patterns, labeledPattern(scanPattern))
	}
	if formats.Airfield && formats.IATA {
		patterns = append(patterns, labeledPattern(iataScanPattern))
	}
	return patterns
}

func labeledPattern(re *regexp.Regexp) decorators.Pattern {
	return decorators.Pattern{
		Regexp:       re,
		ReplaceGroup: 1,
		Extract:      func(m []string) string { return m[2] },
		Boundary:     decorators.BoundaryOK,
	}
}

func (d *Decorator) Parse(value string, _ time.Time) (url.Values, bool) {
	switch {
	case MatchesIdentShape(value):
		if _, ok := Lookup(value); !ok {
			return nil, false
		}
		return url.Values{ParamValue: {value}}, true
	case MatchesIATAShape(value):
		if _, ok := LookupIATA(value); !ok {
			return nil, false
		}
		return url.Values{ParamIATA: {value}}, true
	}

	return nil, false
}

type Reference struct {
	Ident string
	IATA  string
}

var ErrParamsConflict = errors.New("airport: exactly one of v and i is required")

var ErrParamsInvalid = errors.New("airport: not an airfield code this plugin issued")

func ReferenceFromParams(params url.Values) (Reference, error) {
	hasIdent, hasIATA := params.Has(ParamValue), params.Has(ParamIATA)
	if hasIdent == hasIATA {
		return Reference{}, ErrParamsConflict
	}

	if hasIdent {
		ident := params.Get(ParamValue)
		if !MatchesIdentShape(ident) {
			return Reference{}, ErrParamsInvalid
		}
		return Reference{Ident: ident}, nil
	}

	code := params.Get(ParamIATA)
	if !MatchesIATAShape(code) {
		return Reference{}, ErrParamsInvalid
	}
	return Reference{IATA: code}, nil
}

func (r Reference) Code() string {
	if r.Ident != "" {
		return r.Ident
	}
	return r.IATA
}

func (r Reference) Resolve() (string, bool) {
	if r.Ident != "" {
		_, ok := Lookup(r.Ident)
		return r.Ident, ok
	}
	a, ok := LookupIATA(r.IATA)
	if !ok {
		return "", false
	}
	return a.Ident, true
}

func (d *Decorator) RenderPage(w http.ResponseWriter, params url.Values) {
	ref, err := ReferenceFromParams(params)
	if errors.Is(err, ErrParamsConflict) {
		decorators.WriteError(w, http.StatusBadRequest,
			errcode.WithCode(errcode.AirportPageParamsConflict, "An airfield link names exactly one code."))
		return
	}
	if err != nil {
		decorators.WriteError(w, http.StatusBadRequest,
			errcode.WithCode(errcode.AirportPageInvalid, "That is not an airfield code this plugin issued."))
		return
	}

	var details Details
	ident, found := ref.Resolve()
	if found {
		details, found = Describe(ident)
	}

	w.Header().Set("Cache-Control", "private, max-age=300")

	decorators.WritePage(w, decorators.Page{
		Title:    pageTitle,
		Theme:    decorators.ThemeFromParams(params),
		StyleCSS: pageStyles,
		BodyHTML: renderBody(ref.Code(), details, found),
	})
}
