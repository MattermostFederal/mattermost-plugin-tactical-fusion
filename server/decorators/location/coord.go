// Package location decorates geographic coordinates.
//
// Every grammar this package recognizes normalizes to one Location, and the
// identity of a location is the pair (Format, canonical token). Latitude,
// longitude, resolution and every alternative notation are derived from those
// two and are never carried alongside them, so nothing in a link can disagree
// with the token it was built from.
package location

import (
	"strconv"
	"strings"
)

// Format is the grammar a token was written in.
//
// Two grammars that resolve to the same place still get different ids when they
// look different on the page, which is why dd and ddh are separate and why the
// USMTF compact shapes are not folded into dms.
type Format string

const (
	// FormatDD is signed decimal degrees: 34.0561,-118.2500.
	FormatDD Format = "dd"

	// FormatDDH is decimal degrees with hemisphere letters: 34.0561N,118.2500W.
	FormatDDH Format = "ddh"

	// FormatDMS is degrees, minutes and seconds, and doubles as USMTF LATS.
	// Seconds may carry a fraction, which is USMTF LATDS and DMPID.
	FormatDMS Format = "dms"

	// FormatDDM is degrees and decimal minutes, and doubles as USMTF GEOK.
	FormatDDM Format = "ddm"

	// FormatLATD is USMTF degrees only: 35N079W.
	FormatLATD Format = "latd"

	// FormatLATM is USMTF degrees and whole minutes: 3510N07901W.
	FormatLATM Format = "latm"

	// FormatVLATM is USMTF LATM carrying a confidence digit per axis:
	// 3510N9-07901W7.
	FormatVLATM Format = "vlatm"

	// FormatMGRS is a military grid reference: 11SLT1234567890.
	//
	// The only grammar here that names a square rather than a point, and the
	// first that cannot be read without a projection.
	FormatMGRS Format = "mgrs"

	// FormatUTM is a UTM zone, band, easting and northing: 11S3850003769000.
	FormatUTM Format = "utm"
)

// AllFormatIDs is every id this package will accept in a link, in a fixed
// order. Nothing may parse a format id that is not in here: the id selects the
// grammar, so treating it as open would let a crafted link pick a grammar the
// token was never written in.
var AllFormatIDs = []Format{
	FormatDD, FormatDDH, FormatDMS, FormatDDM, FormatLATD, FormatLATM, FormatVLATM,
	FormatMGRS, FormatUTM,
}

// There is deliberately no isGrid helper here, unlike isGridFormat in the
// webapp. Every place Go cares is already a switch over Format that has to name
// the two grammars separately anyway, because they behave differently: MGRS
// names a square and UTM names a point. A predicate collapsing them would read
// as though it did not matter which.

var formatByID = func() map[Format]bool {
	m := make(map[Format]bool, len(AllFormatIDs))
	for _, f := range AllFormatIDs {
		m[f] = true
	}
	return m
}()

// KnownFormat reports whether an id names a grammar this build understands.
func KnownFormat(f Format) bool { return formatByID[f] }

// maxFrac is the most fractional digits any token may carry.
//
// Past this a float64 stops being able to reproduce the decimal string it came
// from, and Decimal is what every rendered row is built on. Eight decimal
// degrees is well under a millimeter, so the cap costs nothing real. A token
// with more digits is declined rather than truncated: truncating would silently
// change what the author wrote.
const maxFrac = 8

// NoConfidence marks an axis whose token carried no USMTF confidence digit.
const NoConfidence int8 = -1

// FracUnit names the field an axis's fractional digits belong to.
//
// It has to be explicit rather than inferred from which components are
// non-zero: 34.0561 has no minutes and no seconds and its fraction is degrees,
// while 3400.366 also has no seconds and its fraction is minutes. Guessing from
// the zeroes would read one as the other.
type FracUnit uint8

const (
	FracNone FracUnit = iota
	FracDegrees
	FracMinutes
	FracSeconds
)

// Axis is one half of a coordinate, held as the components the author wrote
// rather than as a number.
//
// This is deliberately textual. Holding a float64 and rebuilding the canonical
// form from it cannot round-trip a sexagesimal token: 340322N parses to
// 34.05611111..., and recovering whole seconds from that float can land on
// 21.99999999, so the canonical form comes back as 340321N. Since a link is
// rejected when it does not reproduce its own token, that would turn a token
// accepted at decoration into a permanent 400 on the page behind it, editable
// only by hand. Integers in, integers out.
type Axis struct {
	Deg int // 0-90 for latitude, 0-180 for longitude
	Min int // 0-59; unused by dd, ddh and latd
	Sec int // 0-59; unused by dd, ddh, latd, latm and ddm

	// Frac is the fractional digits exactly as written, without the point, and
	// "" when the token carried none.
	//
	// A string rather than a count, because it is what canonical writes back
	// out. Digits is len(Frac), derived, so the two cannot disagree.
	Frac string

	// FracUnit says which field Frac belongs to. FracNone when Frac is "".
	FracUnit FracUnit

	// Hemi is 'N', 'S', 'E' or 'W', or 0 for signed decimal degrees.
	Hemi byte

	// Neg is the sign, and is only ever set when Hemi is 0.
	Neg bool

	// Conf is the USMTF verified confidence digit, or NoConfidence.
	//
	// It describes how well the position is known rather than where it is, so
	// it never reaches Decimal and is rendered beside the resolution rather
	// than folded into it.
	Conf int8
}

// Grid is a position on the UTM grid, held as the components the author wrote.
//
// Textual for the same reason Axis is: the canonical form is rebuilt from these
// integers and letters, never from the latitude and longitude they project to.
// A grid reference that had to survive a round trip through a float would be at
// the mercy of the projection reproducing the square it started in, and a token
// that failed to would be rewritten into somebody's message with a page behind
// it that answers 400 forever.
type Grid struct {
	// Format is FormatMGRS or FormatUTM. The two share this type because they
	// are the same grid read at different granularities.
	Format Format

	// Zone is the UTM zone, 1-60.
	Zone int

	// Band is the latitude band letter, which is what carries the hemisphere: a
	// northing alone cannot say which side of the equator it is on.
	Band byte

	// Col and Row are the 100 km square letters, and are zero for UTM.
	Col, Row byte

	// Easting and Northing are meters. For MGRS they are the offset within the
	// 100 km square, at the token's own resolution; for UTM they are the full
	// grid coordinates.
	Easting, Northing int

	// Digits is how many digits each of Easting and Northing was written with,
	// 1-5, and therefore how big the square is. Unused by UTM, whose widths are
	// fixed.
	Digits int
}

// Location is a coordinate together with the grammar it was written in.
//
// Exactly one of the two representations is populated, chosen by Format: the
// textual grammars fill Lat and Lon, the projected ones fill Grid. Point is
// what reads whichever it is, and nothing outside it may assume either.
type Location struct {
	Lat, Lon Axis
	Grid     Grid
	Format   Format
}

// Point is the position in signed decimal degrees.
//
// For a grid this is the center of the square the token names, and computing it
// runs the inverse projection, so it can fail: a crafted token can name a
// square that does not exist. Every caller has to handle that rather than
// rendering whatever a failed conversion left behind.
func (l Location) Point() (lat, lon float64, ok bool) {
	switch l.Format {
	case FormatMGRS:
		return mgrsCenter(l.Grid)
	case FormatUTM:
		return utmPointOf(l.Grid)
	default:
		return l.Lat.Decimal(), l.Lon.Decimal(), true
	}
}

// Decimal is the axis in signed decimal degrees.
//
// Derived, never stored, and never read by canonical. It is what the page and
// the panel render and what a later phase would feed to a projection.
func (a Axis) Decimal() float64 {
	whole := float64(a.Deg) + float64(a.Min)/60 + float64(a.Sec)/3600
	if a.Frac != "" {
		whole += a.fracValue()
	}

	if a.Hemi == 'S' || a.Hemi == 'W' || a.Neg {
		return -whole
	}
	return whole
}

// fracValue is the fractional part, scaled by the field it belongs to.
func (a Axis) fracValue() float64 {
	f, err := strconv.ParseFloat("0."+a.Frac, 64)
	if err != nil {
		// Unreachable: Frac is digits only by construction and is capped at
		// maxFrac, so it always parses. Zero is the honest answer if it ever
		// does not.
		return 0
	}

	switch a.FracUnit {
	case FracSeconds:
		return f / 3600
	case FracMinutes:
		return f / 60
	case FracDegrees:
		return f
	default:
		return 0
	}
}

// Digits is the resolution the token carried, as a count of fractional digits.
//
// Meters are rendered from this and the format at display time. A meter count
// cannot be stored instead: six-decimal degrees is about 0.1 m, which has no
// integer meter representation, and zero would be indistinguishable from an
// absent value.
func (a Axis) Digits() int { return len(a.Frac) }

// Digits is the resolution of the pair, which is that of its coarser half.
//
// The pair is only as good as the half that was written less precisely, so this
// is what the resolution row quotes and what every derived grid row is sized
// from: a grid reference is one square and has to be the square that holds both
// halves.
//
// It must reach NOTHING that writes a value out, and there are two of those.
// canonicalString is one: writing both halves at this count silently moves the
// finer one, which for "34.5N,118.2500W" put the stored longitude 5.5 km from
// the author's. The rendered decimal, DMS and DDM rows are the other, for the
// same reason and with the same magnitude, which is why they go through
// axisResolutionDegrees instead. Both write per axis; only the resolution the
// pair is described BY is shared.
func (l Location) Digits() int {
	return min(l.Lat.Digits(), l.Lon.Digits())
}

// Confidence returns the pair's USMTF confidence digits, and whether the token
// carried any.
func (l Location) Confidence() (lat, lon int8, ok bool) {
	if l.Lat.Conf == NoConfidence || l.Lon.Conf == NoConfidence {
		return 0, 0, false
	}
	return l.Lat.Conf, l.Lon.Conf, true
}

// canonicalString builds the canonical token for a location.
//
// The property this is held to is IDEMPOTENCE, not identity: the canonical form
// of an accepted token, re-parsed, must canonicalize to itself byte for byte.
// It is emphatically not true that canonical(parse(v)) == v for every accepted
// token, and saying so would be misleading, since normalization is the point:
// "N34.0561,W118.2500" and "34.0561, -118.2500" both move. What validateParams
// depends on is the weaker property, which is why the round trip there compares
// against the canonical form rather than against the author's text.
//
// It reassembles from the integer components and Frac and must never route
// through Decimal.
func (l Location) canonicalString() string {
	var b strings.Builder

	switch l.Format {
	case FormatDD:
		// Each half keeps its OWN digits. Writing both at the pair's coarser
		// count moves the finer one: "34.5N,118.2500W" became "34.5N,118.2W"
		// and the stored longitude was 5.5 km from the one the author wrote.
		// Resolution is the coarser half (Digits), but the identity has to be
		// faithful to the text.
		writeSignedDegrees(&b, l.Lat, l.Lat.Digits())
		b.WriteByte(',')
		writeSignedDegrees(&b, l.Lon, l.Lon.Digits())

	case FormatDDH:
		// Unpadded, unlike the fixed-width USMTF shapes below. Decimal degrees
		// is a free-form notation and "018.42E" is not how anybody writes it.
		writeDegrees(&b, l.Lat, l.Lat.Digits())
		b.WriteByte(l.Lat.Hemi)
		b.WriteByte(',')
		writeDegrees(&b, l.Lon, l.Lon.Digits())
		b.WriteByte(l.Lon.Hemi)

	case FormatDMS:
		writeDMS(&b, l.Lat, 2)
		b.WriteByte(l.Lat.Hemi)
		writeDMS(&b, l.Lon, 3)
		b.WriteByte(l.Lon.Hemi)

	case FormatDDM:
		writeDDM(&b, l.Lat, 2)
		b.WriteByte(l.Lat.Hemi)
		writeDDM(&b, l.Lon, 3)
		b.WriteByte(l.Lon.Hemi)

	case FormatLATD:
		writePadded(&b, l.Lat.Deg, 2)
		b.WriteByte(l.Lat.Hemi)
		writePadded(&b, l.Lon.Deg, 3)
		b.WriteByte(l.Lon.Hemi)

	case FormatLATM:
		writePadded(&b, l.Lat.Deg, 2)
		writePadded(&b, l.Lat.Min, 2)
		b.WriteByte(l.Lat.Hemi)
		writePadded(&b, l.Lon.Deg, 3)
		writePadded(&b, l.Lon.Min, 2)
		b.WriteByte(l.Lon.Hemi)

	case FormatVLATM:
		writePadded(&b, l.Lat.Deg, 2)
		writePadded(&b, l.Lat.Min, 2)
		b.WriteByte(l.Lat.Hemi)
		b.WriteString(strconv.Itoa(int(l.Lat.Conf)))
		b.WriteByte('-')
		writePadded(&b, l.Lon.Deg, 3)
		writePadded(&b, l.Lon.Min, 2)
		b.WriteByte(l.Lon.Hemi)
		b.WriteString(strconv.Itoa(int(l.Lon.Conf)))

	case FormatMGRS:
		// The zone is written as one or two digits rather than zero-padded,
		// which is how a grid reference is actually written. It stays
		// unambiguous because what follows it is always a letter.
		b.WriteString(strconv.Itoa(l.Grid.Zone))
		b.WriteByte(l.Grid.Band)
		b.WriteByte(l.Grid.Col)
		b.WriteByte(l.Grid.Row)
		writePadded(&b, l.Grid.Easting/int(squareMeters(l.Grid.Digits)), l.Grid.Digits)
		writePadded(&b, l.Grid.Northing/int(squareMeters(l.Grid.Digits)), l.Grid.Digits)

	case FormatUTM:
		b.WriteString(strconv.Itoa(l.Grid.Zone))
		b.WriteByte(l.Grid.Band)
		writePadded(&b, l.Grid.Easting, 6)
		writePadded(&b, l.Grid.Northing, 7)

	default:
		return ""
	}

	return b.String()
}

// writeSignedDegrees writes a signed decimal-degree value with no padding,
// which is the shape a person types.
//
// The sign comes from Neg rather than from Decimal, because canonical must
// never route through a float. Reading the sign off a float would also lose it
// for -0.0000, which is a real coordinate on the equator.
func writeSignedDegrees(b *strings.Builder, a Axis, digits int) {
	if a.Neg {
		b.WriteByte('-')
	}
	b.WriteString(strconv.Itoa(a.Deg))
	writeFrac(b, a.Frac, digits)
}

// writeDegrees writes an unsigned decimal-degree value with no padding.
func writeDegrees(b *strings.Builder, a Axis, digits int) {
	b.WriteString(strconv.Itoa(a.Deg))
	writeFrac(b, a.Frac, digits)
}

func writeDMS(b *strings.Builder, a Axis, width int) {
	writePadded(b, a.Deg, width)
	writePadded(b, a.Min, 2)
	writePadded(b, a.Sec, 2)
	writeFrac(b, a.Frac, len(a.Frac))
}

func writeDDM(b *strings.Builder, a Axis, width int) {
	writePadded(b, a.Deg, width)
	writePadded(b, a.Min, 2)
	writeFrac(b, a.Frac, len(a.Frac))
}

// writeFrac writes a fractional part trimmed or padded to exactly digits.
//
// Trimming is what makes the "coarser half wins" rule reproduce itself: both
// halves are written at the pair's digit count, so re-parsing the result yields
// the same count again.
func writeFrac(b *strings.Builder, frac string, digits int) {
	if digits <= 0 {
		return
	}

	b.WriteByte('.')
	switch {
	case len(frac) >= digits:
		b.WriteString(frac[:digits])
	default:
		b.WriteString(frac)
		b.WriteString(strings.Repeat("0", digits-len(frac)))
	}
}

func writePadded(b *strings.Builder, v, width int) {
	s := strconv.Itoa(v)
	for i := len(s); i < width; i++ {
		b.WriteByte('0')
	}
	b.WriteString(s)
}
