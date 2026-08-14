package location

import "math"

// The WGS 84 ellipsoid and the transverse Mercator series UTM is defined on.
//
// Hand-written rather than taken from a dependency, and that is a decision
// rather than an oversight. This plugin targets air-gapped installs and gates
// every release on an SBOM and a CVE scan, so a dependency is a permanent cost
// paid by the operator; the maths below is a closed, published series that has
// not changed since 1987 and cannot go stale. What hand-writing it buys in
// exchange is the obligation to prove it, which is why geodesy_test.go checks
// against external anchors (the WGS 84 meridian quadrant, the exact easting on
// a central meridian, the exact northing on the equator) rather than against
// round trips of this code with itself. A round trip proves the inverse undoes
// the forward and says nothing at all about whether either one is right.
//
// Snyder, "Map Projections: A Working Manual" (USGS Professional Paper 1395),
// equations 3-21 and 8-9 through 8-25. The series is truncated at e^6, which is
// accurate to roughly a millimeter within the 6 degrees a UTM zone spans. That
// is three orders of magnitude finer than the finest grid this package will
// render. What keeps a point from being projected far enough off its own
// central meridian for the truncation to matter is zoneFor on the way in, and
// on the way back the easting bound and zoneNear check in gridPoint, which is
// the path a UTM token takes.
const (
	// wgs84A is the semi-major axis in meters.
	wgs84A = 6378137.0

	// wgs84F is the flattening, as its reciprocal.
	wgs84InvF = 298.257223563

	// utmK0 is the scale factor on the central meridian.
	utmK0 = 0.9996

	// utmFalseEasting keeps every easting in a zone positive.
	utmFalseEasting = 500000.0

	// utmFalseNorthing does the same for the southern hemisphere.
	utmFalseNorthing = 10000000.0
)

var (
	wgs84F  = 1 / wgs84InvF
	wgs84E2 = wgs84F * (2 - wgs84F)

	// wgs84EP2 is the second eccentricity squared, e'^2 = e^2/(1-e^2).
	wgs84EP2 = wgs84E2 / (1 - wgs84E2)
)

// The latitude span UTM covers. Beyond it the projection is the Universal
// Polar Stereographic grid, which is a different system with a different
// notation, so a polar coordinate is declined rather than approximated.
const (
	minUTMLat = -80.0
	maxUTMLat = 84.0
)

// bandLetters are the UTM latitude bands from -80 upward, 8 degrees each except
// X, which is 12.
//
// I and O are absent because they read as 1 and 0. A and B are the southern
// polar zones and Y and Z the northern ones, none of which is UTM.
const bandLetters = "CDEFGHJKLMNPQRSTUVWX"

// bandFor is the latitude band a point falls in.
func bandFor(lat float64) (byte, bool) {
	if math.IsNaN(lat) || lat < minUTMLat || lat >= maxUTMLat {
		return 0, false
	}

	// X is the one band that is not 8 degrees tall: it was widened to 12 so
	// that the northern limit reaches the whole of Svalbard and northern
	// Greenland rather than stopping in the middle of them.
	if lat >= 72 {
		return 'X', true
	}

	idx := int(math.Floor((lat + 80) / 8))
	if idx < 0 || idx >= len(bandLetters) {
		return 0, false
	}
	return bandLetters[idx], true
}

// bandIndex is the position of a band letter, or -1.
func bandIndex(b byte) int {
	for i := range len(bandLetters) {
		if bandLetters[i] == b {
			return i
		}
	}
	return -1
}

// bandNorthern reports whether a band is north of the equator.
//
// This is the whole reason a UTM token has to carry a band letter: the northing
// alone cannot say which hemisphere it is in, because the southern hemisphere
// is measured from a false origin 10,000 km south of the equator and every
// value collides with a northern one.
func bandNorthern(b byte) bool {
	return bandIndex(b) >= bandIndex('N')
}

// bandLatitudes are the bounds of a band, which is what makes a decoded grid
// reference checkable: a northing recovered from a 100 km letter is only one of
// several candidates, and the band is what picks between them.
func bandLatitudes(b byte) (south, north float64, ok bool) {
	idx := bandIndex(b)
	if idx < 0 {
		return 0, 0, false
	}

	south = minUTMLat + float64(idx)*8
	north = south + 8
	if b == 'X' {
		north = maxUTMLat
	}
	return south, north, true
}

// withinBand reports whether a latitude belongs to a band, allowing the token's
// own granularity as slop.
//
// The slop is not a fudge factor, it is the notation. A grid square straddling
// a band boundary has its center on one side or the other, so a 10 km square
// can legitimately report a center 5 km outside the band its letter names; and
// a UTM northing written to the meter, rounded to the meter, can land a
// fraction of a meter across the line it started on. Without this a coordinate
// on a band boundary is declined for arithmetic reasons that have nothing to do
// with whether it is a coordinate.
//
// It stays far too small to let a band letter mean a different band: the
// largest slop here is half a 100 km square, against bands eight degrees tall.
func withinBand(b byte, lat, slopMeters float64) bool {
	south, north, ok := bandLatitudes(b)
	if !ok {
		return false
	}

	// degreeMeters is the length of a degree of LONGITUDE at the equator and is
	// the larger figure; a degree of latitude is about 110,574 m, and a UTM
	// northing is scaled by k0 on top of that. Converting meters to degrees
	// with the larger number therefore produces slop that is slightly too
	// small, and the square's center is displaced in easting as well, which
	// away from the central meridian adds latitude offset of its own.
	//
	// Too little slop here is not a rounding curiosity: mgrsFor produced
	// "56D MF 3 0" for 72 south, and Parse then refused to read it back, so the
	// package rendered a reference it could not itself accept. A factor of two
	// covers both effects with room to spare and stays four orders of magnitude
	// inside a band, which is the only thing this must not reach.
	slop := 2 * slopMeters / 110574
	return lat >= south-slop && lat < north+slop
}

// zoneFor is the UTM zone a point falls in, including the two regions where the
// zones are not 6 degrees wide.
//
// Both exceptions are real and both are load-bearing. Norway's zone 32 was
// widened westward in 1950 so that the whole of the south-western mainland sits
// in one zone; Svalbard's zones were rearranged so no island is split. An
// implementation without them puts points in the neighboring zone, which is a
// difference of hundreds of kilometers in easting and produces a grid reference
// no other tool will agree with.
func zoneFor(lat, lon float64) (int, bool) {
	if math.IsNaN(lat) || math.IsNaN(lon) || lat < minUTMLat || lat >= maxUTMLat {
		return 0, false
	}
	if lon < -180 || lon >= 180 {
		return 0, false
	}

	zone := int(math.Floor((lon+180)/6)) + 1

	// South-west Norway.
	if lat >= 56 && lat < 64 && lon >= 3 && lon < 12 {
		zone = 32
	}

	// Svalbard.
	if lat >= 72 && lat < 84 {
		switch {
		case lon >= 0 && lon < 9:
			zone = 31
		case lon >= 9 && lon < 21:
			zone = 33
		case lon >= 21 && lon < 33:
			zone = 35
		case lon >= 33 && lon < 42:
			zone = 37
		}
	}

	if zone < 1 || zone > 60 {
		return 0, false
	}
	return zone, true
}

// centralMeridian is the longitude a zone is projected about.
func centralMeridian(zone int) float64 {
	return float64((zone-1)*6-180) + 3
}

// utmPoint is a position on the UTM grid.
type utmPoint struct {
	Zone     int
	Band     byte
	Easting  float64
	Northing float64
}

// projectUTM converts a geographic position to its UTM grid coordinates.
//
// The zone and band come from the position rather than being passed in, which
// is what makes this usable as a validator: a token naming a zone the position
// does not fall in is not a token this code would ever have produced.
func projectUTM(lat, lon float64) (utmPoint, bool) {
	zone, ok := zoneFor(lat, lon)
	if !ok {
		return utmPoint{}, false
	}
	band, ok := bandFor(lat)
	if !ok {
		return utmPoint{}, false
	}

	phi := lat * math.Pi / 180
	lambda := lon * math.Pi / 180
	lambda0 := centralMeridian(zone) * math.Pi / 180

	sinPhi, cosPhi := math.Sincos(phi)
	tanPhi := sinPhi / cosPhi

	// N is the radius of curvature in the prime vertical.
	n := wgs84A / math.Sqrt(1-wgs84E2*sinPhi*sinPhi)
	t := tanPhi * tanPhi
	c := wgs84EP2 * cosPhi * cosPhi
	a := (lambda - lambda0) * cosPhi

	m := meridianArc(phi)

	a2 := a * a
	a3 := a2 * a
	a4 := a3 * a
	a5 := a4 * a
	a6 := a5 * a

	easting := utmK0*n*(a+
		(1-t+c)*a3/6+
		(5-18*t+t*t+72*c-58*wgs84EP2)*a5/120) + utmFalseEasting

	northing := utmK0 * (m + n*tanPhi*(a2/2+
		(5-t+9*c+4*c*c)*a4/24+
		(61-58*t+t*t+600*c-330*wgs84EP2)*a6/720))

	if !bandNorthern(band) {
		northing += utmFalseNorthing
	}

	if math.IsNaN(easting) || math.IsNaN(northing) {
		return utmPoint{}, false
	}

	return utmPoint{Zone: zone, Band: band, Easting: easting, Northing: northing}, true
}

// unprojectUTM converts UTM grid coordinates back to a geographic position.
//
// The band is read only for its hemisphere. Whether the result actually falls
// in that band is the caller's check, not this one's, because a grid reference
// whose northing lands outside its own band is exactly the kind of crafted
// input the round trip in grid.go exists to reject.
func unprojectUTM(p utmPoint) (lat, lon float64, ok bool) {
	if p.Zone < 1 || p.Zone > 60 || bandIndex(p.Band) < 0 {
		return 0, 0, false
	}

	x := p.Easting - utmFalseEasting
	y := p.Northing
	if !bandNorthern(p.Band) {
		y -= utmFalseNorthing
	}

	m := y / utmK0

	// mu is the footpoint latitude's arc, and e1 the coefficient of the series
	// that inverts the meridian arc.
	mu := m / (wgs84A * (1 - wgs84E2/4 - 3*pow(wgs84E2, 2)/64 - 5*pow(wgs84E2, 3)/256))
	e1 := (1 - math.Sqrt(1-wgs84E2)) / (1 + math.Sqrt(1-wgs84E2))

	phi1 := mu +
		(3*e1/2-27*pow(e1, 3)/32)*math.Sin(2*mu) +
		(21*pow(e1, 2)/16-55*pow(e1, 4)/32)*math.Sin(4*mu) +
		(151*pow(e1, 3)/96)*math.Sin(6*mu) +
		(1097*pow(e1, 4)/512)*math.Sin(8*mu)

	// The series is only meaningful over the latitudes UTM covers, and it does
	// not fail loudly outside them: it fails by returning a number.
	//
	// This is not a theoretical guard. A 100 km row letter names a northing
	// every 2,000,000 meters, so decoding one means trying several candidates,
	// and a candidate past the pole runs the tan-squared terms up by nine
	// orders of magnitude. One such candidate came back as latitude 55.4 with a
	// longitude of 3883 degrees, which is plausible enough on the latitude
	// alone to be mistaken for the right answer by a caller checking only that.
	if math.IsNaN(phi1) || math.Abs(phi1) > (maxUTMLat+1)*math.Pi/180 {
		return 0, 0, false
	}

	sinPhi1, cosPhi1 := math.Sincos(phi1)
	if cosPhi1 == 0 {
		// The pole, which UTM does not cover and bandFor already refuses. Guard
		// anyway: the division below would otherwise produce an infinity that
		// propagates silently into a rendered coordinate.
		return 0, 0, false
	}
	tanPhi1 := sinPhi1 / cosPhi1

	c1 := wgs84EP2 * cosPhi1 * cosPhi1
	t1 := tanPhi1 * tanPhi1
	n1 := wgs84A / math.Sqrt(1-wgs84E2*sinPhi1*sinPhi1)
	r1 := wgs84A * (1 - wgs84E2) / math.Pow(1-wgs84E2*sinPhi1*sinPhi1, 1.5)
	d := x / (n1 * utmK0)

	d2 := d * d
	d3 := d2 * d
	d4 := d3 * d
	d5 := d4 * d
	d6 := d5 * d

	phi := phi1 - (n1*tanPhi1/r1)*(d2/2-
		(5+3*t1+10*c1-4*c1*c1-9*wgs84EP2)*d4/24+
		(61+90*t1+298*c1+45*t1*t1-252*wgs84EP2-3*c1*c1)*d6/720)

	lambda := centralMeridian(p.Zone)*math.Pi/180 +
		(d-(1+2*t1+c1)*d3/6+
			(5-2*c1+28*t1-3*c1*c1+8*wgs84EP2+24*t1*t1)*d5/120)/cosPhi1

	lat = phi * 180 / math.Pi
	lon = lambda * 180 / math.Pi

	if math.IsNaN(lat) || math.IsNaN(lon) || math.IsInf(lat, 0) || math.IsInf(lon, 0) {
		return 0, 0, false
	}

	// Longitude is cyclic, so a grid square clipped by the antimeridian has a
	// center a fraction of a degree past it and that is a real place, not an
	// error. Wrapped ONCE, deliberately: a single turn is all a clipped square
	// can need, while a blown-up series produces hundreds of degrees and a
	// single wrap leaves that still out of range and still refused. The
	// footpoint guard above is the actual defense; this is arithmetic.
	//
	// Refusing instead cost the easternmost strip of zone 60, where mgrsFor
	// wrote references that Parse would not read back, about one position in
	// 27,000.
	switch {
	case lon < -180:
		lon += 360
	case lon > 180:
		lon -= 360
	}

	if lat < -90 || lat > 90 || lon < -180 || lon > 180 {
		return 0, 0, false
	}

	return lat, lon, true
}

// meridianArc is the distance from the equator to a latitude along the
// meridian, Snyder equation 3-21.
//
// Checked against the published WGS 84 meridian quadrant, 10,001,965.729 m,
// which is the one number in this file with an authority outside it.
func meridianArc(phi float64) float64 {
	e2 := wgs84E2
	e4 := e2 * e2
	e6 := e4 * e2

	return wgs84A * ((1-e2/4-3*e4/64-5*e6/256)*phi -
		(3*e2/8+3*e4/32+45*e6/1024)*math.Sin(2*phi) +
		(15*e4/256+45*e6/1024)*math.Sin(4*phi) -
		(35*e6/3072)*math.Sin(6*phi))
}

func pow(v float64, n int) float64 {
	out := 1.0
	for range n {
		out *= v
	}
	return out
}
