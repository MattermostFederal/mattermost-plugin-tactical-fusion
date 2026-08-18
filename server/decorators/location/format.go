package location

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

// Rendering rules.
//
// Every row is rendered at the resolution the token carried and never finer.
// For the decimal forms that means a decimal count; for the sexagesimal ones it
// means dropping the field the token never had rather than padding it with
// zeroes. A coordinate written to two decimals renders "34°03'N", because
// "34°03'22\"N" would be a claim about a second the author did not write.
//
// Rounding, never truncation. The reference implementation in
// mattermost-plugin-aocanywhere truncates minutes when it converts back
// (udl/peek.go), which biases every result up to a whole minute south and west.
//
// "Never finer" is a ceiling and not a floor, which is why a VALUE is rendered
// per axis while the RESOLUTION row is rendered for the pair. The two halves of
// a coordinate need not be written to the same precision: "34.0561N,118.2W" is
// an ordinary thing to paste, and rendering its latitude at the longitude's one
// decimal moved it 4.9 km north. That is the same defect canonicalString is
// held away from, for the same reason, and it is a loss of what the author
// actually wrote rather than a refusal to over-claim.
//
// So the coarser half governs how finely the PAIR is known, which is what
// ResolutionText and every derived grid row ask for, and each half governs how
// finely ITSELF is written, which is what the decimal, DMS and DDM rows ask
// for. A pair reading "34.0561° N, 118.2° W" against "about 11.1 km" is telling
// the truth twice rather than contradicting itself.

// degreeMeters is the length of a degree of latitude, near enough for a
// human-readable resolution. Longitude shortens toward the poles, which is
// exactly the sort of false precision this row exists to avoid claiming.
const degreeMeters = 111_320.0

// resolutionDegrees is the angular size of the smallest field the PAIR carried,
// which is that of its coarser half.
//
// This is what the resolution row quotes and what every derived grid row is
// sized from, because a grid reference is one square and has to be the square
// that holds the whole pair. It is emphatically NOT what a value row uses: see
// axisResolutionDegrees.
//
// The grid grammars are linear rather than angular, so theirs is converted the
// other way round: a grid reference states a size in meters directly and that
// figure is exact, where a degree count only becomes meters through the
// approximation degreeMeters already makes.
func (l Location) resolutionDegrees() float64 {
	return l.resolutionAt(l.Digits())
}

// axisResolutionDegrees is the angular size of the smallest field ONE HALF of
// the pair carried.
//
// This is what a value row renders at, so that a half written finely keeps
// every digit its author wrote even when the other half is coarse.
//
// The grammars that carry no fraction at all, and the grid grammars, have no
// per-axis notion to read: latd is whole degrees on both halves by
// construction, and a grid square's resolution is a property of the square
// rather than of either axis. They fall through to the pair's figure, which for
// them is the same number.
func (l Location) axisResolutionDegrees(a Axis) float64 {
	switch l.Format {
	case FormatDD, FormatDDH, FormatDDM, FormatDMS:
		return l.resolutionAt(a.Digits())
	default:
		return l.resolutionDegrees()
	}
}

// resolutionAt is the angular size a format's smallest field has at a given
// count of fractional digits.
//
// Shared by both callers above so the pair-wide and per-axis figures cannot
// drift into using different ladders.
func (l Location) resolutionAt(digits int) float64 {
	d := float64(digits)

	switch l.Format {
	case FormatDD, FormatDDH:
		return math.Pow(10, -d)
	case FormatLATD:
		return 1
	case FormatLATM, FormatVLATM:
		return 1.0 / 60
	case FormatDDM:
		return math.Pow(10, -d) / 60
	case FormatDMS:
		return math.Pow(10, -d) / 3600
	case FormatMGRS:
		return squareMeters(l.Grid.Digits) / degreeMeters
	case FormatUTM:
		return 1 / degreeMeters
	case FormatGEOREF, FormatGARS, FormatPlusCode:
		return areaResolutionDegrees(l.Area)
	default:
		return 1
	}
}

// resolutionMeters is the same figure on the ground.
func (l Location) resolutionMeters() float64 {
	return l.resolutionDegrees() * degreeMeters
}

// gridDigitsFor is how many digits per axis a grid reference may carry without
// claiming more than the token it was derived from.
//
// The largest square that is no bigger than the resolution: a coordinate
// written to about 11 m gets a 10 m square, and one written to 1.9 km gets a
// 1 km square. Going finer would invent precision and going coarser would throw
// away what the author did write.
//
// Zero digits, a bare 100 km square, is reachable here but is not a token this
// package will accept from a message. That is not an inconsistency: a coarse
// coordinate honestly renders as a 100 km square, while "11SLT" typed into a
// channel is five characters and far too weak to be worth the risk of matching.
func gridDigitsFor(meters float64) int {
	for d := range maxGridDigits {
		if squareMeters(d) <= meters {
			return d
		}
	}

	// Finer than a meter, which is where the notation itself stops.
	return maxGridDigits
}

// decimalPlaces is how many decimal degrees express one half's resolution
// without inventing any.
func (l Location) decimalPlaces(a Axis) int {
	return clampPlaces(math.Ceil(-math.Log10(l.axisResolutionDegrees(a))))
}

// clampPlaces bounds a computed width before it reaches a formatter.
//
// Parse caps fractional digits at maxFrac, so nothing built through it can go
// wide. Axis is exported with exported fields and no constructor, though, and a
// Frac long enough underflows resolutionDegrees to zero, making Log10 return
// -Inf and int(+Inf) undefined. Formatting at that width does not return.
func clampPlaces(places float64) int {
	if math.IsNaN(places) || places <= 0 {
		return 0
	}
	if places > maxFrac+4 {
		return maxFrac + 4
	}
	return int(places)
}

// DecimalText renders the pair as decimal degrees with hemisphere letters, at
// the token's own resolution.
//
// Empty when the position cannot be derived, which only a grid token can
// manage. Every caller has to handle that rather than showing a zero: a
// coordinate that reads "0.0000 N, 0.0000 E" because a conversion failed is
// worse than one that admits it is missing.
func (l Location) DecimalText() string {
	lat, lon, ok := l.Point()
	if !ok {
		return ""
	}

	return decimalAxis(lat, l.decimalPlaces(l.Lat), "N", "S") + ", " +
		decimalAxis(lon, l.decimalPlaces(l.Lon), "E", "W")
}

func decimalAxis(v float64, places int, pos, neg string) string {
	hemi := pos
	if math.Signbit(v) {
		hemi = neg
	}
	return strconv.FormatFloat(math.Abs(v), 'f', places, 64) + "° " + hemi
}

// DMSText renders the pair as degrees, minutes and seconds, showing only the
// fields the token's resolution supports.
func (l Location) DMSText() string {
	lat, lon, ok := l.Point()
	if !ok {
		return ""
	}

	return dmsAxis(lat, l.axisResolutionDegrees(l.Lat), "N", "S") + " " +
		dmsAxis(lon, l.axisResolutionDegrees(l.Lon), "E", "W")
}

func dmsAxis(v float64, resolution float64, pos, neg string) string {
	hemi := pos
	if math.Signbit(v) {
		hemi = neg
	}

	switch {
	case resolution >= 1:
		deg := int(math.Round(math.Abs(v)))
		return fmt.Sprintf("%d°%s", deg, hemi)

	case resolution >= 1.0/60:
		deg, minutes := degMin(math.Abs(v))
		return fmt.Sprintf("%d°%02d'%s", deg, minutes, hemi)

	default:
		// Fractional seconds only when the token was written finer than one.
		places := 0
		if resolution < 1.0/3600 {
			places = clampPlaces(math.Ceil(-math.Log10(resolution * 3600)))
		}
		deg, minutes, seconds := degMinSec(math.Abs(v), places)
		return fmt.Sprintf("%d°%02d'%0*.*f\"%s", deg, minutes, fieldWidth(places), places, seconds, hemi)
	}
}

// DDMText renders the pair as degrees and decimal minutes.
func (l Location) DDMText() string {
	lat, lon, ok := l.Point()
	if !ok {
		return ""
	}

	return ddmAxis(lat, l.axisResolutionDegrees(l.Lat), "N", "S") + " " +
		ddmAxis(lon, l.axisResolutionDegrees(l.Lon), "E", "W")
}

// USMTFText renders the position as a USMTF compact coordinate.
//
// USMTF names a family rather than a format, so the shape is chosen by the
// token's resolution on the same principle gridDigitsFor uses to size a grid
// square: the coarsest field set whose step is no coarser than what the author
// wrote. Padding a field nobody wrote is a claim, and LATM on a degrees-only
// token would be exactly that.
//
//	whole degrees         LATD    35N079W
//	whole minutes         LATM    3510N07901W
//	whole seconds         LATS    400948N1221400W
//	finer than a second   LATDS   331000.0N1183000.0W
//
// Sized from the PAIR rather than per axis, unlike the DMS and DDM rows. A
// USMTF token is one fixed-width shape covering both halves, so there is no
// spelling of it in which latitude carries seconds and longitude carries only
// minutes. The coarser half therefore governs, which is the same reason the
// grid rows are sized that way.
//
// Rounding, never truncation. LatLonToLATM in mattermost-plugin-aocanywhere
// truncates, which biases every result up to a whole minute south and west; the
// helpers below are shared with the DMS row precisely so that this cannot drift
// into a second answer.
//
// The verified variants are not reachable from here. Confidence says how well a
// position is KNOWN, and a derived reading knows nothing about that, so the
// plain shape is the only honest one. A token that carried confidence still
// shows it in its own row.
func (l Location) USMTFText() string {
	lat, lon, ok := l.Point()
	if !ok {
		return ""
	}

	resolution := l.resolutionDegrees()

	return usmtfAxis(lat, resolution, 2, "N", "S") +
		usmtfAxis(lon, resolution, 3, "E", "W")
}

// usmtfAxis renders one half at a fixed width.
//
// degWidth is 2 for latitude and 3 for longitude, which is the whole difference
// between the two halves of every shape in the family.
func usmtfAxis(v float64, resolution float64, degWidth int, pos, neg string) string {
	hemi := pos
	if math.Signbit(v) {
		hemi = neg
	}

	switch {
	case resolution >= 1:
		return fmt.Sprintf("%0*d%s", degWidth, int(math.Round(math.Abs(v))), hemi)

	case resolution >= 1.0/60:
		deg, minutes := degMin(math.Abs(v))
		return fmt.Sprintf("%0*d%02d%s", degWidth, deg, minutes, hemi)

	default:
		// Fractional seconds only when the token was written finer than one,
		// which is what separates LATDS from LATS.
		places := 0
		if resolution < 1.0/3600 {
			places = clampPlaces(math.Ceil(-math.Log10(resolution * 3600)))
		}
		deg, minutes, seconds := degMinSec(math.Abs(v), places)
		return fmt.Sprintf("%0*d%02d%0*.*f%s",
			degWidth, deg, minutes, fieldWidth(places), places, seconds, hemi)
	}
}

// The renderers below are the Go half of the paired fixtures.
//
// No page reaches them: every surface renders through format.ts since the Go
// page renderer went. They stay because renderFixtures and areaFixtures are the
// same inputs and the same expected strings as the table in format.spec.ts, and
// that pairing is what keeps the two implementations honest. Deleting them
// would leave the TypeScript side asserting against nothing.

// ResolutionText is the human reading of how finely the token was written.
//
// Deliberately "about", not "accurate to". A phone emitting six decimals is not
// claiming 0.1 m of accuracy, and a row headed "precision" invites exactly that
// misreading. "about" hedges; the row label already supplies the noun.
func (l Location) ResolutionText() string {
	meters := l.resolutionMeters()

	switch l.Format {
	case FormatMGRS:
		// A grid reference is a square and the position reported for it is the
		// center, which the row has to say: read as a corner, a 10 km reference
		// puts a reader 7 km from where the grid points.
		return humanMeters(meters) + " grid, at center"
	case FormatUTM:
		// Exact rather than hedged, unlike the forms above: a UTM easting is
		// written to the meter and means it.
		return "1 m"

	case FormatGEOREF, FormatGARS, FormatPlusCode:
		return humanMeters(meters) + " cell, at center"
	}

	// Below a centimeter the figure rounds to "0 m" at any sane width, which
	// reads as a claim of infinite precision rather than as the very fine
	// number it is. An eight-decimal token really does reach this.
	if meters < 0.01 {
		return "finer than 0.01 m"
	}

	return "about " + humanMeters(meters)
}

func humanMeters(m float64) string {
	switch {
	case m >= 1000:
		return trimZeroes(strconv.FormatFloat(m/1000, 'f', 1, 64)) + " km"
	case m >= 1:
		return strconv.FormatFloat(m, 'f', 0, 64) + " m"
	default:
		// ResolutionText refuses anything below a centimeter before it reaches
		// here, so two decimals is always enough to say something.
		return trimZeroes(strconv.FormatFloat(m, 'f', 2, 64)) + " m"
	}
}

func trimZeroes(s string) string {
	if !strings.Contains(s, ".") {
		return s
	}
	return strings.TrimSuffix(strings.TrimRight(s, "0"), ".")
}

// ConfidenceText describes the USMTF verified digits, or "" when the token
// carried none.
//
// Kept separate from the resolution: how well a position is known and how
// finely it was written are different facts, and folding one into the other
// would claim something the token does not say.
func (l Location) ConfidenceText() string {
	lat, lon, ok := l.Confidence()
	if !ok {
		return ""
	}
	return fmt.Sprintf("stated confidence %d (latitude), %d (longitude)", lat, lon)
}

// MGRSText is the military grid reference for this location, at the token's own
// resolution and no finer.
//
// Empty when the position is outside the grid, which is the polar regions: past
// 84 north and 80 south the notation is Universal Polar Stereographic, a
// different system this package does not implement. A blank row is the honest
// answer there.
func (l Location) MGRSText() string {
	// A grid reference is its own answer. Deriving it back from its own center
	// would project twice for nothing, and near a zone boundary a square's
	// center re-encodes into the neighboring zone, so the round trip would
	// occasionally rewrite the reference the author wrote.
	if l.Format == FormatMGRS {
		return gridText(l.Grid)
	}

	digits := gridDigitsFor(l.resolutionMeters())

	// A UTM token is already on the grid, so this is a relabelling rather than
	// a conversion.
	if l.Format == FormatUTM {
		p, ok := gridPoint(l.Grid)
		if !ok {
			return ""
		}
		g, ok := mgrsAt(p, digits)
		if !ok {
			return ""
		}
		return gridText(g)
	}

	lat, lon, ok := l.Point()
	if !ok {
		return ""
	}

	g, ok := mgrsFor(lat, lon, digits)
	if !ok {
		return ""
	}
	return gridText(g)
}

// gridStep is the rounding step a derived grid value uses, in meters.
//
// The decimal ladder the grid itself is built on rather than the raw
// resolution: rounding a 111 km resolution to the nearest 111,320 m produces
// "667920", which is precise-looking noise. Rounding to the nearest 100 km
// produces "700000", which reads as the coarse figure it is.
func (l Location) gridStep() float64 {
	return squareMeters(gridDigitsFor(l.resolutionMeters()))
}

// UTMText is the UTM grid position, at the token's own resolution.
//
// Rounded to that resolution rather than always to the meter, for the same
// reason every other row is: "35N079W" is a 111 km square, and reporting a
// six-figure easting for it would claim a position the author never wrote.
func (l Location) UTMText() string {
	if l.Format == FormatUTM {
		return gridText(l.Grid)
	}

	step := l.gridStep()

	if l.Format == FormatMGRS {
		p, ok := gridPoint(l.Grid)
		if !ok {
			return ""
		}

		// The digits of a grid reference are literally its south-west corner on
		// the UTM grid, so the two rows are the same numbers written two ways
		// and must not be made to disagree. gridPoint returns the center, which
		// is the right answer for a position and the wrong one here: quoting it
		// would put the UTM easting of "MV 12" at 412500 against a square that
		// runs from 412000, half a square adrift and looking like a conversion
		// error.
		half := squareMeters(l.Grid.Digits) / 2
		p.Easting -= half
		p.Northing -= half

		g, ok := utmAt(p, step)
		if !ok {
			return ""
		}
		return gridText(g)
	}

	lat, lon, ok := l.Point()
	if !ok {
		return ""
	}

	g, ok := utmFor(lat, lon, step)
	if !ok {
		return ""
	}
	return gridText(g)
}

// gridText is how a grid reference is written for a reader: spaced into its
// parts, which is how they are read aloud and checked against a map.
//
// The canonical form in the URL is the same characters without the spaces, so
// this and Canonical differ only in whitespace.
func gridText(g Grid) string {
	head := strconv.Itoa(g.Zone) + string(g.Band)

	if g.Format == FormatUTM {
		return head + " " + fmt.Sprintf("%06dE %07dN", g.Easting, g.Northing)
	}

	head += " " + string(g.Col) + string(g.Row)
	if g.Digits == 0 {
		// A 100 km square, which only ever arrives here as a derived value for
		// a coordinate too coarse to say more.
		return head
	}

	size := int(squareMeters(g.Digits))
	return fmt.Sprintf("%s %0*d %0*d", head,
		g.Digits, g.Easting/size, g.Digits, g.Northing/size)
}

func ddmAxis(v float64, resolution float64, pos, neg string) string {
	hemi := pos
	if math.Signbit(v) {
		hemi = neg
	}

	if resolution >= 1 {
		return fmt.Sprintf("%d°%s", int(math.Round(math.Abs(v))), hemi)
	}

	places := 0
	if resolution < 1.0/60 {
		places = clampPlaces(math.Ceil(-math.Log10(resolution * 60)))
	}

	deg, minutes := degDecimalMin(math.Abs(v), places)
	return fmt.Sprintf("%d°%0*.*f'%s", deg, fieldWidth(places), places, minutes, hemi)
}

// fieldWidth keeps the leading zero on a two-digit field once a decimal point
// is added: "05.5" rather than "5.5".
func fieldWidth(places int) int {
	if places == 0 {
		return 2
	}
	return 3 + places
}

// degMin splits a positive decimal degree into whole degrees and minutes,
// rounding rather than truncating and carrying whatever the rounding produces.
func degMin(v float64) (int, int) {
	deg := int(v)
	minutes := int(math.Round((v - float64(deg)) * 60))
	if minutes >= 60 {
		minutes -= 60
		deg++
	}
	return deg, minutes
}

func degDecimalMin(v float64, places int) (int, float64) {
	deg := int(v)
	minutes := roundTo((v-float64(deg))*60, places)
	if minutes >= 60 {
		minutes -= 60
		deg++
	}
	return deg, minutes
}

func degMinSec(v float64, places int) (int, int, float64) {
	deg := int(v)
	rest := (v - float64(deg)) * 60
	minutes := int(rest)
	seconds := roundTo((rest-float64(minutes))*60, places)

	if seconds >= 60 {
		seconds -= 60
		minutes++
	}
	if minutes >= 60 {
		minutes -= 60
		deg++
	}
	return deg, minutes, seconds
}

// roundTo rounds to a fixed number of decimal places, and never returns
// negative zero.
//
// The normalization is not tidiness. On arm64 the compiler is permitted to
// contract a multiply and a subtract into a single FMA, which Go's spec allows,
// so the seconds residue in degMinSec comes out at about -4e-14 rather than
// zero. math.Round keeps that sign, and fmt renders the result "-0", filling
// the field width exactly so nothing looks wrong until the token is read back:
// "0°01'-0\"N" from the DMS row, and "0001-0N1000100E" from the USMTF row,
// which no USMTF grammar accepts. It reached 0.13% of two-decimal hemisphere
// coordinates, and the same build on amd64 produced the correct value, so the
// page and the panel disagreed about the same link.
//
// Normalized here rather than at the call sites because this is the single
// place every rounded field passes through, including degDecimalMin, and
// because the fusion happens upstream of any cast a caller could apply.
func roundTo(v float64, places int) float64 {
	scale := math.Pow(10, float64(places))

	out := math.Round(v*scale) / scale
	if out == 0 {
		return 0
	}
	return out
}

func (l Location) GEOREFText() string {
	return l.areaText(FormatGEOREF, func(lat, lon float64, resolution float64) string {
		return georefAt(lat, lon, georefDigitsFor(resolution))
	})
}

func (l Location) GARSText() string {
	return l.areaText(FormatGARS, func(lat, lon float64, resolution float64) string {
		return garsAt(lat, lon, garsMinutesFor(resolution))
	})
}

func (l Location) PlusCodeText() string {
	return l.areaText(FormatPlusCode, func(lat, lon float64, resolution float64) string {
		return plusCodeAt(lat, lon, plusCodeLengthFor(resolution))
	})
}

func (l Location) areaText(f Format, encode func(lat, lon, resolution float64) string) string {
	if l.Format == f {
		return l.Area.Code
	}

	lat, lon, ok := l.Point()
	if !ok {
		return ""
	}

	return encode(lat, lon, l.resolutionDegrees())
}
