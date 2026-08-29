package geojson

import (
	"errors"
	"strings"
	"testing"
)

func parse(t *testing.T, source string) *Document {
	t.Helper()

	document, err := Parse([]byte(source))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	return document
}

func TestGeoJSONIsLongitudeFirst(t *testing.T) {
	// Longitude 100 is legal, latitude 100 is not, so a transposed reader
	// cannot pass this: it would refuse the first and accept the second.
	document := parse(t, `{"type":"Point","coordinates":[100,45]}`)

	position := document.Features[0].Geometry.Parts[0].Rings[0][0]
	if position.Lon != "100" || position.Lat != "45" {
		t.Fatalf("lon=%q lat=%q, want lon=100 lat=45", position.Lon, position.Lat)
	}

	transposed := parse(t, `{"type":"Point","coordinates":[45,100]}`)
	if transposed.Features[0].Note != PositionUnusableNote {
		t.Fatalf("a latitude of 100 was accepted: note=%q", transposed.Features[0].Note)
	}
}

func TestPartCardinalityMatchesTheTable(t *testing.T) {
	// The cross-language shape guard runs both sides over one fixture, so it
	// agrees by construction whichever encoding is chosen and cannot catch a
	// wrong one. This is the test that can.
	cases := []struct {
		name       string
		source     string
		kind       Kind
		parts      int
		ringsFirst int
		posFirst   int
	}{
		{"Point", `{"type":"Point","coordinates":[1,2]}`, KindPoint, 1, 1, 1},
		{"MultiPoint", `{"type":"MultiPoint","coordinates":[[1,2],[3,4],[5,6]]}`, KindMultiPoint, 3, 1, 1},
		{"LineString", `{"type":"LineString","coordinates":[[1,2],[3,4],[5,6]]}`, KindLineString, 1, 1, 3},
		{"MultiLineString", `{"type":"MultiLineString","coordinates":[[[1,2],[3,4]],[[5,6],[7,8]]]}`, KindMultiLine, 2, 1, 2},
		{"Polygon", fixturePolygon, KindPolygon, 1, 2, 5},
		{"MultiPolygon", `{"type":"MultiPolygon","coordinates":[` + fixtureRing + `,` + fixtureRing + `]}`, KindMultiPoly, 1, 2, 4},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			geometry := parse(t, testCase.source).Features[0].Geometry

			if geometry.Kind != testCase.kind {
				t.Fatalf("kind=%q want %q", geometry.Kind, testCase.kind)
			}
			if len(geometry.Parts) != testCase.parts {
				t.Fatalf("parts=%d want %d", len(geometry.Parts), testCase.parts)
			}
			if len(geometry.Parts[0].Rings) != testCase.ringsFirst {
				t.Fatalf("rings=%d want %d", len(geometry.Parts[0].Rings), testCase.ringsFirst)
			}
			if len(geometry.Parts[0].Rings[0]) != testCase.posFirst {
				t.Fatalf("positions=%d want %d", len(geometry.Parts[0].Rings[0]), testCase.posFirst)
			}
		})
	}
}

func TestMultiLineStringKeepsItsLinesApart(t *testing.T) {
	// One part of N positions would join two disjoint lines into one polyline,
	// which is the whole reason the parts level exists.
	geometry := parse(t, `{"type":"MultiLineString","coordinates":[[[1,2],[3,4]],[[50,51],[52,53]]]}`).Features[0].Geometry

	if len(geometry.Parts) != 2 {
		t.Fatalf("parts=%d, want 2", len(geometry.Parts))
	}
	if geometry.Parts[0].Rings[0][0].Lon != "1" || geometry.Parts[1].Rings[0][0].Lon != "50" {
		t.Fatal("the two lines were merged")
	}
}

func TestPolygonKeepsRingZeroAsTheExterior(t *testing.T) {
	rings := parse(t, fixturePolygon).Features[0].Geometry.Parts[0].Rings

	if len(rings) != 2 {
		t.Fatalf("rings=%d, want 2", len(rings))
	}
	if len(rings[0]) <= len(rings[1]) {
		t.Fatal("ring 0 is not the exterior")
	}
}

func TestNestedMultiPolygonKeepsItsGrouping(t *testing.T) {
	// The one case parts alone cannot express: flattening to P parts of kind
	// Polygon loses which rings belonged to which member polygon.
	source := `{"type":"GeometryCollection","geometries":[{"type":"MultiPolygon","coordinates":[` +
		fixtureRing + `,` + fixtureRing + `]}]}`

	geometry := parse(t, source).Features[0].Geometry

	if len(geometry.Parts) != 1 {
		t.Fatalf("parts=%d, want 1", len(geometry.Parts))
	}
	part := geometry.Parts[0]
	if part.Kind != KindMultiPoly {
		t.Fatalf("kind=%q, want %q", part.Kind, KindMultiPoly)
	}
	if len(part.RingCounts) != 2 || part.RingCounts[0] != 1 || part.RingCounts[1] != 1 {
		t.Fatalf("ring_counts=%v, want [1 1]", part.RingCounts)
	}
}

func TestNullGeometryIsUnlocatedRatherThanAnError(t *testing.T) {
	// RFC 7946 section 3.2 makes it legal.
	feature := parse(t, `{"type":"Feature","geometry":null,"properties":{}}`).Features[0]

	if feature.Geometry.Kind != KindNone {
		t.Fatalf("kind=%q, want %q", feature.Geometry.Kind, KindNone)
	}
	if feature.Note != UnlocatedNote {
		t.Fatalf("note=%q", feature.Note)
	}
}

func TestExponentAndLongDecimalCoordinatesAreDrawn(t *testing.T) {
	// json.dumps writes 1e-05 for 0.00001, and ogr2ogr sits at fifteen
	// decimals. Porting cot's decimalShape would refuse both.
	for _, lexeme := range []string{"1e-05", "1E-05", "-118.250000000000001", "0.000010"} {
		document := parse(t, `{"type":"Point","coordinates":[`+lexeme+`,34.05]}`)

		if note := document.Features[0].Note; note != "" {
			t.Fatalf("%s was refused: %q", lexeme, note)
		}
		if got := document.Features[0].Geometry.Parts[0].Rings[0][0].Lon; got != lexeme {
			t.Fatalf("lexeme %q was rewritten to %q", lexeme, got)
		}
	}
}

func TestUnreasonableCoordinatesAreNotDrawn(t *testing.T) {
	cases := map[string]string{
		"out of range":  "999",
		"huge exponent": "1e99999",
		"many zeros":    "0." + strings.Repeat("0", 60000),
	}

	for name, lexeme := range cases {
		t.Run(name, func(t *testing.T) {
			document := parse(t, `{"type":"Point","coordinates":[`+lexeme+`,34.05]}`)
			if document.Features[0].Note != PositionUnusableNote {
				t.Fatalf("note=%q", document.Features[0].Note)
			}
		})
	}
}

func TestRepeatedKeysAreFirstWins(t *testing.T) {
	// Matches cot's parser, which records the rule and why it is one rule.
	document := parse(t, `{"type":"Point","type":"Polygon","coordinates":[1,2]}`)

	if document.Features[0].Geometry.Kind != KindPoint {
		t.Fatalf("kind=%q, want the first", document.Features[0].Geometry.Kind)
	}
}

func TestForeignMembersAreSkipped(t *testing.T) {
	// RFC 7946 section 6.1 permits them anywhere.
	parse(t, `{"type":"Point","coordinates":[1,2],"whatever":{"deep":[1,2,3]},"style":"x"}`)
}

func TestCapsRefuseTheWholeDocument(t *testing.T) {
	cases := []struct {
		name   string
		source string
		want   error
	}{
		{"too large", `{"type":"Point","coordinates":[1,2],"pad":"` + strings.Repeat("x", MaxSourceBytes) + `"}`, ErrTooLarge},
		{"unknown type", `{"type":"Topology","objects":{}}`, ErrUnknownType},
		{"not an object", `[1,2,3]`, ErrNotGeoJSON},
		{"trailing content", `{"type":"Point","coordinates":[1,2]} {}`, ErrTrailing},
		{"nested collection", `{"type":"GeometryCollection","geometries":[{"type":"GeometryCollection","geometries":[]}]}`, ErrNestedCollection},
		{"too many features", manyFeatures(MaxFeatures + 1), ErrTooManyFeatures},
		{"too many vertices", manyVertices(MaxVertices + 1), ErrTooManyVertices},
		{"too deep in coordinates", `{"type":"Point","coordinates":` + nested(MaxJSONDepth+2) + `}`, ErrTooDeep},
		{"too deep in properties", `{"type":"Feature","geometry":null,"properties":{"a":` + nested(MaxJSONDepth+2) + `}}`, ErrTooDeep},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			_, err := Parse([]byte(testCase.source))
			if !errors.Is(err, testCase.want) {
				t.Fatalf("err=%v, want %v", err, testCase.want)
			}
		})
	}
}

func TestInvalidUTF8IsRefused(t *testing.T) {
	if _, err := Parse([]byte(`{"type":"Point","coordinates":[1,2],"a":"` + "\xff" + `"}`)); !errors.Is(err, ErrNotUTF8) {
		t.Fatalf("err=%v, want %v", err, ErrNotUTF8)
	}
}

func TestForeignCRSIsReadButNotDrawn(t *testing.T) {
	source := `{"type":"FeatureCollection","crs":{"type":"name","properties":{"name":"EPSG:4326"}},` +
		`"features":[{"type":"Feature","geometry":{"type":"Point","coordinates":[1,2]},"properties":{}}]}`

	document := parse(t, source)

	if len(document.Features) != 1 {
		t.Fatalf("features=%d, want 1", len(document.Features))
	}
	if document.Note != ForeignCRSNote {
		t.Fatalf("note=%q", document.Note)
	}
}

func TestCRS84IsDrawn(t *testing.T) {
	source := `{"type":"FeatureCollection","crs":{"type":"name","properties":{"name":"urn:ogc:def:crs:OGC:1.3:CRS84"}},` +
		`"features":[{"type":"Feature","geometry":{"type":"Point","coordinates":[1,2]},"properties":{}}]}`

	if note := parse(t, source).Note; note != "" {
		t.Fatalf("note=%q, want none", note)
	}
}

func TestMalformedBoundingBoxIsNoted(t *testing.T) {
	source := `{"type":"FeatureCollection","bbox":[1,2,3],` +
		`"features":[{"type":"Feature","geometry":{"type":"Point","coordinates":[1,2]},"properties":{}}]}`

	if note := parse(t, source).Note; note != BadBoxNote {
		t.Fatalf("note=%q", note)
	}
}

func TestEmptyCollectionIsLegal(t *testing.T) {
	document := parse(t, `{"type":"FeatureCollection","features":[]}`)

	if len(document.Features) != 0 {
		t.Fatalf("features=%d, want 0", len(document.Features))
	}
	if document.Note != NoFeaturesNote {
		t.Fatalf("note=%q", document.Note)
	}
}

func TestOpenAndShortRingsAreNotDrawn(t *testing.T) {
	open := `{"type":"Polygon","coordinates":[[[0,0],[1,0],[1,1],[0,1]]]}`
	if note := parse(t, open).Features[0].Note; note != RingOpenNote {
		t.Fatalf("open ring note=%q", note)
	}

	short := `{"type":"Polygon","coordinates":[[[0,0],[1,0],[0,0]]]}`
	if note := parse(t, short).Features[0].Note; note != RingShortNote {
		t.Fatalf("short ring note=%q", note)
	}
}

func TestAltitudeIsKept(t *testing.T) {
	position := parse(t, `{"type":"Point","coordinates":[1,2,300.5]}`).Features[0].Geometry.Parts[0].Rings[0][0]

	if position.Alt != "300.5" {
		t.Fatalf("alt=%q", position.Alt)
	}
}

func manyFeatures(n int) string {
	parts := make([]string, 0, n)
	for range n {
		parts = append(parts, `{"type":"Feature","geometry":null,"properties":{}}`)
	}
	return `{"type":"FeatureCollection","features":[` + strings.Join(parts, ",") + `]}`
}

func manyVertices(n int) string {
	parts := make([]string, 0, n)
	for range n {
		parts = append(parts, `[1,2]`)
	}
	return `{"type":"MultiPoint","coordinates":[` + strings.Join(parts, ",") + `]}`
}

func nested(depth int) string {
	return strings.Repeat("[", depth) + strings.Repeat("]", depth)
}

/*
 * The vertex cap counts what is DRAWN, not what looks like a coordinate.
 *
 * The count used to run in the walker, which sees structure and not meaning. It
 * charged number pairs in a properties bag against the geometry budget, and it
 * missed a bare Point entirely, because `coordinates` is an object member
 * rather than an element of an array.
 */
func TestTheVertexCapCountsGeometryAndNotProperties(t *testing.T) {
	pairs := make([]string, 0, MaxVertices+50)
	for range MaxVertices + 50 {
		pairs = append(pairs, `[1,2]`)
	}

	source := `{"type":"Feature","geometry":{"type":"Point","coordinates":[1,2]},` +
		`"properties":{"series":[` + strings.Join(pairs, ",") + `]}}`

	if len(source) > MaxSourceBytes {
		t.Skipf("fixture is %d bytes, over the source cap", len(source))
	}

	document, err := Parse([]byte(source))
	if err != nil {
		t.Fatalf("a properties bag of number pairs was charged against the geometry budget: %v", err)
	}
	if got := document.Features[0].Geometry.Kind; got != KindPoint {
		t.Fatalf("kind = %q", got)
	}
}

func TestTheVertexCapCountsALonePointsCoordinates(t *testing.T) {
	// One position per feature, past the cap. Every one is an object member
	// rather than an array element, which the structural counter never saw.
	features := make([]string, 0, MaxVertices+10)
	for range MaxVertices + 10 {
		features = append(features, `{"type":"Feature","geometry":{"type":"Point","coordinates":[1,2]},"properties":{}}`)
	}
	source := `{"type":"FeatureCollection","features":[` + strings.Join(features, ",") + `]}`

	// MaxFeatures bites first here, which is itself a refusal. What must NOT
	// happen is silent acceptance, so either sentinel is correct.
	if _, err := Parse([]byte(source)); err == nil {
		t.Fatal("a document past both caps was accepted")
	}
}

// A root GeometryCollection was the one way past MaxFeatures entirely: its
// members were uncapped while Counts reported a single feature.
func TestACollectionIsCappedLikeAFeatureCollection(t *testing.T) {
	members := make([]string, 0, MaxFeatures+1)
	for range MaxFeatures + 1 {
		members = append(members, `{"type":"Point","coordinates":[1,2]}`)
	}
	source := `{"type":"GeometryCollection","geometries":[` + strings.Join(members, ",") + `]}`

	if _, err := Parse([]byte(source)); !errors.Is(err, ErrTooManyFeatures) {
		t.Fatalf("err = %v, want %v", err, ErrTooManyFeatures)
	}
}

// A polygon with no rings drew and measured nothing while counting as a polygon.
func TestAPolygonWithNoRingsIsNoted(t *testing.T) {
	if note := parse(t, `{"type":"Polygon","coordinates":[]}`).Features[0].Note; note != RingShortNote {
		t.Fatalf("note = %q, want %q", note, RingShortNote)
	}
}

// An altitude has no range, but it does have the same lexeme contract as a
// coordinate: readAltitude used to be a second copy that dropped the bound.
func TestAnAltitudeKeepsTheCoordinateLexemeContract(t *testing.T) {
	if note := parse(t, `{"type":"Point","coordinates":[1,2,1e400]}`).Features[0].Note; note != PositionUnusableNote {
		t.Fatalf("an unreadable altitude was accepted: note = %q", note)
	}

	document := parse(t, `{"type":"Point","coordinates":[1,2,30000]}`)
	if got := document.Features[0].Geometry.Parts[0].Rings[0][0].Alt; got != "30000" {
		t.Fatalf("alt = %q, want 30000", got)
	}
}

/*
 * Every way a stated bounding box can be unreadable.
 *
 * The existing test covers the wrong LENGTH. The element checks were never
 * executed, and a real GIS export is exactly where a null or a stringified
 * number in a bbox comes from. The box is never used for anything, so the whole
 * point is the note: the reader is told the document said something this build
 * could not read, and the features are still drawn from their own coordinates.
 */
func TestABoundingBoxThisBuildCannotReadIsNoted(t *testing.T) {
	const feature = `"features":[{"type":"Feature","geometry":null,"properties":{}}]}`

	for _, tc := range []struct {
		name string
		box  string
		want string
	}{
		{"a string where a number belongs", `[1,2,"3",4]`, BadBoxNote},
		{"a null", `[1,2,null,4]`, BadBoxNote},
		{"a nested array", `[1,2,[3],4]`, BadBoxNote},
		{"a number too large to be finite", `[1,2,1e400,4]`, BadBoxNote},
		{"the wrong length", `[1,2,3]`, BadBoxNote},
		{"not an array at all", `"1,2,3,4"`, BadBoxNote},

		// The two shapes RFC 7946 permits, which must NOT be noted: four
		// numbers in two dimensions and six in three.
		{"four numbers", `[1,2,3,4]`, ""},
		{"six numbers", `[1,2,3,4,5,6]`, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			source := `{"type":"FeatureCollection","bbox":` + tc.box + `,` + feature

			if note := parse(t, source).Note; note != tc.want {
				t.Errorf("note = %q, want %q", note, tc.want)
			}
		})
	}
}
