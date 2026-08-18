package location

import (
	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/decorators/location/mapdata"
)

const regionSource = "Natural Earth 110m"

func (l Location) RegionText() string {
	lat, lon, ok := l.Point()
	if !ok {
		return ""
	}
	name := regionAt(lat, lon)
	if name == "" {
		return ""
	}
	return name + " (" + regionSource + ")"
}

func regionAt(lat, lon float64) string {
	for _, c := range mapdata.Countries {
		if !c.Box.Contains(lon, lat) {
			continue
		}
		if countryContains(c, lon, lat) {
			return c.Name
		}
	}
	return ""
}

func countryContains(c mapdata.Country, lon, lat float64) bool {
	for _, poly := range c.Polys {
		if len(poly) == 0 {
			continue
		}
		if !poly[0].Box.Contains(lon, lat) || !ringContains(poly[0], lon, lat) {
			continue
		}
		inHole := false
		for _, hole := range poly[1:] {
			if hole.Box.Contains(lon, lat) && ringContains(hole, lon, lat) {
				inHole = true
				break
			}
		}
		if !inHole {
			return true
		}
	}
	return false
}

func ringContains(r mapdata.Ring, lon, lat float64) bool {
	inside := false

	// Bounded by the SHORTER axis. Ring is exported with exported slice fields
	// and no constructor, so the equal-length invariant holds only because
	// decodePoints is the sole producer and appends in lockstep. Indexing Lat by
	// len(Lon) turns any future violation into an index-out-of-range inside
	// ServeHTTP, and nothing on the HTTP path recovers: the only recover in this
	// plugin is in decoratePost, so the panic takes the plugin process down for
	// every reader on the node rather than failing one request.
	n := min(len(r.Lon), len(r.Lat))
	for i, j := 0, n-1; i < n; j, i = i, i+1 {
		yi, yj := r.Lat[i], r.Lat[j]
		if (yi > lat) == (yj > lat) {
			continue
		}
		xi, xj := r.Lon[i], r.Lon[j]
		if lon < (xj-xi)*(lat-yi)/(yj-yi)+xi {
			inside = !inside
		}
	}
	return inside
}
