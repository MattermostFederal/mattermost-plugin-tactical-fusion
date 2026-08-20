package airport

import (
	_ "embed"
	"encoding/csv"
	"fmt"
	"strconv"
	"strings"
)

//go:embed data/airports.csv
var airportsCSV string

// Airport is one airfield, exactly the fields data/airports.csv carries.
//
// ElevationFt is a pointer because sea level is zero and is a real elevation,
// while 1,444 rows in this subset state none at all. Nothing may test it for
// truthiness; that is the defect this plugin deliberately does not inherit from
// mattermost-plugin-aocanywhere.
type Airport struct {
	Ident        string
	Type         string
	Name         string
	Municipality string
	Country      string
	Region       string
	IATA         string
	ElevationFt  *int
	Lat          float64
	Lon          float64
}

var airfields = mustParseAirfields(airportsCSV)

// Lookup returns the airfield with this ident.
//
// The ident is not folded, because both callers validate it against
// identShape first and the grammar admits upper case only. That is what keeps
// one spelling of every link.
func Lookup(ident string) (Airport, bool) {
	a, ok := airfields[ident]
	return a, ok
}

// Count is how many airfields this build holds.
func Count() int { return len(airfields) }

func mustParseAirfields(source string) map[string]Airport {
	parsed, err := parseAirfields(source)
	if err != nil {
		panic("airport: " + err.Error())
	}
	return parsed
}

func parseAirfields(source string) (map[string]Airport, error) {
	reader := csv.NewReader(strings.NewReader(source))
	rows, err := reader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("reading the airfield data: %w", err)
	}
	if len(rows) < 2 {
		return nil, fmt.Errorf("the airfield data is empty")
	}

	out := make(map[string]Airport, len(rows)-1)
	for _, row := range rows[1:] {
		if len(row) != 10 {
			return nil, fmt.Errorf("a record has %d fields, want 10", len(row))
		}

		lat, err := strconv.ParseFloat(row[8], 64)
		if err != nil {
			return nil, fmt.Errorf("%s: latitude: %w", row[0], err)
		}
		lon, err := strconv.ParseFloat(row[9], 64)
		if err != nil {
			return nil, fmt.Errorf("%s: longitude: %w", row[0], err)
		}

		a := Airport{
			Ident:        row[0],
			Type:         row[1],
			Name:         row[2],
			Municipality: row[3],
			Country:      row[4],
			Region:       row[5],
			IATA:         row[6],
			Lat:          lat,
			Lon:          lon,
		}

		if row[7] != "" {
			feet, err := strconv.Atoi(row[7])
			if err != nil {
				return nil, fmt.Errorf("%s: elevation: %w", a.Ident, err)
			}
			a.ElevationFt = &feet
		}

		if _, dup := out[a.Ident]; dup {
			return nil, fmt.Errorf("duplicate ident %q", a.Ident)
		}
		out[a.Ident] = a
	}

	return out, nil
}
