package geojson

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"unicode/utf8"
)

// MaxSourceBytes is the largest source this package will read, and is what the
// hook checks an attachment against before asking the filestore for it.
const MaxSourceBytes = 64 * 1024

// MaxFeatures is how many features one document may carry, exported so the
// examples command can state the limit rather than repeat the number.
const MaxFeatures = 256

// MaxVertices is how many positions one document may carry in total.
//
// Document-wide, and a refusal. Both differ from cot.MaxVertices, which is per
// shape and truncates; docs/design/geojson.md argues why, so that nobody
// reconciles the two by changing one.
const MaxVertices = 4096

// MaxJSONDepth bounds the walker's own recursion.
//
// encoding/json caps nesting at 10000, but that cap lives in the value scanner
// and Decoder.Token bypasses it for '[' and '{'. A source of thirty thousand
// open brackets walks clean through Token and is refused only by Unmarshal,
// which this package never calls, so the walk counts its own depth.
const MaxJSONDepth = 32

const (
	maxCollectionDepth = 1

	maxProperties      = 32
	maxPropertyKeyRune = 64
	maxPropertyValRune = 256

	maxNameRunes = 128

	// maxCoordRunes bounds the one field stored verbatim. A latitude of sixty
	// thousand leading zeros parses, ranges and renders as a coordinate.
	maxCoordRunes = 32

	minRingPositions = 4

	// RFC 7946 section 3.1.4. Two positions is what makes a line a line; one is
	// a point the document called a line, and it drew and measured nothing
	// while still being counted as one.
	minLinePositions = 2
)

const crs84 = "urn:ogc:def:crs:ogc:1.3:crs84"

const (
	PositionUnusableNote = "A coordinate in this feature is not one this build will stand behind, so it is not drawn."
	RingOpenNote         = "A ring in this feature does not close, so it is not drawn."
	RingShortNote        = "A ring in this feature has too few points to enclose an area, so it is not drawn."
	LineShortNote        = "A line in this feature has fewer than two points, so it is not drawn."
	UnlocatedNote        = "The document states no position for this feature."
	ForeignCRSNote       = "The document states a coordinate reference system whose axis order this build cannot confirm, so nothing is drawn."
	BadBoxNote           = "The document states a bounding box this build could not read. The features are drawn from their own coordinates."
	NoFeaturesNote       = "The document names no features."
)

// Notes is every sentence this package may attach to a feature or a document.
//
// A catalog rather than a list at each call site, so a test that wants "every
// note" reads it. The repo convention is that such a test must read the catalog
// rather than name its members, and naming two of seven is how the earlier
// version passed while five went uncovered.
var Notes = []string{
	PositionUnusableNote,
	RingOpenNote,
	RingShortNote,
	LineShortNote,
	UnlocatedNote,
	ForeignCRSNote,
	BadBoxNote,
	NoFeaturesNote,
}

var (
	ErrTooLarge         = errors.New("geojson: source exceeds the maximum size")
	ErrNotUTF8          = errors.New("geojson: source is not valid UTF-8")
	ErrNotGeoJSON       = errors.New("geojson: source is not a GeoJSON document")
	ErrUnknownType      = errors.New("geojson: source names a type this build does not read")
	ErrTooManyFeatures  = errors.New("geojson: source carries more features than this build reads")
	ErrTooManyVertices  = errors.New("geojson: source carries more positions than this build reads")
	ErrTooDeep          = errors.New("geojson: source nests too deeply")
	ErrNestedCollection = errors.New("geojson: source nests a geometry collection inside another")
	ErrTrailing         = errors.New("geojson: source carries content after the document")
)

// Parse reads a whole GeoJSON document under a fixed budget.
//
// Every cap refuses the document rather than truncating it: a card showing the
// first 256 features of nine hundred is a card that is quietly wrong about what
// was posted. A coordinate this build will not stand behind is the exception,
// because that is one feature's problem and not the document's.
func Parse(source []byte) (*Document, error) {
	if len(source) > MaxSourceBytes {
		return nil, ErrTooLarge
	}
	if !utf8.Valid(source) {
		return nil, ErrNotUTF8
	}

	decoder := json.NewDecoder(bytes.NewReader(source))
	decoder.UseNumber()

	walker := &walker{decoder: decoder}

	root, err := walker.value(0)
	if err != nil {
		return nil, err
	}
	if err := walker.atEOF(); err != nil {
		return nil, err
	}

	document, ok := root.(object)
	if !ok {
		return nil, ErrNotGeoJSON
	}

	return build(document)
}

func (w *walker) atEOF() error {
	if _, err := w.decoder.Token(); !errors.Is(err, io.EOF) {
		return ErrTrailing
	}
	return nil
}

// object is a decoded JSON object with first-wins keys.
//
// First-wins matches cot's parser, which records the rule and the reason it is
// one rule rather than two. json.Unmarshal is last-wins, but nothing in this
// plugin unmarshals the document, so there is no second reader to disagree.
type object map[string]any

type walker struct {
	decoder *json.Decoder
}

func (w *walker) value(depth int) (any, error) {
	if depth > MaxJSONDepth {
		return nil, ErrTooDeep
	}

	token, err := w.decoder.Token()
	if err != nil {
		return nil, wrapSyntax(err)
	}

	delimiter, ok := token.(json.Delim)
	if !ok {
		return token, nil
	}

	switch delimiter {
	case '{':
		return w.object(depth)
	case '[':
		return w.array(depth)
	}

	return nil, ErrNotGeoJSON
}

func (w *walker) object(depth int) (any, error) {
	result := object{}

	for w.decoder.More() {
		token, err := w.decoder.Token()
		if err != nil {
			return nil, wrapSyntax(err)
		}

		key, ok := token.(string)
		if !ok {
			return nil, ErrNotGeoJSON
		}

		value, err := w.value(depth + 1)
		if err != nil {
			return nil, err
		}

		if _, seen := result[key]; !seen {
			result[key] = value
		}
	}

	if _, err := w.decoder.Token(); err != nil {
		return nil, wrapSyntax(err)
	}

	return result, nil
}

func (w *walker) array(depth int) (any, error) {
	var result []any

	for w.decoder.More() {
		value, err := w.value(depth + 1)
		if err != nil {
			return nil, err
		}

		result = append(result, value)
	}

	if _, err := w.decoder.Token(); err != nil {
		return nil, wrapSyntax(err)
	}

	return result, nil
}

func wrapSyntax(err error) error {
	if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
		return ErrNotGeoJSON
	}
	return err
}
