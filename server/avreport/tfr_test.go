package avreport

import (
	"strings"
	"testing"
	"time"
)

func TestATFRWithAOneDigitYearDecodesItsRowsAndItsCircle(t *testing.T) {
	report, err := Decode(tfrExample, ref)
	if err != nil {
		t.Fatalf("the TFR was refused: %v", err)
	}

	for label, want := range map[string]string{
		RestrictionLabel: "temporary flight restriction (14 CFR 91.137)",
		"Number":         "6/1234",
		"Effective":      "22 Sep 2026 18:00Z",
		"Expires":        "23 Sep 2026 02:00Z",
		"Altitudes":      "surface to 5,000 ft MSL",
		"Radius":         "5 NM",
	} {
		if got := rowValue(report.Rows, label); got != want {
			t.Errorf("%s = %q, want %q", label, got, want)
		}
	}
	if want := time.Date(2026, time.September, 22, 18, 0, 0, 0, time.UTC); !report.IssuedAt.Equal(want) {
		t.Errorf("IssuedAt = %v, want %v", report.IssuedAt, want)
	}
	if report.Center == nil || report.RadiusNm != "5" || report.Value != "43.6167,-116.2000" {
		t.Errorf("the circle was not placed: center %v radius %q value %q", report.Center, report.RadiusNm, report.Value)
	}
	if len(report.Area) != 0 {
		t.Errorf("a circle carried an area: %v", report.Area)
	}
	if report.Station != "" {
		t.Errorf("an FDC NOTAM named station %q", report.Station)
	}
}

func TestATFRPolygonCarriesItsRingAndIsPlacedAtItsCentroid(t *testing.T) {
	report, err := Decode(tfrPolygon, ref)
	if err != nil {
		t.Fatal(err)
	}

	if len(report.Area) != 4 {
		t.Fatalf("area = %v, want four vertices", report.Area)
	}
	if report.Area[0].Lat < 43.61 || report.Area[0].Lat > 43.62 || report.Area[0].Lon != -116.2 {
		t.Errorf("the first vertex is %v", report.Area[0])
	}
	if report.Center == nil || report.Value == "" || report.RadiusNm != "" {
		t.Errorf("the polygon was not placed at a centroid: %v %q %q", report.Center, report.Value, report.RadiusNm)
	}
	for label, want := range map[string]string{
		RestrictionLabel: "temporary flight restriction (14 CFR 99.7)",
		"Effective":      "immediately",
		"Expires":        "until further notice",
		"Altitudes":      "surface to FL180",
		"Area":           "4 points",
	} {
		if got := rowValue(report.Rows, label); got != want {
			t.Errorf("%s = %q, want %q", label, got, want)
		}
	}

	tokens, _ := Blob(report)["area"].([]any)
	if len(tokens) != 4 || tokens[0] != "43.6167,-116.2000" {
		t.Errorf("the blob carries area %v", tokens)
	}
}

func TestATFRCoordinateThatWillNotParseDrawsNothing(t *testing.T) {
	text := strings.Replace(tfrExample, "433700N1161200W", "999900N1161200W", 1)
	report, err := Decode(text, ref)
	if err != nil {
		t.Fatal(err)
	}
	if report.Center != nil || report.RadiusNm != "" || report.Value != "" {
		t.Errorf("a malformed coordinate was placed: %v %q %q", report.Center, report.RadiusNm, report.Value)
	}
	if len(report.Unknown) == 0 || report.Unknown[0] != "999900N1161200W" {
		t.Errorf("the malformed coordinate is not listed: %v", report.Unknown)
	}
}

func TestATFRRingPastTheCapIsNotDrawn(t *testing.T) {
	vertices := make([]string, 0, MaxAreaPoints+1)
	for i := range MaxAreaPoints + 1 {
		vertices = append(vertices, "43"+twoDigits(i%60)+"00N1161200W")
	}
	text := "!FDC 6/9999 ZZZ AIRSPACE TEMPORARY FLIGHT RESTRICTIONS WI AN AREA DEFINED AS " +
		strings.Join(vertices, " TO ") + " TO POINT OF ORIGIN SFC-5000FT MSL"
	report, err := Decode(text, ref)
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Area) != 0 {
		t.Errorf("a ring of %d vertices was carried", len(report.Area))
	}
}

func TestATwoDigitYearFAANotamStillDecodesAsBefore(t *testing.T) {
	report, err := Decode("!HNL 09/123 HNL RWY 08L/26R CLSD 2609221200-2609232359", ref)
	if err != nil {
		t.Fatal(err)
	}
	if hasRow(report, RestrictionLabel) {
		t.Error("a runway closure was read as a TFR")
	}
	if got := rowValue(report.Rows, "Number"); got != "09/123" {
		t.Errorf("Number = %q", got)
	}
	if report.Station != "PHNL" || report.Center != nil || len(report.Area) != 0 {
		t.Errorf("station %q center %v area %v", report.Station, report.Center, report.Area)
	}
}

func TestATFRTableIsHeadedByTheRestrictionAndShowsAnImmediateStart(t *testing.T) {
	for text, want := range map[string]string{
		tfrExample: "| Radius | 5 NM |",
		tfrPolygon: "| Effective | immediately |",
	} {
		report, err := Decode(text, ref)
		if err != nil {
			t.Fatal(err)
		}
		table, ok := Expanded(tableHREF, report)
		if !ok {
			t.Fatal("no table")
		}
		if !strings.HasPrefix(table, "| NOTAM | "+tfrHeading+" |\n") {
			t.Errorf("header:\n%s", table)
		}
		if !strings.Contains(table, want) {
			t.Errorf("missing %q in:\n%s", want, table)
		}
	}
}

func twoDigits(n int) string {
	if n < 10 {
		return "0" + string(rune('0'+n))
	}
	return string(rune('0'+n/10)) + string(rune('0'+n%10))
}
