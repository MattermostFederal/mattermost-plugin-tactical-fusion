package airport

import (
	"strconv"
	"strings"

	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/decorators/location"
)

const coordinateDigits = 4

type Coordinate struct {
	Format string
	Token  string
}

type Runway struct {
	Designation string
	Length      string
	Width       string
	Surface     string
	Lighted     string
	Closed      string
	Ends        *[2]Coordinate
}

type Frequency struct {
	Type        string
	Description string
	MHz         string
}

type Details struct {
	Ident     string
	Name      string
	Type      string
	Place     string
	Elevation string
	IATA      string
	Military  string

	Runways     []Runway
	Frequencies []Frequency

	Format string
	Token  string

	Region string

	HasPosition bool
}

func DescribeFields(ident string) (Details, bool) {
	a, ok := Lookup(ident)
	if !ok {
		return Details{}, false
	}

	return fieldsOf(a), true
}

func fieldsOf(a Airport) Details {
	return Details{
		Ident:       a.Ident,
		Name:        a.Name,
		Type:        typeText(a.Type),
		Place:       placeText(a),
		Elevation:   elevationText(a),
		IATA:        a.IATA,
		Military:    a.Military,
		Runways:     runwayDetails(runwaysOf(a.Ident)),
		Frequencies: frequencyDetails(frequenciesOf(a.Ident)),
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

func position(a Airport) (token, region string, ok bool) {
	token = ddToken(a.Lat, a.Lon)

	conversion, ok := location.Convert(location.FormatDD, token, "")
	if !ok {
		return "", "", false
	}

	return token, conversion.Region, true
}

func ddToken(lat, lon float64) string {
	return strconv.FormatFloat(lat, 'f', coordinateDigits, 64) +
		"," + strconv.FormatFloat(lon, 'f', coordinateDigits, 64)
}

func endCoordinate(lat, lon float64) (Coordinate, bool) {
	token := ddToken(lat, lon)
	parsed, ok := location.Parse(location.FormatDD, token)
	if !ok || parsed.Canonical() != token {
		return Coordinate{}, false
	}
	return Coordinate{Format: string(location.FormatDD), Token: token}, true
}

func runwayDetails(records []runwayRecord) []Runway {
	if len(records) == 0 {
		return nil
	}

	out := make([]Runway, 0, len(records))
	for _, r := range records {
		runway := Runway{
			Designation: designationText(r),
			Length:      feetText(r.LengthFt),
			Width:       feetText(r.WidthFt),
			Surface:     r.Surface,
		}
		if r.Lighted {
			runway.Lighted = "Lighted"
		}
		if r.Closed {
			runway.Closed = "Closed"
		}
		if r.HasEnds {
			low, lowOK := endCoordinate(r.LowLat, r.LowLon)
			high, highOK := endCoordinate(r.HighLat, r.HighLon)
			if lowOK && highOK {
				runway.Ends = &[2]Coordinate{low, high}
			}
		}
		out = append(out, runway)
	}
	return out
}

func designationText(r runwayRecord) string {
	if r.HighIdent == "" {
		return r.LowIdent
	}
	return r.LowIdent + "/" + r.HighIdent
}

func RunwayLine(r Runway) string {
	var parts []string
	switch {
	case r.Length != "" && r.Width != "":
		parts = append(parts, strings.TrimSuffix(r.Length, " ft")+" x "+r.Width)
	case r.Length != "":
		parts = append(parts, r.Length)
	case r.Width != "":
		parts = append(parts, r.Width+" wide")
	}
	if r.Surface != "" {
		parts = append(parts, r.Surface)
	}
	if r.Lighted != "" {
		parts = append(parts, strings.ToLower(r.Lighted))
	}
	if r.Closed != "" {
		parts = append(parts, strings.ToLower(r.Closed))
	}
	return strings.Join(parts, ", ")
}

func frequencyDetails(records []frequencyRecord) []Frequency {
	if len(records) == 0 {
		return nil
	}

	out := make([]Frequency, 0, len(records))
	for _, r := range records {
		out = append(out, Frequency(r))
	}
	return out
}

func feetText(feet *int) string {
	if feet == nil {
		return ""
	}
	return withThousands(*feet) + " ft"
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
	return feetText(a.ElevationFt)
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
