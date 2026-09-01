package geojson

import (
	"encoding/json"
	"math"
	"strconv"
	"unicode/utf8"
)

/*
 * budget counts the positions a document actually draws.
 *
 * Counted HERE rather than in the walker, and the difference is the point. The
 * walker sees structure and not meaning, so a structural test counted every
 * two-number array anywhere as a position: it charged a properties bag full of
 * number pairs against the geometry budget, and it missed a bare Point's
 * `coordinates` entirely, because that is an object member rather than an
 * element of an array. Counting where a position becomes part of a geometry is
 * the only place the count means what the cap says it means.
 */
type budget struct{ vertices int }

func (b *budget) position() error {
	b.vertices++
	if b.vertices > MaxVertices {
		return ErrTooManyVertices
	}
	return nil
}

// buildGeometry reads one geometry into parts, at the cardinality
// docs/design/geojson.md tables.
//
// Every geometry becomes a list of parts, every part a list of rings, every
// ring a list of positions. MultiPoint and MultiLineString are N parts rather
// than one part of N positions, so a multi-part geometry is distinguishable
// from a single-part one at the parts level and not only by its kind: one part
// of N positions would join two disjoint lines into one polyline.
func buildGeometry(geometry object, depth int, b *budget) (Geometry, string, error) {
	kind, ok := geometry.string("type")
	if !ok || !geometryKinds[kind] {
		return Geometry{}, "", ErrNotGeoJSON
	}

	if kind == KindCollection {
		if depth >= maxCollectionDepth {
			return Geometry{}, "", ErrNestedCollection
		}
		return buildCollection(geometry, depth, b)
	}

	raw, present := geometry["coordinates"]
	if !present {
		return Geometry{}, "", ErrNotGeoJSON
	}

	parts, note, err := coordinateParts(kind, raw, b)
	if err != nil {
		return Geometry{}, "", err
	}

	return Geometry{Kind: kind, Parts: parts}, note, nil
}

func buildCollection(geometry object, depth int, b *budget) (Geometry, string, error) {
	members, ok := geometry["geometries"].([]any)
	if !ok {
		return Geometry{}, "", ErrNotGeoJSON
	}

	// The same cap a FeatureCollection's features get. Without it a root
	// GeometryCollection was the one way past MaxFeatures entirely: 1500 members
	// were accepted while Counts reported one feature, which is the "quietly
	// wrong about what was posted" outcome every other refusal exists to stop.
	if len(members) > MaxFeatures {
		return Geometry{}, "", ErrTooManyFeatures
	}

	built := Geometry{Kind: KindCollection}
	note := ""

	for _, raw := range members {
		member, ok := raw.(object)
		if !ok {
			return Geometry{}, "", ErrNotGeoJSON
		}

		inner, innerNote, err := buildGeometry(member, depth+1, b)
		if err != nil {
			return Geometry{}, "", err
		}
		if innerNote != "" && note == "" {
			note = innerNote
		}

		// A MultiPolygon member stays ONE part carrying its own kind, so its
		// grouping survives. Flattening it to P parts of kind Polygon is the
		// one case parts alone cannot express, which is what RingCounts is for.
		built.Parts = append(built.Parts, inner.Parts...)
	}

	return built, note, nil
}

func coordinateParts(kind Kind, raw any, b *budget) ([]Part, string, error) {
	switch kind {
	case KindPoint:
		position, ok := readPosition(raw)
		if !ok {
			return nil, PositionUnusableNote, nil
		}
		if err := b.position(); err != nil {
			return nil, "", err
		}
		return []Part{{Kind: kind, Rings: []Ring{{position}}}}, "", nil

	case KindMultiPoint:
		items, ok := raw.([]any)
		if !ok {
			return nil, "", ErrNotGeoJSON
		}
		parts := make([]Part, 0, len(items))
		for _, item := range items {
			position, ok := readPosition(item)
			if !ok {
				return nil, PositionUnusableNote, nil
			}
			if err := b.position(); err != nil {
				return nil, "", err
			}
			parts = append(parts, Part{Kind: kind, Rings: []Ring{{position}}})
		}
		return parts, "", nil

	case KindLineString:
		ring, note, err := readRing(raw, b)
		if err != nil || note != "" {
			return nil, note, err
		}
		if len(ring) < minLinePositions {
			return nil, LineShortNote, nil
		}
		return []Part{{Kind: kind, Rings: []Ring{ring}}}, "", nil

	case KindMultiLine:
		items, ok := raw.([]any)
		if !ok {
			return nil, "", ErrNotGeoJSON
		}
		parts := make([]Part, 0, len(items))
		for _, item := range items {
			ring, note, err := readRing(item, b)
			if err != nil || note != "" {
				return nil, note, err
			}
			if len(ring) < minLinePositions {
				return nil, LineShortNote, nil
			}
			parts = append(parts, Part{Kind: kind, Rings: []Ring{ring}})
		}
		return parts, "", nil

	case KindPolygon:
		rings, note, err := readPolygon(raw, b)
		if err != nil || note != "" {
			return nil, note, err
		}
		return []Part{{Kind: kind, Rings: rings}}, "", nil

	case KindMultiPoly:
		items, ok := raw.([]any)
		if !ok {
			return nil, "", ErrNotGeoJSON
		}
		part := Part{Kind: kind}
		for _, item := range items {
			rings, note, err := readPolygon(item, b)
			if err != nil || note != "" {
				return nil, note, err
			}
			part.Rings = append(part.Rings, rings...)
			part.RingCounts = append(part.RingCounts, len(rings))
		}
		return []Part{part}, "", nil
	}

	return nil, "", ErrNotGeoJSON
}

// readPolygon reads one polygon's rings. Ring 0 is the exterior and the rest
// are holes, per RFC 7946 section 3.1.6, and that order is what the map relies
// on to draw a hole as a hole rather than as a solid island.
func readPolygon(raw any, b *budget) ([]Ring, string, error) {
	items, ok := raw.([]any)
	if !ok {
		return nil, "", ErrNotGeoJSON
	}

	// A polygon with no rings draws and measures nothing, so it was counted as
	// a polygon and then silently absent from the map.
	if len(items) == 0 {
		return nil, RingShortNote, nil
	}

	rings := make([]Ring, 0, len(items))

	for _, item := range items {
		ring, note, err := readRing(item, b)
		if err != nil || note != "" {
			return nil, note, err
		}
		if len(ring) < minRingPositions {
			return nil, RingShortNote, nil
		}
		if !ringIsClosed(ring) {
			return nil, RingOpenNote, nil
		}
		rings = append(rings, ring)
	}

	return rings, "", nil
}

func ringIsClosed(ring Ring) bool {
	first, last := ring[0], ring[len(ring)-1]
	return first == last
}

func readRing(raw any, b *budget) (Ring, string, error) {
	items, ok := raw.([]any)
	if !ok {
		return nil, "", ErrNotGeoJSON
	}

	ring := make(Ring, 0, len(items))

	for _, item := range items {
		position, ok := readPosition(item)
		if !ok {
			return nil, PositionUnusableNote, nil
		}
		if err := b.position(); err != nil {
			return nil, "", err
		}
		ring = append(ring, position)
	}

	return ring, "", nil
}

// readPosition reads [lon, lat] or [lon, lat, alt].
//
// Longitude FIRST, per RFC 7946 section 3.1.1. TestGeoJSONIsLongitudeFirst is
// the guard, and it is named for the invariant because transposing the pair is
// the classic defect in every GeoJSON reader.
func readPosition(raw any) (Position, bool) {
	items, ok := raw.([]any)
	if !ok || len(items) < 2 || len(items) > 3 {
		return Position{}, false
	}

	lon, ok := readCoord(items[0], 180)
	if !ok {
		return Position{}, false
	}

	lat, ok := readCoord(items[1], 90)
	if !ok {
		return Position{}, false
	}

	position := Position{Lon: lon, Lat: lat}

	if len(items) == 3 {
		// No range: an altitude is not a longitude. math.Abs(x) > +Inf is
		// always false, so this is readCoord with the bound removed rather
		// than a second copy of its lexeme contract.
		alt, ok := readCoord(items[2], math.Inf(1))
		if !ok {
			return Position{}, false
		}
		position.Alt = alt
	}

	return position, true
}

// readCoord validates a lexeme and hands it back unchanged.
//
// Deliberately NOT cot's decimalShape. That regex guards an XML attribute,
// where ParseFloat would otherwise accept a hex float, and a JSON number has
// already been through a grammar that admits neither those nor a leading plus.
// Applied here it would instead refuse legal GeoJSON: 1e-05 is what json.dumps
// writes for 0.00001, and sixteen decimals is where ogr2ogr sits. A rune cap
// closes the sixty-thousand-zeros defect the regex was really for, and the
// range test does the rest.
func readCoord(raw any, limit float64) (string, bool) {
	number, ok := raw.(json.Number)
	if !ok {
		return "", false
	}

	lexeme := number.String()
	if utf8.RuneCountInString(lexeme) > maxCoordRunes {
		return "", false
	}

	value, err := strconv.ParseFloat(lexeme, 64)
	if err != nil || math.IsInf(value, 0) || math.IsNaN(value) {
		return "", false
	}

	if math.Abs(value) > limit {
		return "", false
	}

	return lexeme, true
}
