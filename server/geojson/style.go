package geojson

import (
	"encoding/json"
	"math"
	"regexp"
	"strings"
	"unicode/utf8"
)

/*
 * simplestyle-spec, the convention every GIS tool writes and no standard
 * defines.
 *
 * RFC 7946 says nothing about styling, so a document's appearance lives in
 * ordinary feature properties under names github.com/mapbox/simplestyle-spec
 * settled on.
 *
 * This build once read only the three that decide a COLOR, on the argument that
 * widths, opacities and symbol names are a styling language and carrying them
 * would make this a renderer of somebody else's stylesheet. That was narrowed
 * on request: a document that says a hazard area is red at a quarter opacity is
 * saying something about the hazard, and dropping it silently redrew the
 * author's meaning in the theme's colors. The line now falls between what this
 * build can draw and what it cannot, not between color and everything else.
 *
 * marker-symbol is the one still ignored, and it is a capability rather than a
 * decision: a symbol name indexes an icon sprite, this plugin ships an offline
 * basemap with generated glyph ranges and no sprite at all, and simplestyle's
 * vocabulary is Maki, which is some two hundred icons with their own license
 * and bundle cost. It stays visible as a property row, which is what every
 * unread key gets.
 */
const (
	styleMarkerColor = "marker-color"
	styleStroke      = "stroke"
	styleFill        = "fill"

	styleStrokeWidth   = "stroke-width"
	styleStrokeOpacity = "stroke-opacity"
	styleFillOpacity   = "fill-opacity"
	styleMarkerSize    = "marker-size"
)

/*
 * What a stated width may be.
 *
 * simplestyle-spec sets no bound, and an unbounded one is a document that can
 * paint over the whole map: a stroke-width of 4000 is a solid screen with no
 * way for a reader to tell it from a rendering fault. Ten device pixels is
 * already a very heavy line at any zoom this build draws.
 *
 * Refused rather than clamped. A clamp draws something the document did not ask
 * for while claiming to honor it, which is the failure this file exists to
 * avoid; a refusal falls back to the theme, which is what an unstyled feature
 * gets and is honestly not the author's width.
 */
const maxStrokeWidth = 10

// markerSizes is simplestyle's whole vocabulary for marker-size. Anything else
// is not a size, and is left to the theme rather than guessed at.
var markerSizes = map[string]bool{"small": true, "medium": true, "large": true}

// MarkerSizes is the catalog, exported so the cross-language guard reads it
// rather than keeping a second copy of the same three words.
func MarkerSizes() map[string]bool {
	out := make(map[string]bool, len(markerSizes))
	for size := range markerSizes {
		out[size] = true
	}

	return out
}

/*
 * Style is everything about a feature's appearance this build will draw.
 *
 * Every field is already validated and rendered to the shape the props blob
 * carries, so nothing downstream re-derives one. Empty means the feature stated
 * nothing this build will stand behind, and the map falls back to the theme,
 * which is the same outcome as stating nothing at all.
 */
type Style struct {
	Color         string
	Width         string
	StrokeOpacity string
	FillOpacity   string
	MarkerSize    string
}

/*
 * hexColor is the whole of what may become a color.
 *
 * Three or six hex digits after a hash, and nothing else. simplestyle-spec
 * permits exactly this, so the check refuses nothing a conforming document
 * writes, and it is the gate that keeps an author string out of a paint
 * property: a value reaching MapLibre is a value the browser will interpret.
 * The webapp validates again before it draws, because a props blob is not a
 * trusted input either, and that second gate is the one the map's own tests
 * hold.
 */
var hexColor = regexp.MustCompile(`^#([0-9a-fA-F]{3}|[0-9a-fA-F]{6})$`)

/*
 * featureStyle is everything a feature asks for that this build can draw.
 *
 * The opacities are read for the geometry they belong to and not otherwise: a
 * point has no fill and a polygon's stroke-opacity is its outline's, so reading
 * both onto everything would have a marker's fill-opacity quietly deciding how
 * solid a marker is. That is the same argument featureColor makes about
 * choosing a color name by kind.
 */
func featureStyle(kind Kind, properties object) Style {
	style := Style{Color: featureColor(kind, properties)}

	switch kind {
	case KindPoint, KindMultiPoint:
		style.MarkerSize = markerSize(properties[styleMarkerSize])
	case KindPolygon, KindMultiPoly:
		style.Width = strokeWidth(properties[styleStrokeWidth])
		style.StrokeOpacity = opacity(properties[styleStrokeOpacity])
		style.FillOpacity = opacity(properties[styleFillOpacity])

	// A collection is every kind at once. markersFor draws its Point parts and
	// shapesFor its rings, so reading only the stroke would have dropped a
	// fill-opacity and a marker-size the document stated and this build draws.
	case KindCollection:
		style.Width = strokeWidth(properties[styleStrokeWidth])
		style.StrokeOpacity = opacity(properties[styleStrokeOpacity])
		style.FillOpacity = opacity(properties[styleFillOpacity])
		style.MarkerSize = markerSize(properties[styleMarkerSize])
	default:
		style.Width = strokeWidth(properties[styleStrokeWidth])
		style.StrokeOpacity = opacity(properties[styleStrokeOpacity])
	}

	return style
}

/*
 * strokeWidth is a stated width in device pixels, or "".
 *
 * A json.Number is kept as its own lexeme rather than reparsed and reprinted,
 * for the reason every other number in this package is: the document's text is
 * what both surfaces show, and two renderers rounding the same figure
 * separately is how they come to disagree.
 */
func strokeWidth(raw any) string {
	value, ok := positiveNumber(raw)
	if !ok || value > maxStrokeWidth {
		return ""
	}

	return numberLexeme(raw)
}

// opacity is a stated 0..1 opacity, or "". Zero is legal and means invisible,
// which is a thing a document may deliberately say about a fill.
func opacity(raw any) string {
	number, ok := raw.(json.Number)
	if !ok {
		return ""
	}

	value, err := number.Float64()
	if err != nil || math.IsNaN(value) || value < 0 || value > 1 {
		return ""
	}

	return numberLexeme(raw)
}

func markerSize(raw any) string {
	text, ok := raw.(string)
	if !ok {
		return ""
	}

	lowered := strings.ToLower(strings.TrimSpace(text))
	if !markerSizes[lowered] {
		return ""
	}

	return lowered
}

func positiveNumber(raw any) (float64, bool) {
	number, ok := raw.(json.Number)
	if !ok {
		return 0, false
	}

	value, err := number.Float64()
	if err != nil || math.IsNaN(value) || value <= 0 {
		return 0, false
	}

	return value, true
}

func numberLexeme(raw any) string {
	number, ok := raw.(json.Number)
	if !ok {
		return ""
	}

	// Refused past the cap, not truncated. sanitize appends a marker, so a
	// number with a three hundred digit mantissa was validated as a width and
	// then stored as "0.000…", which the webapp's own gate cannot read: the
	// server would have said the width was good and shipped a lexeme that is
	// not a number. readCoord refuses for the same reason.
	lexeme := number.String()
	if utf8.RuneCountInString(lexeme) > maxPropertyValRune {
		return ""
	}

	return lexeme
}

// featureColor is the color a feature asks to be drawn in, or "".
//
// One color per feature, because that is what the map draws with: a line takes
// it directly and an area takes it at the fill alpha the theme uses. A document
// that colors its stroke and its fill differently gets the one that matches how
// this build draws that geometry.
//
// The name is chosen by KIND rather than by taking whichever key is present,
// so a point that carries both marker-color and fill is drawn in the one that
// was meant for a marker.
func featureColor(kind Kind, properties object) string {
	var order []string

	switch kind {
	case KindPoint, KindMultiPoint:
		order = []string{styleMarkerColor, styleStroke, styleFill}
	case KindPolygon, KindMultiPoly:
		order = []string{styleFill, styleStroke, styleMarkerColor}
	default:
		order = []string{styleStroke, styleMarkerColor, styleFill}
	}

	for _, name := range order {
		if color, ok := normalizeColor(properties[name]); ok {
			return color
		}
	}

	return ""
}

// normalizeColor expands a three-digit color to six and lowercases it, so the
// webapp's own gate meets one shape rather than four.
func normalizeColor(raw any) (string, bool) {
	text, ok := raw.(string)
	if !ok {
		return "", false
	}

	trimmed := strings.TrimSpace(text)
	if !hexColor.MatchString(trimmed) {
		return "", false
	}

	lowered := strings.ToLower(trimmed)
	if len(lowered) == 7 {
		return lowered, true
	}

	// #abc means #aabbcc.
	var expanded strings.Builder
	expanded.WriteByte('#')
	for i := 1; i < 4; i++ {
		expanded.WriteByte(lowered[i])
		expanded.WriteByte(lowered[i])
	}

	return expanded.String(), true
}
