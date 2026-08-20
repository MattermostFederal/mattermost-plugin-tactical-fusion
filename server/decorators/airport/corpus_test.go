package airport

import (
	_ "embed"
	"strings"
	"testing"
	"time"
)

//go:embed data/words4.txt
var fourLetterWords string

// wordIdents are the idents that are also ordinary English words.
//
// These are the reason nothing here is detected without a label. Measured
// rather than argued: the count is what makes "four letters is what prose is
// made of" a fact about this dataset instead of an intuition.
//
// The word list is embedded rather than read from /usr/share/dict/words, which
// exists on macOS and not on the CI runner, so this used to skip on every pull
// request. See data/README.md.
func wordIdents(t *testing.T) []string {
	t.Helper()

	var out []string
	for word := range strings.FieldsSeq(fourLetterWords) {
		upper := strings.ToUpper(word)
		if _, ok := Lookup(upper); ok {
			out = append(out, upper)
		}
	}

	if len(out) == 0 {
		t.Fatal("the embedded word list matched no idents at all, so it is not being read")
	}

	return out
}

func TestManyIdentsAreOrdinaryWords(t *testing.T) {
	words := wordIdents(t)
	if len(words) < 100 {
		t.Fatalf("only %d idents are dictionary words, which does not match the measurement "+
			"the label-only grammar rests on", len(words))
	}
	t.Logf("%d of %d idents are English dictionary words", len(words), Count())
}

// The label rule, swept over EVERY ident rather than over the word-shaped ones.
//
// Every assertion here holds for all of them: there is no bare pattern, a
// label is matched in upper case only, and an ident this build holds decorates
// behind one. Restricting the sweep to dictionary words made it strictly weaker
// and made it depend on a word list it does not need, which is why it skipped
// in CI while claiming to be the corpus the design rests on.
//
// Short-gated, because it runs the whole tagger tens of thousands of times.
// CLAUDE.md records that make coverage needs -short and says anything slow
// enough should use it rather than a bigger timeout.
func TestIdentsOnlyDecorateBehindAnUpperCaseLabel(t *testing.T) {
	if testing.Short() {
		t.Skip("the corpus sweep is slow under -race and coverage")
	}

	tagger := newTagger(t)
	ref := time.Now().UTC()

	sentences := []string{
		"the %s report is due",
		"we need %s before nightfall",
		"%s",
		"send %s now",
	}

	for ident := range airfields {
		for _, shape := range sentences {
			bare := strings.ReplaceAll(shape, "%s", ident)
			if tagger.Decorate(bare, ref) != bare {
				t.Fatalf("a bare ident was decorated: %q", bare)
			}
		}

		for _, label := range []string{"loc", "icao", "deploc", "arrloc", "Loc", "Icao"} {
			lower := "the " + label + ": " + strings.ToLower(ident) + " report is due"
			if tagger.Decorate(lower, ref) != lower {
				t.Fatalf("a lower-case label decorated: %q", lower)
			}

			mixed := "the " + label + ": " + ident + " report is due"
			if tagger.Decorate(mixed, ref) != mixed {
				t.Fatalf("a non-upper-case label decorated: %q", mixed)
			}
		}

		upper := "ICAO:" + ident
		if tagger.Decorate(upper, ref) == upper {
			t.Fatalf("an upper-case label did not decorate: %q", upper)
		}
	}

	t.Logf("swept %d idents", Count())
}

// Every shipped ident behind every label, written the way a sentence is
// written. Nothing may decorate: the separator admits nothing after the colon,
// which is what keeps 343 word-shaped idents out of ordinary capitalized prose.
func TestNoIdentIsReachedByCapitalizedProse(t *testing.T) {
	tagger := newTagger(t)
	now := time.Now().UTC()

	var rewritten []string
	for ident := range airfields {
		for _, prefix := range monikerPrefixes {
			message := prefix + ": " + ident + " IS THE NEXT WORD"
			if out := tagger.Decorate(message, now); out != message {
				rewritten = append(rewritten, out)
			}
		}
	}

	if len(rewritten) != 0 {
		t.Errorf("%d capitalized sentences were rewritten, first three:", len(rewritten))
		for _, out := range rewritten[:min(3, len(rewritten))] {
			t.Errorf("  %s", out)
		}
	}
}
