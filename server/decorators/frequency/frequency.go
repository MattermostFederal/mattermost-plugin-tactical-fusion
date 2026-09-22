package frequency

import (
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/decorators"
	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/errcode"
)

const Type = "frequency"

const ParamValue = "v"

const label = "FREQ"

const (
	frequencyBody = `(?:\d{1,4}\.\d{1,3}|\d{4,5})(?:[ \t]*[A-Z]HZ)?`

	separator = `[ \t]*:`

	setTerminator = `(?://)?`
)

const (
	MinKHz = 2_000

	MaxKHz = 1_300_000
)

var scanExpr = `(` + label + separator + `(` + frequencyBody + `))` + setTerminator

var scanPattern = regexp.MustCompile(scanExpr)

var frequencyShape = regexp.MustCompile(`^(\d{1,4}\.\d{1,3}|\d{4,5})(?:[ \t]*(MHZ|KHZ))?$`)

func BodyExpr() string { return frequencyBody }

type Formats struct {
	Frequency bool
}

var AllFormats = Formats{Frequency: true}

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

func (d *Decorator) Patterns() []decorators.Pattern {
	if !d.formats().Frequency {
		return nil
	}

	return []decorators.Pattern{{
		Regexp:       scanPattern,
		ReplaceGroup: 1,
		Extract:      func(m []string) string { return m[2] },
		Boundary:     decorators.BoundaryOK,
	}}
}

func (d *Decorator) Parse(value string, _ time.Time) (url.Values, bool) {
	if _, ok := ParseToken(value); !ok {
		return nil, false
	}
	return url.Values{ParamValue: {value}}, true
}

type Frequency struct {
	Token string
	KHz   int
	Unit  string
}

func ParseToken(value string) (Frequency, bool) {
	m := frequencyShape.FindStringSubmatch(value)
	if m == nil {
		return Frequency{}, false
	}
	number, unit := m[1], m[2]

	khz, ok := toKHz(number, unit)
	if !ok || khz < MinKHz || khz > MaxKHz {
		return Frequency{}, false
	}

	return Frequency{Token: value, KHz: khz, Unit: unit}, true
}

func toKHz(number, unit string) (int, bool) {
	whole, fraction, decimal := strings.Cut(number, ".")
	if unit == "" {
		if decimal {
			unit = "MHZ"
		} else {
			unit = "KHZ"
		}
	}

	wholeN, err := strconv.Atoi(whole)
	if err != nil {
		return 0, false
	}
	fractionN := 0
	if decimal {
		padded := (fraction + "000")[:3]
		fractionN, err = strconv.Atoi(padded)
		if err != nil {
			return 0, false
		}
	}

	switch unit {
	case "MHZ":
		return wholeN*1000 + fractionN, true
	case "KHZ":
		if fractionN != 0 {
			return 0, false
		}
		return wholeN, true
	}
	return 0, false
}

func Validate(params url.Values) (Frequency, bool) {
	return ParseToken(params.Get(ParamValue))
}

func (d *Decorator) RenderPage(w http.ResponseWriter, params url.Values) {
	f, ok := Validate(params)
	if !ok {
		decorators.WriteError(w, http.StatusBadRequest,
			errcode.WithCode(errcode.FrequencyPageInvalid, "That is not a frequency this plugin issued."))
		return
	}

	w.Header().Set("Cache-Control", "private, max-age=300")

	decorators.WritePage(w, decorators.Page{
		Title:    pageTitle,
		Theme:    decorators.ThemeFromParams(params),
		StyleCSS: pageStyles,
		BodyHTML: renderBody(Describe(f)),
	})
}
