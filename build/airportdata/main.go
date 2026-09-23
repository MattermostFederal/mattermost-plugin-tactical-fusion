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
	"unicode"
	"unicode/utf8"
)

const (
	defaultSourceDir = "build/airportdata/source"
	destinationDir   = "server/decorators/airport/data"

	airportsSource     = "airports.csv"
	runwaysSource      = "runways.csv"
	frequenciesSource  = "airport-frequencies.csv"
	airportsOutput     = "airports.csv"
	runwaysOutput      = "runways.csv"
	frequenciesOutput  = "frequencies.csv"
	coordinateDigits   = 4
	frequencyDigits    = 3
	maxSurfaceRunes    = 24
	maxRunwaysPer      = 32
	maxFrequenciesPer  = 48
	minFrequencyMHz    = 0.1
	maxFrequencyMHz    = 1300
	maxRunwayLengthFt  = 30000
	maxRunwayWidthFt   = 3000
	maxHeadingDegrees  = 360
	maxTypeRunes       = 12
	maxDescriptionRune = 64
)

var identShape = regexp.MustCompile(`^[A-Z]{4}$`)

var iataShape = regexp.MustCompile(`^[A-Z]{3}$`)

var reserved = map[string]string{
	"ZZZZ": "ICAO reserves it for an aerodrome that is not listed",
}

var militaryDesignators = []string{
	"Air Force Base", "Air Base", "Airbase", "Naval Air Station", "Marine Corps Air Station",
	"Army Airfield", "Army Air Field", "Joint Base", "Air Station",
	"AFB", "AB", "NAS", "MCAS", "AAF", "AFS", "ANGB", "RAF", "RAAF", "RNZAF", "CFB", "NAF", "MCAF",
}

var surfaceNames = map[string]string{
	"ASP": "Asphalt", "ASPH": "Asphalt", "ASPHALT": "Asphalt", "ASPH-G": "Asphalt", "ASPH-F": "Asphalt",
	"ASPH-P": "Asphalt", "ASPH-E": "Asphalt", "BIT": "Asphalt", "PAVED": "Paved", "PAV": "Paved",
	"CON": "Concrete", "CONC": "Concrete", "CONCRETE": "Concrete", "CONC-G": "Concrete", "CONC-F": "Concrete",
	"C": "Concrete", "B": "Asphalt", "A": "Asphalt",
	"GRS": "Grass", "GRASS": "Grass", "TURF": "Turf", "TURF-G": "Turf", "TURF-F": "Turf", "TURF-P": "Turf",
	"G": "Grass", "GRE": "Gravel", "GVL": "Gravel", "GRVL": "Gravel", "GRAVEL": "Gravel", "GRV": "Gravel",
	"GRVL-G": "Gravel", "GRVL-F": "Gravel", "GRVL-P": "Gravel", "GRAVEL-G": "Gravel",
	"DIRT": "Dirt", "DIRT-G": "Dirt", "DIRT-F": "Dirt", "DIRT-P": "Dirt", "EARTH": "Earth", "CLA": "Clay",
	"SAN": "Sand", "SAND": "Sand", "WATER": "Water", "WAT": "Water", "PEM": "Permeable", "PER": "Permeable",
	"MET": "Metal", "MAT": "Matting", "PSP": "Matting", "COR": "Coral", "CORAL": "Coral", "COP": "Composite",
	"COM": "Composite", "ICE": "Ice", "SNOW": "Snow", "SNW": "Snow", "LAT": "Laterite", "MAC": "Macadam",
	"TAR": "Tarmac", "TARMAC": "Tarmac", "UNK": "", "UNKNOWN": "", "X": "", "N": "", "S": "", "L": "", "U": "",
}

var allowedPunctuation = " _-,.'\"()[]/&`+" + "\u00a0\u00ad\u00b4\u2013\u2019\u201c\u201e"

var autolinkTriggers = []string{"www.", "://"}

type record []string

type table struct {
	columns []string
	rows    []record
}

type counts struct {
	airfields, military, runways, frequencies         int
	surfacesNormalized, surfacesKept, surfacesDropped int
	descriptionsDropped, frequenciesDropped           int
	runwaysDropped                                    int
}

func main() {
	sourceDir := defaultSourceDir
	if len(os.Args) > 1 {
		sourceDir = os.Args[1]
	}

	if err := run(sourceDir); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(sourceDir string) error {
	var c counts

	airfields, kept, err := filterAirfields(sourceDir+"/"+airportsSource, &c)
	if err != nil {
		return err
	}
	runways, err := filterRunways(sourceDir+"/"+runwaysSource, kept, &c)
	if err != nil {
		return err
	}
	frequencies, err := filterFrequencies(sourceDir+"/"+frequenciesSource, kept, &c)
	if err != nil {
		return err
	}

	for name, t := range map[string]table{
		airportsOutput:    airfields,
		runwaysOutput:     runways,
		frequenciesOutput: frequencies,
	} {
		if err := write(destinationDir+"/"+name, t); err != nil {
			return err
		}
	}

	fmt.Printf("wrote %d airfields (%d with a military designator), %d runways and %d frequencies to %s\n",
		c.airfields, c.military, c.runways, c.frequencies, destinationDir)
	fmt.Printf("surfaces: %d normalized, %d kept as written, %d dropped\n",
		c.surfacesNormalized, c.surfacesKept, c.surfacesDropped)
	fmt.Printf("dropped: %d runways, %d frequencies, %d frequency descriptions\n",
		c.runwaysDropped, c.frequenciesDropped, c.descriptionsDropped)
	return nil
}

func filterAirfields(source string, c *counts) (table, map[string]bool, error) {
	out := table{columns: []string{
		"ident", "type", "name", "municipality",
		"iso_country", "iso_region", "iata_code", "elevation_ft",
		"lat", "lon", "military",
	}}
	kept := map[string]bool{}
	iata := map[string]string{}

	err := readRows(source, []string{"ident", "latitude_deg", "longitude_deg"}, func(row map[string]string) error {
		ident := row["ident"]
		if !identShape.MatchString(ident) {
			return nil
		}
		if _, why := reserved[ident]; why {
			return nil
		}
		if kept[ident] {
			return fmt.Errorf("duplicate ident %q", ident)
		}
		kept[ident] = true

		lat, err := axis(row["latitude_deg"], 90)
		if err != nil {
			return fmt.Errorf("%s: %w", ident, err)
		}
		lon, err := axis(row["longitude_deg"], 180)
		if err != nil {
			return fmt.Errorf("%s: %w", ident, err)
		}
		if lat == "0.0000" && lon == "0.0000" {
			return fmt.Errorf("%s: coordinates are the null pair", ident)
		}

		code := row["iata_code"]
		if code != "" {
			if !iataShape.MatchString(code) {
				return fmt.Errorf("%s: IATA code %q is not three upper-case letters", ident, code)
			}
			if other, dup := iata[code]; dup {
				return fmt.Errorf("IATA code %q names both %s and %s", code, other, ident)
			}
			iata[code] = ident
		}

		designator := militaryDesignator(row["name"])
		if designator != "" {
			c.military++
		}

		out.rows = append(out.rows, record{
			ident, row["type"], row["name"], row["municipality"],
			row["iso_country"], row["iso_region"], code, elevation(row["elevation_ft"]),
			lat, lon, designator,
		})
		return nil
	})
	if err != nil {
		return table{}, nil, err
	}

	sort.Slice(out.rows, func(i, j int) bool { return out.rows[i][0] < out.rows[j][0] })
	c.airfields = len(out.rows)
	return out, kept, nil
}

func filterRunways(source string, kept map[string]bool, c *counts) (table, error) {
	out := table{columns: []string{
		"ident", "le_ident", "he_ident", "length_ft", "width_ft", "surface", "lighted", "closed",
		"le_lat", "le_lon", "he_lat", "he_lon", "le_heading",
	}}
	per := map[string]int{}

	err := readRows(source, []string{"airport_ident", "le_ident"}, func(row map[string]string) error {
		ident := row["airport_ident"]
		if !kept[ident] {
			return nil
		}

		le, he := row["le_ident"], row["he_ident"]
		if le == "" || !validText(le) || !validText(he) {
			c.runwaysDropped++
			return nil
		}

		length, err := wholeNumber(row["length_ft"], maxRunwayLengthFt)
		if err != nil {
			return fmt.Errorf("%s %s: length: %w", ident, le, err)
		}
		width, err := wholeNumber(row["width_ft"], maxRunwayWidthFt)
		if err != nil {
			return fmt.Errorf("%s %s: width: %w", ident, le, err)
		}
		heading, err := wholeNumber(row["le_heading_degT"], maxHeadingDegrees)
		if err != nil {
			return fmt.Errorf("%s %s: heading: %w", ident, le, err)
		}

		leLat, leLon, err := optionalPair(row["le_latitude_deg"], row["le_longitude_deg"])
		if err != nil {
			return fmt.Errorf("%s %s: low end: %w", ident, le, err)
		}
		heLat, heLon, err := optionalPair(row["he_latitude_deg"], row["he_longitude_deg"])
		if err != nil {
			return fmt.Errorf("%s %s: high end: %w", ident, le, err)
		}
		if leLat == "" || heLat == "" {
			leLat, leLon, heLat, heLon = "", "", "", ""
		}

		per[ident]++
		if per[ident] > maxRunwaysPer {
			return fmt.Errorf("%s has more than %d runways", ident, maxRunwaysPer)
		}

		out.rows = append(out.rows, record{
			ident, le, he, length, width, surface(row["surface"], c),
			flag(row["lighted"]), flag(row["closed"]),
			leLat, leLon, heLat, heLon, heading,
		})
		return nil
	})
	if err != nil {
		return table{}, err
	}

	sort.SliceStable(out.rows, func(i, j int) bool {
		if out.rows[i][0] != out.rows[j][0] {
			return out.rows[i][0] < out.rows[j][0]
		}
		return out.rows[i][1] < out.rows[j][1]
	})
	c.runways = len(out.rows)
	return out, nil
}

func filterFrequencies(source string, kept map[string]bool, c *counts) (table, error) {
	out := table{columns: []string{"ident", "type", "description", "mhz"}}
	per := map[string]int{}

	err := readRows(source, []string{"airport_ident", "type", "frequency_mhz"}, func(row map[string]string) error {
		ident := row["airport_ident"]
		if !kept[ident] {
			return nil
		}

		kind := row["type"]
		if kind == "" || !validText(kind) || utf8.RuneCountInString(kind) > maxTypeRunes {
			c.frequenciesDropped++
			return nil
		}

		mhz, err := strconv.ParseFloat(row["frequency_mhz"], 64)
		if err != nil || math.IsNaN(mhz) || math.IsInf(mhz, 0) || mhz < minFrequencyMHz || mhz > maxFrequencyMHz {
			c.frequenciesDropped++
			return nil
		}

		description := row["description"]
		if !validText(description) || utf8.RuneCountInString(description) > maxDescriptionRune {
			description = ""
			c.descriptionsDropped++
		}

		per[ident]++
		if per[ident] > maxFrequenciesPer {
			return fmt.Errorf("%s has more than %d frequencies", ident, maxFrequenciesPer)
		}

		out.rows = append(out.rows, record{
			ident, kind, description, strconv.FormatFloat(mhz, 'f', frequencyDigits, 64),
		})
		return nil
	})
	if err != nil {
		return table{}, err
	}

	sort.SliceStable(out.rows, func(i, j int) bool {
		a, b := out.rows[i], out.rows[j]
		if a[0] != b[0] {
			return a[0] < b[0]
		}
		if a[1] != b[1] {
			return a[1] < b[1]
		}
		return a[3] < b[3]
	})
	c.frequencies = len(out.rows)
	return out, nil
}

func readRows(source string, required []string, each func(row map[string]string) error) error {
	f, err := os.Open(source) //nolint:gosec // G304: a path this program is told to read
	if err != nil {
		return fmt.Errorf("open %s: %w", source, err)
	}
	defer f.Close()

	reader := csv.NewReader(f)
	header, err := reader.Read()
	if err != nil {
		return fmt.Errorf("%s: read header: %w", source, err)
	}

	index := map[string]int{}
	for i, name := range header {
		index[name] = i
	}
	for _, want := range required {
		if _, ok := index[want]; !ok {
			return fmt.Errorf("%s is missing the %q column", source, want)
		}
	}

	for {
		raw, err := reader.Read()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return fmt.Errorf("%s: read record: %w", source, err)
		}

		row := make(map[string]string, len(header))
		for name, i := range index {
			if i < len(raw) {
				row[name] = strings.TrimSpace(raw[i])
			}
		}
		for name, value := range row {
			if strings.ContainsAny(value, "\n\r") {
				return fmt.Errorf("%s: %s: the %s field carries a line break", source, row["ident"]+row["airport_ident"], name)
			}
		}

		if err := each(row); err != nil {
			return err
		}
	}
}

func militaryDesignator(name string) string {
	for _, designator := range militaryDesignators {
		at := 0
		for {
			i := strings.Index(name[at:], designator)
			if i < 0 {
				break
			}
			start, end := at+i, at+i+len(designator)
			if wordBoundary(name, start, end) {
				return designator
			}
			at = start + 1
		}
	}
	return ""
}

func wordBoundary(s string, start, end int) bool {
	before, _ := utf8.DecodeLastRuneInString(s[:start])
	after, _ := utf8.DecodeRuneInString(s[end:])
	return !unicode.IsLetter(before) && !unicode.IsLetter(after)
}

func surface(raw string, c *counts) string {
	if raw == "" {
		return ""
	}
	if named, ok := surfaceNames[strings.ToUpper(raw)]; ok {
		c.surfacesNormalized++
		return named
	}
	if validText(raw) && utf8.RuneCountInString(raw) <= maxSurfaceRunes {
		c.surfacesKept++
		return raw
	}
	c.surfacesDropped++
	return ""
}

func flag(raw string) string {
	if raw == "1" {
		return "1"
	}
	return "0"
}

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

func wholeNumber(raw string, limit float64) (string, error) {
	if raw == "" {
		return "", nil
	}
	value, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return "", err
	}
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return "", fmt.Errorf("%q is not a finite number", raw)
	}
	if value < 0 || value > limit {
		return "", nil
	}
	return strconv.Itoa(int(math.Round(value))), nil
}

func optionalPair(rawLat, rawLon string) (string, string, error) {
	if rawLat == "" || rawLon == "" {
		return "", "", nil
	}
	lat, err := axis(rawLat, 90)
	if err != nil {
		return "", "", err
	}
	lon, err := axis(rawLon, 180)
	if err != nil {
		return "", "", err
	}
	if lat == "0.0000" && lon == "0.0000" {
		return "", "", nil
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

	if math.IsNaN(value) || math.IsInf(value, 0) {
		return "", fmt.Errorf("axis %q is not a finite number", raw)
	}
	if math.Abs(value) > limit {
		return "", fmt.Errorf("axis %v is outside %v", value, limit)
	}

	return strconv.FormatFloat(value, 'f', coordinateDigits, 64), nil
}

func write(path string, t table) error {
	f, err := os.Create(path) //nolint:gosec // G304: a path this program owns
	if err != nil {
		return fmt.Errorf("create %s: %w", path, err)
	}
	defer f.Close()

	w := csv.NewWriter(f)
	if err := w.Write(t.columns); err != nil {
		return fmt.Errorf("write header: %w", err)
	}
	for _, row := range t.rows {
		if err := w.Write(row); err != nil {
			return fmt.Errorf("write record: %w", err)
		}
	}
	w.Flush()
	return w.Error()
}
