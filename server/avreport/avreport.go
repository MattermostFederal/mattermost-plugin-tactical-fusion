package avreport

import (
	"errors"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/decorators"
	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/decorators/airport"
	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/decorators/location"
)

const (
	Type = "avreport"

	PostType     = decorators.PostTypePrefix + "tf_avreport"
	PropsKey     = "tactical_fusion_avreport"
	PropsVersion = 1

	SourceMessage = "message"
	SourceFence   = "fence"

	MaxSourceRunes = 2048
	MaxSourceLines = 64
	MaxRows        = 64
	MaxPeriods     = 24
	MaxUnknown     = 64
	MaxFlags       = 8

	maxNoteRunes = 65536
)

type Row struct {
	Label string
	Value string
	At    time.Time
}

type Period struct {
	Period string
	Rows   []Row
}

type Center struct {
	Lat float64
	Lon float64
}

type Report struct {
	Kind        string
	Station     string
	StationName string
	IssuedAt    time.Time
	Inferred    bool
	Flags       []string
	Rows        []Row
	Periods     []Period
	Remarks     []Row
	Unknown     []string
	Center      *Center
	RadiusNm    string
	Raw         string

	Format string
	Value  string
}

type Source struct {
	Kind  string
	Lead  string
	Trail string
	Text  string
}

var ErrTooLong = errors.New("avreport: the report is too long")

var ErrNotAReport = errors.New("avreport: not a report this build reads")

var stationKnown = func(ident string) bool {
	_, ok := airport.Lookup(ident)
	return ok
}

var stationByIATA = func(code string) (string, bool) {
	a, ok := airport.LookupIATA(code)
	if !ok {
		return "", false
	}
	return a.Ident, true
}

func Decode(text string, ref time.Time) (Report, error) {
	normalized := strings.TrimRight(strings.TrimSpace(strings.ReplaceAll(text, "\r\n", "\n")), "= \t")
	if utf8.RuneCountInString(normalized) > MaxSourceRunes || strings.Count(normalized, "\n") >= MaxSourceLines {
		return Report{}, ErrTooLong
	}
	if normalized == "" {
		return Report{}, ErrNotAReport
	}

	if carriesASecondReport(normalized) {
		return Report{}, ErrNotAReport
	}

	report, ok := decodeAny(normalized, ref)
	if !ok {
		return Report{}, ErrNotAReport
	}

	report.Raw = normalized
	report.Rows = capRows(report.Rows, MaxRows)
	report.Remarks = capRows(report.Remarks, MaxRows)
	if len(report.Periods) > MaxPeriods {
		report.Periods = report.Periods[:MaxPeriods]
	}
	if len(report.Unknown) > MaxUnknown {
		report.Unknown = report.Unknown[:MaxUnknown]
	}
	if len(report.Flags) > MaxFlags {
		report.Flags = report.Flags[:MaxFlags]
	}
	placeStation(&report)

	return report, nil
}

func decodeAny(text string, ref time.Time) (Report, bool) {
	first := strings.TrimSpace(strings.SplitN(text, "\n", 2)[0])

	switch {
	case strings.HasPrefix(first, "TAF"):
		return decodeTAF(text, ref)
	case strings.HasPrefix(first, "!"):
		return decodeFAANotam(strings.Join(strings.Fields(text), " "), ref)
	case icaoNotamHeaderPattern.MatchString(first):
		return decodeICAONotam(text, ref)
	default:
		return decodeMETAR(strings.Join(strings.Fields(text), " "), ref)
	}
}

func carriesASecondReport(text string) bool {
	lines := strings.Split(text, "\n")
	return slices.ContainsFunc(lines[1:], LooksLikeHeader)
}

func LooksLikeHeader(line string) bool {
	first := strings.TrimSpace(line)
	if first == "" {
		return false
	}
	switch {
	case strings.HasPrefix(first, "METAR "), strings.HasPrefix(first, "SPECI "), strings.HasPrefix(first, "TAF "), strings.HasPrefix(first, "!"):
		return true
	case icaoNotamHeaderPattern.MatchString(first):
		return true
	}
	m := metarHeaderPattern.FindStringSubmatch(first)
	return m != nil && bareWindPattern.MatchString(m[7])
}

func capRows(rows []Row, limit int) []Row {
	if len(rows) > limit {
		return rows[:limit]
	}
	return rows
}

func placeStation(report *Report) {
	if report.Center != nil {
		if token, ok := airport.DDToken(report.Center.Lat, report.Center.Lon); ok {
			report.Format = string(location.FormatDD)
			report.Value = token
		}
	}

	if report.Station == "" {
		return
	}
	a, ok := airport.Lookup(report.Station)
	if !ok {
		return
	}
	report.StationName = a.Name
	if report.Format != "" {
		return
	}
	if token, ok := airport.DDToken(a.Lat, a.Lon); ok {
		report.Format = string(location.FormatDD)
		report.Value = token
	}
}

func (r Report) Instant() int64 {
	if r.IssuedAt.IsZero() {
		return 0
	}
	return r.IssuedAt.UnixMilli()
}

func Props(report Report, src Source) map[string]any {
	return props(report, src, true)
}

func PropsWithoutRows(report Report, src Source) map[string]any {
	return props(report, src, false)
}

func props(report Report, src Source, withRows bool) map[string]any {
	blob := Blob(report)
	blob["version"] = PropsVersion
	blob["source"] = src.Kind
	blob["lead"] = sanitizeText(src.Lead, maxNoteRunes)
	blob["trail"] = sanitizeText(src.Trail, maxNoteRunes)

	if !withRows {
		blob["rows"] = []any{}
		blob["periods"] = []any{}
		blob["remarks"] = []any{}
		blob["unknown"] = []any{}
		blob["rows_dropped"] = "1"
	}

	return blob
}

func Blob(report Report) map[string]any {
	region := ""
	if report.Format != "" {
		if conversion, ok := location.Convert(location.FormatDD, report.Value, ""); ok {
			region = conversion.Region
		}
	}

	return map[string]any{
		"kind":         report.Kind,
		"station":      report.Station,
		"station_name": report.StationName,
		"issued":       zuluText(report.IssuedAt),
		"issued_at":    strconv.FormatInt(report.Instant(), 10),
		"inferred":     report.Inferred,
		"flags":        stringsAny(report.Flags),
		"rows":         rowsAny(report.Rows),
		"periods":      periodsAny(report.Periods),
		"remarks":      rowsAny(report.Remarks),
		"unknown":      stringsAny(report.Unknown),
		"format":       report.Format,
		"value":        report.Value,
		"region":       region,
		"radius_nm":    report.RadiusNm,
		"src":          report.Raw,
	}
}

func rowsAny(rows []Row) []any {
	out := make([]any, 0, len(rows))
	for _, row := range rows {
		out = append(out, map[string]any{"label": row.Label, "value": row.Value})
	}
	return out
}

func periodsAny(periods []Period) []any {
	out := make([]any, 0, len(periods))
	for _, period := range periods {
		out = append(out, map[string]any{"period": period.Period, "rows": rowsAny(period.Rows)})
	}
	return out
}

func stringsAny(values []string) []any {
	out := make([]any, 0, len(values))
	for _, value := range values {
		out = append(out, value)
	}
	return out
}

func sanitizeText(raw string, maxRunes int) string {
	cleaned := strings.ToValidUTF8(raw, "�")
	if utf8.RuneCountInString(cleaned) <= maxRunes {
		return cleaned
	}
	runes := []rune(cleaned)
	return string(runes[:maxRunes]) + "…"
}

func Fixture() string {
	return "METAR PHNL 221651Z 07012G18KT 10SM FEW025 SCT045 27/19 A3010 RMK AO2 SLP193 T02720194"
}
