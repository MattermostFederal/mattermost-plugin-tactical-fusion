package location

import (
	"strings"
	"testing"
)

// The conversion endpoint is a second door onto the same coordinates as the
// public page, so the thing worth testing is that it is not a weaker one.
//
// Every rejection below is a link the page also refuses. If the two ever
// disagree, the sidebar renders a position the page will not, which is exactly
// the "accepted here, rejected there" split the whole validation layout exists
// to prevent.
func TestConvertAcceptsWhatThePageAccepts(t *testing.T) {
	for _, tc := range acceptedTokens {
		t.Run(tc.canonical, func(t *testing.T) {
			raw := ""
			if tc.token != tc.canonical {
				raw = tc.token
			}

			conversion, ok := Convert(tc.format, tc.canonical, raw)
			if !ok {
				t.Fatalf("Convert(%s, %q, %q) refused a token the page renders",
					tc.format, tc.canonical, raw)
			}

			// The rows the panel cannot work out for itself, which are the
			// whole reason this endpoint exists.
			if conversion.Decimal == "" {
				t.Error("no decimal reading")
			}
			if conversion.DMS == "" || conversion.DDM == "" {
				t.Error("no sexagesimal reading")
			}
		})
	}
}

func TestConvertRefusesWhatThePageRefuses(t *testing.T) {
	for _, tc := range []struct {
		name           string
		format         Format
		canonical, raw string
	}{
		{"an unknown format id", "usng", "18SUJ2347806483", ""},
		{"an empty format id", "", "18SUJ2347806483", ""},
		{"a token that is not canonical", FormatMGRS, "18S UJ 23478 06483", ""},
		{"a token of a different grammar", FormatMGRS, "3510N07901W", ""},
		{"an empty token", FormatMGRS, "", ""},
		{"a square that names nothing", FormatMGRS, "32WDL123123", ""},

		// The four gates on the author's own text. Two of them need the token
		// grammar, so these are the cases the webapp cannot catch by itself and
		// asks about here instead.
		{"prose wrapped around a coordinate", FormatLATM, "3510N07901W", "3510N07901W ALFA"},
		{"an r naming a different place", FormatLATM, "3510N07901W", "3610N08001W"},
		{
			// Gate 1, the 64-byte cap, and it has to be exercised by a value
			// that is long AND otherwise valid. A row of 65 NUL bytes or 65
			// digits fails the alphabet or the grammar first, so the cap itself
			// was never reached: raising it to 100,000 broke no test.
			//
			// It is genuinely load-bearing, because degSep carries an unbounded
			// `sp+`. Padding a DMS token with runs of spaces keeps it an
			// anchored match for its own grammar and keeps it round-tripping to
			// the same canonical form, so nothing downstream of gate 1 objects
			// to it at any length.
			"an r that is long but otherwise valid", FormatDMS, "340322N1181500W",
			"34" + strings.Repeat(" ", 30) + "03" + strings.Repeat(" ", 30) + "22 N 118 15 00 W",
		},
		{"markup in r", FormatLATM, "3510N07901W", "3510N07901W<b>"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, ok := Convert(tc.format, tc.canonical, tc.raw); ok {
				t.Fatalf("Convert(%s, %q, %q) accepted a link the page refuses",
					tc.format, tc.canonical, tc.raw)
			}
		})
	}
}

// A coordinate outside the grid is still a coordinate, and the conversion has
// to say so rather than failing: the reader would otherwise be told their
// perfectly good polar position is not one this plugin issued.
func TestConvertKeepsAPolarCoordinate(t *testing.T) {
	conversion, ok := Convert(FormatDD, "85.1234,10.0000", "")
	if !ok {
		t.Fatal("Convert refused a valid coordinate outside the UTM latitudes")
	}

	if conversion.MGRS != "" || conversion.UTM != "" {
		t.Errorf("grid rows are %q and %q for a polar coordinate, want both empty",
			conversion.MGRS, conversion.UTM)
	}
	if conversion.Decimal == "" {
		t.Error("the rows that need no projection went missing with the ones that do")
	}
}

// Every accepted token converts to something the panel can put on screen.
func TestConvertFillsTheGridRowsEverywhereThereIsAGrid(t *testing.T) {
	for _, tc := range acceptedTokens {
		conversion, ok := Convert(tc.format, tc.canonical, "")
		if !ok {
			t.Fatalf("Convert refused %q", tc.canonical)
		}

		// The corpus is all mid-latitude, so every row should be populated.
		if conversion.MGRS == "" || conversion.UTM == "" {
			t.Errorf("%q converted with an empty grid row: mgrs=%q utm=%q",
				tc.canonical, conversion.MGRS, conversion.UTM)
		}
		if conversion.DMS == "" || conversion.DDM == "" {
			t.Errorf("%q converted with an empty sexagesimal row", tc.canonical)
		}
	}
}
