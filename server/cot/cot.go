package cot

import (
	"encoding/json"
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/decorators"
	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/decorators/dtg"
	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/decorators/location"
)

const (
	PostType = decorators.PostTypePrefix + "tf_cot"
	PropsKey = "tactical_fusion_cot"
	// PropsVersion is 2 because a source may carry several events, so the blob
	// holds an "events" array where version 1 held one "event". A post stamped
	// before that keeps its version and the webapp still reads it; a bundle
	// older than this meets a version it does not know and falls back to text,
	// which is what the field is for.
	PropsVersion = 2
)

const (
	SourceFence = "fence"
	SourceFile  = "file"
)

const (
	maxFieldRunes   = 128
	maxRemarksRunes = 1024
	maxNoteRunes    = 65536

	// maxInlineSrcRunes is the source pane, and it covers everything Parse
	// read. It was an eighth of that, which left an extension parsed from byte
	// 20000 with nothing in the pane to check it against.
	maxInlineSrcRunes = MaxSourceBytes

	truncationMarker = "…"
)

const unknownSentinel = 9999999.0

const (
	nullIslandNote = "The event reports 0,0, which is the Cursor on Target value for a position that was never set."
	coarseNote     = "The position is written too coarsely to link to the coordinate tools."
	rangeNote      = "The event reports a position outside the range of the earth."
	missingNote    = "The event states no position."
)

type Source struct {
	Kind     string
	Lead     string
	Trail    string
	Text     string
	FileID   string
	FileName string
}

func Props(events []Event, src Source) map[string]any {
	return props(events, src, true)
}

// PropsWithoutDetail is the same blob with the <detail> extension keys left out,
// and is the middle rung of the hook's budget ladder.
//
// Everything version 2 ever wrote survives it, so a post too large to carry the
// registry still gets exactly the card this feature shipped with rather than
// falling all the way back to raw XML.
func PropsWithoutDetail(events []Event, src Source) map[string]any {
	return props(events, src, false)
}

func props(events []Event, src Source, withDetail bool) map[string]any {
	rendered := make([]any, 0, len(events))
	for _, event := range events {
		rendered = append(rendered, eventProps(event, withDetail))
	}

	props := map[string]any{
		"version": PropsVersion,
		"source":  src.Kind,
		"lead":    sanitizeText(src.Lead, maxNoteRunes),
		"trail":   sanitizeText(src.Trail, maxNoteRunes),
		"events":  rendered,
	}

	// The source is sanitised too, and it is the one that matters most: the
	// disclosure is what a reader opens to check the card against, so a
	// direction override surviving here would subvert the verification rather
	// than the claim.
	//
	// Truncated rather than withheld past the cap. The webapp never reads the
	// message, so withholding it left a fenced event over the cap with no way
	// for any reader to reach the XML at all; a visible marker is the honest
	// version of the same limit.
	props["src"] = sanitizeText(src.Text, maxInlineSrcRunes)

	if src.Kind == SourceFile {
		props["file_id"] = src.FileID
		props["file_name"] = sanitize(src.FileName, maxFieldRunes)
	}

	return props
}

func eventProps(event Event, withDetail bool) map[string]any {
	props := map[string]any{
		"uid":      sanitize(event.UID, maxFieldRunes),
		"cot_type": sanitize(event.Type, maxFieldRunes),
		"how":      sanitize(event.How, maxFieldRunes),
	}

	decoded := decodeType(event.Type)
	props["type_label"] = decoded.Label
	props["affiliation"] = decoded.Affiliation
	props["how_label"] = decodeHow(event.How)

	putIfSet(props, "callsign", sanitize(event.Detail.Callsign, maxFieldRunes))
	putIfSet(props, "group", sanitize(event.Detail.Group, maxFieldRunes))
	putIfSet(props, "role", sanitize(event.Detail.Role, maxFieldRunes))
	putIfSet(props, "remarks", sanitize(event.Detail.Remarks, maxRemarksRunes))
	addLinks(props, event.Detail.Links)

	addTimes(props, event)
	addPosition(props, event.Point)

	putIfSet(props, "hae", metersText(event.Point.HAE, false))
	putIfSet(props, "ce", metersText(event.Point.CE, true))
	putIfSet(props, "le", metersText(event.Point.LE, true))
	putIfSet(props, "ce_meters", knownMeters(event.Point.CE))
	putIfSet(props, "speed", speedText(event.Detail.Speed))
	putIfSet(props, "course", courseText(event.Detail.Course))

	if !withDetail {
		// Said on the card rather than only in the server log. Without it the
		// panel draws no groups and no unrecognised count, which reads as "this
		// event carried nothing" instead of "this did not fit".
		props["detail_dropped"] = presenceValue
		return props
	}

	putIfSet(props, "class", classify(event))
	putIfSet(props, "detail_unknown", countText(event.Detail.Unknown))

	// Merged rather than written through, and nothing on the registry path is
	// handed this map. format, value and affiliation live in it: the first two
	// build a location URL the webapp follows and the third keys the marker
	// colour, so nothing author-derived may land beside them.
	for key, value := range blockProps(event.Detail.Blocks) {
		if _, held := props[key]; !held {
			props[key] = value
		}
	}

	if flow := flowProps(event.Detail.Flow); len(flow) > 0 {
		props["flow"] = flow
	}

	if geometry := geometryProps(drawnGeometry(event.Detail)); geometry != nil {
		props["geometry"] = geometry
	}

	if list := checklistProps(event.Detail.Checklist); list != nil {
		props["checklist"] = list
	}

	return props
}

func checklistProps(list Checklist) map[string]any {
	if list.empty() {
		return nil
	}

	props := map[string]any{}
	putIfSet(props, "count", countText(list.Seen))

	kinds := make([]any, 0, len(list.Kinds))
	for _, kind := range list.Kinds {
		name := strings.TrimSpace(stripUnsafe(kind.Name))
		if name == "" || utf8.RuneCountInString(name) > maxFieldRunes {
			continue
		}
		kinds = append(kinds, map[string]any{"name": name, "count": countText(kind.Count)})
	}
	if len(kinds) > 0 {
		props["kinds"] = kinds
	}

	return props
}

// drawnGeometry prefers the shape an event drew over the route its links imply.
// An event carrying both is describing the shape, and its links are still
// relations wherever they carry a uid.
func drawnGeometry(detail Detail) Geometry {
	if !detail.Shape.empty() {
		return detail.Shape
	}
	return detail.Route
}

// geometryProps is the shape an event describes, or nil when it describes none.
//
// One key holding an ordered list, for the reason flow is: the order of the
// vertices IS the shape. The coordinates are the event's own digits, as lat and
// lon already are, so the resolution the source carried survives.
func geometryProps(geometry Geometry) map[string]any {
	if geometry.empty() {
		return nil
	}

	props := map[string]any{"kind": geometry.Kind}

	if geometry.Kind == GeometryPolyline && geometry.Closed {
		props["closed"] = presenceValue
	}

	putIfSet(props, "major", positiveText(geometry.Major, unitMeters))
	putIfSet(props, "minor", positiveText(geometry.Minor, unitMeters))
	putIfSet(props, "angle", signedText(geometry.Angle, unitDegrees))

	putIfSet(props, "major_m", positiveText(geometry.Major, ""))
	putIfSet(props, "minor_m", positiveText(geometry.Minor, ""))
	putIfSet(props, "angle_deg", signedText(geometry.Angle, ""))

	if geometry.Undrawable != "" {
		props["note"] = geometry.Undrawable
		putIfSet(props, "count", countText(geometry.Seen))
		return props
	}

	if geometry.Kind == GeometryEllipse {
		// An ellipse is its axes, both of them, because the map draws a ring
		// from both. This is the vertex list's "fewer than two is not a shape"
		// rule applied to the other kind.
		if props["major"] == nil || props["minor"] == nil {
			return nil
		}
		return props
	}

	if len(geometry.Vertices) < 2 {
		return nil
	}

	points := make([]any, 0, len(geometry.Vertices))
	for _, vertex := range geometry.Vertices {
		points = append(points, map[string]any{"lat": vertex.Lat, "lon": vertex.Lon})
	}
	props["points"] = points
	props["count"] = countText(len(points))

	return props
}

func blockProps(blocks []Block) map[string]any {
	props := map[string]any{}

	for _, block := range blocks {
		ext, ok := extensionByElement(block.Name)
		if !ok {
			continue
		}

		if len(ext.Attrs) == 0 {
			props[ext.Prefix] = presenceValue
			continue
		}

		for _, attr := range ext.Attrs {
			key := ext.Prefix + "_" + attr.Key
			if _, held := props[key]; held {
				continue
			}
			if rendered := renderAttr(block.Attrs[attr.Key], attr.Unit); rendered != "" {
				props[key] = rendered
			}
		}
	}

	return props
}

func renderAttr(raw, unit string) string {
	switch unit {
	case unitMeters:
		return metersText(raw, false)
	case unitDegrees:
		return signedText(raw, unitDegrees)
	case unitPercent:
		return percentText(raw)
	case unitColor:
		return colorText(raw)
	case unitDbm:
		return signedText(raw, unitDbm)
	case unitHashCount:
		return hashCount(raw)
	default:
		return sanitize(raw, maxFieldRunes)
	}
}

// flowProps is the processing path, in the order the event wrote it.
//
// An ordered array rather than a map, because json.Marshal sorts map keys and
// the ordering IS the path. A name too long to render is dropped rather than
// truncated: a truncated key is our word rather than the event's, and two long
// names would collapse into two rows a reader cannot tell apart.
func flowProps(tags []FlowTag) []any {
	rendered := make([]any, 0, len(tags))

	for _, tag := range tags {
		system := strings.TrimSpace(stripUnsafe(tag.System))
		if system == "" || utf8.RuneCountInString(system) > maxFieldRunes {
			continue
		}

		at := sanitize(tag.Time, maxFieldRunes)
		if instant, ok := instantOf(tag.Time); ok {
			at = dtg.FormatZulu(instant)
		}

		rendered = append(rendered, map[string]any{"system": system, "time": at})
	}

	return rendered
}

// addLinks says who sent the event and what else it names.
//
// ATAK writes a "p-p" relation on almost every event naming the device that
// produced it, and usually puts the sending unit's callsign on it. That answers
// "who sent this", which is the one thing a relation is good for without the
// other event in hand.
func addLinks(props map[string]any, links []Link) {
	var related []string

	for _, link := range links {
		if parent := sanitize(link.ParentCallsign, maxFieldRunes); parent != "" {
			putIfSet(props, "parent", parent)
		}
		if uid := sanitize(link.UID, maxFieldRunes); uid != "" {
			related = append(related, uid)
		}
	}

	putIfSet(props, "related", strings.Join(related, ", "))
}

func addTimes(props map[string]any, event Event) {
	start, startOK := instantOf(event.Time)
	if startOK {
		props["time"] = dtg.FormatZulu(start)
		props["time_at"] = strconv.FormatInt(start.UnixMilli(), 10)
		putIfSet(props, "time_q", dtgQuery(start))
	}

	if begin, ok := instantOf(event.Start); ok {
		props["start"] = dtg.FormatZulu(begin)
		putIfSet(props, "start_q", dtgQuery(begin))
	}

	stale, staleOK := instantOf(event.Stale)
	if !staleOK {
		return
	}

	props["stale"] = dtg.FormatZulu(stale)
	props["stale_at"] = strconv.FormatInt(stale.UnixMilli(), 10)
	putIfSet(props, "stale_q", dtgQuery(stale))
}

// dtgQuery is the date-time group decorator's own link params, or "" for an
// instant its grammar cannot spell.
//
// Built here rather than in the webapp for the reason the position pair is: the
// page re-derives everything from the canonical token and refuses a set that
// does not round trip, and only the package that owns the grammar can produce
// one that does.
func dtgQuery(t time.Time) string {
	params, ok := dtg.ParamsForZulu(t)
	if !ok {
		return ""
	}

	return params.Encode()
}

// decimalShape is what a lat or lon must look like before it is shown as one.
//
// strconv.ParseFloat also accepts hexadecimal floats and exponent notation,
// which JavaScript's Number does not, so a value like "0x1p+3" would be
// validated here, stored verbatim, shown as a position and then read as NaN by
// the map, leaving the card and the picture disagreeing about whether there is
// a position at all.
var decimalShape = regexp.MustCompile(`^[+-]?[0-9]+(\.[0-9]+)?$`)

func addPosition(props map[string]any, point Point) {
	lat, latOK := decimalNumber(point.Lat)
	lon, lonOK := decimalNumber(point.Lon)

	if !latOK || !lonOK {
		props["position_note"] = missingNote
		return
	}

	if math.Abs(lat) > 90 || math.Abs(lon) > 180 {
		props["position_note"] = rangeNote
		return
	}

	props["lat"] = point.Lat
	props["lon"] = point.Lon

	if lat == 0 && lon == 0 {
		props["position_note"] = nullIslandNote
		return
	}

	parsed, ok := location.Parse(location.FormatDD, point.Lat+","+point.Lon)
	if !ok {
		props["position_note"] = coarseNote
		return
	}

	props["format"] = string(location.FormatDD)
	props["value"] = parsed.Canonical()
}

// numberText refuses a figure too long to be a reading rather than clipping it.
//
// FormatFloat never uses exponent notation, so a subnormal like 1e-320 expands
// to its full positional form: 324 runes in a cell whose stated cap is 128. A
// clipped number still reads as a number, which is worse than no row, so this
// follows the flow-tag rule and drops it.
func numberText(value float64, unit string) string {
	text := trimFloat(value)
	if utf8.RuneCountInString(text) > maxFieldRunes {
		return ""
	}
	return text + unit
}

func speedText(raw string) string {
	value, ok := knownNumber(raw, true)
	if !ok {
		return ""
	}
	return numberText(value, " m/s")
}

func courseText(raw string) string {
	value, ok := numberOf(raw)
	if !ok || value < 0 || value > 360 {
		return ""
	}
	return numberText(value, "°")
}

// hashCount is how many attachments an event references, not which.
//
// A content hash is longer than a field, so the list truncates mid-hash into
// something that looks like a hash and is not. The count is the part a reader
// can act on, and the whole list is still under "As posted".
func hashCount(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}

	var hashes []string
	if err := json.Unmarshal([]byte(trimmed), &hashes); err != nil {
		return ""
	}

	return countText(len(hashes))
}

func signedText(raw, unit string) string {
	value, ok := knownNumber(raw, false)
	if !ok {
		return ""
	}
	return numberText(value, unit)
}

func positiveText(raw, unit string) string {
	value, ok := knownNumber(raw, true)
	if !ok || value <= 0 {
		return ""
	}
	return numberText(value, unit)
}

func percentText(raw string) string {
	value, ok := knownNumber(raw, true)
	if !ok {
		return ""
	}
	return numberText(value, "%")
}

// colorText is the event's stated display colour, and never this plugin's.
//
// ATAK writes argb as a signed 32-bit decimal rather than as hex. The alpha
// byte is dropped: a fully transparent swatch is a row that says nothing. The
// hex is validated here and again in the webapp before it reaches a style
// property, because a props blob is not a trusted input either.
func colorText(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}

	value, err := strconv.ParseInt(trimmed, 10, 64)
	if err != nil || value < math.MinInt32 || value > math.MaxUint32 {
		return ""
	}

	return fmt.Sprintf("#%06x", value&0x00FFFFFF)
}

func countText(count int) string {
	if count <= 0 {
		return ""
	}
	return strconv.Itoa(count)
}

func metersText(raw string, nonNegative bool) string {
	value, ok := knownNumber(raw, nonNegative)
	if !ok {
		return ""
	}
	return numberText(value, unitMeters)
}

func knownMeters(raw string) string {
	value, ok := knownNumber(raw, true)
	if !ok {
		return ""
	}
	return numberText(value, "")
}

func knownNumber(raw string, nonNegative bool) (float64, bool) {
	value, ok := numberOf(raw)
	if !ok || math.Abs(value) >= unknownSentinel {
		return 0, false
	}
	if nonNegative && value < 0 {
		return 0, false
	}
	return value, true
}

func decimalNumber(raw string) (float64, bool) {
	if !decimalShape.MatchString(raw) {
		return 0, false
	}
	return numberOf(raw)
}

func numberOf(raw string) (float64, bool) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return 0, false
	}

	value, err := strconv.ParseFloat(trimmed, 64)
	if err != nil || math.IsNaN(value) || math.IsInf(value, 0) {
		return 0, false
	}
	return value, true
}

// trimFloat normalises negative zero, which formats as "-0" and reads as a
// direction on a bearing and as a sign on a battery.
func trimFloat(value float64) string {
	if value == 0 {
		value = 0
	}
	return strconv.FormatFloat(value, 'f', -1, 64)
}

func instantOf(raw string) (time.Time, bool) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return time.Time{}, false
	}

	parsed, err := time.Parse(time.RFC3339, trimmed)
	if err != nil {
		return time.Time{}, false
	}
	return parsed.UTC(), true
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

func putIfSet(props map[string]any, key, value string) {
	if value != "" {
		props[key] = value
	}
}
