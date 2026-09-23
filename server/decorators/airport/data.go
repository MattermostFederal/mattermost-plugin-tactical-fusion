package airport

import (
	_ "embed"
	"encoding/csv"
	"fmt"
	"math"
	"strconv"
	"strings"
	"unicode"
)

//go:embed data/airports.csv
var airportsCSV string

//go:embed data/runways.csv
var runwaysCSV string

//go:embed data/frequencies.csv
var frequenciesCSV string

const (
	MaxRunwaysPerAirfield     = 32
	MaxFrequenciesPerAirfield = 48
)

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
	Military     string
}

type runwayRecord struct {
	LowIdent  string
	HighIdent string
	LengthFt  *int
	WidthFt   *int
	Surface   string
	Lighted   bool
	Closed    bool
	HasEnds   bool
	LowLat    float64
	LowLon    float64
	HighLat   float64
	HighLon   float64
	Heading   *int
}

type frequencyRecord struct {
	Type        string
	Description string
	MHz         string
}

var airfields = mustParseAirfields(airportsCSV)

var iataIndex = mustBuildIATAIndex(airfields)

var runways = mustParseRunways(runwaysCSV, airfields)

var frequencies = mustParseFrequencies(frequenciesCSV, airfields)

func Lookup(ident string) (Airport, bool) {
	a, ok := airfields[ident]
	return a, ok
}

func LookupIATA(code string) (Airport, bool) {
	ident, ok := iataIndex[code]
	if !ok {
		return Airport{}, false
	}
	return Lookup(ident)
}

func Count() int { return len(airfields) }

func Idents() []string {
	out := make([]string, 0, len(airfields))
	for ident := range airfields {
		out = append(out, ident)
	}
	return out
}

func IATACount() int { return len(iataIndex) }

func RunwayCount() int {
	n := 0
	for _, records := range runways {
		n += len(records)
	}
	return n
}

func FrequencyCount() int {
	n := 0
	for _, records := range frequencies {
		n += len(records)
	}
	return n
}

func MilitaryCount() int {
	n := 0
	for _, a := range airfields {
		if a.Military != "" {
			n++
		}
	}
	return n
}

func runwaysOf(ident string) []runwayRecord { return runways[ident] }

func frequenciesOf(ident string) []frequencyRecord { return frequencies[ident] }

func mustParseAirfields(source string) map[string]Airport {
	parsed, err := parseAirfields(source)
	if err != nil {
		panic("airport: " + err.Error())
	}
	return parsed
}

func mustBuildIATAIndex(fields map[string]Airport) map[string]string {
	index, err := buildIATAIndex(fields)
	if err != nil {
		panic("airport: " + err.Error())
	}
	return index
}

func mustParseRunways(source string, fields map[string]Airport) map[string][]runwayRecord {
	parsed, err := parseRunways(source, fields)
	if err != nil {
		panic("airport: " + err.Error())
	}
	return parsed
}

func mustParseFrequencies(source string, fields map[string]Airport) map[string][]frequencyRecord {
	parsed, err := parseFrequencies(source, fields)
	if err != nil {
		panic("airport: " + err.Error())
	}
	return parsed
}

const allowedPunctuation = " _-,.'\"()[]/&`+" +
	"\u00a0" +
	"\u00ad" +
	"\u00b4" +
	"\u2013" +
	"\u2019" +
	"\u201c" +
	"\u201e"

var autolinkTriggers = []string{"www.", "://"}

func validText(field string) bool {
	lowered := strings.ToLower(field)
	for _, trigger := range autolinkTriggers {
		if strings.Contains(lowered, trigger) {
			return false
		}
	}

	for _, r := range field {
		if unicode.IsLetter(r) || unicode.IsMark(r) || unicode.IsNumber(r) {
			continue
		}
		if !strings.ContainsRune(allowedPunctuation, r) {
			return false
		}
	}

	return true
}

const airfieldColumns = 11

func parseAirfields(source string) (map[string]Airport, error) {
	rows, err := readRecords(source, "airfield", airfieldColumns)
	if err != nil {
		return nil, err
	}

	out := make(map[string]Airport, len(rows))
	for _, row := range rows {
		for _, field := range append(row[1:8:8], row[10]) {
			if !validText(field) {
				return nil, fmt.Errorf("%s: a field carries a character the whitelist refuses: %q", row[0], field)
			}
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
			Military:     row[10],
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

func buildIATAIndex(fields map[string]Airport) (map[string]string, error) {
	index := make(map[string]string, len(fields))
	for ident, a := range fields {
		if a.IATA == "" {
			continue
		}
		if !MatchesIATAShape(a.IATA) {
			return nil, fmt.Errorf("%s: IATA code %q is not three upper-case letters", ident, a.IATA)
		}
		if other, dup := index[a.IATA]; dup {
			return nil, fmt.Errorf("IATA code %q names both %s and %s", a.IATA, other, ident)
		}
		index[a.IATA] = ident
	}
	return index, nil
}

const runwayColumns = 13

func parseRunways(source string, fields map[string]Airport) (map[string][]runwayRecord, error) {
	rows, err := readRecords(source, "runway", runwayColumns)
	if err != nil {
		return nil, err
	}

	out := make(map[string][]runwayRecord)
	for _, row := range rows {
		ident := row[0]
		if _, known := fields[ident]; !known {
			return nil, fmt.Errorf("runway %s: ident %q is not an airfield this build holds", row[1], ident)
		}
		for _, field := range row[1:6] {
			if !validText(field) {
				return nil, fmt.Errorf("%s runway %s: a field carries a character the whitelist refuses: %q", ident, row[1], field)
			}
		}
		if row[1] == "" {
			return nil, fmt.Errorf("%s: a runway has no designation", ident)
		}

		r := runwayRecord{
			LowIdent:  row[1],
			HighIdent: row[2],
			Surface:   row[5],
			Lighted:   row[6] == "1",
			Closed:    row[7] == "1",
		}

		if r.LengthFt, err = optionalInt(row[3]); err != nil {
			return nil, fmt.Errorf("%s runway %s: length: %w", ident, row[1], err)
		}
		if r.WidthFt, err = optionalInt(row[4]); err != nil {
			return nil, fmt.Errorf("%s runway %s: width: %w", ident, row[1], err)
		}
		if r.Heading, err = optionalInt(row[12]); err != nil {
			return nil, fmt.Errorf("%s runway %s: heading: %w", ident, row[1], err)
		}

		ends := row[8:12]
		blank := 0
		for _, end := range ends {
			if end == "" {
				blank++
			}
		}
		switch blank {
		case 4:
		case 0:
			var values [4]float64
			for i, end := range ends {
				if values[i], err = strconv.ParseFloat(end, 64); err != nil {
					return nil, fmt.Errorf("%s runway %s: end: %w", ident, row[1], err)
				}
			}
			r.HasEnds = true
			r.LowLat, r.LowLon, r.HighLat, r.HighLon = values[0], values[1], values[2], values[3]
		default:
			return nil, fmt.Errorf("%s runway %s: an end is half stated", ident, row[1])
		}

		out[ident] = append(out[ident], r)
		if len(out[ident]) > MaxRunwaysPerAirfield {
			return nil, fmt.Errorf("%s has more than %d runways", ident, MaxRunwaysPerAirfield)
		}
	}

	return out, nil
}

const frequencyColumns = 4

func parseFrequencies(source string, fields map[string]Airport) (map[string][]frequencyRecord, error) {
	rows, err := readRecords(source, "frequency", frequencyColumns)
	if err != nil {
		return nil, err
	}

	out := make(map[string][]frequencyRecord)
	for _, row := range rows {
		ident := row[0]
		if _, known := fields[ident]; !known {
			return nil, fmt.Errorf("frequency %s: ident %q is not an airfield this build holds", row[3], ident)
		}
		for _, field := range row[1:3] {
			if !validText(field) {
				return nil, fmt.Errorf("%s frequency %s: a field carries a character the whitelist refuses: %q", ident, row[3], field)
			}
		}
		if row[1] == "" {
			return nil, fmt.Errorf("%s: a frequency has no type", ident)
		}
		mhz, err := strconv.ParseFloat(row[3], 64)
		if err != nil || math.IsNaN(mhz) || math.IsInf(mhz, 0) || mhz < 0 {
			return nil, fmt.Errorf("%s frequency: %q is not a frequency", ident, row[3])
		}

		out[ident] = append(out[ident], frequencyRecord{Type: row[1], Description: row[2], MHz: row[3]})
		if len(out[ident]) > MaxFrequenciesPerAirfield {
			return nil, fmt.Errorf("%s has more than %d frequencies", ident, MaxFrequenciesPerAirfield)
		}
	}

	return out, nil
}

func readRecords(source, what string, width int) ([][]string, error) {
	reader := csv.NewReader(strings.NewReader(source))
	rows, err := reader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("reading the %s data: %w", what, err)
	}
	if len(rows) < 2 {
		return nil, fmt.Errorf("the %s data is empty", what)
	}

	for _, row := range rows[1:] {
		if len(row) != width {
			return nil, fmt.Errorf("a %s record has %d fields, want %d", what, len(row), width)
		}
	}

	return rows[1:], nil
}

func optionalInt(raw string) (*int, error) {
	if raw == "" {
		return nil, nil
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return nil, err
	}
	return &value, nil
}
