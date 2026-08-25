package cot

import (
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/decorators"
	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/decorators/dtg"
	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/decorators/location"
)

func propsFor(t *testing.T, source string) map[string]any {
	t.Helper()

	events, err := Parse([]byte(source))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	props := Props(events, Source{Kind: SourceFence, Text: source})

	rendered, ok := props["events"].([]any)
	if !ok || len(rendered) != 1 {
		t.Fatalf("props carry %v events, want 1", props["events"])
	}

	inner, ok := rendered[0].(map[string]any)
	if !ok {
		t.Fatal("the event is not a map")
	}
	return inner
}

func pointEvent(attrs string) string {
	return `<event uid="u" type="a-f-G" time="2026-08-23T11:43:38Z"><point ` + attrs + `/></event>`
}

func TestPostTypeFitsTheDatabaseColumn(t *testing.T) {
	if !strings.HasPrefix(PostType, decorators.PostTypePrefix) {
		t.Errorf("PostType %q lacks the custom_ prefix Post.IsValid requires", PostType)
	}
	if len(PostType) > decorators.PostTypeMaxLen {
		t.Errorf("PostType %q is %d bytes, over the %d the Posts.Type column holds",
			PostType, len(PostType), decorators.PostTypeMaxLen)
	}
}

func TestPositionLinkKeepsTheDigitsTheEventCarried(t *testing.T) {
	cases := map[string]struct {
		lat, lon string
		value    string
		linked   bool
	}{
		"seven decimals": {"30.0090270", "-85.9578740", "30.0090270,-85.9578740", true},
		"four decimals":  {"30.0090", "-85.9578", "30.0090,-85.9578", true},
		"one decimal":    {"30.0", "-86.0", "", false},
		"integer":        {"30", "-86", "", false},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			props := propsFor(t, pointEvent(`lat="`+tc.lat+`" lon="`+tc.lon+`"`))

			value, linked := props["value"].(string)
			if linked != tc.linked {
				t.Fatalf("linked = %v, want %v (value %q)", linked, tc.linked, value)
			}
			if linked && value != tc.value {
				t.Errorf("value = %q, want %q", value, tc.value)
			}
			if !linked && props["position_note"] == nil {
				t.Error("an unlinked position carries no note explaining why")
			}
			if props["lat"] != tc.lat {
				t.Errorf("lat = %v, want %q; a value we hold must still be shown", props["lat"], tc.lat)
			}
		})
	}
}

func TestLinkedPositionRoundTripsThroughTheLocationGrammar(t *testing.T) {
	props := propsFor(t, pointEvent(`lat="30.009027" lon="-85.957874"`))

	value, _ := props["value"].(string)
	parsed, ok := location.Parse(location.FormatDD, value)
	if !ok {
		t.Fatalf("the emitted value %q is not accepted by the location grammar", value)
	}
	if parsed.Canonical() != value {
		t.Errorf("value %q does not round trip; canonical is %q", value, parsed.Canonical())
	}
}

func TestNullIslandIsShownAndNotLinked(t *testing.T) {
	props := propsFor(t, pointEvent(`lat="0.000000" lon="0.000000"`))

	if props["value"] != nil || props["format"] != nil {
		t.Error("0,0 was linked; no pin may be drawn for the unset sentinel")
	}
	if props["lat"] != "0.000000" || props["lon"] != "0.000000" {
		t.Error("0,0 was hidden; the card must show a value it holds")
	}
	if props["position_note"] != nullIslandNote {
		t.Errorf("position_note = %v, want the null island note", props["position_note"])
	}
}

func TestPositionOutsideTheEarthIsRefused(t *testing.T) {
	props := propsFor(t, pointEvent(`lat="130.00000" lon="-85.95787"`))

	if props["value"] != nil || props["lat"] != nil {
		t.Error("an out-of-range position was carried through")
	}
	if props["position_note"] != rangeNote {
		t.Errorf("position_note = %v, want the range note", props["position_note"])
	}
}

func TestUnknownSentinelsAreNeverShownAsFigures(t *testing.T) {
	for _, raw := range []string{"9999999.0", "9999999", "9999999.00000", "-1"} {
		props := propsFor(t, pointEvent(`lat="1.00000" lon="2.00000" ce="`+raw+`" le="`+raw+`"`))

		if props["ce"] != nil {
			t.Errorf("ce=%q rendered as %v, want absent", raw, props["ce"])
		}
		if props["le"] != nil {
			t.Errorf("le=%q rendered as %v, want absent", raw, props["le"])
		}
		if props["ce_meters"] != nil {
			t.Errorf("ce=%q produced a circle radius %v", raw, props["ce_meters"])
		}
	}
}

func TestKnownAccuracyIsRenderedAndCarriesARadius(t *testing.T) {
	props := propsFor(t, pointEvent(`lat="1.00000" lon="2.00000" ce="45.3" le="99.5" hae="-42.6"`))

	checks := map[string]string{"ce": "45.3 m", "le": "99.5 m", "hae": "-42.6 m", "ce_meters": "45.3"}
	for key, want := range checks {
		if props[key] != want {
			t.Errorf("%s = %v, want %q", key, props[key], want)
		}
	}
}

func TestNegativeHaeIsAPositionNotASentinel(t *testing.T) {
	props := propsFor(t, pointEvent(`lat="1.00000" lon="2.00000" hae="-42.6"`))

	if props["hae"] != "-42.6 m" {
		t.Errorf("hae = %v; a negative altitude is below the ellipsoid, not unknown", props["hae"])
	}
}

func TestValidityWindowComesFromTheEventAlone(t *testing.T) {
	props := propsFor(t, `<event uid="u" type="a-f-G" time="2026-08-23T11:43:38Z" `+
		`stale="2026-08-23T11:45:38Z"><point lat="1.00000" lon="2.00000"/></event>`)

	if props["stale_at"] == nil || props["time_at"] == nil {
		t.Error("the two epochs are absent; the webapp computes every duration from them")
	}
	if props["valid_for"] != nil {
		t.Error("valid_for is still rendered in Go; the duration formatter lives in the webapp alone")
	}
	if props["time"] != "231143ZAUG26" {
		t.Errorf("time = %v, want a Zulu DTG", props["time"])
	}
}

func TestAuthorTextIsCappedAndStripped(t *testing.T) {
	long := strings.Repeat("A", maxFieldRunes+50)
	hostile := "DELTA\u202E1"

	props := propsFor(t, `<event uid="`+long+`" type="a-f-G" time="2026-08-23T11:43:38Z">`+
		`<point lat="1.00000" lon="2.00000"/><detail><contact callsign="`+hostile+`"/></detail></event>`)

	uid, _ := props["uid"].(string)
	if len([]rune(uid)) != maxFieldRunes+1 {
		t.Errorf("uid is %d runes, want the cap plus a marker", len([]rune(uid)))
	}
	if !strings.HasSuffix(uid, truncationMarker) {
		t.Error("a truncated field does not say it was truncated")
	}

	callsign, _ := props["callsign"].(string)
	if callsign != "DELTA1" {
		t.Errorf("callsign = %q, want %q; control and bidi characters must be stripped", callsign, "DELTA1")
	}
}

// Truncated with a visible marker rather than withheld. The webapp never reads
// the message, so withholding left a fenced event over the cap with no way for
// any reader to reach its XML at all.
func TestAnOversizedSourceIsTruncatedRatherThanWithheld(t *testing.T) {
	events, err := Parse([]byte(atakPLI))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	big := strings.Repeat("x", maxInlineSrcRunes+50)
	props := Props(events, Source{Kind: SourceFile, Text: big, FileID: "f1", FileName: "event.cot"})

	src, ok := props["src"].(string)
	if !ok {
		t.Fatal("an oversized source was withheld entirely")
	}
	if len([]rune(src)) != maxInlineSrcRunes+1 {
		t.Errorf("src is %d runes, want the cap plus a marker", len([]rune(src)))
	}
	if !strings.HasSuffix(src, truncationMarker) {
		t.Error("a truncated source does not say it was truncated")
	}
	if props["file_id"] != "f1" {
		t.Error("the file case lost its file id, so the reader has no way to reach the whole source")
	}
}

func TestTypeDecodingSaysWhenItDoesNotKnow(t *testing.T) {
	cases := map[string]struct {
		raw         string
		label       string
		affiliation string
	}{
		"ground combat unit":    {"a-f-G-U-C", "Friendly Ground Combat Unit", "friend"},
		"battle dimension only": {"a-f-A", "Friendly Air Track", "friend"},
		"fixed wing":            {"a-h-A-M-F", "Hostile Military Fixed Wing Aircraft", "hostile"},
		"infantry":              {"a-f-G-U-C-I", "Friendly Infantry Unit", "friend"},
		"submarine":             {"a-u-U-S", "Unknown Submarine", "unknown"},
		"medical":               {"a-f-G-U-S-M", "Friendly Medical Unit", "friend"},
		"affiliation only":      {"a-n", "Neutral", "neutral"},
		"unknown dimension":     {"a-f-Z", "Friendly", "friend"},
		"geochat":               {"b-t-f", "Chat Message", ""},
		"unrecognized":          {"z-q-q", "", ""},
		"empty":                 {"", "", ""},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			decoded := decodeType(tc.raw)
			if decoded.Label != tc.label {
				t.Errorf("label = %q, want %q", decoded.Label, tc.label)
			}
			if decoded.Affiliation != tc.affiliation {
				t.Errorf("affiliation = %q, want %q", decoded.Affiliation, tc.affiliation)
			}
		})
	}
}

func TestHowDecodingSaysWhenItDoesNotKnow(t *testing.T) {
	cases := map[string]string{
		"m-g": "Machine, GPS",
		"h-e": "Human, Estimated",
		"m-z": "Machine",
		"zzz": "",
		"":    "",
	}

	for raw, want := range cases {
		if got := decodeHow(raw); got != want {
			t.Errorf("decodeHow(%q) = %q, want %q", raw, got, want)
		}
	}
}

func TestEveryPropsValueIsAStringExceptTheVersion(t *testing.T) {
	events, err := Parse([]byte(atakPLI))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	props := Props(events, Source{Kind: SourceFence, Text: atakPLI})
	for key, value := range props {
		if key == "version" || key == "events" {
			continue
		}
		if _, ok := value.(string); !ok {
			t.Errorf("props[%q] is %T, want string", key, value)
		}
	}

	for i, rendered := range props["events"].([]any) {
		for key, value := range rendered.(map[string]any) {
			if _, ok := value.(string); !ok {
				t.Errorf("events[%d][%q] is %T, want string", i, key, value)
			}
		}
	}
}

func TestSanitizeStripsWhatXMLItselfWouldHaveAllowed(t *testing.T) {
	cases := map[string]struct{ in, want string }{
		"bidi override":     {"DELTA\u202E1", "DELTA1"},
		"isolate":           {"A\u2066B\u2069C", "ABC"},
		"byte order mark":   {"\uFEFFDELTA1", "DELTA1"},
		"bell":              {"DELTA\u00071", "DELTA1"},
		"carriage return":   {"one\r\ntwo", "one\ntwo"},
		"tab kept":          {"one\ttwo", "one\ttwo"},
		"newline kept":      {"one\ntwo", "one\ntwo"},
		"surrounding space": {"  DELTA1  ", "DELTA1"},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			if got := sanitize(tc.in, maxFieldRunes); got != tc.want {
				t.Errorf("sanitize(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestSanitizeCountsRunesRatherThanBytes(t *testing.T) {
	multibyte := strings.Repeat("\u00E9", maxFieldRunes+10)

	got := sanitize(multibyte, maxFieldRunes)
	if len([]rune(got)) != maxFieldRunes+1 {
		t.Errorf("sanitize kept %d runes, want the cap plus a marker", len([]rune(got)))
	}
}

// The disclosure is what a reader opens to check the card against, so it is the
// one string where a direction override would subvert the verification rather
// than the claim. It was the only author-controlled value with no filter.
func TestTheSourcePaneIsSanitisedToo(t *testing.T) {
	events, err := Parse([]byte(atakPLI))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	hostile := "<event uid=\"u\"\u202E lat=\"1\"/>"
	props := Props(events, Source{Kind: SourceFence, Text: hostile, Lead: "note\u200F", Trail: "\u200Btail"})

	for _, key := range []string{"src", "lead", "trail"} {
		value, _ := props[key].(string)
		for _, bad := range []rune{0x202E, 0x200F, 0x200B} {
			if strings.ContainsRune(value, bad) {
				t.Errorf("props[%q] still carries U+%04X", key, bad)
			}
		}
	}
}

// Trimming is for a field. The author's own message text keeps its whitespace,
// because restyling what somebody wrote is a different thing from removing what
// they cannot see.
func TestAuthorTextKeepsItsOwnWhitespace(t *testing.T) {
	events, err := Parse([]byte(atakPLI))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	props := Props(events, Source{Kind: SourceFence, Text: atakPLI, Lead: "latest PLI\n", Trail: "\nfrom ALPHA"})

	if props["lead"] != "latest PLI\n" {
		t.Errorf("lead = %q, want the author's trailing newline kept", props["lead"])
	}
	if props["trail"] != "\nfrom ALPHA" {
		t.Errorf("trail = %q, want the author's leading newline kept", props["trail"])
	}
}

// strconv.ParseFloat accepts more shapes than JavaScript's Number does, and a
// position the card shows but the map reads as NaN is the two surfaces
// disagreeing about whether there is a position at all.
func TestPositionRefusesShapesJavaScriptWouldNotRead(t *testing.T) {
	for _, raw := range []string{"0x1p+3", "1e2", "Inf", "NaN", " 12.5", "12.5\n", "+.5"} {
		props := propsFor(t, pointEvent(`lat="`+raw+`" lon="2.00000"`))

		if props["lat"] != nil || props["value"] != nil {
			t.Errorf("lat=%q was carried through as %v", raw, props["lat"])
		}
	}
}

func TestOrdinaryDecimalsStillParse(t *testing.T) {
	for _, raw := range []string{"34.0561", "-118.25000", "0.0000", "90.0000", "+12.3456"} {
		if _, ok := decimalNumber(raw); !ok {
			t.Errorf("decimalNumber(%q) refused an ordinary decimal", raw)
		}
	}
}

// Two prefixes in the table match this type, so ranging the map decided the
// label by iteration order and wrote the coin flip permanently into the post.
func TestOverlappingTypePrefixesResolveToTheLongest(t *testing.T) {
	for range 50 {
		if got := decodeType("t-x-c-t-r-1").Label; got != "Ping Response" {
			t.Fatalf("decodeType = %q, want the longest matching prefix", got)
		}
	}
}

// The date-time group page re-derives everything from the canonical token and
// refuses a set that does not round trip, so a hand-built query would open on a
// refusal rather than a panel.
func TestTimesCarryLinkParamsThatTheDTGPageAccepts(t *testing.T) {
	props := propsFor(t, `<event uid="u" type="a-f-G" time="2026-08-23T11:43:38Z" `+
		`start="2026-08-23T11:40:00Z" stale="2026-08-23T11:45:38Z">`+
		`<point lat="1.00000" lon="2.00000"/></event>`)

	for _, key := range []string{"time_q", "start_q", "stale_q"} {
		raw, ok := props[key].(string)
		if !ok || raw == "" {
			t.Fatalf("%s is absent", key)
		}

		params, err := url.ParseQuery(raw)
		if err != nil {
			t.Fatalf("%s is not a query string: %v", key, err)
		}

		if _, accepted := (&dtg.Decorator{}).Parse(params.Get("dtg"), time.Now()); !accepted {
			t.Errorf("%s carries a token the DTG grammar refuses: %q", key, params.Get("dtg"))
		}
		if params.Get("t") == "" || params.Get("z") != "Z" {
			t.Errorf("%s = %q, want a Zulu instant", key, raw)
		}
	}
}

// An instant the grammar cannot spell carries no link rather than one pointing
// a century away.
func TestATimeOutsideTheGrammarCarriesNoLink(t *testing.T) {
	props := propsFor(t, `<event uid="u" type="a-f-G" time="1999-08-23T11:43:38Z">`+
		`<point lat="1.00000" lon="2.00000"/></event>`)

	if props["time"] == nil {
		t.Error("the reading itself was dropped; the card should still show what the event said")
	}
	if props["time_q"] != nil {
		t.Errorf("time_q = %v, want absent for an instant outside 2000-2099", props["time_q"])
	}
}

// An unrecognized tail costs the letters after it, never the ones before. The
// raw type is on the card beside the label, so a reader can see what was not
// decoded rather than being told a guess.
func TestAnUnknownTailKeepsTheDeepestKnownLabel(t *testing.T) {
	cases := map[string]string{
		"a-f-G-U-C-I-XYZ": "Friendly Infantry Unit",
		"a-f-G-U-C-ZZZ":   "Friendly Ground Combat Unit",
		"a-f-G-ZZZ":       "Friendly Ground Track",
		"a-f-ZZZ":         "Friendly",
	}

	for raw, want := range cases {
		if got := decodeType(raw).Label; got != want {
			t.Errorf("decodeType(%q) = %q, want %q", raw, got, want)
		}
	}
}

// The generated table is embedded, so a build that lost it would decode every
// atom to its affiliation alone and nothing else would say so.
func TestTheGeneratedTableIsEmbedded(t *testing.T) {
	if len(atomPaths) < 500 {
		t.Fatalf("atomPaths holds %d entries; the generated table did not reach the binary",
			len(atomPaths))
	}

	for path, label := range atomPaths {
		if label == "" {
			t.Errorf("atomPaths[%q] is empty", path)
		}
		if strings.TrimSpace(label) != label {
			t.Errorf("atomPaths[%q] = %q, which is padded", path, label)
		}
		if strings.ContainsAny(path, ".*?") {
			t.Errorf("atomPaths carries the wildcard path %q, which nothing can match", path)
		}
	}
}

// Upstream leaves gaps: "A-M" is absent while "A" and "A-M-L" are both present.
// The walk keeps the deepest match rather than stopping at the first miss, so a
// gap costs nothing. An earlier version of this test asserted every path had its
// parent, which was a claim about a hand-written table and is not true of the
// catalog.
func TestADeeperPathResolvesThroughAMissingParent(t *testing.T) {
	if _, present := atomPaths["A-M"]; present {
		t.Skip("upstream now lists A-M; pick another gap to pin this on")
	}

	deep, ok := atomPaths["A-M-L"]
	if !ok {
		t.Fatal("A-M-L is absent, so this no longer tests a gap")
	}

	if got := decodeType("a-f-A-M-L").Label; got != "Friendly "+deep {
		t.Errorf("decodeType = %q, want the deeper label through the gap", got)
	}
}

// Case is part of the code, not decoration. The catalog distinguishes an upper
// from a lower case letter at the same position and means different things by
// them, so a lookup that folded case would answer one with the other.
func TestTypeCodesAreCaseSensitive(t *testing.T) {
	pairs := []struct{ lower, upper string }{
		{"a-f-G-I-r", "a-f-G-I-R"},
		{"a-f-G-U-C-V-R-s", "a-f-G-U-C-V-R-S"},
	}

	for _, pair := range pairs {
		lower := decodeType(pair.lower).Label
		upper := decodeType(pair.upper).Label

		if lower == "" || upper == "" {
			t.Fatalf("%s or %s decoded to nothing, so this pins nothing", pair.lower, pair.upper)
		}
		if lower == upper {
			t.Errorf("%s and %s both decode to %q; case was folded somewhere",
				pair.lower, pair.upper, lower)
		}
	}
}

// The affiliation is one character. A path is keyed below it, so the same row
// answers for every affiliation rather than one per affiliation.
func TestOneRowAnswersForEveryAffiliation(t *testing.T) {
	want := map[string]string{
		"a-f-A-C": "Friendly",
		"a-h-A-C": "Hostile",
		"a-u-A-C": "Unknown",
		"a-n-A-C": "Neutral",
	}

	shared, ok := atomPaths["A-C"]
	if !ok {
		t.Fatal("A-C is absent, so this pins nothing")
	}

	for raw, affiliation := range want {
		if got := decodeType(raw).Label; got != affiliation+" "+shared {
			t.Errorf("decodeType(%q) = %q, want the affiliation in front of %q", raw, got, shared)
		}
	}
}

// No two types read alike, anywhere in the table.
//
// A label is the whole of what the card says an event IS, so two paths sharing
// one is two different things a reader cannot tell apart. Upstream named each
// leaf by its own code letter alone, which collided in two ways: the civil and
// military air branches both said "Fixed Wing" and eleven fixed against rotary
// pairs said the same thing as each other, so a tanker was a tanker whether it
// was a KC-135 or a helicopter; and outside the air branch a further thirty-six
// labels repeated, "Fire" five times across incident states and "Station"
// across a radio mast, a TV mast, a surface picket and a submarine.
//
// Every label now carries whatever distinguishes it. Adding a row that repeats
// one fails here rather than reaching a reader.
func TestNoTwoTypesReadAlike(t *testing.T) {
	seen := make(map[string]string, len(atomPaths))

	for path, label := range atomPaths {
		if first, clash := seen[label]; clash {
			t.Errorf("%s and %s both read %q", first, path, label)
			continue
		}
		seen[label] = path
	}

	if len(seen) < 900 {
		t.Errorf("only %d paths; the table did not load", len(seen))
	}
}

// The same, across the hand-written tables, which reach the same row on the
// card and so must not repeat each other either.
func TestTheHandWrittenTypesDoNotRepeatTheTable(t *testing.T) {
	byLabel := make(map[string]string, len(atomPaths))
	for path, label := range atomPaths {
		byLabel[label] = path
	}

	for key, label := range wholeTypes {
		if path, clash := byLabel[label]; clash {
			t.Errorf("wholeTypes[%q] reads %q, which the atom path %s already reads", key, label, path)
		}
	}
}

// Every label reaches the same row on the card, so they are cased alike. A
// sentence-case entry beside a Title Case one reads as a bug rather than as a
// distinction.
func TestLabelsAreCasedAlike(t *testing.T) {
	// A word that is all lower case and is not one Title Case leaves alone.
	minor := map[string]bool{
		"and": true, "of": true, "to": true, "with": true, "than": true,
		"the": true, "a": true, "an": true, "for": true, "or": true, "in": true,
	}

	check := func(source, key, label string) {
		for i, word := range strings.Fields(label) {
			trimmed := strings.Trim(word, "()/,.-")
			if trimmed == "" || minor[trimmed] && i > 0 {
				continue
			}
			if first := trimmed[:1]; first == strings.ToLower(first) && first != strings.ToUpper(first) {
				t.Errorf("%s[%q] = %q: %q is lower case", source, key, label, trimmed)
			}
		}
	}

	for key, label := range wholeTypes {
		check("wholeTypes", key, label)
	}
	for key, label := range howSources {
		check("howSources", key, label)
	}
	for _, aff := range affiliations {
		check("affiliations", aff.id, aff.label)
	}
}

// The card says what the event says, and a Cursor on Target event states no
// country. The region was derived here from a polygon lookup on the position,
// which put a determination this plugin made into a row a reader would take for
// something the event reported.
//
// The accuracy rows stay, because ce, le and hae ARE point attributes and are
// the event's own words.
func TestNoCountryIsDerivedForAnEvent(t *testing.T) {
	props := propsFor(t, pointEvent(`lat="34.056100" lon="-118.250000" ce="45.3" le="99.5" hae="-42.6"`))

	if _, ok := props["region"]; ok {
		t.Errorf("props carry a region, which no event states: %v", props["region"])
	}

	for _, key := range []string{"ce", "le", "hae"} {
		if _, ok := props[key]; !ok {
			t.Errorf("props lost %q, which the point element does state", key)
		}
	}
}

func TestPropsCarryEveryEventInOrder(t *testing.T) {
	source := `<event uid="a" type="a-f-G-U-C" time="t"><point lat="1.00000" lon="2.00000"/></event>` +
		`<event uid="b" type="a-h-A-M-F" time="t"><point lat="3.00000" lon="4.00000"/></event>`

	events, err := Parse([]byte(source))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	props := Props(events, Source{Kind: SourceFence, Text: source})
	rendered, ok := props["events"].([]any)
	if !ok || len(rendered) != 2 {
		t.Fatalf("props carry %v, want two events", props["events"])
	}

	first := rendered[0].(map[string]any)
	second := rendered[1].(map[string]any)

	if first["uid"] != "a" || second["uid"] != "b" {
		t.Errorf("events are out of order: %v then %v", first["uid"], second["uid"])
	}
	if first["affiliation"] != "friend" || second["affiliation"] != "hostile" {
		t.Errorf("an event took another's affiliation")
	}
	if first["value"] == second["value"] {
		t.Error("both events resolved to one position")
	}
}

// The version is the migration lever, and a card built from a version a bundle
// does not know falls back rather than misreading. Bumping it is therefore a
// decision, not an edit.
func TestPropsDeclareTheVersionTheShapeIs(t *testing.T) {
	if PropsVersion != 2 {
		t.Errorf("PropsVersion is %d; the events array is version 2, and the webapp "+
			"reads version 1's single event for posts stamped before it", PropsVersion)
	}
}

// The relation ATAK writes on almost every event, which answers "who sent this".
func TestLinksSayWhoSentTheEvent(t *testing.T) {
	props := propsFor(t, `<event uid="u" type="a-f-G" time="t"><point lat="1.00000" lon="2.00000"/>`+
		`<detail><link uid="ANDROID-9" relation="p-p" parent_callsign="ALPHA"/></detail></event>`)

	if props["parent"] != "ALPHA" {
		t.Errorf("parent = %v, want the sending callsign", props["parent"])
	}
	if props["related"] != "ANDROID-9" {
		t.Errorf("related = %v, want the linked uid", props["related"])
	}
}

func TestAnEventWithNoLinksSaysNothingAboutThem(t *testing.T) {
	props := propsFor(t, pointEvent(`lat="1.00000" lon="2.00000"`))

	for _, key := range []string{"parent", "related"} {
		if _, ok := props[key]; ok {
			t.Errorf("props carry %q for an event that declared no relation", key)
		}
	}
}

// An atom this build cannot read is unnamed rather than half-named.
//
// decodeAtom has two ways to give up before it reaches the type table, and
// neither had a test: a raw type too short to carry an affiliation at all, and
// one whose affiliation character names nothing. Both must leave the label AND
// the affiliation empty, because a colour with no word beside it is exactly
// what the card's "colour is never the only channel" rule forbids.
func TestDecodeTypeGivesUpWholeOnAnAtomItCannotRead(t *testing.T) {
	for name, raw := range map[string]string{
		"no affiliation at all":                    "a-",
		"a multi-character affiliation":            "a-xx-G-U-C",
		"an affiliation letter that names nothing": "a-q-G-U-C",
	} {
		t.Run(name, func(t *testing.T) {
			decoded := decodeType(raw)

			if decoded.Label != "" {
				t.Errorf("Label = %q, want nothing", decoded.Label)
			}
			if decoded.Affiliation != "" {
				t.Errorf("Affiliation = %q, want nothing beside an empty label", decoded.Affiliation)
			}
		})
	}
}

// A numeric field that is not a number is dropped, not rendered as zero.
//
// numberOf's reject branch had no test because lat and lon are regex-guarded
// before they reach it. The other numeric fields are not, so this is the only
// thing standing between "ce=abc" and an accuracy row claiming 0 m.
func TestPropsDropANumericFieldThatIsNotANumber(t *testing.T) {
	for name, source := range map[string]string{
		"letters":             `<event uid="u" type="a-f-G" time="2026-08-09T16:30:00Z"><point lat="21.0" lon="-157.0" ce="abc"/></event>`,
		"not a number at all": `<event uid="u" type="a-f-G" time="2026-08-09T16:30:00Z"><point lat="21.0" lon="-157.0" ce="NaN"/></event>`,
	} {
		t.Run(name, func(t *testing.T) {
			events, err := Parse([]byte(source))
			if err != nil {
				t.Fatalf("Parse: %v", err)
			}

			row := Props(events, Source{})["events"].([]any)[0].(map[string]any)
			if _, ok := row["ce"]; ok {
				t.Errorf("ce = %v, want the field left out rather than rendered", row["ce"])
			}
			if _, ok := row["ce_meters"]; ok {
				t.Errorf("ce_meters = %v, want the field left out", row["ce_meters"])
			}
		})
	}
}

// Every codepoint the explicit bidi, isolate and BOM arms used to name.
//
// Those three arms sat below the Cf test and could never run, because all ten
// codepoints they named are in category Cf. They came out, and this is what the
// deletion rests on: it sweeps the ranges rather than sampling them, so a Go
// release that moved any of these out of Cf would fail here rather than quietly
// letting an override back into a callsign.
func TestTheCategoryTestCoversTheRangesItReplaced(t *testing.T) {
	var named []rune
	for r := rune(0x202A); r <= 0x202E; r++ {
		named = append(named, r)
	}
	for r := rune(0x2066); r <= 0x2069; r++ {
		named = append(named, r)
	}
	named = append(named, 0xFEFF)

	for _, r := range named {
		if got := sanitize("AL"+string(r)+"PHA", maxFieldRunes); got != "ALPHA" {
			t.Errorf("U+%04X survived sanitize: %q", r, got)
		}
	}
}

// decodeHow puts a method word in front of a source, and falls back to the
// source alone when the leading letter names no method.
//
// That fallback is unreachable today, which is the point: it is the only thing
// between a future howSources entry with an unlisted leading letter and a label
// rendered as ", GPS". Reaching it in a test would mean faking a table state, so
// the invariant it defends is pinned here instead. Read from the catalog rather
// than listed, per the repo's convention.
func TestEveryHowSourceHasAMethod(t *testing.T) {
	if len(howSources) == 0 {
		t.Fatal("howSources is empty, so this test is checking nothing")
	}

	for code := range howSources {
		if code == "" {
			t.Error("howSources holds an empty code")
			continue
		}
		if _, ok := howMethods[code[0]]; !ok {
			t.Errorf("howSources has %q and howMethods has no %q, so its label would begin with a comma", code, code[0])
		}
	}
}
