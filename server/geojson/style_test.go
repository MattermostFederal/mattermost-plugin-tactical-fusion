package geojson

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func styleOf(t *testing.T, source string) Style {
	t.Helper()
	return parse(t, source).Features[0].Style
}

func colorOf(t *testing.T, source string) string {
	t.Helper()
	return parse(t, source).Features[0].Style.Color
}

// styled wraps a geometry in a feature carrying the given properties. Named
// apart from fixture.go's own `feature`, which builds the shared fixture.
func styled(geometry, properties string) string {
	return `{"type":"Feature","geometry":` + geometry + `,"properties":` + properties + `}`
}

func TestAValidatedColorIsKept(t *testing.T) {
	got := colorOf(t, styled(`{"type":"Point","coordinates":[1,2]}`, `{"marker-color":"#FF0000"}`))
	if got != "#ff0000" {
		t.Fatalf("color = %q, want #ff0000", got)
	}
}

// simplestyle-spec permits three digits, so refusing them would refuse a
// conforming document.
func TestAThreeDigitColorIsExpanded(t *testing.T) {
	got := colorOf(t, styled(`{"type":"Point","coordinates":[1,2]}`, `{"marker-color":"#abc"}`))
	if got != "#aabbcc" {
		t.Fatalf("color = %q, want #aabbcc", got)
	}
}

/*
 * The gate. A value reaching MapLibre is a value the browser will interpret, so
 * anything that is not a hex triple carries no color at all rather than being
 * passed along for somebody else to reject.
 */
func TestAnythingThatIsNotAHexTripleCarriesNoColor(t *testing.T) {
	for _, raw := range []string{
		`"url(https://attacker.example/px)"`,
		`"red"`,
		`"rgb(255,0,0)"`,
		`"#ff00"`,
		`"#gggggg"`,
		`"javascript:alert(1)"`,
		`"#ff0000; background:url(x)"`,
		`""`,
		`7`,
		`null`,
		`{"r":255}`,
	} {
		source := styled(`{"type":"Point","coordinates":[1,2]}`, `{"marker-color":`+raw+`}`)
		if got := colorOf(t, source); got != "" {
			t.Errorf("marker-color %s became %q, want no color", raw, got)
		}
	}
}

// Chosen by kind, so a point that carries both is drawn in the one meant for a
// marker.
func TestTheColorIsChosenByGeometryKind(t *testing.T) {
	cases := []struct {
		name     string
		geometry string
		want     string
	}{
		{"a point prefers marker-color", `{"type":"Point","coordinates":[1,2]}`, "#111111"},
		{"a polygon prefers fill", fixturePolygon, "#333333"},
		{"a line prefers stroke", `{"type":"LineString","coordinates":[[0,0],[1,1]]}`, "#222222"},
	}

	props := `{"marker-color":"#111111","stroke":"#222222","fill":"#333333"}`

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			if got := colorOf(t, styled(testCase.geometry, props)); got != testCase.want {
				t.Fatalf("color = %q, want %q", got, testCase.want)
			}
		})
	}
}

// A document that states nothing carries nothing, and the map falls back to the
// theme rather than to a color this build invented.
func TestNoStatedColorMeansNoColor(t *testing.T) {
	if got := colorOf(t, styled(`{"type":"Point","coordinates":[1,2]}`, `{"name":"x"}`)); got != "" {
		t.Fatalf("color = %q, want none", got)
	}
}

// The color still travels in the props blob, and only when there is one.
func TestOnlyAColoredFeatureCarriesTheKey(t *testing.T) {
	colored := Props(parse(t, styled(`{"type":"Point","coordinates":[1,2]}`, `{"marker-color":"#abc"}`)), Source{Kind: SourceFence})
	if colored["features"].([]any)[0].(map[string]any)["color"] != "#aabbcc" {
		t.Error("a colored feature lost its color in props")
	}

	plain := Props(parse(t, styled(`{"type":"Point","coordinates":[1,2]}`, `{}`)), Source{Kind: SourceFence})
	if _, present := plain["features"].([]any)[0].(map[string]any)["color"]; present {
		t.Error("an uncolored feature carries a color key")
	}
}

const polygonGeometry = `{"type":"Polygon","coordinates":[[[0,0],[1,0],[1,1],[0,1],[0,0]]]}`

const lineGeometry = `{"type":"LineString","coordinates":[[0,0],[1,1]]}`

func TestAPolygonKeepsEveryStyleItCanDraw(t *testing.T) {
	got := styleOf(t, styled(polygonGeometry, `{
		"stroke": "#ff0000", "stroke-width": 3, "stroke-opacity": 0.8,
		"fill": "#ff0000", "fill-opacity": 0.25
	}`))

	want := Style{Color: "#ff0000", Width: "3", StrokeOpacity: "0.8", FillOpacity: "0.25"}
	if got != want {
		t.Fatalf("style = %+v, want %+v", got, want)
	}
}

// The lexeme the document wrote, not a reprint of it. Both surfaces show these
// figures, and two renderers rounding one separately is how they disagree.
func TestAWidthKeepsTheDocumentsOwnLexeme(t *testing.T) {
	got := styleOf(t, styled(lineGeometry, `{"stroke-width": 2.50}`))
	if got.Width != "2.50" {
		t.Fatalf("width = %q, want the lexeme 2.50", got.Width)
	}
}

/*
 * Refused rather than clamped, for the reason maxStrokeWidth records: a clamp
 * draws something the document did not ask for while claiming to honor it.
 */
func TestAWidthThisBuildWillNotDrawIsRefused(t *testing.T) {
	for _, value := range []string{"0", "-2", "4000", "1e9", `"3"`, "null"} {
		got := styleOf(t, styled(lineGeometry, `{"stroke-width": `+value+`}`))
		if got.Width != "" {
			t.Errorf("stroke-width %s became %q", value, got.Width)
		}
	}
}

func TestAnOpacityOutsideZeroToOneIsRefused(t *testing.T) {
	for _, value := range []string{"-0.1", "1.5", "2", `"0.5"`, "null"} {
		got := styleOf(t, styled(polygonGeometry, `{"fill-opacity": `+value+`}`))
		if got.FillOpacity != "" {
			t.Errorf("fill-opacity %s became %q", value, got.FillOpacity)
		}
	}
}

// Zero is legal and is not "unset": a document may deliberately say a fill is
// invisible, leaving an outline with nothing inside it.
func TestAZeroOpacityIsAStatedValue(t *testing.T) {
	got := styleOf(t, styled(polygonGeometry, `{"fill-opacity": 0}`))
	if got.FillOpacity != "0" {
		t.Fatalf("fill-opacity = %q, want the stated 0", got.FillOpacity)
	}
}

func TestOnlySimplestylesOwnMarkerSizesAreKept(t *testing.T) {
	for raw, want := range map[string]string{
		`"small"`: "small", `"MEDIUM"`: "medium", `" large "`: "large",
		`"huge"`: "", `"3"`: "", `3`: "", `null`: "",
	} {
		got := styleOf(t, styled(`{"type":"Point","coordinates":[1,2]}`, `{"marker-size": `+raw+`}`))
		if got.MarkerSize != want {
			t.Errorf("marker-size %s = %q, want %q", raw, got.MarkerSize, want)
		}
	}
}

/*
 * A style is read for the geometry it belongs to, and not otherwise.
 *
 * A point has no fill and no outline width, so reading them onto one would have
 * a marker's fill-opacity quietly deciding how solid a marker is. This is the
 * argument featureColor already makes about choosing a color name by kind.
 */
func TestAStyleIsReadForItsOwnGeometry(t *testing.T) {
	point := styleOf(t, styled(`{"type":"Point","coordinates":[1,2]}`,
		`{"marker-color":"#00ff00","stroke-width":4,"fill-opacity":0.5,"marker-size":"large"}`))
	if point.Width != "" || point.FillOpacity != "" {
		t.Errorf("a point took an outline width or a fill opacity: %+v", point)
	}
	if point.MarkerSize != "large" || point.Color != "#00ff00" {
		t.Errorf("a point lost its own style: %+v", point)
	}

	line := styleOf(t, styled(lineGeometry,
		`{"stroke":"#00ff00","stroke-width":4,"fill-opacity":0.5,"marker-size":"large"}`))
	if line.FillOpacity != "" || line.MarkerSize != "" {
		t.Errorf("a line took a fill opacity or a marker size: %+v", line)
	}
	if line.Width != "4" {
		t.Errorf("a line lost its stroke width: %+v", line)
	}
}

// A capability, not a decision: a symbol name indexes a sprite this offline
// basemap does not ship. It stays visible as an ordinary property row.
func TestAMarkerSymbolIsNotDrawn(t *testing.T) {
	document := parse(t, styled(`{"type":"Point","coordinates":[1,2]}`,
		`{"marker-symbol":"airport"}`))

	named := false
	for _, property := range document.Features[0].Properties {
		if property.Key == "marker-symbol" && property.Value == "airport" {
			named = true
		}
	}
	if !named {
		t.Error("marker-symbol is not drawn and is not listed either, so it is simply lost")
	}
}

/*
 * A numeric lexeme past the property cap is refused, not truncated.
 *
 * sanitize appends a marker, so a stroke-width with a three hundred digit
 * mantissa was validated as a width (it parses, and it is inside the range) and
 * then stored as "0.000…", which is not a number. The server would have said
 * the width was good and shipped a lexeme the webapp's own gate cannot read.
 * readCoord refuses an over-long coordinate for the same reason.
 */
func TestAnOverLongNumberIsRefusedRatherThanTruncated(t *testing.T) {
	long := "1." + strings.Repeat("0", maxPropertyValRune) + "1"
	if utf8.RuneCountInString(long) <= maxPropertyValRune {
		t.Fatalf("the fixture is not over the cap: %d runes", utf8.RuneCountInString(long))
	}

	got := styleOf(t, styled(lineGeometry, `{"stroke-width": `+long+`}`))
	if got.Width != "" {
		t.Errorf("width = %q, want none: a truncated number is not a number", got.Width)
	}
	if strings.Contains(got.Width, truncationMarker) {
		t.Errorf("width carries the truncation marker: %q", got.Width)
	}

	// And a width inside the cap still round-trips its own lexeme, so the
	// refusal above is the cap rather than the whole path going dark.
	if kept := styleOf(t, styled(lineGeometry, `{"stroke-width": 2.50}`)); kept.Width != "2.50" {
		t.Errorf("an ordinary width = %q, want 2.50", kept.Width)
	}
}
