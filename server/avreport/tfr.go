package avreport

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/decorators/airport"
	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/decorators/location"
)

const (
	RestrictionLabel = "Restriction"
	tfrHeading       = "Temporary flight restriction"
	MinAreaPoints    = 3
)

const tfrCoordExpr = `(\d{6}[NS]\d{7}[EW]|\d{4}[NS]\d{5}[EW])`

var (
	tfrNamedPattern     = regexp.MustCompile(`TEMPORARY FLIGHT RESTRICTION`)
	tfrSectionPattern   = regexp.MustCompile(`14 CFR (?:SECTION|PART|SEC) (\d{2}\.\d+)`)
	tfrEffectivePattern = regexp.MustCompile(`EFFECTIVE (?:(\d{10}) UTC|IMMEDIATELY)(?: UNTIL (?:(\d{10}) UTC|(FURTHER NOTICE)))?`)
	tfrAltitudePattern  = regexp.MustCompile(`\b(SFC|\d+ ?FT (?:MSL|AGL))-(\d+ ?FT (?:MSL|AGL)|FL ?\d{3}|UNL)\b`)
	tfrCirclePattern    = regexp.MustCompile(`\b(\d{1,3}(?:\.\d+)?) NM RADIUS OF ` + tfrCoordExpr + `(?: \(([^)]*)\))?`)
	tfrPlacePattern     = regexp.MustCompile(`^(.+?)\. TEMPORARY FLIGHT RESTRICTION`)
	tfrAreaStart        = regexp.MustCompile(`AREA DEFINED AS `)
	tfrAreaVertex       = regexp.MustCompile(`^` + tfrCoordExpr + `(?: \([^)]*\))?`)
	tfrAreaNext         = regexp.MustCompile(`^ TO (?:(?:THE )?POINT OF ORIGIN)?`)
	tfrFeetPattern      = regexp.MustCompile(`^(\d+) ?FT (MSL|AGL)$`)
)

func (r Report) IsRestriction() bool {
	return hasRow(r, RestrictionLabel)
}

func readTFR(report *Report, body string) {
	restriction := restrictionText(body)
	if restriction == "" {
		return
	}

	rows := make([]Row, 0, len(report.Rows)+2)
	rows = append(rows, Row{Label: RestrictionLabel, Value: restriction})
	if m := tfrPlacePattern.FindStringSubmatch(body); m != nil {
		rows = append(rows, Row{Label: "Place", Value: m[1]})
	}
	for _, row := range report.Rows {
		if row.Label != "Subject" {
			rows = append(rows, row)
		}
	}
	report.Rows = rows

	if m := tfrAltitudePattern.FindStringSubmatch(body); m != nil {
		report.Rows = append(report.Rows, Row{Label: "Altitudes", Value: altitudeText(m[1]) + " to " + altitudeText(m[2])})
	}

	if m := tfrCirclePattern.FindStringSubmatch(body); m != nil {
		center, ok := tfrPoint(m[2])
		switch {
		case !positiveRadius(m[1]):
			report.Unknown = append(report.Unknown, m[1]+" NM")
		case ok:
			report.Center = &center
			report.RadiusNm = m[1]
			report.Rows = append(report.Rows, Row{Label: "Radius", Value: m[1] + " NM"})
			if m[3] != "" {
				report.Rows = append(report.Rows, Row{Label: "Reference", Value: strings.ToUpper(expandContractions(m[3]))})
			}
		default:
			report.Unknown = append(report.Unknown, m[2])
		}
	}

	if ring, ok := tfrArea(report, body); ok {
		report.Area = ring
		if report.Center == nil {
			center := centroid(ring)
			report.Center = &center
		}
		report.Rows = append(report.Rows, Row{Label: "Area", Value: strconv.Itoa(len(ring)) + " points"})
	}

	if loc := tfrEffectivePattern.FindStringIndex(body); loc != nil {
		operations := strings.TrimSpace(strings.TrimLeft(body[loc[1]:], ". "))
		if operations != "" {
			report.Rows = append(report.Rows, Row{Label: "Operations", Value: strings.ToUpper(expandContractions(operations))})
		}
	}
}

func readTFREffective(report *Report, body string) {
	m := tfrEffectivePattern.FindStringSubmatch(body)
	if m == nil {
		return
	}

	if m[1] == "" {
		report.Rows = append(report.Rows, Row{Label: "Effective", Value: "immediately"})
	} else if start, ok := fullDate(m[1]); ok {
		report.IssuedAt = start
		report.Rows = append(report.Rows, Row{Label: "Effective", Value: zuluText(start), At: start})
	} else {
		report.Unknown = append(report.Unknown, m[1])
	}

	switch {
	case m[3] != "":
		report.Rows = append(report.Rows, Row{Label: "Expires", Value: "until further notice"})
	case m[2] != "":
		if end, ok := fullDate(m[2]); ok {
			report.Rows = append(report.Rows, Row{Label: "Expires", Value: zuluText(end), At: end})
		} else {
			report.Unknown = append(report.Unknown, m[2])
		}
	}
}

func restrictionText(body string) string {
	section := ""
	if m := tfrSectionPattern.FindStringSubmatch(body); m != nil {
		section = m[1]
	}
	if section == "" && !tfrNamedPattern.MatchString(body) {
		return ""
	}
	if section == "" {
		return "temporary flight restriction"
	}
	return "temporary flight restriction (14 CFR " + section + ")"
}

func altitudeText(raw string) string {
	switch {
	case raw == "SFC":
		return "surface"
	case raw == "UNL":
		return "unlimited"
	case strings.HasPrefix(raw, "FL"):
		return "FL" + strings.TrimSpace(strings.TrimPrefix(raw, "FL"))
	}
	if m := tfrFeetPattern.FindStringSubmatch(raw); m != nil {
		if feet, err := strconv.Atoi(m[1]); err == nil {
			return airport.WithThousands(feet) + " ft " + m[2]
		}
	}
	return raw
}

func tfrPoint(raw string) (Center, bool) {
	for _, format := range []location.Format{location.FormatDMS, location.FormatLATM} {
		if loc, ok := location.Parse(format, raw); ok {
			return Center{Lat: loc.Lat.Decimal(), Lon: loc.Lon.Decimal()}, true
		}
	}
	return Center{}, false
}

func tfrArea(report *Report, body string) ([]Center, bool) {
	start := tfrAreaStart.FindStringIndex(body)
	if start == nil {
		return nil, false
	}

	rest := body[start[1]:]
	var ring []Center
	for {
		vertex := tfrAreaVertex.FindStringSubmatchIndex(rest)
		if vertex == nil {
			break
		}
		point, ok := tfrPoint(rest[vertex[2]:vertex[3]])
		if !ok {
			report.Unknown = append(report.Unknown, rest[vertex[2]:vertex[3]])
			return nil, false
		}
		ring = append(ring, point)
		rest = rest[vertex[1]:]

		next := tfrAreaNext.FindString(rest)
		if next == "" {
			break
		}
		rest = rest[len(next):]
		if strings.HasSuffix(strings.TrimSpace(next), "ORIGIN") {
			break
		}
	}

	if len(ring) < MinAreaPoints || len(ring) > MaxAreaPoints {
		return nil, false
	}
	for _, point := range ring {
		if _, ok := airport.DDToken(point.Lat, point.Lon); !ok {
			return nil, false
		}
	}
	return ring, true
}

func positiveRadius(text string) bool {
	radius, err := strconv.ParseFloat(text, 64)
	return err == nil && radius > 0
}

func centroid(ring []Center) Center {
	var lat, lon float64
	for _, point := range ring {
		lat += point.Lat
		lon += unwrappedLongitude(point.Lon, ring[0].Lon)
	}
	n := float64(len(ring))
	return Center{Lat: lat / n, Lon: wrappedLongitude(lon / n)}
}

func unwrappedLongitude(lon, reference float64) float64 {
	switch {
	case lon-reference > 180:
		return lon - 360
	case reference-lon > 180:
		return lon + 360
	}
	return lon
}

func wrappedLongitude(lon float64) float64 {
	switch {
	case lon > 180:
		return lon - 360
	case lon < -180:
		return lon + 360
	}
	return lon
}

func areaTokens(ring []Center) []any {
	tokens := make([]any, 0, len(ring))
	for _, point := range ring {
		if token, ok := airport.DDToken(point.Lat, point.Lon); ok {
			tokens = append(tokens, token)
		}
	}
	return tokens
}
