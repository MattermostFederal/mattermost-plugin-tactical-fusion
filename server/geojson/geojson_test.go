package geojson

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/mattermost/mattermost/server/public/model"
)

func fixtureDocument(t *testing.T) *Document {
	t.Helper()
	return parse(t, Fixture())
}

func TestFixtureCoversEveryKind(t *testing.T) {
	// The guard's coverage has to come from the fixture, so a kind added later
	// cannot quietly fall outside it.
	seen := map[Kind]bool{}
	for _, feature := range fixtureDocument(t).Features {
		seen[feature.Geometry.Kind] = true
	}

	for _, kind := range Kinds {
		if !seen[kind] {
			t.Errorf("Fixture carries no %s feature", kind)
		}
	}
}

// Every note this package can attach is reachable from a document, and the list
// is READ rather than named: the earlier version hardcoded two of the seven, so
// five were uncovered while the test claimed otherwise.
func TestEveryNoteIsReachable(t *testing.T) {
	reachable := map[string]bool{}

	for _, source := range []string{
		Fixture(),
		`{"type":"Polygon","coordinates":[[[0,0],[1,0],[1,1],[0,1]]]}`,
		`{"type":"Polygon","coordinates":[[[0,0],[1,0],[0,0]]]}`,
		`{"type":"LineString","coordinates":[[1,2]]}`,
		`{"type":"FeatureCollection","crs":{"type":"name","properties":{"name":"EPSG:4326"}},"features":[]}`,
		`{"type":"FeatureCollection","bbox":[1,2,3],"features":[{"type":"Feature","geometry":null,"properties":{}}]}`,
		`{"type":"FeatureCollection","features":[]}`,
	} {
		document := parse(t, source)
		reachable[document.Note] = true
		for _, feature := range document.Features {
			reachable[feature.Note] = true
		}
	}

	for _, note := range Notes {
		if !reachable[note] {
			t.Errorf("no fixture in this test produces %q", note)
		}
	}
}

func TestNamePrecedence(t *testing.T) {
	cases := []struct {
		name   string
		source string
		want   string
	}{
		{"name wins", `{"type":"Feature","id":"i","geometry":null,"properties":{"name":"N","title":"T","label":"L"}}`, "N"},
		{"then title", `{"type":"Feature","id":"i","geometry":null,"properties":{"title":"T","label":"L"}}`, "T"},
		{"then label", `{"type":"Feature","id":"i","geometry":null,"properties":{"label":"L"}}`, "L"},
		{"then id", FixtureID(), "NAMED-BY-ID"},
		{"then the index", `{"type":"Feature","geometry":null,"properties":{}}`, "Feature 1"},
		{"blank name falls through", `{"type":"Feature","geometry":null,"properties":{"name":"   ","title":"T"}}`, "T"},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			if got := parse(t, testCase.source).Features[0].Name; got != testCase.want {
				t.Fatalf("name=%q, want %q", got, testCase.want)
			}
		})
	}
}

func TestNameSurvivesTheLowerRung(t *testing.T) {
	// The whole reason the name is hoisted out of the properties bag.
	document := parse(t, `{"type":"Feature","geometry":null,"properties":{"name":"Kept"}}`)

	blob := PropsWithoutProperties(document, Source{Kind: SourceFence})
	feature := blob["features"].([]any)[0].(map[string]any)

	if feature["name"] != "Kept" {
		t.Fatalf("name=%v", feature["name"])
	}
	if _, present := feature["properties"]; present {
		t.Fatal("the lower rung carried properties")
	}
}

func TestPropertyValueRendering(t *testing.T) {
	source := `{"type":"Feature","geometry":null,"properties":{` +
		`"s":"text","n":12.50,"b":true,"z":null,"arr":[1,"two"],"obj":{"k":1}}}`

	properties := parse(t, source).Features[0].Properties

	got := map[string]string{}
	for _, property := range properties {
		got[property.Key] = property.Value
	}

	want := map[string]string{
		"s":   "text",
		"n":   "12.50",
		"b":   "true",
		"arr": `[1,"two"]`,
		"obj": `{"k":1}`,
	}

	for key, value := range want {
		if got[key] != value {
			t.Errorf("%s=%q, want %q", key, got[key], value)
		}
	}

	// A null is dropped along with its key: rendering it as "null" would state
	// a value the document did not carry.
	if _, present := got["z"]; present {
		t.Error("a null property was rendered")
	}
}

func TestPropertiesAreCappedAndOrdered(t *testing.T) {
	pairs := make([]string, 0, maxProperties+10)
	for i := range maxProperties + 10 {
		pairs = append(pairs, `"k`+string(rune('a'+i%26))+string(rune('a'+i/26))+`":1`)
	}
	source := `{"type":"Feature","geometry":null,"properties":{` + strings.Join(pairs, ",") + `}}`

	properties := parse(t, source).Features[0].Properties
	if len(properties) != maxProperties {
		t.Fatalf("properties=%d, want %d", len(properties), maxProperties)
	}

	for i := 1; i < len(properties); i++ {
		if properties[i-1].Key > properties[i].Key {
			t.Fatal("properties are not in a stable order")
		}
	}
}

func TestTheWidestRungCarriesNoDroppedMarker(t *testing.T) {
	blob := Props(fixtureDocument(t), Source{Kind: SourceFence})

	if _, present := blob["properties_dropped"]; present {
		t.Fatal("the widest rung marked itself degraded")
	}

	lower := PropsWithoutProperties(fixtureDocument(t), Source{Kind: SourceFence})
	if lower["properties_dropped"] != "1" {
		t.Fatal("the lower rung did not mark itself")
	}
}

func TestSourceIsTruncatedRatherThanWithheld(t *testing.T) {
	// Once Post.Type is set the webapp never reads post.message, so this is the
	// only copy of the document any reader can reach.
	document := parse(t, `{"type":"Point","coordinates":[1,2]}`)

	blob := Props(document, Source{Kind: SourceFence, Text: strings.Repeat("x", maxInlineSrcRunes+50)})

	src, ok := blob["src"].(string)
	if !ok || src == "" {
		t.Fatal("src was withheld")
	}
	if !strings.HasSuffix(src, truncationMarker) {
		t.Fatal("src was truncated without a visible marker")
	}
}

func TestCounts(t *testing.T) {
	counts := fixtureDocument(t).Counts()

	if counts.Features != 9 {
		t.Errorf("features=%d, want 9", counts.Features)
	}
	if counts.Unlocated != 1 {
		t.Errorf("unlocated=%d, want 1", counts.Unlocated)
	}
	if counts.Undrawable != 1 {
		t.Errorf("undrawable=%d, want 1", counts.Undrawable)
	}
	if counts.Collections != 1 {
		t.Errorf("collections=%d, want 1", counts.Collections)
	}
}

// TestMaxVerticesFitsThePropsBudget derives the cap rather than asserting it.
//
// The number cannot be chosen by eye: props are measured whole, and the blob
// carries each position as an object of lexemes, so the encoded cost per vertex
// is several times the source cost. This solves for the worst case with every
// OTHER cap at its maximum, which is the non-circular form of the question.
//
// The binding case is LOW precision. At realistic precision the byte cap stops
// a document long before MaxVertices does, which is what
// TestTheLargestDocumentFitsThePropsBudget measures; MaxVertices exists for
// "[1,2]", where 64 KiB holds far more positions than the map should draw.
func TestMaxVerticesFitsThePropsBudget(t *testing.T) {
	positions := make([]string, 0, MaxVertices)
	for range MaxVertices {
		positions = append(positions, `[1,2]`)
	}

	source := `{"type":"MultiPoint","coordinates":[` + strings.Join(positions, ",") + `]}`

	if len(source) > MaxSourceBytes {
		t.Fatalf("a document at MaxVertices is %d bytes, over the %d source cap, so the vertex cap can never bind",
			len(source), MaxSourceBytes)
	}

	document, err := Parse([]byte(source))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	encoded, err := json.Marshal(Props(document, Source{Kind: SourceFence, Text: source}))
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	runes := utf8.RuneCountInString(string(encoded))
	if runes > model.PostPropsMaxUserRunes {
		t.Fatalf("a document at MaxVertices encodes to %d runes, over the %d budget",
			runes, model.PostPropsMaxUserRunes)
	}

	t.Logf("at MaxVertices=%d: %d source bytes, %d props runes of %d (%.0f%%)",
		MaxVertices, len(source), runes, model.PostPropsMaxUserRunes,
		100*float64(runes)/float64(model.PostPropsMaxUserRunes))
}

// TestMaxVerticesBindsBeforeTheByteCapAtLowPrecision is the other half: the cap
// has to be reachable, or it is a constant that refuses nothing.
func TestMaxVerticesBindsBeforeTheByteCapAtLowPrecision(t *testing.T) {
	source := manyVertices(MaxVertices + 1)

	if len(source) > MaxSourceBytes {
		t.Fatalf("MaxVertices+1 is already %d bytes, over the %d source cap: the vertex cap is unreachable",
			len(source), MaxSourceBytes)
	}

	if _, err := Parse([]byte(source)); !errors.Is(err, ErrTooManyVertices) {
		t.Fatalf("err=%v, want %v", err, ErrTooManyVertices)
	}
}

// TestTheLargestDocumentFitsThePropsBudget is the same question asked of the
// cap that actually binds, which is the byte cap.
func TestTheLargestDocumentFitsThePropsBudget(t *testing.T) {
	var positions []string
	source := ""
	for {
		next := make([]string, len(positions), len(positions)+1)
		copy(next, positions)
		next = append(next, `[-118.2500000,34.0561000]`)

		candidate := `{"type":"MultiPoint","coordinates":[` + strings.Join(next, ",") + `]}`
		if len(candidate) > MaxSourceBytes {
			break
		}
		positions, source = next, candidate
	}

	document, err := Parse([]byte(source))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	encoded, err := json.Marshal(Props(document, Source{Kind: SourceFence, Text: source}))
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	runes := utf8.RuneCountInString(string(encoded))
	if runes > model.PostPropsMaxUserRunes {
		t.Fatalf("the largest readable document encodes to %d runes, over the %d budget",
			runes, model.PostPropsMaxUserRunes)
	}

	t.Logf("largest document: %d positions, %d source bytes, %d props runes of %d (%.0f%%)",
		len(positions), len(source), runes, model.PostPropsMaxUserRunes,
		100*float64(runes)/float64(model.PostPropsMaxUserRunes))
}

// A lone point carries the pair the location tools take, and nothing else does.
//
// A polygon has no one position and a MultiPoint has several, so linking either
// would be picking one and calling it the feature's.
func TestOnlyALonePointCarriesALocationLink(t *testing.T) {
	cases := []struct {
		name   string
		source string
		linked bool
	}{
		{"a lone point", `{"type":"Point","coordinates":[-118.250000,34.056100]}`, true},
		{"a multi point", `{"type":"MultiPoint","coordinates":[[1,2],[3,4]]}`, false},
		{"a line", `{"type":"LineString","coordinates":[[1,2],[3,4]]}`, false},
		{"a polygon", fixturePolygon, false},
		{"an undrawable point", `{"type":"Point","coordinates":[999,34.05]}`, false},
		{"an unlocated feature", `{"type":"Feature","geometry":null,"properties":{}}`, false},

		// Written too coarsely for the location grammar to stand behind, which
		// is the same refusal cot's coarseNote records. The row is still shown,
		// as text.
		{"a coarse point", `{"type":"Point","coordinates":[-118.2,34.0]}`, false},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			blob := Props(parse(t, testCase.source), Source{Kind: SourceFence})
			feature := blob["features"].([]any)[0].(map[string]any)

			_, hasFormat := feature["format"]
			_, hasValue := feature["value"]

			if hasFormat != testCase.linked || hasValue != testCase.linked {
				t.Fatalf("format=%v value=%v, want both %v", hasFormat, hasValue, testCase.linked)
			}
		})
	}
}

// The pair is the identity only. Nothing derived travels with it, which is what
// lets every route re-derive the rest and refuse a link that disagrees.
func TestTheLocationLinkCarriesOnlyTheIdentity(t *testing.T) {
	blob := Props(parse(t, `{"type":"Point","coordinates":[-118.250000,34.056100]}`), Source{Kind: SourceFence})
	feature := blob["features"].([]any)[0].(map[string]any)

	if feature["format"] != "dd" {
		t.Errorf("format = %v, want dd", feature["format"])
	}
	if feature["value"] == "" {
		t.Error("value is empty")
	}
}

// A numeric id was the one author string stored with neither sanitize nor a
// cap, so a 60,000-digit id became a 60,000-rune name and ate the props budget.
func TestANumericIDIsCappedLikeEveryOtherName(t *testing.T) {
	huge := strings.Repeat("9", 60000)
	document := parse(t, `{"type":"Feature","id":`+huge+`,"geometry":null,"properties":{}}`)

	if got := utf8.RuneCountInString(document.Features[0].Name); got > maxNameRunes+1 {
		t.Fatalf("name is %d runes, want at most %d plus a marker", got, maxNameRunes)
	}
}

/*
 * stripUnsafe removes what it claims to remove.
 *
 * This is the sanitizer on author text: feature names and property values come
 * off a document anybody can post and are rendered on three surfaces. The keep
 * branch for tab and newline was covered; nothing executed the three removal
 * branches, so a build that dropped them would have passed.
 *
 * The Cf case is the one with teeth. U+202E RIGHT-TO-LEFT OVERRIDE reorders
 * everything after it when rendered, so a feature name carrying one can be made
 * to display as something other than what it says.
 *
 * Written as \u escapes, not as literal runes: bidichk and gosec refuse a
 * literal one in source (G116, Trojan Source), and they are right to. A test
 * for a bidi hazard must not itself put a bidi character in a file somebody
 * reads.
 */
func TestUnsafeRunesAreStrippedFromAuthorText(t *testing.T) {
	for _, tc := range []struct {
		name string
		raw  string
		want string
	}{
		{"a carriage return", "Depot\rNorth", "DepotNorth"},
		{"a control character", "Depot\x00North", "DepotNorth"},
		{"a bell", "Depot\aNorth", "DepotNorth"},
		{"a right-to-left override", "Depot\u202eNorth", "DepotNorth"},
		{"a zero width joiner", "Depot\u200dNorth", "DepotNorth"},

		// Kept: these are the two the switch admits, and a strip that took
		// them would flatten every multi-line remark in a document.
		{"a tab", "Depot\tNorth", "Depot\tNorth"},
		{"a newline", "Depot\nNorth", "Depot\nNorth"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			encoded, err := json.Marshal(tc.raw)
			if err != nil {
				t.Fatalf("could not encode the fixture: %v", err)
			}

			document := parse(t, `{"type":"Feature","geometry":{"type":"Point","coordinates":[1,2]},`+
				`"properties":{"name":`+string(encoded)+`}}`)

			if got := document.Features[0].Name; got != tc.want {
				t.Errorf("name = %q, want %q", got, tc.want)
			}
		})
	}
}

/*
 * Only a lone Point becomes a coordinate link.
 *
 * The webapp turns `format`/`value` into the link, which carries the "a link
 * may never disagree with itself" contract: a feature holding several positions
 * has no single one to name.
 *
 * What this actually exercises is the KIND gate. Mutation testing showed the
 * two `len(...)` checks below it are unreachable through the public API, since
 * the walker gives a Point exactly one part of one ring of one position; they
 * are defensive, like the type assertions elsewhere in this package, and are
 * deliberately left uncovered rather than reached with a hand-built value the
 * parser cannot produce.
 */
func TestOnlyASinglePositionBecomesALink(t *testing.T) {
	linked := func(source string) bool {
		document := parse(t, source)
		props := Props(document, Source{Kind: SourceFence, Text: source})

		features, ok := props["features"].([]any)
		if !ok || len(features) == 0 {
			t.Fatalf("the props carry no features: %v", props)
		}

		feature, ok := features[0].(map[string]any)
		if !ok {
			t.Fatalf("a feature is %T, not a record", features[0])
		}

		_, named := feature["value"]

		return named
	}

	if !linked(`{"type":"Feature","geometry":{"type":"Point","coordinates":[-118.250000,34.056100]},"properties":{}}`) {
		t.Error("a lone point carries no link, so nothing in the panel is clickable")
	}

	if linked(`{"type":"Feature","geometry":{"type":"MultiPoint",` +
		`"coordinates":[[-118.250000,34.056100],[-118.240000,34.060000]]},"properties":{}}`) {
		t.Error("a feature of several positions was linked, so the link names one of them and hides the rest")
	}

	if linked(`{"type":"Feature","geometry":{"type":"GeometryCollection","geometries":[` +
		`{"type":"Point","coordinates":[-118.250000,34.056100]},` +
		`{"type":"Point","coordinates":[-118.240000,34.060000]}]},"properties":{}}`) {
		t.Error("a collection of several parts was linked")
	}
}
