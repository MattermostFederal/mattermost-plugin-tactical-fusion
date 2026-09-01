package geojson

import (
	"math"
	"strconv"
)

// earthRadiusMeters is the IUGG mean radius, which is what a spherical model
// takes when it is not pretending to be an ellipsoid.
//
// The location package projects on WGS 84 because a coordinate has to
// round-trip; nothing here does. A length and an area are read to judge whether
// a route is ten kilometers or a hundred, and the sphere is within about 0.5%
// of the ellipsoid at every latitude, which is far inside what the rendering
// below claims. Using wgs84A here would be a precision this does not have.
const earthRadiusMeters = 6371008.8

// Measurement is what a feature's geometry works out to, already rendered.
//
// Rendered in Go rather than handed over as numbers, for the reason property
// values are: the card and the panel both show it, and two renderers rounding
// the same figure separately is how the two surfaces come to disagree.
type Measurement struct {
	Length string
	Area   string
}

// measure works out what a geometry can honestly be measured as.
//
// Lines get a length, areas get an area, and a point gets neither. A
// GeometryCollection gets both when its members provide both, since it is one
// feature and a reader asking "how big is this" means the whole of it.
//
// A feature the parse noted is not measured at all. The note says this build
// will not stand behind the shape, and a figure printed beside that would be
// standing behind it.
func measure(feature Feature) Measurement {
	if feature.Note != "" {
		return Measurement{}
	}

	var meters, square float64

	for _, part := range feature.Geometry.Parts {
		switch part.Kind {
		case KindLineString, KindMultiLine:
			for _, ring := range part.Rings {
				length, ok := ringLength(ring)
				if !ok {
					return Measurement{}
				}
				meters += length
			}

		case KindPolygon, KindMultiPoly:
			area, ok := polygonArea(part)
			if !ok {
				return Measurement{}
			}
			square += area
		}
	}

	return Measurement{Length: lengthText(meters), Area: areaText(square)}
}

// polygonArea is the exterior ring's area less its holes.
//
// RingCounts is what makes a MultiPolygon work here: its rings are every ring
// of every member polygon in one list, so without the boundaries the second
// member's exterior would be subtracted as though it were a hole in the first.
func polygonArea(part Part) (float64, bool) {
	counts := part.RingCounts
	if len(counts) == 0 {
		counts = []int{len(part.Rings)}
	}

	total, at := 0.0, 0
	for _, count := range counts {
		for i := 0; i < count && at < len(part.Rings); i++ {
			area, ok := ringArea(part.Rings[at])
			if !ok {
				return 0, false
			}
			if i == 0 {
				total += area
			} else {
				total -= area
			}
			at++
		}
	}

	if total < 0 {
		return 0, true
	}

	return total, true
}

// ringLength is the great-circle distance along a ring, in meters.
// The bool is "every lexeme parsed". A ring this cannot read suppresses the
// whole measurement rather than contributing zero, because a figure computed
// from part of a shape is a figure standing behind the part it could read.
func ringLength(ring Ring) (float64, bool) {
	total := 0.0

	for i := 1; i < len(ring); i++ {
		from, ok := radians(ring[i-1])
		if !ok {
			return 0, false
		}
		to, ok := radians(ring[i])
		if !ok {
			return 0, false
		}
		total += haversine(from, to)
	}

	return total, true
}

// ringArea is the spherical excess of a closed ring, in square meters.
//
// The signed area is taken and its magnitude returned, so winding order does
// not decide whether an area is positive. RFC 7946 states a winding rule and
// this build neither checks nor normalizes it, so relying on it here would make
// the figure depend on something nothing enforces.
func ringArea(ring Ring) (float64, bool) {
	if len(ring) < 4 {
		return 0, true
	}

	total := 0.0

	for i := 0; i < len(ring)-1; i++ {
		from, ok := radians(ring[i])
		if !ok {
			return 0, false
		}
		to, ok := radians(ring[i+1])
		if !ok {
			return 0, false
		}

		// Normalized to the short way round. A raw difference makes a two
		// degree step across the antimeridian read as 358 degrees, and a small
		// box straddling the line measured 179 times its real area. Measured
		// against the same box moved off the line.
		total += math.Remainder(to.lon-from.lon, 2*math.Pi) *
			(2 + math.Sin(from.lat) + math.Sin(to.lat))
	}

	return math.Abs(total) * earthRadiusMeters * earthRadiusMeters / 2, true
}

type point struct{ lat, lon float64 }

func radians(p Position) (point, bool) {
	lat, latErr := strconv.ParseFloat(p.Lat, 64)
	lon, lonErr := strconv.ParseFloat(p.Lon, 64)

	if latErr != nil || lonErr != nil || math.IsNaN(lat) || math.IsNaN(lon) {
		return point{}, false
	}

	return point{lat: lat * math.Pi / 180, lon: lon * math.Pi / 180}, true
}

func haversine(from, to point) float64 {
	dLat := to.lat - from.lat
	dLon := to.lon - from.lon

	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(from.lat)*math.Cos(to.lat)*math.Sin(dLon/2)*math.Sin(dLon/2)

	return 2 * earthRadiusMeters * math.Asin(math.Min(1, math.Sqrt(a)))
}

/*
 * Rendering, and the precision it claims.
 *
 * Three significant figures and no more. The sphere is within about half a
 * percent of the ellipsoid, the coordinates themselves are written to whatever
 * resolution the author chose, and a route measured off a map is not a surveyed
 * distance. Printing 12.4174 km would be claiming a precision that none of
 * those three support, so the figure is cut to what it can carry.
 */
func lengthText(meters float64) string {
	if !(meters > 0) || math.IsInf(meters, 0) {
		return ""
	}

	if meters < 1000 {
		return trim(round3(meters)) + " m"
	}

	return trim(round3(meters/1000)) + " km"
}

func areaText(square float64) string {
	if !(square > 0) || math.IsInf(square, 0) {
		return ""
	}

	if square < 1e6 {
		return trim(round3(square)) + " m²"
	}

	return trim(round3(square/1e6)) + " km²"
}

// round3 keeps three significant figures.
func round3(value float64) float64 {
	if value == 0 {
		return 0
	}

	magnitude := math.Pow(10, 2-math.Floor(math.Log10(math.Abs(value))))
	if math.IsInf(magnitude, 0) {
		return value
	}

	return math.Round(value*magnitude) / magnitude
}

func trim(value float64) string {
	return strconv.FormatFloat(value, 'f', -1, 64)
}
