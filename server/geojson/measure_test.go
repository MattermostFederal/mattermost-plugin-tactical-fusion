package geojson

import (
	"math"
	"strings"
	"testing"
)

// within reports whether got is inside tolerance percent of want.
func within(got, want, tolerancePercent float64) bool {
	if want == 0 {
		return got == 0
	}
	return math.Abs(got-want)/math.Abs(want)*100 <= tolerancePercent
}

// A degree of latitude is about 111.19 km on this sphere, everywhere.
//
// Checked against a figure that does not come from this package, so the test
// cannot agree with the code by sharing its mistake.
func TestRingLengthMatchesADegreeOfLatitude(t *testing.T) {
	ring := Ring{{Lon: "0", Lat: "0"}, {Lon: "0", Lat: "1"}}

	got, _ := ringLength(ring)
	if !within(got, 111194.9, 0.1) {
		t.Fatalf("a degree of latitude measured %.1f m, want about 111195", got)
	}
}

// London to Paris is about 344 km, which is the distance every haversine
// example in the world is checked against.
func TestRingLengthMatchesAKnownCityPair(t *testing.T) {
	ring := Ring{
		{Lon: "-0.1278", Lat: "51.5074"},
		{Lon: "2.3522", Lat: "48.8566"},
	}

	got, _ := ringLength(ring)
	if !within(got, 343900, 1) {
		t.Fatalf("London to Paris measured %.0f m, want about 343900", got)
	}
}

// A one degree square at the equator is about 12,308 km².
func TestRingAreaMatchesADegreeSquare(t *testing.T) {
	ring := Ring{
		{Lon: "0", Lat: "0"},
		{Lon: "1", Lat: "0"},
		{Lon: "1", Lat: "1"},
		{Lon: "0", Lat: "1"},
		{Lon: "0", Lat: "0"},
	}

	got, _ := ringArea(ring)
	if !within(got, 1.23e10, 1) {
		t.Fatalf("a degree square measured %.3e m², want about 1.23e10", got)
	}
}

// Winding order is neither checked nor normalized by this build, so an area
// that depended on it would depend on something nothing enforces.
func TestRingAreaIgnoresWindingOrder(t *testing.T) {
	clockwise := Ring{
		{Lon: "0", Lat: "0"},
		{Lon: "0", Lat: "1"},
		{Lon: "1", Lat: "1"},
		{Lon: "1", Lat: "0"},
		{Lon: "0", Lat: "0"},
	}
	counter := Ring{
		{Lon: "0", Lat: "0"},
		{Lon: "1", Lat: "0"},
		{Lon: "1", Lat: "1"},
		{Lon: "0", Lat: "1"},
		{Lon: "0", Lat: "0"},
	}

	cw, _ := ringArea(clockwise)
	ccw, _ := ringArea(counter)
	if !within(cw, ccw, 0.001) {
		t.Fatalf("winding changed the area: %.3e vs %.3e", cw, ccw)
	}
}

// A hole is subtracted from the ring that encloses it.
func TestAHoleIsSubtractedFromItsExterior(t *testing.T) {
	document := parse(t, fixturePolygon)

	holed := measure(document.Features[0])
	if holed.Area == "" {
		t.Fatal("a polygon measured no area")
	}

	// The same exterior with no hole must measure larger.
	solid := parse(t, `{"type":"Polygon","coordinates":[`+
		`[[-118.30,34.00],[-118.10,34.00],[-118.10,34.20],[-118.30,34.20],[-118.30,34.00]]]}`)

	if measure(solid.Features[0]).Area == holed.Area {
		t.Fatal("the hole was not subtracted")
	}
}

// A MultiPolygon's rings arrive as one list, so without RingCounts the second
// member's exterior would be subtracted as though it were a hole in the first.
func TestAMultiPolygonSumsItsMembersRatherThanSubtractingThem(t *testing.T) {
	one := parse(t, `{"type":"MultiPolygon","coordinates":[`+fixtureRing+`]}`)
	two := parse(t, `{"type":"MultiPolygon","coordinates":[`+fixtureRing+`,`+fixtureRing+`]}`)

	single, _ := polygonArea(one.Features[0].Geometry.Parts[0])
	double, _ := polygonArea(two.Features[0].Geometry.Parts[0])

	if !within(double, single*2, 0.001) {
		t.Fatalf("two identical members measured %.3e, want twice %.3e", double, single)
	}
}

func TestOnlyTheGeometriesThatHaveOneAreMeasured(t *testing.T) {
	cases := []struct {
		name         string
		source       string
		length, area bool
	}{
		{"a point has neither", `{"type":"Point","coordinates":[1,2]}`, false, false},
		{"a line has a length", `{"type":"LineString","coordinates":[[0,0],[0,1]]}`, true, false},
		{"a polygon has an area", fixturePolygon, false, true},
		{"a multi point has neither", `{"type":"MultiPoint","coordinates":[[1,2],[3,4]]}`, false, false},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			m := measure(parse(t, testCase.source).Features[0])

			if (m.Length != "") != testCase.length {
				t.Errorf("length = %q, want present=%v", m.Length, testCase.length)
			}
			if (m.Area != "") != testCase.area {
				t.Errorf("area = %q, want present=%v", m.Area, testCase.area)
			}
		})
	}
}

// A feature this build will not stand behind gets no figure. Printing one
// beside "not drawn" would be standing behind it.
func TestANotedFeatureIsNotMeasured(t *testing.T) {
	document := parse(t, `{"type":"Polygon","coordinates":[[[0,0],[1,0],[1,1],[0,1]]]}`)

	if document.Features[0].Note == "" {
		t.Fatal("the fixture was expected to carry a note")
	}
	if m := measure(document.Features[0]); m.Area != "" || m.Length != "" {
		t.Fatalf("a noted feature measured %+v", m)
	}
}

// Three significant figures and no more: the sphere, the author's own
// resolution and a route measured off a map do not support more.
func TestMeasurementsClaimThreeSignificantFigures(t *testing.T) {
	cases := map[float64]string{
		0:       "",
		999:     "999 m",
		1234:    "1.23 km",
		12345.6: "12.3 km",
		999999:  "1000 km",
	}

	for meters, want := range cases {
		if got := lengthText(meters); got != want {
			t.Errorf("lengthText(%v) = %q, want %q", meters, got, want)
		}
	}

	if got := areaText(1.5e7); got != "15 km²" {
		t.Errorf("areaText(1.5e7) = %q, want %q", got, "15 km²")
	}
	if got := areaText(250); got != "250 m²" {
		t.Errorf("areaText(250) = %q, want %q", got, "250 m²")
	}
}

func TestAMeasurementNeverRunsAway(t *testing.T) {
	for _, value := range []float64{math.Inf(1), math.NaN(), -5} {
		if got := lengthText(value); got != "" {
			t.Errorf("lengthText(%v) = %q, want empty", value, got)
		}
		if got := areaText(value); got != "" {
			t.Errorf("areaText(%v) = %q, want empty", value, got)
		}
	}
}

// A collection is one feature, so "how big is this" means the whole of it.
func TestACollectionMeasuresEveryMemberItHas(t *testing.T) {
	source := `{"type":"GeometryCollection","geometries":[` +
		`{"type":"LineString","coordinates":[[0,0],[0,1]]},` +
		`{"type":"Polygon","coordinates":` + fixtureRing + `}]}`

	m := measure(parse(t, source).Features[0])

	if m.Length == "" || m.Area == "" {
		t.Fatalf("a collection of a line and a polygon measured %+v", m)
	}
	if !strings.HasSuffix(m.Length, "km") && !strings.HasSuffix(m.Length, "m") {
		t.Fatalf("length reads %q", m.Length)
	}
}

// A ring crossing the antimeridian is two degrees wide, not 358.
//
// The raw longitude difference made a small box straddling the line measure 179
// times its real area, and the figure is written permanently into post props.
func TestRingAreaCrossesTheAntimeridianCorrectly(t *testing.T) {
	offLine := Ring{
		{Lon: "10", Lat: "0"},
		{Lon: "12", Lat: "0"},
		{Lon: "12", Lat: "1"},
		{Lon: "10", Lat: "1"},
		{Lon: "10", Lat: "0"},
	}
	onLine := Ring{
		{Lon: "179", Lat: "0"},
		{Lon: "-179", Lat: "0"},
		{Lon: "-179", Lat: "1"},
		{Lon: "179", Lat: "1"},
		{Lon: "179", Lat: "0"},
	}

	off, _ := ringArea(offLine)
	on, _ := ringArea(onLine)

	if !within(on, off, 1) {
		t.Fatalf("the same box measured %.4e on the antimeridian and %.4e off it", on, off)
	}
}

// Length was never affected, because haversine is symmetric in the longitude
// difference. Pinned so a later "fix" does not introduce the asymmetry.
func TestRingLengthCrossesTheAntimeridianCorrectly(t *testing.T) {
	offLine, _ := ringLength(Ring{{Lon: "10", Lat: "0"}, {Lon: "12", Lat: "0"}})
	onLine, _ := ringLength(Ring{{Lon: "179", Lat: "0"}, {Lon: "-179", Lat: "0"}})

	if !within(onLine, offLine, 0.001) {
		t.Fatalf("the same span measured %.0f m on the antimeridian and %.0f m off it", onLine, offLine)
	}
}

/*
 * A hole larger than its exterior reports NO area, never a negative one.
 *
 * Guarded TWICE, which mutation testing is how I found out: `polygonArea`
 * returns 0 for a negative total, and `areaText` independently refuses
 * anything that is not `> 0`. Removing either one alone changes nothing here,
 * so this test pins the OUTCOME rather than either guard, and neither guard can
 * be covered through the public API while the other stands.
 *
 * The outcome is the part that matters: a negative square meter figure on a
 * card would be a measurement the document does not support.
 */
func TestAHoleBiggerThanItsExteriorMeasuresNothing(t *testing.T) {
	document := parse(t, `{"type":"Polygon","coordinates":[`+
		`[[-118.20,34.00],[-118.10,34.00],[-118.10,34.10],[-118.20,34.10],[-118.20,34.00]],`+
		`[[-118.90,33.00],[-117.10,33.00],[-117.10,35.00],[-118.90,35.00],[-118.90,33.00]]`+
		`]}`)

	got := measure(document.Features[0])
	if got.Area != "" {
		t.Errorf("area = %q, want none: the hole is larger than the exterior", got.Area)
	}
	if strings.HasPrefix(got.Area, "-") {
		t.Errorf("area = %q, which is negative", got.Area)
	}
}

/*
 * round3's two guards, which the rendering table never reaches.
 *
 * Zero is its own case because log10(0) is -Inf; a value small enough that
 * 10^(2-floor(log10 v)) overflows to +Inf is the other, and multiplying by that
 * would yield NaN rather than a rounded figure.
 */
func TestRoundingHandlesZeroAndTheVerySmall(t *testing.T) {
	if got := round3(0); got != 0 {
		t.Errorf("round3(0) = %v, want 0", got)
	}

	// 10^(2-floor(log10 v)) overflows for a value near the bottom of float64,
	// and the guard hands the value straight back rather than returning NaN.
	tiny := 5e-308
	got := round3(tiny)
	if math.IsNaN(got) || math.IsInf(got, 0) {
		t.Fatalf("round3(%v) = %v, which is not a number", tiny, got)
	}
	if got != tiny {
		t.Errorf("round3(%v) = %v, want the value unchanged", tiny, got)
	}
}
