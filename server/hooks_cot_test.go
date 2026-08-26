package main

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/mattermost/mattermost/server/public/model"

	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/cot"
	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/decorators"
	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/errcode"
)

// Valid Mattermost ids, because cotFileSource now checks the shape of the id and
// that the file belongs to the poster before it reads anything.
const (
	testFileID      = "cotfileaaaaaaaaaaaaaaaaaaa"
	testOtherFileID = "cotfilebbbbbbbbbbbbbbbbbbb"
)

const cotEventXML = `<event version="2.0" uid="ANDROID-1" type="a-f-G-U-C" how="m-g" ` +
	`time="2026-08-23T11:43:38Z" start="2026-08-23T11:43:38Z" stale="2026-08-23T11:45:38Z">` +
	`<point lat="30.009027" lon="-85.957874" hae="-42.6" ce="45.3" le="99.5"/>` +
	`<detail><contact callsign="DELTA1"/></detail></event>`

func cotFence(info, body string) string {
	return "```" + info + "\n" + body + "\n```"
}

func cotBlob(t *testing.T, post *model.Post) map[string]any {
	t.Helper()

	if post == nil {
		t.Fatal("the post was not stamped")
	}
	blob, ok := post.GetProps()[cot.PropsKey].(map[string]any)
	if !ok {
		t.Fatalf("props carry no %s blob: %#v", cot.PropsKey, post.GetProps())
	}
	return blob
}

func TestCotStampsAFencedEvent(t *testing.T) {
	for _, info := range []string{"cot", "xml", "CoT", "XML"} {
		t.Run(info, func(t *testing.T) {
			p := newTestPlugin(t, "https://example.com", true)
			post := &model.Post{Message: cotFence(info, cotEventXML), UserId: testUserID}

			updated := p.decoratePost(post, hookRef)
			if updated == nil {
				t.Fatalf("a %s fence carrying a valid event was not stamped", info)
			}
			if updated.Type != cot.PostType {
				t.Errorf("Type = %q, want %q", updated.Type, cot.PostType)
			}

			blob := cotBlob(t, updated)
			if blob["source"] != cot.SourceFence {
				t.Errorf("source = %v, want %q", blob["source"], cot.SourceFence)
			}
			if blob["src"] != cotEventXML {
				t.Error("the stored source does not match the fence body")
			}
		})
	}
}

func TestCotLeavesTheMessageExactlyAsWritten(t *testing.T) {
	p := newTestPlugin(t, "https://example.com", true)
	message := "latest PLI\n" + cotFence("cot", cotEventXML) + "\nfrom ALPHA"

	updated := p.decoratePost(&model.Post{Message: message, UserId: testUserID}, hookRef)
	if updated == nil {
		t.Fatal("the post was not stamped")
	}
	if updated.Message != message {
		t.Errorf("the stored message was rewritten:\n got %q\nwant %q", updated.Message, message)
	}

	blob := cotBlob(t, updated)
	if blob["lead"] != "latest PLI\n" {
		t.Errorf("lead = %q", blob["lead"])
	}
	if blob["trail"] != "\nfrom ALPHA" {
		t.Errorf("trail = %q", blob["trail"])
	}
}

// A CoT post is never decorated. The card renders lead and trail as plain text,
// so a decorator link written into either would reach the reader as literal
// markdown.
func TestCotSuppressesDecorationOfTheTextAroundIt(t *testing.T) {
	p := newTestPlugin(t, "https://example.com", true)
	message := "target at 34.0561,-118.2500 now\n" + cotFence("cot", cotEventXML)

	updated := p.decoratePost(&model.Post{Message: message, UserId: testUserID}, hookRef)
	if updated == nil {
		t.Fatal("the post was not stamped")
	}
	if updated.Message != message {
		t.Errorf("the coordinate beside the event was decorated:\n%q", updated.Message)
	}
	if strings.Contains(updated.Message, "/decorate/") {
		t.Error("a decorator link reached a post whose body the card owns")
	}

	blob := cotBlob(t, updated)
	if !strings.Contains(blob["lead"].(string), "34.0561,-118.2500") {
		t.Errorf("the coordinate did not survive verbatim in lead: %q", blob["lead"])
	}
}

func TestAMessageWithNoEventIsStillDecorated(t *testing.T) {
	p := newTestPlugin(t, "https://example.com", true)

	updated := p.decoratePost(&model.Post{Message: "target at 34.0561,-118.2500", UserId: testUserID}, hookRef)
	if updated == nil {
		t.Fatal("an ordinary coordinate post was not decorated")
	}
	if updated.Type != "" {
		t.Errorf("Type = %q, want empty; only a CoT post is stamped", updated.Type)
	}
	if !strings.Contains(updated.Message, "/decorate/location") {
		t.Errorf("the coordinate was not decorated: %q", updated.Message)
	}
}

func TestCotRefusesWhatItCannotRead(t *testing.T) {
	cases := map[string]string{
		"not xml at all":     cotFence("xml", "just words"),
		"not a cot event":    cotFence("xml", "<note><to>you</to></note>"),
		"no uid":             cotFence("cot", `<event type="a-f" time="2026-08-23T11:43:38Z"/>`),
		"doctype":            cotFence("cot", `<!DOCTYPE event []><event uid="u" type="a-f" time="t"/>`),
		"two fences":         cotFence("cot", cotEventXML) + "\n" + cotFence("cot", cotEventXML),
		"unterminated fence": "```cot\n" + cotEventXML,
		"wrong info string":  cotFence("json", cotEventXML),
	}

	for name, message := range cases {
		t.Run(name, func(t *testing.T) {
			p := newTestPlugin(t, "https://example.com", true)

			updated := p.decoratePost(&model.Post{Message: message, UserId: testUserID}, hookRef)
			if updated != nil && updated.Type == cot.PostType {
				t.Errorf("a post was stamped for %s", name)
			}
		})
	}
}

func TestCotLeavesAnotherIntegrationsPostAlone(t *testing.T) {
	p := newTestPlugin(t, "https://example.com", true)
	post := &model.Post{
		Message: cotFence("cot", cotEventXML),
		Type:    "custom_something_else",
		UserId:  testUserID,
	}

	if updated := p.decoratePost(post, hookRef); updated != nil {
		t.Errorf("a post already carrying a custom type was rewritten to %q", updated.Type)
	}
}

func TestCotIsSilentWhenTheAdminTurnedItOff(t *testing.T) {
	p := newTestPlugin(t, "https://example.com", true)
	config := *p.getConfiguration()
	config.EnableCot = false
	p.setConfiguration(&config)

	post := &model.Post{Message: cotFence("cot", cotEventXML), UserId: testUserID}
	if updated := p.decoratePost(post, hookRef); updated != nil && updated.Type == cot.PostType {
		t.Error("a post was stamped with the feature switched off")
	}

	api := p.API.(*fakeAPI)
	if len(api.ephemeral) != 0 {
		t.Error("an author was told about a setting that is not theirs to change")
	}
}

func TestCotKeepsAnotherIntegrationsProps(t *testing.T) {
	p := newTestPlugin(t, "https://example.com", true)
	post := &model.Post{Message: cotFence("cot", cotEventXML), UserId: testUserID}
	post.AddProp("from_webhook", "true")

	updated := p.decoratePost(post, hookRef)
	if updated == nil {
		t.Fatal("the post was not stamped")
	}
	if updated.GetProps()["from_webhook"] != "true" {
		t.Error("stamping discarded another integration's props")
	}
	if _, ok := updated.GetProps()[cot.PropsKey]; !ok {
		t.Error("the event blob is missing")
	}
}

func TestCotRefusesRatherThanRiskingAPostTheServerWouldReject(t *testing.T) {
	p := newTestPlugin(t, "https://example.com", true)

	post := &model.Post{Message: cotFence("cot", cotEventXML), UserId: testUserID}
	post.AddProp("bulky", strings.Repeat("x", model.PostPropsMaxUserRunes))

	updated := p.decoratePost(post, hookRef)
	if updated != nil && updated.Type == cot.PostType {
		t.Fatal("the post was stamped past the props budget, which the server would refuse")
	}

	api := p.API.(*fakeAPI)
	if len(api.warnings) == 0 {
		t.Error("nothing was logged about the refusal")
	}
}

func TestAnEventBesideAnOrdinaryPropsBlobStillPosts(t *testing.T) {
	p := newTestPlugin(t, "https://example.com", true)

	post := &model.Post{Message: cotFence("cot", cotEventXML), UserId: testUserID}
	post.AddProp("attachments", strings.Repeat("y", 4096))

	updated := p.decoratePost(post, hookRef)
	if updated == nil || updated.Type != cot.PostType {
		t.Fatal("an ordinary foreign props blob blocked stamping")
	}
}

func TestCotReadsASoleAttachment(t *testing.T) {
	p := newTestPlugin(t, "https://example.com", true)
	api := p.API.(*fakeAPI)
	api.files = map[string]*model.FileInfo{
		testFileID: {Id: testFileID, CreatorId: testUserID, Name: "event.cot", Size: int64(len(cotEventXML))},
	}
	api.fileContent = map[string][]byte{testFileID: []byte(cotEventXML)}

	post := &model.Post{Message: "", FileIds: model.StringArray{testFileID}, UserId: testUserID}

	updated := p.decoratePost(post, hookRef)
	if updated == nil {
		t.Fatal("a sole .cot attachment was not stamped")
	}

	blob := cotBlob(t, updated)
	if blob["source"] != cot.SourceFile {
		t.Errorf("source = %v, want %q", blob["source"], cot.SourceFile)
	}
	if blob["file_id"] != testFileID || blob["file_name"] != "event.cot" {
		t.Errorf("the file is not named in the props: %#v", blob)
	}
}

func TestCotReadsAnAttachmentBesideACoveringNote(t *testing.T) {
	p := newTestPlugin(t, "https://example.com", true)
	api := p.API.(*fakeAPI)
	api.files = map[string]*model.FileInfo{
		testFileID: {Id: testFileID, CreatorId: testUserID, Name: "event.xml", Size: int64(len(cotEventXML))},
	}
	api.fileContent = map[string][]byte{testFileID: []byte(cotEventXML)}

	post := &model.Post{Message: "latest PLI", FileIds: model.StringArray{testFileID}, UserId: testUserID}

	updated := p.decoratePost(post, hookRef)
	if updated == nil {
		t.Fatal("an attachment with a covering note was not stamped")
	}
	if blob := cotBlob(t, updated); blob["lead"] != "latest PLI" {
		t.Errorf("lead = %v, want the covering note", blob["lead"])
	}
}

func TestCotSurvivesAFilestoreThatWillNotAnswer(t *testing.T) {
	cases := map[string]func(*fakeAPI){
		"GetFileInfo fails": func(a *fakeAPI) { a.fileInfoErr = &model.AppError{Message: "down"} },
		"GetFile fails": func(a *fakeAPI) {
			a.files = map[string]*model.FileInfo{testFileID: {Id: testFileID, CreatorId: testUserID, Name: "event.cot", Size: 10}}
			a.fileErr = &model.AppError{Message: "down"}
		},
		"file too large": func(a *fakeAPI) {
			a.files = map[string]*model.FileInfo{testFileID: {Id: testFileID, CreatorId: testUserID, Name: "event.cot", Size: cot.MaxSourceBytes + 1}}
		},
		"wrong extension": func(a *fakeAPI) {
			a.files = map[string]*model.FileInfo{testFileID: {Id: testFileID, CreatorId: testUserID, Name: "photo.jpg", Size: 10}}
		},
	}

	for name, setup := range cases {
		t.Run(name, func(t *testing.T) {
			p := newTestPlugin(t, "https://example.com", true)
			setup(p.API.(*fakeAPI))

			post := &model.Post{Message: "", FileIds: model.StringArray{testFileID}, UserId: testUserID}

			updated := p.decoratePost(post, hookRef)
			if updated != nil && updated.Type == cot.PostType {
				t.Errorf("a post was stamped despite %s", name)
			}
		})
	}
}

func TestCotIgnoresAttachmentsWhenTheFileSwitchIsOff(t *testing.T) {
	p := newTestPlugin(t, "https://example.com", true)
	config := *p.getConfiguration()
	config.EnableCotFile = false
	p.setConfiguration(&config)

	api := p.API.(*fakeAPI)
	api.files = map[string]*model.FileInfo{testFileID: {Id: testFileID, CreatorId: testUserID, Name: "event.cot", Size: 10}}
	api.fileContent = map[string][]byte{testFileID: []byte(cotEventXML)}

	post := &model.Post{Message: "", FileIds: model.StringArray{testFileID}, UserId: testUserID}
	if updated := p.decoratePost(post, hookRef); updated != nil {
		t.Error("an attachment was read with the file switch off")
	}
}

func TestCotIgnoresMoreThanOneAttachment(t *testing.T) {
	p := newTestPlugin(t, "https://example.com", true)

	post := &model.Post{Message: "", FileIds: model.StringArray{testFileID, testOtherFileID}, UserId: testUserID}
	if updated := p.decoratePost(post, hookRef); updated != nil {
		t.Error("a post with two attachments was stamped")
	}
}

func TestTheFenceWinsWhenAPostCarriesBoth(t *testing.T) {
	p := newTestPlugin(t, "https://example.com", true)
	api := p.API.(*fakeAPI)
	api.files = map[string]*model.FileInfo{testFileID: {Id: testFileID, CreatorId: testUserID, Name: "other.cot", Size: 10}}
	api.fileContent = map[string][]byte{testFileID: []byte(cotEventXML)}

	post := &model.Post{
		Message: cotFence("cot", cotEventXML),
		FileIds: model.StringArray{testFileID},
		UserId:  testUserID,
	}

	updated := p.decoratePost(post, hookRef)
	if updated == nil {
		t.Fatal("the post was not stamped")
	}
	if blob := cotBlob(t, updated); blob["source"] != cot.SourceFence {
		t.Errorf("source = %v, want the visible fence to win", blob["source"])
	}
}

func TestAnExplicitCotFenceThatFailsTellsItsAuthor(t *testing.T) {
	p := newTestPlugin(t, "https://example.com", true)
	post := &model.Post{Message: cotFence("cot", "<event/>"), UserId: testUserID, ChannelId: "c1"}

	p.decoratePost(post, hookRef)

	api := p.API.(*fakeAPI)
	if len(api.ephemeral) != 1 {
		t.Fatalf("the author was told nothing; %d ephemeral posts", len(api.ephemeral))
	}
	if !strings.Contains(api.ephemeral[0].Message, "TF-11005") {
		t.Errorf("the notice carries no error code: %q", api.ephemeral[0].Message)
	}
	if strings.Contains(api.ephemeral[0].Message, "this post") {
		t.Error("the notice points at a post it cannot identify")
	}
}

func TestAnXMLFenceThatFailsIsSilent(t *testing.T) {
	p := newTestPlugin(t, "https://example.com", true)
	post := &model.Post{Message: cotFence("xml", "<note>hello</note>"), UserId: testUserID}

	p.decoratePost(post, hookRef)

	if api := p.API.(*fakeAPI); len(api.ephemeral) != 0 {
		t.Error("an ambiguous xml fence produced a notice")
	}
}

func TestCotNeverCommitsAHalfStamp(t *testing.T) {
	p := newTestPlugin(t, "https://example.com", true)
	post := &model.Post{Message: cotFence("cot", cotEventXML), UserId: testUserID}

	updated := p.decoratePost(post, hookRef)
	if updated == nil {
		t.Fatal("the post was not stamped")
	}

	stampedType := updated.Type == cot.PostType
	_, stampedProps := updated.GetProps()[cot.PropsKey]
	if stampedType != stampedProps {
		t.Errorf("half a stamp landed: type=%v props=%v", stampedType, stampedProps)
	}

	if post.Type != "" {
		t.Error("the caller's post was mutated rather than a clone")
	}
	if _, ok := post.GetProps()[cot.PropsKey]; ok {
		t.Error("the caller's post received props rather than a clone")
	}
}

func TestCotSurvivesAPanicWithoutCostingTheDecoration(t *testing.T) {
	p := newTestPlugin(t, "https://example.com", true)
	p.SetAPI(nil)

	post := &model.Post{Message: cotFence("cot", cotEventXML), UserId: testUserID}

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("a panic escaped the hook: %v", r)
		}
	}()

	p.MessageWillBePosted(nil, post)
}

func TestCotAndDecorationAreExclusive(t *testing.T) {
	p := newTestPlugin(t, "https://example.com", true)
	message := "DTG: 091630ZAUG26 and 34.0561,-118.2500\n" + cotFence("cot", cotEventXML)

	updated := p.decoratePost(&model.Post{Message: message, UserId: testUserID}, hookRef)
	if updated == nil {
		t.Fatal("the post was not stamped")
	}
	if updated.Message != message {
		t.Errorf("decoration ran on a stamped post:\n%q", updated.Message)
	}
	if _, ok := updated.GetProps()[decorators.PostPropsKey]; ok {
		t.Error("a decorator payload was written beside the event blob")
	}
}

func TestTheHookStillAllowsAPostItCannotHandle(t *testing.T) {
	p := newTestPlugin(t, "https://example.com", true)

	post, reason := p.MessageWillBePosted(nil, &model.Post{Message: cotFence("cot", "<event/>"), UserId: testUserID})
	if reason != "" {
		t.Errorf("the hook refused a post: %q", reason)
	}
	_ = post
}

// Post.IsValid accepts any custom_ type from an ordinary client, so anyone who
// can post could otherwise hand a reader a card whose rows and whose own XML
// pane were both authored to agree with each other, which is exactly what a
// reader opens the pane to rule out.
func TestAForgedPostTypeIsStripped(t *testing.T) {
	p := newTestPlugin(t, "https://example.com", true)

	post := &model.Post{Message: "nothing to see", Type: cot.PostType, UserId: testUserID}
	post.AddProp(cot.PropsKey, map[string]any{
		"version": 1,
		"source":  "fence",
		"src":     `<event uid="X" type="a-h-G-U-C"/>`,
		"event":   map[string]any{"uid": "X", "affiliation": "friend", "type_label": "Friend ground"},
	})

	updated := p.decoratePost(post, hookRef)
	if updated == nil {
		t.Fatal("a forged post was left exactly as it arrived")
	}
	if updated.Type != "" {
		t.Errorf("Type = %q, want the forged type stripped", updated.Type)
	}
	if _, ok := updated.GetProps()[cot.PropsKey]; ok {
		t.Error("the forged event blob survived")
	}
	if updated.Message != "nothing to see" {
		t.Errorf("the message was altered: %q", updated.Message)
	}
}

// Stripping must not cost a real event its card: the message is re-read and
// re-stamped on its own merits.
func TestAForgedTypeOverARealEventIsRestamped(t *testing.T) {
	p := newTestPlugin(t, "https://example.com", true)

	post := &model.Post{Message: cotFence("cot", cotEventXML), Type: cot.PostType, UserId: testUserID}
	post.AddProp(cot.PropsKey, map[string]any{"version": 1, "source": "fence", "event": map[string]any{"uid": "LIE"}})

	updated := p.decoratePost(post, hookRef)
	if updated == nil || updated.Type != cot.PostType {
		t.Fatal("a real event lost its card because the type arrived set")
	}

	blob := cotBlob(t, updated)
	rendered, _ := blob["events"].([]any)
	if len(rendered) != 1 {
		t.Fatalf("blob carries %d events, want 1", len(rendered))
	}
	inner, _ := rendered[0].(map[string]any)
	if inner["uid"] != "ANDROID-1" {
		t.Errorf("uid = %v, want the value re-read from the message", inner["uid"])
	}
}

// Stamping is the plugin's alone, but another integration's custom type is real
// mission content and must survive untouched.
func TestAnotherIntegrationsTypeIsStillLeftAlone(t *testing.T) {
	p := newTestPlugin(t, "https://example.com", true)

	post := &model.Post{Message: cotFence("cot", cotEventXML), Type: "custom_something", UserId: testUserID}
	if updated := p.decoratePost(post, hookRef); updated != nil {
		t.Errorf("a foreign custom type was rewritten to %q", updated.Type)
	}
}

// The recover has to be entered, not merely present. Nothing else on the CoT
// path can be made to panic from a test, so the filestore is the injection
// point, and this is the only test that proves the deferred function runs.
func TestCotRecoversFromARealPanic(t *testing.T) {
	p := newTestPlugin(t, "https://example.com", true)
	api := p.API.(*fakeAPI)
	api.panicOnFileInfo = true

	post := &model.Post{
		Message: "target at 34.0561,-118.2500",
		FileIds: model.StringArray{testFileID},
		UserId:  testUserID,
	}

	updated := p.decoratePost(post, hookRef)

	if len(api.warnings) == 0 {
		t.Fatal("the recover did not run; nothing was logged")
	}
	if post.Type != "" {
		t.Error("the caller's post was left stamped after a panic")
	}

	// Decoration still gets its turn, which is the whole reason CoT recovers
	// separately rather than sharing decoratePost's.
	if updated == nil || !strings.Contains(updated.Message, "/decorate/location") {
		t.Error("a panic in recognition cost the author their decoration")
	}
}

// A panic after the forged type was stripped must still hand back the stripped
// post. Returning nil there would put the forgery back.
func TestAPanicAfterStrippingStillReturnsTheStrippedPost(t *testing.T) {
	p := newTestPlugin(t, "https://example.com", true)
	api := p.API.(*fakeAPI)
	api.panicOnFileInfo = true

	post := &model.Post{
		Message: "",
		Type:    cot.PostType,
		FileIds: model.StringArray{testFileID},
		UserId:  testUserID,
	}
	post.AddProp(cot.PropsKey, map[string]any{"version": 1, "source": "fence", "event": map[string]any{"uid": "LIE"}})

	updated := p.decoratePost(post, hookRef)
	if updated == nil {
		t.Fatal("a panic put the forged post back")
	}
	if updated.Type != "" {
		t.Errorf("Type = %q, want the forged type still stripped after a panic", updated.Type)
	}
	if _, ok := updated.GetProps()[cot.PropsKey]; ok {
		t.Error("the forged blob survived a panic")
	}
}

// A bare event, with no fence around it.
func TestCotStampsABareEvent(t *testing.T) {
	p := newTestPlugin(t, "https://example.com", true)
	message := "latest PLI " + cotEventXML + " out"

	updated := p.decoratePost(&model.Post{Message: message, UserId: testUserID}, hookRef)
	if updated == nil {
		t.Fatal("a bare event was not stamped")
	}
	if updated.Message != message {
		t.Errorf("the stored message was rewritten:\n%q", updated.Message)
	}

	blob := cotBlob(t, updated)
	if blob["lead"] != "latest PLI " || blob["trail"] != " out" {
		t.Errorf("lead = %v, trail = %v", blob["lead"], blob["trail"])
	}
}

// An author who fenced an event without labelling the fence has said it is
// code. Reading it anyway would be the corruption protected ranges exist to
// stop, and it is the one thing the bare scan must not do.
func TestABareScanNeverReachesIntoCode(t *testing.T) {
	cases := map[string]string{
		"unlabelled fence": "```\n" + cotEventXML + "\n```",
		"inline code":      "`" + cotEventXML + "`",
		"indented code":    "    " + cotEventXML,
	}

	for name, message := range cases {
		t.Run(name, func(t *testing.T) {
			p := newTestPlugin(t, "https://example.com", true)

			updated := p.decoratePost(&model.Post{Message: message, UserId: testUserID}, hookRef)
			if updated != nil && updated.Type == cot.PostType {
				t.Errorf("a post was stamped from %s", name)
			}
		})
	}
}

// A fence around an event is read as a fence, so the info string still decides.
func TestAFencedEventIsStillReadAsAFence(t *testing.T) {
	p := newTestPlugin(t, "https://example.com", true)

	updated := p.decoratePost(&model.Post{Message: cotFence("cot", cotEventXML), UserId: testUserID}, hookRef)
	if updated == nil || updated.Type != cot.PostType {
		t.Fatal("a labelled fence stopped being read")
	}

	if blob := cotBlob(t, updated); blob["lead"] != "" {
		t.Errorf("lead = %v; the fence markers leaked into it", blob["lead"])
	}
}

func TestABareEventThatIsNotOneIsLeftAlone(t *testing.T) {
	p := newTestPlugin(t, "https://example.com", true)

	for _, message := range []string{
		"the <eventual> plan",
		"<event>nothing useful</event>",
		"see <events><a/></events>",
	} {
		updated := p.decoratePost(&model.Post{Message: message, UserId: testUserID}, hookRef)
		if updated != nil && updated.Type == cot.PostType {
			t.Errorf("a post was stamped from %q", message)
		}
	}
}

func TestCotStampsSeveralEventsInOneBlock(t *testing.T) {
	p := newTestPlugin(t, "https://example.com", true)

	body := `<event uid="a" type="a-f-G-U-C" time="2026-08-23T11:43:38Z">` +
		`<point lat="30.009027" lon="-85.957874"/></event>` +
		`<event uid="b" type="a-h-A-M-F" time="2026-08-23T11:44:00Z">` +
		`<point lat="31.009027" lon="-86.957874"/></event>`

	updated := p.decoratePost(&model.Post{Message: cotFence("cot", body), UserId: testUserID}, hookRef)
	if updated == nil {
		t.Fatal("a two-event block was not stamped")
	}

	blob := cotBlob(t, updated)
	rendered, ok := blob["events"].([]any)
	if !ok || len(rendered) != 2 {
		t.Fatalf("blob carries %v, want two events", blob["events"])
	}
	if rendered[0].(map[string]any)["uid"] != "a" {
		t.Error("the events are out of order")
	}
}

// A forged type is stripped AND the message underneath it still decorates.
//
// The strip used to end the post's journey: cotStamp returned the stripped
// clone, decoratePost treated any non-nil answer as final, and the message went
// out undecorated. Spoofing a type on a post was therefore a way to turn
// decoration off for it.
func TestAStrippedPostStillReachesTheDecorators(t *testing.T) {
	p := newTestPlugin(t, "https://example.com", true)

	post := &model.Post{Message: "meet at 21.3353, -157.9483", Type: cot.PostType, UserId: testUserID}
	post.AddProp(cot.PropsKey, map[string]any{"version": 1})

	updated := p.decoratePost(post, hookRef)
	if updated == nil {
		t.Fatal("the forged post came back unchanged, so the type was not stripped")
	}
	if updated.Type != "" {
		t.Errorf("Type = %q, want the forged type stripped", updated.Type)
	}
	if !strings.Contains(updated.Message, "](") {
		t.Errorf("the coordinate was not decorated:\n%s", updated.Message)
	}
}

// The file the hook reads has to be the poster's own.
//
// post.FileIds arrives from the client and this hook runs before Mattermost
// binds files to posts, so nothing else has checked that the sender may read
// the id they named. Quoting somebody else's file id would copy that file's
// text into props anyone who can read the attacker's post can read.
func TestCotRefusesAnAttachmentThatIsNotThePostersOwn(t *testing.T) {
	for _, tc := range []struct {
		name string
		info *model.FileInfo
	}{
		{"another user's file", &model.FileInfo{Id: testFileID, CreatorId: "someone-else", Name: "event.cot"}},
		{"a file already on a post", &model.FileInfo{Id: testFileID, CreatorId: testUserID, PostId: "p9", Name: "event.cot"}},
		{"a deleted file", &model.FileInfo{Id: testFileID, CreatorId: testUserID, DeleteAt: 1, Name: "event.cot"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := newTestPlugin(t, "https://example.com", true)
			api := p.API.(*fakeAPI)
			tc.info.Size = int64(len(cotEventXML))
			api.files = map[string]*model.FileInfo{testFileID: tc.info}
			api.fileContent = map[string][]byte{testFileID: []byte(cotEventXML)}

			post := &model.Post{Message: "", FileIds: model.StringArray{testFileID}, UserId: testUserID}
			if updated, stamped := p.cotStamp(post); stamped {
				t.Errorf("the attachment was read anyway: %v", updated.GetProps()[cot.PropsKey])
			}

			// The CODE, not just that something was logged. This refusal and a
			// filestore outage both write one warning, and the code is the only
			// thing that tells an operator which happened. Asserting only that
			// warnings is non-empty is what let this call site keep the
			// unreadable code after it had stopped meaning that.
			if !slices.Contains(api.warnCodes, errcode.HooksCotFileNotOwned) {
				t.Errorf("logged %v, want a %d saying the file is not the poster's",
					api.warnCodes, errcode.HooksCotFileNotOwned)
			}
			if slices.Contains(api.warnCodes, errcode.HooksCotFileUnreadable) {
				t.Error("the refusal was filed as a filestore failure, which it is not")
			}
		})
	}
}

// A file an integration uploaded is read, even though nobody owns it.
//
// plugin.API.UploadFile takes no user id, so the server credits those to
// model.UploadNoUserID and they can never equal a post's author. Refusing them
// silently ignored every attachment a companion plugin posted, which for a
// Cursor on Target plugin is the likeliest producer of .cot files there is.
func TestCotReadsAnAttachmentUploadedByAPlugin(t *testing.T) {
	p := newTestPlugin(t, "https://example.com", true)
	api := p.API.(*fakeAPI)

	api.files = map[string]*model.FileInfo{
		testFileID: {
			Id:        testFileID,
			CreatorId: model.UploadNoUserID,
			Name:      "event.cot",
			Size:      int64(len(cotEventXML)),
		},
	}
	api.fileContent = map[string][]byte{testFileID: []byte(cotEventXML)}

	post := &model.Post{Message: "", FileIds: model.StringArray{testFileID}, UserId: testUserID}
	if _, stamped := p.cotStamp(post); !stamped {
		t.Fatalf("a plugin-uploaded attachment was refused: %v", api.warnCodes)
	}
}

// A malformed file id is refused before the filestore is asked anything.
func TestCotRefusesAFileIdThatIsNotOne(t *testing.T) {
	p := newTestPlugin(t, "https://example.com", true)
	api := p.API.(*fakeAPI)

	post := &model.Post{Message: "", FileIds: model.StringArray{"f1"}, UserId: testUserID}
	if _, stamped := p.cotStamp(post); stamped {
		t.Error("a post naming a malformed file id was stamped")
	}
	if api.fileInfoCalls != 0 {
		t.Errorf("the filestore was asked about a malformed id %d times", api.fileInfoCalls)
	}
}

// The shape that actually decides whether a real post drops to raw XML: every
// event this build reads, each carrying every registry entry, with the author's
// own notes at their cap on both sides of the fence.
//
// A test built on ONE maximal event measures a case that was never in doubt.
func TestABatchOfMaximalEventsStillStamps(t *testing.T) {
	p := newTestPlugin(t, "https://example.com", true)

	source, count := maximalBatch(t)

	note := strings.Repeat("n", 65536)
	post := &model.Post{
		Message: note + "\n" + cotFence("cot", source) + "\n" + note,
		UserId:  testUserID,
	}

	updated, stamped := p.cotStamp(post)
	if !stamped {
		t.Fatal("a batch of maximal events was left unstamped; a reader meets raw XML where a card belongs")
	}

	blob, ok := updated.GetProps()[cot.PropsKey].(map[string]any)
	if !ok {
		t.Fatal("the event blob is missing")
	}
	rendered, ok := blob["events"].([]any)
	if !ok || len(rendered) != count {
		t.Fatalf("the blob carries %v events, want %d", blob["events"], count)
	}

	// The widest rung has to be the one that ran. Without this the two rungs
	// could be swapped, or the first deleted, and every test here would still
	// pass while every stamped post silently lost its extension keys.
	first, ok := rendered[0].(map[string]any)
	if !ok {
		t.Fatal("the first event is not a props map")
	}
	for _, key := range []string{"takv_platform", "status_battery", "flow"} {
		if _, held := first[key]; !held {
			t.Errorf("a post that fits was stamped without %q, so the ladder took the wrong rung", key)
		}
	}
	if _, held := first["detail_dropped"]; held {
		t.Error("a post that fits was marked as degraded")
	}
}

// Over budget, the extension keys go and the card stays. Everything version 2
// ever wrote survives, so the degraded card is exactly the card this feature
// shipped with rather than a fall all the way back to raw XML.
func TestOverBudgetTheDetailIsDroppedBeforeTheCardIs(t *testing.T) {
	p := newTestPlugin(t, "https://example.com", true)

	events, _ := maximalBatch(t)

	post := &model.Post{Message: cotFence("cot", events), UserId: testUserID}

	// Sized so the full blob cannot fit and the degraded one can. Measured
	// rather than guessed, since both blobs move as the registry grows.
	full := len(mustJSON(t, cot.Props(mustParse(t, events), cot.Source{Kind: cot.SourceFence, Text: events})))
	lean := len(mustJSON(t, cot.PropsWithoutDetail(mustParse(t, events), cot.Source{Kind: cot.SourceFence, Text: events})))
	if lean >= full {
		t.Fatal("dropping the detail did not make the blob smaller")
	}
	post.AddProp("bulky", strings.Repeat("x", model.PostPropsMaxUserRunes-((full+lean)/2)))

	updated, stamped := p.cotStamp(post)
	if !stamped {
		t.Fatal("the post was refused rather than degraded")
	}

	blob := updated.GetProps()[cot.PropsKey].(map[string]any)
	first := blob["events"].([]any)[0].(map[string]any)

	if _, held := first["takv_platform"]; held {
		t.Error("the degraded blob still carries extension keys")
	}
	if first["callsign"] != "DELTA1" {
		t.Errorf("callsign is %v; the degraded card keeps everything version 2 wrote", first["callsign"])
	}

	api := p.API.(*fakeAPI)
	if !slices.Contains(api.warnCodes, errcode.HooksCotDetailDropped) {
		t.Errorf("the degraded stamp was not logged under TF-%d", errcode.HooksCotDetailDropped)
	}
}

// The largest shape that can actually reach this hook, which is bounded by the
// source cap rather than by the vertex cap.
//
// What actually bounds a shape is the ELEMENT budget, not the byte cap.
//
// A vertex costs three units of maxCotElements (itself plus two attributes), so
// roughly 1360 vertices exhaust it while the same vertices are only about 53 KB
// of a 64 KiB allowance. The vertex cap sits below both, at 512, so it is
// reachable and meaningful rather than shadowed. Worth pinning, because the
// arithmetic that suggests otherwise is easy to redo and get wrong.
func TestAShapeIsBoundedByTheElementBudgetNotTheByteCap(t *testing.T) {
	source := func(n int) string {
		var vertices strings.Builder
		for i := range n {
			fmt.Fprintf(&vertices, `<vertex lat="34.%04d" lon="-118.%04d"/>`, i%10000, i%10000)
		}
		return fmt.Sprintf(`<event version="2.0" uid="SHAPE-1" type="u-d-f" how="h-g" `+
			`time="2026-08-23T11:43:38Z" stale="2026-08-23T11:45:38Z">`+
			`<point lat="34.056100" lon="-118.250000" ce="45.3"/>`+
			`<detail><contact callsign="AREA1"/><shape><polyline closed="true">%s`+
			`</polyline></shape></detail></event>`, vertices.String())
	}

	drawable := source(cot.MaxVertices)
	if len(drawable) > cot.MaxSourceBytes {
		t.Fatalf("a shape at the vertex cap is %d bytes, past the %d source cap",
			len(drawable), cot.MaxSourceBytes)
	}

	p := newTestPlugin(t, "https://example.com", true)
	updated, stamped := p.cotStamp(&model.Post{Message: cotFence("cot", drawable), UserId: testUserID})
	if !stamped {
		t.Fatal("a shape at the vertex cap was left unstamped")
	}

	blob := updated.GetProps()[cot.PropsKey].(map[string]any)
	geometry := blob["events"].([]any)[0].(map[string]any)["geometry"].(map[string]any)
	if geometry["count"] != fmt.Sprint(cot.MaxVertices) {
		t.Errorf("count is %v, want %d drawn", geometry["count"], cot.MaxVertices)
	}

	// One past it and the shape is not drawn, while the event keeps everything
	// else it stated.
	over := newTestPlugin(t, "https://example.com", true)
	updated, stamped = over.cotStamp(&model.Post{Message: cotFence("cot", source(cot.MaxVertices+1)), UserId: testUserID})
	if !stamped {
		t.Fatal("a shape past the vertex cap cost the event its card")
	}

	blob = updated.GetProps()[cot.PropsKey].(map[string]any)
	first := blob["events"].([]any)[0].(map[string]any)
	if first["callsign"] != "AREA1" {
		t.Errorf("callsign is %v; the event was lost over its geometry", first["callsign"])
	}
	geometry = first["geometry"].(map[string]any)
	if _, drawn := geometry["points"]; drawn {
		t.Error("a shape past the cap was drawn, ending where the shape does not")
	}
	if geometry["note"] == nil {
		t.Error("an undrawn shape says nothing about why")
	}
}

// maximalBatch is as many events carrying every registry entry as a source can
// hold, computed rather than assumed.
//
// The registry outgrew MaxEvents: one maximal event is about two kilobytes, so
// thirty-two are past the 64 KiB source cap and Parse refuses the batch before
// the budget ladder ever sees it. Hard-coding MaxEvents measured a post nobody
// can make, and the number falls again every time an extension is added.
func maximalBatch(t *testing.T) (string, int) {
	t.Helper()

	var events strings.Builder
	count := 0

	for i := range cot.MaxEvents {
		event := fmt.Sprintf(`<event version="2.0" uid="UID-%d" type="a-f-G-U-C-I" how="m-g" `+
			`time="2026-08-23T11:43:38Z" start="2026-08-23T11:43:38Z" stale="2026-08-23T11:45:38Z">`+
			`<point lat="34.056100" lon="-118.250000" hae="-42.6" ce="45.3" le="99.5"/>`+
			`<detail>%s</detail></event>`, i, cot.FixtureDetail())

		if events.Len()+len(event) > cot.MaxSourceBytes {
			break
		}
		events.WriteString(event)
		count++
	}

	if count < 2 {
		t.Fatalf("only %d maximal events fit a source; the fixture has outgrown a batch entirely", count)
	}

	return events.String(), count
}

func mustParse(t *testing.T, source string) []cot.Event {
	t.Helper()

	events, err := cot.Parse([]byte(source))
	if err != nil {
		t.Fatalf("fixture did not parse: %v", err)
	}
	return events
}

func mustJSON(t *testing.T, blob map[string]any) []byte {
	t.Helper()

	encoded, err := json.Marshal(blob)
	if err != nil {
		t.Fatalf("blob did not marshal: %v", err)
	}
	return encoded
}

// A props value that will not marshal stops the ladder, rather than falling
// through to a rung that would stamp a map nobody could measure.
//
// The size gate exists to stop the server refusing the post outright, and an
// unmeasurable map is exactly the input that gate cannot see. It also must not
// be reported as a size failure: the offending value came from elsewhere on the
// post and no rung of this ladder can shed it.
func TestPropsThatWillNotMarshalStopTheLadder(t *testing.T) {
	p := newTestPlugin(t, "https://example.com", true)

	post := &model.Post{Message: cotFence("cot", cotEventXML), UserId: testUserID}
	post.AddProp("unmarshalable", make(chan int))

	updated, stamped := p.cotStamp(post)
	if stamped {
		t.Fatal("a post whose props cannot be measured was stamped anyway")
	}
	if updated != nil && updated.Type == cot.PostType {
		t.Error("the post wears the card type after a refusal")
	}

	api := p.API.(*fakeAPI)
	if !slices.Contains(api.warnCodes, errcode.HooksCotPropsUnmeasurable) {
		t.Errorf("warn codes are %v, want TF-%d", api.warnCodes, errcode.HooksCotPropsUnmeasurable)
	}
	if slices.Contains(api.warnCodes, errcode.HooksCotPropsTooLarge) {
		t.Error("an unmeasurable props map was reported to the author as a size failure")
	}
}

// The claim the refusal makes: the value that would not marshal cannot have
// come from this package, so telling the author to post a smaller event would
// be wrong whatever the size.
func TestEveryBlobThisPackageWritesCanBeMarshalled(t *testing.T) {
	events := mustParse(t, cotEventXML)
	source := cot.Source{Kind: cot.SourceFence, Text: cotEventXML}

	for name, blob := range map[string]map[string]any{
		"full":     cot.Props(events, source),
		"degraded": cot.PropsWithoutDetail(events, source),
	} {
		if _, err := json.Marshal(blob); err != nil {
			t.Errorf("the %s blob does not marshal: %v", name, err)
		}
	}
}
