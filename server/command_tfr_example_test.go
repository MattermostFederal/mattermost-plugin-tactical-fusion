package main

import (
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/avreport"
)

func TestTheTFRExampleIsPostedAsACard(t *testing.T) {
	p := newTestPlugin(t, "https://example.com", true)
	api := p.API.(*fakeAPI)

	runExamplePosts(t, p)

	var blob map[string]any
	for _, post := range api.created {
		if post.Type == avreport.PostType && strings.HasPrefix(post.Message, "!FDC 6/4321") {
			blob, _ = post.GetProps()[avreport.PropsKey].(map[string]any)
		}
	}
	if blob == nil {
		t.Fatal("the TFR example was not posted as a card")
	}
	if blob["radius_nm"] != "3" || blob["value"] == "" {
		t.Errorf("the card does not carry the circle: radius %v value %v", blob["radius_nm"], blob["value"])
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
