package geojson

import "strings"

// Fixture is a document exercising every kind, every note and every optional
// field, for the cross-language guard.
//
// "Every optional field" includes the simplestyle color: the shape guard's
// reverse direction found `color` uncovered the moment it was implemented,
// because no feature here had stated one.
//
// Built from the kind table rather than hand-listed, so a kind added later
// cannot quietly fall out of the guard's coverage.
// TestFixtureCoversEveryKind holds it to that.
func Fixture() string {
	features := []string{
		feature(`{"type":"Point","coordinates":[-118.250000,34.056100,120]}`, `{"name":"Point feature","marker-color":"#ff8800","marker-size":"large","count":3,"ok":true,"nothing":null,"tags":["a","b"]}`),
		feature(`{"type":"MultiPoint","coordinates":[[-118.25,34.05],[-118.24,34.06]]}`, `{"title":"Multi point"}`),
		feature(`{"type":"LineString","coordinates":[[-118.25,34.05],[-118.24,34.06]]}`, `{"label":"Line"}`),
		feature(`{"type":"MultiLineString","coordinates":[[[-118.25,34.05],[-118.24,34.06]],[[-118.20,34.00],[-118.19,34.01]]]}`, `{}`),
		feature(fixturePolygon, `{"name":"Holed polygon","stroke":"#ff0000","stroke-width":3,"stroke-opacity":0.8,"fill":"#ff0000","fill-opacity":0.25}`),
		feature(`{"type":"MultiPolygon","coordinates":[`+fixtureRing+`,`+fixtureRing+`]}`, `{}`),
		feature(`{"type":"GeometryCollection","geometries":[{"type":"Point","coordinates":[1,2]},{"type":"MultiPolygon","coordinates":[`+fixtureRing+`]}]}`, `{}`),
		feature(`null`, `{"name":"Unlocated"}`),
		feature(`{"type":"Point","coordinates":[999,34.05]}`, `{"name":"Out of range"}`),
	}

	return `{"type":"FeatureCollection","features":[` + strings.Join(features, ",") + `]}`
}

const fixtureRing = `[[[-118.25,34.05],[-118.24,34.05],[-118.24,34.06],[-118.25,34.05]]]`

const fixturePolygon = `{"type":"Polygon","coordinates":[` +
	`[[-118.30,34.00],[-118.10,34.00],[-118.10,34.20],[-118.30,34.20],[-118.30,34.00]],` +
	`[[-118.25,34.05],[-118.20,34.05],[-118.20,34.10],[-118.25,34.05]]` +
	`]}`

func feature(geometry, properties string) string {
	return `{"type":"Feature","geometry":` + geometry + `,"properties":` + properties + `}`
}

// FixtureID is a feature naming itself through its own id rather than through
// any property, which is the fourth rung of the name precedence.
func FixtureID() string {
	return `{"type":"Feature","id":"NAMED-BY-ID","geometry":{"type":"Point","coordinates":[1,2]},"properties":{}}`
}
