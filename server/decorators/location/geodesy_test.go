package location

import (
	"fmt"
	"math"
	"math/rand"
	"strings"
	"testing"
)

// The projection is hand-written, so the tests that matter are the ones with an
// authority outside this repository. A round trip proves only that the inverse
// undoes the forward, which is equally true of two functions that are both
// wrong in the same way, so the anchors come first and the round trips come
// after.

// The WGS 84 meridian quadrant: the distance from the equator to the pole along
// a meridian, 10,001,965.729 m. This is a published property of the ellipsoid
// and nothing in this package can influence it, which makes it the single
// strongest check on the meridian arc series.
func TestMeridianArcMatchesThePublishedQuadrant(t *testing.T) {
	const want = 10001965.729

	got := meridianArc(math.Pi / 2)
	if math.Abs(got-want) > 0.001 {
		t.Fatalf("meridianArc(90 degrees) = %.4f, want %.4f within a millimeter", got, want)
	}
}

// One degree of latitude at the equator is 110,574.3 m on WGS 84.
//
// The published quantity is the degree CENTERED on the equator, which is
// meridianArc(+0.5) - meridianArc(-0.5), not the arc from 0 to 1. An earlier
// version of this test asserted the latter against 110574.39, a number that is
// this code's own output rather than anybody's published figure, so it could
// only ever have confirmed itself. The two differ by 8 cm, which is a hundred
// times the tolerance here, so the distinction is the whole test.
func TestMeridianArcMatchesOneDegreeAtTheEquator(t *testing.T) {
	const want = 110574.30

	// meridianArc is odd in phi, so the centered degree is twice the half.
	got := 2 * meridianArc(0.5*math.Pi/180)
	if math.Abs(got-want) > 0.01 {
		t.Fatalf("the degree centered on the equator = %.4f, want %.2f", got, want)
	}
}

// On a central meridian the easting is the false easting exactly, and on the
// equator the northing is zero exactly. Both are definitional rather than
// approximate, so they are asserted to the millimeter rather than to a
// tolerance chosen to make them pass.
func TestProjectionIsExactAtItsOrigin(t *testing.T) {
	// Zone 18's central meridian is 75 degrees west.
	p, ok := projectUTM(0, -75)
	if !ok {
		t.Fatal("projectUTM refused the origin of zone 18")
	}

	if p.Zone != 18 {
		t.Errorf("zone = %d, want 18", p.Zone)
	}
	// The literals, not the package constants. Asserting utmFalseEasting
	// against utmFalseEasting passes for any value of it, and did: changing the
	// constant to 500001 left this test green.
	if math.Abs(p.Easting-500000) > 0.001 {
		t.Errorf("easting on the central meridian = %.6f, want 500000", p.Easting)
	}
	if math.Abs(p.Northing) > 0.001 {
		t.Errorf("northing on the equator = %.6f, want 0", p.Northing)
	}
}

// The southern hemisphere is measured from a false origin 10,000 km south of
// the equator, which is why a UTM token has to carry a band letter at all.
func TestSouthernHemisphereCarriesTheFalseNorthing(t *testing.T) {
	p, ok := projectUTM(-0.0001, -75)
	if !ok {
		t.Fatal("projectUTM refused a point just south of the equator")
	}

	// The literal false origin, 10,000 km south of the equator, for the same
	// reason as above.
	if !(p.Northing > 10000000-20 && p.Northing < 10000000) {
		t.Fatalf("northing just south of the equator = %.3f, want just under 10000000",
			p.Northing)
	}
	if bandNorthern(p.Band) {
		t.Fatalf("band %c reads as northern for a southern point", p.Band)
	}
}

// The truncated series is what bounds how finely this package may claim to
// work, so the bound is measured rather than asserted from the literature.
//
// Two figures, because they mean different things. Inside the six degrees a
// zone normally spans the error is sub-millimeter. The two regions where a zone
// was widened by hand (south-west Norway, Svalbard) put points up to six
// degrees off their central meridian, and there the error grows to a few
// centimeters. That is still twenty times finer than the finest grid square
// this package will ever render, which is the reason the wider zones are
// supported rather than declined.
func TestRoundTripStaysWellInsideTheFinestGrid(t *testing.T) {
	var worstNormal, worstWidened float64

	for lat := -79.5; lat < maxUTMLat; lat += 0.25 {
		for lon := -179.9; lon < 180; lon += 0.25 {
			p, ok := projectUTM(lat, lon)
			if !ok {
				t.Fatalf("projectUTM refused %.2f, %.2f", lat, lon)
			}

			gotLat, gotLon, ok := unprojectUTM(p)
			if !ok {
				t.Fatalf("unprojectUTM refused the output of %.2f, %.2f", lat, lon)
			}

			meters := math.Hypot(
				(gotLat-lat)*degreeMeters,
				(gotLon-lon)*degreeMeters*math.Cos(lat*math.Pi/180))

			if math.Abs(lon-centralMeridian(p.Zone)) <= 3 {
				worstNormal = math.Max(worstNormal, meters)
			} else {
				worstWidened = math.Max(worstWidened, meters)
			}
		}
	}

	if worstNormal > 0.005 {
		t.Errorf("worst round trip inside a standard zone = %.6f m, want under 5 mm", worstNormal)
	}
	if worstWidened > 0.1 {
		t.Errorf("worst round trip in a widened zone = %.6f m, want under 100 mm", worstWidened)
	}

	t.Logf("worst round trip: %.6f m standard, %.6f m widened", worstNormal, worstWidened)
}

// The two places the zones are not six degrees wide. Getting either wrong puts
// a point in the neighboring zone, which is hundreds of kilometers of easting
// and a grid reference no other tool agrees with.
func TestZoneExceptions(t *testing.T) {
	for _, tc := range []struct {
		name     string
		lat, lon float64
		want     int
	}{
		{"south-west Norway is zone 32, not 31", 60, 5, 32},
		{"just south of the Norway exception", 55.9, 5, 31},
		{"just north of the Norway exception", 64.1, 5, 31},
		{"just west of the Norway exception", 60, 2.9, 31},
		{"just east of the Norway exception", 60, 12.1, 33},
		// Longitudes chosen so the plain formula gives a DIFFERENT answer,
		// which is the only way these rows assert anything. An earlier version
		// used 5, 15 and 25 east, where the formula already produces 31, 33 and
		// 35, so deleting the whole Svalbard block left three of the four
		// passing.
		{"Svalbard 7 east is zone 31, not 32", 78, 7, 31},
		{"Svalbard 10 east is zone 33, not 32", 78, 10, 33},
		{"Svalbard 20 east is zone 33, not 34", 78, 20, 33},
		{"Svalbard 22 east is zone 35, not 34", 78, 22, 35},
		{"Svalbard 30 east is zone 35, not 36", 78, 30, 35},
		{"Svalbard 35 east is zone 37, not 36", 78, 35, 37},

		// And the even zones are unreachable in that band.
		{"Svalbard has no zone 32", 78, 8.9, 31},
		{"Svalbard has no zone 34", 78, 21, 35},
		{"ordinary longitude is the plain formula", 38.8895, -77.0353, 18},
		{"the antimeridian is zone 1", 0, -180, 1},
		{"the far east is zone 60", 0, 179.9, 60},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := zoneFor(tc.lat, tc.lon)
			if !ok {
				t.Fatalf("zoneFor(%v, %v) refused a valid position", tc.lat, tc.lon)
			}
			if got != tc.want {
				t.Fatalf("zoneFor(%v, %v) = %d, want %d", tc.lat, tc.lon, got, tc.want)
			}
		})
	}

	// The exception is a rectangle, and the rows above walk out of it on all
	// four sides. Five degrees east is zone 31 by the plain formula, so every
	// "just outside" row asserting 31 is asserting that the widening stopped;
	// the two longitude rows land in the neighboring zones for the same
	// reason.

	// Between 72 and 84 north the widened scheme uses only odd zones, so no
	// longitude at all may produce 32, 34 or 36.
	for lon := 0.0; lon < 42; lon += 0.25 {
		got, ok := zoneFor(78, lon)
		if !ok {
			t.Fatalf("zoneFor(78, %v) refused a valid position", lon)
		}
		if got%2 == 0 {
			t.Errorf("zoneFor(78, %v) = %d, which is even; Svalbard has no even zones", lon, got)
		}
	}
}

// An external anchor OFF the central meridian.
//
// Every other anchor in this file sits on the central meridian or the equator,
// where the easting series vanishes and only the meridian arc is exercised.
// Mutation testing showed the gap plainly: swapping two coefficients in the A^6
// northing term, or deleting the t^2 sub-term entirely, failed no test here.
//
// These two figures were produced by an independently written Krueger series, a
// different formulation with no term in common with Snyder's, and agree with
// this code to 4 micrometers of easting and 0.12 mm of northing. The same
// transcription reproduces the EPSG Guidance Note 7-2 worked Transverse
// Mercator example to under a centimeter.
func TestProjectionOffTheCentralMeridian(t *testing.T) {
	p, ok := projectUTM(38.8895, -77.0353)
	if !ok {
		t.Fatal("projectUTM refused the Washington Monument")
	}

	if p.Zone != 18 || p.Band != 'S' {
		t.Fatalf("zone/band = %d%c, want 18S", p.Zone, p.Band)
	}
	if math.Abs(p.Easting-323478.063) > 0.01 {
		t.Errorf("easting = %.4f, want 323478.063", p.Easting)
	}
	if math.Abs(p.Northing-4306483.242) > 0.01 {
		t.Errorf("northing = %.4f, want 4306483.242", p.Northing)
	}
}

// Anything mgrsFor writes, Parse must read back.
//
// The two are deliberately different code paths, so nothing makes this true by
// construction, and both places it failed were quiet: a band-boundary strip
// where the slop in withinBand was fractionally too small, and the easternmost
// strip of zone 60, where a square clipped by the antimeridian has its center
// just past it. Neither corrupted anything, because Parse refusing is safe, but
// both meant the package rendered a reference it would not itself accept.
func TestEveryReferenceItWritesItCanRead(t *testing.T) {
	rng := rand.New(rand.NewSource(5)) //nolint:gosec // deterministic on purpose, not a security context

	var mismatches []string
	for range 400000 {
		lat := -80 + rng.Float64()*164
		lon := -180 + rng.Float64()*360
		digits := 1 + rng.Intn(maxGridDigits)

		g, ok := mgrsFor(lat, lon, digits)
		if !ok {
			continue
		}

		token := Location{Grid: g, Format: FormatMGRS}.Canonical()
		if _, parsed := Parse(FormatMGRS, token); !parsed {
			if len(mismatches) < 10 {
				mismatches = append(mismatches,
					fmt.Sprintf("%s from %.9f,%.9f at %d digits", token, lat, lon, digits))
			}
		}
	}

	if len(mismatches) > 0 {
		t.Fatalf("%d references were written but not readable:\n%s",
			len(mismatches), strings.Join(mismatches, "\n"))
	}
}

// The band letters, which carry the hemisphere and are what pick a decoded
// northing out of its five candidates.
func TestLatitudeBands(t *testing.T) {
	for _, tc := range []struct {
		lat  float64
		want byte
	}{
		{-80, 'C'},
		{-72.1, 'C'},
		{-72, 'D'},
		{-0.1, 'M'},
		{0, 'N'},
		{38.8895, 'S'},
		{56, 'V'},
		{64, 'W'},
		{72, 'X'},
		{83.9, 'X'},
	} {
		got, ok := bandFor(tc.lat)
		if !ok {
			t.Fatalf("bandFor(%v) refused a valid latitude", tc.lat)
		}
		if got != tc.want {
			t.Errorf("bandFor(%v) = %c, want %c", tc.lat, got, tc.want)
		}
	}

	// I and O are absent from the sequence because they read as 1 and 0.
	for _, letter := range []byte{'I', 'O', 'A', 'B', 'Y', 'Z'} {
		if bandIndex(letter) >= 0 {
			t.Errorf("%c is a band letter, and must not be", letter)
		}
	}

	// X is the one band that is not eight degrees tall.
	south, north, _ := bandLatitudes('X')
	if south != 72 || north != maxUTMLat {
		t.Errorf("band X spans %v to %v, want 72 to %v", south, north, maxUTMLat)
	}
}

// Above 84 north and below 80 south the grid is Universal Polar Stereographic,
// which is a different system with a different notation. Declining is the whole
// behavior: approximating a polar position onto a UTM zone would produce a
// reference that no polar tool would accept back.
func TestPolarLatitudesAreDeclined(t *testing.T) {
	for _, lat := range []float64{84, 84.1, -80.1, 89, -90, math.NaN()} {
		if _, ok := bandFor(lat); ok {
			t.Errorf("bandFor(%v) accepted a latitude outside the UTM span", lat)
		}
		if _, ok := projectUTM(lat, 0); ok {
			t.Errorf("projectUTM(%v, 0) accepted a latitude outside the UTM span", lat)
		}
	}
}

// The 100 km letter scheme, checked on its published structure rather than on
// this code's own output: three column sets repeating every three zones, and a
// five-letter row stagger on even zones.
func TestGridSquareLetterScheme(t *testing.T) {
	// The three published column sets, written out. The previous version of
	// this compared gridSquareSetCols[(zone-1)%3] against
	// gridSquareSetCols[(zone+2)%3], which are the same index for every zone,
	// so it could not fail for any data at all: rotating all three sets left it
	// green while twenty other tests went red.
	for i, want := range [3]string{"ABCDEFGH", "JKLMNPQR", "STUVWXYZ"} {
		if gridSquareSetCols[i] != want {
			t.Errorf("column set %d = %q, want %q", i, gridSquareSetCols[i], want)
		}
	}
	if gridSquareRows != "ABCDEFGHJKLMNPQRSTUV" {
		t.Errorf("row letters = %q, want the published sequence", gridSquareRows)
	}

	// Zone 1 takes the first set, and the sets advance by one per zone.
	for zone, want := range map[int]string{1: "ABCDEFGH", 2: "JKLMNPQR", 3: "STUVWXYZ", 4: "ABCDEFGH"} {
		if got := gridSquareSetCols[(zone-1)%3]; got != want {
			t.Errorf("zone %d uses column set %q, want %q", zone, got, want)
		}
	}

	// Neither sequence contains I or O.
	for _, set := range gridSquareSetCols {
		for _, r := range set {
			if r == 'I' || r == 'O' {
				t.Errorf("column set %q contains %c", set, r)
			}
		}
	}
	for _, r := range gridSquareRows {
		if r == 'I' || r == 'O' {
			t.Errorf("row letters contain %c", r)
		}
	}

	// The stagger: the same northing gets row letters five apart in an odd and
	// an even zone. Without it two squares either side of a zone boundary would
	// carry the same pair of letters.
	_, oddRow, _ := gridSquareFor(utmPoint{Zone: 11, Easting: 500000, Northing: 4300000})
	_, evenRow, _ := gridSquareFor(utmPoint{Zone: 12, Easting: 500000, Northing: 4300000})

	// Five letters, written out rather than taken from rowOffsetEvenZone.
	// Applying the constant on both sides of the comparison made the assertion
	// vacuous: setting it to 0, which deletes the stagger entirely, left this
	// test green.
	oddIdx := indexOf(gridSquareRows, oddRow)
	evenIdx := indexOf(gridSquareRows, evenRow)
	if (oddIdx+5)%len(gridSquareRows) != evenIdx {
		t.Errorf("row stagger = %d letters (odd zone %c, even zone %c), want 5",
			(evenIdx-oddIdx+len(gridSquareRows))%len(gridSquareRows), oddRow, evenRow)
	}

	// columnIndex and rowIndex are the inverses the decoder relies on.
	for zone := 1; zone <= 60; zone++ {
		set := gridSquareSetCols[(zone-1)%3]
		for i := range len(set) {
			if got := columnIndex(zone, set[i]); got != i {
				t.Fatalf("columnIndex(%d, %c) = %d, want %d", zone, set[i], got, i)
			}
		}

		// A letter from another zone's set names no square in this one, which
		// is the check the reference implementation in
		// mattermost-plugin-aocanywhere does not do at all.
		other := gridSquareSetCols[zone%3]
		if columnIndex(zone, other[0]) >= 0 {
			t.Fatalf("columnIndex(%d, %c) accepted a letter from another zone's set",
				zone, other[0])
		}
	}
}

func indexOf(s string, b byte) int {
	for i := range len(s) {
		if s[i] == b {
			return i
		}
	}
	return -1
}

// The Washington Monument, whose grid reference is widely published as
// 18S UJ 23xxx 06xxx.
//
// The zone, the band and the two 100 km letters are derivable by hand from the
// published scheme and are the externally checkable part: zone 18 from the
// longitude, band S from the latitude, column U as the third letter of zone
// 18's set for a 300 km easting, row J from the stagger on an even zone. The
// five-digit offsets are a regression pin on this code rather than an external
// vector, which is stated here rather than implied so nobody reads them as
// corroboration.
func TestKnownGridReference(t *testing.T) {
	g, ok := mgrsFor(38.8895, -77.0353, 5)
	if !ok {
		t.Fatal("mgrsFor refused the Washington Monument")
	}

	if g.Zone != 18 || g.Band != 'S' || g.Col != 'U' || g.Row != 'J' {
		t.Fatalf("grid square = %d%c %c%c, want 18S UJ", g.Zone, g.Band, g.Col, g.Row)
	}
	if g.Easting != 23478 || g.Northing != 6483 {
		t.Errorf("offsets = %05d %05d, want 23478 06483", g.Easting, g.Northing)
	}
}

// Encoding a position and decoding the reference must land back in the square
// the position was in, everywhere on the grid and at every resolution.
//
// The assertion is a distance rather than an equality, and the reason is a real
// property of the notation rather than a weakness here. Grid squares are
// defined inside a zone and are clipped where a zone ends, so a square against
// a zone boundary can have its geometric center beyond that boundary, in the
// next zone's grid. Re-encoding such a center legitimately produces a reference
// in the neighboring zone. What must always hold is that the center is inside
// the square, which is what this measures.
func TestGridSquaresDecodeIntoTheSquareTheyName(t *testing.T) {
	for digits := 1; digits <= maxGridDigits; digits++ {
		size := squareMeters(digits)

		for lat := -79.0; lat < maxUTMLat; lat += 1.7 {
			for lon := -179.0; lon < 180; lon += 3.1 {
				g, ok := mgrsFor(lat, lon, digits)
				if !ok {
					t.Fatalf("mgrsFor(%v, %v, %d) refused a valid position", lat, lon, digits)
				}

				centerLat, centerLon, ok := mgrsCenter(g)
				if !ok {
					t.Fatalf("mgrsCenter refused %+v (from %v, %v)", g, lat, lon)
				}

				// The point is somewhere in the square and the answer is its
				// center, so they cannot be further apart than half a diagonal.
				meters := math.Hypot(
					(centerLat-lat)*degreeMeters,
					(centerLon-lon)*degreeMeters*math.Cos(lat*math.Pi/180))

				if limit := size * 0.8; meters > limit {
					t.Fatalf("center of %+v is %.1f m from %v, %v, want under %.1f m",
						g, meters, lat, lon, limit)
				}
			}
		}
	}
}

// Away from a zone boundary the stronger property does hold: the center of a
// square re-encodes to exactly the square it came from.
//
// Restricted to points well inside a zone precisely because the general case
// above is not an equality. Asserting it here keeps the exact behavior pinned
// where it is exact, instead of settling for the distance check everywhere.
func TestGridSquaresReEncodeExactlyAwayFromZoneBoundaries(t *testing.T) {
	for digits := 1; digits <= maxGridDigits; digits++ {
		for lat := -78.0; lat < 83; lat += 1.3 {
			for zone := 1; zone <= 60; zone++ {
				// Two degrees either side of the central meridian, which leaves
				// a margin far wider than any square.
				for _, offset := range []float64{-2, -1, 0, 1, 2} {
					lon := centralMeridian(zone) + offset

					// Skip the widened zones. There a point two degrees from
					// one zone's central meridian belongs to a different zone
					// altogether and sits six degrees off that one's, which is
					// the very situation this test exists to stay out of.
					if got, ok := zoneFor(lat, lon); !ok || got != zone {
						continue
					}

					assertReEncodesExactly(t, lat, lon, digits)
				}
			}
		}
	}
}

func assertReEncodesExactly(t *testing.T, lat, lon float64, digits int) {
	t.Helper()

	g, ok := mgrsFor(lat, lon, digits)
	if !ok {
		t.Fatalf("mgrsFor(%v, %v, %d) refused a valid position", lat, lon, digits)
	}

	centerLat, centerLon, ok := mgrsCenter(g)
	if !ok {
		t.Fatalf("mgrsCenter refused %+v", g)
	}

	again, ok := mgrsFor(centerLat, centerLon, digits)
	if !ok {
		t.Fatalf("mgrsFor refused the center of %+v", g)
	}
	if again != g {
		t.Fatalf("center of %+v re-encoded to %+v", g, again)
	}
}

// The center of a square is the center, not a corner. A ten kilometer reference
// read as its south-west corner puts a reader seven kilometers from where the
// grid says to look.
func TestGridPositionIsTheCenterOfItsSquare(t *testing.T) {
	g, ok := mgrsFor(38.8895, -77.0353, 1)
	if !ok {
		t.Fatal("mgrsFor refused a valid position")
	}

	lat, lon, ok := mgrsCenter(g)
	if !ok {
		t.Fatal("mgrsCenter refused a square it had just produced")
	}

	p, ok := projectUTM(lat, lon)
	if !ok {
		t.Fatal("projectUTM refused the center of a square")
	}

	// A one-digit reference is a 10 km square, so the center sits 5 km into it
	// on both axes.
	const size = 10000
	for _, tc := range []struct {
		name string
		got  float64
	}{
		{"easting", math.Mod(p.Easting, size)},
		{"northing", math.Mod(p.Northing, size)},
	} {
		if math.Abs(tc.got-size/2) > 1 {
			t.Errorf("%s offset within the square = %.1f m, want %d m", tc.name, tc.got, size/2)
		}
	}
}

// UTM names a point rather than a square, which is why it round-trips to the
// meter rather than to a cell.
//
// The position is asserted everywhere; the components are asserted everywhere
// except within a meter of a band boundary. A northing written to the meter and
// read back can land a fraction of a meter on the other side of the line, which
// changes the band letter and nothing else. utmPointOf allows exactly that
// meter of slop when reading a token, so the behavior is consistent; what
// cannot be asserted is that the letter comes back identical.
func TestUTMRoundTripsToTheMeter(t *testing.T) {
	for lat := -79.0; lat < maxUTMLat; lat += 2.3 {
		for lon := -179.0; lon < 180; lon += 5.7 {
			g, ok := utmFor(lat, lon, 1)
			if !ok {
				t.Fatalf("utmFor(%v, %v) refused a valid position", lat, lon)
			}

			gotLat, gotLon, ok := utmPointOf(g)
			if !ok {
				t.Fatalf("utmPointOf refused %+v", g)
			}

			meters := math.Hypot(
				(gotLat-lat)*degreeMeters,
				(gotLon-lon)*degreeMeters*math.Cos(lat*math.Pi/180))
			if meters > 1.5 {
				t.Fatalf("%v, %v round-tripped through %+v to %.3f m away", lat, lon, g, meters)
			}

			if onBandBoundary(lat) {
				continue
			}

			again, ok := utmFor(gotLat, gotLon, 1)
			if !ok {
				t.Fatalf("utmFor refused the position of %+v", g)
			}
			if again != g {
				t.Fatalf("%+v round-tripped to %+v", g, again)
			}
		}
	}
}

// onBandBoundary reports whether a latitude is within a meter of the line
// between two bands.
func onBandBoundary(lat float64) bool {
	const meter = 1 / degreeMeters

	for _, edge := range []float64{minUTMLat, maxUTMLat, 72} {
		if math.Abs(lat-edge) < meter {
			return true
		}
	}
	return math.Abs(math.Mod(lat+80, 8)) < meter || math.Abs(math.Mod(lat+80, 8)-8) < meter
}

// A grid square whose 100 km letters do not belong to its zone names nothing,
// and must be refused rather than silently resolved to whatever the arithmetic
// produces.
func TestImpossibleGridSquaresAreRefused(t *testing.T) {
	valid, ok := mgrsFor(38.8895, -77.0353, 3)
	if !ok {
		t.Fatal("mgrsFor refused a valid position")
	}

	for _, tc := range []struct {
		name string
		with func(Grid) Grid
	}{
		{"a column letter from another zone's set", func(g Grid) Grid { g.Col = 'A'; return g }},
		{"a row letter that is not one", func(g Grid) Grid { g.Row = 'I'; return g }},
		{"a band letter that is not one", func(g Grid) Grid { g.Band = 'O'; return g }},
		{"a zone below the range", func(g Grid) Grid { g.Zone = 0; return g }},
		{"a zone above the range", func(g Grid) Grid { g.Zone = 61; return g }},
		{"a band the square cannot reach", func(g Grid) Grid { g.Band = 'C'; return g }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, _, ok := mgrsCenter(tc.with(valid)); ok {
				t.Fatal("mgrsCenter accepted a square that names nothing")
			}
		})
	}
}

// An independent implementation of the same projection, for cross-checking.
//
// This is the Krueger n-series (Karney 2011), and it is a genuinely different
// formulation from the Snyder e-series in geodesy.go: it works through the
// conformal latitude and hyperbolic functions and shares not one term with the
// code it checks. Two implementations that disagree cannot both be right, and
// two that agree to a fraction of a millimeter over the whole grid are not both
// wrong in the same way by accident.
//
// This exists because the fixture tests above genuinely cannot see the
// high-order terms. Measured: swapping the 600c and 330e'^2 coefficients in the
// A^6 northing term moves the Washington Monument by 9 micrometers, and
// deleting the t^2 sub-term moves it by 1. Only a point six degrees off its
// central meridian feels them at all, and then by about 2.6 mm, which no
// honest fixture tolerance would catch. So the check has to be against another
// series rather than against a number.
func kruegerProject(lat, lon float64, zone int) (easting, northing float64) {
	const (
		a  = wgs84A
		k0 = utmK0
	)

	f := 1 / wgs84InvF
	n := f / (2 - f)
	n2, n3, n4 := n*n, n*n*n, n*n*n*n

	bigA := a / (1 + n) * (1 + n2/4 + n4/64)

	alpha := [5]float64{
		0,
		n/2 - 2*n2/3 + 5*n3/16 + 41*n4/180,
		13*n2/48 - 3*n3/5 + 557*n4/1440,
		61*n3/240 - 103*n4/140,
		49561 * n4 / 161280,
	}

	phi := lat * math.Pi / 180
	lam := (lon - centralMeridian(zone)) * math.Pi / 180

	e := 2 * math.Sqrt(n) / (1 + n)
	sinPhi := math.Sin(phi)
	t := math.Sinh(math.Atanh(sinPhi) - e*math.Atanh(e*sinPhi))

	xi := math.Atan(t / math.Cos(lam))
	eta := math.Atanh(math.Sin(lam) / math.Sqrt(1+t*t))

	sumXi, sumEta := xi, eta
	for j := 1; j <= 4; j++ {
		jf := float64(j)
		sumXi += alpha[j] * math.Sin(2*jf*xi) * math.Cosh(2*jf*eta)
		sumEta += alpha[j] * math.Cos(2*jf*xi) * math.Sinh(2*jf*eta)
	}

	easting = utmFalseEasting + k0*bigA*sumEta
	northing = k0 * bigA * sumXi
	if lat < 0 {
		northing += utmFalseNorthing
	}
	return easting, northing
}

func TestProjectionAgreesWithAnIndependentSeries(t *testing.T) {
	var worst float64
	var worstAt string

	for lat := -79.5; lat < maxUTMLat; lat += 0.5 {
		for lon := -179.5; lon < 180; lon += 0.5 {
			got, ok := projectUTM(lat, lon)
			if !ok {
				t.Fatalf("projectUTM refused %v, %v", lat, lon)
			}

			wantE, wantN := kruegerProject(lat, lon, got.Zone)
			d := math.Hypot(got.Easting-wantE, got.Northing-wantN)
			if d > worst {
				worst, worstAt = d, fmt.Sprintf("%.2f, %.2f", lat, lon)
			}
		}
	}

	// A millimeter and a half. The floor is not the tolerance but the two
	// series' own truncation difference, measured at 1.07 mm, so this is as
	// tight as the method allows.
	//
	// What it catches, verified by mutation: a dropped sub-term, a wrong
	// divisor and a mistyped meridian-arc coefficient all fail here. What it
	// cannot catch is one coefficient PAIR swap in the A^6 northing term, whose
	// worst effect is 0.65 mm and which actually moves this figure slightly
	// DOWN, so no threshold would see it. That is a real limit and it is stated
	// rather than papered over; the consolation is that 0.65 mm is a thousand
	// times finer than the finest square this package can express.
	if worst > 0.0015 {
		t.Fatalf("worst disagreement with the Krueger series = %.6f m at %s, want under 1.5 mm",
			worst, worstAt)
	}

	t.Logf("worst disagreement with an independent series: %.6f m at %s", worst, worstAt)
}

func TestBandHelpersRefuseWhatIsNotABand(t *testing.T) {
	if b, ok := bandFor(-81); ok {
		t.Errorf("bandFor(-81) = %q, want a refusal below the band table", b)
	}
	for _, letter := range []byte{'I', 'O', 'A', 'Y', '0'} {
		if south, north, ok := bandLatitudes(letter); ok {
			t.Errorf("bandLatitudes(%q) = %v, %v, want a refusal", letter, south, north)
		}
		if withinBand(letter, 0, 0) {
			t.Errorf("withinBand(%q, 0, 0) accepted a letter that names no band", letter)
		}
	}
}

func TestZoneForRefusesALongitudeOffTheGraticule(t *testing.T) {
	for _, lon := range []float64{-180.1, 180, 360} {
		if zone, ok := zoneFor(0, lon); ok {
			t.Errorf("zoneFor(0, %v) = %d, want a refusal", lon, zone)
		}
	}
}

func TestProjectUTMRefusesALatitudeTheGridDoesNotCover(t *testing.T) {
	for _, lat := range []float64{-81, 85, 90} {
		if p, ok := projectUTM(lat, 0); ok {
			t.Errorf("projectUTM(%v, 0) = %v, want a refusal", lat, p)
		}
	}
}

func TestUnprojectUTMRefusesANonFinitePosition(t *testing.T) {
	for name, easting := range map[string]float64{
		"not a number": math.NaN(),
		"infinite":     math.Inf(1),
		"far off grid": 1e9,
	} {
		if lat, lon, ok := unprojectUTM(utmPoint{Zone: 31, Band: 'U', Easting: easting, Northing: 5000000}); ok {
			t.Errorf("%s easting unprojected to %v, %v, want a refusal", name, lat, lon)
		}
	}
}
