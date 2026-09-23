package note

import (
	"net/http"
	"net/url"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/decorators"
	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/errcode"
)

const Type = "note"

const ParamValue = "v"

const MaxNoteRunes = 1000

type Decorator struct{}

var _ decorators.Decorator = (*Decorator)(nil)

func (d *Decorator) Type() string { return Type }

func (d *Decorator) Patterns() []decorators.Pattern { return nil }

func (d *Decorator) Parse(value string, _ time.Time) (url.Values, bool) {
	params := url.Values{ParamValue: {value}}
	if _, ok := Validate(params); !ok {
		return nil, false
	}
	return params, true
}

func Validate(params url.Values) (string, bool) {
	markdown := params.Get(ParamValue)
	if strings.TrimSpace(markdown) == "" || !utf8.ValidString(markdown) {
		return "", false
	}
	if utf8.RuneCountInString(markdown) > MaxNoteRunes {
		return "", false
	}
	return markdown, true
}

func (d *Decorator) RenderPage(w http.ResponseWriter, params url.Values) {
	markdown, ok := Validate(params)
	if !ok {
		decorators.WriteError(w, http.StatusBadRequest,
			errcode.WithCode(errcode.NotePageInvalid, "That is not a note this plugin issued."))
		return
	}

	w.Header().Set("Cache-Control", "private, max-age=300")

	decorators.WritePage(w, decorators.Page{
		Title:    pageTitle,
		Theme:    decorators.ThemeFromParams(params),
		StyleCSS: pageStyles,
		BodyHTML: renderBody(markdown),
	})
}
