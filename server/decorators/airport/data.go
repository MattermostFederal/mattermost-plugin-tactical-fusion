package airport

import (
	_ "embed"
	"encoding/csv"
	"fmt"
	"strconv"
	"strings"
	"unicode"
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

// allowedPunctuation is every non-alphanumeric rune the shipped database uses.
//
// A whitelist rather than a blacklist, which is the rule this plugin already
// follows for the one other place message text reaches a reader. Escaping on
// output is the last layer, not the first, and here it cannot be the only one:
// a value reaching a posted message can carry things markdown escaping does not
// reach at all. "@here" is the sharp case. Backslash-escaping it suppresses the
// rendered link, but Mattermost's mention scan reads the RAW message text and
// treats a backslash as a separator, so the notification fires anyway, from a
// message whose author typed four letters.
//
// The set is measured against all 19,012 rows rather than guessed, so a
// refreshed database carrying anything new fails at init and in every test.
const allowedPunctuation = " _-,.'\"()[]/&`+" +
	"\u00a0" + // no-break space
	"\u00ad" + // soft hyphen
	"\u00b4" + // acute accent
	"\u2013" + // en dash
	"\u2019" + // right single quote
	"\u201c" + // left double quote
	"\u201e" //  low double quote

// autolinkTriggers are the sequences GFM turns into a link with no delimiters
// around them, which a rune whitelist cannot see because every character in
// them is otherwise ordinary.
//
// The "@" form needs no entry: the whitelist already refuses that rune, which
// covers both an email autolink and an at-mention.
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

		for _, field := range row[1:8] {
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
