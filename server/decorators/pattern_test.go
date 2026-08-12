package decorators_test

import (
	"regexp"
	"testing"
	"unicode"

	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/decorators"
)

// rejectAdjacentDigits is the shape every location grammar's guard has: a token
// must not sit inside a longer numeric run.
func rejectAdjacentDigits(before, after rune) bool {
	bad := func(r rune) bool {
		return r != 0 && (unicode.IsDigit(r) || r == '.' || r == '-')
	}
	return !bad(before) && !bad(after)
}

func fixtureWithPattern(typ string, p decorators.Pattern) *fixtureDecorator {
	return &fixtureDecorator{typ: typ, patterns: []decorators.Pattern{p}}
}

// The case the consumed-guard design got wrong.
//
// Two tokens separated by a single space must BOTH be decorated. A pattern that
// consumed its own guards could not do this: FindAllStringSubmatchIndex returns
// non-overlapping matches, so the first token would eat the space the second one
// needed as its leading guard and the second would silently go undecorated.
func TestBoundaryAllowsTwoTokensSharingOneSeparator(t *testing.T) {
	tagger := taggerWith(t, testPrefix, fixtureWithPattern("tok", decorators.Pattern{
		Regexp:   regexp.MustCompile(`\d+\.\d+`),
		Boundary: rejectAdjacentDigits,
	}))

	got := tagger.Decorate("1.5 2.5", testRef)
	want := "[1.5](" + testPrefix + "/tok?v=1.5) [2.5](" + testPrefix + "/tok?v=2.5)"

	if got != want {
		t.Fatalf("Decorate()\n got: %s\nwant: %s", got, want)
	}
}

func TestBoundaryRejectsMatchInsideALongerRun(t *testing.T) {
	tagger := taggerWith(t, testPrefix, fixtureWithPattern("tok", decorators.Pattern{
		Regexp:   regexp.MustCompile(`\d\.\d`),
		Boundary: rejectAdjacentDigits,
	}))

	cases := []string{
		"11.5",  // digit before
		"1.55",  // digit after
		"1.5.2", // dot after, i.e. the middle of a range
		"9-1.5", // hyphen before
		"1.5-9", // hyphen after
	}

	for _, input := range cases {
		if got := tagger.Decorate(input, testRef); got != input {
			t.Errorf("Decorate(%q) = %q, want it left alone", input, got)
		}
	}
}

// A token at either edge of the message has no rune on that side, which the
// framework reports as 0 rather than as whatever byte happens to be there.
func TestBoundarySeesZeroAtMessageEdges(t *testing.T) {
	tagger := taggerWith(t, testPrefix, fixtureWithPattern("tok", decorators.Pattern{
		Regexp:   regexp.MustCompile(`\d+\.\d+`),
		Boundary: rejectAdjacentDigits,
	}))

	for _, input := range []string{"1.5", "1.5 tail", "head 1.5"} {
		if got := tagger.Decorate(input, testRef); got == input {
			t.Errorf("Decorate(%q) left the token alone, want it decorated", input)
		}
	}
}

// The rune before a match is decoded, not indexed by byte, so a multi-byte
// character is reported as itself rather than as a continuation byte that would
// pass every check by accident.
func TestBoundaryDecodesMultiByteNeighbors(t *testing.T) {
	type neighbors struct{ before, after rune }

	seen := make(chan neighbors, 4)
	tagger := taggerWith(t, testPrefix, fixtureWithPattern("tok", decorators.Pattern{
		Regexp: regexp.MustCompile(`\d+\.\d+`),
		Boundary: func(before, after rune) bool {
			seen <- neighbors{before, after}
			return true
		},
	}))

	tagger.Decorate("°1.5°", testRef)
	close(seen)

	got := <-seen
	if got.before != '°' || got.after != '°' {
		t.Fatalf("Boundary saw (%q, %q), want (%q, %q)", got.before, got.after, '°', '°')
	}
}

// A nil Boundary is the zero value and must impose nothing, which is what keeps
// every existing decorator working unchanged.
func TestNilBoundaryImposesNoConstraint(t *testing.T) {
	tagger := taggerWith(t, testPrefix, newFixture("tok", `\d\.\d`))

	if got := tagger.Decorate("11.55", testRef); got == "11.55" {
		t.Fatal("Decorate() left the token alone, want a nil Boundary to allow it")
	}
}

// ReplaceGroup links only part of what it matched, leaving the rest of the
// author's text in place. This is what a location moniker needs and what a DTG
// moniker deliberately does not use.
func TestReplaceGroupRewritesOnlyTheNamedSubmatch(t *testing.T) {
	tagger := taggerWith(t, testPrefix, fixtureWithPattern("tok", decorators.Pattern{
		Regexp:       regexp.MustCompile(`LATM:(\d+\.\d+)`),
		ReplaceGroup: 1,
	}))

	got := tagger.Decorate("LATM:1.5 confirmed", testRef)
	want := "LATM:[1.5](" + testPrefix + "/tok?v=1.5) confirmed"

	if got != want {
		t.Fatalf("Decorate()\n got: %s\nwant: %s", got, want)
	}
}

// The default consumes the whole match, which is what the DTG moniker relies on.
func TestReplaceGroupZeroConsumesTheWholeMatch(t *testing.T) {
	tagger := taggerWith(t, testPrefix, fixtureWithPattern("tok", decorators.Pattern{
		Regexp: regexp.MustCompile(`LATM:(\d+\.\d+)`),
	}))

	got := tagger.Decorate("LATM:1.5 confirmed", testRef)
	want := "[1.5](" + testPrefix + "/tok?v=1.5) confirmed"

	if got != want {
		t.Fatalf("Decorate()\n got: %s\nwant: %s", got, want)
	}
}

// Overlap resolution ranks on the match but claims the replace span, so a
// labeled pattern still beats a bare one for the same token.
func TestLabeledPatternOutranksBarePatternForTheSameToken(t *testing.T) {
	d := &fixtureDecorator{
		typ: "tok",
		patterns: []decorators.Pattern{
			{Regexp: regexp.MustCompile(`LATM:(\d+\.\d+)`), ReplaceGroup: 1},
			{Regexp: regexp.MustCompile(`\d+\.\d+`)},
		},
	}
	tagger := taggerWith(t, testPrefix, d)

	got := tagger.Decorate("LATM:1.5", testRef)
	want := "LATM:[1.5](" + testPrefix + "/tok?v=1.5)"

	if got != want {
		t.Fatalf("Decorate()\n got: %s\nwant: %s", got, want)
	}
}

// The whole match is what protected ranges are tested against, so a moniker
// inside a code span protects the token behind it even though only the token
// would have been rewritten.
func TestReplaceGroupStillProtectsOnTheWholeMatch(t *testing.T) {
	tagger := taggerWith(t, testPrefix, fixtureWithPattern("tok", decorators.Pattern{
		Regexp:       regexp.MustCompile(`LATM:(\d+\.\d+)`),
		ReplaceGroup: 1,
	}))

	const input = "the field is `LATM:1.5` in the header"
	if got := tagger.Decorate(input, testRef); got != input {
		t.Fatalf("Decorate() = %q, want the code span left alone", got)
	}
}

// A ReplaceGroup naming a group that did not participate is a coding error in
// the pattern. Drop the candidate rather than panicking on the post path.
func TestReplaceGroupOutOfRangeDropsTheCandidate(t *testing.T) {
	tagger := taggerWith(t, testPrefix, fixtureWithPattern("tok", decorators.Pattern{
		Regexp:       regexp.MustCompile(`(\d+)\.\d+`),
		ReplaceGroup: 7,
	}))

	const input = "1.5"
	if got := tagger.Decorate(input, testRef); got != input {
		t.Fatalf("Decorate() = %q, want the input unchanged", got)
	}
}
