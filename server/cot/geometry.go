package cot

import (
	"encoding/xml"
	"math"
	"strings"
)

const (
	shapeElement    = "shape"
	polylineElement = "polyline"
	ellipseElement  = "ellipse"
	vertexElement   = "vertex"

	shapeChildDepth = 4
	vertexDepth     = 5

	routePointAttr = "point"
	closedAttr     = "closed"

	GeometryTooLargeNote = "This build does not draw a shape with this many points, so it is not on the map. The event is unchanged under \"As posted\"."
	GeometryUnusableNote = "A point in this shape is not one this build will stand behind, so the shape is not drawn."
)

const maxCotVertices = MaxVertices

// MaxVertices is exported so the tests that measure the budget can build the
// largest shape this build draws rather than guessing at it.
const MaxVertices = 512

const (
	GeometryPolyline = "polyline"
	GeometryEllipse  = "ellipse"
	GeometryRoute    = "route"
)

type Vertex struct {
	Lat, Lon string
}

type Geometry struct {
	Kind     string
	Closed   bool
	Vertices []Vertex

	// Seen is what the event carried, which is not len(Vertices) once a cap or
	// a refusal has stopped the list growing.
	Seen int

	Major, Minor, Angle string

	// Undrawable is why this shape is not drawn, or "" when it is.
	Undrawable string
}

func (g *Geometry) empty() bool { return g.Kind == "" }

func (g *Geometry) addVertex(lat, lon string) {
	g.Seen++

	if !usableCoord(lat, lon) {
		g.Undrawable = GeometryUnusableNote
		return
	}
	if len(g.Vertices) >= maxCotVertices {
		g.Undrawable = GeometryTooLargeNote
		return
	}

	g.Vertices = append(g.Vertices, Vertex{Lat: lat, Lon: lon})
}

func usableCoord(lat, lon string) bool {
	latitude, latOK := decimalNumber(lat)
	longitude, lonOK := decimalNumber(lon)

	return latOK && lonOK && math.Abs(latitude) <= 90 && math.Abs(longitude) <= 180
}

// readShapeChild reports whether it recognised the element, which is what the
// caller marks accepted on.
func readShapeChild(geometry *Geometry, start xml.StartElement) bool {
	if !geometry.empty() {
		return false
	}

	switch start.Name.Local {
	case polylineElement:
		geometry.Kind = GeometryPolyline
		geometry.Closed = truthy(attrValue(start, closedAttr))
		return true

	case ellipseElement:
		geometry.Kind = GeometryEllipse
		geometry.Major = attrValue(start, "major")
		geometry.Minor = attrValue(start, "minor")
		geometry.Angle = attrValue(start, "angle")
		return true
	}

	return false
}

// routeVertex reads a link written as a route point, or reports that this link
// is not one and should be read as a relation instead.
func routeVertex(raw string) (Vertex, bool) {
	parts := strings.Split(raw, ",")
	if len(parts) < 2 || len(parts) > 3 {
		return Vertex{}, false
	}

	vertex := Vertex{Lat: parts[0], Lon: parts[1]}
	if !usableCoord(vertex.Lat, vertex.Lon) {
		return Vertex{}, false
	}

	return vertex, true
}

func addRouteVertex(geometry *Geometry, vertex Vertex) {
	if geometry.empty() {
		geometry.Kind = GeometryRoute
	}
	geometry.addVertex(vertex.Lat, vertex.Lon)
}

func truthy(raw string) bool {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "true", "1", "yes":
		return true
	}
	return false
}

// FixtureGeometry is a shape, for the cross-language guard's event.
func FixtureGeometry() string {
	return `<shape><polyline closed="true">` +
		`<vertex lat="34.0561" lon="-118.2500"/>` +
		`<vertex lat="34.0600" lon="-118.2400"/>` +
		`<vertex lat="34.0700" lon="-118.2600"/>` +
		`</polyline></shape>`
}
