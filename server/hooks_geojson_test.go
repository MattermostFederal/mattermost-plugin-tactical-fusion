package main

import (
	"encoding/json"
	"math"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/mattermost/mattermost/server/public/model"

	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/cot"
	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/geojson"
)

const geoFileID = "geofileaaaaaaaaaaaaaaaaaaa"

const geoPoint = `{"type":"Feature","geometry":{"type":"Point","coordinates":[-118.25,34.0561]},` +
	`"properties":{"name":"Depot"}}`

func geoFence(info, body string) string {
	return "```" + info + "\n" + body + "\n```"
}

func geoBlob(t *testing.T, post *model.Post) map[string]any {
	t.Helper()

	if post == nil {
		t.Fatal("the post was not stamped")
	}
	blob, ok := post.GetProps()[geojson.PropsKey].(map[string]any)
	if !ok {
		t.Fatalf("props carry no %s blob: %#v", geojson.PropsKey, post.GetProps())
	}
	return blob
}

func TestGeoJSONStampsAFencedDocument(t *testing.T) {
	for _, info := range []string{"geojson", "GeoJSON", "GEOJSON"} {
		t.Run(info, func(t *testing.T) {
			p := newTestPlugin(t, "https://example.com", true)
			post := &model.Post{Message: geoFence(info, geoPoint), UserId: testUserID}

			updated := p.decoratePost(post, hookRef)
			if updated == nil {
				t.Fatalf("a %s fence carrying a valid document was not stamped", info)
			}
			if updated.Type != geojson.PostType {
				t.Errorf("Type = %q, want %q", updated.Type, geojson.PostType)
			}

			blob := geoBlob(t, updated)
			if blob["source"] != geojson.SourceFence {
				t.Errorf("source = %v, want %q", blob["source"], geojson.SourceFence)
			}
			if blob["src"] != geoPoint {
				t.Error("the stored source does not match the fence body")
			}
		})
	}
}

// TestAJSONFenceIsNeverStamped is the guard on the Phase 1 recognition
// decision. Stamping costs a post its search matches permanently, so widening
// recognition to `json` has to change a test rather than slip through.
func TestAJSONFenceIsNeverStamped(t *testing.T) {
	p := newTestPlugin(t, "https://example.com", true)

	for _, body := range []string{geoPoint, `{"name":"my-package","version":"1.0.0"}`} {
		post := &model.Post{Message: geoFence("json", body), UserId: testUserID}

		if updated := p.decoratePost(post, hookRef); updated != nil && updated.Type != "" {
			t.Fatalf("a json fence was stamped as %q", updated.Type)
		}
	}
}

func TestGeoJSONLeavesTheMessageExactlyAsWritten(t *testing.T) {
	p := newTestPlugin(t, "https://example.com", true)
	message := geoFence("geojson", geoPoint)

	updated := p.decoratePost(&model.Post{Message: message, UserId: testUserID}, hookRef)

	if updated.Message != message {
		t.Fatalf("the message was rewritten:\n%q", updated.Message)
	}
}

func TestGeoJSONIsSilentWhenTheAdminTurnedItOff(t *testing.T) {
	p := newTestPlugin(t, "https://example.com", true)
	config := p.getConfiguration().Clone()
	config.EnableGeoJSON = false
	p.setConfiguration(config)

	post := &model.Post{Message: geoFence("geojson", geoPoint), UserId: testUserID}

	if updated := p.decoratePost(post, hookRef); updated != nil && updated.Type != "" {
		t.Fatalf("Type = %q with the switch off", updated.Type)
	}
}

func TestAnExplicitGeoJSONFenceThatFailsTellsItsAuthor(t *testing.T) {
	p := newTestPlugin(t, "https://example.com", true)
	api := p.API.(*fakeAPI)

	post := &model.Post{Message: geoFence("geojson", `{"type":"Topology"}`), UserId: testUserID}

	if updated := p.decoratePost(post, hookRef); updated != nil && updated.Type != "" {
		t.Fatal("an unreadable document was stamped")
	}
	if len(api.ephemeral) == 0 {
		t.Fatal("the author was not told")
	}
	if !strings.Contains(api.ephemeral[0].Message, "TF-11010") {
		t.Fatalf("the notice carries no code: %q", api.ephemeral[0].Message)
	}
}

func TestGeoJSONReadsASoleAttachment(t *testing.T) {
	p := newTestPlugin(t, "https://example.com", true)
	api := p.API.(*fakeAPI)
	api.files = map[string]*model.FileInfo{
		geoFileID: {Id: geoFileID, Name: "overlay.geojson", Size: int64(len(geoPoint)), CreatorId: testUserID},
	}
	api.fileContent = map[string][]byte{geoFileID: []byte(geoPoint)}

	post := &model.Post{FileIds: []string{geoFileID}, UserId: testUserID}

	updated := p.decoratePost(post, hookRef)
	if updated == nil || updated.Type != geojson.PostType {
		t.Fatal("a sole .geojson attachment was not stamped")
	}

	blob := geoBlob(t, updated)
	if blob["source"] != geojson.SourceFile {
		t.Errorf("source = %v, want %q", blob["source"], geojson.SourceFile)
	}
	if blob["file_id"] != geoFileID {
		t.Errorf("file_id = %v", blob["file_id"])
	}
}

func TestAJSONAttachmentIsNeverRead(t *testing.T) {
	p := newTestPlugin(t, "https://example.com", true)
	api := p.API.(*fakeAPI)
	api.files = map[string]*model.FileInfo{
		geoFileID: {Id: geoFileID, Name: "overlay.json", Size: int64(len(geoPoint)), CreatorId: testUserID},
	}
	api.fileContent = map[string][]byte{geoFileID: []byte(geoPoint)}

	post := &model.Post{FileIds: []string{geoFileID}, UserId: testUserID}

	if updated := p.decoratePost(post, hookRef); updated != nil && updated.Type != "" {
		t.Fatalf("a .json attachment was stamped as %q", updated.Type)
	}
}

// TestTheVisibleFenceBeatsAnAttachmentAcrossFormats is the collision
// format-major leaves, and the guard in cotFileSource is what closes it.
func TestTheVisibleFenceBeatsAnAttachmentAcrossFormats(t *testing.T) {
	t.Run("a geojson fence beside a .cot attachment", func(t *testing.T) {
		p := newTestPlugin(t, "https://example.com", true)
		api := p.API.(*fakeAPI)
		api.files = map[string]*model.FileInfo{
			testFileID: {Id: testFileID, Name: "track.cot", Size: int64(len(cotEventXML)), CreatorId: testUserID},
		}
		api.fileContent = map[string][]byte{testFileID: []byte(cotEventXML)}

		post := &model.Post{
			Message: geoFence("geojson", geoPoint),
			FileIds: []string{testFileID},
			UserId:  testUserID,
		}

		updated := p.decoratePost(post, hookRef)
		if updated == nil {
			t.Fatal("the post was not stamped at all")
		}
		if updated.Type != geojson.PostType {
			t.Fatalf("the attachment beat the visible fence: Type = %q", updated.Type)
		}
	})

	t.Run("a cot fence beside a .geojson attachment", func(t *testing.T) {
		p := newTestPlugin(t, "https://example.com", true)
		api := p.API.(*fakeAPI)
		api.files = map[string]*model.FileInfo{
			geoFileID: {Id: geoFileID, Name: "overlay.geojson", Size: int64(len(geoPoint)), CreatorId: testUserID},
		}
		api.fileContent = map[string][]byte{geoFileID: []byte(geoPoint)}

		post := &model.Post{
			Message: cotFence("cot", cotEventXML),
			FileIds: []string{geoFileID},
			UserId:  testUserID,
		}

		updated := p.decoratePost(post, hookRef)
		if updated == nil || updated.Type != cot.PostType {
			t.Fatal("the attachment beat the visible fence")
		}
	})
}

// TestARefusedFenceDoesNotFallThroughToAnAttachment is what source-major
// sequencing would have changed. Today a source that is found and then refuses
// to parse ends the attempt.
func TestARefusedFenceDoesNotFallThroughToAnAttachment(t *testing.T) {
	p := newTestPlugin(t, "https://example.com", true)
	api := p.API.(*fakeAPI)
	api.files = map[string]*model.FileInfo{
		testFileID: {Id: testFileID, Name: "track.cot", Size: int64(len(cotEventXML)), CreatorId: testUserID},
	}
	api.fileContent = map[string][]byte{testFileID: []byte(cotEventXML)}

	post := &model.Post{
		Message: cotFence("cot", "<event>not a valid event</event>"),
		FileIds: []string{testFileID},
		UserId:  testUserID,
	}

	if updated := p.decoratePost(post, hookRef); updated != nil && updated.Type != "" {
		t.Fatalf("a refused fence fell through and stamped from the attachment as %q", updated.Type)
	}
}

func TestGeoJSONRefusesAnAttachmentThatIsNotThePostersOwn(t *testing.T) {
	p := newTestPlugin(t, "https://example.com", true)
	api := p.API.(*fakeAPI)
	api.files = map[string]*model.FileInfo{
		geoFileID: {Id: geoFileID, Name: "overlay.geojson", Size: int64(len(geoPoint)), CreatorId: "someoneelseaaaaaaaaaaaaaaa"},
	}
	api.fileContent = map[string][]byte{geoFileID: []byte(geoPoint)}

	post := &model.Post{FileIds: []string{geoFileID}, UserId: testUserID}

	if updated := p.decoratePost(post, hookRef); updated != nil && updated.Type != "" {
		t.Fatal("a stranger's file was read into props")
	}
}

func TestGeoJSONRefusesBeforeAskingTheStoreWhenTheSwitchIsOff(t *testing.T) {
	p := newTestPlugin(t, "https://example.com", true)
	config := p.getConfiguration().Clone()
	config.EnableGeoJSONFile = false
	config.EnableCotFile = false
	p.setConfiguration(config)

	api := p.API.(*fakeAPI)
	api.files = map[string]*model.FileInfo{
		geoFileID: {Id: geoFileID, Name: "overlay.geojson", Size: int64(len(geoPoint)), CreatorId: testUserID},
	}

	p.decoratePost(&model.Post{FileIds: []string{geoFileID}, UserId: testUserID}, hookRef)

	if api.fileInfoCalls != 0 {
		t.Fatalf("the store was asked %d times with both file switches off", api.fileInfoCalls)
	}
}

func TestGeoJSONRecoversFromARealPanic(t *testing.T) {
	p := newTestPlugin(t, "https://example.com", true)
	api := p.API.(*fakeAPI)
	api.panicOnFileInfo = true
	api.files = map[string]*model.FileInfo{geoFileID: {Id: geoFileID, Name: "overlay.geojson"}}

	// Cursor on Target must not reach the store first, or its recover is the
	// one under test rather than this one.
	config := p.getConfiguration().Clone()
	config.EnableCotFile = false
	p.setConfiguration(config)

	post := &model.Post{
		Message: "",
		FileIds: []string{geoFileID},
		UserId:  testUserID,
	}

	if updated := p.decoratePost(post, hookRef); updated != nil && updated.Type != "" {
		t.Fatal("a panicking filestore still produced a stamp")
	}

	// A recover that is never entered is a recover nothing has verified, so the
	// log is the proof this one ran rather than the panic never happening.
	if len(api.warnings) == 0 {
		t.Fatal("the recover did not run; nothing was logged")
	}
	if post.Type != "" {
		t.Error("the caller's post was left stamped after a panic")
	}
}

// A panic must still hand back a stripped post. Returning nil there would put
// the forgery back on.
func TestAGeoJSONPanicAfterStrippingStillReturnsTheStrippedPost(t *testing.T) {
	p := newTestPlugin(t, "https://example.com", true)
	api := p.API.(*fakeAPI)
	api.panicOnFileInfo = true
	api.files = map[string]*model.FileInfo{geoFileID: {Id: geoFileID, Name: "overlay.geojson"}}

	config := p.getConfiguration().Clone()
	config.EnableCotFile = false
	p.setConfiguration(config)

	post := &model.Post{
		Message: "",
		FileIds: []string{geoFileID},
		UserId:  testUserID,
		Type:    geojson.PostType,
	}
	post.AddProp(geojson.PropsKey, map[string]any{"forged": true})

	updated := p.decoratePost(post, hookRef)

	if updated == nil {
		t.Fatal("a panic after stripping handed back nothing, putting the forged type back")
	}
	if updated.Type != "" {
		t.Errorf("Type = %q after a panic, want it stripped", updated.Type)
	}
	if _, carried := updated.GetProps()[geojson.PropsKey]; carried {
		t.Error("the forged blob survived the panic")
	}
}

// TestAForgedGeoJSONTypeIsStripped and its sibling below are the widened strip.
func TestAForgedGeoJSONTypeIsStripped(t *testing.T) {
	p := newTestPlugin(t, "https://example.com", true)

	post := &model.Post{Message: "nothing to see", UserId: testUserID, Type: geojson.PostType}
	post.AddProp(geojson.PropsKey, map[string]any{"version": 1, "features": []any{}})

	updated := p.decoratePost(post, hookRef)
	if updated == nil {
		t.Fatal("a forged type was left on the post")
	}
	if updated.Type != "" {
		t.Errorf("Type = %q, want it stripped", updated.Type)
	}
	if _, carried := updated.GetProps()[geojson.PropsKey]; carried {
		t.Error("the forged blob survived")
	}
}

// TestAForgedSiblingBlobIsAlsoStripped is the reason the strip clears every key
// in the table rather than only the one matching the post's type. cotProps
// copies the post's props forward, so a forged sibling blob would otherwise be
// carried into stored props permanently.
func TestAForgedSiblingBlobIsAlsoStripped(t *testing.T) {
	p := newTestPlugin(t, "https://example.com", true)

	post := &model.Post{Message: cotFence("cot", cotEventXML), UserId: testUserID}
	post.AddProp(geojson.PropsKey, map[string]any{"forged": true})

	updated := p.decoratePost(post, hookRef)
	if updated == nil || updated.Type != cot.PostType {
		t.Fatal("the event was not stamped")
	}
	if _, carried := updated.GetProps()[geojson.PropsKey]; carried {
		t.Fatal("a forged sibling blob was carried into stored props")
	}
}

func TestAnotherIntegrationsTypeIsStillLeftAloneByGeoJSON(t *testing.T) {
	p := newTestPlugin(t, "https://example.com", true)

	post := &model.Post{Message: geoFence("geojson", geoPoint), UserId: testUserID, Type: "custom_someone_else"}

	if updated := p.decoratePost(post, hookRef); updated != nil && updated.Type != "custom_someone_else" {
		t.Fatalf("Type = %q, want it left alone", updated.Type)
	}
}

func TestGeoJSONAndDecorationAreExclusive(t *testing.T) {
	p := newTestPlugin(t, "https://example.com", true)

	// A coordinate in the text around the fence, which must not be rewritten:
	// the card renders that text as plain nodes, so a decorator link written
	// into it could not render there.
	post := &model.Post{
		Message: "seen near 21.3353N 157.9483W\n" + geoFence("geojson", geoPoint),
		UserId:  testUserID,
	}

	updated := p.decoratePost(post, hookRef)
	if updated == nil || updated.Type != geojson.PostType {
		t.Fatal("the document was not stamped")
	}
	if strings.Contains(updated.Message, "/plugins/") {
		t.Fatal("the text around the fence was decorated")
	}
}

func TestGeoJSONKeepsAnotherIntegrationsProps(t *testing.T) {
	p := newTestPlugin(t, "https://example.com", true)

	post := &model.Post{Message: geoFence("geojson", geoPoint), UserId: testUserID}
	post.AddProp("from_another_plugin", "keep me")

	updated := p.decoratePost(post, hookRef)
	if updated.GetProp("from_another_plugin") != "keep me" {
		t.Fatal("another integration's props were dropped")
	}
}

// geoHeavyProperties is a document whose property bags are the bulk of it, so
// the two rungs differ by enough for the ladder to be observable.
func geoHeavyProperties(features, keys, valueRunes int) string {
	parts := make([]string, 0, features)
	for range features {
		pairs := make([]string, 0, keys)
		for k := range keys {
			pairs = append(pairs, `"k`+string(rune('a'+k%26))+string(rune('a'+k/26))+`":"`+strings.Repeat("v", valueRunes)+`"`)
		}
		parts = append(parts, `{"type":"Feature","geometry":{"type":"Point","coordinates":[1,2]},"properties":{`+
			strings.Join(pairs, ",")+`}}`)
	}
	return `{"type":"FeatureCollection","features":[` + strings.Join(parts, ",") + `]}`
}

// geoRungRunes is the encoded size of one rung of a document's blob.
func geoRungRunes(t *testing.T, document string, withProperties bool) int {
	t.Helper()

	parsed, err := geojson.Parse([]byte(document))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	source := geojson.Source{Kind: geojson.SourceFence, Text: document}

	blob := geojson.PropsWithoutProperties(parsed, source)
	if withProperties {
		blob = geojson.Props(parsed, source)
	}

	encoded, err := json.Marshal(blob)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	return utf8.RuneCountInString(string(encoded))
}

func TestOverBudgetThePropertiesAreDroppedBeforeTheCardIs(t *testing.T) {
	p := newTestPlugin(t, "https://example.com", true)

	document := geoHeavyProperties(6, 32, 200)
	post := &model.Post{Message: geoFence("geojson", document), UserId: testUserID}

	// Sized from the rungs themselves rather than from a guess, so the test
	// keeps asking its question if either blob's shape changes: enough room for
	// the lower rung and not enough for the widest. The card is worth more than
	// the property bags, which is what the ladder is for.
	post.AddProp("bulky", strings.Repeat("x", model.PostPropsMaxUserRunes-geoRungRunes(t, document, false)-1000))

	updated := p.decoratePost(post, hookRef)
	if updated == nil || updated.Type != geojson.PostType {
		t.Fatal("the document was not stamped on the lower rung")
	}

	blob := geoBlob(t, updated)
	if blob["properties_dropped"] != "1" {
		t.Fatal("the lower rung did not mark itself")
	}

	features, ok := blob["features"].([]any)
	if !ok || len(features) != 6 {
		t.Fatalf("the card lost its features: %v", blob["features"])
	}
	if _, carried := features[0].(map[string]any)["properties"]; carried {
		t.Fatal("the lower rung still carried properties")
	}
}

// TestPastTheLastRungTheDocumentIsRefused proves the ladder ends rather than
// stamping something that will not fit.
func TestPastTheLastRungTheDocumentIsRefused(t *testing.T) {
	p := newTestPlugin(t, "https://example.com", true)
	api := p.API.(*fakeAPI)

	post := &model.Post{Message: geoFence("geojson", geoPoint), UserId: testUserID}
	post.AddProp("bulky", strings.Repeat("x", model.PostPropsMaxUserRunes))

	if updated := p.decoratePost(post, hookRef); updated != nil && updated.Type != "" {
		t.Fatal("a document over the budget was stamped anyway")
	}
	if len(api.ephemeral) == 0 || !strings.Contains(api.ephemeral[0].Message, "TF-11011") {
		t.Fatal("the author was not told why")
	}
}

// unlabeled returns a plugin with the ambiguous spellings switched on.
func unlabeled(t *testing.T) *Plugin {
	t.Helper()

	p := newTestPlugin(t, "https://example.com", true)
	config := p.getConfiguration().Clone()
	config.EnableGeoJSONUnlabeled = true
	p.setConfiguration(config)

	return p
}

/*
 * The unlabeled spellings, which are off unless an install asks for them.
 *
 * TestAJSONFenceIsNeverStamped and TestAJSONAttachmentIsNeverRead above pin the
 * DEFAULT, and they still pass unchanged: turning the switch on is what changes
 * the answer, which is the whole point of putting it behind one.
 */
func TestTheUnlabeledSwitchReadsAJSONFence(t *testing.T) {
	p := unlabeled(t)

	post := &model.Post{Message: geoFence("json", geoPoint), UserId: testUserID}

	updated := p.decoratePost(post, hookRef)
	if updated == nil || updated.Type != geojson.PostType {
		t.Fatal("a json fence was not stamped with the switch on")
	}
}

func TestTheUnlabeledSwitchReadsABareDocument(t *testing.T) {
	p := unlabeled(t)

	post := &model.Post{Message: "overlay follows\n" + geoPoint, UserId: testUserID}

	updated := p.decoratePost(post, hookRef)
	if updated == nil || updated.Type != geojson.PostType {
		t.Fatal("a bare document was not stamped with the switch on")
	}

	blob := geoBlob(t, updated)
	if blob["lead"] != "overlay follows\n" {
		t.Errorf("lead = %q", blob["lead"])
	}
}

func TestTheUnlabeledSwitchReadsAJSONAttachment(t *testing.T) {
	p := unlabeled(t)
	api := p.API.(*fakeAPI)
	api.files = map[string]*model.FileInfo{
		geoFileID: {Id: geoFileID, Name: "overlay.json", Size: int64(len(geoPoint)), CreatorId: testUserID},
	}
	api.fileContent = map[string][]byte{geoFileID: []byte(geoPoint)}

	post := &model.Post{FileIds: []string{geoFileID}, UserId: testUserID}

	updated := p.decoratePost(post, hookRef)
	if updated == nil || updated.Type != geojson.PostType {
		t.Fatal("a .json attachment was not read with the switch on")
	}
}

// The switch widens what is READ, never what is accepted. Ordinary JSON still
// has to fail the parse, which is the only thing standing between a package
// manifest and a permanently unsearchable post.
func TestTheUnlabeledSwitchStillRefusesOrdinaryJSON(t *testing.T) {
	p := unlabeled(t)

	for _, body := range []string{
		`{"name":"my-package","version":"1.0.0","scripts":{"build":"webpack"}}`,
		`{"type":"object","properties":{"a":{"type":"string"}}}`,
		`{"data":[1,2,3],"ok":true}`,
	} {
		post := &model.Post{Message: geoFence("json", body), UserId: testUserID}

		if updated := p.decoratePost(post, hookRef); updated != nil && updated.Type != "" {
			t.Fatalf("ordinary JSON was stamped as %q: %s", updated.Type, body)
		}
	}
}

// A bare object an author fenced as code is code. Reading it anyway is the
// corruption protected ranges exist to stop.
func TestABareDocumentNeverReachesIntoCode(t *testing.T) {
	p := unlabeled(t)

	post := &model.Post{Message: "```\n" + geoPoint + "\n```", UserId: testUserID}

	if updated := p.decoratePost(post, hookRef); updated != nil && updated.Type != "" {
		t.Fatalf("a document inside an unlabeled fence was stamped as %q", updated.Type)
	}
}

// Only the geojson spelling earns an answer. A json fence and a bare object are
// ambiguous, and a message on every one would fire constantly on ordinary posts.
func TestAnUnlabeledRefusalIsSilent(t *testing.T) {
	for _, message := range []string{
		geoFence("json", `{"type":"Topology"}`),
		`{"type":"Topology","objects":{}}`,
	} {
		p := unlabeled(t)
		api := p.API.(*fakeAPI)

		p.decoratePost(&model.Post{Message: message, UserId: testUserID}, hookRef)

		if len(api.ephemeral) != 0 {
			t.Fatalf("an unlabeled refusal told the author: %q", api.ephemeral[0].Message)
		}
	}
}

// A labeled fence still earns one, with the switch on or off.
func TestALabeledRefusalStillTellsItsAuthor(t *testing.T) {
	p := unlabeled(t)
	api := p.API.(*fakeAPI)

	p.decoratePost(&model.Post{
		Message: geoFence("geojson", `{"type":"Topology"}`), UserId: testUserID,
	}, hookRef)

	if len(api.ephemeral) == 0 {
		t.Fatal("a labeled fence was refused silently")
	}
}

/*
 * The cross-format gate has to track the CONFIGURATION, not one spelling.
 *
 * A fixed test was wrong in both directions: it ignored the switch, so a
 * geojson fence suppressed a good .cot attachment on an install with GeoJSON
 * off and the reader got no card at all; and it knew only the geojson spelling,
 * so an unlabeled document lost to the attachment it should have beaten.
 */
func TestAGeoJSONFenceDoesNotSuppressCoTWhenGeoJSONIsOff(t *testing.T) {
	p := newTestPlugin(t, "https://example.com", true)
	config := p.getConfiguration().Clone()
	config.EnableGeoJSON = false
	p.setConfiguration(config)

	api := p.API.(*fakeAPI)
	api.files = map[string]*model.FileInfo{
		testFileID: {Id: testFileID, Name: "track.cot", Size: int64(len(cotEventXML)), CreatorId: testUserID},
	}
	api.fileContent = map[string][]byte{testFileID: []byte(cotEventXML)}

	post := &model.Post{
		Message: geoFence("geojson", geoPoint),
		FileIds: []string{testFileID},
		UserId:  testUserID,
	}

	updated := p.decoratePost(post, hookRef)
	if updated == nil || updated.Type != cot.PostType {
		t.Fatal("with GeoJSON off, a geojson fence still cost the reader their Cursor on Target card")
	}
}

func TestAnUnlabeledDocumentAlsoBeatsAnAttachment(t *testing.T) {
	for _, message := range []string{geoFence("json", geoPoint), "overlay\n" + geoPoint} {
		p := unlabeled(t)
		api := p.API.(*fakeAPI)
		api.files = map[string]*model.FileInfo{
			testFileID: {Id: testFileID, Name: "track.cot", Size: int64(len(cotEventXML)), CreatorId: testUserID},
		}
		api.fileContent = map[string][]byte{testFileID: []byte(cotEventXML)}

		post := &model.Post{Message: message, FileIds: []string{testFileID}, UserId: testUserID}

		updated := p.decoratePost(post, hookRef)
		if updated == nil || updated.Type != geojson.PostType {
			t.Fatalf("the attachment beat a visible unlabeled document: %q", message)
		}
	}
}

func TestGeoJSONRefusesAnAttachmentOverTheSizeCap(t *testing.T) {
	p := newTestPlugin(t, "https://example.com", true)
	api := p.API.(*fakeAPI)
	api.files = map[string]*model.FileInfo{
		geoFileID: {Id: geoFileID, Name: "overlay.geojson", Size: geojson.MaxSourceBytes + 1, CreatorId: testUserID},
	}
	api.fileContent = map[string][]byte{geoFileID: []byte(geoPoint)}

	post := &model.Post{FileIds: []string{geoFileID}, UserId: testUserID}

	if updated := p.decoratePost(post, hookRef); updated != nil && updated.Type != "" {
		t.Fatal("an over-sized attachment was read")
	}
}

func TestGeoJSONRefusesAFileIdThatIsNotOne(t *testing.T) {
	p := newTestPlugin(t, "https://example.com", true)
	api := p.API.(*fakeAPI)

	p.decoratePost(&model.Post{FileIds: []string{"not-an-id"}, UserId: testUserID}, hookRef)

	if api.fileInfoCalls != 0 {
		t.Fatal("a malformed file id reached the store")
	}
}

// The unmeasurable-props rung, which is reached when something else on the post
// will not marshal. Its ephemeral was never exercised.
func TestGeoJSONReportsPropsItCannotMeasure(t *testing.T) {
	p := newTestPlugin(t, "https://example.com", true)
	api := p.API.(*fakeAPI)

	post := &model.Post{Message: geoFence("geojson", geoPoint), UserId: testUserID}
	post.AddProp("unmarshalable", math.Inf(1))

	if updated := p.decoratePost(post, hookRef); updated != nil && updated.Type != "" {
		t.Fatal("a post whose props cannot be measured was stamped anyway")
	}
	if len(api.ephemeral) == 0 || !strings.Contains(api.ephemeral[0].Message, "TF-11012") {
		t.Fatal("the author was not told the props could not be measured")
	}
}

// An unlabeled source that will not parse is not news. With the switch on any
// brace pair reaches the reader, so a warn per message would drown the log.
func TestAnUnlabeledRefusalIsNotLogged(t *testing.T) {
	p := unlabeled(t)
	api := p.API.(*fakeAPI)

	p.decoratePost(&model.Post{Message: "set it to {5} please", UserId: testUserID}, hookRef)

	for _, warning := range api.warnings {
		if strings.Contains(warning, "GeoJSON source could not be read") {
			t.Fatalf("an ordinary message wrote a refusal warning: %q", warning)
		}
	}
}

func TestALabeledRefusalIsStillLogged(t *testing.T) {
	p := newTestPlugin(t, "https://example.com", true)
	api := p.API.(*fakeAPI)

	p.decoratePost(&model.Post{
		Message: geoFence("geojson", `{"type":"Topology"}`), UserId: testUserID,
	}, hookRef)

	found := false
	for _, warning := range api.warnings {
		if strings.Contains(warning, "GeoJSON source could not be read") {
			found = true
		}
	}
	if !found {
		t.Fatal("a labeled refusal wrote no warning")
	}
}
