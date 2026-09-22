package avreport

import (
	"regexp"
	"strconv"
	"strings"
	"time"
)

var metarHeaderPattern = regexp.MustCompile(`^(?:(METAR|SPECI)[ \t]+)?(?:(COR)[ \t]+)?([A-Z][A-Z0-9]{3})[ \t]+(\d{2})(\d{2})(\d{2})Z(?:[ \t]+(.*))?$`)

var bareWindPattern = regexp.MustCompile(`^(?:AUTO[ \t]+|COR[ \t]+)?(?:\d{3}|VRB)\d{2,3}(?:G\d{2,3})?(?:KT|MPS)(?:[ \t]|$)`)

var trendKeywords = map[string]string{
	"NOSIG": "no significant change expected",
	"TEMPO": "temporarily",
	"BECMG": "becoming",
}

func decodeMETAR(line string, ref time.Time) (Report, bool) {
	m := metarHeaderPattern.FindStringSubmatch(strings.TrimSpace(line))
	if m == nil {
		return Report{}, false
	}
	keyword, corrected, station, day, hour, minute, rest := m[1], m[2], m[3], m[4], m[5], m[6], m[7]

	if keyword == "" && !bareWindPattern.MatchString(rest) {
		return Report{}, false
	}

	report := Report{Kind: KindMETAR, Station: station}
	if keyword == KindSPECI {
		report.Kind = KindSPECI
	}
	if corrected != "" {
		report.Flags = append(report.Flags, "COR")
	}

	issued, ok := resolveGroup(day, hour, minute, ref)
	if !ok {
		return Report{}, false
	}
	report.IssuedAt = issued
	report.Inferred = true

	body, remarks := splitRemarks(rest)
	report.Rows, report.Periods, report.Unknown = decodeBody(strings.Fields(body), &report.Flags)
	if remarks != "" {
		report.Remarks, report.Unknown = decodeRemarks(strings.Fields(remarks), report.Unknown)
	}
	report.Summary = summarize(report.Rows)

	return report, true
}

func splitRemarks(rest string) (string, string) {
	body, remarks, found := strings.Cut(" "+rest+" ", " RMK ")
	if !found {
		return strings.TrimSpace(rest), ""
	}
	return strings.TrimSpace(body), strings.TrimSpace(remarks)
}

func decodeBody(tokens []string, flags *[]string) ([]Row, []Period, []string) {
	var rows []Row
	var periods []Period
	var unknown []string

	current := &rows
	for i := 0; i < len(tokens); i++ {
		token := tokens[i]

		switch token {
		case "AUTO", "COR", "NIL", "AMD", "CNL":
			*flags = append(*flags, token)
			continue
		case "CAVOK":
			*current = append(*current, Row{Label: "Visibility", Value: "10 km or more, no cloud below 5,000 ft, no significant weather"})
			continue
		case "NSW":
			*current = append(*current, Row{Label: "Weather", Value: "no significant weather"})
			continue
		case "WS":
			if i+2 < len(tokens) && tokens[i+1] == "ALL" && tokens[i+2] == "RWY" {
				*current = append(*current, Row{Label: "Wind shear", Value: "all runways"})
				i += 2
				continue
			}
			if i+1 < len(tokens) && strings.HasPrefix(tokens[i+1], "R") {
				*current = append(*current, Row{Label: "Wind shear", Value: "runway " + strings.TrimPrefix(tokens[i+1], "R")})
				i++
				continue
			}
		}

		if text, ok := trendKeywords[token]; ok {
			label := token + " (" + text + ")"
			if token == "NOSIG" {
				*current = append(*current, Row{Label: "Trend", Value: text})
				continue
			}
			periods = append(periods, Period{Period: label})
			current = &periods[len(periods)-1].Rows
			continue
		}

		if row, consumed, ok := decodeGroup(tokens, i); ok {
			*current = append(*current, row)
			i += consumed
			continue
		}

		unknown = append(unknown, token)
	}

	return rows, periods, unknown
}

func decodeGroup(tokens []string, i int) (Row, int, bool) {
	token := tokens[i]

	if m := windPattern.FindStringSubmatch(token); m != nil {
		return Row{Label: "Wind", Value: windText(m)}, 0, true
	}
	if m := windVariablePattern.FindStringSubmatch(token); m != nil {
		return Row{Label: "Wind direction", Value: "varying between " + compass(m[1]) + " and " + compass(m[2])}, 0, true
	}
	if m := visibilityMetersPattern.FindStringSubmatch(token); m != nil && !looksLikeValidity(tokens, i) {
		return Row{Label: "Visibility", Value: visibilityMetersText(m)}, 0, true
	}
	if i+1 < len(tokens) && isWholeNumber(token) && strings.HasSuffix(tokens[i+1], "SM") && strings.Contains(tokens[i+1], "/") {
		if m := visibilityMilesPattern.FindStringSubmatch(token + " " + tokens[i+1]); m != nil {
			return Row{Label: "Visibility", Value: visibilityMilesText(m)}, 1, true
		}
	}
	if m := visibilityMilesPattern.FindStringSubmatch(token); m != nil && (m[2] != "" || m[3] != "") {
		return Row{Label: "Visibility", Value: visibilityMilesText(m)}, 0, true
	}
	if m := rvrPattern.FindStringSubmatch(token); m != nil {
		return Row{Label: "Runway visual range", Value: rvrText(m)}, 0, true
	}
	if m := weatherPattern.FindStringSubmatch(token); m != nil {
		return Row{Label: "Weather", Value: weatherText(m)}, 0, true
	}
	if text, ok := skyClear[token]; ok {
		return Row{Label: "Sky", Value: text}, 0, true
	}
	if m := skyPattern.FindStringSubmatch(token); m != nil {
		return Row{Label: "Sky", Value: skyText(m)}, 0, true
	}
	if m := verticalVisibilityPattern.FindStringSubmatch(token); m != nil {
		if m[1] == "///" {
			return Row{Label: "Sky", Value: "sky obscured, vertical visibility unknown"}, 0, true
		}
		hundreds, _ := strconv.Atoi(m[1])
		return Row{Label: "Sky", Value: "sky obscured, vertical visibility " + withThousands(hundreds*100) + " ft"}, 0, true
	}
	if m := temperaturePattern.FindStringSubmatch(token); m != nil {
		temperature, hasTemperature := temperatureValue(m[1])
		dewPoint, hasDewPoint := temperatureValue(m[2])
		switch {
		case hasTemperature && hasDewPoint:
			return Row{Label: "Temperature", Value: temperature + ", dew point " + dewPoint}, 0, true
		case hasTemperature:
			return Row{Label: "Temperature", Value: temperature + ", dew point not reported"}, 0, true
		case hasDewPoint:
			return Row{Label: "Temperature", Value: "not reported, dew point " + dewPoint}, 0, true
		}
	}
	if m := altimeterPattern.FindStringSubmatch(token); m != nil {
		return Row{Label: "Altimeter", Value: altimeterText(m)}, 0, true
	}
	if m := windShearPattern.FindStringSubmatch(token); m != nil {
		speed, _ := strconv.Atoi(m[3])
		return Row{Label: "Wind shear", Value: "at " + withThousands(mustAtoi(m[1])*100) + " ft, wind " + compass(m[2]) + " at " + strconv.Itoa(speed) + " kt"}, 0, true
	}

	return Row{}, 0, false
}

func looksLikeValidity(tokens []string, i int) bool {
	return i > 0 && tokens[i-1] == "TAF"
}

func isWholeNumber(token string) bool {
	if token == "" || len(token) > 2 {
		return false
	}
	for _, r := range token {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func mustAtoi(s string) int {
	n, _ := strconv.Atoi(s)
	return n
}

func decodeRemarks(tokens []string, unknown []string) ([]Row, []string) {
	var remarks []Row

	for i := 0; i < len(tokens); i++ {
		token := tokens[i]

		if text, ok := remarkFlags[token]; ok {
			remarks = append(remarks, Row{Label: "Station", Value: text})
			continue
		}
		if m := slpPattern.FindStringSubmatch(token); m != nil {
			remarks = append(remarks, Row{Label: "Sea level pressure", Value: slpText(m)})
			continue
		}
		if m := preciseTemperaturePattern.FindStringSubmatch(token); m != nil {
			remarks = append(remarks, Row{Label: "Precise temperature", Value: preciseTemperatureText(m)})
			continue
		}
		if token == "PK" && i+2 < len(tokens) && tokens[i+1] == "WND" {
			if m := peakWindPattern.FindStringSubmatch(tokens[i+2]); m != nil {
				speed, _ := strconv.Atoi(m[2])
				at := m[4]
				if m[3] != "" {
					at = m[3] + ":" + m[4]
				} else {
					at = ":" + at
				}
				remarks = append(remarks, Row{Label: "Peak wind", Value: compass(m[1]) + " at " + strconv.Itoa(speed) + " kt at " + at + "Z"})
				i += 2
				continue
			}
		}
		if m := precipitationEventPattern.FindStringSubmatch(token); m != nil {
			remarks = append(remarks, Row{Label: "Precipitation", Value: precipitationEventText(m)})
			continue
		}
		if m := hourlyPrecipitationPattern.FindStringSubmatch(token); m != nil {
			hundredths, _ := strconv.Atoi(m[1])
			remarks = append(remarks, Row{Label: "Hourly precipitation", Value: strconv.FormatFloat(float64(hundredths)/100, 'f', 2, 64) + " in"})
			continue
		}

		unknown = append(unknown, "RMK "+token)
	}

	return remarks, unknown
}

var eventPattern = regexp.MustCompile(`([BE])(\d{2,4})`)

func precipitationEventText(m []string) string {
	weather := weatherPattern.FindStringSubmatch(m[1])
	name := m[1]
	if weather != nil {
		name = weatherText(weather)
	}

	var events []string
	for _, event := range eventPattern.FindAllStringSubmatch(m[2], -1) {
		verb := "began"
		if event[1] == "E" {
			verb = "ended"
		}
		at := event[2]
		if len(at) == 4 {
			at = at[:2] + ":" + at[2:]
		} else {
			at = ":" + at
		}
		events = append(events, verb+" at "+at+"Z")
	}
	return name + " " + strings.Join(events, ", ")
}

func summarize(rows []Row) string {
	wanted := []string{"Wind", "Visibility", "Weather", "Sky", "Temperature", "Altimeter"}
	seen := map[string]bool{}
	var parts []string
	for _, label := range wanted {
		for _, row := range rows {
			if row.Label != label || seen[label] {
				continue
			}
			seen[label] = true
			switch label {
			case "Wind":
				parts = append(parts, "wind "+row.Value)
			default:
				parts = append(parts, row.Value)
			}
		}
	}
	if len(parts) == 0 {
		return ""
	}
	text := strings.Join(parts, "; ")
	return strings.ToUpper(text[:1]) + text[1:]
}
