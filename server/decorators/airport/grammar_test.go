package airport

import (
	"strings"
	"testing"
)

func TestByLengthDescendingSortsAndCopies(t *testing.T) {
	original := []string{"LOC", "ICAO", "DEPLOC", "AB", "ARRLOC"}
	before := strings.Join(original, ",")

	sorted := byLengthDescending(original)

	for i := 1; i < len(sorted); i++ {
		if len(sorted[i]) > len(sorted[i-1]) {
			t.Fatalf("%v is not longest first", sorted)
		}
	}
	if got := strings.Join(original, ","); got != before {
		t.Errorf("the input was reordered to %q, want it left at %q", got, before)
	}
}

func TestMonikerAlternationIsLongestFirst(t *testing.T) {
	for _, longer := range []string{"DEPLOC", "ARRLOC"} {
		if !strings.HasSuffix(longer, "LOC") {
			t.Fatalf("%s no longer ends in LOC, so this test no longer states the risk", longer)
		}
		if strings.Index(monikerExpr, longer) > strings.Index(monikerExpr, "|LOC") {
			t.Errorf("%s follows LOC in %q, so LOC would win the shorter reading", longer, monikerExpr)
		}
	}
}
