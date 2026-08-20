package decorators_test

import (
	"net/url"
	"testing"

	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/decorators"
)

// Pattern.Value is what lets a pattern match more than it links: the moniker
// pattern matches "DTG: 091630ZAUG26" and links only the token. The default is
// submatch 1, falling back to the whole match for a pattern with no group.
//
// Extract is the escape hatch for a value that is not any single group, such as
// one assembled from two of them. No shipped pattern uses it yet, so nothing but
// this test proves it is wired up at all.
func TestPatternValue(t *testing.T) {
	joinFirstTwo := func(m []string) string { return m[1] + m[2] }

	cases := []struct {
		name    string
		pattern decorators.Pattern
		match   []string
		want    string
	}{
		{
			name:    "no capture group falls back to the whole match",
			pattern: decorators.Pattern{},
			match:   []string{"091630ZAUG26"},
			want:    "091630ZAUG26",
		},
		{
			name:    "first capture group wins over the whole match",
			pattern: decorators.Pattern{},
			match:   []string{"DTG: 091630ZAUG26", "091630ZAUG26"},
			want:    "091630ZAUG26",
		},
		{
			name:    "Extract wins over the first capture group",
			pattern: decorators.Pattern{Extract: joinFirstTwo},
			match:   []string{"whole", "091630Z", "AUG26"},
			want:    "091630ZAUG26",
		},
		{
			name:    "Extract wins over the whole-match fallback",
			pattern: decorators.Pattern{Extract: func(m []string) string { return "fixed" }},
			match:   []string{"whole"},
			want:    "fixed",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.pattern.Value(tc.match); got != tc.want {
				t.Fatalf("Value(%v) = %q, want %q", tc.match, got, tc.want)
			}
		})
	}
}

// A pattern using Extract has to survive the whole tagger path, not just the
// Value call: the extracted string is both what Parse is handed and what the
// link is labeled with.
func TestExtractReachesParseAndTheLabel(t *testing.T) {
	d := newFixture("extract", `\bAT (\d{4})(Z)\b`)
	d.patterns[0].Extract = func(m []string) string { return m[1] + m[2] }

	tagger := taggerWith(t, testPrefix, d)

	got := tagger.Decorate("meet AT 1630Z sharp", testRef)

	// "AT " is consumed: the pattern matched it but Extract did not include it,
	// so it is neither in the label nor in the params.
	want := "meet [1630Z](" + testPrefix + "/extract?v=1630Z) sharp"
	if got != want {
		t.Fatalf("Decorate()\n got: %s\nwant: %s", got, want)
	}
}

type postTypeDecorator struct {
	decorators.Decorator
	postType string
}

func (d *postTypeDecorator) PostType() string { return d.postType }

// Posts.Type is VARCHAR(26) and Post.IsValid checks the prefix but never the
// length, so an over-long type is not a bad render, it is a database error at
// save time: the author cannot post at all. Refusing it here costs the inline
// rendering and nothing else, which is the direction every other guard in the
// post path already leans.
func TestStandalonePostTypeRefusesATypeThatCannotBeStored(t *testing.T) {
	for _, tc := range []struct {
		name     string
		postType string
		want     string
	}{
		{"a legal custom type", "custom_tf_location", "custom_tf_location"},
		{"exactly the column width", "custom_" + "abcdefghijklmnopqrs", "custom_abcdefghijklmnopqrs"},
		{"one over the column width", "custom_" + "abcdefghijklmnopqrst", ""},
		{"missing the custom prefix", "tf_location", ""},
		{"empty", "", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if len(tc.want) > decorators.PostTypeMaxLen {
				t.Fatalf("the test case itself is over the limit: %q", tc.want)
			}

			got := decorators.StandalonePostType(&postTypeDecorator{postType: tc.postType})
			if got != tc.want {
				t.Fatalf("StandalonePostType(%q) = %q, want %q", tc.postType, got, tc.want)
			}
		})
	}
}

// A decorator that declares nothing gets no type, which is what keeps a lone
// DTG post an ordinary post.
func TestStandalonePostTypeIsEmptyForADecoratorThatDeclaresNone(t *testing.T) {
	if got := decorators.StandalonePostType(&plainDecorator{}); got != "" {
		t.Fatalf("StandalonePostType() = %q, want %q", got, "")
	}
}

type plainDecorator struct {
	decorators.Decorator
}

// A decorator that does not implement MessageExpander is left alone, which is
// what keeps adding one from reaching the others.
func TestStandaloneExpansionIsEmptyForADecoratorThatDeclaresNone(t *testing.T) {
	if got := decorators.StandaloneExpansion(&plainDecorator{}, "/href", "", url.Values{}); got != "" {
		t.Errorf("StandaloneExpansion() = %q, want empty", got)
	}
}

// And a nil decorator, which is what Registry.Get answers for a type it does
// not hold. A type assertion on a nil interface is false rather than a panic,
// and this is on the path that must never stop somebody posting.
func TestStandaloneExpansionIsEmptyForANilDecorator(t *testing.T) {
	if got := decorators.StandaloneExpansion(nil, "/href", "", url.Values{}); got != "" {
		t.Errorf("StandaloneExpansion(nil) = %q, want empty", got)
	}
}

// The positive path, so the dispatch itself is pinned rather than only its
// refusals.
func TestStandaloneExpansionReachesADecoratorThatDeclaresOne(t *testing.T) {
	got := decorators.StandaloneExpansion(&expanderDecorator{}, "/href", "//", url.Values{"v": {"KIND"}})

	if want := "/href|//|KIND"; got != want {
		t.Errorf("StandaloneExpansion() = %q, want %q", got, want)
	}
}

type expanderDecorator struct {
	decorators.Decorator
}

func (e *expanderDecorator) ExpandMessage(href, trail string, params url.Values) string {
	return href + "|" + trail + "|" + params.Get("v")
}
