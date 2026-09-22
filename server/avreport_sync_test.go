package main

import (
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/avreport"
	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/decorators/dtg"
)

func TestWebappAvReportPostTypeMatches(t *testing.T) {
	source := readWebappFile(t, "avreport", "types.ts")

	for name, want := range map[string]string{
		"AVREPORT_POST_TYPE":     avreport.PostType,
		"AVREPORT_PROPS_KEY":     avreport.PropsKey,
		"AVREPORT_PROPS_VERSION": strconv.Itoa(avreport.PropsVersion),
		"SOURCE_MESSAGE":         avreport.SourceMessage,
		"SOURCE_FENCE":           avreport.SourceFence,
		"MAX_REPORT_ROWS":        strconv.Itoa(avreport.MaxRows),
		"MAX_REPORT_PERIODS":     strconv.Itoa(avreport.MaxPeriods),
		"MAX_REPORT_UNKNOWN":     strconv.Itoa(avreport.MaxUnknown),
		"MAX_REPORT_FLAGS":       strconv.Itoa(avreport.MaxFlags),
	} {
		pattern := regexp.MustCompile(`export const ` + name + ` = '?([^';]+)'?;`)
		m := pattern.FindStringSubmatch(source)
		if m == nil {
			t.Fatalf("no `export const %s` in the webapp's avreport/types.ts; if it was renamed, "+
				"point this test at the new name rather than deleting it", name)
		}
		if m[1] != want {
			t.Errorf("%s = %q in the webapp, %q in Go", name, m[1], want)
		}
	}

	link := readWebappFile(t, "decorators", "avreport", "index.ts")
	if !strings.Contains(link, "export const MAX_SOURCE_RUNES = "+strconv.Itoa(avreport.MaxSourceRunes)+";") {
		t.Errorf("the webapp's MAX_SOURCE_RUNES is not %d", avreport.MaxSourceRunes)
	}
	if !strings.Contains(link, "type: '"+avreport.Type+"'") {
		t.Errorf("the webapp decorator's type is not %q", avreport.Type)
	}
	for name, want := range map[string]int64{
		"MIN_INSTANT_MS": dtg.MinInstantMillis,
		"MAX_INSTANT_MS": dtg.MaxInstantMillis,
	} {
		if !strings.Contains(link, "export const "+name+" = "+strconv.FormatInt(want, 10)+";") {
			t.Errorf("the webapp's %s is not %d, the window the report page holds an instant to", name, want)
		}
	}
}

func TestWebappAvReportKindsMatch(t *testing.T) {
	source := readWebappFile(t, "avreport", "types.ts")

	m := regexp.MustCompile(`export const KINDS = \[([^\]]+)\] as const;`).FindStringSubmatch(source)
	if m == nil {
		t.Fatal("no KINDS list in the webapp's avreport/types.ts")
	}
	var webapp []string
	for item := range strings.SplitSeq(m[1], ",") {
		webapp = append(webapp, strings.Trim(strings.TrimSpace(item), "'"))
	}

	if strings.Join(webapp, ",") != strings.Join(avreport.Kinds, ",") {
		t.Errorf("KINDS = %v in the webapp, %v in Go", webapp, avreport.Kinds)
	}
}

func TestWebappAvReportShapeMatches(t *testing.T) {
	source := readWebappFile(t, "avreport", "types.ts")

	webappKeys := map[string]bool{}
	for _, m := range regexp.MustCompile(`\w+\(\w+, '([a-z_]+)'`).FindAllStringSubmatch(source, -1) {
		webappKeys[m[1]] = true
	}
	for _, m := range regexp.MustCompile(`blob\.([a-z_]+)`).FindAllStringSubmatch(source, -1) {
		webappKeys[m[1]] = true
	}
	if len(webappKeys) == 0 {
		t.Fatal("no reads found in the webapp's avreport/types.ts")
	}

	goKeys := map[string]bool{}
	for _, text := range []string{avreport.Fixture(), reportTAF, reportICAONotam} {
		report, err := avreport.Decode(text, hookRef)
		if err != nil {
			t.Fatalf("Decode: %v", err)
		}
		collectKeys(avreport.PropsWithoutRows(report, avreport.Source{Kind: avreport.SourceFence, Text: text}), goKeys)
		collectKeys(avreport.Props(report, avreport.Source{Kind: avreport.SourceFence, Text: text}), goKeys)
	}

	for key := range goKeys {
		if !webappKeys[key] {
			t.Errorf("Go writes %q and the webapp never reads it", key)
		}
	}
	for key := range webappKeys {
		if !goKeys[key] {
			t.Errorf("the webapp reads %q but Go never writes it", key)
		}
	}
}

func TestWebappAvReportFixtureIsWhatGoRenders(t *testing.T) {
	source := readWebappFile(t, "avreport", "report_fixtures.ts")

	ref := time.Date(2026, time.September, 22, 17, 0, 0, 0, time.UTC)
	report, err := avreport.Decode("METAR PHNL 221651Z 07012G18KT 10SM FEW025 SCT045 27/19 A3010 RMK AO2 Q9999", ref)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	for _, want := range []string{
		"summary: '" + report.Summary + "'",
		"value: '" + report.Value + "'",
		"issuedAt: '" + strconv.FormatInt(report.Instant(), 10) + "'",
		"issued: '" + zuluOf(t, report) + "'",
		"stationName: '" + report.StationName + "'",
	} {
		if !strings.Contains(source, want) {
			t.Errorf("the webapp fixture does not carry %s", want)
		}
	}
}

func zuluOf(t *testing.T, report avreport.Report) string {
	t.Helper()

	issued, ok := avreport.Blob(report)["issued"].(string)
	if !ok {
		t.Fatal("the blob carries no issued text")
	}
	return issued
}
