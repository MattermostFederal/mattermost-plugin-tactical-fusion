package cot

import (
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
	maxFieldRunes     = 128
	maxRemarksRunes   = 1024
	maxInlineSrcRunes = 8192
	maxNoteRunes      = 65536
	truncationMarker  = "…"
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
	rendered := make([]any, 0, len(events))
	for _, event := range events {
		rendered = append(rendered, eventProps(event))
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

func eventProps(event Event) map[string]any {
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

	return props
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

func speedText(raw string) string {
	value, ok := knownNumber(raw, true)
	if !ok {
		return ""
	}
	return trimFloat(value) + " m/s"
}

func courseText(raw string) string {
	value, ok := numberOf(raw)
	if !ok || value < 0 || value > 360 {
		return ""
	}
	return trimFloat(value) + "°"
}

func metersText(raw string, nonNegative bool) string {
	value, ok := knownNumber(raw, nonNegative)
	if !ok {
		return ""
	}
	return trimFloat(value) + " m"
}

func knownMeters(raw string) string {
	value, ok := knownNumber(raw, true)
	if !ok {
		return ""
	}
	return trimFloat(value)
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

func trimFloat(value float64) string {
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
		case r >= 0x202A && r <= 0x202E:
			return -1
		case r >= 0x2066 && r <= 0x2069:
			return -1
		case r == 0xFEFF:
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
