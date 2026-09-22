package avreport

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

var ref = time.Date(2026, time.September, 22, 18, 0, 0, 0, time.UTC)

func rowValue(rows []Row, label string) string {
	for _, row := range rows {
		if row.Label == label {
			return row.Value
		}
	}
	return ""
}

func TestDecodesAKeywordMETAR(t *testing.T) {
	report, err := Decode("METAR KJFK 221651Z 28012G20KT 250V310 10SM -RA BR FEW025 BKN045CB 24/12 A3012 RMK AO2 SLP203 T02440122 RAB35 P0002=", ref)
	if err != nil {
		t.Fatal(err)
	}

	if report.Kind != KindMETAR || report.Station != "KJFK" {
		t.Fatalf("kind %q station %q", report.Kind, report.Station)
	}
	if report.StationName == "" || report.Format != "dd" {
		t.Errorf("the station was not placed: %+v", report)
	}
	if !report.IssuedAt.Equal(time.Date(2026, time.September, 22, 16, 51, 0, 0, time.UTC)) || !report.Inferred {
		t.Errorf("issued = %v, inferred = %v", report.IssuedAt, report.Inferred)
	}
	if strings.HasSuffix(report.Raw, "=") {
		t.Error("the terminator reached the raw report")
	}

	for label, want := range map[string]string{
		"Wind":           "280° at 12 kt, gusting 20 kt",
		"Wind direction": "varying between 250° and 310°",
		"Visibility":     "10 statute miles",
		"Weather":        "light rain",
		"Temperature":    "24°C, dew point 12°C",
		"Altimeter":      "30.12 inHg",
	} {
		if got := rowValue(report.Rows, label); got != want {
			t.Errorf("%s = %q, want %q", label, got, want)
		}
	}

	var sky, weather []string
	for _, row := range report.Rows {
		switch row.Label {
		case "Sky":
			sky = append(sky, row.Value)
		case "Weather":
			weather = append(weather, row.Value)
		}
	}
	if strings.Join(sky, "|") != "few clouds at 2,500 ft|broken clouds at 4,500 ft, cumulonimbus" {
		t.Errorf("sky = %v", sky)
	}
	if strings.Join(weather, "|") != "light rain|mist" {
		t.Errorf("weather = %v", weather)
	}

	for label, want := range map[string]string{
		"Station":              "automated station with a precipitation discriminator",
		"Sea level pressure":   "1020.3 hPa",
		"Precise temperature":  "24.4°C, dew point 12.2°C",
		"Precipitation":        "rain began at :35Z",
		"Hourly precipitation": "0.02 in",
	} {
		if got := rowValue(report.Remarks, label); got != want {
			t.Errorf("remark %s = %q, want %q", label, got, want)
		}
	}
	if len(report.Unknown) != 0 {
		t.Errorf("unknown = %v", report.Unknown)
	}
	if !strings.HasPrefix(report.Summary, "Wind 280° at 12 kt, gusting 20 kt; 10 statute miles; light rain") {
		t.Errorf("summary = %q", report.Summary)
	}
}

func TestDecodesABareMETARAndASPECI(t *testing.T) {
	bare, err := Decode("PHNL 221653Z 07010KT 10SM FEW025 27/19 A3010", ref)
	if err != nil {
		t.Fatal(err)
	}
	if bare.Kind != KindMETAR || bare.Station != "PHNL" || bare.StationName == "" {
		t.Errorf("bare = %+v", bare)
	}

	speci, err := Decode("SPECI EGLL 221720Z 24015KT 9999 -SHRA SCT018 BKN030 17/12 Q1009 NOSIG", ref)
	if err != nil {
		t.Fatal(err)
	}
	if speci.Kind != KindSPECI {
		t.Errorf("kind = %q", speci.Kind)
	}
	for label, want := range map[string]string{
		"Visibility": "10 km or more",
		"Weather":    "light showers of rain",
		"Altimeter":  "1009 hPa",
		"Trend":      "no significant change expected",
	} {
		if got := rowValue(speci.Rows, label); got != want {
			t.Errorf("%s = %q, want %q", label, got, want)
		}
	}
}

func TestABareLineWithoutAWindGroupIsNotAMETAR(t *testing.T) {
	for _, text := range []string{
		"KJFK 221651Z ready for departure",
		"ABCD 221651Z",
		"221651Z 28012KT",
		"KJFK 226151Z 28012KT",
	} {
		if _, err := Decode(text, ref); err == nil {
			t.Errorf("%q decoded as a report", text)
		}
	}
}

func TestUnknownGroupsAreKeptVerbatim(t *testing.T) {
	report, err := Decode("METAR KJFK 221651Z 28012KT 10SM FEW025 24/12 A3012 XYZZY RMK AO2 PLUGH", ref)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(report.Unknown, "|") != "XYZZY|RMK PLUGH" {
		t.Errorf("unknown = %v", report.Unknown)
	}
}

func TestDecodesATAF(t *testing.T) {
	text := "TAF AMD PGUA 221720Z 2218/2324 07012KT P6SM SCT025 TX31/2304Z TN25/2318Z\n  FM230000 09008KT P6SM FEW030\n  TEMPO 2306/2310 4SM -SHRA BKN020\n  PROB30 2312/2316 2SM TSRA OVC015CB"
	report, err := Decode(text, ref)
	if err != nil {
		t.Fatal(err)
	}

	if report.Kind != KindTAF || report.Station != "PGUA" || report.StationName == "" {
		t.Errorf("report = %+v", report)
	}
	if strings.Join(report.Flags, ",") != "AMD" {
		t.Errorf("flags = %v", report.Flags)
	}
	if got := rowValue(report.Rows, "Valid"); got != "from day 22 at 18:00Z to day 23 at 24:00Z" {
		t.Errorf("valid = %q", got)
	}
	if got := rowValue(report.Rows, "Maximum temperature"); got != "31°C at day 23 at 04:00Z" {
		t.Errorf("max = %q", got)
	}

	labels := make([]string, 0, len(report.Periods))
	for _, period := range report.Periods {
		labels = append(labels, period.Period)
	}
	want := []string{
		"Base forecast",
		"From day 23 at 00:00Z",
		"Temporarily day 23 at 06:00Z to day 23 at 10:00Z",
		"30% probability day 23 at 12:00Z to day 23 at 16:00Z",
	}
	if strings.Join(labels, "|") != strings.Join(want, "|") {
		t.Errorf("periods = %v", labels)
	}
	if got := rowValue(report.Periods[0].Rows, "Visibility"); got != "more than 6 statute miles" {
		t.Errorf("base visibility = %q", got)
	}
	if got := rowValue(report.Periods[3].Rows, "Weather"); got != "thunderstorm with rain" {
		t.Errorf("prob weather = %q", got)
	}
	if got := rowValue(report.Periods[3].Rows, "Sky"); got != "overcast at 1,500 ft, cumulonimbus" {
		t.Errorf("prob sky = %q", got)
	}
	if report.Raw != strings.TrimSpace(text) {
		t.Errorf("raw = %q", report.Raw)
	}
}

func TestDecodesANilTAF(t *testing.T) {
	report, err := Decode("TAF KJFK 221720Z NIL", ref)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(report.Flags, ",") != "NIL" {
		t.Errorf("flags = %v", report.Flags)
	}
}

func TestDecodesAnFAANotam(t *testing.T) {
	report, err := Decode("!HNL 09/123 HNL RWY 08L/26R CLSD WEF WIP 2609221200-2609232359EST", ref)
	if err != nil {
		t.Fatal(err)
	}

	if report.Kind != KindNOTAM || report.Station != "PHNL" || report.StationName == "" {
		t.Errorf("report = %+v", report)
	}
	for label, want := range map[string]string{
		"Location":  "HNL",
		"Number":    "09/123",
		"Affects":   "HNL",
		"Subject":   "runway (RWY)",
		"Text":      "08L/26R closed with effect from work in progress",
		"Effective": "22 Sep 2026 12:00Z",
		"Expires":   "23 Sep 2026 23:59Z (estimated)",
	} {
		if got := rowValue(report.Rows, label); got != want {
			t.Errorf("%s = %q, want %q", label, got, want)
		}
	}
	if !report.IssuedAt.Equal(time.Date(2026, time.September, 22, 12, 0, 0, 0, time.UTC)) || report.Inferred {
		t.Errorf("issued = %v, inferred = %v", report.IssuedAt, report.Inferred)
	}

	permanent, err := Decode("!JFK 09/001 JFK TWY A CLSD 2609221200-PERM", ref)
	if err != nil {
		t.Fatal(err)
	}
	if got := rowValue(permanent.Rows, "Expires"); got != "permanent" {
		t.Errorf("expires = %q", got)
	}
	if permanent.Station != "KJFK" {
		t.Errorf("station = %q", permanent.Station)
	}
}

func TestDecodesAnICAONotam(t *testing.T) {
	text := "A1234/26 NOTAMN\nQ) KZNY/QMRLC/IV/NBO/A/000/999/4038N07347W005\nA) KJFK B) 2609221200 C) 2609232359\nE) RWY 04L/22R CLSD DUE WIP"
	report, err := Decode(text, ref)
	if err != nil {
		t.Fatal(err)
	}

	if report.Kind != KindNOTAM || report.Station != "KJFK" {
		t.Errorf("report = %+v", report)
	}
	for label, want := range map[string]string{
		"Number":    "A1234/26",
		"Series":    "new",
		"FIR":       "KZNY",
		"Condition": "runway: closed (QMRLC)",
		"Traffic":   "IFR, VFR",
		"Purpose":   "immediate attention, briefing, flight operations",
		"Scope":     "aerodrome",
		"Levels":    "FL000 to FL999",
		"Radius":    "5 NM",
		"Location":  "KJFK",
		"Effective": "22 Sep 2026 12:00Z",
		"Expires":   "23 Sep 2026 23:59Z",
		"Text":      "runway 04L/22R closed DUE work in progress",
	} {
		if got := rowValue(report.Rows, label); got != want {
			t.Errorf("%s = %q, want %q", label, got, want)
		}
	}
	if report.Center == nil || report.Value != "40.6333,-73.7833" || report.RadiusNm != "5" {
		t.Errorf("center = %v, value = %q, radius = %q", report.Center, report.Value, report.RadiusNm)
	}
}

func TestAnUnknownQCodeIsListedNotGuessed(t *testing.T) {
	text := "A1234/26 NOTAMN\nQ) KZNY/QZZZZ/IV/NBO/A/000/999/4038N07347W005\nA) KJFK B) 2609221200 C) 2609232359\nE) TEST"
	report, err := Decode(text, ref)
	if err != nil {
		t.Fatal(err)
	}
	if rowValue(report.Rows, "Condition") != "" || strings.Join(report.Unknown, ",") != "QZZZZ" {
		t.Errorf("condition = %q, unknown = %v", rowValue(report.Rows, "Condition"), report.Unknown)
	}
}

func TestDecodeRefusesWhatIsNotAReport(t *testing.T) {
	for _, text := range []string{
		"",
		"hello world",
		"TAF",
		"TAF KJFK",
		"TAF KJFK 221720Z",
		"TAF KJFK 221720Z later",
		"!JFK",
		"A1234/26 NOTAMN\nQ) x",
		"METAR KJFK 321651Z 28012KT",
	} {
		if _, err := Decode(text, ref); err == nil {
			t.Errorf("%q decoded", text)
		}
	}

	long := "METAR KJFK 221651Z 28012KT " + strings.Repeat("FEW025 ", 400)
	if _, err := Decode(long, ref); err != ErrTooLong {
		t.Errorf("a long report: err = %v", err)
	}
	lines := "TAF KJFK 221720Z 2218/2324 28012KT P6SM\n" + strings.Repeat("FM230000 09008KT P6SM\n", MaxSourceLines)
	if _, err := Decode(lines, ref); err != ErrTooLong {
		t.Errorf("a tall report: err = %v", err)
	}
}

func TestAMETAROnlyHeaderDecodesWithNoRows(t *testing.T) {
	report, err := Decode("METAR KJFK 221651Z", ref)
	if err != nil {
		t.Fatalf("a headline METAR was refused: %v", err)
	}
	if len(report.Rows) != 0 {
		t.Errorf("rows = %v", report.Rows)
	}
}

func TestPropsCarryTheWholeShapeAndMarshal(t *testing.T) {
	report, err := Decode(Fixture(), ref)
	if err != nil {
		t.Fatal(err)
	}
	blob := Props(report, Source{Kind: SourceFence, Lead: "wx", Trail: "end"})

	for _, key := range []string{"version", "source", "lead", "trail", "src", "kind", "station", "station_name", "issued", "issued_at", "inferred", "summary", "flags", "rows", "periods", "remarks", "unknown", "format", "value", "region", "radius_nm"} {
		if _, ok := blob[key]; !ok {
			t.Errorf("props lack %q", key)
		}
	}
	if blob["region"] == "" {
		t.Error("the station's region is empty for a Honolulu report")
	}
	if _, err := json.Marshal(blob); err != nil {
		t.Fatal(err)
	}

	lower := PropsWithoutRows(report, Source{Kind: SourceMessage})
	if lower["rows_dropped"] != "1" || len(lower["rows"].([]any)) != 0 || lower["src"] == "" {
		t.Errorf("the lower rung = %v", lower)
	}
}

func TestTheStationIsPlacedWhenTheBuildHoldsIt(t *testing.T) {
	report, err := Decode("METAR QZQZ 221651Z 28012KT 10SM FEW025 24/12 A3012", ref)
	if err != nil {
		t.Fatal(err)
	}
	if report.StationName != "" || report.Format != "" {
		t.Errorf("an unknown station was placed: %+v", report)
	}
}

func TestExpandContractionsKeepsWhatItDoesNotKnow(t *testing.T) {
	if got := expandContractions("RWY 09/27 CLSD, TWY B U/S. FROBNICATE"); got != "runway 09/27 closed, taxiway B unserviceable. FROBNICATE" {
		t.Errorf("got %q", got)
	}
}

func TestVisibilityFractions(t *testing.T) {
	for text, want := range map[string]string{
		"METAR KJFK 221651Z 28012KT 1 1/2SM BR 24/12 A3012": "1 1/2 statute miles",
		"METAR KJFK 221651Z 28012KT M1/4SM FG 24/12 A3012":  "less than 1/4 statute miles",
		"METAR KJFK 221651Z 28012KT 3/4SM BR 24/12 A3012":   "3/4 statute miles",
		"METAR KJFK 221651Z 28012KT 1SM BR 24/12 A3012":     "1 statute mile",
		"METAR KJFK 221651Z 28012KT 4000 BR 24/12 Q1012":    "4,000 m",
		"METAR KJFK 221651Z 28012KT CAVOK 24/12 Q1012":      "10 km or more, no cloud below 5,000 ft, no significant weather",
	} {
		report, err := Decode(text, ref)
		if err != nil {
			t.Fatalf("%s: %v", text, err)
		}
		if got := rowValue(report.Rows, "Visibility"); got != want {
			t.Errorf("%s: visibility = %q, want %q", text, got, want)
		}
	}
}

func TestWindSpellings(t *testing.T) {
	for text, want := range map[string]string{
		"METAR KJFK 221651Z 00000KT 10SM 24/12 A3012":    "calm",
		"METAR KJFK 221651Z VRB03KT 10SM 24/12 A3012":    "variable at 3 kt",
		"METAR KJFK 221651Z 28012MPS 10SM 24/12 A3012":   "280° at 12 m/s",
		"METAR KJFK 221651Z /////KT 10SM 24/12 A3012":    "not reported",
		"METAR KJFK 221651Z 28012G30KT 10SM 24/12 A3012": "280° at 12 kt, gusting 30 kt",
	} {
		report, err := Decode(text, ref)
		if err != nil {
			t.Fatalf("%s: %v", text, err)
		}
		if got := rowValue(report.Rows, "Wind"); got != want {
			t.Errorf("%s: wind = %q, want %q", text, got, want)
		}
	}
}

func TestEveryKindIsNamed(t *testing.T) {
	seen := map[string]bool{}
	for _, text := range []string{
		Fixture(),
		"SPECI KJFK 221651Z 28012KT 10SM 24/12 A3012",
		"TAF KJFK 221720Z 2218/2324 28012KT P6SM SKC",
		"!JFK 09/001 JFK TWY A CLSD 2609221200-PERM",
	} {
		report, err := Decode(text, ref)
		if err != nil {
			t.Fatal(err)
		}
		seen[report.Kind] = true
	}
	for _, kind := range Kinds {
		if !seen[kind] {
			t.Errorf("no fixture produced %s", kind)
		}
	}
}
