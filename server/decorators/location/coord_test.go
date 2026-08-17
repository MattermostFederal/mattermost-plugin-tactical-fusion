package location

import (
	"strings"
	"testing"
)

// Axis and Location are exported with exported fields and no constructor, so a
// hand-built one reaches these helpers with values the parser would never
// produce. That is the hazard the guards below exist for, and the only way to
// reach them.

// The FracUnit switch answers in the unit the field belongs to, and anything it
// does not recognise contributes nothing rather than being read as degrees.
func TestFracValueScalesByTheFieldItBelongsTo(t *testing.T) {
	for _, tc := range []struct {
		name string
		axis Axis
		want float64
	}{
		{"seconds are a 3600th of a degree", Axis{Frac: "5", FracUnit: FracSeconds}, 0.5 / 3600},
		{"minutes are a 60th", Axis{Frac: "5", FracUnit: FracMinutes}, 0.5 / 60},
		{"degrees are themselves", Axis{Frac: "5", FracUnit: FracDegrees}, 0.5},
		{"no unit contributes nothing", Axis{Frac: "5", FracUnit: FracNone}, 0},

		// Frac is digits only by construction, so this is only reachable on a
		// hand-built Axis. Zero is the honest answer; a panic here would be
		// inside MessageWillBePosted.
		{"a fraction that is not digits", Axis{Frac: "x", FracUnit: FracDegrees}, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.axis.fracValue(); got != tc.want {
				t.Fatalf("fracValue() = %v, want %v", got, tc.want)
			}
		})
	}
}

// Confidence is a property of the pair, so one axis without it is the pair
// without it. Reporting the other axis's digit alone would be a claim about a
// position half of which nothing vouched for.
func TestConfidenceNeedsBothAxes(t *testing.T) {
	for _, tc := range []struct {
		name string
		lat  int8
		lon  int8
		want bool
	}{
		{"both carry a digit", 9, 7, true},
		{"neither does", NoConfidence, NoConfidence, false},
		{"only latitude does", 9, NoConfidence, false},
		{"only longitude does", NoConfidence, 7, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			l := Location{Lat: Axis{Conf: tc.lat}, Lon: Axis{Conf: tc.lon}}

			lat, lon, ok := l.Confidence()
			if ok != tc.want {
				t.Fatalf("Confidence() ok = %v, want %v", ok, tc.want)
			}
			if !ok {
				if lat != 0 || lon != 0 {
					t.Fatalf("Confidence() = %d, %d with ok=false, want a zero pair", lat, lon)
				}
				return
			}
			if lat != tc.lat || lon != tc.lon {
				t.Fatalf("Confidence() = %d, %d, want %d, %d", lat, lon, tc.lat, tc.lon)
			}
		})
	}
}

// Both halves of a pair are written at the pair's digit count, which is what
// makes the "coarser half wins" rule reproduce itself when the result is parsed
// again. A half with fewer digits than the pair is padded rather than left
// short, or the written token would carry a different count than it claims.
func TestWriteFracWritesTheDigitsItWasAskedFor(t *testing.T) {
	for _, tc := range []struct {
		name   string
		frac   string
		digits int
		want   string
	}{
		{"no digits writes no point at all", "5", 0, ""},
		{"a negative count writes nothing", "5", -1, ""},
		{"exactly the digits it has", "567", 3, ".567"},
		{"more digits than asked for are trimmed", "56789", 3, ".567"},
		{"fewer are padded to the count", "5", 3, ".500"},
		{"an empty fraction is all padding", "", 2, ".00"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var b strings.Builder
			writeFrac(&b, tc.frac, tc.digits)

			if got := b.String(); got != tc.want {
				t.Fatalf("writeFrac(%q, %d) wrote %q, want %q", tc.frac, tc.digits, got, tc.want)
			}
		})
	}
}
