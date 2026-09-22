package main

import (
	"encoding/json"
	"errors"
	"math"
	"net/url"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/mattermost/mattermost/server/public/model"

	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/avreport"
	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/cot"
	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/decorators"
	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/decorators/airport"
	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/errcode"
	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/geojson"
)

const reportMETAR = "METAR PHNL 221651Z 07012G18KT 10SM FEW025 SCT045 27/19 A3010 RMK AO2 SLP193"

const reportTAF = "TAF PHNL 221720Z 2218/2324 07012KT P6SM SCT025\n" +
	"  FM230600 09008KT P6SM FEW030\n" +
	"  TEMPO 2312/2316 5SM -SHRA BKN020"

const reportICAONotam = "A1234/26 NOTAMN\n" +
	"Q) PHZH/QMRLC/IV/NBO/A/000/999/2119N15755W005\n" +
	"A) PHNL B) 2609221200 C) 2609232359\n" +
	"E) RWY 08L/26R CLSD DUE WIP"

const reportFAANotam = "!HNL 09/123 HNL RWY 08L/26R CLSD 2609221200-2609232359"

func reportFence(info, body string) string {
	return "```" + info + "\n" + body + "\n```"
}

func reportBlob(t *testing.T, post *model.Post) map[string]any {
	t.Helper()

	if post == nil {
		t.Fatal("the post was not stamped")
	}
	blob, ok := post.GetProps()[avreport.PropsKey].(map[string]any)
	if !ok {
		t.Fatalf("props carry no %s blob: %#v", avreport.PropsKey, post.GetProps())
	}
	return blob
}

func stampedReport(t *testing.T, p *Plugin, message string) *model.Post {
	t.Helper()

	updated := p.decoratePost(&model.Post{Message: message, UserId: testUserID}, hookRef)
	if updated == nil || updated.Type != avreport.PostType {
		t.Fatalf("the report was not stamped: %+v", updated)
	}
	if _, ok := updated.GetProps()[avreport.PropsKey]; !ok {
		t.Fatal("the type was set without its props")
	}
	return updated
}

func TestAvReportStampsAFencedReportInAnySpelling(t *testing.T) {
	for _, info := range []string{"metar", "METAR", "Metar", "speci", "taf", "notam"} {
		t.Run(info, func(t *testing.T) {
			p := newTestPlugin(t, "https://example.com", true)
			updated := stampedReport(t, p, reportFence(info, reportMETAR))

			blob := reportBlob(t, updated)
			if blob["source"] != avreport.SourceFence {
				t.Errorf("source = %v, want %q", blob["source"], avreport.SourceFence)
			}
			if blob["src"] != reportMETAR {
				t.Errorf("src = %q", blob["src"])
			}
			if blob["kind"] != avreport.KindMETAR {
				t.Errorf("kind = %v", blob["kind"])
			}
		})
	}
}

func TestAvReportStampsAFencedTAFAndNotam(t *testing.T) {
	p := newTestPlugin(t, "https://example.com", true)

	for body, kind := range map[string]string{
		reportTAF:       avreport.KindTAF,
		reportICAONotam: avreport.KindNOTAM,
		reportFAANotam:  avreport.KindNOTAM,
	} {
		blob := reportBlob(t, stampedReport(t, p, reportFence("taf", body)))
		if blob["kind"] != kind {
			t.Errorf("kind = %v for %q, want %s", blob["kind"], body[:12], kind)
		}
	}
}

func TestAvReportStampsABareMultiLineReport(t *testing.T) {
	p := newTestPlugin(t, "https://example.com", true)
	withConfiguration(p, tableOff)

	for _, body := range []string{reportTAF, reportICAONotam, reportFAANotam + "\nRMK NONE"} {
		blob := reportBlob(t, stampedReport(t, p, body))
		if blob["source"] != avreport.SourceMessage {
			t.Errorf("source = %v, want %q", blob["source"], avreport.SourceMessage)
		}
	}
}

func TestASoleSingleLineReportIsExpandedNotStamped(t *testing.T) {
	p := newTestPlugin(t, "https://example.com", true)

	for _, message := range []string{reportMETAR, reportMETAR + "\n", reportMETAR + "=", reportFAANotam, "  " + reportMETAR + "  "} {
		updated := p.decoratePost(&model.Post{Message: message, UserId: testUserID}, hookRef)
		if updated == nil {
			t.Fatalf("%q was left alone", message)
		}
		if updated.Type != "" {
			t.Fatalf("%q was stamped as %q", message, updated.Type)
		}
		if props := standaloneProps(t, updated); props != nil {
			t.Fatalf("%q was given props: %v", message, props)
		}
		for _, want := range []string{"|:--|:--|\n| ", "| Details | [Open details](/plugins/", "/decorate/avreport?"} {
			if !strings.Contains(updated.Message, want) {
				t.Fatalf("%q was not expanded; missing %q in:\n%s", message, want, updated.Message)
			}
		}
		if n := strings.Count(updated.Message, "/decorate/avreport?"); n != 1 {
			t.Fatalf("%q links its destination %d times, want once:\n%s", message, n, updated.Message)
		}
	}
}

func TestTheReportTableLinksTheStationTheWayTheTaggerWould(t *testing.T) {
	p := newTestPlugin(t, "https://example.com", true)
	tagger := &decorators.Tagger{Registry: p.decorators, URLPrefix: p.decorateURLPrefix()}
	want := "| METAR | [PHNL - Daniel K. Inouye International Airport](" + tagger.URLFor(airport.Type, url.Values{"v": {"PHNL"}}) + ") |"

	for _, message := range []string{reportMETAR, reportTAF} {
		updated := p.decoratePost(&model.Post{Message: message, UserId: testUserID}, hookRef)
		if updated == nil || !strings.Contains(updated.Message, strings.Replace(want, "METAR", strings.Fields(message)[0], 1)) {
			t.Errorf("the station is not linked to its airfield:\n%s", updated.Message)
		}
	}
}

func TestAReportInsideASentenceIsLinkedNotExpanded(t *testing.T) {
	p := newTestPlugin(t, "https://example.com", true)

	updated := p.decoratePost(&model.Post{Message: "current: " + reportMETAR, UserId: testUserID}, hookRef)
	if updated == nil || !strings.Contains(updated.Message, "/decorate/avreport?") {
		t.Fatalf("the report was not linked: %+v", updated)
	}
	if strings.Contains(updated.Message, "| Details |") {
		t.Errorf("a report inside a sentence was expanded:\n%s", updated.Message)
	}
}

func TestASoleReportIsLinkedNotExpandedWhenTheTableIsOff(t *testing.T) {
	p := newTestPlugin(t, "https://example.com", true)

	config := p.getConfiguration().Clone()
	config.EnableAvReportTable = false
	p.setConfiguration(config)

	updated := p.decoratePost(&model.Post{Message: reportMETAR, UserId: testUserID}, hookRef)
	if updated == nil || !strings.HasPrefix(updated.Message, "["+reportMETAR+"](") {
		t.Fatalf("the report was not linked on its own: %+v", updated)
	}
	if strings.Contains(updated.Message, "| Details |") {
		t.Errorf("a table was written with the switch off:\n%s", updated.Message)
	}
}

func TestAnExpandedReportIsNotExpandedAgain(t *testing.T) {
	p := newTestPlugin(t, "https://example.com", true)

	for _, message := range []string{
		reportMETAR,
		"TAF PGUA 221720Z 2218/2324 07012KT P6SM SCT025",
		reportFAANotam,
		reportMETAR + " RMK |PIPE|",
		reportMETAR + " RMK `TICK",
		reportMETAR + " RMK |PIPE| TICK`",
	} {
		first := p.decoratePost(&model.Post{Message: message, UserId: testUserID}, hookRef)
		if first == nil || !strings.Contains(first.Message, "| Details |") {
			t.Fatalf("%q was not expanded: %+v", message, first)
		}
		if again := p.decoratePost(&model.Post{Message: first.Message, UserId: testUserID}, hookRef); again != nil {
			t.Errorf("the stored table for %q was rewritten again:\n%s", message, again.Message)
		}
	}
}

func TestTwoReportsOnTwoLinesAreLinkedNotStamped(t *testing.T) {
	p := newTestPlugin(t, "https://example.com", true)
	second := "METAR PGUA 221654Z 07008KT 10SM SCT020 29/24 A2990"

	updated := p.decoratePost(&model.Post{Message: reportMETAR + "\n" + second, UserId: testUserID}, hookRef)
	if updated == nil || updated.Type != "" {
		t.Fatalf("a bundle of reports was stamped: %+v", updated)
	}
	if strings.Count(updated.Message, "/decorate/avreport?") != 2 {
		t.Fatalf("each line was not linked on its own: %q", updated.Message)
	}
}

func TestAvReportLeavesTheMessageExactlyAsWritten(t *testing.T) {
	p := newTestPlugin(t, "https://example.com", true)
	withConfiguration(p, tableOff)

	for _, message := range []string{reportFence("metar", reportMETAR), reportTAF, "  " + reportTAF + "\n\n"} {
		updated := stampedReport(t, p, message)
		if updated.Message != message {
			t.Fatalf("the message was rewritten:\n%q\n%q", message, updated.Message)
		}
	}
}

func tableOff(c *configuration) { c.EnableAvReportTable = false }

func TestABareMultiLineReportIsExpandedNotStamped(t *testing.T) {
	p := newTestPlugin(t, "https://example.com", true)

	for _, body := range []string{reportTAF, reportICAONotam, reportFAANotam + "\nRMK NONE"} {
		updated := p.decoratePost(&model.Post{Message: body, UserId: testUserID}, hookRef)
		if updated == nil || updated.Type != "" {
			t.Fatalf("%q was not left an ordinary post: %+v", body, updated)
		}
		if _, ok := updated.GetProps()[avreport.PropsKey]; ok {
			t.Fatalf("%q was given the card's props", body)
		}
		if !strings.HasPrefix(updated.Message, "| ") || strings.Contains(updated.Message, strings.TrimSpace(body)) {
			t.Fatalf("%q is not the table alone:\n%s", body, updated.Message)
		}
		if !strings.Contains(updated.Message, "| Details | [Open details](/plugins/") || strings.Count(updated.Message, "/decorate/avreport?") != 1 {
			t.Fatalf("%q has no single details link:\n%s", body, updated.Message)
		}
		if again := p.decoratePost(&model.Post{Message: updated.Message, UserId: testUserID}, hookRef); again != nil {
			t.Errorf("the stored table for %q was rewritten again as %q:\n%s", body, again.Type, again.Message)
		}
	}
}

func TestABareMultiLineReportIsPlainTextWhenTheTableAndTheCardAreOff(t *testing.T) {
	p := newTestPlugin(t, "https://example.com", true)
	withConfiguration(p, func(c *configuration) { c.EnableAvReportTable = false; c.EnableAvReportCard = false })

	updated := p.decoratePost(&model.Post{Message: reportTAF, UserId: testUserID}, hookRef)
	if updated != nil && (updated.Type != "" || strings.Contains(updated.Message, "| Details |") || strings.HasPrefix(updated.Message, "```")) {
		t.Fatalf("a bare report was given a shape with both shapes off: %+v", updated)
	}
	if updated := stampedReport(t, withTable(p), reportFence("taf", reportTAF)); updated.Message != reportFence("taf", reportTAF) {
		t.Fatal("a fenced report was rewritten")
	}
}

func withTable(p *Plugin) *Plugin {
	withConfiguration(p, func(c *configuration) { c.EnableAvReportTable = true; c.EnableAvReportCard = true })
	return p
}

func TestTheTableBeatsAnAttachmentAsTheCardDoes(t *testing.T) {
	p := newTestPlugin(t, "https://example.com", true)
	api := p.API.(*fakeAPI)
	api.files = map[string]*model.FileInfo{
		testFileID: {Id: testFileID, Name: "track.cot", Size: int64(len(cotEventXML)), CreatorId: testUserID},
	}
	api.fileContent = map[string][]byte{testFileID: []byte(cotEventXML)}

	updated := p.decoratePost(&model.Post{Message: reportTAF, FileIds: []string{testFileID}, UserId: testUserID}, hookRef)
	if updated == nil || updated.Type != "" || !strings.HasPrefix(updated.Message, "| TAF | ") {
		t.Fatalf("the attachment won over the visible report: %+v", updated)
	}
}

func multiLineReportOfKind(t *testing.T, kind string) string {
	t.Helper()

	switch kind {
	case avreport.KindMETAR:
		return reportFence("metar", reportMETAR)
	case avreport.KindSPECI:
		return reportFence("speci", strings.Replace(reportMETAR, "METAR", "SPECI", 1))
	case avreport.KindTAF:
		return reportTAF
	case avreport.KindNOTAM:
		return reportICAONotam
	}
	t.Fatalf("no fixture for kind %q; add one when a kind is added", kind)
	return ""
}

func kindSwitchOff(t *testing.T, kind string) func(c *configuration) {
	t.Helper()

	switch kind {
	case avreport.KindMETAR, avreport.KindSPECI:
		return func(c *configuration) { c.EnableAvReportMETAR = false }
	case avreport.KindTAF:
		return func(c *configuration) { c.EnableAvReportTAF = false }
	case avreport.KindNOTAM:
		return func(c *configuration) { c.EnableAvReportNOTAM = false }
	}
	t.Fatalf("no switch for kind %q; add one when a kind is added", kind)
	return nil
}

func kindFamily(kind string) string {
	if kind == avreport.KindSPECI {
		return avreport.KindMETAR
	}
	return kind
}

func TestAvReportIsSilentWhenTheAdminTurnedItOff(t *testing.T) {
	type switchCase struct {
		mutate  func(c *configuration)
		offKind string
	}
	cases := map[string]switchCase{
		"card":   {func(c *configuration) { c.EnableAvReportCard = false }, ""},
		"parent": {func(c *configuration) { c.EnableAvReport = false }, ""},
	}
	for _, kind := range avreport.Kinds {
		cases["kind "+kind] = switchCase{kindSwitchOff(t, kind), kind}
	}

	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			p := newTestPlugin(t, "https://example.com", true)
			withConfiguration(p, tableOff)
			withConfiguration(p, c.mutate)
			api := p.API.(*fakeAPI)

			for _, kind := range avreport.Kinds {
				message := multiLineReportOfKind(t, kind)
				refused := c.offKind == "" || kindFamily(kind) == kindFamily(c.offKind)
				if !refused {
					stampedReport(t, p, message)
					continue
				}
				updated := p.decoratePost(&model.Post{Message: message, UserId: testUserID}, hookRef)
				if updated != nil && updated.Type != "" {
					t.Fatalf("%s: Type = %q with the switch off", kind, updated.Type)
				}
			}
			if len(api.ephemeral) != 0 || len(api.warnings) != 0 {
				t.Fatal("a switched-off stamper spoke")
			}
		})
	}
}

func TestAKindThatIsOffDoesNotSuppressAnAttachment(t *testing.T) {
	p := newTestPlugin(t, "https://example.com", true)
	withConfiguration(p, func(c *configuration) { c.EnableAvReportTAF = false })
	api := p.API.(*fakeAPI)
	api.files = map[string]*model.FileInfo{
		testFileID: {Id: testFileID, Name: "track.cot", Size: int64(len(cotEventXML)), CreatorId: testUserID},
	}
	api.fileContent = map[string][]byte{testFileID: []byte(cotEventXML)}

	for name, message := range map[string]string{
		"bare":   reportTAF,
		"fenced": reportFence("taf", reportTAF),
	} {
		t.Run(name, func(t *testing.T) {
			post := &model.Post{Message: message, FileIds: []string{testFileID}, UserId: testUserID}
			updated := p.decoratePost(post, hookRef)
			if updated == nil || updated.Type != cot.PostType {
				t.Fatalf("a TAF whose kind is off still suppressed the attachment: %+v", updated)
			}
		})
	}
}

func TestALabeledReportFenceThatFailsTellsItsAuthor(t *testing.T) {
	p := newTestPlugin(t, "https://example.com", true)
	api := p.API.(*fakeAPI)

	post := &model.Post{Message: reportFence("metar", "not a report at all"), UserId: testUserID}
	if updated := p.decoratePost(post, hookRef); updated != nil && updated.Type != "" {
		t.Fatal("an unreadable report was stamped")
	}
	if len(api.ephemeral) == 0 || !strings.Contains(api.ephemeral[0].Message, "TF-11019") {
		t.Fatalf("the author was not told: %+v", api.ephemeral)
	}
	found := false
	for _, code := range api.warnCodes {
		if code == errcode.HooksAvReportUnreadable {
			found = true
		}
	}
	if !found {
		t.Fatal("a labeled refusal wrote no warning")
	}
}

func TestABareRefusalIsSilentAndUnlogged(t *testing.T) {
	p := newTestPlugin(t, "https://example.com", true)
	api := p.API.(*fakeAPI)

	for _, message := range []string{
		"TAF PHNL 221720Z\nnothing more",
		"!HNL 09/123\nsecond line",
		"KJFK 221651Z 28012KT\nnot a metar after all\n" + strings.Repeat("x", avreport.MaxSourceRunes),
	} {
		if updated := p.decoratePost(&model.Post{Message: message, UserId: testUserID}, hookRef); updated != nil && updated.Type != "" {
			t.Fatalf("%q was stamped", message)
		}
	}
	if len(api.ephemeral) != 0 {
		t.Fatalf("a bare failure told the author: %+v", api.ephemeral)
	}
	for _, code := range api.warnCodes {
		if code == errcode.HooksAvReportUnreadable {
			t.Fatal("a bare failure was logged")
		}
	}
}

func TestOtherFencesAreNeverAReport(t *testing.T) {
	p := newTestPlugin(t, "https://example.com", true)
	api := p.API.(*fakeAPI)

	for _, info := range []string{"json", "xml", "", "text", "wx"} {
		post := &model.Post{Message: reportFence(info, reportMETAR), UserId: testUserID}
		if updated := p.decoratePost(post, hookRef); updated != nil && updated.Type != "" {
			t.Fatalf("a %q fence was stamped as %q", info, updated.Type)
		}
	}
	if len(api.ephemeral) != 0 {
		t.Fatal("an unlabeled fence spoke")
	}
}

func TestAReportFenceIsNeverCotOrGeoJSON(t *testing.T) {
	p := newTestPlugin(t, "https://example.com", true)

	for _, body := range []string{cotEventXML, geoPoint} {
		post := &model.Post{Message: reportFence("metar", body), UserId: testUserID}
		if updated := p.decoratePost(post, hookRef); updated != nil && updated.Type != "" {
			t.Fatalf("a metar fence was stamped as %q", updated.Type)
		}
	}
}

func TestAvReportAndDecorationAreExclusive(t *testing.T) {
	p := newTestPlugin(t, "https://example.com", true)

	updated := stampedReport(t, p, "seen near 21.3353N 157.9483W at 091630ZAUG26\n"+reportFence("taf", reportTAF))
	if strings.Contains(updated.Message, "/plugins/") {
		t.Fatal("the text around the fence was decorated")
	}
}

func TestAForgedAvReportTypeIsStripped(t *testing.T) {
	p := newTestPlugin(t, "https://example.com", true)

	post := &model.Post{Message: "nothing to see", UserId: testUserID, Type: avreport.PostType}
	post.AddProp(avreport.PropsKey, map[string]any{"forged": true})

	updated := p.decoratePost(post, hookRef)
	if updated == nil {
		t.Fatal("a forged type was left in place")
	}
	if updated.Type != "" {
		t.Errorf("Type = %q, want it stripped", updated.Type)
	}
	if _, carried := updated.GetProps()[avreport.PropsKey]; carried {
		t.Error("the forged blob survived")
	}
}

func TestAForgedAvReportSiblingBlobIsStripped(t *testing.T) {
	p := newTestPlugin(t, "https://example.com", true)

	post := &model.Post{Message: reportFence("geojson", geoPoint), UserId: testUserID}
	post.AddProp(avreport.PropsKey, map[string]any{"forged": true})

	updated := p.decoratePost(post, hookRef)
	if updated == nil || updated.Type != geojson.PostType {
		t.Fatal("the document was not stamped")
	}
	if _, carried := updated.GetProps()[avreport.PropsKey]; carried {
		t.Error("a forged report blob rode along on a GeoJSON stamp")
	}
}

func TestAnotherIntegrationsTypeIsStillLeftAloneByAvReport(t *testing.T) {
	p := newTestPlugin(t, "https://example.com", true)

	post := &model.Post{Message: reportTAF, UserId: testUserID, Type: "custom_other"}
	post.AddProp("from_another_plugin", "keep me")

	if updated := p.decoratePost(post, hookRef); updated != nil && updated.Type != "custom_other" {
		t.Fatalf("Type = %q, want the other integration's", updated.Type)
	}
}

func TestAvReportKeepsAnotherIntegrationsProps(t *testing.T) {
	p := newTestPlugin(t, "https://example.com", true)
	withConfiguration(p, tableOff)

	post := &model.Post{Message: reportTAF, UserId: testUserID}
	post.AddProp("from_another_plugin", "keep me")

	updated := p.decoratePost(post, hookRef)
	if updated == nil || updated.Type != avreport.PostType {
		t.Fatal("the report was not stamped")
	}
	if updated.GetProp("from_another_plugin") != "keep me" {
		t.Fatal("another integration's props were dropped")
	}
}

func reportRungRunes(t *testing.T, text string, withRows bool) int {
	t.Helper()

	report, err := avreport.Decode(text, hookRef)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	source := avreport.Source{Kind: avreport.SourceMessage, Text: text}

	blob := avreport.PropsWithoutRows(report, source)
	if withRows {
		blob = avreport.Props(report, source)
	}
	encoded, err := json.Marshal(blob)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	return utf8.RuneCountInString(string(encoded))
}

func TestOverBudgetTheRowsAreDroppedBeforeTheCardIs(t *testing.T) {
	p := newTestPlugin(t, "https://example.com", true)
	withConfiguration(p, tableOff)
	api := p.API.(*fakeAPI)

	post := &model.Post{Message: reportTAF, UserId: testUserID}
	post.AddProp("bulky", strings.Repeat("x", model.PostPropsMaxUserRunes-reportRungRunes(t, reportTAF, false)-200))

	if reportRungRunes(t, reportTAF, true)-reportRungRunes(t, reportTAF, false) <= 200 {
		t.Fatal("the fixture's rows are too small to tell the rungs apart")
	}

	updated := p.decoratePost(post, hookRef)
	if updated == nil || updated.Type != avreport.PostType {
		t.Fatal("the report was not stamped on the lower rung")
	}

	blob := reportBlob(t, updated)
	if blob["rows_dropped"] != "1" {
		t.Fatal("the lower rung did not mark itself")
	}
	if rows, _ := blob["rows"].([]any); len(rows) != 0 {
		t.Fatal("the lower rung still carried rows")
	}
	if blob["src"] != strings.TrimSpace(reportTAF) {
		t.Fatal("the lower rung lost the report text")
	}
	if blob["summary"] == "" {
		t.Fatal("the lower rung lost the summary")
	}

	found := false
	for _, code := range api.warnCodes {
		if code == errcode.HooksAvReportRowsDropped {
			found = true
		}
	}
	if !found {
		t.Fatal("the degraded rung was not logged")
	}
}

func TestPastTheLastRungTheReportIsRefused(t *testing.T) {
	p := newTestPlugin(t, "https://example.com", true)
	api := p.API.(*fakeAPI)

	post := &model.Post{Message: reportFence("taf", reportTAF), UserId: testUserID}
	post.AddProp("bulky", strings.Repeat("x", model.PostPropsMaxUserRunes))

	if updated := p.decoratePost(post, hookRef); updated != nil && updated.Type != "" {
		t.Fatal("a report over the budget was stamped anyway")
	}
	if len(api.ephemeral) == 0 || !strings.Contains(api.ephemeral[0].Message, "TF-11021") {
		t.Fatal("the author was not told why")
	}
}

func TestAvReportReportsPropsItCannotMeasure(t *testing.T) {
	p := newTestPlugin(t, "https://example.com", true)
	api := p.API.(*fakeAPI)

	post := &model.Post{Message: reportFence("taf", reportTAF), UserId: testUserID}
	post.AddProp("unmarshalable", math.Inf(1))

	if updated := p.decoratePost(post, hookRef); updated != nil && updated.Type != "" {
		t.Fatal("a post whose props cannot be measured was stamped anyway")
	}
	if len(api.ephemeral) == 0 || !strings.Contains(api.ephemeral[0].Message, "TF-11020") {
		t.Fatal("the author was not told the props could not be measured")
	}
}

func TestRunStamperRecoversAndHandsBackTheStrippedPost(t *testing.T) {
	p := newTestPlugin(t, "https://example.com", true)
	api := p.API.(*fakeAPI)

	post := &model.Post{Message: reportTAF, UserId: testUserID, Type: avreport.PostType}
	post.AddProp(avreport.PropsKey, map[string]any{"forged": true})

	updated, stamped := p.runStamper(post, func() bool { return true }, errcode.HooksAvReportPanic, "tactical-fusion: test panic",
		func(*model.Post) (*model.Post, bool) { panic(errors.New("boom")) })

	if stamped {
		t.Fatal("a panicking recognizer still stamped")
	}
	if updated == nil || updated.Type != "" {
		t.Fatalf("the stripped post was not handed back: %+v", updated)
	}
	if _, carried := updated.GetProps()[avreport.PropsKey]; carried {
		t.Error("the forged blob survived the panic")
	}
	if len(api.warnCodes) == 0 || api.warnCodes[len(api.warnCodes)-1] != errcode.HooksAvReportPanic {
		t.Fatalf("the recover did not log its code: %v", api.warnCodes)
	}
}

func TestRunStamperNeverRecognizesWhenOffOrTyped(t *testing.T) {
	p := newTestPlugin(t, "https://example.com", true)
	called := false
	recognize := func(post *model.Post) (*model.Post, bool) { called = true; return post, true }

	if _, stamped := p.runStamper(&model.Post{Message: "x"}, func() bool { return false }, 0, "", recognize); stamped || called {
		t.Fatal("a switched-off stamper reached its recognizer")
	}
	if _, stamped := p.runStamper(&model.Post{Message: "x", Type: "custom_other"}, func() bool { return true }, 0, "", recognize); stamped || called {
		t.Fatal("a typed post reached the recognizer")
	}
}

func TestTheVisibleReportBeatsAnAttachmentAcrossFormats(t *testing.T) {
	t.Run("a taf fence beside a .cot attachment", func(t *testing.T) {
		p := newTestPlugin(t, "https://example.com", true)
		withConfiguration(p, tableOff)
		api := p.API.(*fakeAPI)
		api.files = map[string]*model.FileInfo{
			testFileID: {Id: testFileID, Name: "track.cot", Size: int64(len(cotEventXML)), CreatorId: testUserID},
		}
		api.fileContent = map[string][]byte{testFileID: []byte(cotEventXML)}

		post := &model.Post{Message: reportFence("taf", reportTAF), FileIds: []string{testFileID}, UserId: testUserID}
		updated := p.decoratePost(post, hookRef)
		if updated == nil || updated.Type != avreport.PostType {
			t.Fatalf("the attachment beat the visible report: %+v", updated)
		}
	})

	t.Run("a bare TAF beside a .geojson attachment", func(t *testing.T) {
		p := newTestPlugin(t, "https://example.com", true)
		withConfiguration(p, tableOff)
		api := p.API.(*fakeAPI)
		api.files = map[string]*model.FileInfo{
			geoFileID: {Id: geoFileID, Name: "overlay.geojson", Size: int64(len(geoPoint)), CreatorId: testUserID},
		}
		api.fileContent = map[string][]byte{geoFileID: []byte(geoPoint)}

		post := &model.Post{Message: reportTAF, FileIds: []string{geoFileID}, UserId: testUserID}
		updated := p.decoratePost(post, hookRef)
		if updated == nil || updated.Type != avreport.PostType {
			t.Fatalf("the attachment beat the visible report: %+v", updated)
		}
	})

	t.Run("a report fence does not suppress the attachment when the card is off", func(t *testing.T) {
		p := newTestPlugin(t, "https://example.com", true)
		withConfiguration(p, tableOff)
		withConfiguration(p, func(c *configuration) { c.EnableAvReportCard = false })
		api := p.API.(*fakeAPI)
		api.files = map[string]*model.FileInfo{
			testFileID: {Id: testFileID, Name: "track.cot", Size: int64(len(cotEventXML)), CreatorId: testUserID},
		}
		api.fileContent = map[string][]byte{testFileID: []byte(cotEventXML)}

		post := &model.Post{Message: reportFence("taf", reportTAF), FileIds: []string{testFileID}, UserId: testUserID}
		updated := p.decoratePost(post, hookRef)
		if updated == nil || updated.Type != cot.PostType {
			t.Fatalf("a switched-off report still suppressed the attachment: %+v", updated)
		}
	})
}

func TestALabeledReportFailureStillSuppressesTheAttachment(t *testing.T) {
	p := newTestPlugin(t, "https://example.com", true)
	api := p.API.(*fakeAPI)
	api.files = map[string]*model.FileInfo{
		testFileID: {Id: testFileID, Name: "track.cot", Size: int64(len(cotEventXML)), CreatorId: testUserID},
	}
	api.fileContent = map[string][]byte{testFileID: []byte(cotEventXML)}

	post := &model.Post{Message: reportFence("metar", "not a report"), FileIds: []string{testFileID}, UserId: testUserID}
	if updated := p.decoratePost(post, hookRef); updated != nil && updated.Type != "" {
		t.Fatalf("the attachment was read past a labeled report fence: %q", updated.Type)
	}
	if len(api.ephemeral) == 0 || !strings.Contains(api.ephemeral[0].Message, "TF-11019") {
		t.Fatal("the author was not told why the fence was refused")
	}
}

func TestABareReportNeverReachesIntoCode(t *testing.T) {
	p := newTestPlugin(t, "https://example.com", true)

	for _, message := range []string{
		reportTAF + "\nsee `FM230600`",
		"TAF PHNL 221720Z 2218/2324 07012KT P6SM SCT025\n```\nFM230600 09008KT P6SM FEW030\n```",
	} {
		if updated := p.decoratePost(&model.Post{Message: message, UserId: testUserID}, hookRef); updated != nil && updated.Type != "" {
			t.Fatalf("a report reached into code: %q", message)
		}
	}
}

func TestCRLFIsReadAsMultiLine(t *testing.T) {
	p := newTestPlugin(t, "https://example.com", true)
	withConfiguration(p, tableOff)

	blob := reportBlob(t, stampedReport(t, p, strings.ReplaceAll(reportTAF, "\n", "\r\n")))
	if blob["src"] != reportTAF {
		t.Fatalf("src = %q", blob["src"])
	}
}

func TestEveryReportBlobMarshals(t *testing.T) {
	for _, text := range []string{reportMETAR, reportTAF, reportICAONotam, reportFAANotam, avreport.Fixture()} {
		report, err := avreport.Decode(text, hookRef)
		if err != nil {
			t.Fatalf("Decode(%q): %v", text[:10], err)
		}
		for _, blob := range []map[string]any{
			avreport.Props(report, avreport.Source{Kind: avreport.SourceFence, Text: text}),
			avreport.PropsWithoutRows(report, avreport.Source{Kind: avreport.SourceMessage, Text: text}),
			avreport.Blob(report),
		} {
			if _, err := json.Marshal(blob); err != nil {
				t.Errorf("Marshal: %v", err)
			}
		}
	}
}

func TestTheReportDecoratorDeclaresNoPostType(t *testing.T) {
	var d decorators.Decorator = &avreport.Decorator{}

	if _, ok := d.(decorators.PostRenderer); ok {
		t.Fatal("the report decorator declares a standalone post type; a single-line report is a link")
	}
	if _, ok := d.(decorators.MultiPostRenderer); ok {
		t.Fatal("the report decorator declares a multi-token post type")
	}
	if got := decorators.StandalonePostType(d); got != "" {
		t.Fatalf("StandalonePostType = %q", got)
	}
}

func TestAvReportPostTypeFitsTheColumn(t *testing.T) {
	if len(avreport.PostType) > decorators.PostTypeMaxLen {
		t.Fatalf("%q is %d bytes, over the %d the column holds", avreport.PostType, len(avreport.PostType), decorators.PostTypeMaxLen)
	}
	if !strings.HasPrefix(avreport.PostType, decorators.PostTypePrefix) {
		t.Fatalf("%q does not carry the custom prefix", avreport.PostType)
	}
}

func TestAvReportIsInTheStripTable(t *testing.T) {
	key, ok := stampedPropsKey(avreport.PostType)
	if !ok || key != avreport.PropsKey {
		t.Fatalf("stampedPropsKey(%q) = %q, %v", avreport.PostType, key, ok)
	}
}

func TestTheSwitchIsReadInsideTheRecover(t *testing.T) {
	p := newTestPlugin(t, "https://example.com", true)
	api := p.API.(*fakeAPI)

	updated, stamped := p.runStamper(&model.Post{Message: "x"}, func() bool { panic("switch") }, errcode.HooksAvReportPanic, "tactical-fusion: test panic",
		func(post *model.Post) (*model.Post, bool) { return post, true })

	if stamped || updated != nil {
		t.Fatalf("a panicking switch read escaped the recover: %+v, %v", updated, stamped)
	}
	if len(api.warnCodes) == 0 || api.warnCodes[len(api.warnCodes)-1] != errcode.HooksAvReportPanic {
		t.Fatalf("the recover did not log its code: %v", api.warnCodes)
	}
}
