package main

import (
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/avreport"
)

func TestTheTFRExampleIsPostedAsARestrictionTable(t *testing.T) {
	p := newTestPlugin(t, "https://example.com", true)
	api := p.API.(*fakeAPI)

	runExamplePosts(t, p)

	var table string
	for _, post := range api.created {
		if strings.Contains(post.Message, "| NOTAM | Temporary flight restriction |") {
			table = post.Message
		}
	}
	if table == "" {
		t.Fatal("the TFR example was not posted as a table")
	}
	for _, want := range []string{"| Radius | 3 NM |", "| Altitudes | surface to 3,000 ft MSL |", "| Details | [Open details]("} {
		if !strings.Contains(table, want) {
			t.Errorf("missing %q in:\n%s", want, table)
		}
	}
}

func TestTheTFRExampleDecodesAtAnyTime(t *testing.T) {
	now := time.Date(2026, time.December, 31, 23, 30, 0, 0, time.UTC)
	report, err := avreport.Decode(tfrExampleMessage(now), now)
	if err != nil {
		t.Fatalf("the example does not decode: %v", err)
	}
	if report.Center == nil || report.RadiusNm != "3" {
		t.Errorf("the example draws no circle: %v %q", report.Center, report.RadiusNm)
	}
	if want := time.Date(2026, time.December, 31, 23, 0, 0, 0, time.UTC); !report.IssuedAt.Equal(want) {
		t.Errorf("IssuedAt = %v, want %v", report.IssuedAt, want)
	}
	if n := utf8.RuneCountInString(tfrExampleMessage(now)); n > safePostRunes {
		t.Errorf("the example is %d runes, past the safe post size", n)
	}
}

func TestTheTFRExampleFollowsTheReportSwitches(t *testing.T) {
	p := newTestPlugin(t, "https://example.com", true)
	withConfiguration(p, func(c *configuration) { c.EnableAvReportNOTAM = false })

	if n := p.tfrExampleCount(); n != 0 || len(p.tfrExampleMessages()) != 0 {
		t.Errorf("the TFR example is offered with NOTAMs off: %d", n)
	}
}
