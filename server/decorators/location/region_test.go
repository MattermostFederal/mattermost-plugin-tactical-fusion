package location

import (
	"strings"
	"testing"

	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/decorators/location/mapdata"
)

func TestRegionNamesWellKnownPositions(t *testing.T) {
	cases := []struct {
		name string
		lat  float64
		lon  float64
		want string
	}{
		{"Los Angeles", 34.0561, -118.25, "United States of America"},
		{"London", 51.5074, -0.1278, "United Kingdom"},
		{"Kabul", 34.5553, 69.2075, "Afghanistan"},
		{"Canberra", -35.2809, 149.13, "Australia"},
		{"Brasilia", -15.7939, -47.8828, "Brazil"},
		{"Null Island is ocean", 0, 0, ""},
		{"mid Pacific", 0, -140, ""},
		{"mid Atlantic", 30, -40, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := regionAt(c.lat, c.lon); got != c.want {
				t.Fatalf("regionAt(%v, %v) = %q, want %q", c.lat, c.lon, got, c.want)
			}
		})
	}
}

func TestRegionIsOmittedRatherThanGuessed(t *testing.T) {
	if got := regionAt(0, 0); got != "" {
		t.Fatalf("regionAt over open ocean = %q, want an empty string rather than a nearest guess", got)
	}
}

func TestRegionHandlesTheAntimeridian(t *testing.T) {
	cases := []struct {
		name string
		lat  float64
		lon  float64
		want string
	}{
		{"Chukotka, east of 180", 66.0, 179.5, "Russia"},
		{"Chukotka, west of -180 wrap", 66.0, -179.5, "Russia"},
		{"Fiji, Vanua Levu, east of 180", -16.5, 179.5, "Fiji"},
		{"Fiji, Viti Levu", -17.8, 178.0, "Fiji"},
		{"Fiji, Taveuni group, west of -180", -16.2, -179.9, "Fiji"},
		{"open water between Fiji's islands", -17.0, 179.0, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := regionAt(c.lat, c.lon); got != c.want {
				t.Fatalf("regionAt(%v, %v) = %q, want %q", c.lat, c.lon, got, c.want)
			}
		})
	}
}

func TestRegionTiebreakIsStableAcrossRegeneration(t *testing.T) {
	var last string
	for i, c := range mapdata.Countries {
		if i > 0 && c.Name < last {
			t.Fatalf("Countries is not sorted at index %d: %q follows %q. "+
				"regionAt returns the first match, so an unstable order makes a shared "+
				"border resolve differently every time the source is regenerated", i, c.Name, last)
		}
		last = c.Name
	}
}

func TestRegionTextCitesItsSource(t *testing.T) {
	loc, ok := Parse(FormatDDH, "34.0561N,118.2500W")
	if !ok {
		t.Fatal("fixture did not parse")
	}
	got := loc.RegionText()
	if !strings.Contains(got, regionSource) {
		t.Fatalf("RegionText() = %q, want it to cite %q so it reads as a basemap lookup "+
			"rather than a determination", got, regionSource)
	}
	if !strings.HasPrefix(got, "United States of America") {
		t.Fatalf("RegionText() = %q, want it to name the administering entity first", got)
	}
}

func TestRegionTextIsEmptyWhereThereIsNoAnswer(t *testing.T) {
	loc, ok := Parse(FormatDDH, "30.0000N,40.0000W")
	if !ok {
		t.Fatal("fixture did not parse")
	}
	if got := loc.RegionText(); got != "" {
		t.Fatalf("RegionText() over open ocean = %q, want empty so the row is omitted", got)
	}
}

// The hole in South Africa's polygon, which is Lesotho.
//
// This cannot be tested through regionAt: Countries is sorted by name and the
// first match wins, so "Lesotho" is returned whether or not the interior ring is
// honored. It is also the ONLY multi-ring polygon in the 110m dataset, so the
// hole branch has a sample size of one and no coverage without this.
func TestCountryPolygonHolesAreSubtracted(t *testing.T) {
	var za mapdata.Country
	for _, c := range mapdata.Countries {
		if c.Name == "South Africa" {
			za = c
			break
		}
	}
	if za.Name == "" {
		t.Fatal("South Africa is not in the dataset")
	}

	if countryContains(za, 28.2, -29.6) {
		t.Fatal("South Africa's polygon claims Maseru; the interior ring is not " +
			"being subtracted, so an enclave would be named as its neighbor")
	}
	if !countryContains(za, 24.0, -29.0) {
		t.Fatal("South Africa's polygon excludes a point that is plainly inside it")
	}
}

func TestRegionSurvivesDegenerateGeometry(t *testing.T) {
	// Whatever the generator emits, a request must not panic.
	for _, c := range []mapdata.Country{
		{},
		{Name: "empty polys", Polys: [][]mapdata.Ring{}},
		{Name: "empty poly", Polys: [][]mapdata.Ring{{}}},
		{Name: "one point", Polys: [][]mapdata.Ring{{{Lon: []float64{1}, Lat: []float64{1}}}}},

		// Ring is exported with exported slice fields and no constructor, so
		// nothing in the type stops the two axes disagreeing. Indexing Lat by
		// len(Lon) panicked here, and nothing on the HTTP path recovers, so it
		// would have taken the plugin down for every reader on the node rather
		// than failing one request. decodeCountries refuses such a ring now;
		// this pins the read path against a hand-built one either way.
		{Name: "more lons than lats", Polys: [][]mapdata.Ring{{{
			Lon: []float64{0, 1, 2, 3}, Lat: []float64{0, 1},
		}}}},
		{Name: "more lats than lons", Polys: [][]mapdata.Ring{{{
			Lon: []float64{0, 1}, Lat: []float64{0, 1, 2, 3},
		}}}},
	} {
		if countryContains(c, 0, 0) {
			t.Fatalf("%q claimed a position it has no geometry for", c.Name)
		}
	}
}
