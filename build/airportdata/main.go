package main

import (
	"encoding/csv"
	"fmt"
	"io"
	"math"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

const (
	defaultSource = "build/airportdata/source/airport-codes.csv"
	destination   = "server/decorators/airport/data/airports.csv"

	coordinateDigits = 4
)

var identShape = regexp.MustCompile(`^[A-Z]{4}$`)

// reserved are idents ICAO gives a meaning other than "this airfield".
//
// ZZZZ is the one that matters and it is a real row upstream, Satsuma Iojima in
// Japan. In a flight plan, and therefore in the USMTF DEPLOC and ARRLOC fields
// this decorator reads, ZZZZ is the code for an aerodrome that is NOT listed,
// with the actual field named in remarks. Shipping the upstream row would make
// "DEPLOC:ZZZZ" resolve to a specific island airfield 30 degrees north of
// wherever the mission actually departs from.
//
// That is the failure this plugin refuses everywhere else: not a false positive
// on text that was never a code, but a real code rendered as a confidently
// wrong place. Same argument as the UTM band letter, and the same answer.
//
// AFIL, the other reserved code, is not in the upstream data at all.
var reserved = map[string]string{
	"ZZZZ": "ICAO reserves it for an aerodrome that is not listed",
}

var columns = []string{
	"ident", "type", "name", "municipality",
	"iso_country", "iso_region", "iata_code", "elevation_ft",
	"lat", "lon",
}

type record struct {
	fields []string
}

func main() {
	source := defaultSource
	if len(os.Args) > 1 {
		source = os.Args[1]
	}

	rows, err := filter(source)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	if err := write(destination, rows); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	fmt.Printf("wrote %d airfields to %s\n", len(rows), destination)
}

func filter(source string) ([]record, error) {
	f, err := os.Open(source) //nolint:gosec // G304: a path this program is told to read
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", source, err)
	}
	defer f.Close()

	reader := csv.NewReader(f)
	header, err := reader.Read()
	if err != nil {
		return nil, fmt.Errorf("read header: %w", err)
	}

	index := map[string]int{}
	for i, name := range header {
		index[name] = i
	}
	for _, want := range []string{"ident", "coordinates"} {
		if _, ok := index[want]; !ok {
			return nil, fmt.Errorf("source is missing the %q column", want)
		}
	}

	seen := map[string]bool{}
	var rows []record

	for {
		row, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("read record: %w", err)
		}

		ident := column(row, index, "ident")
		if !identShape.MatchString(ident) {
			continue
		}
		if _, why := reserved[ident]; why {
			continue
		}
		if seen[ident] {
			return nil, fmt.Errorf("duplicate ident %q", ident)
		}
		seen[ident] = true

		lat, lon, err := coordinates(column(row, index, "coordinates"))
		if err != nil {
			return nil, fmt.Errorf("%s: %w", ident, err)
		}

		out := record{fields: []string{
			ident,
			column(row, index, "type"),
			column(row, index, "name"),
			column(row, index, "municipality"),
			column(row, index, "iso_country"),
			column(row, index, "iso_region"),
			column(row, index, "iata_code"),
			elevation(column(row, index, "elevation_ft")),
			lat,
			lon,
		}}

		for _, field := range out.fields {
			if strings.ContainsAny(field, "\n\r") {
				return nil, fmt.Errorf("%s: a field carries a line break", ident)
			}
		}

		rows = append(rows, out)
	}

	sort.Slice(rows, func(i, j int) bool { return rows[i].fields[0] < rows[j].fields[0] })
	return rows, nil
}

func column(row []string, index map[string]int, name string) string {
	i, ok := index[name]
	if !ok || i >= len(row) {
		return ""
	}
	return strings.TrimSpace(row[i])
}

// elevation keeps a whole number of feet or nothing at all. An empty field is
// "not stated", which is a different thing from sea level.
func elevation(raw string) string {
	if raw == "" {
		return ""
	}
	feet, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return ""
	}
	return strconv.Itoa(int(math.Round(feet)))
}

// coordinates renders the pair at coordinateDigits, which is what makes it a
// token the location grammar accepts: the source carries anything from zero to
// eighteen fractional digits, and FormatDD requires at least four while
// Axis.Frac caps at eight.
//
// Negative zero is normalized away. A "-0.0000" axis would not reproduce its
// own canonical form, which is the same defect roundTo records for arm64.
func coordinates(raw string) (string, string, error) {
	parts := strings.Split(raw, ",")
	if len(parts) != 2 {
		return "", "", fmt.Errorf("coordinates %q are not a pair", raw)
	}

	lat, err := axis(parts[0], 90)
	if err != nil {
		return "", "", err
	}
	lon, err := axis(parts[1], 180)
	if err != nil {
		return "", "", err
	}
	if lat == "0.0000" && lon == "0.0000" {
		return "", "", fmt.Errorf("coordinates are the null pair")
	}
	return lat, lon, nil
}

func axis(raw string, limit float64) (string, error) {
	value, err := strconv.ParseFloat(strings.TrimSpace(raw), 64)
	if err != nil {
		return "", fmt.Errorf("axis %q: %w", raw, err)
	}

	scale := math.Pow(10, coordinateDigits)
	value = math.Round(value*scale) / scale
	if value == 0 {
		value = 0
	}
	if math.Abs(value) > limit {
		return "", fmt.Errorf("axis %v is outside %v", value, limit)
	}

	return strconv.FormatFloat(value, 'f', coordinateDigits, 64), nil
}

func write(path string, rows []record) error {
	f, err := os.Create(path) //nolint:gosec // G304: a path this program owns
	if err != nil {
		return fmt.Errorf("create %s: %w", path, err)
	}
	defer f.Close()

	w := csv.NewWriter(f)
	if err := w.Write(columns); err != nil {
		return fmt.Errorf("write header: %w", err)
	}
	for _, row := range rows {
		if err := w.Write(row.fields); err != nil {
			return fmt.Errorf("write record: %w", err)
		}
	}
	w.Flush()
	return w.Error()
}
