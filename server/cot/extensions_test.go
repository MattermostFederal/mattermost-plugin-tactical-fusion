package cot

import (
	"fmt"
	"slices"
	"strings"
	"testing"
)

func eventAround(detail string) string {
	return `<event version="2.0" uid="U1" type="a-f-G-U-C" how="m-g" ` +
		`time="2026-08-23T11:43:38Z" start="2026-08-23T11:43:38Z" stale="2026-08-23T11:45:38Z">` +
		`<point lat="34.056100" lon="-118.250000" hae="1" ce="2" le="3"/>` +
		`<detail>` + detail + `</detail></event>`
}

func detailProps(t *testing.T, detail string) map[string]any {
	t.Helper()
	return propsFor(t, eventAround(detail))
}

// extensionByElement looks an entry up by name alone, so two entries sharing a
// name would make a Block resolve to whichever came first.
func TestRegistryElementsAreUnique(t *testing.T) {
	seen := map[string]bool{}

	for _, ext := range Extensions() {
		if seen[ext.Element] {
			t.Errorf("two registry entries are both named %q", ext.Element)
		}
		seen[ext.Element] = true
	}
}

// The registry-derived fixture is what keeps the cross-language guard from
// narrowing as entries are added, so it has to actually produce every key.
func TestEveryRegisteredKeyIsWrittenFromTheFixture(t *testing.T) {
	props := detailProps(t, FixtureDetail())

	for _, key := range PropKeys() {
		if _, held := props[key]; !held {
			t.Errorf("the registry declares %q and the fixture does not produce it", key)
		}
	}
}

// A contact nested anywhere used to become the event's callsign.
func TestAContactOutsideDetailIsNotTheCallsign(t *testing.T) {
	source := `<event version="2.0" uid="U1" type="a-f-G" how="m-g" time="2026-08-23T11:43:38Z">` +
		`<point lat="1.0000" lon="2.0000" hae="1" ce="2" le="3"/>` +
		`<contact callsign="SPOOFED"/>` +
		`<detail><__video url="x"><contact callsign="ALSO-SPOOFED"/></__video></detail></event>`

	event, err := parseOne([]byte(source))
	if err != nil {
		t.Fatalf("fixture did not parse: %v", err)
	}

	if event.Detail.Callsign != "" {
		t.Errorf("callsign is %q; a contact outside <detail> is not the event's own", event.Detail.Callsign)
	}
}

// A link inside __video used to land in the card's "Relates to" row.
func TestALinkInsideVideoIsNotARelation(t *testing.T) {
	event, err := parseOne([]byte(eventAround(`<__video url="rtsp://x"><link uid="NOT-A-RELATION"/></__video>`)))
	if err != nil {
		t.Fatalf("fixture did not parse: %v", err)
	}

	if len(event.Detail.Links) != 0 {
		t.Errorf("read %d links; a link inside __video is not a relation", len(event.Detail.Links))
	}
}

func TestANamespacedElementIsNotRead(t *testing.T) {
	event, err := parseOne([]byte(eventAround(`<x:contact xmlns:x="urn:e" callsign="SPOOFED"/>`)))
	if err != nil {
		t.Fatalf("fixture did not parse: %v", err)
	}

	if event.Detail.Callsign != "" {
		t.Errorf("callsign is %q; <x:contact> is not <contact>", event.Detail.Callsign)
	}
}

func TestANamespacedAttributeIsNotRead(t *testing.T) {
	event, err := parseOne([]byte(eventAround(`<contact xmlns:x="urn:e" x:callsign="SPOOFED"/>`)))
	if err != nil {
		t.Fatalf("fixture did not parse: %v", err)
	}

	if event.Detail.Callsign != "" {
		t.Errorf("callsign is %q; x:callsign is not callsign", event.Detail.Callsign)
	}
}

// attrValue was already first-wins while a repeated element was last-wins,
// because each arm assigned. One rule now, and it is attrValue's.
func TestARepeatedElementTakesTheFirst(t *testing.T) {
	event, err := parseOne([]byte(eventAround(`<contact callsign="FIRST"/><contact callsign="SECOND"/>`)))
	if err != nil {
		t.Fatalf("fixture did not parse: %v", err)
	}

	if event.Detail.Callsign != "FIRST" {
		t.Errorf("callsign is %q, want FIRST", event.Detail.Callsign)
	}
}

func TestARepeatedBlockTakesTheFirst(t *testing.T) {
	props := detailProps(t, `<takv platform="FIRST"/><takv platform="SECOND"/>`)

	if props["takv_platform"] != "FIRST" {
		t.Errorf("takv_platform is %v, want FIRST", props["takv_platform"])
	}
}

// encoding/xml performs no duplicate-attribute check, verified against the
// pinned SDK, so this is ours to decide and to pin.
func TestADuplicateAttributeTakesTheFirst(t *testing.T) {
	props := detailProps(t, `<_medevac_ security="FIRST" Security="SECOND"/>`)

	if props["medevac_security"] != "FIRST" {
		t.Errorf("medevac_security is %v, want FIRST", props["medevac_security"])
	}
}

// readText swallows the remarks subtree and calls counts.enter itself. A stack
// kept separately in Parse's loop desyncs here, and the symptom is the sibling
// after it being attributed to the wrong parent.
func TestMarkupInRemarksDoesNotDesyncTheParentStack(t *testing.T) {
	props := detailProps(t, `<remarks>see <b>this</b> now</remarks><takv platform="ATAK"/>`)

	if props["takv_platform"] != "ATAK" {
		t.Errorf("takv_platform is %v; the sibling after markup in <remarks> was lost", props["takv_platform"])
	}
	if props["remarks"] != "see this now" {
		t.Errorf("remarks is %v", props["remarks"])
	}
}

func TestANestedChildNeedsItsOwnParent(t *testing.T) {
	props := detailProps(t, `<chatgrp uid0="LOOSE"/>`)

	if _, held := props["chatgrp_uid0"]; held {
		t.Error("chatgrp was read directly under <detail>, where it does not belong")
	}
}

func TestANestedChildIsReadUnderItsParent(t *testing.T) {
	props := detailProps(t, `<__chat chatroom="Ops"><chatgrp uid0="A" uid1="B"/></__chat>`)

	if props["chatgrp_uid0"] != "A" {
		t.Errorf("chatgrp_uid0 is %v, want A", props["chatgrp_uid0"])
	}
}

// __chat/@id is the thread and chatgrp/@id is the group. Flattening both into
// one block overwrote one with the other, with nothing on screen to say so.
func TestChatAndChatgrpIdsDoNotCollide(t *testing.T) {
	props := detailProps(t, `<__chat id="THREAD"><chatgrp id="GROUP"/></__chat>`)

	if props["chat_id"] != "THREAD" {
		t.Errorf("chat_id is %v, want THREAD", props["chat_id"])
	}
	if props["chatgrp_id"] != "GROUP" {
		t.Errorf("chatgrp_id is %v, want GROUP", props["chatgrp_id"])
	}
}

// Attributes were free, and _flow-tags_ is the first element whose attribute
// count is author-chosen without bound.
func TestAttributesCountTowardTheElementBudget(t *testing.T) {
	var attrs strings.Builder
	for i := range maxCotElements + 1 {
		fmt.Fprintf(&attrs, ` a%d="1"`, i)
	}

	_, err := Parse([]byte(eventAround(`<_flow-tags_` + attrs.String() + `/>`)))
	if err != ErrTooMany {
		t.Errorf("Parse returned %v, want ErrTooMany", err)
	}
}

func TestFlowTagsKeepTheirOrder(t *testing.T) {
	props := detailProps(t, `<_flow-tags_ alpha="2026-08-23T20:10:00Z" bravo="2026-08-23T20:10:02Z" charlie="2026-08-23T20:10:04Z"/>`)

	flow, ok := props["flow"].([]any)
	if !ok || len(flow) != 3 {
		t.Fatalf("flow is %v, want three hops", props["flow"])
	}

	want := []string{"alpha", "bravo", "charlie"}
	for i, entry := range flow {
		if got := entry.(map[string]any)["system"]; got != want[i] {
			t.Errorf("hop %d is %v, want %s; the order IS the processing path", i, got, want[i])
		}
	}
}

// Name.Local on xmlns:x is "x", verified against the pinned SDK, so an
// unfiltered reader renders a namespace URI as a hop's timestamp.
func TestFlowTagsSkipNamespaceDeclarations(t *testing.T) {
	props := detailProps(t, `<_flow-tags_ xmlns:x="urn:evil" alpha="2026-08-23T20:10:00Z"/>`)

	flow := props["flow"].([]any)
	for _, entry := range flow {
		if got := entry.(map[string]any)["system"]; got == "x" {
			t.Error("a namespace declaration was rendered as a hop")
		}
	}
	if len(flow) != 1 {
		t.Errorf("flow has %d hops, want 1", len(flow))
	}
}

func TestFlowTagsExcludeVersion(t *testing.T) {
	props := detailProps(t, `<_flow-tags_ version="0.2" alpha="2026-08-23T20:10:00Z"/>`)

	flow := props["flow"].([]any)
	if len(flow) != 1 {
		t.Fatalf("flow has %d hops, want 1", len(flow))
	}
	if got := flow[0].(map[string]any)["system"]; got != "alpha" {
		t.Errorf("hop is %v; version is not a system", got)
	}
}

// Flow tags are appended, so document order is oldest first and the tail is
// what a reader is looking for.
func TestTheFlowCapDropsTheOldestHops(t *testing.T) {
	var attrs strings.Builder
	for i := range maxCotFlowTags + 4 {
		fmt.Fprintf(&attrs, ` s%02d="2026-08-23T20:10:00Z"`, i)
	}

	props := detailProps(t, `<_flow-tags_`+attrs.String()+`/>`)

	flow := props["flow"].([]any)
	if len(flow) != maxCotFlowTags {
		t.Fatalf("flow has %d hops, want %d", len(flow), maxCotFlowTags)
	}

	last := flow[len(flow)-1].(map[string]any)["system"]
	if last != fmt.Sprintf("s%02d", maxCotFlowTags+3) {
		t.Errorf("the last hop is %v; the cap dropped the newest rather than the oldest", last)
	}
}

// A truncated key is our word rather than the event's, and two long names would
// collapse into two rows a reader cannot tell apart.
func TestAnOverlongFlowSystemIsDroppedRatherThanTruncated(t *testing.T) {
	long := strings.Repeat("s", maxFieldRunes+1)
	props := detailProps(t, `<_flow-tags_ `+long+`="2026-08-23T20:10:00Z" alpha="2026-08-23T20:10:02Z"/>`)

	flow := props["flow"].([]any)
	if len(flow) != 1 {
		t.Fatalf("flow has %d hops, want 1", len(flow))
	}
	if got := flow[0].(map[string]any)["system"]; got != "alpha" {
		t.Errorf("hop is %v, want alpha", got)
	}
}

// Omitting it would show a shorter route than the event described.
func TestAFlowHopWithAnUnreadableTimeIsKept(t *testing.T) {
	props := detailProps(t, `<_flow-tags_ alpha="not a time"/>`)

	flow := props["flow"].([]any)
	if len(flow) != 1 {
		t.Fatalf("flow has %d hops, want 1", len(flow))
	}
	if got := flow[0].(map[string]any)["time"]; got != "not a time" {
		t.Errorf("time is %v; an unreadable time is shown as stated, not dropped", got)
	}
}

// Once the panel enumerates blocks, an event with none reads as "carried
// nothing" rather than "we did not recognise what it carried".
func TestDetailUnknownCountsWhatThisBuildDidNotRead(t *testing.T) {
	// Deliberately the elements this build still defers, so the fixture goes
	// stale only when one of them is actually implemented. checklist and
	// checklistColumn stood here until the checklist counter read them.
	props := detailProps(t, `<takv platform="ATAK"/><__network/><fileshare/><mystery-thing/>`)

	if props["detail_unknown"] != "3" {
		t.Errorf("detail_unknown is %v, want 3", props["detail_unknown"])
	}
}

func TestDetailUnknownIsAbsentWhenEverythingWasRead(t *testing.T) {
	props := detailProps(t, `<takv platform="ATAK"/>`)

	if _, held := props["detail_unknown"]; held {
		t.Errorf("detail_unknown is %v; it is absent when there is nothing to say", props["detail_unknown"])
	}
}

// format and value build a location URL the webapp follows, and affiliation
// keys the marker colour. Nothing author-derived may land beside them, and
// _flow-tags_ is the one element whose attribute NAMES are author-chosen.
func TestEventPropsKeysAreClosed(t *testing.T) {
	hostile := `<_flow-tags_ format="x" value="x" affiliation="x" lat="x" lon="x" uid="x" ` +
		`type_label="x" class="x" cot_type="x" src="x"/>`

	props := detailProps(t, FixtureDetail()+hostile)

	allowed := append(PropKeys(),
		"uid", "cot_type", "how", "type_label", "affiliation", "how_label",
		"callsign", "group", "role", "remarks", "parent", "related",
		"time", "time_at", "time_q", "start", "start_q", "stale", "stale_at", "stale_q",
		"lat", "lon", "format", "value", "position_note",
		"hae", "ce", "le", "ce_meters", "speed", "course",
		"class", "detail_unknown", "flow",
	)

	for key := range props {
		if !slices.Contains(allowed, key) {
			t.Errorf("event props carry %q, which is outside the closed key set", key)
		}
	}
}

// courseText rejects anything below zero, which is right for a course and wrong
// for pitch, roll, slope and sensor elevation.
func TestSignedAnglesKeepTheirSign(t *testing.T) {
	props := detailProps(t, `<Attitude roll="2.4" pitch="-7.1" yaw="183.5"/><track slope="-3"/><sensor elevation="-12"/>`)

	checks := map[string]string{
		"attitude_pitch":   "-7.1°",
		"attitude_roll":    "2.4°",
		"attitude_yaw":     "183.5°",
		"track_slope":      "-3°",
		"sensor_elevation": "-12°",
	}

	for key, want := range checks {
		if props[key] != want {
			t.Errorf("%s is %v, want %s", key, props[key], want)
		}
	}
}

// ATAK writes argb as a signed 32-bit decimal rather than as hex.
func TestColorIsDecodedFromASignedDecimal(t *testing.T) {
	cases := map[string]string{
		"-1":        "#ffffff",
		"-65536":    "#ff0000",
		"-16776961": "#0000ff",
	}

	for raw, want := range cases {
		props := detailProps(t, `<color argb="`+raw+`"/>`)
		if props["color_argb"] != want {
			t.Errorf("argb %q decoded to %v, want %s", raw, props["color_argb"], want)
		}
	}
}

// No author string is ever used as a style value, so anything that is not a
// signed decimal produces no colour at all rather than a value a browser reads.
func TestAColorThatIsNotASignedDecimalIsNotWritten(t *testing.T) {
	for _, raw := range []string{"url(https://attacker/px)", "#ff0000", "red", "", "1e9999"} {
		props := detailProps(t, `<color argb="`+raw+`"/>`)
		if held, ok := props["color_argb"]; ok {
			t.Errorf("argb %q produced %v; only a signed decimal is decoded", raw, held)
		}
	}
}

func TestAPresenceOnlyBlockIsStated(t *testing.T) {
	props := detailProps(t, `<archive/>`)

	if props["archive"] != presenceValue {
		t.Errorf("archive is %v, want %q", props["archive"], presenceValue)
	}
}

// A stated zero and an unstated field are different facts, and these are the
// highest-consequence numbers in the inventory.
func TestMedevacStatedZerosSurvive(t *testing.T) {
	props := detailProps(t, `<_medevac_ urgent="0" priority="1" routine="0"/>`)

	checks := map[string]string{
		"medevac_urgent":   "0",
		"medevac_priority": "1",
		"medevac_routine":  "0",
	}

	for key, want := range checks {
		if props[key] != want {
			t.Errorf("%s is %v, want %q; a stated zero is not an unstated field", key, props[key], want)
		}
	}

	if _, held := props["medevac_litter"]; held {
		t.Error("medevac_litter is written for an event that never stated it")
	}
}

// The pane is what a reader opens to check the card, so it has to cover
// everything the card was derived from.
func TestTheSourcePaneCoversEverythingTheParserReads(t *testing.T) {
	if maxInlineSrcRunes < MaxSourceBytes {
		t.Errorf("maxInlineSrcRunes is %d and Parse accepts %d, so an extension parsed past the cap "+
			"has nothing in the pane to check it against", maxInlineSrcRunes, MaxSourceBytes)
	}
}

// Everything version 2 ever wrote survives the middle rung of the hook's budget
// ladder, so a degraded card is exactly the card this feature shipped with.
func TestDroppingDetailKeepsEverythingVersionTwoWrote(t *testing.T) {
	event, err := parseOne([]byte(eventAround(FixtureDetail())))
	if err != nil {
		t.Fatalf("fixture did not parse: %v", err)
	}

	full := eventProps(event, true)
	degraded := eventProps(event, false)

	for _, key := range []string{"uid", "cot_type", "callsign", "group", "role", "speed", "course", "remarks", "parent", "related", "lat", "lon", "ce"} {
		if full[key] != degraded[key] {
			t.Errorf("%s is %v full and %v degraded", key, full[key], degraded[key])
		}
	}

	for _, key := range PropKeys() {
		if _, held := degraded[key]; held {
			t.Errorf("the degraded blob still carries the extension key %q", key)
		}
	}
	for _, key := range []string{"flow", "class", "detail_unknown"} {
		if _, held := degraded[key]; held {
			t.Errorf("the degraded blob still carries %q", key)
		}
	}
}

// A nested <detail> put a top-level entry at depth four under a parent named
// "detail", which matched the entry's own declared parent and read a doubly
// nested element as though it were the event's own.
func TestANestedDetailDoesNotSmuggleATopLevelEntry(t *testing.T) {
	props := detailProps(t, `<detail><takv platform="SMUGGLED"/><contact endpoint="SMUGGLED"/></detail>`)

	for _, key := range []string{"takv_platform", "contact_endpoint"} {
		if held, ok := props[key]; ok {
			t.Errorf("%s is %v; it was read from a nested <detail>", key, held)
		}
	}
}

// The degraded rung says so on the card, not only in the server log.
//
// Without it the panel draws no groups and no unrecognised count, which a
// reader meets as "this event carried nothing" rather than "this did not fit".
func TestTheDegradedBlobSaysSo(t *testing.T) {
	events, err := parseOne([]byte(eventAround(FixtureDetail())))
	if err != nil {
		t.Fatalf("fixture did not parse: %v", err)
	}

	degraded := eventProps(events, false)
	if degraded["detail_dropped"] != presenceValue {
		t.Errorf("detail_dropped is %v, want %q", degraded["detail_dropped"], presenceValue)
	}

	if full := eventProps(events, true); full["detail_dropped"] != nil {
		t.Errorf("a full blob carries detail_dropped = %v", full["detail_dropped"])
	}
}

// Each event keeps its own detail.
//
// `seen` is first-wins keyed by element name and is shared across the document
// until Parse resets it at each root. Drop that reset and events two onward
// lose their callsign, every block, their team and their processing path, as
// blank rows with nothing logged.
func TestEachEventInABatchKeepsItsOwnDetail(t *testing.T) {
	source := `<event uid="U1" type="a-f-G-U-C" time="2026-08-23T11:43:38Z">` +
		`<point lat="1.0000" lon="2.0000"/><detail><contact callsign="ALPHA"/>` +
		`<takv platform="ATAK-CIV"/><_flow-tags_ hopA="2026-08-23T20:10:00Z"/></detail></event>` +
		`<event uid="U2" type="a-f-G-U-C" time="2026-08-23T11:43:38Z">` +
		`<point lat="3.0000" lon="4.0000"/><detail><contact callsign="BRAVO"/>` +
		`<takv platform="WINTAK"/><_flow-tags_ hopB="2026-08-23T20:10:02Z"/></detail></event>`

	events, err := Parse([]byte(source))
	if err != nil {
		t.Fatalf("fixture did not parse: %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("parsed %d events, want 2", len(events))
	}

	want := []struct{ callsign, platform, hop string }{
		{"ALPHA", "ATAK-CIV", "hopA"},
		{"BRAVO", "WINTAK", "hopB"},
	}

	for i, w := range want {
		props := eventProps(events[i], true)
		if props["callsign"] != w.callsign {
			t.Errorf("event %d callsign is %v, want %s", i, props["callsign"], w.callsign)
		}
		if props["takv_platform"] != w.platform {
			t.Errorf("event %d takv_platform is %v, want %s", i, props["takv_platform"], w.platform)
		}

		flow, ok := props["flow"].([]any)
		if !ok || len(flow) != 1 {
			t.Fatalf("event %d flow is %v, want one hop", i, props["flow"])
		}
		if got := flow[0].(map[string]any)["system"]; got != w.hop {
			t.Errorf("event %d hop is %v, want %s", i, got, w.hop)
		}
	}
}

// The namespace rule has to hold on ancestors, not only on the element matched.
//
// The parent stack pushed Name.Local alone, so <x:detail> was indistinguishable
// from <detail> to every parent test below it, and everything inside it was read
// as the event's own while the source pane showed markup that was not <detail>.
func TestANamespacedDetailIsNotTheEventsDetail(t *testing.T) {
	source := `<event uid="U1" type="a-h-G-U-C-A" time="2026-08-23T11:43:38Z">` +
		`<point lat="1.0000" lon="2.0000"/>` +
		`<x:detail xmlns:x="urn:e"><contact callsign="ADMIN"/><takv platform="SPOOFED"/>` +
		`<_flow-tags_ hop="2026-08-23T20:10:00Z"/><remarks>not mine</remarks></x:detail></event>`

	event, err := parseOne([]byte(source))
	if err != nil {
		t.Fatalf("fixture did not parse: %v", err)
	}

	if event.Detail.Callsign != "" {
		t.Errorf("callsign is %q; <x:detail> is not <detail>", event.Detail.Callsign)
	}
	if event.Detail.Remarks != "" {
		t.Errorf("remarks is %q", event.Detail.Remarks)
	}
	if len(event.Detail.Blocks) != 0 {
		t.Errorf("read %d blocks from a namespaced detail", len(event.Detail.Blocks))
	}
	if len(event.Detail.Flow) != 0 {
		t.Errorf("read %d flow hops from a namespaced detail", len(event.Detail.Flow))
	}
}

// A nested child needs its parent to have been ACCEPTED, not merely named.
//
// Matching on the parent's name alone read a ConnectionEntry whose <__video>
// had itself been skipped for sitting outside <detail>, so a panel row was
// built from markup that was never in <detail> at all.
func TestANestedChildNeedsAnAcceptedParent(t *testing.T) {
	source := `<event uid="U1" type="a-h-G-U-C-A" time="2026-08-23T11:43:38Z">` +
		`<point lat="1.0000" lon="2.0000"/>` +
		`<zz><__video><ConnectionEntry address="10.0.0.1" port="1234"/></__video>` +
		`<__chat><chatgrp uid0="A" uid1="B" id="G"/></__chat></zz></event>`

	event, err := parseOne([]byte(source))
	if err != nil {
		t.Fatalf("fixture did not parse: %v", err)
	}

	props := eventProps(event, true)
	for _, key := range []string{"video_conn_address", "video_conn_port", "chatgrp_uid0", "chatgrp_id"} {
		if held, ok := props[key]; ok {
			t.Errorf("%s is %v; its parent was never accepted as a <detail> child", key, held)
		}
	}
}

// Version is excluded whatever its casing, or the processing path gains a
// system called Version whose timestamp is its version number.
func TestFlowTagsExcludeVersionWhateverItsCase(t *testing.T) {
	for _, spelling := range []string{"version", "Version", "VERSION"} {
		props := detailProps(t, `<_flow-tags_ `+spelling+`="0.2" alpha="2026-08-23T20:10:00Z"/>`)

		flow, ok := props["flow"].([]any)
		if !ok || len(flow) != 1 {
			t.Fatalf("%s: flow is %v, want one hop", spelling, props["flow"])
		}
		if got := flow[0].(map[string]any)["system"]; got != "alpha" {
			t.Errorf("%s: hop is %v, want alpha", spelling, got)
		}
	}
}

// FormatFloat never uses exponent notation, so a subnormal expands to its full
// positional form: 324 runes in a cell whose stated cap is 128. A clipped
// number still reads as a number, so it is refused like an overlong flow name.
func TestAFigureTooLongToBeAReadingIsRefused(t *testing.T) {
	props := detailProps(t, `<sensor range="1e-320" fov="18"/>`)

	if held, ok := props["sensor_range"]; ok {
		t.Errorf("sensor_range is %v (%d runes)", held, len(held.(string)))
	}
	if props["sensor_fov"] != "18°" {
		t.Errorf("sensor_fov is %v; the neighbouring reading is unaffected", props["sensor_fov"])
	}
}

// "-0" reads as a direction on a bearing and as a sign on a battery.
func TestNegativeZeroIsNormalised(t *testing.T) {
	props := detailProps(t, `<Attitude pitch="-0.0" roll="-0"/><status battery="-0"/>`)

	checks := map[string]string{"attitude_pitch": "0°", "attitude_roll": "0°", "status_battery": "0%"}
	for key, want := range checks {
		if props[key] != want {
			t.Errorf("%s is %v, want %s", key, props[key], want)
		}
	}
}

// The event's own attributes take the namespace rule too.
//
// setOnce keeps the FIRST value it is given, so without the guard a namespaced
// attribute written ahead of the real one locks in the spoofed value and the
// event's own uid is discarded. TestANamespacedAttributeIsNotRead covers
// attrValue; this covers readEvent, which does not go through it.
func TestANamespacedEventAttributeIsNotRead(t *testing.T) {
	source := `<event xmlns:x="urn:e" x:uid="SPOOFED" x:type="a-f-G" x:how="m-g" ` +
		`uid="REAL" type="a-h-G-U-C-A" how="h-e" time="2026-08-23T11:43:38Z">` +
		`<point lat="1.0000" lon="2.0000"/><detail/></event>`

	event, err := parseOne([]byte(source))
	if err != nil {
		t.Fatalf("fixture did not parse: %v", err)
	}

	checks := map[string]struct{ got, want string }{
		"uid":  {event.UID, "REAL"},
		"type": {event.Type, "a-h-G-U-C-A"},
		"how":  {event.How, "h-e"},
	}

	for name, c := range checks {
		if c.got != c.want {
			t.Errorf("%s is %q, want %q; a namespaced attribute is not the event's own", name, c.got, c.want)
		}
	}
}

// The same substitution, on the coordinate, where it moves the pin.
func TestANamespacedPointAttributeIsNotRead(t *testing.T) {
	source := `<event uid="U1" type="a-f-G-U-C" time="2026-08-23T11:43:38Z">` +
		`<point xmlns:x="urn:e" x:lat="9.9999" x:lon="8.8888" ` +
		`lat="1.0000" lon="2.0000" ce="3"/><detail/></event>`

	event, err := parseOne([]byte(source))
	if err != nil {
		t.Fatalf("fixture did not parse: %v", err)
	}

	if event.Point.Lat != "1.0000" || event.Point.Lon != "2.0000" {
		t.Errorf("position is %q,%q, want 1.0000,2.0000; a namespaced attribute is not the point's own",
			event.Point.Lat, event.Point.Lon)
	}
}

// A battery reading that is not a number writes no row rather than a blank one,
// and one the event states above 100 is shown as stated rather than clamped:
// clamping would be this plugin's claim about a figure the device reported.
func TestPercentRefusesWhatIsNotAPercentage(t *testing.T) {
	for _, raw := range []string{"abc", "-5", "", "9999999"} {
		props := detailProps(t, `<status battery="`+raw+`"/>`)
		if held, ok := props["status_battery"]; ok {
			t.Errorf("battery %q produced %v; it is not a percentage", raw, held)
		}
	}

	props := detailProps(t, `<status battery="250"/>`)
	if props["status_battery"] != "250%" {
		t.Errorf("battery 250 is %v, want 250%%, as stated rather than clamped", props["status_battery"])
	}
}

// Acceptance is about the element instance, not its name.
//
// A flat seen-set answered "some element with this name was accepted somewhere
// earlier", so a legitimate <detail><__video/> vouched for a second <__video>
// parked outside <detail> entirely.
func TestAnEarlierBlockDoesNotVouchForALaterOne(t *testing.T) {
	source := `<event uid="U1" type="a-h-G-U-C-A" time="2026-08-23T11:43:38Z">` +
		`<point lat="1.0000" lon="2.0000"/>` +
		`<detail><__video uid="V1" url="rtsp://real/one"/></detail>` +
		`<zz><__video><ConnectionEntry address="10.0.0.1" port="1234"/></__video></zz></event>`

	event, err := parseOne([]byte(source))
	if err != nil {
		t.Fatalf("fixture did not parse: %v", err)
	}

	props := eventProps(event, true)
	if props["video_uid"] != "V1" {
		t.Errorf("video_uid is %v; the legitimate block was lost", props["video_uid"])
	}
	for _, key := range []string{"video_conn_address", "video_conn_port"} {
		if held, ok := props[key]; ok {
			t.Errorf("%s is %v; its own parent was never accepted", key, held)
		}
	}
}

// First-wins rejects the second <__chat>, so its <chatgrp> is not the event's
// either. Otherwise a repeat adds a key, which the whole first-wins rule and
// the sync fixture that rests on it both say cannot happen.
func TestARejectedRepeatDoesNotContributeItsChild(t *testing.T) {
	props := detailProps(t, `<__chat senderCallsign="ALPHA" chatroom="Ops"/>`+
		`<__chat senderCallsign="BRAVO"><chatgrp uid0="X" uid1="Y" id="G2"/></__chat>`)

	if props["chat_sender"] != "ALPHA" {
		t.Errorf("chat_sender is %v, want ALPHA", props["chat_sender"])
	}
	for _, key := range []string{"chatgrp_uid0", "chatgrp_uid1", "chatgrp_id"} {
		if held, ok := props[key]; ok {
			t.Errorf("%s is %v; it came from a block first-wins had rejected", key, held)
		}
	}
}

// A prefix bound to the empty URI resolves back to no namespace, which undoes
// every namespace test in this package at once: <x:detail xmlns:x=""> arrives
// as plain <detail>. The source is refused rather than read.
func TestAPrefixBoundToNothingIsRefused(t *testing.T) {
	source := `<event uid="U1" type="a-h-G-U-C-A" time="2026-08-23T11:43:38Z">` +
		`<point lat="1.0000" lon="2.0000"/>` +
		`<x:detail xmlns:x=""><contact callsign="ADMIN"/><takv platform="SPOOFED"/>` +
		`<remarks>not mine</remarks></x:detail></event>`

	if _, err := Parse([]byte(source)); err != ErrNullPrefix {
		t.Errorf("Parse returned %v, want ErrNullPrefix", err)
	}
}

// The legal default undeclaration is not the same thing and stays readable.
func TestADefaultUndeclarationIsNotRefused(t *testing.T) {
	source := `<event uid="U1" type="a-f-G-U-C" time="2026-08-23T11:43:38Z">` +
		`<point lat="1.0000" lon="2.0000"/>` +
		`<detail xmlns=""><contact callsign="DELTA1"/></detail></event>`

	event, err := parseOne([]byte(source))
	if err != nil {
		t.Fatalf("a legal xmlns=\"\" was refused: %v", err)
	}
	if event.Detail.Callsign != "DELTA1" {
		t.Errorf("callsign is %q, want DELTA1", event.Detail.Callsign)
	}
}

// A namespaced root is refused rather than stamped.
//
// Every child is skipped for carrying a namespace, so the card would otherwise
// tell a reader the event stated no position when it stated one, which is the
// one thing this package refuses to do.
func TestANamespacedRootIsRefused(t *testing.T) {
	source := `<event xmlns="urn:cot" uid="U1" type="a-f-G-U-C" time="2026-08-23T11:43:38Z">` +
		`<point lat="34.0561" lon="-118.2500" ce="45"/>` +
		`<detail><contact callsign="DELTA1"/></detail></event>`

	if _, err := Parse([]byte(source)); err != ErrNotEvent {
		t.Errorf("Parse returned %v, want ErrNotEvent", err)
	}
}

// A hash is longer than a field, so the list truncates mid-hash into something
// that looks like a hash and is not. The count is the part a reader can act on.
func TestAttachmentsAreCountedRatherThanPrinted(t *testing.T) {
	hashes := `[&quot;` + strings.Repeat("a", 64) + `&quot;,&quot;` + strings.Repeat("b", 64) + `&quot;]`
	props := detailProps(t, `<attachment_list hashes="`+hashes+`"/>`)

	if props["attachments_count"] != "2" {
		t.Errorf("attachments_count is %v, want 2", props["attachments_count"])
	}

	for key, value := range props {
		if text, ok := value.(string); ok && strings.Contains(text, strings.Repeat("a", 20)) {
			t.Errorf("%s carries a hash: %q", key, text)
		}
	}
}

func TestAnUnreadableAttachmentListIsNotCounted(t *testing.T) {
	for _, raw := range []string{"garbage", "", "{}", `[&quot;a&quot;`, "[1,2]"} {
		props := detailProps(t, `<attachment_list hashes="`+raw+`"/>`)
		if held, ok := props["attachments_count"]; ok {
			t.Errorf("hashes %q produced a count of %v", raw, held)
		}
	}
}

// An unlabelled -71 is the derived-claim failure in reverse: the reader
// supplies the unit instead of the plugin.
func TestARadioSignalCarriesItsUnit(t *testing.T) {
	props := detailProps(t, `<_radio rssi="-71" gps="3"/>`)

	if props["radio_rssi"] != "-71 dBm" {
		t.Errorf("radio_rssi is %v, want -71 dBm", props["radio_rssi"])
	}
	if props["radio_gps"] != "3" {
		t.Errorf("radio_gps is %v", props["radio_gps"])
	}
}

// TakControl carries nothing itself and its children carry everything, which is
// the nested-parent rule chatgrp and ConnectionEntry already established.
func TestTakControlReadsItsChildren(t *testing.T) {
	props := detailProps(t, `<TakControl><TakProtocolSupport version="1"/>`+
		`<TakRequest version="1"/><TakResponse status="true"/></TakControl>`)

	checks := map[string]string{
		"takcontrol":                 presenceValue,
		"takcontrol_support_version": "1",
		"takcontrol_request_version": "1",
		"takcontrol_response_status": "true",
	}

	for key, want := range checks {
		if props[key] != want {
			t.Errorf("%s is %v, want %s", key, props[key], want)
		}
	}
}

// The same instance rule the other nested children get: a protocol child whose
// TakControl was never accepted is not the event's own.
func TestATakControlChildNeedsItsParent(t *testing.T) {
	props := detailProps(t, `<TakProtocolSupport version="9"/>`)

	if held, ok := props["takcontrol_support_version"]; ok {
		t.Errorf("takcontrol_support_version is %v without a TakControl around it", held)
	}
}

func TestTheLongTailIsNoLongerUnrecognised(t *testing.T) {
	detail := `<__chatReceipt id="r1" chatroom="Ops" ackuid="a1" senderCallsign="ALPHA"/>` +
		`<__serverdestination destinations="server-a"/>` +
		`<_radio rssi="-71"/>` +
		`<__geofence monitor="Friendly" trigger="Entry" boundingSphere="500"/>` +
		`<attachment_list hashes="[&quot;a&quot;]"/>` +
		`<TakControl><TakProtocolSupport version="1"/></TakControl>`

	props := detailProps(t, detail)

	if held, ok := props["detail_unknown"]; ok {
		t.Errorf("detail_unknown is %v; every element above is read now", held)
	}
}

func TestARadioSignalRefusesWhatIsNotAReading(t *testing.T) {
	for _, rssi := range []string{"abc", "", "9999999", "-9999999"} {
		t.Run(fmt.Sprintf("rssi %q", rssi), func(t *testing.T) {
			props := detailProps(t, `<_radio rssi="`+rssi+`" gps="3"/>`)

			if value, ok := props["radio_rssi"]; ok {
				t.Errorf("rssi=%q was rendered as %v; it is not a reading", rssi, value)
			}
			if props["radio_gps"] != "3" {
				t.Errorf("gps is %v, so the refusal took the whole element with it", props["radio_gps"])
			}
		})
	}
}

func TestARadioSignalKeepsItsSignAndUnit(t *testing.T) {
	props := detailProps(t, `<_radio rssi="-72.5" gps="3"/>`)

	if props["radio_rssi"] != "-72.5 dBm" {
		t.Errorf("rssi rendered as %v, want %q", props["radio_rssi"], "-72.5 dBm")
	}
}
