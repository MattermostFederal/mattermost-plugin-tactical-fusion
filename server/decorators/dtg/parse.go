package dtg

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// DTG is a validated Date-Time Group.
type DTG struct {
	Day    int
	Hour   int
	Minute int
	Zone   byte // military zone letter
	Month  time.Month
	Year   int

	// AssumedMonth and AssumedYear record that the short form supplied neither,
	// so both were taken from the reference time. The UI says so rather than
	// silently presenting an inferred date as fact.
	AssumedMonth bool
	AssumedYear  bool
}

// The canonical form carries two year digits, so the accepted range is exactly
// the century those digits can represent without ambiguity.
const (
	minYear = 2000
	maxYear = 2099
)

var monthAbbrevs = map[string]time.Month{
	"JAN": time.January, "FEB": time.February, "MAR": time.March,
	"APR": time.April, "MAY": time.May, "JUN": time.June,
	"JUL": time.July, "AUG": time.August, "SEP": time.September,
	"OCT": time.October, "NOV": time.November, "DEC": time.December,
}

// parseDTG validates a candidate token against a reference time.
//
// It accepts three forms:
//
//	DDHHMM<Z>MMMYY    091630ZAUG26   any zone letter except I and J
//	DDHHMM<Z>MMMYYYY  091630ZAUG2026 any zone letter except I and J
//	DDHHMMZ           091630Z        literal Z only; month and year inferred
//
// ok=false means "not really a DTG", and the caller leaves the text alone.
func parseDTG(value string, ref time.Time) (DTG, bool) {
	if len(value) < 7 {
		return DTG{}, false
	}

	// DDHHMM is common to every form.
	if !allDigits(value[:6]) {
		return DTG{}, false
	}
	day, _ := strconv.Atoi(value[0:2])
	hour, _ := strconv.Atoi(value[2:4])
	minute, _ := strconv.Atoi(value[4:6])

	zone := value[6]
	offset, zoneOK := zoneOffsetHours(zone)
	if !zoneOK {
		return DTG{}, false
	}

	d := DTG{Day: day, Hour: hour, Minute: minute, Zone: zone}
	rest := value[7:]

	switch {
	case rest == "":
		// Short form. Only Zulu, because a bare six-digit run followed by any
		// letter is far too loose to claim in general chat: it collides with
		// part numbers, serials, and truncated hashes.
		if zone != 'Z' {
			return DTG{}, false
		}
		refUTC := ref.UTC()
		d.Month = refUTC.Month()
		d.Year = refUTC.Year()
		d.AssumedMonth = true
		d.AssumedYear = true

	case len(rest) == 5 || len(rest) == 7:
		month, monthOK := monthAbbrevs[strings.ToUpper(rest[:3])]
		if !monthOK {
			return DTG{}, false
		}
		digits := rest[3:]
		if !allDigits(digits) {
			return DTG{}, false
		}
		year, _ := strconv.Atoi(digits)
		if len(digits) == 2 {
			// Two-digit years are operational shorthand and always near-term.
			year += 2000
		}
		d.Month = month
		d.Year = year

	default:
		return DTG{}, false
	}

	if !d.valid(offset) {
		return DTG{}, false
	}
	return d, true
}

// valid rejects out-of-range components and impossible calendar dates.
//
// The day check is per-month and leap-year aware on purpose. A plain 01-31
// range test would let 31 FEB through, and time.Date normalises that silently
// to 3 March, so every row of the timezone table would then confidently show
// the wrong date.
func (d DTG) valid(offset int) bool {
	if d.Hour > 23 || d.Minute > 59 {
		return false
	}

	// Years outside 2000-2099 are rejected because the canonical form carries
	// only two year digits. Accepting 2150 would canonicalise it to "50", which
	// reads back as 2050: the link would silently describe a different century
	// from the text the author typed. Two-digit years already mean 20NN, and a
	// DTG is operational and near-term, so this costs nothing real.
	if d.Year < minYear || d.Year > maxYear {
		return false
	}
	if d.Day < 1 || d.Day > daysInMonth(d.Year, d.Month) {
		return false
	}

	// Guard the arithmetic in resolveInstant rather than trusting it.
	return offset >= -12 && offset <= 12
}

func daysInMonth(year int, month time.Month) int {
	// Day 0 of the following month is the last day of this one.
	return time.Date(year, month+1, 0, 0, 0, 0, 0, time.UTC).Day()
}

func allDigits(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return len(s) > 0
}

// resolveInstant converts a DTG to an absolute UTC instant.
//
// The wall-clock components are expressed in the DTG's own zone, so the zone
// offset is subtracted to get UTC. The reference implementation this design
// draws on hardcoded UTC because it only ever handled Zulu.
func (d DTG) resolveInstant() time.Time {
	offset, _ := zoneOffsetHours(d.Zone)
	return time.Date(d.Year, d.Month, d.Day, d.Hour, d.Minute, 0, 0, time.UTC).
		Add(-time.Duration(offset) * time.Hour)
}

// canonical renders the DTG back to its standard string form.
//
// parseDTG followed by canonical round trips for every accepted token, which is
// what proves no component was silently normalised into a different date. The
// four-digit-year form normalises to the two-digit one, losslessly, because
// only 2000-2099 is accepted.
func (d DTG) canonical() string {
	base := fmt.Sprintf("%02d%02d%02d%c", d.Day, d.Hour, d.Minute, d.Zone)
	if d.AssumedMonth && d.AssumedYear {
		return base
	}
	return fmt.Sprintf("%s%s%02d", base, strings.ToUpper(d.Month.String()[:3]), d.Year%100)
}

// FormatZulu renders an instant as a Zulu long-form DTG, e.g. 091630ZAUG26.
//
// The result always parses back through this package, so callers generating
// example or test text cannot accidentally produce something the decorator
// declines. Seconds are dropped, because a DTG has no place to put them.
func FormatZulu(t time.Time) string {
	utc := t.UTC()

	return DTG{
		Day:    utc.Day(),
		Hour:   utc.Hour(),
		Minute: utc.Minute(),
		Zone:   'Z',
		Month:  utc.Month(),
		Year:   utc.Year(),
	}.canonical()
}

// assumedCode is the compact "a" URL parameter: "my" when both month and year
// were inferred, "y" when only the year was, empty when neither.
func (d DTG) assumedCode() string {
	code := ""
	if d.AssumedMonth {
		code += "m"
	}
	if d.AssumedYear {
		code += "y"
	}
	return code
}

// ISO is a validated RFC 3339 timestamp.
//
// Kept beside DTG rather than folded into it: a DTG's zone is a military
// letter, which can only name a whole-hour offset, and RFC 3339 offsets run to
// half and quarter hours. The two share everything downstream, an instant and
// an offset, and nothing upstream.
type ISO struct {
	Instant time.Time

	// OffsetMinutes is the offset the token itself was written in, which is
	// what the page describes it in. Zero for a Z timestamp.
	OffsetMinutes int
}

// maxOffsetMinutes bounds a written offset. Real zones run from -12:00 to
// +14:00; anything beyond that is a crafted value, not a place.
const maxOffsetMinutes = 14 * 60

// isoLayouts are the shapes parseISO accepts, in the order it tries them.
//
// Seconds and fractional seconds are both optional in practice even though
// RFC 3339 requires seconds, because timestamps written by hand routinely drop
// them. Fractional seconds are parsed and then discarded: the panel counts in
// whole seconds, so keeping them would only make the canonical form harder to
// reproduce exactly.
var isoLayouts = []string{
	"2006-01-02T15:04:05.999999999Z07:00",
	"2006-01-02T15:04:05Z07:00",
	"2006-01-02T15:04Z07:00",
}

// parseISO validates an RFC 3339 timestamp.
//
// Only the extended form, with its hyphens and colons, is accepted. The basic
// form (20260809T163000Z) is deliberately absent: it turns up inside filenames
// such as snapshot-20260809T163000Z.zip, where a hyphen is a word boundary and
// the token would match in the middle of somebody's attachment name.
//
// ok=false means "not really a timestamp", and the caller leaves the text alone.
func parseISO(value string) (ISO, bool) {
	// Uppercased first: RFC 3339 permits a lowercase t and z, and accepting
	// them here rather than in three layouts each keeps the shapes readable.
	normalised := strings.ToUpper(value)

	for _, layout := range isoLayouts {
		parsed, err := time.Parse(layout, normalised)
		if err != nil {
			continue
		}

		_, offsetSeconds := parsed.Zone()
		offset := offsetSeconds / 60
		if offset < -maxOffsetMinutes || offset > maxOffsetMinutes {
			return ISO{}, false
		}

		// The same window validateParams enforces on "t". Decoration and
		// rendering have to agree about what is representable: a timestamp
		// accepted here but rejected there would be rewritten permanently into
		// a link whose own page answers 400, and editing the post by hand is
		// the only way back. The military grammar cannot reach this, since its
		// years are clamped to a single century, but RFC 3339 has no such
		// limit and "1918-11-11T11:00:00Z" is an ordinary thing to write.
		millis := parsed.UnixMilli()
		if millis < minInstantMillis || millis > maxInstantMillis {
			return ISO{}, false
		}

		// Truncated, not rounded: the countdown ticks in whole seconds and the
		// canonical form has nowhere to put a fraction.
		return ISO{Instant: parsed.Truncate(time.Second).UTC(), OffsetMinutes: offset}, true
	}

	return ISO{}, false
}

// canonical renders the timestamp back to one normalised shape.
//
// Always seconds, always an uppercase T, no fraction, and a zero offset written
// as Z. parseISO followed by canonical round trips, which is what lets the page
// re-derive the whole payload from the canonical form and reject anything
// self-inconsistent.
func (i ISO) canonical() string {
	local := i.Instant.In(time.FixedZone("", i.OffsetMinutes*60))
	if i.OffsetMinutes == 0 {
		return local.Format("2006-01-02T15:04:05Z")
	}

	return local.Format("2006-01-02T15:04:05-07:00")
}

// FormatOffset renders an offset in minutes as RFC 3339 writes it.
func FormatOffset(minutes int) string {
	if minutes == 0 {
		return "Z"
	}

	sign := "+"
	if minutes < 0 {
		sign = "-"
		minutes = -minutes
	}

	return fmt.Sprintf("%s%02d:%02d", sign, minutes/60, minutes%60)
}
