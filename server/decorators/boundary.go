package decorators

import "unicode"

// BoundaryOK is the guard a token that does not start and end with a word
// character needs, and is what Pattern.Boundary usually wants.
//
// It is a function rather than part of the expression on purpose. A pattern
// that consumed its own guard characters would break the next match, because
// the regexp scan returns non-overlapping matches: the first token would eat
// the space the second one needed as its leading guard, and the second would go
// silently undecorated. Two tokens on one line is the most ordinary input this
// has.
//
// Rejecting "." and "," on the trailing side costs a decoration when a token
// ends a sentence with no following space. That is deliberate: at the point
// this runs, "-118.2500." and the middle of "-118.2500..-118.2600" are the same
// thing. A missed decoration is a feature gap and rewriting a range is
// corruption, so the trade goes this way.
//
// A pattern needing to accept one of these characters must consume it in the
// expression rather than weaken the guard. The airfield grammar does exactly
// that for the "//" a USMTF set line ends with, because boundaryOK is handed
// the runes flanking the whole match while ReplaceGroup narrows only what is
// rewritten.
func BoundaryOK(before, after rune) bool {
	return !BadNeighbor(before) && !BadNeighbor(after)
}

// BadNeighbor reports whether a rune beside a match makes it unsafe to rewrite.
func BadNeighbor(r rune) bool {
	if r == 0 {
		// The start or end of the message, which is always a clean edge.
		return false
	}

	switch r {
	case '.', ',', '-', '+', '/', ':':
		return true

	case '_':
		// Underscore binds an identifier exactly as a letter or a digit does,
		// and those are rejected below for that reason.
		//
		// This was missing, and its absence made the guard strictly weaker than
		// the `\b` it exists to replace: Go's `\b` counts `_` as a word
		// character, which is why the DTG decorator, still using word
		// boundaries, never had the problem. So `snapshot_3510N07901W_v2` was
		// rewritten in place while `FOO_091630ZAUG26_BAR` was correctly left
		// alone, in the same message.
		return true
	}

	return unicode.IsDigit(r) || unicode.IsLetter(r)
}
