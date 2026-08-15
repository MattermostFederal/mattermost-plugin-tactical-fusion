package mapdata

import (
	"math"
	"strconv"
	"strings"
)

const (
	pathScale  = 1000000
	adminScale = 1000000

	MercatorLimit = 85.0511287798066
)

type Box struct {
	MinX, MinY, MaxX, MaxY float64
}

func (b Box) Contains(x, y float64) bool {
	return x >= b.MinX && x <= b.MaxX && y >= b.MinY && y <= b.MaxY
}

type Ring struct {
	Box Box
	Lon []float64
	Lat []float64
}

type Country struct {
	Name  string
	Box   Box
	Polys [][]Ring
}

func decodeCountries(encoded string) []Country {
	if encoded == "" {
		return nil
	}
	lines := strings.Split(encoded, "\n")
	countries := make([]Country, 0, len(lines))
	for _, line := range lines {
		name, geom, ok := strings.Cut(line, "|")
		if !ok || name == "" {
			continue
		}
		c := Country{Name: name, Box: Box{MinX: math.Inf(1), MinY: math.Inf(1), MaxX: math.Inf(-1), MaxY: math.Inf(-1)}}
		for polyPart := range strings.SplitSeq(geom, ";") {
			var poly []Ring
			sound := true
			for ringPart := range strings.SplitSeq(polyPart, ":") {
				lons, lats, ok := decodePoints(ringPart, adminScale)
				if !ok || len(lons) < 3 {
					// The whole polygon goes, not just this ring. poly[0] is the
					// outer ring and poly[1:] are its holes, so dropping one in
					// place would promote a hole to the outer ring and invert
					// the polygon.
					sound = false
					break
				}
				ring := Ring{Box: boxOf(lons, lats), Lon: lons, Lat: lats}
				poly = append(poly, ring)
			}
			if sound && len(poly) > 0 {
				for _, ring := range poly {
					c.Box = union(c.Box, ring.Box)
				}
				c.Polys = append(c.Polys, poly)
			}
		}
		if len(c.Polys) > 0 {
			countries = append(countries, c)
		}
	}
	return countries
}

// A malformed pair fails the whole run rather than being skipped. Skipping one
// leaves the delta accumulators at the previous point, so every later point in
// the ring is displaced by the missing delta and boxOf agrees with the
// corruption. A missing shape is visible; a silently shifted one is not.
func decodePoints(encoded string, scale float64) ([]float64, []float64, bool) {
	if encoded == "" {
		return nil, nil, false
	}
	pairs := strings.Split(encoded, ",")
	xs := make([]float64, 0, len(pairs))
	ys := make([]float64, 0, len(pairs))
	var px, py int64
	for _, pair := range pairs {
		xRaw, yRaw, ok := strings.Cut(pair, " ")
		if !ok {
			return nil, nil, false
		}
		dx, err := strconv.ParseInt(xRaw, 36, 64)
		if err != nil {
			return nil, nil, false
		}
		dy, err := strconv.ParseInt(yRaw, 36, 64)
		if err != nil {
			return nil, nil, false
		}
		px += dx
		py += dy
		xs = append(xs, float64(px)/scale)
		ys = append(ys, float64(py)/scale)
	}
	return xs, ys, true
}

func boxOf(xs, ys []float64) Box {
	b := Box{MinX: math.Inf(1), MinY: math.Inf(1), MaxX: math.Inf(-1), MaxY: math.Inf(-1)}
	for i := range xs {
		b.MinX = math.Min(b.MinX, xs[i])
		b.MaxX = math.Max(b.MaxX, xs[i])
		b.MinY = math.Min(b.MinY, ys[i])
		b.MaxY = math.Max(b.MaxY, ys[i])
	}
	return b
}

func union(a, b Box) Box {
	return Box{
		MinX: math.Min(a.MinX, b.MinX),
		MinY: math.Min(a.MinY, b.MinY),
		MaxX: math.Max(a.MaxX, b.MaxX),
		MaxY: math.Max(a.MaxY, b.MaxY),
	}
}
