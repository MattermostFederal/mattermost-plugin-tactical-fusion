package decorators_test

import (
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
// link is labelled with.
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
