package avreport

import (
	_ "embed"
	"encoding/csv"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/decorators/airport"
)

//go:embed data/contractions.csv
var contractionsCSV string

//go:embed data/qcodes.csv
var qcodesCSV string

var contractions = mustParseContractions(contractionsCSV)

var qSubjects, qConditions = mustParseQCodes(qcodesCSV)

func mustParseContractions(source string) map[string]string {
	rows, err := csv.NewReader(strings.NewReader(source)).ReadAll()
	if err != nil {
		panic("avreport: " + err.Error())
	}
	out := make(map[string]string, len(rows))
	for _, row := range rows[1:] {
		if len(row) != 2 || row[0] == "" || row[1] == "" {
			panic("avreport: a contraction row is malformed: " + strings.Join(row, ","))
		}
		if _, dup := out[row[0]]; dup {
			panic("avreport: contraction " + row[0] + " is listed twice")
		}
		out[row[0]] = row[1]
	}
	return out
}

func mustParseQCodes(source string) (map[string]string, map[string]string) {
	rows, err := csv.NewReader(strings.NewReader(source)).ReadAll()
	if err != nil {
		panic("avreport: " + err.Error())
	}
	subjects := map[string]string{}
	conditions := map[string]string{}
	for _, row := range rows[1:] {
		if len(row) != 3 || len(row[1]) != 3 || row[2] == "" {
			panic("avreport: a Q-code row is malformed: " + strings.Join(row, ","))
		}
		var table map[string]string
		switch row[0] {
		case "subject":
			table = subjects
		case "condition":
			table = conditions
		default:
			panic("avreport: a Q-code row has role " + row[0])
		}
		if _, dup := table[row[1]]; dup {
			panic("avreport: Q-code " + row[1] + " is listed twice as a " + row[0])
		}
		table[row[1]] = row[2]
	}
	return subjects, conditions
}

var faaNotamPattern = regexp.MustCompile(`^!([A-Z]{3})[ \t]+(\d{1,2}/\d{3,4})[ \t]+([A-Z]{3,4})[ \t]+([A-Z]{2,8}|\([OU]\))[ \t]+(.*)$`)

var faaEffectivePattern = regexp.MustCompile(`(\d{10})-(\d{10}|PERM)(EST)?$`)

var icaoNotamHeaderPattern = regexp.MustCompile(`^([A-Z]\d{4}/\d{2})[ \t]+(NOTAMN|NOTAMR|NOTAMC)(?:[ \t]+([A-Z]\d{4}/\d{2}))?$`)

var icaoQLinePattern = regexp.MustCompile(`^([A-Z]{4})/Q([A-Z]{2})([A-Z]{2})/([IVK]+)/([NBOMK]+)/([AEWK]+)/(\d{3})/(\d{3})/(\d{2})(\d{2})([NS])(\d{3})(\d{2})([EW])(\d{3})$`)

var notamKeywords = map[string]string{
	"RWY": "runway", "TWY": "taxiway", "APRON": "apron", "AD": "aerodrome", "OBST": "obstruction",
	"NAV": "navigation aid", "COM": "communications", "SVC": "services", "AIRSPACE": "airspace",
	"ODP": "obstacle departure procedure", "SID": "standard instrument departure",
	"STAR": "standard terminal arrival", "CHART": "chart", "DATA": "data", "IAP": "instrument approach procedure",
	"VFP": "visual flight procedure", "ROUTE": "route", "SPECIAL": "special", "SECURITY": "security",
	"(O)": "other", "(U)": "unverified",
}

var icaoTraffic = map[byte]string{'I': "IFR", 'V': "VFR", 'K': "checklist"}

var icaoPurpose = map[byte]string{'N': "immediate attention", 'B': "briefing", 'O': "flight operations", 'M': "miscellaneous", 'K': "checklist"}

var icaoScope = map[byte]string{'A': "aerodrome", 'E': "en route", 'W': "navigation warning", 'K': "checklist"}

var faaPrefixes = []string{"K", "PH", "PA", "PG", "TJ", "PJ", "PM", "PW"}

func decodeFAANotam(line string, ref time.Time) (Report, bool) {
	m := faaNotamPattern.FindStringSubmatch(strings.TrimSpace(line))
	if m == nil {
		return Report{}, false
	}
	location, number, affected, keyword, body := m[1], m[2], m[3], m[4], m[5]

	report := Report{Kind: KindNOTAM, Station: faaStation(affected)}
	report.Rows = append(report.Rows,
		Row{Label: "Issued by", Value: issuerText(location)},
		Row{Label: "Number", Value: number},
	)
	if affected != noSingleLocation {
		report.Rows = append(report.Rows, Row{Label: "Affects", Value: affected})
	}
	report.Rows = append(report.Rows, Row{Label: "Subject", Value: subjectText(keyword)})

	text := body
	if e := faaEffectivePattern.FindStringSubmatch(body); e != nil {
		text = strings.TrimSpace(body[:len(body)-len(e[0])])
		start, ok := fullDate(e[1])
		if ok {
			report.IssuedAt = start
			report.Rows = append(report.Rows, Row{Label: "Effective", Value: zuluText(start), At: start})
		} else {
			report.Unknown = append(report.Unknown, e[1])
		}
		switch e[2] {
		case "PERM":
			report.Rows = append(report.Rows, Row{Label: "Expires", Value: "permanent"})
		default:
			end, ok := fullDate(e[2])
			if ok {
				value := zuluText(end)
				if e[3] != "" {
					value += " (estimated)"
				}
				report.Rows = append(report.Rows, Row{Label: "Expires", Value: value, At: end})
			} else {
				report.Unknown = append(report.Unknown, e[2])
			}
		}
	}
	if report.IssuedAt.IsZero() && !hasRow(report, "Expires") {
		readTFREffective(&report, body)
	}
	readTFR(&report, body)
	if text != "" {
		report.Rows = append(report.Rows, Row{Label: "Text", Value: expandContractions(text)})
	}
	report.Summary = summarizeNotam(report)

	return report, true
}

const noSingleLocation = "ZZZ"

var faaIssuers = map[string]string{
	"FDC": "FDC (FAA Flight Data Center)",
}

func issuerText(location string) string {
	if name, ok := faaIssuers[location]; ok {
		return name
	}
	return location
}

func subjectText(keyword string) string {
	subject, ok := notamKeywords[keyword]
	if !ok || strings.EqualFold(subject, keyword) {
		return keyword
	}
	return subject + " (" + keyword + ")"
}

func hasRow(report Report, label string) bool {
	for _, row := range report.Rows {
		if row.Label == label {
			return true
		}
	}
	return false
}

func faaStation(affected string) string {
	if len(affected) == 4 {
		return affected
	}
	if ident, ok := stationByIATA(affected); ok {
		return ident
	}
	for _, prefix := range faaPrefixes {
		candidate := prefix + affected
		if len(candidate) == 4 && stationKnown(candidate) {
			return candidate
		}
	}
	return ""
}

func decodeICAONotam(text string, ref time.Time) (Report, bool) {
	lines := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
	var header []string
	for _, line := range lines {
		if strings.TrimSpace(line) != "" {
			header = strings.Fields(line)
			break
		}
	}
	if len(header) == 0 {
		return Report{}, false
	}
	m := icaoNotamHeaderPattern.FindStringSubmatch(strings.Join(header, " "))
	if m == nil {
		return Report{}, false
	}

	report := Report{Kind: KindNOTAM}
	report.Rows = append(report.Rows, Row{Label: "Number", Value: m[1]})
	switch m[2] {
	case "NOTAMN":
		report.Rows = append(report.Rows, Row{Label: "Series", Value: "new"})
	case "NOTAMR":
		report.Rows = append(report.Rows, Row{Label: "Series", Value: "replaces " + m[3]})
	case "NOTAMC":
		report.Rows = append(report.Rows, Row{Label: "Series", Value: "cancels " + m[3]})
	}

	fields := icaoFields(strings.Join(lines, "\n"))
	if q, ok := fields["Q"]; ok {
		decodeQLine(&report, q)
	}
	if a, ok := fields["A"]; ok && a != "" {
		if codes := strings.Fields(a); airport.MatchesIdentShape(codes[0]) {
			report.Station = codes[0]
		}
		report.Rows = append(report.Rows, Row{Label: "Location", Value: a})
	}
	if b, ok := fields["B"]; ok {
		if start, valid := fullDate(strings.TrimSpace(b)); valid {
			report.IssuedAt = start
			report.Rows = append(report.Rows, Row{Label: "Effective", Value: zuluText(start), At: start})
		} else {
			report.Unknown = append(report.Unknown, "B) "+b)
		}
	}
	if c, ok := fields["C"]; ok {
		value := strings.TrimSpace(c)
		switch value {
		case "PERM":
			report.Rows = append(report.Rows, Row{Label: "Expires", Value: "permanent"})
		default:
			raw := strings.TrimSuffix(strings.TrimSuffix(value, "EST"), " ")
			if end, valid := fullDate(strings.TrimSpace(raw)); valid {
				text := zuluText(end)
				if strings.HasSuffix(value, "EST") {
					text += " (estimated)"
				}
				report.Rows = append(report.Rows, Row{Label: "Expires", Value: text, At: end})
			} else {
				report.Unknown = append(report.Unknown, "C) "+c)
			}
		}
	}
	if d, ok := fields["D"]; ok {
		report.Rows = append(report.Rows, Row{Label: "Schedule", Value: expandContractions(d)})
	}
	if e, ok := fields["E"]; ok {
		report.Rows = append(report.Rows, Row{Label: "Text", Value: expandContractions(e)})
	}
	if f, ok := fields["F"]; ok {
		report.Rows = append(report.Rows, Row{Label: "Lower limit", Value: f})
	}
	if g, ok := fields["G"]; ok {
		report.Rows = append(report.Rows, Row{Label: "Upper limit", Value: g})
	}
	if _, ok := fields["E"]; !ok {
		return Report{}, false
	}

	report.Summary = summarizeNotam(report)
	return report, true
}

var icaoFieldPattern = regexp.MustCompile(`(?m)(?:^|\s)([A-GQ])\)\s*`)

func icaoFields(text string) map[string]string {
	fields := map[string]string{}
	matches := icaoFieldPattern.FindAllStringSubmatchIndex(text, -1)
	for i, m := range matches {
		key := text[m[2]:m[3]]
		start := m[1]
		end := len(text)
		if i+1 < len(matches) {
			end = matches[i+1][0]
		}
		value := strings.TrimSpace(strings.Join(strings.Fields(text[start:end]), " "))
		if _, seen := fields[key]; !seen {
			fields[key] = value
		}
	}
	return fields
}

func decodeQLine(report *Report, raw string) {
	m := icaoQLinePattern.FindStringSubmatch(strings.TrimSpace(raw))
	if m == nil {
		report.Unknown = append(report.Unknown, "Q) "+raw)
		return
	}

	report.Rows = append(report.Rows, Row{Label: "FIR", Value: m[1]})
	subject, subjectKnown := qSubjects["Q"+m[2]]
	condition, conditionKnown := qConditions["Q"+m[3]]
	code := "Q" + m[2] + m[3]
	switch {
	case subjectKnown && conditionKnown:
		report.Rows = append(report.Rows, Row{Label: "Condition", Value: subject + ": " + condition + " (" + code + ")"})
	default:
		report.Unknown = append(report.Unknown, code)
	}

	report.Rows = append(report.Rows,
		Row{Label: "Traffic", Value: expandLetters(m[4], icaoTraffic)},
		Row{Label: "Purpose", Value: expandLetters(m[5], icaoPurpose)},
		Row{Label: "Scope", Value: expandLetters(m[6], icaoScope)},
		Row{Label: "Levels", Value: "FL" + m[7] + " to FL" + m[8]},
	)

	lat := degreesMinutes(m[9], m[10], m[11] == "S")
	lon := degreesMinutes(m[12], m[13], m[14] == "W")
	report.Center = &Center{Lat: lat, Lon: lon}
	radius, _ := strconv.Atoi(m[15])
	report.RadiusNm = strconv.Itoa(radius)
	report.Rows = append(report.Rows, Row{Label: "Radius", Value: strconv.Itoa(radius) + " NM"})
}

func expandLetters(raw string, table map[byte]string) string {
	parts := make([]string, 0, len(raw))
	for i := 0; i < len(raw); i++ {
		if text, ok := table[raw[i]]; ok {
			parts = append(parts, text)
		}
	}
	return strings.Join(parts, ", ")
}

func degreesMinutes(degrees, minutes string, negative bool) float64 {
	d, _ := strconv.Atoi(degrees)
	m, _ := strconv.Atoi(minutes)
	value := float64(d) + float64(m)/60
	if negative {
		return -value
	}
	return value
}

func expandContractions(text string) string {
	words := strings.Fields(text)
	for i, word := range words {
		core := strings.TrimRightFunc(word, func(r rune) bool { return r == '.' || r == ',' || r == ';' || r == ':' })
		if core == "" {
			continue
		}
		if expansion, ok := contractions[core]; ok {
			words[i] = expansion + word[len(core):]
		}
	}
	return strings.Join(words, " ")
}

func summarizeNotam(report Report) string {
	var parts []string
	for _, label := range []string{RestrictionLabel, "Condition", "Subject", "Text"} {
		for _, row := range report.Rows {
			if row.Label == label {
				parts = append(parts, row.Value)
				break
			}
		}
	}
	if len(parts) == 0 {
		return ""
	}
	return sanitizeText(strings.Join(parts, "; "), summaryMaxRunes)
}
