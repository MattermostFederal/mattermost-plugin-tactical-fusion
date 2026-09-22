package cyber

import (
	"net/http"
	"net/url"
	"regexp"
	"time"

	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/decorators"
	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/decorators/cyber/intel"
	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/errcode"
)

const Type = "cyber"

const (
	ParamKind  = "k"
	ParamValue = "v"
)

type Formats struct {
	CVE    bool
	CWE    bool
	Attack bool
	IP     bool
	Hash   bool
}

var AllFormats = Formats{CVE: true, CWE: true, Attack: true, IP: true, Hash: true}

func (f Formats) any() bool {
	return f.CVE || f.CWE || f.Attack || f.IP || f.Hash
}

func (f Formats) enabled(kind Kind) bool {
	switch kind {
	case KindCVE:
		return f.CVE
	case KindCWE:
		return f.CWE
	case KindAttack:
		return f.Attack
	case KindIP:
		return f.IP
	case KindHash:
		return f.Hash
	}

	return false
}

type Decorator struct {
	Enabled func() Formats

	Intel func() *intel.Set
}

var _ decorators.Decorator = (*Decorator)(nil)

func (d *Decorator) Type() string { return Type }

func (d *Decorator) formats() Formats {
	if d.Enabled == nil {
		return AllFormats
	}
	return d.Enabled()
}

func (d *Decorator) datasets() *intel.Set {
	if d.Intel == nil {
		return nil
	}
	return d.Intel()
}

var scansByKind = map[Kind][]*regexp.Regexp{
	KindCVE:    {cveScan},
	KindCWE:    {cweScan},
	KindAttack: {attackScan},
	KindIP:     {ipv4Scan, ipv6Scan},
	KindHash:   {sha256LabelScan, sha1LabelScan, md5LabelScan, hashScan},
}

func (d *Decorator) Patterns() []decorators.Pattern {
	formats := d.formats()
	if !formats.any() {
		return nil
	}

	var patterns []decorators.Pattern
	for _, kind := range Kinds {
		if !formats.enabled(kind) {
			continue
		}
		for _, scan := range scansByKind[kind] {
			patterns = append(patterns, decorators.Pattern{
				Regexp:       scan,
				ReplaceGroup: 1,
				Boundary:     decorators.BoundaryOK,
			})
		}
	}

	return patterns
}

func (d *Decorator) Parse(value string, _ time.Time) (url.Values, bool) {
	kind, canonical, ok := Recognize(value)
	if !ok {
		return nil, false
	}
	if !d.formats().enabled(kind) {
		return nil, false
	}

	return url.Values{
		ParamKind:  {string(kind)},
		ParamValue: {canonical},
	}, true
}

func (d *Decorator) RenderPage(w http.ResponseWriter, params url.Values) {
	kind := Kind(params.Get(ParamKind))
	value := params.Get(ParamValue)

	if !RecognizeAs(kind, value) {
		decorators.WriteError(w, http.StatusBadRequest,
			errcode.WithCode(errcode.CyberPageInvalid, "That is not an indicator this plugin issued."))
		return
	}

	details := Describe(kind, value, d.datasets())

	w.Header().Set("Cache-Control", "private, max-age=60")

	decorators.WritePage(w, decorators.Page{
		Title:    details.Title,
		Theme:    decorators.ThemeFromParams(params),
		StyleCSS: pageStyles,
		BodyHTML: renderBody(details),
	})
}
