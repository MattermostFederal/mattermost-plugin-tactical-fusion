package avreport

import (
	"net/url"
	"strings"
	"testing"
)

const tableHREF = "/plugins/tf/decorate/avreport?v=X&t=1"

func expandFixture(t *testing.T, text, trail string) string {
	t.Helper()

	params, ok := (&Decorator{}).Parse(text, ref)
	if !ok {
		t.Fatalf("Parse refused %q", text)
	}
	table := (&Decorator{}).ExpandMessage(tableHREF, trail, params)
	if table == "" {
		t.Fatalf("no table for %q", text)
	}
	return table
}

func TestReportTableEndsWithTheDetailsLink(t *testing.T) {
	for _, text := range []string{
		metarLine,
		"TAF PGUA 221720Z 2218/2324 07012KT P6SM SCT025",
		"!HNL 09/123 HNL RWY 08L/26R CLSD 2609221200-2609232359",
	} {
		lines := strings.Split(expandFixture(t, text, ""), "\n")
		if last := lines[len(lines)-1]; last != "| Details | [Open details]("+tableHREF+") |" {
			t.Errorf("%q ends with %q", text, last)
		}
	}
}

func TestReportTableCarriesTheReportVerbatimInACodeSpanWithItsTerminator(t *testing.T) {
	table := expandFixture(t, metarLine, "=")

	if !strings.Contains(table, "| Report | `"+metarLine+"`= |") {
		t.Errorf("the report row is not the report as written:\n%s", table)
	}
	if n := strings.Count(table, "/decorate/avreport?"); n != 1 {
		t.Errorf("the destination appears %d times, want once:\n%s", n, table)
	}
}

func TestReportTableEscapesAReportThatACodeSpanCannotHold(t *testing.T) {
	hostile := metarLine + " RMK |PIPE| `TICK`"
	table := expandFixture(t, hostile, "")

	if strings.Contains(table, "`"+hostile+"`") {
		t.Fatalf("a report carrying a backtick was written as a code span:\n%s", table)
	}
	if !strings.Contains(table, `\|PIPE\| \`+"`TICK\\`") {
		t.Errorf("the report row is not escaped:\n%s", table)
	}
}

func TestReportTableNamesTheStationAndItsAirfield(t *testing.T) {
	table := expandFixture(t, metarLine, "")
	if !strings.HasPrefix(table, "| METAR KJFK | John F. Kennedy International Airport |\n|:--|:--|\n") {
		t.Errorf("header:\n%s", table)
	}

	unknown := expandFixture(t, "METAR ZZZZ 221651Z 28012KT 10SM FEW250 24/12 A3012", "")
	if !strings.HasPrefix(unknown, "| METAR ZZZZ | "+tableFallbackHeading+" |\n") {
		t.Errorf("a station the build does not hold:\n%s", unknown)
	}
}

func TestReportTableSaysWhenTheDateWasInferred(t *testing.T) {
	table := expandFixture(t, metarLine, "")
	if !strings.Contains(table, "| Issued | 22 Sep 2026 16:51Z"+inferredDateNote+" |") {
		t.Errorf("the inferred instant is not flagged:\n%s", table)
	}

	notam := expandFixture(t, "!HNL 09/123 HNL RWY 08L/26R CLSD 2609221200-2609232359", "")
	if !strings.Contains(notam, "| Effective | 22 Sep 2026 12:00Z |") || strings.Contains(notam, inferredDateNote) {
		t.Errorf("a NOTAM carries its own date and is labeled Effective:\n%s", notam)
	}
}

func TestReportTableWritesEachTAFPeriodAsOneRow(t *testing.T) {
	table := expandFixture(t, "TAF PGUA 221720Z 2218/2324 07012KT P6SM SCT025 TEMPO 2312/2316 5SM -SHRA BKN020", "")
	for _, want := range []string{
		"| Base forecast | Wind: 070° at 12 kt; ",
		"| Temporarily day 23 at 12:00Z to day 23 at 16:00Z | Visibility: 5 statute miles; ",
	} {
		if !strings.Contains(table, want) {
			t.Errorf("missing %q in:\n%s", want, table)
		}
	}
}

func TestReportTableIsWellFormed(t *testing.T) {
	for _, text := range []string{
		metarLine,
		metarLine + " RMK |PIPE|",
		"TAF PGUA 221720Z 2218/2324 07012KT P6SM SCT025 TEMPO 2312/2316 5SM -SHRA BKN020",
		"!HNL 09/123 HNL RWY 08L/26R CLSD 2609221200-2609232359",
		"!JFK 09/001 JFK TWY A CLSD",
	} {
		for i, line := range strings.Split(expandFixture(t, text, ""), "\n") {
			if i == 1 {
				if line != "|:--|:--|" {
					t.Errorf("%q line 2 = %q", text, line)
				}
				continue
			}
			cells := strings.Count(strings.ReplaceAll(line, `\|`, ""), "|")
			if !strings.HasPrefix(line, "| ") || !strings.HasSuffix(line, " |") || cells != 3 {
				t.Errorf("%q has a malformed row %q", text, line)
			}
		}
	}
}

func TestExpandMessageFollowsTheSwitchAndTheLink(t *testing.T) {
	params, _ := (&Decorator{}).Parse(metarLine, ref)

	off := &Decorator{Enabled: func() Formats { return Formats{METAR: true} }}
	if got := off.ExpandMessage(tableHREF, "", params); got != "" {
		t.Errorf("a table was written with the switch off:\n%s", got)
	}
	if got := (&Decorator{}).ExpandMessage("", "", params); got != "" {
		t.Errorf("a table was written with no destination:\n%s", got)
	}
	if got := (&Decorator{}).ExpandMessage(tableHREF, "", url.Values{ParamValue: {metarLine}, ParamInstant: {"1"}}); got != "" {
		t.Errorf("a table was written for a hand-edited link:\n%s", got)
	}
}
