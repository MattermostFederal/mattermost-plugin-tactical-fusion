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

func TestReportTableKeepsAHostileReportInsideOneCodeSpan(t *testing.T) {
	for raw, want := range map[string]string{
		metarLine + " RMK |PIPE|":              "| Report | `" + metarLine + " RMK \\|PIPE\\|` |",
		metarLine + " RMK `TICK":               "| Report | ``" + metarLine + " RMK `TICK`` |",
		metarLine + " RMK ``TWO`` |PIPE| ONE`": "| Report | ``` " + metarLine + " RMK ``TWO`` \\|PIPE\\| ONE` ``` |",
	} {
		table := expandFixture(t, raw, "")
		if !strings.Contains(table, want) {
			t.Errorf("%q is not held in one code span; want %q in:\n%s", raw, want, table)
		}
	}
}

func TestReportTableNamesTheStationAndItsAirfield(t *testing.T) {
	table := expandFixture(t, metarLine, "")
	if !strings.HasPrefix(table, "| METAR | [KJFK - John F. Kennedy International Airport](/plugins/tf/decorate/airport?v=KJFK) |\n|:--|:--|\n") {
		t.Errorf("header:\n%s", table)
	}

	unknown := expandFixture(t, "METAR ZZZZ 221651Z 28012KT 10SM FEW250 24/12 A3012", "")
	if !strings.HasPrefix(unknown, "| METAR | ZZZZ |\n") {
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

const multiLineTAF = "TAF PGUA 221720Z 2218/2324 07012KT P6SM SCT025\n  FM230600 09008KT P6SM FEW030\n  TEMPO 2312/2316 5SM -SHRA BKN020"

const icaoNotam = "A1234/26 NOTAMN\nQ) PHZH/QMRLC/IV/NBO/A/000/999/2119N15755W005\nA) PHNL B) 2609221200 C) 2609232359\nE) RWY 08L/26R CLSD DUE WIP"

func TestExpandedWritesAMultiLineReportAsAFenceAboveTheTable(t *testing.T) {
	for _, text := range []string{multiLineTAF, icaoNotam} {
		report, err := Decode(text, ref)
		if err != nil {
			t.Fatal(err)
		}
		message, ok := Expanded(tableHREF, report)
		if !ok {
			t.Fatalf("no expansion for %q", text)
		}

		if !strings.HasPrefix(message, "```\n"+report.Raw+"\n```\n| "+report.Kind+" | ["+report.Station+" - ") {
			t.Errorf("the fence does not lead:\n%s", message)
		}
		if strings.Contains(message, "| Report |") {
			t.Errorf("the report row repeats what the fence holds:\n%s", message)
		}
		if !strings.HasSuffix(message, "| Details | [Open details]("+tableHREF+") |") {
			t.Errorf("the details link is not last:\n%s", message)
		}
	}
}

func TestExpandedWritesASingleLineReportAsTheTableAlone(t *testing.T) {
	report, err := Decode(metarLine, ref)
	if err != nil {
		t.Fatal(err)
	}
	message, ok := Expanded(tableHREF, report)
	if !ok || strings.HasPrefix(message, "```") || !strings.Contains(message, "| Report | `"+metarLine+"` |") {
		t.Errorf("a one-line report is not the table alone:\n%s", message)
	}
}

func TestExpandedRefusesWhatAFenceCannotHold(t *testing.T) {
	report, err := Decode(multiLineTAF+"\n```", ref)
	if err != nil {
		t.Fatal(err)
	}
	if message, ok := Expanded(tableHREF, report); ok {
		t.Errorf("a report carrying a fence marker was fenced:\n%s", message)
	}
	if _, ok := Expanded("", report); ok {
		t.Error("an expansion was written with no destination")
	}
}
