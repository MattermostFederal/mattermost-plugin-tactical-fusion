package location_test

import (
	"math/rand"
	"strings"
	"testing"
	"time"

	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/decorators"
	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/decorators/dtg"
	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/decorators/location"
)

const prefix = "/plugins/com.mattermost.plugin-tactical-fusion/decorate"

var ref = time.Date(2026, time.August, 11, 12, 0, 0, 0, time.UTC)

func tagger(t *testing.T, ds ...decorators.Decorator) *decorators.Tagger {
	t.Helper()

	registry, err := decorators.NewDefaultRegistry(ds...)
	if err != nil {
		t.Fatalf("NewDefaultRegistry() = %v", err)
	}
	return &decorators.Tagger{Registry: registry, URLPrefix: prefix}
}

func linkCount(s string) int { return strings.Count(s, "](") }

// skipCorpusSweepWhenShort drops the generated corpus sweeps under -short.
//
// They are the slowest thing in this repository by a wide margin, and
// make coverage-backend runs the suite under BOTH -race and coverage
// instrumentation, which together pushed this package past go test's ten
// minute default and broke that target outright.
//
// Skipping them costs no PRODUCT coverage: every path they exercise, Decorate,
// findCandidates, findProtectedRanges, the scanning patterns and the boundary
// guard, is covered many times over by the ordinary cases in this file. What
// they measure is a RATE, which needs hundreds of thousands of samples and is
// worth nothing at a smaller size, so they run in full under make test, which
// is what CI gates on, and are skipped only where the number they produce is
// not what is being asked for.
func skipCorpusSweepWhenShort(t *testing.T) {
	t.Helper()

	if testing.Short() {
		t.Skip("generated corpus sweep; runs in full under make test")
	}
}

// The case the consumed-guard design got wrong, now against the real tagger.
//
// Two coordinates separated by a single space must BOTH be decorated. This is
// the most ordinary input the feature has and the first thing to check when
// reviewing the boundary code.
func TestTwoCoordinatesOnOneLineAreBothDecorated(t *testing.T) {
	tg := tagger(t, &location.Decorator{})

	got := tg.Decorate("34.0561,-118.2500 34.0562,-118.2501", ref)
	if n := linkCount(got); n != 2 {
		t.Fatalf("decorated %d coordinates, want 2:\n%s", n, got)
	}
}

func TestCoordinatesInProseAreDecorated(t *testing.T) {
	tg := tagger(t, &location.Decorator{})

	cases := []string{
		"target at 34.0561, -118.2500 confirmed",
		"grid 3510N07901W",
		"DZ is 34°03'22\"N 118°15'00\"W",
		"posn 400948N1221400W over",
		"(34.0561, -118.2500)",
		"34.0561, -118.2500",
	}

	for _, input := range cases {
		if got := tg.Decorate(input, ref); got == input {
			t.Errorf("Decorate(%q) left it undecorated", input)
		}
	}
}

// The negative corpus, run through the real tagger rather than the parser, so
// the boundary guards are what is being tested.
func TestNeverDecorated(t *testing.T) {
	tg := tagger(t, &location.Decorator{})

	cases := []struct {
		name  string
		input string
	}{
		{"coarse decimals", "34.05, -118.25"},
		{"integers", "scores were 12, 34"},
		{"no comma", "34.0561 -118.2500"},
		{"newtons and watts", "Load 12 N, 5 W"},
		{"a numeric list", "0.1234, 0.5678, 0.9012"},
		{"inside a longer run on the left", "11834.0561, -118.2500"},
		{"a range", "-118.2500..-118.2600"},
		{"bare LATD", "35N079W"},
		{"a version pair", "v1.2345, 6.7890beta"},
		{"decimal comma", "34,0561, -118,2500"},
		{"epoch seconds", "1723385400.1234, 1723385400.5678"},
		{"aocanywhere accepts this and it is latitude 99.98", "9999N99999W"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tg.Decorate(tc.input, ref); got != tc.input {
				t.Fatalf("Decorate(%q) = %q, want it left alone", tc.input, got)
			}
		})
	}
}

// Protection is per span, so a token inside one of these is left exactly as
// written while the rest of the message is still decorated.
func TestProtectedSpansAreLeftAlone(t *testing.T) {
	tg := tagger(t, &location.Decorator{})

	cases := []string{
		"the format is `3510N07901W`",
		"```\n3510N07901W\n```",
		"[grid](https://example.com/3510N07901W)",
		"see https://example.com/g/3510N07901W for detail",
		"[3510N07901W](https://example.com/x)",
	}

	for _, input := range cases {
		if got := tg.Decorate(input, ref); got != input {
			t.Errorf("Decorate(%q) = %q, want it left alone", input, got)
		}
	}
}

// A USMTF label is matched but not consumed, so a decorated line still reads as
// USMTF and the author's structured record survives.
func TestUSMTFLabelSurvivesDecoration(t *testing.T) {
	tg := tagger(t, &location.Decorator{})

	got := tg.Decorate("LATM:2130N15730W", ref)

	if !strings.HasPrefix(got, "LATM:[2130N15730W](") {
		t.Fatalf("Decorate() = %q, want the label kept and only the token linked", got)
	}
}

// A label vouches for nothing: the token behind it still has to be one.
func TestLabelDoesNotRescueAnInvalidToken(t *testing.T) {
	tg := tagger(t, &location.Decorator{})

	const input = "LATM:9999N99999W"
	if got := tg.Decorate(input, ref); got != input {
		t.Fatalf("Decorate(%q) = %q, want it left alone", input, got)
	}
}

// LATD is reachable behind a label and nowhere else.
func TestLATDNeedsItsLabel(t *testing.T) {
	tg := tagger(t, &location.Decorator{})

	if got := tg.Decorate("35N079W", ref); got != "35N079W" {
		t.Fatalf("bare LATD was decorated: %q", got)
	}
	if got := tg.Decorate("LATD:35N079W", ref); got == "LATD:35N079W" {
		t.Fatal("labeled LATD was not decorated")
	}
}

func TestAreaCodeDetectionFollowsTheAlphabet(t *testing.T) {
	tg := tagger(t, &location.Decorator{})

	for _, tc := range []struct {
		bare      string
		labeled   string
		wantsBare bool
	}{
		{"GJNJ5753", "GEOREF:GJNJ5753", false},
		{"006AG39", "GARS:006AG39", false},
		{"849VCWC8+R9", "PLUSCODE:849VCWC8+R9", true},
	} {
		t.Run(tc.bare, func(t *testing.T) {
			text := "posn " + tc.bare + " confirmed"
			if decorated := tg.Decorate(text, ref) != text; decorated != tc.wantsBare {
				t.Errorf("bare %q decorated = %v, want %v", tc.bare, decorated, tc.wantsBare)
			}

			if got := tg.Decorate(tc.labeled, ref); got == tc.labeled {
				t.Errorf("labeled %q was not decorated", tc.labeled)
			}
		})
	}
}

func TestAreaLabelsDoNotRescueAnInvalidCode(t *testing.T) {
	tg := tagger(t, &location.Decorator{})

	for _, input := range []string{
		"GEOREF:IJNJ5753",
		"GARS:721LT",
		"GARS:206ZZ",
		"PLUSCODE:CWC8+R9",
		"PLUSCODE:849VC000+",
	} {
		if got := tg.Decorate(input, ref); got != input {
			t.Errorf("Decorate(%q) = %q, want it left alone", input, got)
		}
	}
}

func TestUSNGLabelsAGridReference(t *testing.T) {
	tg := tagger(t, &location.Decorator{})

	got := tg.Decorate("USNG:18SUJ2306", ref)
	if got == "USNG:18SUJ2306" {
		t.Fatal("a USNG label did not reach the grid grammar")
	}
	if !strings.Contains(got, "f=mgrs") {
		t.Fatalf("a USNG label produced %q, want a link carrying f=mgrs", got)
	}

	if !strings.HasPrefix(got, "USNG:") {
		t.Fatalf("the USNG label was consumed: %q", got)
	}
}

func TestBarePlusCodesDoNotMatchOrdinaryRuns(t *testing.T) {
	skipCorpusSweepWhenShort(t)

	tg := tagger(t, &location.Decorator{})

	rng := rand.New(rand.NewSource(20260814)) //nolint:gosec // deterministic on purpose, not a security context

	const base64Alphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/"

	versions := func(alphabet, tail string) func() string {
		return func() string {
			var b strings.Builder
			for range 4 + rng.Intn(8) {
				b.WriteByte(alphabet[rng.Intn(len(alphabet))])
			}
			b.WriteByte('+')
			for range 2 + rng.Intn(8) {
				b.WriteByte(tail[rng.Intn(len(tail))])
			}
			return b.String()
		}
	}

	const (
		lowerBody = "0123456789abcdefghijklmnopqrstuvwxyz"
		upperBody = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	)

	for _, corpus := range []struct {
		name  string
		build func() string
		n     int

		allowedRate float64
	}{
		{
			// Smaller than the version-string corpora because there is no rate
			// to size against: this class is refused by the trailing guard as
			// much as by the case, and measured zero both before and after the
			// upper-case restriction. It is a shape check, not a rate check.
			name: "base64 fragments",
			build: func() string {
				var b strings.Builder
				for range 12 + rng.Intn(20) {
					b.WriteByte(base64Alphabet[rng.Intn(len(base64Alphabet))])
				}
				return b.String()
			},
			n: 50000,
		},
		{
			// Sized to the same standard the grid sweep uses: about five
			// expected hits if the upper-case restriction were lifted, against
			// the measured one-in-50,000 rate. Larger buys nothing and this
			// runs under -race and coverage instrumentation in make coverage.
			name:  "version strings",
			build: versions(lowerBody+".", lowerBody),
			n:     250000,
		},
		{
			name:        "upper case version strings",
			build:       versions(upperBody+".", upperBody),
			n:           100000,
			allowedRate: 1.0 / 10000,
		},
	} {
		t.Run(corpus.name, func(t *testing.T) {
			var decorated []string

			for range corpus.n {
				run := corpus.build()

				text := "see build " + run + " for the fix"
				if tg.Decorate(text, ref) != text {
					decorated = append(decorated, run)
				}
			}

			if rate := float64(len(decorated)) / float64(corpus.n); rate > corpus.allowedRate {
				show := decorated
				if len(show) > 10 {
					show = show[:10]
				}
				t.Fatalf("%d of %d %s were rewritten into links (%.5f, allowed %.5f): %v",
					len(decorated), corpus.n, corpus.name, rate, corpus.allowedRate, show)
			}
		})
	}
}

// The two decorators must not compete for the same span, and each must still
// work with the other registered.
func TestLocationAndDTGCoexist(t *testing.T) {
	tg := tagger(t, &dtg.Decorator{}, &location.Decorator{})

	got := tg.Decorate("execute 091630ZAUG26 at 3510N07901W", ref)
	if n := linkCount(got); n != 2 {
		t.Fatalf("decorated %d tokens, want 2:\n%s", n, got)
	}
	if !strings.Contains(got, "/decorate/dtg?") || !strings.Contains(got, "/decorate/location?") {
		t.Fatalf("Decorate() did not produce one link of each type:\n%s", got)
	}
}

// Decoration is idempotent: a link it already wrote is itself a protected span.
func TestDecorateIsIdempotent(t *testing.T) {
	tg := tagger(t, &location.Decorator{})

	once := tg.Decorate("grid 3510N07901W", ref)
	if twice := tg.Decorate(once, ref); twice != once {
		t.Fatalf("Decorate() is not idempotent:\n once: %s\ntwice: %s", once, twice)
	}
}

// A coordinate at either edge of the message has no rune on that side, which
// must read as a clean edge rather than as an unknown neighbor.
func TestCoordinatesAtMessageEdges(t *testing.T) {
	tg := tagger(t, &location.Decorator{})

	for _, input := range []string{"3510N07901W", "3510N07901W tail", "head 3510N07901W"} {
		if got := tg.Decorate(input, ref); got == input {
			t.Errorf("Decorate(%q) left it undecorated", input)
		}
	}
}

// A non-breaking space is whitespace, so a coordinate separated by one behaves
// exactly as it would with an ordinary space.
func TestNonBreakingSpaceIsASeparator(t *testing.T) {
	tg := tagger(t, &location.Decorator{})

	// A literal U+00A0 between the two, which unicode.IsSpace accepts and a
	// naive == ' ' check would not.
	got := tg.Decorate("3510N07901W\u00a03610N08001W", ref)
	if n := linkCount(got); n != 2 {
		t.Fatalf("decorated %d coordinates, want 2:\n%s", n, got)
	}
}

// A run-together grid reference is decorated without a label, and the two
// restrictions that make that safe.
//
// Both were chosen against the measurement in the fuzz below rather than by
// argument, and the argument they replaced was wrong in an instructive way: the
// worry was part numbers, and the real collision is hexadecimal.
func TestBareRunTogetherGridReferences(t *testing.T) {
	tg := tagger(t, &location.Decorator{})

	for _, tc := range []struct {
		name    string
		text    string
		want    bool
		labelOK bool
	}{
		{"upper case, five digits per axis", "grid 18SUJ2347806483 confirmed", true, true},
		{"upper case, four digits per axis", "grid 4QFJ12345678 confirmed", true, true},
		{"upper case, three digits per axis", "grid 18SUJ234064 confirmed", true, true},

		// Lower case reaches the hexadecimal collision space, so it is not
		// detected on its own. Spacing or a label still works, so nothing here
		// is unreachable.
		{"lower case", "grid 18suj2347806483 confirmed", false, true},

		// Coarser than 100 m is a shorter run, and a short run is where the
		// part-number collisions live: "26HMA1997" is a well-formed grid
		// reference. Same argument that keeps LATD behind a label.
		{"two digits per axis", "grid 18SUJ2306 confirmed", false, true},
		{"one digit per axis", "grid 18SUJ23 confirmed", false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			token := strings.Fields(tc.text)[1]

			if got := tg.Decorate(tc.text, ref) != tc.text; got != tc.want {
				t.Errorf("bare %q decorated = %v, want %v", token, got, tc.want)
			}

			// Whatever the bare answer, a label must reach it, or the
			// restriction would be removing the token rather than deferring it.
			labeled := "MGRS:" + token
			if got := tg.Decorate(labeled, ref) != labeled; got != tc.labelOK {
				t.Errorf("labeled %q decorated = %v, want %v", token, got, tc.labelOK)
			}
		})
	}

	// And spacing reaches the two the bare run-together form declines. Note the
	// 10 km square spaces as two single digits: the halves of a grid reference
	// are separate groups, so "18S UJ 23" is the run-together form with a
	// spaced digraph, not the spaced form.
	for _, spaced := range []string{"18s uj 23478 06483", "18S UJ 2 3"} {
		text := "grid " + spaced + " confirmed"
		if tg.Decorate(text, ref) == text {
			t.Errorf("spaced %q was not decorated", spaced)
		}
	}
}

// The measurement the bare run-together grammar was chosen against.
//
// A short git SHA is seven to twelve lower-case hexadecimal characters, which
// is exactly the shape a run-together grid reference takes. Before the
// upper-case restriction, one in roughly 3,900 of them was a well-formed grid
// reference and got a link written into somebody's message: "58cbe40" is zone
// 58, band C, square BE, a 10 km square off the coast of Antarctica.
//
// This is deliberately generated rather than a fixed list. A fixed list only
// proves that the strings someone thought of are safe, and nobody thinks of
// 58cbe40.
func TestBareGridReferencesDoNotMatchOrdinaryRuns(t *testing.T) {
	skipCorpusSweepWhenShort(t)

	tg := tagger(t, &location.Decorator{})

	// A fixed seed, deliberately. This corpus exists to catch a regression in
	// what the tagger will rewrite into somebody's message, so a failure has to
	// be reproducible from the failure message alone. gosec objects to
	// math/rand on principle; there is nothing here for an attacker to predict.
	rng := rand.New(rand.NewSource(20260811)) //nolint:gosec // deterministic on purpose, not a security context

	// Sized against the rates each restriction was measured to remove, so that
	// a regression is caught rather than so the numbers look large. This matters
	// more than it sounds: an earlier version of this test used 30,000
	// uppercase samples against a restriction whose regression rate is about one
	// in 75,000, so it missed the regression roughly seven times in eight and
	// passed only because its seed happened to be clean. The uppercase corpus
	// therefore dominates the runtime here, and it is the one that has to.
	//
	//	restriction          regression rate   n        expected hits if regressed
	//	upper case only      1 in 3,900        60,000   ~15
	//	3 digits per axis    1 in 75,000       400,000  ~5
	for _, corpus := range []struct {
		name     string
		alphabet string
		lengths  []int
		n        int
	}{
		{"short git SHAs", "0123456789abcdef", []int{7, 8, 9, 10, 11, 12}, 60000},
		{"full git SHAs", "0123456789abcdef", []int{40}, 5000},
		{"upper case alphanumeric", "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ", []int{6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17}, 400000},
	} {
		t.Run(corpus.name, func(t *testing.T) {
			var decorated []string

			for range corpus.n {
				var b strings.Builder
				for range corpus.lengths[rng.Intn(len(corpus.lengths))] {
					b.WriteByte(corpus.alphabet[rng.Intn(len(corpus.alphabet))])
				}

				// In a sentence, so the boundary guard runs the way it does on
				// a real message rather than against the edges of the string.
				text := "see commit " + b.String() + " for the fix"
				if tg.Decorate(text, ref) != text {
					decorated = append(decorated, b.String())
				}
			}

			if len(decorated) > 0 {
				show := decorated
				if len(show) > 10 {
					show = show[:10]
				}
				t.Fatalf("%d of %d %s were rewritten into links: %v",
					len(decorated), corpus.n, corpus.name, show)
			}
		})
	}
}

// The bare grammar accepts upper case and refuses lower case, for the same
// square.
//
// Written as a paired assertion on both cases of the SAME token because the
// obvious version of this test was vacuous: it looped A to Z, never used a
// lower-case letter at all, and passed even when bare detection was removed
// entirely. The property has a case dimension, so the test has to have one.
func TestBareGridGrammarIsUpperCaseOnly(t *testing.T) {
	tg := tagger(t, &location.Decorator{})

	// Squares that really exist, so a refusal can only be about case.
	for _, upper := range []string{"18SUJ234064", "4QFJ12345678", "11SLT84636908"} {
		lower := strings.ToLower(upper)

		if tg.Decorate(upper, ref) == upper {
			t.Errorf("%q was not decorated bare", upper)
		}
		if tg.Decorate(lower, ref) != lower {
			t.Errorf("%q was decorated bare, and lower case must not be", lower)
		}

		// Both cases stay reachable behind a label, or the restriction would be
		// removing the token rather than deferring it.
		for _, token := range []string{upper, lower} {
			labeled := "MGRS:" + token
			if tg.Decorate(labeled, ref) == labeled {
				t.Errorf("%q was not decorated", labeled)
			}
		}
	}
}

// A label may not reach across a line break to claim a token.
//
// \s in RE2 includes \n, \f and \r, and with * on both sides of the colon a
// label at the end of one line captured whatever started the next: "MGRS:" then
// a newline then a lower-case git SHA was decorated, walking straight past the
// upper-case restriction that exists to keep hexadecimal out. The token
// sub-expressions have followed the same rule from the start, and dtg.go
// already used the right separator.
func TestMonikerDoesNotCrossALineBreak(t *testing.T) {
	tg := tagger(t, &location.Decorator{})

	// Tokens that are reachable ONLY behind a label, so a decoration here can
	// only have come from the label reaching across the break. Using a
	// bare-detectable token instead would pass for the wrong reason: the bare
	// pattern would claim it on the next line whatever the label did.
	for _, input := range []string{
		"MGRS:\n58cbe40 is the commit",
		"the grid is MGRS:\n\n18suj2347806483",
		"MGRS:\n18SUJ2306",
		"LATD:\n35N079W",
		"MGRS:\r\n18suj2347806483",
		"LATD:\f35N079W",
	} {
		if got := tg.Decorate(input, ref); got != input {
			t.Errorf("Decorate(%q) reached across the break: %q", input, got)
		}
	}

	// Spaces and tabs on one line are still fine, which is what dtg.go allows.
	for _, input := range []string{
		"MGRS: 18suj2347806483",
		"MGRS:\t18SUJ2306",
		"LATD : 35N079W",
	} {
		if got := tg.Decorate(input, ref); got == input {
			t.Errorf("Decorate(%q) left a legitimate label undecorated", input)
		}
	}
}

// UTM's optional axis letters must be adjacent to their digits.
//
// They are optional, so a space before one would let the token reach into the
// following word: in "11S 384640 3769080 East" the trailing letter would be
// swallowed, the boundary guard would then see a letter, and a token that
// decorates today would silently stop. That is why the letters are written
// adjacent in the grammar rather than separated by an optional space, and this
// is the regression that would catch relaxing it.
func TestUTMAxisLettersDoNotReachIntoTheNextWord(t *testing.T) {
	tg := tagger(t, &location.Decorator{})

	for _, tc := range []struct{ name, text, want string }{
		{"a word beginning with E", "11S 384640 3769080 East", "11S 384640 3769080"},
		{"a word beginning with N", "at 11S 384640 3769080 North", "11S 384640 3769080"},
		{"the letters attached, which is the whole point", "11S 384640E 3769080N", "11S 384640E 3769080N"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := tg.Decorate(tc.text, ref)
			if !strings.Contains(got, "["+tc.want+"]") {
				t.Fatalf("decorated %q as\n%s\nwant the token to be %q", tc.text, got, tc.want)
			}
		})
	}
}

// The paste that started this: a UTM position in the ordinary military
// spelling, band letter and axis letters and all.
func TestMilitaryUTMSpellingIsDecorated(t *testing.T) {
	tg := tagger(t, &location.Decorator{})

	const text = "11S 384640E 3769080N"

	got := tg.Decorate(text, ref)
	if got == text {
		t.Fatalf("Decorate(%q) left it undecorated", text)
	}
	if !strings.Contains(got, "f=utm") {
		t.Fatalf("decorated as something other than UTM:\n%s", got)
	}
}

// A label is guarded on both sides, exactly as a bare token is.
//
// The leading guard used to refuse only a letter or a digit, so a moniker was
// matched inside something the author had written: "logs/MGRS:18SUJ2347806483"
// was rewritten in place while the bare token in the identical position was
// correctly declined. Rewriting the middle of a path is corruption; a missed
// decoration is a feature gap.
func TestALabelIsGuardedTheSameWayABareTokenIs(t *testing.T) {
	tg := tagger(t, &location.Decorator{})

	for _, tc := range []struct {
		name      string
		text      string
		decorated bool
	}{
		{"inside a path", "logs/MGRS:18SUJ2347806483 failed", false},
		{"inside an identifier", "job_MGRS:18SUJ2347806483_raw", false},
		{"after a hyphen", "a-MGRS:18SUJ2347806483", false},
		{"after a colon", "x:LATM:3510N07901W", false},
		{"after a plus", "+LATM:3510N07901W", false},
		{"after a full stop", ".MGRS:18SUJ2306", false},

		// The cost, named rather than hidden: a slash-delimited USMTF line no
		// longer decorates. A genuine one ends "//" and was already declined by
		// the trailing side of the same guard, so this costs little.
		{"a slash-delimited USMTF line", "GEOLOC/LATM:3510N07901W", false},

		// And the ordinary cases still work.
		{"in prose", "see MGRS:18SUJ2347806483 there", true},
		{"at the start of a message", "MGRS:18SUJ2347806483", true},
		{"in parentheses", "(LATM:3510N07901W)", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := tg.Decorate(tc.text, ref)

			if decorated := got != tc.text; decorated != tc.decorated {
				t.Fatalf("Decorate(%q) decorated = %v, want %v:\n%s",
					tc.text, decorated, tc.decorated, got)
			}
		})
	}
}

// An underscore binds an identifier, so a coordinate inside one is not a
// coordinate.
//
// This is the one character on which the hand-written guard was weaker than the
// `\b` it replaced: Go counts `_` as a word character, so the DTG decorator,
// which still uses word boundaries, never had this hole while the location
// grammars did. The two therefore disagreed about the same shape of text in the
// same message, which is how it went unnoticed.
//
// The cost is a decoration, and it is the right way round: a coordinate written
// deliberately still decorates with a space, a bracket or a line start in front
// of it, while rewriting the middle of a filename is permanent.
func TestCoordinatesInsideIdentifiersAreLeftAlone(t *testing.T) {
	tg := tagger(t, &location.Decorator{})

	for _, tc := range []struct {
		name      string
		text      string
		decorated bool
	}{
		{"leading underscore", "_3510N07901W", false},
		{"trailing underscore", "3510N07901W_", false},
		{"both sides", "FOO_3510N07901W_BAR", false},
		{"a snake_case filename", "snapshot_3510N07901W_v2.zip", false},
		{"a run-together grid reference", "FOO_18SUJ2347806483_BAR", false},
		{"a spaced grid reference", "job_18S UJ 23478 06483", false},

		// Still decorated, so this bought the corruption fix and nothing else.
		// Brackets are absent on purpose: a bracketed span is protected by the
		// tagger, so it would pass here for a reason that has nothing to do
		// with this guard.
		{"in prose", "the target is 3510N07901W now", true},
		{"in parentheses", "(3510N07901W)", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := tg.Decorate(tc.text, ref)

			if decorated := got != tc.text; decorated != tc.decorated {
				t.Fatalf("Decorate(%q) decorated = %v, want %v:\n%s",
					tc.text, decorated, tc.decorated, got)
			}
		})
	}
}

// The two guards are the same function, so they cannot drift apart again.
//
// Asserted over every rune the leading side has to refuse rather than by
// inspecting the source, since the failure this pins is behavioural.
func TestBareAndLabeledGuardsAgree(t *testing.T) {
	tg := tagger(t, &location.Decorator{})

	for _, before := range []string{"/", ":", "-", "+", ".", ",", "_", "a", "7"} {
		t.Run("before "+before, func(t *testing.T) {
			bare := tg.Decorate(before+"18S UJ 23478 06483", ref)
			labeled := tg.Decorate(before+"MGRS:18SUJ2347806483", ref)

			if bare != before+"18S UJ 23478 06483" {
				t.Errorf("the bare token was decorated after %q", before)
			}
			if labeled != before+"MGRS:18SUJ2347806483" {
				t.Errorf("the labeled token was decorated after %q", before)
			}
		})
	}
}
