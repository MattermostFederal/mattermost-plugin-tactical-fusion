package decorators

import (
	"net/http"
	"net/url"
	"regexp"
	"testing"
	"time"
)

// Merging matters independently of the bail-out gate: dropping a range because
// it overlapped an earlier one meant a construct could lose protection entirely
// and have a link written into its interior.
func TestMergeRanges(t *testing.T) {
	cases := []struct {
		name  string
		input []byteRange
		want  []byteRange
	}{
		{"empty", nil, nil},
		{"single", []byteRange{{0, 5}}, []byteRange{{0, 5}}},
		{"disjoint stay separate", []byteRange{{0, 5}, {10, 15}}, []byteRange{{0, 5}, {10, 15}}},
		{"overlapping merge", []byteRange{{0, 10}, {5, 15}}, []byteRange{{0, 15}}},
		{"nested collapses", []byteRange{{0, 20}, {5, 10}}, []byteRange{{0, 20}}},
		{"adjacent merge", []byteRange{{0, 5}, {5, 10}}, []byteRange{{0, 10}}},
		{"unsorted input", []byteRange{{10, 20}, {0, 5}, {15, 30}, {5, 8}}, []byteRange{{0, 8}, {10, 30}}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := mergeRanges(tc.input)

			if len(got) != len(tc.want) {
				t.Fatalf("mergeRanges() = %v, want %v", got, tc.want)
			}
			for i := range tc.want {
				if got[i] != tc.want[i] {
					t.Fatalf("mergeRanges() = %v, want %v", got, tc.want)
				}
			}
		})
	}
}

// A blank line is "indented" by any prefix test, but it does not open a code
// block. Treating one as code would protect the rest of the message from the
// blank line onwards and silently stop decorating everything after it.
func TestIsIndentedCode(t *testing.T) {
	cases := []struct {
		name string
		line string
		want bool
	}{
		{"tab indent", "\tcode", true},
		{"four spaces", "    code", true},
		{"three spaces is prose", "   text", false},
		{"unindented", "text", false},
		{"empty line", "", false},
		{"tab only", "\t", false},
		{"spaces only", "        ", false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := isIndentedCode(tc.line); got != tc.want {
				t.Fatalf("isIndentedCode(%q) = %v, want %v", tc.line, got, tc.want)
			}
		})
	}
}

// A closing run must be exactly as wide as the opener. A run of a different
// width is interior text and the scan has to continue past it, which is the
// whole reason this is hand-scanned rather than a regexp: RE2 has no
// backreference to say "a run matching the opener".
func TestCodeSpanRangesMatchesRunWidth(t *testing.T) {
	cases := []struct {
		name    string
		message string
		want    []byteRange
	}{
		{"single backticks", "a `code` b", []byteRange{{2, 8}}},
		{"narrower run inside a wider span", "``a`b`` c", []byteRange{{0, 7}}},
		{"wider run does not close a narrower opener", "`a``b` c", []byteRange{{0, 6}}},
		{"unterminated opener is literal text", "a `code b", nil},
		{"unterminated wider opener", "``a`b c", nil},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := codeSpanRanges(tc.message)

			if len(got) != len(tc.want) {
				t.Fatalf("codeSpanRanges(%q) = %v, want %v", tc.message, got, tc.want)
			}
			for i := range tc.want {
				if got[i] != tc.want[i] {
					t.Fatalf("codeSpanRanges(%q) = %v, want %v", tc.message, got, tc.want)
				}
			}
		})
	}
}

// nilRegexpDecorator offers one unusable pattern followed by one that works.
// A decorator that builds its patterns from configuration can end up with a
// zero Pattern, and the tagger runs on the post path: a nil dereference here
// would stop somebody from posting.
type nilRegexpDecorator struct{}

func (*nilRegexpDecorator) Type() string { return "nilre" }

func (*nilRegexpDecorator) Patterns() []Pattern {
	return []Pattern{
		{},
		{Regexp: regexp.MustCompile(`\bTOKEN\b`)},
	}
}

func (*nilRegexpDecorator) Parse(value string, _ time.Time) (url.Values, bool) {
	return url.Values{"v": {value}}, true
}

func (*nilRegexpDecorator) RenderPage(w http.ResponseWriter, _ url.Values) {
	WritePage(w, Page{Title: "nilre"})
}

func TestFindCandidatesSkipsAPatternWithNoRegexp(t *testing.T) {
	registry := NewRegistry()
	if err := registry.Register(&nilRegexpDecorator{}); err != nil {
		t.Fatalf("Register() = %v, want nil", err)
	}

	tagger := &Tagger{Registry: registry, URLPrefix: "/plugins/test/decorate"}

	got := tagger.findCandidates("say TOKEN now", time.Now().UTC(), nil)

	if len(got) != 1 {
		t.Fatalf("findCandidates() returned %d candidates, want 1", len(got))
	}
	if got[0].label != "TOKEN" {
		t.Fatalf("candidate label = %q, want %q", got[0].label, "TOKEN")
	}
	// The usable pattern is the second one, so its index proves the nil was
	// skipped rather than counted as a match.
	if got[0].patternIdx != 1 {
		t.Fatalf("candidate patternIdx = %d, want 1", got[0].patternIdx)
	}
}

// Equal-length matches from the same decorator fall through to the pattern
// index, which is what makes "patterns are tried in registration order" true
// for a tie rather than merely for the search.
func TestResolveOverlapsBreaksTiesOnPatternIndex(t *testing.T) {
	later := candidate{match: byteRange{0, 5}, replace: byteRange{0, 5}, decoratorIdx: 0, patternIdx: 3, typ: "a", label: "later"}
	earlier := candidate{match: byteRange{0, 5}, replace: byteRange{0, 5}, decoratorIdx: 0, patternIdx: 1, typ: "a", label: "earlier"}

	got := resolveOverlaps([]candidate{later, earlier})

	if len(got) != 1 {
		t.Fatalf("resolveOverlaps() returned %d candidates, want 1", len(got))
	}
	if got[0].label != "earlier" {
		t.Fatalf("resolveOverlaps() kept %q, want %q", got[0].label, "earlier")
	}
}

// monikerDecorator mirrors the shape the location decorator has: a bare pattern
// and a labeled one whose ReplaceGroup leaves the label in the message. That
// difference is the whole reason soleTokenResult measures match rather than
// replace.
type monikerDecorator struct{}

func (*monikerDecorator) Type() string { return "mock" }

func (*monikerDecorator) Patterns() []Pattern {
	return []Pattern{
		{Regexp: regexp.MustCompile(`GRID:[ \t]*(\d{4})`), ReplaceGroup: 1},
		{Regexp: regexp.MustCompile(`\b(\d{4})\b`)},
	}
}

func (*monikerDecorator) Parse(value string, _ time.Time) (url.Values, bool) {
	return url.Values{"v": {value}}, true
}

func (*monikerDecorator) RenderPage(w http.ResponseWriter, _ url.Values) {
	WritePage(w, Page{Title: "mock"})
}

func monikerTagger(t *testing.T) *Tagger {
	t.Helper()

	registry := NewRegistry()
	if err := registry.Register(&monikerDecorator{}); err != nil {
		t.Fatalf("Register() = %v, want nil", err)
	}
	return &Tagger{Registry: registry, URLPrefix: "/plugins/test/decorate"}
}

func TestSoleTokenResultRecognizesAMessageThatIsOneToken(t *testing.T) {
	for _, tc := range []struct {
		name    string
		message string
		want    bool
	}{
		{"a bare token and nothing else", "1234", true},

		// The label is not consumed, so match covers it and replace does not.
		// Measuring replace would refuse the ordinary military spelling.
		{"a label the decorator did not consume", "GRID: 1234", true},

		{"surrounding whitespace", "\n  1234  \n", true},
		{"a token in a sentence", "see 1234 now", false},
		{"a labeled token in a sentence", "see GRID: 1234 now", false},
		{"trailing prose after a label", "GRID: 1234 confirmed", false},
		{"two tokens", "1234 5678", false},
		{"no token at all", "nothing here", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, got := monikerTagger(t).DecorateWithResult(tc.message, time.Now().UTC())

			if got.SoleToken != tc.want {
				t.Fatalf("SoleToken = %v for %q, want %v", got.SoleToken, tc.message, tc.want)
			}
			if !tc.want {
				return
			}
			if got.Type != "mock" {
				t.Fatalf("Type = %q, want %q", got.Type, "mock")
			}
			if got.Params.Get("v") != "1234" {
				t.Fatalf("Params[v] = %q, want %q", got.Params.Get("v"), "1234")
			}
		})
	}
}

// A token inside a protected span is not decorated, so it cannot be the message
// either, however alone it looks.
func TestSoleTokenResultRefusesAProtectedToken(t *testing.T) {
	_, got := monikerTagger(t).DecorateWithResult("`1234`", time.Now().UTC())

	if got.SoleToken {
		t.Fatal("a token inside a code span was reported as the whole message")
	}
}

func TestDecorateReturnsWhatDecorateWithResultReturns(t *testing.T) {
	tagger := monikerTagger(t)
	ref := time.Now().UTC()

	for _, message := range []string{"", "1234", "GRID: 1234", "see 1234 now", "nothing here"} {
		withResult, _ := tagger.DecorateWithResult(message, ref)
		if plain := tagger.Decorate(message, ref); plain != withResult {
			t.Fatalf("Decorate(%q) = %q, DecorateWithResult = %q", message, plain, withResult)
		}
	}
}
