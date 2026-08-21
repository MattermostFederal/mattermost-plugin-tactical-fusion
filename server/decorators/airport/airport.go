package airport

import (
	"net/http"
	"net/url"
	"regexp"
	"time"

	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/decorators"
	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/errcode"
)

// Type is the URL path segment and the key shared with the webapp decorator.
const Type = "airport"

// ParamValue is the only query parameter. The identity of an airfield is its
// ident, and nothing derived from it ever travels in the URL, which is what
// makes a link impossible to disagree with itself.
const ParamValue = "v"

// Formats selects what this decorator matches.
//
// One flag, not two. The grammar is label-only by construction, so a separate
// moniker switch would turn the whole decorator off under a second name.
//
// It governs decoration only. RenderPage never consults it, because a link
// already written into a message must keep working after an admin turns the
// decorator off: switching it off stops new decoration, it does not break the
// history.
type Formats struct {
	Airfield bool

	// Table governs whether a message that is nothing but an airfield code is
	// expanded with the field's details, and nothing else.
	//
	// Separate from Airfield because it is a much larger rewrite of what the
	// author wrote: the table is written into the STORED message, so it lands
	// in exports, an author editing the post sees all of it, and the values are
	// frozen at the moment of posting rather than looked up on each view.
	Table bool
}

// AllFormats is what a decorator with no selector matches.
var AllFormats = Formats{Airfield: true, Table: true}

type Decorator struct {
	// Enabled is read fresh for every message so an admin toggle takes effect
	// without a restart. Nil means all of them.
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

// ExpandMessage adds the airfield's details under a message that is nothing but
// its code, as a markdown table.
//
// A table written into the message rather than a custom post type rendered by
// the webapp. That keeps the post an ordinary one: it stays in the
// Elasticsearch and OpenSearch indexes, keeps its link previews and its
// translation, and renders on every client rather than only the ones running
// the webapp bundle.
//
// The cost is that it is permanent and frozen. The table replaces the author's
// line rather than sitting under it, so the airfield's name carries the link
// and the author's own token survives in the Code row, terminator and all.
func (d *Decorator) ExpandMessage(href, trail string, params url.Values) string {
	if !d.formats().Table || href == "" {
		return ""
	}

	// Fields only. Describe also projects the coordinate and looks the country
	// up in the polygons, and the table reads none of that: this runs inline on
	// the post path, where Parse's contract is that the work is cheap.
	fields, ok := DescribeFields(params.Get(ParamValue))
	if !ok {
		return ""
	}

	return airfieldTable(href, trail, fields)
}

var scanPattern = regexp.MustCompile(scanExpr)

func (d *Decorator) Patterns() []decorators.Pattern {
	if !d.formats().Airfield {
		return nil
	}

	return []decorators.Pattern{{
		Regexp: scanPattern,

		// The label and the ident together, so the label is CONSUMED and
		// "ICAO:KIND" becomes a link reading "KIND". The same thing the DTG
		// moniker does, and the reason is the same: the label says what kind of
		// thing follows, and once the thing is a link that says so itself the
		// label is repeating it.
		//
		// The cost is real and is accepted rather than overlooked: a USMTF set
		// line quoted verbatim loses its field label, permanently, because this
		// rewrites the stored message. "DEPLOC:KIND//" becomes "[KIND](...)//".
		//
		// Group 1 is the label and the ident. The set terminator sits outside
		// it, so the "//" is still matched, still seen by the guard, and still
		// left in the message.
		ReplaceGroup: 1,

		// Group 2, the ident alone. Without this the link would be labeled with
		// group 1, which is the label and the ident together.
		Extract: func(m []string) string { return m[2] },

		Boundary: decorators.BoundaryOK,
	}}
}

// Parse validates by lookup rather than by shape.
//
// An ident this build does not hold declines, which is what keeps "LOC: HOME"
// and "ICAO: FAST" from becoming links: four letters is what prose is made of,
// so the shape alone vouches for nothing.
//
// Pure and one map read, which is what it has to be to run on the post path.
func (d *Decorator) Parse(value string, _ time.Time) (url.Values, bool) {
	if !MatchesIdentShape(value) {
		return nil, false
	}
	if _, ok := Lookup(value); !ok {
		return nil, false
	}

	return url.Values{ParamValue: {value}}, true
}

// RenderPage writes the standalone page.
//
// An ident this build does not hold renders at 200 with a note rather than
// refusing. The location decorator rejects a link that cannot reproduce itself
// because a crafted URL could otherwise pair a token with an unrelated instant;
// here the URL carries one value and derives everything from it, so a refusal
// buys nothing and costs something real: refresh the dataset, drop an ident,
// and every message that ever named it holds a link answering 400 forever, with
// hand-editing the only way back.
//
// A malformed ident is still a 400, and that shape check is what makes echoing
// the value below safe, so it runs before any rendering path.
func (d *Decorator) RenderPage(w http.ResponseWriter, params url.Values) {
	ident := params.Get(ParamValue)
	if !MatchesIdentShape(ident) {
		decorators.WriteError(w, http.StatusBadRequest,
			errcode.WithCode(errcode.AirportPageInvalid, "That is not an airfield code this plugin issued."))
		return
	}

	details, found := Describe(ident)

	w.Header().Set("Cache-Control", "private, max-age=300")

	decorators.WritePage(w, decorators.Page{
		Title:    pageTitle,
		Theme:    decorators.ThemeFromParams(params),
		StyleCSS: pageStyles,
		BodyHTML: renderBody(ident, details, found),
	})
}
