package geojson

import "github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/decorators"

const (
	PostType = decorators.PostTypePrefix + "tf_geojson"
	PropsKey = "tactical_fusion_geojson"

	PropsVersion = 1
)

const (
	SourceFence = "fence"
	SourceFile  = "file"
)

type Kind = string

const (
	KindPoint      Kind = "Point"
	KindMultiPoint Kind = "MultiPoint"
	KindLineString Kind = "LineString"
	KindMultiLine  Kind = "MultiLineString"
	KindPolygon    Kind = "Polygon"
	KindMultiPoly  Kind = "MultiPolygon"
	KindCollection Kind = "GeometryCollection"
	KindNone       Kind = "none"
)

// Kinds is every value the kind field may carry, in a fixed order.
//
// Exported for the sync test that holds the webapp's table to it. The card and
// the map both dispatch on this string, so a kind named here and absent there
// is a feature drawn into no channel at all.
var Kinds = []Kind{
	KindPoint,
	KindMultiPoint,
	KindLineString,
	KindMultiLine,
	KindPolygon,
	KindMultiPoly,
	KindCollection,
	KindNone,
}

var geometryKinds = map[string]bool{
	KindPoint:      true,
	KindMultiPoint: true,
	KindLineString: true,
	KindMultiLine:  true,
	KindPolygon:    true,
	KindMultiPoly:  true,
	KindCollection: true,
}

const (
	typeFeature    = "Feature"
	typeCollection = "FeatureCollection"
)

type Source struct {
	Kind     string
	Lead     string
	Trail    string
	Text     string
	FileID   string
	FileName string
}

// Position is one coordinate, kept as the lexemes the document wrote.
//
// Strings rather than float64 because the digits are the reading: re-rendering
// a parsed float pads a resolution the author never claimed. Alt is "" when the
// document wrote a two-element position.
type Position struct {
	Lon, Lat, Alt string
}

// Ring is a list of positions. A LineString has one; a Polygon has one per
// ring, with ring 0 the exterior and the rest holes.
type Ring []Position

// Part is one member of a geometry.
//
// RingCounts is set only on a MultiPolygon part, where it names how many rings
// each member polygon contributed, in order. Rings alone cannot carry that: a
// MultiPolygon nested inside a GeometryCollection has to stay one part to keep
// its grouping, so the boundaries between its polygons are carried rather than
// inferred.
type Part struct {
	Kind       Kind
	Rings      []Ring
	RingCounts []int
}

type Geometry struct {
	Kind  Kind
	Parts []Part
}

type Property struct {
	Key   string
	Value string
}

type Feature struct {
	Name       string
	Geometry   Geometry
	Properties []Property

	// Style is what this feature asked to look like, validated. Every field is
	// empty when it asked for nothing this build will stand behind, and the map
	// then falls back to the theme.
	Style Style

	// Note is why this feature is not drawn, or "" when it is.
	Note string
}

type Document struct {
	Features []Feature

	// Note is what the document says about itself, or "" when it says nothing.
	Note string

	// Placeable is false when NOTHING in the document can be put on a map,
	// which today means a coordinate reference system whose axis order this
	// build cannot confirm.
	//
	// Separate from Note because the two are different questions and the map
	// was reading the wrong one: a malformed bbox carries a note whose own
	// sentence says the features are still drawn from their own coordinates,
	// and blanking the map on any note contradicted the note, the help page and
	// the design note at once.
	Placeable bool
}

// Counts is what the card's summary line states.
type Counts struct {
	Features    int
	Points      int
	Lines       int
	Polygons    int
	Collections int
	Unlocated   int
	Undrawable  int
}

func (d *Document) Counts() Counts {
	counts := Counts{Features: len(d.Features)}

	for _, feature := range d.Features {
		switch feature.Geometry.Kind {
		case KindPoint, KindMultiPoint:
			counts.Points++
		case KindLineString, KindMultiLine:
			counts.Lines++
		case KindPolygon, KindMultiPoly:
			counts.Polygons++
		case KindCollection:
			counts.Collections++
		case KindNone:
			counts.Unlocated++
		}

		if feature.Note != "" && feature.Geometry.Kind != KindNone {
			counts.Undrawable++
		}
	}

	return counts
}
