package decorators_test

import (
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/decorators"
)

const testPrefix = "/plugins/com.mattermost.plugin-tactical-fusion/decorate"

var testRef = time.Date(2026, time.August, 9, 12, 0, 0, 0, time.UTC)

func taggerWith(t *testing.T, prefix string, ds ...decorators.Decorator) *decorators.Tagger {
	t.Helper()
	r := decorators.NewRegistry()
	for _, d := range ds {
		mustRegister(t, r, d)
	}
	return &decorators.Tagger{Registry: r, URLPrefix: prefix}
}

func TestDecorateReplacesMatch(t *testing.T) {
	tagger := taggerWith(t, testPrefix, newFixture("tok", `\bAAA\b`))

	got := tagger.Decorate("before AAA after", testRef)
	want := "before [AAA](" + testPrefix + "/tok?v=AAA) after"

	if got != want {
		t.Fatalf("Decorate()\n got: %s\nwant: %s", got, want)
	}
}

func TestDecorateReturnsInputWhenNothingMatches(t *testing.T) {
	tagger := taggerWith(t, testPrefix, newFixture("tok", `\bAAA\b`))

	const input = "nothing to see here"
	if got := tagger.Decorate(input, testRef); got != input {
		t.Fatalf("Decorate() = %q, want the input unchanged", got)
	}
}

// Regression tests for confirmed message-corruption bugs.
//
// Every case below was verified to rewrite stored user text at some point.
// Protection is per span, not per message, so a token inside one of these is
// left alone while the rest of the message is still decorated.
func TestProtectedSpansAreNeverRewritten(t *testing.T) {
	tagger := taggerWith(t, testPrefix, newFixture("tok", `\bAAA\b`))

	cases := []struct {
		name  string
		input string
	}{
		// Injected a nested markdown link inside the label.
		{"link whose label contains a code span", "[see `now` AAA](https://example.com)"},
		{"link label", "[AAA](https://example.com)"},
		{"link destination", "[plan](https://example.com/AAA)"},
		{"image", "![AAA](https://example.com/x.png)"},
		{"reference-style link", "[AAA][ref]"},
		{"shortcut reference link", "[AAA]"},
		{"link reference definition", "[ref]: https://example.com/AAA"},

		// Mattermost autolinks bare URLs, so this destroyed the reader's link.
		{"bare url", "see https://example.com/logs/AAA now"},
		{"bare url in a query string", "https://example.com/a?t=AAA"},
		{"www url", "see www.example.com/AAA now"},
		{"angle autolink", "<https://example.com/AAA>"},

		// Text the author explicitly marked as literal.
		{"closed fence", "```\nAAA\n```"},
		{"fence with a language", "```go\nAAA\n```"},
		{"unterminated fence", "```\nlaunch at AAA"},
		{"tilde fence", "~~~\nAAA\n~~~"},
		{"unterminated tilde fence", "~~~\nAAA"},
		{"indented code block", "    AAA"},
		{"tab indented code block", "\tAAA"},
		{"inline code", "text `AAA` text"},
		{"double backtick span", "``AAA``"},
		{"triple backtick span on one line", "```AAA```"},

		// Attributes can carry anything.
		{"html tag attribute", `<span title="AAA">x</span>`},

		// Balanced brackets are legal in link text. The label expression used to
		// refuse a raw "[", so a link with one was not recognised as a link at
		// all and a token in its destination was rewritten, nesting a new link
		// inside the reader's.
		{"link label containing brackets", "[a [b] AAA](https://example.com)"},
		{"token in the destination of a bracketed label", "[a [b] c](https://example.com/AAA)"},
		{"doubly nested label", "[a [b [c]] AAA](https://example.com)"},
		{"image with a bracketed label", "![a [b] AAA](https://example.com/x.png)"},

		// Balanced parentheses are legal in a destination, and a title may
		// contain them freely. Stopping at the first ")" left the rest open.
		{"balanced parentheses in the destination", "[x](/a(foo)AAA)"},
		{"parentheses in the title", `[x](/a "(note) AAA")`},

		// A closing fence takes no info string, so this one is still open.
		{"fence closer with trailing text", "```\ncode\n```js\nAAA\nstill inside\n```"},

		// A closer must be at least as long as its opener.
		{"four-backtick fence closed by three", "````\ncode\n```\nAAA"},
		{"four-tilde fence closed by three", "~~~~\ncode\n~~~\nAAA"},

		// Balanced matching recognises nothing at all when the delimiters do
		// not balance, so an unpaired "(" inside a title lost the link its
		// protection entirely. The simple destination form is kept as a
		// fallback for exactly this.
		{"unbalanced parenthesis in the title", `[x](/u "a (b AAA")`},
		{"unbalanced parenthesis in the destination", `[x](/u(a AAA)`},

		// A real link label may wrap. Bounding it to one line would leave the
		// destination behind it open, which is corruption rather than a missed
		// decoration, so this one deliberately does cross a newline.
		{"link label wrapped over two lines", "[see\nbelow AAA](https://example.com)"},
		{"token in the destination of a wrapped label", "[see\nbelow](https://example.com/AAA)"},

		// A backslash before the closing bracket is an escape to the balanced
		// expression, so the span never terminates and it matches nothing. The
		// simple form never looked for escapes and still covers it.
		{"bracketed span ending in a backslash", `[backup AAA in C:\logs\] done`},

		// The interior of a CRLF fence is still protected.
		{"inside a crlf fence", "```\r\nAAA\r\n```\r\n"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tagger.Decorate(tc.input, testRef); got != tc.input {
				t.Fatalf("Decorate() rewrote a protected span\n got: %q\nwant: %q", got, tc.input)
			}
		})
	}
}

// The point of protecting spans rather than whole messages: a token in ordinary
// prose is decorated even when the message also contains a link or some code.
func TestTokensOutsideProtectedSpansAreDecorated(t *testing.T) {
	tagger := taggerWith(t, testPrefix, newFixture("tok", `\bAAA\b`))

	cases := []struct {
		name  string
		input string
		want  string
	}{
		{
			"alongside a link",
			"[the plan](https://example.com) says AAA",
			"[the plan](https://example.com) says [AAA](" + testPrefix + "/tok?v=AAA)",
		},
		{
			"alongside a bare url",
			"see https://example.com then AAA",
			"see https://example.com then [AAA](" + testPrefix + "/tok?v=AAA)",
		},
		{
			"alongside inline code",
			"run `now` at AAA",
			"run `now` at [AAA](" + testPrefix + "/tok?v=AAA)",
		},
		{
			"after a fenced block",
			"```\ncode\n```\nlaunch AAA",
			"```\ncode\n```\nlaunch [AAA](" + testPrefix + "/tok?v=AAA)",
		},
		{
			// A closer may be followed by whitespace, tabs included. Reading
			// one as content left the block open over the rest of the message.
			"after a fenced block closed with a tab",
			"```\ncode\n```\t\nlaunch AAA",
			"```\ncode\n```\t\nlaunch [AAA](" + testPrefix + "/tok?v=AAA)",
		},
		{
			// A client sending CRLF leaves a carriage return on every line,
			// including the closer, so every message from one lost decoration
			// after its first code block.
			"after a fenced block with crlf line endings",
			"```\r\ncode\r\n```\r\nlaunch AAA",
			"```\r\ncode\r\n```\r\nlaunch [AAA](" + testPrefix + "/tok?v=AAA)",
		},
		{
			// A standalone bracket is bounded to its line. Letting it span the
			// message meant one stray "[" reached forward to any later "]" and
			// silently suppressed decoration for everything in between.
			"inside a bracket span opened on an earlier line",
			"status [see below\n\nlaunch AAA] confirmed",
			"status [see below\n\nlaunch [AAA](" + testPrefix + "/tok?v=AAA)] confirmed",
		},
		{
			"inside a bracket span opened on the previous line",
			"status [see below\nlaunch AAA] confirmed",
			"status [see below\nlaunch [AAA](" + testPrefix + "/tok?v=AAA)] confirmed",
		},
		{
			"in a block quote",
			"> launch AAA",
			"> launch [AAA](" + testPrefix + "/tok?v=AAA)",
		},
		{
			"between html tags",
			"<b>AAA</b>",
			"<b>[AAA](" + testPrefix + "/tok?v=AAA)</b>",
		},
		{
			"one protected, one not",
			"`AAA` and AAA",
			"`AAA` and [AAA](" + testPrefix + "/tok?v=AAA)",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tagger.Decorate(tc.input, testRef); got != tc.want {
				t.Fatalf("Decorate()\n got: %s\nwant: %s", got, tc.want)
			}
		})
	}
}

func TestOrdinaryMessagesAreDecorated(t *testing.T) {
	tagger := taggerWith(t, testPrefix, newFixture("tok", `\bAAA\b`))

	cases := []string{
		"ARCT AAA confirmed",
		"AAA",
		"launch AAA, recovery AAA",
		"mission update: AAA (confirmed)",
		"AAA -- primary window",
		"héllo AAA wörld",
	}

	for _, input := range cases {
		t.Run(input, func(t *testing.T) {
			if got := tagger.Decorate(input, testRef); got == input {
				t.Fatalf("Decorate(%q) left an ordinary message undecorated", input)
			}
		})
	}
}

func TestDecorateSkipsProtectedRanges(t *testing.T) {
	tagger := taggerWith(t, testPrefix, newFixture("tok", `\bAAA\b`))

	cases := []struct {
		name  string
		input string
	}{
		{"fenced code block", "```\nAAA\n```"},
		{"fenced block with language", "```go\nAAA\n```"},
		{"inline code", "text `AAA` text"},
		{"existing markdown link", "text [AAA](https://example.com) text"},
		{"our own link", "text [AAA](" + testPrefix + "/tok?v=AAA) text"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tagger.Decorate(tc.input, testRef); got != tc.input {
				t.Fatalf("Decorate() = %q, want the input unchanged", got)
			}
		})
	}
}

// Idempotence matters because Mattermost runs message hooks across plugins in
// an undefined order, and because a post can pass through more than once.
func TestDecorateIsIdempotent(t *testing.T) {
	tagger := taggerWith(t, testPrefix, newFixture("tok", `\bAAA\b`))

	once := tagger.Decorate("start AAA end", testRef)
	twice := tagger.Decorate(once, testRef)

	if once != twice {
		t.Fatalf("Decorate() is not idempotent\nonce:  %s\ntwice: %s", once, twice)
	}
}

// A rejected match must not claim its range, otherwise a longer candidate that
// fails validation would suppress a shorter valid one at the same span.
func TestRejectedMatchDoesNotClaimRange(t *testing.T) {
	long := &fixtureDecorator{
		typ:      "long",
		patterns: []decorators.Pattern{{Regexp: regexp.MustCompile(`\bAAABBB\b`)}},
		reject:   "AAABBB",
	}
	short := newFixture("short", `AAA`)

	tagger := taggerWith(t, testPrefix, long, short)

	got := tagger.Decorate("x AAABBB y", testRef)
	want := "x [AAA](" + testPrefix + "/short?v=AAA)BBB y"

	if got != want {
		t.Fatalf("Decorate()\n got: %s\nwant: %s", got, want)
	}
}

// Longest match wins regardless of which decorator registered first, so adding
// a decorator cannot silently change an existing one's behaviour.
func TestLongestMatchWinsOverRegistrationOrder(t *testing.T) {
	short := newFixture("short", `AAA`)
	long := newFixture("long", `AAABBB`)

	// short is registered first and would win on registration order alone.
	tagger := taggerWith(t, testPrefix, short, long)

	got := tagger.Decorate("x AAABBB y", testRef)
	want := "x [AAABBB](" + testPrefix + "/long?v=AAABBB) y"

	if got != want {
		t.Fatalf("Decorate()\n got: %s\nwant: %s", got, want)
	}
}

func TestRegistrationOrderBreaksEqualLengthTies(t *testing.T) {
	first := newFixture("first", `AAA`)
	second := newFixture("second", `AAA`)

	tagger := taggerWith(t, testPrefix, first, second)

	if got := tagger.Decorate("x AAA y", testRef); !strings.Contains(got, "/first?") {
		t.Fatalf("Decorate() = %q, want the first-registered decorator to win the tie", got)
	}
}

func TestLabelIsEscaped(t *testing.T) {
	// A label carrying markdown syntax must not be re-parsed as markdown.
	// Brackets are not exercised here because a bracketed span is protected
	// rather than decorated, so the token never reaches the escaper.
	// TestProtectedSpansAreNeverRewritten covers that instead.
	tagger := taggerWith(t, testPrefix, newFixture("tok", `A\*B_C~D`))

	got := tagger.Decorate(`x A*B_C~D y`, testRef)

	if !strings.Contains(got, `[A\*B\_C\~D](`) {
		t.Fatalf("Decorate() = %q, want the label escaped", got)
	}
}

// url.Values.Encode leaves parentheses alone, and an unbalanced ")" ends a
// markdown link destination early.
func TestParenthesesInURLAreEscaped(t *testing.T) {
	tagger := taggerWith(t, testPrefix, newFixture("tok", `A\(B\)C`))

	got := tagger.Decorate("x A(B)C y", testRef)

	// Parentheses are harmless in the label, so only the destination matters.
	_, destination, found := strings.Cut(got, "](")
	if !found {
		t.Fatalf("Decorate() = %q, want a markdown link", got)
	}
	destination, _, _ = strings.Cut(destination, ")")

	if strings.ContainsAny(destination, "()") {
		t.Fatalf("destination = %q, want parentheses percent-encoded", destination)
	}
	if !strings.Contains(destination, "%28B%29") {
		t.Fatalf("destination = %q, want %%28B%%29", destination)
	}
}

func TestURLIsRootRelative(t *testing.T) {
	cases := []struct {
		name   string
		prefix string
		want   string
	}{
		{"root install", testPrefix, "[AAA](" + testPrefix + "/tok?v=AAA)"},
		{
			"subpath install",
			"/mattermost/plugins/com.mattermost.plugin-tactical-fusion/decorate",
			"[AAA](/mattermost/plugins/com.mattermost.plugin-tactical-fusion/decorate/tok?v=AAA)",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tagger := taggerWith(t, tc.prefix, newFixture("tok", `\bAAA\b`))
			got := tagger.Decorate("x AAA y", testRef)

			if !strings.Contains(got, tc.want) {
				t.Fatalf("Decorate() = %q, want it to contain %q", got, tc.want)
			}
			// The URL is stored permanently, so a scheme or host would break
			// every historical post the day the server moves.
			for _, forbidden := range []string{"http://", "https://", "://"} {
				if strings.Contains(got, forbidden) {
					t.Fatalf("Decorate() = %q, want no %q in a stored URL", got, forbidden)
				}
			}
		})
	}
}

func TestMultipleMatchesAreAllReplaced(t *testing.T) {
	tagger := taggerWith(t, testPrefix, newFixture("tok", `\bAAA\b`))

	got := tagger.Decorate("AAA and AAA and AAA", testRef)

	if n := strings.Count(got, "/tok?v=AAA"); n != 3 {
		t.Fatalf("Decorate() produced %d links, want 3: %s", n, got)
	}
}

// Right-to-left replacement keeps earlier byte offsets valid; a left-to-right
// implementation corrupts everything after the first replacement.
func TestMultibyteTextSurvivesReplacement(t *testing.T) {
	tagger := taggerWith(t, testPrefix, newFixture("tok", `\bAAA\b`))

	got := tagger.Decorate("héllo AAA wörld AAA ✈", testRef)

	if !strings.HasPrefix(got, "héllo [AAA]") {
		t.Fatalf("Decorate() = %q, want the leading multibyte text intact", got)
	}
	if !strings.HasSuffix(got, " ✈") {
		t.Fatalf("Decorate() = %q, want the trailing multibyte text intact", got)
	}
}

func TestEmptyMessageIsUntouched(t *testing.T) {
	tagger := taggerWith(t, testPrefix, newFixture("tok", `\bAAA\b`))

	if got := tagger.Decorate("", testRef); got != "" {
		t.Fatalf("Decorate(\"\") = %q, want empty", got)
	}
}
