package decorators

import (
	"testing"
	"time"
)

func TestResultListsEveryAcceptedTokenInMessageOrder(t *testing.T) {
	_, got := monikerTagger(t).DecorateWithResult("1234 GRID: 5678\n9012", time.Now().UTC())

	if len(got.Tokens) != 3 {
		t.Fatalf("Tokens = %d, want 3", len(got.Tokens))
	}
	for i, want := range []string{"1234", "5678", "9012"} {
		if v := got.Tokens[i].Params.Get("v"); v != want {
			t.Errorf("Tokens[%d] = %q, want %q (message order, not longest first)", i, v, want)
		}
		if got.Tokens[i].Type != "mock" {
			t.Errorf("Tokens[%d].Type = %q", i, got.Tokens[i].Type)
		}
	}
}

func TestResultCoversAMessageOfNothingButTokensAndWhitespace(t *testing.T) {
	for _, message := range []string{
		"1234 5678",
		"1234\n5678",
		"  1234 \t 5678  \n",
		"GRID: 1234 GRID: 5678",
		"1234",
	} {
		_, got := monikerTagger(t).DecorateWithResult(message, time.Now().UTC())
		if !got.Covers {
			t.Errorf("%q: Covers = false, want true", message)
		}
		if got.OnlyType != "mock" {
			t.Errorf("%q: OnlyType = %q, want mock", message, got.OnlyType)
		}
	}
}

func TestResultDoesNotCoverProseBetweenTokens(t *testing.T) {
	for _, message := range []string{
		"1234, 5678",
		"1234 then 5678",
		"see 1234 5678",
		"1234 5678 now",
		"nothing here",
	} {
		_, got := monikerTagger(t).DecorateWithResult(message, time.Now().UTC())
		if got.Covers {
			t.Errorf("%q: Covers = true, want false", message)
		}
	}
}

func TestSoleTokenIsCoversWithOneToken(t *testing.T) {
	_, one := monikerTagger(t).DecorateWithResult("GRID: 1234", time.Now().UTC())
	if !one.SoleToken || !one.Covers || len(one.Tokens) != 1 {
		t.Fatalf("one token: SoleToken = %v, Covers = %v, Tokens = %d", one.SoleToken, one.Covers, len(one.Tokens))
	}
	if one.Type != one.Tokens[0].Type || one.Params.Get("v") != one.Tokens[0].Params.Get("v") || one.Trail != one.Tokens[0].Trail {
		t.Error("the sole token and Tokens[0] disagree")
	}

	_, two := monikerTagger(t).DecorateWithResult("1234 5678", time.Now().UTC())
	if two.SoleToken || !two.Covers {
		t.Fatalf("two tokens: SoleToken = %v, Covers = %v", two.SoleToken, two.Covers)
	}
}

func TestTheVerdictSurvivesApplyReplacementsReordering(t *testing.T) {
	tagger := monikerTagger(t)
	decorated, got := tagger.DecorateWithResult("1234 GRID: 5678", time.Now().UTC())

	if decorated == "1234 GRID: 5678" {
		t.Fatal("nothing was decorated")
	}
	if got.Tokens[0].Params.Get("v") != "1234" || got.Tokens[1].Params.Get("v") != "5678" {
		t.Errorf("Tokens are %q then %q, want message order", got.Tokens[0].Params.Get("v"), got.Tokens[1].Params.Get("v"))
	}
}
