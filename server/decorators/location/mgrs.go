package location

import "math"

// The Military Grid Reference System, which is UTM with the leading digits of
// the easting and northing replaced by a pair of letters.
//
// A grid reference names a square, not a point. "11SLT1234567890" is a one
// meter square; "11SLT12" is a ten kilometer one. Everything downstream treats
// it that way: the position this package reports for a grid is the center of
// its square, and the resolution row says how big the square is. Reporting the
// south-west corner instead would put a ten kilometer reference seven
// kilometers from where a reader would point at it on a map.

// gridSquareSetCols are the 100 km column letters, in three sets that repeat
// every three zones.
//
// I and O are absent throughout, as they are everywhere in this notation.
var gridSquareSetCols = [3]string{
	"ABCDEFGH",
	"JKLMNPQR",
	"STUVWXYZ",
}

// gridSquareRows are the 100 km row letters. Twenty of them, so the sequence
// repeats every 2,000,000 meters of northing, which is what makes a decoded
// northing a candidate rather than an answer.
const gridSquareRows = "ABCDEFGHJKLMNPQRSTUV"

// rowOffsetEvenZone staggers the row letters in even-numbered zones.
//
// Without it two squares directly across a zone boundary from each other would
// carry the same pair of letters, and a reference read off the wrong side of
// the line would be indistinguishable from a correct one.
const rowOffsetEvenZone = 5

// maxGridDigits is the finest grid reference: five digits per axis, a one meter
// square. Nothing finer exists in the notation.
const maxGridDigits = 5

// The eastings a 100 km column can occupy, which is what the column letters
// encode: eight of them, starting at 100 km.
const (
	minGridEasting = 100000
	maxGridEasting = 900000
)

// zoneNear reports whether a position is in the zone a token claimed, or in one
// either side of it.
//
// Not an equality, and the slop is the point rather than laziness. Grid squares
// are clipped at zone boundaries, so a square against one legitimately has its
// center in the neighbor; and the two hand-widened regions mean the zone a
// position naturally falls in and the zone it is correctly written in differ by
// design. What this does catch is a token whose numbers put it hundreds of
// kilometers from the zone it names.
func zoneNear(zone int, lat, lon float64) bool {
	natural, ok := zoneFor(lat, lon)
	if !ok {
		return false
	}

	// Zones wrap at the antimeridian: 60 and 1 are neighbors.
	diff := abs(natural - zone)
	return diff <= 1 || diff == 59
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

// gridSquareFor is the 100 km column and row letters for a UTM position.
func gridSquareFor(p utmPoint) (col, row byte, ok bool) {
	colIdx := int(math.Floor(p.Easting/100000)) - 1
	if colIdx < 0 || colIdx >= len(gridSquareSetCols[0]) {
		return 0, 0, false
	}
	col = gridSquareSetCols[(p.Zone-1)%3][colIdx]

	rowIdx := int(math.Floor(p.Northing/100000)) % len(gridSquareRows)
	if rowIdx < 0 {
		return 0, 0, false
	}
	if p.Zone%2 == 0 {
		rowIdx = (rowIdx + rowOffsetEvenZone) % len(gridSquareRows)
	}
	row = gridSquareRows[rowIdx]

	return col, row, true
}

// columnIndex is the position of a 100 km column letter within its zone's set,
// or -1 when the letter is not in that set at all.
//
// This is the check the reference implementation in mattermost-plugin-aocanywhere
// does not do: its pattern is `^\d{1,2}[A-Z]{3}\d{6}$`, which accepts any three
// letters. Only eight of the twenty-four possible column letters are legal in
// any given zone, so two thirds of the letters that pattern accepts name no
// square at all.
func columnIndex(zone int, col byte) int {
	if zone < 1 || zone > 60 {
		return -1
	}

	set := gridSquareSetCols[(zone-1)%3]
	for i := range len(set) {
		if set[i] == col {
			return i
		}
	}
	return -1
}

// rowIndex is the position of a 100 km row letter for a zone, or -1.
func rowIndex(zone int, row byte) int {
	for i := range len(gridSquareRows) {
		if gridSquareRows[i] != row {
			continue
		}

		idx := i
		if zone%2 == 0 {
			idx = (idx - rowOffsetEvenZone + len(gridSquareRows)) % len(gridSquareRows)
		}
		return idx
	}
	return -1
}

// squareMeters is the size of a grid square with this many digits per axis.
//
// Clamped to the range the notation has, because callers divide by int(this).
// Parse bounds Digits to 1..5 and gridDigitsFor returns 0..5, but Grid is
// exported with exported fields and no constructor, so a hand-built one with
// six digits reaches canonicalString and int(0.1) is 0. That is the same hazard
// clampPlaces exists for, on the one struct that was left without it.
func squareMeters(digits int) float64 {
	if digits < 0 {
		digits = 0
	}
	if digits > maxGridDigits {
		digits = maxGridDigits
	}

	return 100000 / math.Pow(10, float64(digits))
}

// gridPoint is the position on the UTM grid that a grid token names.
//
// Everything about a grid token goes through here, and it stays in grid
// coordinates deliberately. Converting a UTM token to a grid reference, or the
// other way, is a relabelling of the same easting and northing and involves no
// geodesy at all; routing it through latitude and longitude instead would
// project twice and land a meter out. It really did: "33U 291000 5628000" came
// back as "33U TS 90999 28000", one meter short, because an easting sitting
// exactly on a cell boundary lost a fraction of a millimeter in the round trip
// and then had it truncated away.
func gridPoint(g Grid) (utmPoint, bool) {
	if g.Zone < 1 || g.Zone > 60 || bandIndex(g.Band) < 0 {
		return utmPoint{}, false
	}

	if g.Format == FormatUTM {
		// The easting has to name a 100 km column that exists.
		//
		// MGRS gets this for free: its column letter is one of eight, so the
		// easting is structurally inside [100000, 900000). UTM has no letters,
		// and without this a token like "31P 000000 1100000" parsed, because
		// nothing but the band was checked. It then reported an empty MGRS row,
		// which everywhere else in this package means "outside the grid" and
		// here would have been a lie, and it put the point 500 km off its
		// central meridian, well outside the range the truncated series is
		// documented to hold to.
		if g.Easting < minGridEasting || g.Easting >= maxGridEasting {
			return utmPoint{}, false
		}

		p := utmPoint{
			Zone:     g.Zone,
			Band:     g.Band,
			Easting:  float64(g.Easting),
			Northing: float64(g.Northing),
		}

		lat, lon, ok := unprojectUTM(p)
		if !ok || !withinBand(g.Band, lat, 1) {
			return utmPoint{}, false
		}

		// And the position has to fall in the zone the token named. A UTM pair
		// is almost always a valid position somewhere, so unlike MGRS there is
		// nothing in the digits themselves to check; this is the only thing
		// tying the numbers to the zone in front of them.
		//
		// One zone of tolerance, because a grid square against a zone boundary
		// legitimately spills into the neighbor, and because the widened zones
		// (Norway, Svalbard) make the natural zone and the written one differ
		// by design.
		if !zoneNear(g.Zone, lat, lon) {
			return utmPoint{}, false
		}

		return p, true
	}

	return mgrsGridPoint(g)
}

// mgrsGridPoint resolves the center of a grid square onto the UTM grid.
//
// The 100 km row letter repeats every 2,000,000 meters, so the northing it
// implies is one of five candidates and the latitude band is what chooses
// between them. Rather than carrying a hand-copied table of band northings,
// this projects each candidate back and keeps the one that lands in the band
// the token named. A table would be one more thing to get quietly wrong; the
// projection is already here and already tested.
func mgrsGridPoint(g Grid) (utmPoint, bool) {
	colIdx := columnIndex(g.Zone, g.Col)
	rowIdx := rowIndex(g.Zone, g.Row)
	if colIdx < 0 || rowIdx < 0 {
		return utmPoint{}, false
	}
	if g.Digits < 1 || g.Digits > maxGridDigits {
		return utmPoint{}, false
	}

	size := squareMeters(g.Digits)
	half := size / 2
	easting := float64((colIdx+1)*100000+g.Easting) + half

	const cycle = 2000000
	baseNorthing := float64(rowIdx*100000 + g.Northing)

	var out utmPoint
	var found bool

	for k := 0; ; k++ {
		northing := baseNorthing + float64(k)*cycle + half

		// The grid stops at the pole. Past it the inverse series is not merely
		// inaccurate, it is meaningless, so the ceiling is part of enumerating
		// the candidates rather than something unprojectUTM is left to notice.
		if northing > utmFalseNorthing {
			break
		}

		candidate := utmPoint{
			Zone:     g.Zone,
			Band:     g.Band,
			Easting:  easting,
			Northing: northing,
		}

		lat, _, converted := unprojectUTM(candidate)

		// Half a square of slop, because a square on a band boundary has its
		// center on one side of the line while belonging to the band on the
		// other.
		if !converted || !withinBand(g.Band, lat, half) {
			continue
		}

		if found {
			// Two candidates in one band would mean the row letters repeat
			// inside a band, which they cannot: the cycle is 2,000,000 meters
			// and the tallest band is under 1,400,000. Declining rather than
			// picking one keeps that assumption from failing silently if the
			// band table is ever edited.
			return utmPoint{}, false
		}

		out, found = candidate, true
	}

	return out, found
}

// mgrsCenter is the geographic position of the center of a grid square.
func mgrsCenter(g Grid) (lat, lon float64, ok bool) {
	p, ok := mgrsGridPoint(g)
	if !ok {
		return 0, 0, false
	}
	return unprojectUTM(p)
}

// mgrsFor is the grid reference for a geographic position, at a given number of
// digits per axis.
//
// This is the ENCODE direction, used to derive a grid row for a coordinate
// written in some other notation. It is not what validates a grid token:
// parseMGRS calls mgrsCenter and accepts a token when a position can be found
// for it, without re-encoding. Those are deliberately different, because
// re-encoding is not stable at a zone boundary, and because the widened Norway
// and Svalbard zones mean the zone a position naturally falls in and the zone
// it is correctly written in can differ. A token in a widened zone therefore
// parses and reports the right position while mgrsFor on that same position
// names the other zone, which is correct behavior for both.
func mgrsFor(lat, lon float64, digits int) (Grid, bool) {
	p, ok := projectUTM(lat, lon)
	if !ok {
		return Grid{}, false
	}
	return mgrsAt(p, digits)
}

// mgrsAt is the grid reference for a position already on the UTM grid.
//
// Split out from mgrsFor because a UTM token is already here: relabelling it as
// a grid reference is arithmetic on the easting and northing and must not go
// back through the projection to get there.
func mgrsAt(p utmPoint, digits int) (Grid, bool) {
	if digits < 0 || digits > maxGridDigits {
		return Grid{}, false
	}

	col, row, ok := gridSquareFor(p)
	if !ok {
		return Grid{}, false
	}

	size := squareMeters(digits)

	// Truncation, not rounding: a grid square is the square a position is
	// inside, and rounding would name the neighboring one for anything in the
	// upper half of it. This is the one place in the package where truncating
	// is right, and it is right for a different reason than the rendering rule
	// that forbids it: there we are choosing how to write a number, here we are
	// choosing which box a point sits in.
	within := func(v float64) int {
		return int(math.Floor(math.Mod(v, 100000) / size))
	}

	return Grid{
		Format:   FormatMGRS,
		Zone:     p.Zone,
		Band:     p.Band,
		Col:      col,
		Row:      row,
		Easting:  within(p.Easting) * int(size),
		Northing: within(p.Northing) * int(size),
		Digits:   digits,
	}, true
}

// utmFor is the UTM grid position for a geographic position.
//
// Unlike a grid reference this names a point rather than a square, which is the
// actual difference between the two notations rather than a choice made here:
// an MGRS token's digits say how big its square is, while a UTM easting is a
// distance.
//
// That is also why this rounds where mgrsAt truncates. Both follow from the
// same rule, which is that a rendered value may not claim more than the token
// it came from: a square is chosen by containment, a distance by nearness.
func utmFor(lat, lon float64, step float64) (Grid, bool) {
	p, ok := projectUTM(lat, lon)
	if !ok {
		return Grid{}, false
	}
	return utmAt(p, step)
}

// utmAt rounds a position on the grid to a step in meters.
func utmAt(p utmPoint, step float64) (Grid, bool) {
	if step < 1 {
		step = 1
	}

	round := func(v float64) int {
		return int(math.Round(v/step) * step)
	}

	easting, northing := round(p.Easting), round(p.Northing)

	// The southern hemisphere is measured from a false origin 10,000 km south
	// of the equator, so the equator itself IS 10,000,000 in that frame. A
	// point a fraction south of it rounds to exactly that and does not fit in
	// the seven digits a UTM northing is written with: "-0.000001,32.500000"
	// lost its UTM row while its MGRS row rendered fine, and an empty UTM row
	// means "outside the grid" everywhere else in this package.
	//
	// Clamped one meter south rather than wrapped to zero. Zero beside a
	// southern band letter is not "the equator", it is the false origin itself,
	// 10,000 km away, and it produced a row this package could not parse back.
	if northing == int(utmFalseNorthing) && !bandNorthern(p.Band) {
		northing = int(utmFalseNorthing) - 1
	}

	if easting < 0 || easting > 999999 || northing < 0 || northing > 9999999 {
		return Grid{}, false
	}

	return Grid{
		Format:   FormatUTM,
		Zone:     p.Zone,
		Band:     p.Band,
		Easting:  easting,
		Northing: northing,
	}, true
}

// utmPointOf is the geographic position a UTM token names.
//
// The band has to contain the position, or the letter the author wrote and the
// northing they wrote would be describing two different places and the page
// would render both without noticing.
func utmPointOf(g Grid) (lat, lon float64, ok bool) {
	p, ok := gridPoint(g)
	if !ok {
		return 0, 0, false
	}
	return unprojectUTM(p)
}
