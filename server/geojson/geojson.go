package geojson

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/decorators/location"
)

const (
	maxFieldRunes = 128
	maxNoteRunes  = 65536

	// maxInlineSrcRunes covers everything Parse read, for the same reason cot's
	// does: the disclosure is what a reader opens to check the card against, so
	// a pane that stopped short of what was parsed could not verify it.
	maxInlineSrcRunes = MaxSourceBytes

	truncationMarker = "…"
)

// Props is the whole blob, the widest rung of the hook's ladder.
func Props(document *Document, src Source) map[string]any {
	return props(document, src, true)
}

// PropsWithoutProperties is the same blob with each feature's property bag left
// out, and is the lower rung.
//
// Properties are the only genuinely unbounded part of the blob. The feature
// name is hoisted out of the bag before this runs, so a document that falls to
// this rung still lists its features by name rather than as "Feature 1,
// Feature 2".
func PropsWithoutProperties(document *Document, src Source) map[string]any {
	return props(document, src, false)
}

func props(document *Document, src Source, withProperties bool) map[string]any {
	rendered := make([]any, 0, len(document.Features))
	for _, feature := range document.Features {
		rendered = append(rendered, featureProps(feature, withProperties))
	}

	counts := document.Counts()

	props := map[string]any{
		"version": PropsVersion,
		"source":  src.Kind,
		"lead":    sanitizeText(src.Lead, maxNoteRunes),
		"trail":   sanitizeText(src.Trail, maxNoteRunes),
		"note":    document.Note,

		// A presence key, like properties_dropped: absent is the ordinary case,
		// so only a document that cannot be placed carries anything.
		"unplaceable": unplaceable(document),
		"counts": map[string]any{
			"features":    counts.Features,
			"points":      counts.Points,
			"lines":       counts.Lines,
			"polygons":    counts.Polygons,
			"collections": counts.Collections,
			"unlocated":   counts.Unlocated,
			"undrawable":  counts.Undrawable,
		},
		"features": rendered,
	}

	// Truncated rather than withheld, per the same argument cot records: once
	// Post.Type is set the webapp never reads post.message, so this is the only
	// copy of the document any reader can reach.
	props["src"] = sanitizeText(src.Text, maxInlineSrcRunes)

	if !withProperties {
		// A presence key, because an absent properties array is otherwise
		// indistinguishable from a feature that genuinely carries none, which
		// is the common case in a real export.
		props["properties_dropped"] = "1"
	}

	if src.Kind == SourceFile {
		props["file_id"] = src.FileID
		props["file_name"] = sanitize(src.FileName, maxFieldRunes)
	}

	return props
}

// unplaceable is "1" when nothing in the document can go on a map, and "" when
// the map should draw whatever the features carry.
func unplaceable(document *Document) string {
	if document.Placeable {
		return ""
	}
	return "1"
}

func featureProps(feature Feature, withProperties bool) map[string]any {
	props := map[string]any{
		"name": feature.Name,
		"kind": feature.Geometry.Kind,
		"note": feature.Note,
	}

	addLocationLink(props, feature)

	// Optional, each of them, so a document that states no style carries no key
	// and the map falls back to the theme. Written one at a time rather than as
	// a nested object, because the shape guard walks flat keys and a style bag
	// would be one more record for it to learn.
	for key, value := range map[string]string{
		"color":        feature.Style.Color,
		"width":        feature.Style.Width,
		"line_opacity": feature.Style.StrokeOpacity,
		"fill_opacity": feature.Style.FillOpacity,
		"marker_size":  feature.Style.MarkerSize,
	} {
		if value != "" {
			props[key] = value
		}
	}

	// Rendered here rather than handed over as numbers, for the reason property
	// values are: both surfaces show it, and two renderers rounding the same
	// figure separately is how they come to disagree. Absent when the geometry
	// has no such measure, which the webapp reads as "do not show a row".
	if m := measure(feature); m.Length != "" || m.Area != "" {
		if m.Length != "" {
			props["length"] = m.Length
		}
		if m.Area != "" {
			props["area"] = m.Area
		}
	}

	parts := make([]any, 0, len(feature.Geometry.Parts))
	for _, part := range feature.Geometry.Parts {
		parts = append(parts, partProps(part))
	}
	props["parts"] = parts

	if withProperties && len(feature.Properties) > 0 {
		properties := make([]any, 0, len(feature.Properties))
		for _, property := range feature.Properties {
			properties = append(properties, map[string]any{
				"key":   property.Key,
				"value": property.Value,
			})
		}
		props["properties"] = properties
	}

	return props
}

// addLocationLink gives a lone point the identity the location tools take.
//
// Only for a single Point, and only when the position round-trips through the
// location grammar. Everything else is left without the pair, and the panel
// renders those rows as plain text: a polygon has no one position, and a
// MultiPoint has several, so linking either would be picking one and calling it
// the feature's.
//
// The pair is the identity and nothing derived travels with it, which is the
// same contract every decorator link carries.
//
// Optional on purpose. A post stamped before this existed carries neither key,
// and its rows read as text rather than as a broken link.
func addLocationLink(props map[string]any, feature Feature) {
	if feature.Note != "" || feature.Geometry.Kind != KindPoint {
		return
	}
	if len(feature.Geometry.Parts) != 1 {
		return
	}

	rings := feature.Geometry.Parts[0].Rings
	if len(rings) != 1 || len(rings[0]) != 1 {
		return
	}

	position := rings[0][0]

	parsed, ok := location.Parse(location.FormatDD, position.Lat+","+position.Lon)
	if !ok {
		return
	}

	props["format"] = string(location.FormatDD)
	props["value"] = parsed.Canonical()
}

func partProps(part Part) map[string]any {
	rings := make([]any, 0, len(part.Rings))
	for _, ring := range part.Rings {
		positions := make([]any, 0, len(ring))
		for _, position := range ring {
			positions = append(positions, positionProps(position))
		}
		rings = append(rings, positions)
	}

	props := map[string]any{
		"kind":  part.Kind,
		"rings": rings,
	}

	if len(part.RingCounts) > 0 {
		counts := make([]any, 0, len(part.RingCounts))
		for _, count := range part.RingCounts {
			counts = append(counts, count)
		}
		props["ring_counts"] = counts
	}

	return props
}

func positionProps(position Position) map[string]any {
	props := map[string]any{
		"lon": position.Lon,
		"lat": position.Lat,
	}

	if position.Alt != "" {
		props["alt"] = position.Alt
	}

	return props
}

func sanitize(raw string, maxRunes int) string {
	return capRunes(strings.TrimSpace(stripUnsafe(raw)), maxRunes)
}

// sanitizeText is sanitize without the trim, for the author's own message text
// and for the source. Trimming those would silently restyle what somebody
// wrote, which is a different thing from removing what they cannot see.
func sanitizeText(raw string, maxRunes int) string {
	return capRunes(stripUnsafe(raw), maxRunes)
}

func capRunes(cleaned string, maxRunes int) string {
	if utf8.RuneCountInString(cleaned) <= maxRunes {
		return cleaned
	}

	return string([]rune(cleaned)[:maxRunes]) + truncationMarker
}

func stripUnsafe(raw string) string {
	return strings.Map(func(r rune) rune {
		switch {
		case r == '\t' || r == '\n':
			return r
		case r == '\r':
			return -1
		case unicode.IsControl(r):
			return -1
		case unicode.Is(unicode.Cf, r):
			return -1
		}
		return r
	}, raw)
}
