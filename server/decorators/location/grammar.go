package location

import (
	"regexp"
	"strings"
	"unicode"
)

// The token sub-expressions.
//
// Every scanning pattern and every anchored validator is built from the strings
// here, so a change to what a token looks like cannot reach one and miss the
// other. That shared origin is also what makes the "r" parameter checkable: r
// is the span a scanning pattern replaced, and it is validated by anchoring the
// same sub-expression it came from.
//
// The symbol classes carry the typographic variants a phone keyboard produces.
// Rejecting a coordinate because somebody's software turned an apostrophe into
// a right single quote would be a support ticket, not a safety property. They
// appear here rather than in a normalization pass so that there is one
// expression per grammar rather than two that can drift.
const (
	degSym = `[\x{00B0}\x{00BA}]`          // ° masculine ordinal
	minSym = `['\x{2032}\x{2019}\x{00B4}]` // ' ′ ’ ´
	secSym = `["\x{2033}\x{201D}]`         // " ″ ”

	ns = `[NnSs]`
	ew = `[EeWw]`

	// sp is the only whitespace legal INSIDE a token: a literal space, never
	// \s.
	//
	// \s in RE2 is [\t\n\f\r ], which would let a coordinate span a line
	// break. Three things go wrong if it does. The token is decorated, but its
	// own page rejects it, because the "r" parameter's alphabet admits a space
	// and nothing else, so the link this plugin just wrote into somebody's
	// message answers 400 forever. The markdown label ends up containing a
	// newline, and a label containing a blank line is not a link at all under
	// CommonMark, so the reader sees raw markup. And a coordinate split across
	// two lines is not one token in the first place.
	//
	// The alphabet here and the one in allowedRawRunes have to stay the same
	// set. TestGrammarAlphabetMatchesRawAlphabet is what enforces that.
	sp = `[ ]`

	// A field separator is the symbol, optionally spaced, or plain spacing.
	// Requiring one is what keeps the spaced grammars from collapsing into the
	// fixed-width compact ones, which have their own expressions.
	degSep = `(?:` + sp + `*` + degSym + sp + `*|` + sp + `+)`
	minSep = `(?:` + sp + `*` + minSym + sp + `*|` + sp + `+)`

	// The seconds symbol is optional because the hemisphere letter that follows
	// is already an unambiguous terminator.
	secEnd = sp + `*` + secSym + `?` + sp + `*`

	// An optional comma between the two halves, which people write and just as
	// often do not.
	half = sp + `*,?` + sp + `*`
)

// Per-grammar token expressions.
//
// Latitude degrees are one or two digits and longitude three, or one to three
// where the grammar is not fixed width; the ranges are checked in parse rather
// than in the pattern, because a pattern that encoded them would be unreadable
// and would still need the check for minutes and seconds.
const (
	// Signed decimal degrees. Four fractional digits are required on both
	// halves: that is about 11 m, which is what real coordinate sources emit,
	// and version pairs, ratios and tolerances essentially never carry it.
	// The comma is required; a space between two decimals is not evidence of
	// anything.
	ddExpr = `[-+]?\d{1,2}\.\d{4,}` + sp + `*,` + sp + `*[-+]?\d{1,3}\.\d{4,}`

	// Decimal degrees with hemisphere letters, which may lead or trail. A
	// decimal point is required on both halves even though the letters make the
	// intent explicit, because N, S, E and W are also newton, siemens, east and
	// watt: "Load 12 N, 5 W" must not become a coordinate.
	ddhSuffixExpr = `\d{1,2}\.\d+` + sp + `*` + degSym + `?` + sp + `*` + ns + half + `\d{1,3}\.\d+` + sp + `*` + degSym + `?` + sp + `*` + ew
	ddhPrefixExpr = ns + sp + `*\d{1,2}\.\d+` + sp + `*` + degSym + `?` + half + ew + sp + `*\d{1,3}\.\d+` + sp + `*` + degSym + `?`

	// Degrees, minutes and seconds. The compact form is USMTF LATS, and with a
	// fractional second it is LATDS and DMPID.
	dmsCompactExpr = `\d{6}(?:\.\d+)?` + ns + half + `\d{7}(?:\.\d+)?` + ew
	dmsSepExpr     = `\d{1,2}` + degSep + `\d{1,2}` + minSep + `\d{1,2}(?:\.\d+)?` + secEnd + ns +
		half + `\d{1,3}` + degSep + `\d{1,2}` + minSep + `\d{1,2}(?:\.\d+)?` + secEnd + ew

	// Degrees and decimal minutes, which USMTF calls GEOK. No seconds
	// component, which is the only thing separating it from DMS.
	ddmCompactExpr = `\d{4}\.\d+` + ns + half + `\d{5}\.\d+` + ew
	ddmSepExpr     = `\d{1,2}` + degSep + `\d{1,2}\.\d+` + sp + `*` + minSym + `?` + sp + `*` + ns +
		half + `\d{1,3}` + degSep + `\d{1,2}\.\d+` + sp + `*` + minSym + `?` + sp + `*` + ew

	// The USMTF fixed-width family.
	latdExpr  = `\d{2}` + ns + `\d{3}` + ew
	latmExpr  = `\d{4}` + ns + `\d{5}` + ew
	vlatmExpr = `\d{4}` + ns + `\d-\d{5}` + ew + `\d`
)

// The grid grammars.
//
// Neither of these can be read without projecting it, so unlike every grammar
// above they are validated by geometry as well as by shape: the letters have to
// name a square that exists in that zone, and the square has to lie in the band
// the token claims. That check is worth far more here than a tighter pattern
// would be, because only eight of the twenty-four possible 100 km column
// letters are legal in any given zone.
const gridZone = `\d{1,2}`

// The letter classes, written once in upper case and lowered mechanically.
//
// Two hand-maintained classes that had to describe the same letters in two
// cases would be a place for them to drift, and the bare grammar below depends
// on the upper-case one being exactly the upper half of the any-case one.
const (
	// Every band letter. I and O are absent from the sequence because they read
	// as 1 and 0.
	bandBody = `C-HJ-NP-X`

	colBody = `A-HJ-NP-Z`
	rowBody = `A-HJ-NP-V`
)

// anyCase turns a character class body into one matching both cases.
func anyCase(body string) string {
	return `[` + body + strings.ToLower(body) + `]`
}

var (
	bandLetter = anyCase(bandBody)
	colLetter  = anyCase(colBody)
	rowLetter  = anyCase(rowBody)

	// The digits split into two space-separated halves, which is how a person
	// writes a grid reference by hand.
	mgrsSpacedExpr = gridZone + bandLetter + sp + `*` + colLetter + rowLetter +
		sp + `+\d{1,5}` + sp + `+\d{1,5}`

	// The digits run together, in any case and at any resolution. This is what
	// a label may introduce and what an "r" parameter is validated against.
	mgrsCompactExpr = gridZone + bandLetter + sp + `*` + colLetter + rowLetter +
		sp + `*\d{2,10}`

	// The run-together form as it is detected WITHOUT a label, which is
	// narrower than the above in two ways. Both are measured rather than
	// assumed; see bareExprs.
	//
	// Upper case only, because the lower-case collision space is hexadecimal:
	// a short git SHA is seven to twelve lower-case hex characters, and one in
	// roughly 3,900 of them is a well-formed grid reference. "58cbe40" is zone
	// 58, band C, square BE, a 10 km square off the coast of Antarctica.
	//
	// At least three digits per axis, which is a 100 m square. Coarser
	// references are shorter, and a short run is both the weakest thing this
	// grammar can say and the easiest to hit by accident: at two digits per
	// axis "26HMA1997" and "3UTA7623" are well-formed grid references and are
	// also exactly the shape of a part number, measured at about one in 75,000
	// uppercase alphanumeric runs. At three the same sweep finds none in 1.2
	// million. This is the same argument that keeps LATD behind a label.
	//
	// Neither restriction puts anything out of reach: a lower-case or coarser
	// reference is still decorated when spaced or labeled.
	mgrsBareCompactExpr = gridZone + `[` + bandBody + `]` + sp + `*` +
		`[` + colBody + `][` + rowBody + `]` + sp + `*(?:\d{2}){3,5}`

	// UTM takes the whole band alphabet, N and S included.
	//
	// Those two are genuinely ambiguous and the ambiguity is not resolvable from
	// the token: "11S" is band S to a military reader and "zone 11, southern
	// hemisphere" to a civilian one. They are read as BANDS, because this is a
	// plugin for a military audience, MGRS is the military implementation of
	// UTM (USGS), and the third character of an MGRS grid zone designator is a
	// latitude band (NGA). Refusing them meant declining the ordinary military
	// spelling of a position in order to protect a civilian reading this
	// audience does not write.
	//
	// The cost is not symmetric between the two letters, which is worth knowing
	// before anybody widens or narrows this:
	//
	//   - N is nearly free. Hemisphere-north and band N both use the northing
	//     as written, so the two readings put the point in the SAME place; the
	//     band reading merely also requires it to fall in the first 8 degrees
	//     north. Reading N as a band can therefore decline a civilian token but
	//     can never misplace one.
	//   - S is where the risk is. Hemisphere-south measures from the false
	//     origin and band S does not, so the readings differ by up to 90 of
	//     latitude.
	//
	// What keeps S honest is that gridPoint validates band containment and zone
	// proximity, so a token only survives when the military reading is
	// geometrically consistent with itself. A civilian southern token is
	// misread only when its northing also happens to land inside the band its
	// letter names, which TestSouthernHemisphereTokensMostlyFail measures.
	//
	// The axis letters are optional and may carry a metre unit: "384640E" and
	// "384640mE". They must be adjacent to their digits. Allowing a space
	// before them would reach into the following word: in
	// "11S 384640 3769080 East" the token would swallow the E, and the boundary
	// guard would then see a letter and decline a token that decorates today.
	utmSpacedExpr = gridZone + bandLetter + sp + `*\d{6}` + utmAxisEast +
		sp + `+\d{7}` + utmAxisNorth
	utmCompactExpr = gridZone + bandLetter + `\d{13}`
)

// The optional axis letters on a UTM easting and northing. Non-capturing, so
// the parsers' numbered groups do not move.
const (
	utmAxisEast  = `(?:m?[Ee])?`
	utmAxisNorth = `(?:m?[Nn])?`
)

// tokenExprs lists every expression that can produce a given format, longest
// and most specific first. A format has more than one when the same reading can
// be written compactly or with separators.
var tokenExprs = map[Format][]string{
	FormatDD:    {ddExpr},
	FormatDDH:   {ddhSuffixExpr, ddhPrefixExpr},
	FormatDMS:   {dmsSepExpr, dmsCompactExpr},
	FormatDDM:   {ddmSepExpr, ddmCompactExpr},
	FormatLATD:  {latdExpr},
	FormatLATM:  {latmExpr},
	FormatVLATM: {vlatmExpr},
	FormatMGRS:  {mgrsSpacedExpr, mgrsCompactExpr},
	FormatUTM:   {utmSpacedExpr, utmCompactExpr},
}

// bareExprs is what a format matches with no label in front of it, for the
// formats where that is narrower than the full set. An absent entry means every
// shape is bare-detectable, which is the case for every grammar outside this
// file.
//
// A run-together grid reference, "18SUJ2347806483", is detected unlabeled.
// Both restrictions in mgrsBareCompactExpr were chosen against measurement, and
// each replaced an argument that turned out to be wrong:
//
//   - The stated worry was part numbers. The real collision was hexadecimal:
//     an any-case pattern decorated 51 of 200,000 generated short git SHAs,
//     about one in 3,900. Uppercase-only removes that class entirely.
//   - Uppercase-only was then claimed to leave nothing behind. It did not. At
//     two digits per axis, 13 of 40 seeds turned up a false positive in
//     uppercase alphanumeric runs, roughly one in 75,000, and they read exactly
//     like part numbers: "26HMA1997", "3UTA7623", "6CZA9867". Three digits per
//     axis finds none across 1.2 million.
//
// The lesson worth keeping is that both wrong answers sounded convincing and
// only the sweep settled them, which is why
// TestBareGridReferencesDoNotMatchOrdinaryRuns generates its corpus rather than
// listing cases somebody thought of.
//
// UTM stays spaced-only, and it is not an oversight that it was not given the
// same treatment. Its run-together form has no letters to check: after the zone
// and the band it is thirteen bare digits, so the only validation available is
// that the northing lands in the band, which a great many thirteen-digit runs
// do. There is nothing there to make narrower.
var bareExprs = map[Format][]string{
	FormatMGRS: {mgrsSpacedExpr, mgrsBareCompactExpr},
	FormatUTM:  {utmSpacedExpr},
}

// anchoredToken is the whole-string form of each format's expressions, which is
// what validates the "r" parameter.
//
// Anchoring the same sub-expression the scanner used is what makes content
// spoofing structurally impossible rather than merely escaped: a value carrying
// prose around a coordinate is not an anchored match for any grammar, so it can
// never reach a rendered page.
var anchoredToken = func() map[Format]*regexp.Regexp {
	m := make(map[Format]*regexp.Regexp, len(tokenExprs))
	for f, exprs := range tokenExprs {
		m[f] = regexp.MustCompile(`^(?:` + strings.Join(exprs, `|`) + `)$`)
	}
	return m
}()

// MatchesTokenGrammar reports whether s is exactly a token of this format.
func MatchesTokenGrammar(f Format, s string) bool {
	re, ok := anchoredToken[f]
	return ok && re.MatchString(s)
}

// scanExpr is every expression for a format, joined for use as a scanning
// pattern over whole message text.
//
// The alternation is ordered longest and most specific first, because Go's
// regexp is leftmost-first rather than leftmost-longest: a shorter alternative
// listed earlier would match the head of a longer token and stop there.
func scanExpr(f Format) string {
	return `(?:` + strings.Join(tokenExprs[f], `|`) + `)`
}

// bareScanExpr is the subset of a format's expressions safe to scan for without
// a label in front of them.
func bareScanExpr(f Format) string {
	exprs, ok := bareExprs[f]
	if !ok {
		return scanExpr(f)
	}
	return `(?:` + strings.Join(exprs, `|`) + `)`
}

// boundaryOK is the guard every location grammar uses.
//
// It is a function rather than part of the expression on purpose. A pattern
// that consumed its own guard characters would break the next match, because
// the regexp scan returns non-overlapping matches: the first token would eat
// the space the second one needed as its leading guard, and the second would go
// silently undecorated. Two grids on one line is the most ordinary input this
// feature has.
//
// Rejecting "." and "," on the trailing side costs a decoration when a
// coordinate ends a sentence with no following space. That is deliberate: at
// the point this runs, "-118.2500." and the middle of "-118.2500..-118.2600"
// are the same thing. A missed decoration is a feature gap and rewriting a
// range is corruption, so the trade goes this way.
func boundaryOK(before, after rune) bool {
	return !badNeighbor(before) && !badNeighbor(after)
}

func badNeighbor(r rune) bool {
	if r == 0 {
		// The start or end of the message, which is always a clean edge.
		return false
	}

	switch r {
	case '.', ',', '-', '+', '/', ':':
		return true
	}

	return unicode.IsDigit(r) || unicode.IsLetter(r)
}
