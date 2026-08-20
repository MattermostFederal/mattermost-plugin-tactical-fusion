package airport

import (
	"strconv"
	"strings"

	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/decorators/location"
)

// coordinateDigits is what build/airportdata rounds to, and what the token
// below is built at. Four is the coarsest FormatDD accepts.
const coordinateDigits = 4

// Details is everything both surfaces show, already rendered.
//
// Strings rather than fields, for the reason Conversion is strings: the
// rendering rules live in Go, and handing the webapp raw fields would put two
// implementations of them in one repository.
//
// It carries the coordinate as a (format, token) PAIR rather than as a set of
// readings. An airfield surface shows the field and links to the coordinate
// rather than reprinting eleven rows the location decorator already renders, so
// what both surfaces need is the identity of that link and nothing else.
//
// The pair comes from here rather than being rebuilt in TypeScript because the
// two languages do not agree about formatting a float: Go rounds ties away from
// zero and JavaScript's toFixed is famously not exact, so a token built in the
// webapp could differ in the last digit and be refused by the very page it
// points at.
type Details struct {
	Ident     string
	Name      string
	Type      string
	Place     string
	Elevation string
	IATA      string

	// Format and Token are the location decorator's own (f, v) pair.
	Format string
	Token  string

	// Region is the country the position falls in, already carrying its own
	// citation. It reaches the map's accessible label and nothing else, which
	// is the only place the country reaches a reader who cannot see the map.
	Region string

	// HasPosition is false when the coordinate will not convert, which no row
	// in the shipped data does. It is still represented, because a surface must
	// never offer a link to a page that would refuse it.
	HasPosition bool
}

// Describe is the one lookup-and-render both the page and the API go through.
// A conversion that accepted an airfield the page refuses, or the other way
// round, would be two doors with different locks on them.
// DescribeFields is the airfield's own fields and nothing derived.
//
// Separate from Describe because Describe also runs position(), which projects
// the coordinate and looks the country up in the compiled-in polygons. Callers
// on the post path want the fields alone, and Parse's contract says that path
// must be cheap.
func DescribeFields(ident string) (Details, bool) {
	a, ok := Lookup(ident)
	if !ok {
		return Details{}, false
	}

	return fieldsOf(a), true
}

func fieldsOf(a Airport) Details {
	return Details{
		Ident:     a.Ident,
		Name:      a.Name,
		Type:      typeText(a.Type),
		Place:     placeText(a),
		Elevation: elevationText(a),
		IATA:      a.IATA,
	}
}

func Describe(ident string) (Details, bool) {
	a, ok := Lookup(ident)
	if !ok {
		return Details{}, false
	}

	d := fieldsOf(a)
	d.Format = string(location.FormatDD)
	d.Token, d.Region, d.HasPosition = position(a)

	return d, true
}

// position renders the airfield's coordinates as a signed decimal degrees token
// and checks that the location decorator accepts it.
//
// No space after the comma. Convert routes through validateParams, which
// requires the token to reproduce its own canonical form, and canonicalString
// writes the separator as a bare comma. A space makes this fail for every
// airfield, silently, and every surface would then offer no coordinate link at
// all.
//
// Only the region is kept from the conversion. The rest is discarded, and
// running it anyway is the point: it is the same gate the coordinate page
// applies, so a token that survives here cannot be refused by the surface it
// reaches.
func position(a Airport) (token, region string, ok bool) {
	token = strconv.FormatFloat(a.Lat, 'f', coordinateDigits, 64) +
		"," + strconv.FormatFloat(a.Lon, 'f', coordinateDigits, 64)

	conversion, ok := location.Convert(location.FormatDD, token, "")
	if !ok {
		return "", "", false
	}

	return token, conversion.Region, true
}

func typeText(raw string) string {
	if raw == "" {
		return ""
	}

	words := strings.Split(raw, "_")
	for i, w := range words {
		if w == "" {
			continue
		}
		words[i] = strings.ToUpper(w[:1]) + strings.ToLower(w[1:])
	}
	return strings.Join(words, " ")
}

// placeText is municipality, subdivision and country, skipping whichever the
// record does not carry. The ISO region is "US-IN", and only the subdivision
// half is worth printing beside a country that follows it.
func placeText(a Airport) string {
	var parts []string
	if a.Municipality != "" {
		parts = append(parts, a.Municipality)
	}
	if a.Region != "" {
		code := a.Region
		if _, after, found := strings.Cut(code, "-"); found && after != "" {
			code = after
		}
		parts = append(parts, code)
	}
	if a.Country != "" {
		parts = append(parts, a.Country)
	}
	return strings.Join(parts, ", ")
}

func elevationText(a Airport) string {
	if a.ElevationFt == nil {
		return ""
	}
	return withThousands(*a.ElevationFt) + " ft"
}

func withThousands(n int) string {
	sign := ""
	if n < 0 {
		sign = "-"
		n = -n
	}

	digits := strconv.Itoa(n)
	var b strings.Builder
	for i, r := range digits {
		if i > 0 && (len(digits)-i)%3 == 0 {
			b.WriteByte(',')
		}
		b.WriteRune(r)
	}
	return sign + b.String()
}
