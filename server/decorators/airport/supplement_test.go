package airport

import (
	"encoding/csv"
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
	if len(rows) != 10 {
		t.Fatalf("the supplement holds %d airfields, want the ten demo airfields", len(rows))
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
