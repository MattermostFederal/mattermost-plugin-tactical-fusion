package avreport

import (
	"reflect"
	"testing"
)

func rowsLabeled(rows []Row, label string) []string {
	var values []string
	for _, row := range rows {
		if row.Label == label {
			values = append(values, row.Value)
		}
	}
	return values
}

func mustDecode(t *testing.T, line string) Report {
	t.Helper()
	report, err := Decode(line, ref)
	if err != nil {
		t.Fatalf("Decode(%q): %v", line, err)
	}
	return report
}

func TestCAVOKAndNOSIGAreSpelledOut(t *testing.T) {
	report := mustDecode(t, "METAR EGLL 221650Z AUTO 24010KT CAVOK 18/09 Q1015 NOSIG")

	if !reflect.DeepEqual(report.Flags, []string{"AUTO"}) {
		t.Errorf("flags = %q, want [AUTO]", report.Flags)
	}
	if got := rowValue(report.Rows, "Visibility"); got != "10 km or more, no cloud below 5,000 ft, no significant weather" {
		t.Errorf("visibility = %q", got)
	}
	if got := rowValue(report.Rows, "Trend"); got != "no significant change expected" {
		t.Errorf("trend = %q", got)
	}
	if len(report.Periods) != 0 {
		t.Errorf("NOSIG opened a period: %+v", report.Periods)
	}
}

func TestANilMETARCarriesOnlyTheFlag(t *testing.T) {
	report := mustDecode(t, "METAR KJFK 221651Z NIL")

	if !reflect.DeepEqual(report.Flags, []string{"NIL"}) {
		t.Errorf("flags = %q, want [NIL]", report.Flags)
	}
	if len(report.Rows) != 0 || report.Summary != "" {
		t.Errorf("rows = %+v, summary = %q", report.Rows, report.Summary)
	}
}

func TestTrendGroupsOpenTheirOwnPeriods(t *testing.T) {
	report := mustDecode(t, "METAR EGLL 221650Z 24010KT 9999 TEMPO 4000 BECMG 9999 NSW")

	if len(report.Periods) != 2 {
		t.Fatalf("periods = %+v, want two", report.Periods)
	}
	for i, want := range []struct{ period, visibility string }{
		{"TEMPO (temporarily)", "4,000 m"},
		{"BECMG (becoming)", "10 km or more"},
	} {
		period := report.Periods[i]
		if period.Period != want.period {
			t.Errorf("period %d = %q, want %q", i, period.Period, want.period)
		}
		if got := rowValue(period.Rows, "Visibility"); got != want.visibility {
			t.Errorf("period %d visibility = %q, want %q", i, got, want.visibility)
		}
	}
	if got := rowValue(report.Periods[1].Rows, "Weather"); got != "no significant weather" {
		t.Errorf("NSW landed outside the BECMG period: %q", got)
	}
	if got := rowValue(report.Rows, "Visibility"); got != "10 km or more" {
		t.Errorf("the trend groups reached the body: %q", got)
	}
}

func TestWindShearSpellings(t *testing.T) {
	report := mustDecode(t, "METAR KXYZ 221650Z 24010KT WS020/27045KT WS ALL RWY WS R27L")

	want := []string{"at 2,000 ft, wind 270° at 45 kt", "all runways", "runway 27L"}
	if got := rowsLabeled(report.Rows, "Wind shear"); !reflect.DeepEqual(got, want) {
		t.Errorf("wind shear = %q, want %q", got, want)
	}
	if len(report.Unknown) != 0 {
		t.Errorf("the runway words leaked into unknown: %q", report.Unknown)
	}
}

func TestAWindShearWordWithoutARunwayIsNotDecoded(t *testing.T) {
	report := mustDecode(t, "METAR KXYZ 221650Z 24010KT WS")

	if got := rowsLabeled(report.Rows, "Wind shear"); len(got) != 0 {
		t.Errorf("wind shear = %q, want none", got)
	}
	if !reflect.DeepEqual(report.Unknown, []string{"WS"}) {
		t.Errorf("unknown = %q, want [WS]", report.Unknown)
	}
}

func TestVerticalVisibilitySpellings(t *testing.T) {
	report := mustDecode(t, "METAR KXYZ 221650Z 24010KT VV/// VV004")

	want := []string{"sky obscured, vertical visibility unknown", "sky obscured, vertical visibility 400 ft"}
	if got := rowsLabeled(report.Rows, "Sky"); !reflect.DeepEqual(got, want) {
		t.Errorf("sky = %q, want %q", got, want)
	}
}

func TestAMissingTemperatureOrDewPointIsSaidSo(t *testing.T) {
	tests := []struct {
		group string
		want  string
	}{
		{"10/M02", "10°C, dew point -2°C"},
		{"M05/", "-5°C, dew point not reported"},
		{"24///", "24°C, dew point not reported"},
		{"///10", "not reported, dew point 10°C"},
	}
	for _, tt := range tests {
		t.Run(tt.group, func(t *testing.T) {
			report := mustDecode(t, "METAR KXYZ 221650Z 24010KT "+tt.group)
			if got := rowValue(report.Rows, "Temperature"); got != tt.want {
				t.Errorf("temperature = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestATemperatureGroupWithNeitherValueIsNotDecoded(t *testing.T) {
	report := mustDecode(t, "METAR KXYZ 221650Z 24010KT /////")

	if got := rowValue(report.Rows, "Temperature"); got != "" {
		t.Errorf("temperature = %q, want none", got)
	}
	if !reflect.DeepEqual(report.Unknown, []string{"/////"}) {
		t.Errorf("unknown = %q, want [/////]", report.Unknown)
	}
}

func TestRemarkSpellings(t *testing.T) {
	report := mustDecode(t, "METAR KJFK 221651Z 28012KT RMK AO2 PRESRR PK WND 28045/1715 PK WND 28045/15 RAB1520E35 SNE05 P0012 SLP203 T0244")

	want := []Row{
		{Label: "Station", Value: "automated station with a precipitation discriminator"},
		{Label: "Station", Value: "pressure rising rapidly"},
		{Label: "Peak wind", Value: "280° at 45 kt at 17:15Z"},
		{Label: "Peak wind", Value: "280° at 45 kt at :15Z"},
		{Label: "Precipitation", Value: "rain began at 15:20Z, ended at :35Z"},
		{Label: "Precipitation", Value: "snow ended at :05Z"},
		{Label: "Hourly precipitation", Value: "0.12 in"},
		{Label: "Sea level pressure", Value: "1020.3 hPa"},
		{Label: "Precise temperature", Value: "24.4°C"},
	}
	if !reflect.DeepEqual(report.Remarks, want) {
		t.Errorf("remarks =\n%+v\nwant\n%+v", report.Remarks, want)
	}
	if len(report.Unknown) != 0 {
		t.Errorf("unknown = %q, want none", report.Unknown)
	}
}

func TestAPeakWindThatDoesNotParseIsKeptVerbatim(t *testing.T) {
	report := mustDecode(t, "METAR KJFK 221651Z 28012KT RMK PK WND XX")

	if len(report.Remarks) != 0 {
		t.Errorf("remarks = %+v, want none", report.Remarks)
	}
	want := []string{"RMK PK", "RMK WND", "RMK XX"}
	if !reflect.DeepEqual(report.Unknown, want) {
		t.Errorf("unknown = %q, want %q", report.Unknown, want)
	}
}

func TestATrailingPeakWindWithoutAValueIsKeptVerbatim(t *testing.T) {
	report := mustDecode(t, "METAR KJFK 221651Z 28012KT RMK PK WND")

	want := []string{"RMK PK", "RMK WND"}
	if !reflect.DeepEqual(report.Unknown, want) {
		t.Errorf("unknown = %q, want %q", report.Unknown, want)
	}
}

func TestMustAtoiReadsZeroForWhatIsNotANumber(t *testing.T) {
	for in, want := range map[string]int{"020": 20, "": 0, "x": 0} {
		if got := mustAtoi(in); got != want {
			t.Errorf("mustAtoi(%q) = %d, want %d", in, got, want)
		}
	}
}
