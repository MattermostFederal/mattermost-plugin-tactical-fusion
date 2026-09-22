package avreport

import (
	"regexp"
	"strings"
	"time"
)

var tafHeaderPattern = regexp.MustCompile(`^TAF(?:[ \t]+(AMD|COR))?[ \t]+([A-Z][A-Z0-9]{3})[ \t]+(\d{2})(\d{2})(\d{2})Z(?:[ \t]+(.*))?$`)

func decodeTAF(text string, ref time.Time) (Report, bool) {
	normalized := strings.Join(strings.Fields(text), " ")
	m := tafHeaderPattern.FindStringSubmatch(normalized)
	if m == nil {
		return Report{}, false
	}
	amended, station, day, hour, minute, rest := m[1], m[2], m[3], m[4], m[5], m[6]

	report := Report{Kind: KindTAF, Station: station}
	if amended != "" {
		report.Flags = append(report.Flags, amended)
	}

	issued, ok := resolveGroup(day, hour, minute, ref)
	if !ok {
		return Report{}, false
	}
	report.IssuedAt = issued
	report.Inferred = true

	tokens := strings.Fields(rest)
	if len(tokens) == 0 {
		return Report{}, false
	}

	switch tokens[0] {
	case "NIL", "CNL":
		report.Flags = append(report.Flags, tokens[0])
		tokens = tokens[1:]
	default:
		validity := tafValidityPattern.FindStringSubmatch(tokens[0])
		if validity == nil {
			return Report{}, false
		}
		report.Rows = append(report.Rows, Row{Label: "Valid", Value: "from " + dayHourText(validity[1], validity[2]) + " to " + dayHourText(validity[3], validity[4])})
		tokens = tokens[1:]
	}

	report.Periods, report.Unknown = decodeTAFBody(tokens, &report.Rows, &report.Flags)

	return report, true
}

func decodeTAFBody(tokens []string, rows *[]Row, flags *[]string) ([]Period, []string) {
	var periods []Period
	var unknown []string

	periods = append(periods, Period{Period: "Base forecast"})
	current := &periods[0].Rows

	open := func(label string) {
		periods = append(periods, Period{Period: label})
		current = &periods[len(periods)-1].Rows
	}

	for i := 0; i < len(tokens); i++ {
		token := tokens[i]

		if m := tafFromPattern.FindStringSubmatch(token); m != nil {
			open("From " + dayHourMinuteText(m[1], m[2], m[3]))
			continue
		}
		if token == "TEMPO" || token == "BECMG" {
			label := trendKeywords[token]
			if i+1 < len(tokens) {
				if v := tafValidityPattern.FindStringSubmatch(tokens[i+1]); v != nil {
					label += " " + dayHourText(v[1], v[2]) + " to " + dayHourText(v[3], v[4])
					i++
				}
			}
			open(strings.ToUpper(label[:1]) + label[1:])
			continue
		}
		if m := tafProbPattern.FindStringSubmatch(token); m != nil {
			label := m[1] + "% probability"
			if i+1 < len(tokens) && tokens[i+1] == "TEMPO" {
				label += ", temporarily"
				i++
			}
			if i+1 < len(tokens) {
				if v := tafValidityPattern.FindStringSubmatch(tokens[i+1]); v != nil {
					label += " " + dayHourText(v[1], v[2]) + " to " + dayHourText(v[3], v[4])
					i++
				}
			}
			open(label)
			continue
		}
		if m := tafExtremePattern.FindStringSubmatch(token); m != nil {
			value, _ := temperatureValue(m[2])
			label := "Maximum temperature"
			if m[1] == "N" {
				label = "Minimum temperature"
			}
			*rows = append(*rows, Row{Label: label, Value: value + " at " + dayHourText(m[3], m[4])})
			continue
		}
		if token == "RMK" {
			var remarks []Row
			remarks, unknown = decodeRemarks(tokens[i+1:], unknown)
			*rows = append(*rows, remarks...)
			break
		}

		switch token {
		case "AMD", "COR", "NIL", "CNL", "AUTO":
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
		}

		if row, consumed, ok := decodeGroup(tokens, i); ok {
			*current = append(*current, row)
			i += consumed
			continue
		}

		unknown = append(unknown, token)
	}

	return periods, unknown
}
