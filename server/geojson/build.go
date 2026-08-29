package geojson

import (
	"encoding/json"
	"math"
	"slices"
	"strconv"
	"strings"
)

func build(root object) (*Document, error) {
	b := &budget{}

	kind, ok := root.string("type")
	if !ok {
		return nil, ErrNotGeoJSON
	}

	document := &Document{}

	switch {
	case kind == typeCollection:
		features, ok := root["features"].([]any)
		if !ok {
			return nil, ErrNotGeoJSON
		}
		if len(features) > MaxFeatures {
			return nil, ErrTooManyFeatures
		}
		for index, raw := range features {
			member, ok := raw.(object)
			if !ok {
				return nil, ErrNotGeoJSON
			}
			feature, err := buildFeature(member, index, b)
			if err != nil {
				return nil, err
			}
			document.Features = append(document.Features, feature)
		}

	case kind == typeFeature:
		feature, err := buildFeature(root, 0, b)
		if err != nil {
			return nil, err
		}
		document.Features = append(document.Features, feature)

	case geometryKinds[kind]:
		geometry, note, err := buildGeometry(root, 0, b)
		if err != nil {
			return nil, err
		}
		document.Features = append(document.Features, Feature{
			Name:     "Feature 1",
			Geometry: geometry,
			Note:     note,
		})

	default:
		return nil, ErrUnknownType
	}

	document.Note = documentNote(root, len(document.Features))
	document.Placeable = crsIsWGS84(root)

	return document, nil
}

// documentNote is why nothing in the document is drawn.
//
// A foreign CRS wins over an empty document because it is the more specific
// statement: an empty collection in a projection we cannot place is still
// empty, but saying so first would hide the projection from the reader.
func documentNote(root object, features int) string {
	if !crsIsWGS84(root) {
		return ForeignCRSNote
	}
	if features == 0 {
		return NoFeaturesNote
	}
	if !boxIsReadable(root) {
		return BadBoxNote
	}

	return ""
}

// crsIsWGS84 reports whether the document may be placed on a map.
//
// RFC 7946 removed crs and fixed the coordinate reference system, so an absent
// member is the ordinary case and the only one that needs no reading. A named
// CRS84 is honored. Everything else, EPSG:4326 included, is refused: many
// producers write that name with latitude first, and this package cannot tell
// which from the document.
func crsIsWGS84(root object) bool {
	raw, present := root["crs"]
	if !present {
		return true
	}

	crs, ok := raw.(object)
	if !ok {
		return false
	}

	properties, ok := crs["properties"].(object)
	if !ok {
		return false
	}

	name, ok := properties.string("name")
	if !ok {
		return false
	}

	return strings.ToLower(strings.TrimSpace(name)) == crs84
}

// boxIsReadable checks the stated bounding box without using it.
//
// The extent is computed from the coordinates, because a stated box is a claim
// the coordinates may not support. It is still read, because a malformed one is
// often the symptom of a producer writing the axes the wrong way round.
func boxIsReadable(root object) bool {
	raw, present := root["bbox"]
	if !present {
		return true
	}

	box, ok := raw.([]any)
	if !ok || (len(box) != 4 && len(box) != 6) {
		return false
	}

	for _, item := range box {
		number, ok := item.(json.Number)
		if !ok {
			return false
		}
		value, err := number.Float64()
		if err != nil || math.IsInf(value, 0) || math.IsNaN(value) {
			return false
		}
	}

	return true
}

func buildFeature(member object, index int, b *budget) (Feature, error) {
	kind, ok := member.string("type")
	if !ok || kind != typeFeature {
		return Feature{}, ErrNotGeoJSON
	}

	feature := Feature{Name: featureName(member, index)}

	properties, _ := member["properties"].(object)
	feature.Properties = buildProperties(properties)

	raw, present := member["geometry"]
	if !present {
		return Feature{}, ErrNotGeoJSON
	}

	// An unlocated feature. RFC 7946 section 3.2 makes a null geometry legal,
	// so it is listed and excluded from the extent rather than refused.
	if raw == nil {
		feature.Geometry = Geometry{Kind: KindNone}
		feature.Note = UnlocatedNote
		return feature, nil
	}

	geometry, ok := raw.(object)
	if !ok {
		return Feature{}, ErrNotGeoJSON
	}

	built, note, err := buildGeometry(geometry, 0, b)
	if err != nil {
		return Feature{}, err
	}

	feature.Geometry = built
	feature.Note = note
	feature.Style = featureStyle(built.Kind, properties)

	return feature, nil
}

// featureName is what the card lists the feature as.
//
// Hoisted out of the properties bag on purpose: the second props rung drops
// properties, and an unhoisted name would vanish exactly when the document is
// largest.
func featureName(member object, index int) string {
	properties, _ := member["properties"].(object)

	for _, key := range []string{"name", "title", "label"} {
		if value, ok := properties.string(key); ok {
			if trimmed := strings.TrimSpace(value); trimmed != "" {
				return sanitize(trimmed, maxNameRunes)
			}
		}
	}

	switch id := member["id"].(type) {
	case string:
		if trimmed := strings.TrimSpace(id); trimmed != "" {
			return sanitize(trimmed, maxNameRunes)
		}
	case json.Number:
		return sanitize(id.String(), maxNameRunes)
	}

	return "Feature " + strconv.Itoa(index+1)
}

func buildProperties(properties object) []Property {
	if len(properties) == 0 {
		return nil
	}

	keys := sortedKeys(properties)
	built := make([]Property, 0, len(keys))

	for _, key := range keys {
		if len(built) == maxProperties {
			break
		}

		value, ok := renderValue(properties[key])
		if !ok {
			continue
		}

		built = append(built, Property{
			Key:   sanitize(key, maxPropertyKeyRune),
			Value: value,
		})
	}

	if len(built) == 0 {
		return nil
	}

	return built
}

// renderValue turns one property value into the string the card shows.
//
// Done here rather than in the webapp so the rune measurement the props ladder
// depends on is measuring what is actually stored, and so the escaping test has
// one place to point at. A null is dropped along with its key: rendering it as
// "null" states a value the document did not carry.
func renderValue(value any) (string, bool) {
	switch typed := value.(type) {
	case nil:
		return "", false
	case string:
		return sanitize(typed, maxPropertyValRune), true
	case json.Number:
		return sanitize(typed.String(), maxPropertyValRune), true
	case bool:
		return strconv.FormatBool(typed), true
	}

	encoded, err := json.Marshal(rebuild(value))
	if err != nil {
		return "", false
	}

	return sanitize(string(encoded), maxPropertyValRune), true
}

// rebuild converts the walker's own types back into what json.Marshal expects.
//
// The walker hands back its own object type for a nested value, and Marshal
// would write a json.Number as a quoted string without this.
func rebuild(value any) any {
	switch typed := value.(type) {
	case object:
		out := make(map[string]any, len(typed))
		for key, item := range typed {
			out[key] = rebuild(item)
		}
		return out
	case []any:
		out := make([]any, 0, len(typed))
		for _, item := range typed {
			out = append(out, rebuild(item))
		}
		return out
	case json.Number:
		return json.RawMessage(typed.String())
	}

	return value
}

func (o object) string(key string) (string, bool) {
	value, ok := o[key].(string)
	return value, ok
}

func sortedKeys(o object) []string {
	keys := make([]string, 0, len(o))
	for key := range o {
		keys = append(keys, key)
	}

	// Sorted so the card lists a feature's properties the same way twice, and
	// so the props blob a test measures does not change between runs. Go
	// randomizes map iteration order, and the blob is written permanently.
	//
	// slices.Sort rather than a hand-rolled insertion sort. The cap below is
	// applied while walking the SORTED list, so the sort sees every key a
	// document carries rather than the 32 that survive: an insertion sort made
	// a 64 KiB document of short keys cost 47 ms on the post path.
	slices.Sort(keys)

	return keys
}
