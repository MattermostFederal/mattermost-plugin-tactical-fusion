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
