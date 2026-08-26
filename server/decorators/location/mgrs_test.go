package location

import (
	"math"
	"testing"
)

// The guards on mgrsAt and utmAt, which the round-trip tests in geodesy_test.go
// cannot reach: a real projection never produces an easting off the grid, so
// the refusals are only reachable from a utmPoint built by hand.

// A position well inside zone 33, band U, used wherever the test is about
// something other than where the point is.
func gridPointFixture() utmPoint {
	return utmPoint{Zone: 33, Band: 'U', Easting: 291000, Northing: 5628000}
}

func TestMgrsAtRefusesDigitsItCannotWrite(t *testing.T) {
	for _, tc := range []struct {
		name   string
		digits int
		want   bool
	}{
		{"below the floor", -1, false},
		{"no digits at all, a 100 km square", 0, true},
		{"the finest square", maxGridDigits, true},
		{"one past the finest", maxGridDigits + 1, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, ok := mgrsAt(gridPointFixture(), tc.digits)
			if ok != tc.want {
				t.Fatalf("mgrsAt(digits=%d) ok = %v, want %v", tc.digits, ok, tc.want)
			}
		})
	}
}

// Both ends of the eight-letter column set, because the guard is a range and a
// test of one end would pass with the other side inverted.
func TestMgrsAtRefusesAPositionOffTheGrid(t *testing.T) {
	for _, tc := range []struct {
		name    string
		easting float64
	}{
		{"west of the first column", 50000},
		{"east of the eighth", 950000},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := gridPointFixture()
			p.Easting = tc.easting

			if _, ok := mgrsAt(p, 5); ok {
				t.Fatalf("mgrsAt accepted an easting of %v, which is in no column", tc.easting)
			}
		})
	}
}

// A step below a meter is clamped rather than refused or divided by, and the
// clamp is what keeps a zero step from making the rounding meaningless.
func TestUtmAtTreatsAStepBelowOneAsOneMetre(t *testing.T) {
	want, ok := utmAt(gridPointFixture(), 1)
	if !ok {
		t.Fatal("utmAt refused the fixture at a one meter step")
	}

	for _, step := range []float64{0, 0.5, -3} {
		got, ok := utmAt(gridPointFixture(), step)
		if !ok {
			t.Fatalf("utmAt(step=%v) refused a position it accepts at step 1", step)
		}
		if got != want {
			t.Fatalf("utmAt(step=%v) = %+v, want the step 1 answer %+v", step, got, want)
		}
	}
}

/*
 * The equator, which is where the two hemispheres' frames meet and disagree.
 *
 * The southern hemisphere is measured from a false origin 10,000 km south, so
 * the equator IS 10,000,000 in that frame, and a point a fraction south of it
 * rounds to exactly that and does not fit the seven digits a UTM northing is
 * written with. "-0.000001, 32.500000" lost its UTM row while its MGRS row
 * rendered fine, and an empty UTM row means "outside the grid" everywhere else
 * in this package.
 *
 * Clamped one meter south rather than wrapped to zero: zero beside a southern
 * band letter is the false origin itself, 10,000 km away, and produced a row
 * this package could not parse back. The northern twin is in the same table
 * because it is the half that makes the asymmetry visible, zero being an
 * ordinary northing there.
 */
func TestUtmAtKeepsAPositionEitherSideOfTheEquator(t *testing.T) {
	for _, tc := range []struct {
		name         string
		lat          float64
		wantNorthing int
		wantNorthern bool
	}{
		{"a fraction south", -0.000001, int(utmFalseNorthing) - 1, false},
		{"a fraction north", 0.000001, 0, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			g, ok := utmFor(tc.lat, 32.5, 1)
			if !ok {
				t.Fatalf("utmFor(%v, 32.5) refused a position on the grid", tc.lat)
			}

			if bandNorthern(g.Band) != tc.wantNorthern {
				t.Fatalf("band %q is northern=%v, want %v", g.Band, bandNorthern(g.Band), tc.wantNorthern)
			}
			if g.Northing != tc.wantNorthing {
				t.Fatalf("northing = %d, want %d", g.Northing, tc.wantNorthing)
			}

			// The row has to be one this package can read back, which is the
			// whole reason the clamp is a meter rather than a wrap to zero.
			lat, _, ok := utmPointOf(g)
			if !ok {
				t.Fatalf("utmPointOf refused %+v, the row utmFor just produced", g)
			}
			if math.Abs(lat) > 0.0001 {
				t.Fatalf("%+v reads back at latitude %v, which is not beside the equator", g, lat)
			}
		})
	}
}

func TestUtmAtRefusesAPositionOutsideTheGrid(t *testing.T) {
	for _, tc := range []struct {
		name     string
		easting  float64
		northing float64
	}{
		{"an easting past six digits", 1500000, 5628000},
		{"a negative easting", -5, 5628000},
		{"a northing past seven digits", 291000, 12000000},
		{"a negative northing", 291000, -5},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := gridPointFixture()
			p.Easting, p.Northing = tc.easting, tc.northing

			if _, ok := utmAt(p, 1); ok {
				t.Fatalf("utmAt accepted %v, %v, which UTM cannot write", tc.easting, tc.northing)
			}
		})
	}
}

// squareMeters clamps rather than refusing, because Grid is exported with
// exported fields and no constructor: a hand-built one with six digits reaches
// canonicalString, where an unclamped 100000/10^6 would be 0.1 and int(0.1) is
// zero. Its callers bound the value first, so only a direct call gets here.
func TestSquareMetersClampsDigitsToASquareItCanName(t *testing.T) {
	for _, tc := range []struct {
		name   string
		digits int
		want   float64
	}{
		{"below the floor is a 100 km square", -1, 100000},
		{"no digits is the same square", 0, 100000},
		{"one digit is 10 km", 1, 10000},
		{"the finest is one meter", maxGridDigits, 1},
		{"past the finest is still one meter", maxGridDigits + 1, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := squareMeters(tc.digits); got != tc.want {
				t.Fatalf("squareMeters(%d) = %v, want %v", tc.digits, got, tc.want)
			}
		})
	}
}

func TestZoneNearRefusesAPositionOffTheGrid(t *testing.T) {
	for _, lat := range []float64{85, -81} {
		if zoneNear(31, lat, 3) {
			t.Errorf("zoneNear(31, %v, 3) accepted a position UTM does not cover", lat)
		}
	}
}

func TestGridSquareForRefusesANorthingBelowTheFalseOrigin(t *testing.T) {
	if col, row, ok := gridSquareFor(utmPoint{Zone: 31, Band: 'U', Easting: 500000, Northing: -100000}); ok {
		t.Errorf("gridSquareFor with a negative northing = %q%q, want a refusal", col, row)
	}
}

func TestUTMTokensAreCheckedAgainstTheZoneTheyName(t *testing.T) {
	for _, tc := range []struct {
		name string
		grid Grid
	}{
		{
			name: "an easting naming no 100 km column",
			grid: Grid{Format: FormatUTM, Zone: 31, Band: 'P', Easting: 0, Northing: 1100000},
		},
		{
			name: "numbers hundreds of kilometers from the zone",
			grid: Grid{Format: FormatUTM, Zone: 31, Band: 'X', Easting: 100000, Northing: 8500000},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if p, ok := gridPoint(tc.grid); ok {
				t.Errorf("gridPoint = %v, want a refusal", p)
			}
		})
	}
}

func TestAGridSquareNeedsADigitCountTheNotationHas(t *testing.T) {
	for _, digits := range []int{0, -1, maxGridDigits + 1} {
		grid := Grid{Format: FormatMGRS, Zone: 18, Band: 'S', Col: 'U', Row: 'J', Digits: digits}
		if p, ok := mgrsGridPoint(grid); ok {
			t.Errorf("mgrsGridPoint with %d digits = %v, want a refusal", digits, p)
		}
	}
}
