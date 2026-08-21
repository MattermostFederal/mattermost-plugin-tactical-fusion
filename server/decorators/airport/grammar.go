package airport

import (
	"regexp"
	"strings"
)

// monikerPrefixes are the four USMTF field labels that introduce an ICAO
// airfield code. They are exactly the four the location decorator reserves and
// refuses, for the reason it refuses them: the position behind one has to be
// looked up rather than computed.
var monikerPrefixes = []string{"DEPLOC", "ARRLOC", "ICAO", "LOC"}

const (
	// identBody is four letters, upper case only.
	//
	// Upper case rather than any case, where every location moniker is
	// (?i:...). A location token carries digits, so a lower-case label in
	// ordinary prose cannot reach one; an airfield ident is four letters, which
	// is what prose is made of, and "loc: fast turnaround on the ramp" would
	// otherwise link FAST (Somerset East Airport). USMTF field labels are upper
	// case, so this costs nothing real.
	//
	// It also makes the "v" parameter self-canonicalizing: there is no case to
	// fold, so a link can be spelled exactly one way.
	identBody = `[A-Z]{4}`

	// setTerminator is the "//" a USMTF set line ends with.
	//
	// Consumed by the pattern rather than permitted by the guard. Loosening
	// BadNeighbor to allow a trailing "/" would also accept "ICAO:KIND/foo",
	// which is path-shaped, and rewriting the middle of a path is the failure
	// the guard exists to stop. Pattern.boundaryOK is handed the runes flanking
	// the whole match while ReplaceGroup narrows only what is rewritten, so
	// matching the terminator lets the guard look past it without the "//"
	// becoming part of the link.
	setTerminator = `(?://)?`

	// separator is spaces and tabs, never \s, and nothing after the colon.
	//
	// RE2's \s includes \n, so with * before the colon a label ending one line
	// would reach across the break and claim whatever started the next one.
	// With 343 of these idents being English dictionary words, "Divert to LOC:"
	// followed by a line beginning "FAST" would rewrite FAST permanently into
	// stored post text.
	separator = `[ \t]*:`
)

var identShape = regexp.MustCompile(`^` + identBody + `$`)

// monikerExpr is the label alternation, longest first so a prefix that is the
// head of another cannot win the shorter reading. LOC is the head of nothing
// here, but DEPLOC and ARRLOC end in it, and ordering by length means a later
// addition cannot quietly break that.
var monikerExpr = `(?:` + strings.Join(byLengthDescending(monikerPrefixes), `|`) + `)`

func byLengthDescending(prefixes []string) []string {
	sorted := append([]string(nil), prefixes...)
	for i := range sorted {
		for j := i + 1; j < len(sorted); j++ {
			if len(sorted[j]) > len(sorted[i]) {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}
	return sorted
}

// scanExpr has TWO groups, and the split is what lets the label be consumed
// while the set terminator is not.
//
// Group 1 is the label and the ident together, which is the span rewritten into
// the link. Group 2 is the ident alone, which is what the link reads and what
// Parse is handed. The "//" sits outside both, so it stays in the message.
var scanExpr = `(` + monikerExpr + separator + `(` + identBody + `))` + setTerminator

// MatchesIdentShape reports whether a value is a well-formed ident.
//
// The shape check, not the lookup: it is what makes echoing the value on a
// public page safe, so it runs before any rendering path and before the
// lookup that decides whether the airfield is known.
func MatchesIdentShape(v string) bool { return identShape.MatchString(v) }

// IdentBodyExpr is the ident sub-expression, for the test that holds the
// webapp's copy of it to this one.
func IdentBodyExpr() string { return identBody }
