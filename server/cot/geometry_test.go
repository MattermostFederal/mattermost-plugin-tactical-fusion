package cot

import (
	"fmt"
	"strings"
	"testing"
)

func geometryOf(t *testing.T, detail string) map[string]any {
	t.Helper()

	props := detailProps(t, detail)
	if props["geometry"] == nil {
		return nil
	}

	geometry, ok := props["geometry"].(map[string]any)
	if !ok {
		t.Fatalf("geometry is %v, want a map", props["geometry"])
	}
	return geometry
}

func TestAPolylineIsReadAsAShape(t *testing.T) {
	geometry := geometryOf(t, `<shape><polyline closed="true">`+
		`<vertex lat="34.0561" lon="-118.2500"/><vertex lat="34.0600" lon="-118.2400"/>`+
		`<vertex lat="34.0700" lon="-118.2600"/></polyline></shape>`)

	if geometry["kind"] != GeometryPolyline {
		t.Errorf("kind is %v, want %s", geometry["kind"], GeometryPolyline)
	}
	if geometry["closed"] != presenceValue {
		t.Errorf("closed is %v; the polyline said it was", geometry["closed"])
	}
	if geometry["count"] != "3" {
		t.Errorf("count is %v, want 3", geometry["count"])
	}

	points := geometry["points"].([]any)
	first := points[0].(map[string]any)
	if first["lat"] != "34.0561" || first["lon"] != "-118.2500" {
		t.Errorf("the first vertex is %v; the digits are the event's own", first)
	}
}

// The order IS the shape, so it survives the trip through props.
func TestAShapeKeepsItsVertexOrder(t *testing.T) {
	geometry := geometryOf(t, `<shape><polyline>`+
		`<vertex lat="1.0000" lon="1.0000"/><vertex lat="2.0000" lon="2.0000"/>`+
		`<vertex lat="3.0000" lon="3.0000"/></polyline></shape>`)

	points := geometry["points"].([]any)
	for i, want := range []string{"1.0000", "2.0000", "3.0000"} {
		if got := points[i].(map[string]any)["lat"]; got != want {
			t.Errorf("vertex %d is %v, want %s", i, got, want)
		}
	}
}

func TestAnEllipseCarriesItsAxes(t *testing.T) {
	geometry := geometryOf(t, `<shape><ellipse major="100" minor="50" angle="-45"/></shape>`)

	checks := map[string]string{"kind": GeometryEllipse, "major": "100 m", "minor": "50 m", "angle": "-45°"}
	for key, want := range checks {
		if geometry[key] != want {
			t.Errorf("%s is %v, want %s", key, geometry[key], want)
		}
	}
	if _, held := geometry["points"]; held {
		t.Error("an ellipse carries no vertex list")
	}
}

// A route's points are links, told apart from relations by carrying a point.
func TestARouteIsReadFromItsLinks(t *testing.T) {
	props := detailProps(t, `<link point="34.0561,-118.2500,10"/><link point="34.0600,-118.2400"/>`+
		`<link uid="ANDROID-88" relation="p-p" parent_callsign="ALPHA"/>`)

	geometry := props["geometry"].(map[string]any)
	if geometry["kind"] != GeometryRoute {
		t.Errorf("kind is %v, want %s", geometry["kind"], GeometryRoute)
	}
	if geometry["count"] != "2" {
		t.Errorf("count is %v, want 2", geometry["count"])
	}

	// And the relation is still a relation.
	if props["parent"] != "ALPHA" {
		t.Errorf("parent is %v; a link with a uid is still a relation", props["parent"])
	}
	if props["related"] != "ANDROID-88" {
		t.Errorf("related is %v", props["related"])
	}
}

// A route's points used to spend the relation budget, so a long route cost the
// reader the row that budget exists to protect.
func TestARouteDoesNotSpendTheRelationBudget(t *testing.T) {
	var links strings.Builder
	for i := range maxCotLinks + 8 {
		fmt.Fprintf(&links, `<link point="1.%04d,2.0000"/>`, i)
	}
	links.WriteString(`<link uid="ANDROID-88" parent_callsign="ALPHA"/>`)

	props := detailProps(t, links.String())

	if props["parent"] != "ALPHA" {
		t.Errorf("parent is %v; the route ate the relation budget", props["parent"])
	}
	if geometry := props["geometry"].(map[string]any); geometry["count"] != fmt.Sprint(maxCotLinks+8) {
		t.Errorf("count is %v, want %d", geometry["count"], maxCotLinks+8)
	}
}

// Past the cap the geometry is not drawn and the event is kept, which is
// neither of the two answers already in this package.
func TestAShapeTooLargeToDrawKeepsTheEvent(t *testing.T) {
	var vertices strings.Builder
	for i := range maxCotVertices + 1 {
		fmt.Fprintf(&vertices, `<vertex lat="1.%04d" lon="2.0000"/>`, i%10000)
	}

	props := detailProps(t, `<contact callsign="DELTA1"/><shape><polyline>`+vertices.String()+`</polyline></shape>`)

	if props["callsign"] != "DELTA1" {
		t.Errorf("callsign is %v; the event was lost over its geometry", props["callsign"])
	}

	geometry := props["geometry"].(map[string]any)
	if _, held := geometry["points"]; held {
		t.Error("a shape past the cap was drawn anyway, ending where the shape does not")
	}
	if geometry["note"] != GeometryTooLargeNote {
		t.Errorf("note is %v", geometry["note"])
	}
}

// A polygon missing a corner is a different polygon, not a partial one.
func TestAShapeWithAnUnusableVertexIsNotDrawn(t *testing.T) {
	props := detailProps(t, `<contact callsign="DELTA1"/><shape><polyline>`+
		`<vertex lat="34.0561" lon="-118.2500"/><vertex lat="0x1p+3" lon="-118.2400"/>`+
		`<vertex lat="34.0700" lon="-118.2600"/></polyline></shape>`)

	if props["callsign"] != "DELTA1" {
		t.Errorf("callsign is %v; the event was lost over one vertex", props["callsign"])
	}

	geometry := props["geometry"].(map[string]any)
	if _, held := geometry["points"]; held {
		t.Error("a shape with a vertex this build will not stand behind was drawn")
	}
	if geometry["note"] != GeometryUnusableNote {
		t.Errorf("note is %v", geometry["note"])
	}
}

// The same gate <point> uses, for the same reason: a value that validates in Go
// and reads as NaN in the browser leaves the card and the picture disagreeing.
func TestAVertexIsHeldToTheDecimalGrammar(t *testing.T) {
	for _, bad := range []string{"0x1p+3", "1e5", "", "north", "34,0561"} {
		props := detailProps(t, `<shape><polyline><vertex lat="`+bad+`" lon="2.0000"/>`+
			`<vertex lat="3.0000" lon="4.0000"/></polyline></shape>`)

		geometry, ok := props["geometry"].(map[string]any)
		if !ok {
			t.Errorf("vertex lat %q left no geometry at all, so nothing says the shape was refused", bad)
			continue
		}
		if _, held := geometry["points"]; held {
			t.Errorf("vertex lat %q was drawn", bad)
		}
		if geometry["note"] != GeometryUnusableNote {
			t.Errorf("vertex lat %q produced note %v, want the refusal", bad, geometry["note"])
		}
	}
}

// A shape needs somewhere the shape can be. One vertex is a point, and the
// event already has one of those.
func TestAShapeOfFewerThanTwoVerticesIsNotGeometry(t *testing.T) {
	props := detailProps(t, `<shape><polyline><vertex lat="1.0000" lon="2.0000"/></polyline></shape>`)

	if held, ok := props["geometry"]; ok {
		t.Errorf("geometry is %v for a one vertex shape", held)
	}
}

// A vertex only counts inside the polyline it belongs to, which is the same
// instance rule the registry's nested children follow.
func TestAVertexOutsideAPolylineIsNotRead(t *testing.T) {
	props := detailProps(t, `<vertex lat="1.0000" lon="2.0000"/>`+
		`<shape><vertex lat="3.0000" lon="4.0000"/></shape>`)

	if held, ok := props["geometry"]; ok {
		t.Errorf("geometry is %v; a loose vertex is not a shape", held)
	}
}

func TestGeometryIsDroppedOnTheDegradedRung(t *testing.T) {
	event, err := parseOne([]byte(eventAround(`<shape><polyline>` +
		`<vertex lat="1.0000" lon="2.0000"/><vertex lat="3.0000" lon="4.0000"/>` +
		`</polyline></shape>`)))
	if err != nil {
		t.Fatalf("fixture did not parse: %v", err)
	}

	if _, held := eventProps(event, true)["geometry"]; !held {
		t.Fatal("the full blob carries no geometry")
	}
	if held, ok := eventProps(event, false)["geometry"]; ok {
		t.Errorf("the degraded blob still carries geometry: %v", held)
	}
}

// A shape and a route are separate geometries.
//
// Sharing one let a junk route point refuse a perfectly good polygon, let a
// route point become a corner of a closed shape, and let links written before a
// shape leave the kind as route and drop `closed`.
func TestAShapeAndARouteDoNotCorruptEachOther(t *testing.T) {
	polygon := `<shape><polyline closed="true">` +
		`<vertex lat="34.0561" lon="-118.2500"/><vertex lat="34.0600" lon="-118.2400"/>` +
		`<vertex lat="34.0700" lon="-118.2600"/></polyline></shape>`

	t.Run("a junk route point does not refuse the shape", func(t *testing.T) {
		props := detailProps(t, `<link point="0x1p+3,2.0" uid="ANDROID-88" parent_callsign="ALPHA"/>`+polygon)

		geometry := props["geometry"].(map[string]any)
		if geometry["note"] != nil {
			t.Errorf("the polygon was refused over a link: %v", geometry["note"])
		}
		if geometry["count"] != "3" {
			t.Errorf("count is %v, want 3", geometry["count"])
		}
		if props["parent"] != "ALPHA" {
			t.Errorf("a link whose point does not parse lost its relation: %v", props["parent"])
		}
	})

	t.Run("a route point is not a corner of the shape", func(t *testing.T) {
		props := detailProps(t, polygon+`<link point="0.0000,0.0000"/>`)

		geometry := props["geometry"].(map[string]any)
		if geometry["count"] != "3" {
			t.Errorf("count is %v; a route point was drawn as a corner", geometry["count"])
		}
	})

	t.Run("links before a shape do not downgrade it", func(t *testing.T) {
		props := detailProps(t, `<link point="1.0000,2.0000"/><link point="3.0000,4.0000"/>`+polygon)

		geometry := props["geometry"].(map[string]any)
		if geometry["kind"] != GeometryPolyline {
			t.Errorf("kind is %v, want %s", geometry["kind"], GeometryPolyline)
		}
		if geometry["closed"] != presenceValue {
			t.Error("the shape lost its closed flag to the links written before it")
		}
		if geometry["count"] != "3" {
			t.Errorf("count is %v, want the shape's own 3", geometry["count"])
		}
	})
}

// Accepted on what the element filled, not on the geometry being non-empty.
//
// Keyed on the latter, a <polyline> after an <ellipse> was marked accepted and
// its vertices were read into the ellipse, so the card said "not drawn" while
// the map drew the ellipse anyway.
func TestAVertexCannotReachAnEllipse(t *testing.T) {
	props := detailProps(t, `<shape><ellipse major="100" minor="50" angle="0"/>`+
		`<polyline><vertex lat="0x1p+3" lon="1.0000"/></polyline></shape>`)

	geometry := props["geometry"].(map[string]any)
	if geometry["kind"] != GeometryEllipse {
		t.Fatalf("kind is %v, want %s", geometry["kind"], GeometryEllipse)
	}
	if geometry["note"] != nil {
		t.Errorf("the ellipse was refused over a vertex in a polyline beside it: %v", geometry["note"])
	}
	if geometry["major"] != "100 m" {
		t.Errorf("major is %v", geometry["major"])
	}
}

// A vertex gets the whole of the gate a position gets, which is a grammar AND a
// range. Only half of it was applied, so a vertex past the poles reached the
// map's fitBounds, which refuses a latitude outside 90.
func TestAVertexOutsideTheEarthIsRefused(t *testing.T) {
	for _, bad := range [][2]string{{"95.0000", "0.0000"}, {"1000.0000", "5000.0000"}, {"1.0000", "181.0000"}} {
		props := detailProps(t, `<shape><polyline><vertex lat="`+bad[0]+`" lon="`+bad[1]+`"/>`+
			`<vertex lat="2.0000" lon="3.0000"/></polyline></shape>`)

		geometry, ok := props["geometry"].(map[string]any)
		if !ok {
			t.Errorf("vertex %v left no geometry", bad)
			continue
		}
		if _, held := geometry["points"]; held {
			t.Errorf("vertex %v was drawn", bad)
		}
	}
}

// The cap is this build's number. Reporting it as the event's said a 900 point
// route carried 512.
func TestTheCountIsWhatTheEventCarried(t *testing.T) {
	var vertices strings.Builder
	for i := range maxCotVertices + 400 {
		fmt.Fprintf(&vertices, `<vertex lat="1.%04d" lon="2.0000"/>`, i%10000)
	}

	props := detailProps(t, `<shape><polyline>`+vertices.String()+`</polyline></shape>`)

	geometry := props["geometry"].(map[string]any)
	if geometry["count"] != fmt.Sprint(maxCotVertices+400) {
		t.Errorf("count is %v, want %d, which is what the event carried", geometry["count"], maxCotVertices+400)
	}
}

// An ellipse is its axes, so one without a usable axis is not a shape. That is
// the vertex list's own rule applied to the other kind.
func TestAnEllipseWithNoAxesIsNotGeometry(t *testing.T) {
	details := []string{
		`<shape><ellipse/></shape>`,
		`<shape><ellipse major="-5" minor="0"/></shape>`,

		// One axis describes no shape either, and the map draws a ring from
		// both, so a single axis is the same refusal.
		`<shape><ellipse major="400"/></shape>`,
		`<shape><ellipse minor="250"/></shape>`,
		`<shape><ellipse major="400" minor="0"/></shape>`,
	}

	for _, detail := range details {
		props := detailProps(t, detail)
		if held, ok := props["geometry"]; ok {
			t.Errorf("%s produced %v", detail, held)
		}
	}
}

// A link whose point this build will not read is still a relation. Returning
// early on the attribute's presence alone threw the relation away.
func TestALinkWithAnUnreadablePointIsStillARelation(t *testing.T) {
	for _, point := range []string{"garbage", "1,", ",,", "1,2,3,4", " 34.0561 , -118.25 "} {
		props := detailProps(t, `<link point="`+point+`" uid="ANDROID-88" parent_callsign="ALPHA"/>`)

		if props["parent"] != "ALPHA" {
			t.Errorf("point %q lost the relation: parent=%v", point, props["parent"])
		}
		if props["related"] != "ANDROID-88" {
			t.Errorf("point %q lost the related uid", point)
		}
	}
}
