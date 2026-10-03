package airport

import (
	"encoding/csv"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

const demoMarker = "(DEMO-DATA)"

func supplementRows(t *testing.T) [][]string {
	t.Helper()

	path := filepath.Join("..", "..", "..", "build", "airportdata", "supplement", "airports.csv")
	raw, err := os.ReadFile(path) // #nosec G304 -- fixed, repo-relative source path
	if err != nil {
		t.Fatalf("open the supplement: %v", err)
	}

	rows, err := csv.NewReader(strings.NewReader(string(raw))).ReadAll()
	if err != nil {
		t.Fatalf("read the supplement: %v", err)
	}
	if len(rows) < 2 {
		t.Fatal("the supplement holds no airfields")
	}
	return rows[1:]
}

func TestTheDemoAirfieldsResolve(t *testing.T) {
	rows := supplementRows(t)
	if len(rows) != 11 {
		t.Fatalf("the supplement holds %d airfields, want the eleven demo airfields", len(rows))
	}

	decorator := &Decorator{}
	for _, row := range rows {
		ident := row[0]

		a, ok := Lookup(ident)
		if !ok {
			t.Errorf("%s is in the supplement and not in the embedded data", ident)
			continue
		}
		if strings.Join(row, "|") != strings.Join(embeddedRow(a), "|") {
			t.Errorf("%s is embedded as %q, the supplement says %q", ident, embeddedRow(a), row)
		}

		params, ok := decorator.Parse(ident, time.Now().UTC())
		if !ok || params.Get(ParamValue) != ident {
			t.Errorf("%s does not parse to its own link: %v", ident, params)
		}

		if message := "ICAO:" + ident; !decorated(t, message) {
			t.Errorf("%q was not decorated", message)
		}
	}
}

func embeddedRow(a Airport) []string {
	elevation := ""
	if a.ElevationFt != nil {
		elevation = strconv.Itoa(*a.ElevationFt)
	}
	return []string{
		a.Ident, a.Type, a.Name, a.Municipality, a.Country, a.Region, a.IATA, elevation,
		strconv.FormatFloat(a.Lat, 'f', 4, 64), strconv.FormatFloat(a.Lon, 'f', 4, 64), a.Military,
	}
}

func TestEveryDemoAirfieldSaysItIsDemoData(t *testing.T) {
	for _, row := range supplementRows(t) {
		d, ok := Describe(row[0])
		if !ok {
			t.Fatalf("%s is not describable", row[0])
		}
		if !strings.HasSuffix(d.Name, demoMarker) {
			t.Errorf("%s is named %q, which does not end in %s", row[0], d.Name, demoMarker)
		}
	}
}

func TestOnlyTheSupplementCarriesTheDemoMarker(t *testing.T) {
	supplemented := map[string]bool{}
	for _, row := range supplementRows(t) {
		supplemented[row[0]] = true
	}

	for ident, a := range airfields {
		if strings.Contains(a.Name, demoMarker) && !supplemented[ident] {
			t.Errorf("%s is named %q and is not in the supplement", ident, a.Name)
		}
	}
}

func TestADemoAirfieldCarriesNoRunwaysFrequenciesOrIATACode(t *testing.T) {
	for _, row := range supplementRows(t) {
		d, ok := Describe(row[0])
		if !ok {
			t.Fatalf("%s is not describable", row[0])
		}
		if len(d.Runways) != 0 || len(d.Frequencies) != 0 || d.IATA != "" {
			t.Errorf("%s carries %d runways, %d frequencies and IATA %q, want none",
				row[0], len(d.Runways), len(d.Frequencies), d.IATA)
		}
		if !d.HasPosition {
			t.Errorf("%s has no position", row[0])
		}
		if _, ok := MapBlob(row[0]); !ok {
			t.Errorf("%s has no map blob", row[0])
		}
	}
}

const handoffToleranceMeters = 50

type handoffPosition struct{ lat, lon float64 }

var scenarioFrameHandoffPositions = map[string]handoffPosition{
	"PCMN": {21.3206, -157.9242},
	"PFRC": {13.5840, 144.9300},
	"PGPC": {21.3353, -157.9483},
	"PLWF": {19.2820, 166.6360},
	"PNTF": {13.584, 144.929998},
	"PORF": {7.3673, 134.5443},
	"PTWF": {21.3187, -157.9224},
	"RCRB": {35.7485, 139.3480},
	"RSPS": {35.4546, 139.4500},
	"RVGF": {15.1859, 120.5603},
	"RWBF": {26.351667, 127.769444},
}

func metersApart(aLat, aLon, bLat, bLon float64) float64 {
	const earthRadiusMeters = 6371008.8

	rad := math.Pi / 180
	dLat := (bLat - aLat) * rad
	dLon := (bLon - aLon) * rad
	meanLat := (aLat + bLat) / 2 * rad

	x := dLon * math.Cos(meanLat)
	return math.Hypot(dLat, x) * earthRadiusMeters
}

func TestMetersApartMeasuresInMeters(t *testing.T) {
	const oneDegreeOfLatitude = 111195.0

	for _, c := range []struct {
		name                   string
		aLat, aLon, bLat, bLon float64
		want                   float64
	}{
		{"a point is no distance from itself", 19.2820, 166.6360, 19.2820, 166.6360, 0},
		{"one degree of latitude", 0, 0, 1, 0, oneDegreeOfLatitude},
		{"one degree of longitude at the equator", 0, 0, 0, 1, oneDegreeOfLatitude},
		{"one degree of longitude at sixty north is half the equator's", 60, 0, 60, 1, oneDegreeOfLatitude / 2},
		{"Lonewatch to the real Wake Island Airfield", 19.2820, 166.6360, 19.2824, 166.6366, 77.1},
	} {
		t.Run(c.name, func(t *testing.T) {
			if got := metersApart(c.aLat, c.aLon, c.bLat, c.bLon); math.Abs(got-c.want) > 1 {
				t.Errorf("metersApart = %.1f m, want %.1f m", got, c.want)
			}
		})
	}
}

func TestEveryDemoAirfieldSitsWhereTheV3HandoffPutsIt(t *testing.T) {
	for _, row := range supplementRows(t) {
		ident := row[0]

		want, ok := scenarioFrameHandoffPositions[ident]
		if !ok {
			t.Errorf("%s is in the supplement and the v3 handoff states no position for it", ident)
			continue
		}

		a, ok := Lookup(ident)
		if !ok {
			t.Errorf("%s is in the supplement and not in the embedded data", ident)
			continue
		}

		if off := metersApart(a.Lat, a.Lon, want.lat, want.lon); off > handoffToleranceMeters {
			t.Errorf("%s is embedded at %.4f,%.4f and the v3 handoff puts it at %.4f,%.4f, %.0f m away",
				ident, a.Lat, a.Lon, want.lat, want.lon, off)
		}
	}
}

func TestEveryPositionTheHandoffStatesIsInTheSupplement(t *testing.T) {
	supplemented := map[string]bool{}
	for _, row := range supplementRows(t) {
		supplemented[row[0]] = true
	}

	for ident := range scenarioFrameHandoffPositions {
		if !supplemented[ident] {
			t.Errorf("the v3 handoff states a position for %s, which is not in the supplement", ident)
		}
	}
}

func TestTradewindIsNotGranitePoint(t *testing.T) {
	tradewind, ok := Lookup("PTWF")
	if !ok {
		t.Fatal("PTWF is not in the embedded data")
	}
	granite, ok := Lookup("PGPC")
	if !ok {
		t.Fatal("PGPC is not in the embedded data")
	}

	if tradewind.Lat == granite.Lat && tradewind.Lon == granite.Lon {
		t.Errorf("PTWF and PGPC share the position %.4f,%.4f, which is the copy-paste slip "+
			"the v3 handoff corrects", tradewind.Lat, tradewind.Lon)
	}
}
