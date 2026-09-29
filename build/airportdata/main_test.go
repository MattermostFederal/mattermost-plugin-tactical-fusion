package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const upstreamHeader = "ident,type,name,municipality,iso_country,iso_region,iata_code,elevation_ft,latitude_deg,longitude_deg\n"

const upstreamRows = "PHIK,medium_airport,Hickam Air Force Base,Honolulu,US,US-HI,,13,21.335278,-157.948333\n" +
	"RJTY,large_airport,Yokota Air Base,Fussa,JP,JP-13,OKO,463,35.748501,139.348007\n" +
	"00AA,small_airport,Not A Four Letter Ident,Leoti,US,US-KS,,3435,38.704022,-101.473911\n"

const supplementHeader = "ident,type,name,municipality,iso_country,iso_region,iata_code,elevation_ft,lat,lon,military\n"

func writeSource(t *testing.T, name, body string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func filterWith(t *testing.T, supplementRows string) (table, map[string]bool, counts, error) {
	t.Helper()

	var c counts
	out, kept, err := filterAirfields(
		writeSource(t, "airports.csv", upstreamHeader+upstreamRows),
		writeSource(t, "supplement.csv", supplementHeader+supplementRows),
		&c,
	)
	return out, kept, c, err
}

func TestSupplementRowsAreMergedInIdentOrder(t *testing.T) {
	out, kept, c, err := filterWith(t,
		"RWBF,large_airport,West Bay Field (DEMO-DATA),West Bay,JP,JP-47,,143,26.3517,127.7694,\n"+
			"PCMN,medium_airport,Camp Meridian C2 Node (DEMO-DATA),Camp Meridian,US,US-HI,,13,21.3206,-157.9242,\n"+
			"RCRB,large_airport,Cobalt Reach Air Base (DEMO-DATA),Cobalt Reach,JP,JP-13,,463,35.7485,139.3480,Air Base\n")
	if err != nil {
		t.Fatal(err)
	}

	var idents []string
	for _, row := range out.rows {
		idents = append(idents, row[0])
	}
	if got, want := strings.Join(idents, " "), "PCMN PHIK RCRB RJTY RWBF"; got != want {
		t.Fatalf("idents = %s, want %s", got, want)
	}

	for _, ident := range idents {
		if !kept[ident] {
			t.Errorf("%s is written and not in the kept set", ident)
		}
	}
	if c.airfields != 5 || c.supplemented != 3 || c.military != 3 {
		t.Errorf("counted %d airfields, %d supplemented, %d military; want 5, 3, 3", c.airfields, c.supplemented, c.military)
	}

	want := record{"RCRB", "large_airport", "Cobalt Reach Air Base (DEMO-DATA)", "Cobalt Reach", "JP", "JP-13", "", "463", "35.7485", "139.3480", "Air Base"}
	if got := out.rows[2]; strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("row = %q, want %q", got, want)
	}
}

func TestSupplementRowsAreRoundedLikeUpstreamRows(t *testing.T) {
	out, _, _, err := filterWith(t,
		"PORF,medium_airport,Outer Reef Field (DEMO-DATA),Outer Reef,PW,PW-004,,176.4,7.36731,134.544236,\n")
	if err != nil {
		t.Fatal(err)
	}

	row := out.rows[1]
	if row[7] != "176" || row[8] != "7.3673" || row[9] != "134.5442" {
		t.Fatalf("elevation, lat, lon = %s, %s, %s", row[7], row[8], row[9])
	}
}

func TestAnEmptySupplementChangesNothing(t *testing.T) {
	out, _, c, err := filterWith(t, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(out.rows) != 2 || c.supplemented != 0 {
		t.Fatalf("%d rows and %d supplemented, want 2 and 0", len(out.rows), c.supplemented)
	}
}

func TestSupplementIsRefused(t *testing.T) {
	tests := []struct {
		name string
		rows string
		want string
	}{
		{
			"when an ident collides with an upstream airfield",
			"PHIK,medium_airport,Tradewind Field (DEMO-DATA),Tradewind,US,US-HI,,13,21.3353,-157.9483,\n",
			`"PHIK" collides with an upstream airfield`,
		},
		{
			"when an ident appears twice",
			"PTWF,medium_airport,Tradewind Field (DEMO-DATA),Tradewind,US,US-HI,,13,21.3353,-157.9483,\n" +
				"PTWF,medium_airport,Tradewind Field (DEMO-DATA),Tradewind,US,US-HI,,13,21.3353,-157.9483,\n",
			`duplicate supplement ident "PTWF"`,
		},
		{
			"when an ident is not four upper-case letters",
			"ptwf,medium_airport,Tradewind Field (DEMO-DATA),Tradewind,US,US-HI,,13,21.3353,-157.9483,\n",
			"is not four upper-case letters",
		},
		{
			"when an ident is reserved",
			"ZZZZ,medium_airport,Tradewind Field (DEMO-DATA),Tradewind,US,US-HI,,13,21.3353,-157.9483,\n",
			"is reserved",
		},
		{
			"when an IATA code is already carried upstream",
			"PTWF,medium_airport,Tradewind Field (DEMO-DATA),Tradewind,US,US-HI,OKO,13,21.3353,-157.9483,\n",
			`IATA code "OKO" names both RJTY and PTWF`,
		},
		{
			"when an axis is outside its range",
			"PTWF,medium_airport,Tradewind Field (DEMO-DATA),Tradewind,US,US-HI,,13,91.0000,-157.9483,\n",
			"is outside 90",
		},
		{
			"when the coordinates are the null pair",
			"PTWF,medium_airport,Tradewind Field (DEMO-DATA),Tradewind,US,US-HI,,13,0,0,\n",
			"coordinates are the null pair",
		},
		{
			"when a field carries a character the whitelist refuses",
			"PTWF,medium_airport,Tradewind <Field>,Tradewind,US,US-HI,,13,21.3353,-157.9483,\n",
			"the name field carries a character the whitelist refuses",
		},
		{
			"when the military column disagrees with the name",
			"RCRB,large_airport,Cobalt Reach Air Base (DEMO-DATA),Cobalt Reach,JP,JP-13,,463,35.7485,139.3480,\n",
			`military is "", the name yields "Air Base"`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, _, _, err := filterWith(t, test.rows)
			if err == nil {
				t.Fatal("the supplement was accepted")
			}
			if !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error = %q, want it to contain %q", err, test.want)
			}
		})
	}
}

func TestSupplementMissingAColumnIsRefused(t *testing.T) {
	var c counts
	_, _, err := filterAirfields(
		writeSource(t, "airports.csv", upstreamHeader+upstreamRows),
		writeSource(t, "supplement.csv", "ident,type,name\nPTWF,medium_airport,Tradewind Field (DEMO-DATA)\n"),
		&c,
	)
	if err == nil || !strings.Contains(err.Error(), "is missing the") {
		t.Fatalf("error = %v, want a missing column", err)
	}
}

func TestTheCommittedSupplementPassesItsOwnValidation(t *testing.T) {
	var c counts
	out, _, err := filterAirfields(
		writeSource(t, "airports.csv", upstreamHeader+upstreamRows),
		filepath.Join("supplement", "airports.csv"),
		&c,
	)
	if err != nil {
		t.Fatal(err)
	}
	if c.supplemented == 0 || len(out.rows) != 2+c.supplemented {
		t.Fatalf("%d rows with %d supplemented", len(out.rows), c.supplemented)
	}

	source, err := os.ReadFile(filepath.Join("supplement", "airports.csv"))
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range strings.Split(strings.TrimSpace(string(source)), "\n")[1:] {
		if !strings.Contains(line, "(DEMO-DATA)") {
			t.Errorf("a supplement row does not say it is demo data: %s", line)
		}
	}
}
