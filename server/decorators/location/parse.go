package location

import (
	"regexp"
	"strconv"
	"strings"
)

// The anchored parsers, one per shape.
//
// Every one is built from the same sub-expressions the scanner uses, wrapped in
// anchors and given capture groups. Building them from the shared strings is
// what stops the thing a pattern matches and the thing this file can read from
// drifting apart.
var (
	ddRe = anchored(
		`([-+]?)(\d{1,2})\.(\d{4,})` + sp + `*,` + sp + `*([-+]?)(\d{1,3})\.(\d{4,})`)

	ddhSuffixRe = anchored(
		`(\d{1,2})\.(\d+)` + sp + `*` + degSym + `?` + sp + `*(` + ns + `)` + half +
			`(\d{1,3})\.(\d+)` + sp + `*` + degSym + `?` + sp + `*(` + ew + `)`)

	ddhPrefixRe = anchored(
		`(` + ns + `)` + sp + `*(\d{1,2})\.(\d+)` + sp + `*` + degSym + `?` + half +
			`(` + ew + `)` + sp + `*(\d{1,3})\.(\d+)` + sp + `*` + degSym + `?`)

	dmsCompactRe = anchored(
		`(\d{2})(\d{2})(\d{2})(?:\.(\d+))?(` + ns + `)` + half +
			`(\d{3})(\d{2})(\d{2})(?:\.(\d+))?(` + ew + `)`)

	dmsSepRe = anchored(
		`(\d{1,2})` + degSep + `(\d{1,2})` + minSep + `(\d{1,2})(?:\.(\d+))?` + secEnd + `(` + ns + `)` + half +
			`(\d{1,3})` + degSep + `(\d{1,2})` + minSep + `(\d{1,2})(?:\.(\d+))?` + secEnd + `(` + ew + `)`)

	ddmCompactRe = anchored(
		`(\d{2})(\d{2})\.(\d+)(` + ns + `)` + half + `(\d{3})(\d{2})\.(\d+)(` + ew + `)`)

	ddmSepRe = anchored(
		`(\d{1,2})` + degSep + `(\d{1,2})\.(\d+)` + sp + `*` + minSym + `?` + sp + `*(` + ns + `)` + half +
			`(\d{1,3})` + degSep + `(\d{1,2})\.(\d+)` + sp + `*` + minSym + `?` + sp + `*(` + ew + `)`)

	latdRe  = anchored(`(\d{2})(` + ns + `)(\d{3})(` + ew + `)`)
	latmRe  = anchored(`(\d{2})(\d{2})(` + ns + `)(\d{3})(\d{2})(` + ew + `)`)
	vlatmRe = anchored(`(\d{2})(\d{2})(` + ns + `)(\d)-(\d{3})(\d{2})(` + ew + `)(\d)`)

	mgrsSpacedRe = anchored(
		`(` + gridZone + `)(` + bandLetter + `)` + sp + `*(` + colLetter + `)(` + rowLetter + `)` +
			sp + `+(\d{1,5})` + sp + `+(\d{1,5})`)

	mgrsCompactRe = anchored(
		`(` + gridZone + `)(` + bandLetter + `)` + sp + `*(` + colLetter + `)(` + rowLetter + `)` +
			sp + `*(\d{2,10})`)

	utmSpacedRe = anchored(
		`(` + gridZone + `)(` + bandLetter + `)` + sp + `*(\d{6})` + utmAxisEast +
			sp + `+(\d{7})` + utmAxisNorth)

	utmCompactRe = anchored(
		`(` + gridZone + `)(` + bandLetter + `)(\d{6})(\d{7})`)
)

func anchored(expr string) *regexp.Regexp {
	return regexp.MustCompile(`^(?:` + expr + `)$`)
}

// Parse reads a token of a known format.
//
// f selects the grammar rather than being guessed at, because "try each until
// one parses" would make the format id decorative and let a crafted link claim
// a notation its token was never written in.
//
// ok=false for anything the grammar does not accept, anything out of range, and
// anything carrying more fractional digits than a float64 can reproduce.
func Parse(f Format, token string) (Location, bool) {
	switch f {
	case FormatDD:
		return parseDD(token)
	case FormatDDH:
		return parseDDH(token)
	case FormatDMS:
		return parseDMS(token)
	case FormatDDM:
		return parseDDM(token)
	case FormatLATD:
		return parseLATD(token)
	case FormatLATM:
		return parseLATM(token)
	case FormatVLATM:
		return parseVLATM(token)
	case FormatMGRS:
		return parseMGRS(token)
	case FormatUTM:
		return parseUTM(token)
	default:
		return Location{}, false
	}
}

// Canonical is the token this location is stored and validated as.
func (l Location) Canonical() string { return l.canonicalString() }

func parseDD(token string) (Location, bool) {
	m := ddRe.FindStringSubmatch(token)
	if m == nil {
		return Location{}, false
	}

	lat, ok := signedAxis(m[1], m[2], m[3], maxLatDeg)
	if !ok {
		return Location{}, false
	}
	lon, ok := signedAxis(m[4], m[5], m[6], maxLonDeg)
	if !ok {
		return Location{}, false
	}

	return finish(Location{Lat: lat, Lon: lon, Format: FormatDD})
}

func parseDDH(token string) (Location, bool) {
	if m := ddhSuffixRe.FindStringSubmatch(token); m != nil {
		return decimalDegrees(m[3], m[1], m[2], m[6], m[4], m[5])
	}
	if m := ddhPrefixRe.FindStringSubmatch(token); m != nil {
		return decimalDegrees(m[1], m[2], m[3], m[4], m[5], m[6])
	}
	return Location{}, false
}

func decimalDegrees(latHemi, latDeg, latFrac, lonHemi, lonDeg, lonFrac string) (Location, bool) {
	lat, ok := hemiAxis(latHemi, latDeg, "", "", latFrac, FracDegrees, maxLatDeg)
	if !ok {
		return Location{}, false
	}
	lon, ok := hemiAxis(lonHemi, lonDeg, "", "", lonFrac, FracDegrees, maxLonDeg)
	if !ok {
		return Location{}, false
	}

	return finish(Location{Lat: lat, Lon: lon, Format: FormatDDH})
}

func parseDMS(token string) (Location, bool) {
	m := dmsSepRe.FindStringSubmatch(token)
	if m == nil {
		m = dmsCompactRe.FindStringSubmatch(token)
	}
	if m == nil {
		return Location{}, false
	}

	lat, ok := hemiAxis(m[5], m[1], m[2], m[3], m[4], FracSeconds, maxLatDeg)
	if !ok {
		return Location{}, false
	}
	lon, ok := hemiAxis(m[10], m[6], m[7], m[8], m[9], FracSeconds, maxLonDeg)
	if !ok {
		return Location{}, false
	}

	return finish(Location{Lat: lat, Lon: lon, Format: FormatDMS})
}

func parseDDM(token string) (Location, bool) {
	m := ddmSepRe.FindStringSubmatch(token)
	if m == nil {
		m = ddmCompactRe.FindStringSubmatch(token)
	}
	if m == nil {
		return Location{}, false
	}

	lat, ok := hemiAxis(m[4], m[1], m[2], "", m[3], FracMinutes, maxLatDeg)
	if !ok {
		return Location{}, false
	}
	lon, ok := hemiAxis(m[8], m[5], m[6], "", m[7], FracMinutes, maxLonDeg)
	if !ok {
		return Location{}, false
	}

	return finish(Location{Lat: lat, Lon: lon, Format: FormatDDM})
}

func parseLATD(token string) (Location, bool) {
	m := latdRe.FindStringSubmatch(token)
	if m == nil {
		return Location{}, false
	}

	lat, ok := hemiAxis(m[2], m[1], "", "", "", FracNone, maxLatDeg)
	if !ok {
		return Location{}, false
	}
	lon, ok := hemiAxis(m[4], m[3], "", "", "", FracNone, maxLonDeg)
	if !ok {
		return Location{}, false
	}

	return finish(Location{Lat: lat, Lon: lon, Format: FormatLATD})
}

func parseLATM(token string) (Location, bool) {
	m := latmRe.FindStringSubmatch(token)
	if m == nil {
		return Location{}, false
	}

	lat, ok := hemiAxis(m[3], m[1], m[2], "", "", FracNone, maxLatDeg)
	if !ok {
		return Location{}, false
	}
	lon, ok := hemiAxis(m[6], m[4], m[5], "", "", FracNone, maxLonDeg)
	if !ok {
		return Location{}, false
	}

	return finish(Location{Lat: lat, Lon: lon, Format: FormatLATM})
}

func parseVLATM(token string) (Location, bool) {
	m := vlatmRe.FindStringSubmatch(token)
	if m == nil {
		return Location{}, false
	}

	lat, ok := hemiAxis(m[3], m[1], m[2], "", "", FracNone, maxLatDeg)
	if !ok {
		return Location{}, false
	}
	lon, ok := hemiAxis(m[7], m[5], m[6], "", "", FracNone, maxLonDeg)
	if !ok {
		return Location{}, false
	}

	// The confidence digits describe how well the position is known, not where
	// it is. They are kept out of the value and rendered beside the resolution.
	lat.Conf = confidenceDigit(m[4])
	lon.Conf = confidenceDigit(m[8])

	return finish(Location{Lat: lat, Lon: lon, Format: FormatVLATM})
}

// parseMGRS reads a military grid reference.
//
// The two shapes differ only in whether the digits are split, and they are read
// into the same fields: the split form states the two halves, the run-together
// form has to be halved, and an odd number of digits is not a grid reference in
// either case.
func parseMGRS(token string) (Location, bool) {
	var zone, band, col, row, east, north string

	switch {
	case mgrsSpacedRe.MatchString(token):
		m := mgrsSpacedRe.FindStringSubmatch(token)
		zone, band, col, row, east, north = m[1], m[2], m[3], m[4], m[5], m[6]

		// "11S LT 1234 5" names nothing: the two halves of a grid reference are
		// the same width by definition, because together they subdivide one
		// square.
		if len(east) != len(north) {
			return Location{}, false
		}

	case mgrsCompactRe.MatchString(token):
		m := mgrsCompactRe.FindStringSubmatch(token)
		zone, band, col, row = m[1], m[2], m[3], m[4]

		digits := m[5]
		if len(digits)%2 != 0 {
			return Location{}, false
		}
		east, north = digits[:len(digits)/2], digits[len(digits)/2:]

	default:
		return Location{}, false
	}

	g := Grid{
		Format: FormatMGRS,
		Zone:   atoi(zone),
		Band:   upperByte(band),
		Col:    upperByte(col),
		Row:    upperByte(row),
		Digits: len(east),
	}

	if g.Zone < 1 || g.Zone > 60 || g.Digits < 1 || g.Digits > maxGridDigits {
		return Location{}, false
	}

	size := int(squareMeters(g.Digits))
	g.Easting = atoi(east) * size
	g.Northing = atoi(north) * size

	// The geometry, which is the whole validation. The letters have to name a
	// square that exists in this zone and the square has to lie in the band the
	// token claims, and mgrsCenter answers both by trying to find the position.
	// Nothing here re-checks the letter sets separately, because a second
	// implementation of that rule is a second chance to get it wrong.
	if _, _, ok := mgrsCenter(g); !ok {
		return Location{}, false
	}

	// No Null Island rule, unlike the textual grammars. That rule exists
	// because a pair of exact zeroes is overwhelmingly an unset field rather
	// than a position; "31NAA0000000000" is not something an unset field
	// produces, it is something somebody wrote.
	return Location{Grid: g, Format: FormatMGRS}, true
}

// parseUTM reads a UTM zone, band, easting and northing.
func parseUTM(token string) (Location, bool) {
	m := utmSpacedRe.FindStringSubmatch(token)
	if m == nil {
		m = utmCompactRe.FindStringSubmatch(token)
	}
	if m == nil {
		return Location{}, false
	}

	g := Grid{
		Format:   FormatUTM,
		Zone:     atoi(m[1]),
		Band:     upperByte(m[2]),
		Easting:  atoi(m[3]),
		Northing: atoi(m[4]),
	}

	if g.Zone < 1 || g.Zone > 60 {
		return Location{}, false
	}

	if _, _, ok := utmPointOf(g); !ok {
		return Location{}, false
	}

	return Location{Grid: g, Format: FormatUTM}, true
}

// upperByte is the upper-case form of a single-character capture group.
func upperByte(s string) byte {
	if s == "" {
		// Unreachable: every caller passes a mandatory one-character group.
		// Guarded because this runs inside MessageWillBePosted, where an index
		// panic is one grammar edit away.
		return 0
	}
	return strings.ToUpper(s)[0]
}

const (
	maxLatDeg = 90
	maxLonDeg = 180
)

// signedAxis builds an axis for the one grammar that carries a sign instead of
// a hemisphere letter.
func signedAxis(sign, deg, frac string, maxDeg int) (Axis, bool) {
	a := Axis{
		Deg:      atoi(deg),
		Frac:     frac,
		FracUnit: FracDegrees,
		Neg:      sign == "-",
		Conf:     NoConfidence,
	}

	if !inRange(a, maxDeg) {
		return Axis{}, false
	}
	return a, true
}

// hemiAxis builds an axis for every grammar that carries a hemisphere letter.
//
// The letter is upper-cased here, which is the only case normalization in the
// package: the digits have no case and the symbols are matched as a class.
func hemiAxis(hemi, deg, minute, sec, frac string, unit FracUnit, maxDeg int) (Axis, bool) {
	// Every caller passes a mandatory capture group, so this cannot be empty
	// today. It is guarded anyway because this runs inside MessageWillBePosted,
	// where an index panic is one grammar edit away and the rest of this file
	// is careful about exactly that.
	if hemi == "" {
		return Axis{}, false
	}

	if frac == "" {
		unit = FracNone
	}

	a := Axis{
		Deg:      atoi(deg),
		Min:      atoi(minute),
		Sec:      atoi(sec),
		Frac:     frac,
		FracUnit: unit,
		Hemi:     strings.ToUpper(hemi)[0],
		Conf:     NoConfidence,
	}

	if !inRange(a, maxDeg) {
		return Axis{}, false
	}
	return a, true
}

// inRange is the whole numeric validation, and it is the thing the reference
// implementation in mattermost-plugin-aocanywhere does not do: without it
// "9999N99999W" parses cleanly into latitude 99.98.
func inRange(a Axis, maxDeg int) bool {
	switch {
	case a.Deg < 0 || a.Deg > maxDeg:
		return false
	case a.Min < 0 || a.Min > 59:
		return false
	case a.Sec < 0 || a.Sec > 59:
		return false
	case len(a.Frac) > maxFrac:
		return false
	}

	// At the pole or the antimeridian every smaller field must be zero, or the
	// position is past the bound the degrees claimed.
	if a.Deg == maxDeg && (a.Min != 0 || a.Sec != 0 || strings.Trim(a.Frac, "0") != "") {
		return false
	}

	return true
}

// finish applies the checks that need the pair rather than one axis.
func finish(l Location) (Location, bool) {
	// Null Island. A pair of exact zeroes is overwhelmingly a default, an unset
	// field or a parse failure somewhere upstream rather than a position in the
	// Gulf of Guinea. Note this is the pair: a zero latitude beside a real
	// longitude is an ordinary coordinate on the equator and is accepted.
	if l.Lat.Decimal() == 0 && l.Lon.Decimal() == 0 {
		return Location{}, false
	}

	return l, true
}

func atoi(s string) int {
	if s == "" {
		return 0
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		// Unreachable: every caller passes a digits-only capture group.
		return -1
	}
	return n
}

// confidenceDigit reads a USMTF verified confidence digit.
//
// Bounded explicitly rather than converted straight from the capture group: the
// pattern only ever offers one digit, but a range check here is what makes that
// true of the type rather than only of the caller.
func confidenceDigit(s string) int8 {
	n := atoi(s)
	if n < 0 || n > 9 {
		return NoConfidence
	}
	return int8(n)
}
