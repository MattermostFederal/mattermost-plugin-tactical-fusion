package location

import (
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/decorators"
	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/errcode"
)

// Type is the URL path segment and the key shared with the webapp decorator.
const Type = "location"

const PostType = decorators.PostTypePrefix + "tf_location"

// pageTitle is the browser tab name and the page heading. A category rather
// than the value, matching the sidebar header and the DTG page.
const pageTitle = "Location"

// The query parameters. The pair (f, v) is the identity of a location; r is
// display only and derives nothing.
const (
	paramFormat = "f"
	paramValue  = "v"
	paramRaw    = "r"
)

// maxRawBytes caps the "r" parameter.
//
// The longest legal token carrying symbols and spaces is well inside this. The
// cap is here so the parameter cannot become somewhere to put things.
const maxRawBytes = 64

// Formats selects which token grammars are matched.
//
// This governs decoration only. RenderPage never consults it, because a link
// already written into a message must keep working after an admin turns its
// format off: switching one off stops new messages being decorated, it does not
// break the history. Maps below is the deliberate exception and says why.
type Formats struct {
	// DDSigned is 34.0561, -118.2500.
	DDSigned bool

	// LatLon is the hemisphere-bearing grammars: decimal degrees with letters,
	// degrees-minutes-seconds, and degrees-decimal-minutes.
	LatLon bool

	// USMTF is the fixed-width compact family: LATD, LATM, LATS, LATDS, DMPID,
	// GEOK and the verified variants.
	USMTF bool

	// MGRS is the military grid reference.
	//
	// Separate from the plain lat/lon grammars because MGRS and UTM are the
	// only ones whose position is computed rather than read, so a workspace
	// that does not want a hand-written projection deciding where a message
	// points can remove both in two switches.
	MGRS bool

	// UTM is the zone, band, easting and northing form.
	//
	// Separate from MGRS, and the only grammar in this package that ships OFF,
	// because it is the only one whose token is ambiguous rather than merely
	// hard to detect. The letter after the zone is read as a latitude band, so
	// "11S" is band S; a civilian writing the same characters means "zone 11,
	// southern hemisphere" and a point 90 degrees of latitude away. The band
	// containment check declines about nine in ten of those, which is a good
	// deal better than nothing and is still not none.
	//
	// Every other switch here trades a false positive against a missed
	// decoration. This one trades a false positive against a decoration that is
	// confidently WRONG, which is a different kind of cost, so the default is
	// the safe reading and turning it on is a decision an admin makes knowing
	// what it buys. MGRS has no such problem: its band letter is followed by
	// two more letters and cannot be read as a hemisphere.
	UTM bool

	// GEOREF is the World Geographic Reference System, and GARS the Global Area
	// Reference System.
	//
	// Two switches rather than one for the same reason MGRS and UTM are split:
	// they are separate notations an install may want separately, and a reader
	// who never sees one written should not have to accept the other's false
	// positives to decline it. Both are label-only, so both also need Moniker,
	// which is the posture LATD already has.
	GEOREF bool
	GARS   bool

	// PlusCode is an Open Location Code.
	//
	// The only grammar added with the area-reference systems that is detected
	// unlabeled, and the only one that could be: its alphabet is twenty
	// characters with no vowel among them, and the separator sits at a fixed
	// position in the middle, which is a shape ordinary text mostly does not
	// have. "Mostly" is measured rather than assumed, and the bare grammar is
	// upper case only because of what was measured; see olcBareExpr.
	PlusCode bool

	// Moniker is a USMTF field label in front of any of the others.
	Moniker bool
}

// AllFormats is what a decorator with no selector matches.
//
// Every grammar, UTM included. This is the test and library default, not the
// shipped one: plugin.json is where an install's posture is set, and there UTM
// defaults off.
var AllFormats = Formats{
	DDSigned: true, LatLon: true, USMTF: true, MGRS: true, UTM: true,
	GEOREF: true, GARS: true, PlusCode: true, Moniker: true,
}

// Maps selects which surfaces draw a coordinate on a map.
//
// The deliberate opposite of Formats above: RenderPage DOES consult this. A
// format switch governs text already written permanently into a message, so
// turning one off may not break the history behind it. A map is drawn live on
// every render and is written into nothing, so turning it off has to reach
// existing links too or the switch would not mean what it says.
type Maps struct {
	// Panel is the sidebar panel and the hover card.
	//
	// One flag for both because the card is the panel's own map component in
	// preview mode rather than a second implementation, and because the card is
	// the map and nothing else: with this off there is no card at all, not an
	// empty one.
	Panel bool

	// Inline is the map under a message whose whole text is one coordinate.
	//
	// The only one of the three with a cost beyond bytes on the wire. Drawing
	// there means stamping the post with a custom Type, which Elasticsearch and
	// OpenSearch index and then never match, so the post is absent from search
	// and from Recent Mentions. Off leaves the post ordinary, and the stamp is
	// skipped rather than merely unread.
	Inline bool

	// Page is the standalone full-window map at /map.
	//
	// Off makes that route answer 404 rather than rendering an empty shell, and
	// takes the "Open larger" link with it.
	Page bool
}

// AllMaps is what a decorator with no selector draws, and is the shipped
// posture: unlike UTM, every map switch defaults on.
var AllMaps = Maps{Panel: true, Inline: true, Page: true}

// Decorator recognizes coordinates and renders the conversion page.
type Decorator struct {
	// Enabled reports which formats to match, read fresh for every message so
	// an admin toggle takes effect without a restart. Nil means all of them.
	Enabled func() Formats

	// Maps reports which surfaces draw a map, read fresh for every render for
	// the same reason. Nil means all of them.
	Maps func() Maps

	// Packages reports the detail map packages this install has, by name, read
	// fresh for every render so one dropped into the directory appears without
	// a restart. Nil means none, which is a global-only install.
	Packages func() []string
}

// packages is the detail areas this install has, or none.
func (d *Decorator) packages() []string {
	if d.Packages == nil {
		return nil
	}

	return d.Packages()
}

// maps is the selector, or every surface when there is none.
func (d *Decorator) maps() Maps {
	if d.Maps == nil {
		return AllMaps
	}

	return d.Maps()
}

var _ decorators.Decorator = (*Decorator)(nil)

var _ decorators.PostRenderer = (*Decorator)(nil)

func (d *Decorator) Type() string { return Type }

// PostType is the custom type a coordinate-only post is stamped with, or "" when
// the admin has turned the inline map off.
//
// Answering "" rather than gating in the hook is what keeps the stamp and the
// reason for it in one place: StandalonePostType already reads "" as "this
// decorator renders no post body", so an unwanted stamp is never written rather
// than written and then ignored. That distinction is the whole value of the
// switch, since the stamp is what costs the post its Elasticsearch and
// OpenSearch matches, and Post.Type survives every edit once it is set.
func (d *Decorator) PostType() string {
	if !d.maps().Inline {
		return ""
	}

	return PostType
}

// monikerPrefixes are the USMTF field labels, taken from the standard rather
// than invented.
//
// LOC, DEPLOC, ARRLOC and ICAO are deliberately absent: in USMTF they introduce
// an ICAO airfield code, which is a facility whose position must be looked up
// rather than computed, and which this decorator does not handle at any phase.
// Claiming one of those labels for a coordinate would put this plugin in direct
// conflict with the standard.
//
// NAME, DEPNAME and ARRNAME introduce free text, and GRID is generic enough to
// vouch for whichever grammar happened to be switched on.
// MGRS, UTMO and UTMT are the grid labels. UTMO and UTMT are USMTF's own names
// for a grid reference and are the fields mattermost-plugin-aocanywhere reads
// them out of; MGRS is not a USMTF field label but is what everybody writes,
// and unlike LOC it collides with nothing in the standard.
//
// USNG is here and is not a format anywhere else in this package, deliberately.
// The United States National Grid is MGRS on WGS 84, character for character,
// so there is nothing for a separate grammar to do except accept MGRS's tokens
// and make the format id on the page a guess about which of two identical
// readings was meant. What "USNG:" needs is to be recognized as introducing a
// grid reference, which is exactly what a moniker is.
var monikerPrefixes = []string{
	"LATDS", "DEPLATM", "ARRLATM", "VLATD", "VLATM", "VLATS", "VGEOT",
	"DMPID", "GEOK", "LATD", "LATM", "LATS",
	"DEPUTMO", "ARRUTMO", "UTMO", "UTMT", "MGRS", "USNG",
	"GEOREF", "GARS", "PLUSCODE", "OLC",
}

// monikerExpr is the label alternation, longest first so a prefix that is the
// head of another cannot win the shorter reading.
var monikerExpr = func() string {
	sorted := append([]string(nil), monikerPrefixes...)
	// Longest first. The list above is already ordered that way; sorting here
	// means a later addition cannot quietly break it.
	for i := range sorted {
		for j := i + 1; j < len(sorted); j++ {
			if len(sorted[j]) > len(sorted[i]) {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}
	return `(?i:` + strings.Join(sorted, `|`) + `)`
}()

// bareFormats are the grammars safe to detect without a label.
//
// LATD is absent on purpose: "35N079W" is seven characters, two of them
// letters, and it resolves to a 111 km square. It is available behind a USMTF
// label only, which is the one place this decorator follows the sibling
// plugin's tag-gated posture exactly rather than diverging from it.
//
// The grid grammars are present but their patterns are narrower here than under
// a label: bareScanExpr gives back only the spaced shapes. That distinction is
// per-expression rather than per-format, which is why it lives in grammar.go
// and not in this list.
func bareFormats(f Formats) []Format {
	var out []Format
	if f.DDSigned {
		out = append(out, FormatDD)
	}
	if f.LatLon {
		out = append(out, FormatDDH, FormatDMS, FormatDDM)
	}
	if f.USMTF {
		out = append(out, FormatLATM, FormatVLATM)
	}
	if f.MGRS {
		out = append(out, FormatMGRS)
	}
	if f.UTM {
		out = append(out, FormatUTM)
	}
	if f.PlusCode {
		out = append(out, FormatPlusCode)
	}
	return out
}

// labeledFormats are the grammars a label may introduce, which is every bare
// one plus the three that are reachable no other way.
func labeledFormats(f Formats) []Format {
	out := bareFormats(f)
	if f.USMTF {
		out = append(out, FormatLATD)
	}
	if f.GEOREF {
		out = append(out, FormatGEOREF)
	}
	if f.GARS {
		out = append(out, FormatGARS)
	}
	return out
}

// patternCache memoises the compiled set per enabled-format combination.
//
// Formats must stay comparable, because it is the key: giving it a slice or map
// field makes Load panic, on the post path, for every message. The recover in
// decoratePost would absorb it, so posting would survive while decoration died
// silently across the whole workspace.
//
// Patterns runs for every decorator on every message, so building these each
// time would be pure waste. Caching on the Formats value rather than
// enumerating every combination up front keeps the cost proportional to what an
// admin actually uses.
var patternCache sync.Map // Formats -> []decorators.Pattern

// Patterns returns the enabled patterns. The tagger resolves overlaps by match
// length, so their order here is only a tiebreak.
func (d *Decorator) Patterns() []decorators.Pattern {
	formats := AllFormats
	if d.Enabled != nil {
		formats = d.Enabled()
	}

	if cached, ok := patternCache.Load(formats); ok {
		if patterns, ok := cached.([]decorators.Pattern); ok {
			return patterns
		}
	}

	// Capped to its own length, so a caller that appends writes into a fresh
	// array rather than into the one every other caller is ranging over. Every
	// other decorator allocates a new slice per call, so "the slice is yours"
	// was true everywhere before this cache existed and stays true now.
	patterns := buildPatterns(formats)
	patterns = patterns[:len(patterns):len(patterns)]

	patternCache.Store(formats, patterns)
	return patterns
}

func buildPatterns(formats Formats) []decorators.Pattern {
	var patterns []decorators.Pattern

	// Labeled first. A labeled token matches more text than the bare one
	// inside it, so it wins on length anyway; putting it first makes that
	// explicit rather than incidental.
	//
	// The label IS consumed, the way the DTG moniker's is: the whole match is
	// rewritten, so "LATM:3510N07901W" becomes a link reading "3510N07901W".
	//
	// It was kept for a long time, on the argument that "LATM:" is part of a
	// structured line an author may be quoting verbatim. The cost of consuming
	// it is exactly that, is real, and is accepted: a labeled coordinate loses
	// its field label from the STORED message, permanently, because this hook
	// rewrites what was written rather than how it is displayed.
	//
	// What softens it is that a genuine USMTF set line ends "//", and the
	// trailing side of monikerBoundaryOK declines those already, so the lines
	// most likely to be quoted verbatim never decorated in the first place.
	//
	// No ReplaceGroup, so the whole match is the span. Value still returns
	// submatch 1, which is the token, so the link reads the token and Parse is
	// handed the token exactly as before.
	if formats.Moniker {
		for _, f := range labeledFormats(formats) {
			// Spaces and tabs around the colon, never \s.
			//
			// \s in RE2 includes \n, \f and \r, and with `*` on both sides a
			// label at the end of one line would reach across a line break, or
			// across a blank line, and claim whatever token started the next
			// one. "MGRS:\n58cbe40" decorated a lower-case git SHA, which is
			// exactly the collision mgrsBareCompactExpr exists to avoid, walked
			// in through the label instead. This is the same rule the token
			// sub-expressions follow with sp, and the same separator dtg.go
			// already uses.
			patterns = append(patterns, decorators.Pattern{
				Regexp:   regexp.MustCompile(monikerExpr + `[ \t]*:[ \t]*(` + scanExpr(f) + `)`),
				Boundary: monikerBoundaryOK,
			})
		}
	}

	for _, f := range bareFormats(formats) {
		patterns = append(patterns, decorators.Pattern{
			Regexp:   regexp.MustCompile(bareScanExpr(f)),
			Boundary: boundaryOK,
		})
	}

	return patterns
}

// monikerBoundaryOK guards a labeled match.
//
// The same guard as the bare case, deliberately. It used to be laxer on the
// leading side, refusing only a letter or a digit, and that let a label be
// matched inside something the author had written:
// "logs/MGRS:18SUJ2347806483" was rewritten in place while the bare token in
// the identical position was correctly declined, because badNeighbor rejects
// "/" and the moniker guard did not. Rewriting the middle of a path is the
// failure this whole file is arranged around, and a missed decoration is a
// feature gap, so the guards are now the same one.
//
// The cost is real and worth naming: a USMTF line quoted with slash delimiters
// no longer decorates. That turns out to cost almost nothing in practice,
// because a genuine line ends "//" and was already declined by the trailing
// side of this same guard.
func monikerBoundaryOK(before, after rune) bool {
	return boundaryOK(before, after)
}

// Parse validates a matched token and returns the params encoding it.
//
// The format is derived here, by trying the enabled grammars, because the
// framework hands this method the matched text and nothing else. That is safe
// in this direction: the text came out of a message and the grammars are
// mutually exclusive, which a test pins. It is emphatically not safe in the
// other direction, which is why validateParams takes the format from the URL
// and never re-derives it.
func (d *Decorator) Parse(value string, _ time.Time) (url.Values, bool) {
	formats := AllFormats
	if d.Enabled != nil {
		formats = d.Enabled()
	}

	for _, f := range labeledFormats(formats) {
		loc, ok := Parse(f, value)
		if !ok {
			continue
		}

		canonical := loc.Canonical()
		params := url.Values{
			paramFormat: {string(f)},
			paramValue:  {canonical},
		}

		// Omitted when it would only repeat the canonical form, which is the
		// common case for the USMTF compact family: the author's text already
		// is the canonical form. An absent r reads as "the author wrote v".
		if value != canonical {
			// Decline rather than emit a link the page would then reject.
			//
			// This is the one invariant that matters most here: a token
			// accepted at decoration and rejected at render is rewritten
			// permanently into somebody's message, and its own page answers 400
			// forever, with hand-editing the post the only way back. Asking
			// validateRaw the question now, with the same function the page
			// will ask later, makes that structurally impossible rather than
			// something each new grammar has to remember.
			//
			// A grammar mismatch reached exactly this way once already: the
			// separators admitted \s, so a coordinate split across a line break
			// was decorated and its page then refused the newline in r.
			params.Set(paramRaw, value)
		}

		// The whole payload, through the very function the page will use.
		// Checking only the pieces would have missed the second instance of
		// this: a pair whose canonical form collapsed onto Null Island, which
		// the parser then declined.
		if _, ok := validateParams(params); !ok {
			return nil, false
		}

		return params, true
	}

	return nil, false
}

// RenderPage renders the conversion table for these params.
//
// Every reading on it is a pure function of the query string: no store, no post,
// no channel, and it is handed no reader, so the answer cannot vary between two
// people holding the same link. The one thing it does consult is Maps, which is
// admin configuration rather than per-reader state and decides only whether a
// picture is drawn beside those readings.
func (d *Decorator) RenderPage(w http.ResponseWriter, params url.Values) {
	page, ok := validateParams(params)
	if !ok {
		decorators.WriteError(w, http.StatusBadRequest, errcode.WithCode(errcode.LocationPageParamsInvalid,
			"That link is missing or has an invalid coordinate."))
		return
	}

	// A coordinate is a pure function of its own token, so unlike the DTG page
	// this one really is cacheable. It is still private: the URL carries a
	// position, and a shared cache holding one is a leak rather than an
	// optimisation.
	w.Header().Set("Cache-Control", "private, max-age=300")

	decorators.WritePage(w, decorators.Page{
		Title:      pageTitle,
		BodyHTML:   renderRoot(page, pageModeLocation, d.maps(), d.packages()),
		Theme:      decorators.ThemeFromParams(params),
		StyleCSS:   pageStyles,
		ScriptSrc:  pageAppFromDecorate,
		Capability: decorators.PageMapping,
	})
}

// pageData is one validated payload.
type pageData struct {
	loc Location

	// raw is the author's own text, or the canonical form when the link carried
	// no r. Every rendering path treats the two the same, so an absent r is not
	// a missing value.
	raw string
}

// validateParams re-derives the whole payload from the query string and rejects
// anything self-inconsistent.
//
// Layered exactly as dtg.validateParams is, and for the same reason: on a public
// route where the URL is user-supplied, validating each parameter in isolation
// would let a crafted link pair an arbitrary token with an unrelated format and
// render both side by side as though they agreed.
func validateParams(params url.Values) (pageData, bool) {
	// 1. The format id is a closed enum. Never "try each grammar until one
	// parses" here, or f is decorative and the notation named on the page is
	// independently spoofable.
	f := Format(params.Get(paramFormat))
	if !KnownFormat(f) {
		return pageData{}, false
	}

	// 2. The token must match that format's grammar before anything else
	// touches it, and it is length-capped first.
	//
	// The cap is not defending against a pathological pattern, since RE2 is
	// linear. It is here because this route is public and unauthenticated, and
	// running any expression over a megabyte of query string is work somebody
	// else chose for us. The longest legal canonical form is about 33 bytes.
	v := params.Get(paramValue)
	if len(v) > maxRawBytes || !MatchesTokenGrammar(f, v) {
		return pageData{}, false
	}

	// 3. It must parse, and it must reproduce itself exactly. This is what
	// rejects an alias such as a token carrying redundant trailing zeroes.
	loc, ok := Parse(f, v)
	if !ok || loc.Canonical() != v {
		return pageData{}, false
	}

	// 4. The author's text, if the link carries any.
	raw, ok := validateRaw(params.Get(paramRaw), f, v)
	if !ok {
		return pageData{}, false
	}

	return pageData{loc: loc, raw: raw}, true
}

// validateRaw checks the one parameter that is text an author typed.
//
// r is echoed onto a public page whose query string an attacker controls
// freely, which is the surface the round trip exists to close. It is answered by
// never treating r as free text: by construction it is a string one of this
// package's own token sub-expressions matched, so it can be whitelisted as
// tightly as the canonical form is.
//
// A failure rejects the whole link rather than just this row. A link carrying an
// r this plugin would never have written is not one this plugin wrote.
func validateRaw(raw string, f Format, canonical string) (string, bool) {
	if raw == "" {
		// Absent, which reads as "the author wrote the canonical form".
		return canonical, true
	}

	// Gate 1: length.
	if len(raw) > maxRawBytes {
		return "", false
	}

	// Gate 2: alphabet. A whitelist, never a blacklist.
	if !allowedRawRunes(raw) {
		return "", false
	}

	// Gate 3: grammar. Anchoring the same sub-expression the scanner used is
	// what makes content spoofing structurally impossible rather than merely
	// escaped, so prose wrapped around a coordinate can never reach the page.
	if !MatchesTokenGrammar(f, raw) {
		return "", false
	}

	// Gate 4: it must name the same place. r may differ from the canonical form
	// only in the ways normalization is allowed to erase: case, separators,
	// symbols and spacing.
	loc, ok := Parse(f, raw)
	if !ok || loc.Canonical() != canonical {
		return "", false
	}

	return raw, true
}

// allowedRawRunes is the whitelist for the r parameter.
//
// Digits, letters, the separators the grammars use, and the four typographic
// variants the symbol classes accept. No "<", no "&", no control characters, no
// other non-ASCII.
//
// The letters are the whole Latin alphabet rather than only NSEW, and that
// widening arrived with the grid grammars: a grid reference carries a band
// letter and two 100 km square letters drawn from most of the alphabet, so a
// hemisphere-only whitelist rejected every one of them. It is a real loss of
// tightness at this gate and it is the right trade, because this gate was never
// the one doing the work. Gate 3 requires r to be an anchored match for the
// token grammar and gate 4 requires it to normalize to the same canonical form,
// so what reaches a page is a coordinate or nothing, whatever this list admits.
//
// TestGrammarAlphabetMatchesRawAlphabet is what keeps this and the grammars
// describing the same set. When they last disagreed, a coordinate split across
// a line break was decorated and its own page then refused the newline, leaving
// a dead link in somebody's message.
func allowedRawRunes(s string) bool {
	for _, r := range s {
		switch {
		case r >= '0' && r <= '9':
		case r >= 'A' && r <= 'Z':
		case r >= 'a' && r <= 'z':
		case strings.ContainsRune(".,-+'\" ", r):
		case r == '°' || r == 'º': // ° º
		case r == '′' || r == '’' || r == '´': // ′ ’ ´
		case r == '″' || r == '”': // ″ ”
		default:
			return false
		}
	}
	return true
}
