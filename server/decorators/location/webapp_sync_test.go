package location

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/decorators"
	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/decorators/location/mapdata"
)

// The webapp keeps its own copy of the canonical shapes, and this is the seam
// where the two drift.
//
// The webapp cannot share the grammar: it is Go, and reimplementing it in
// TypeScript would be a permanent source of exactly this. What it does instead
// is validate the SHAPE of the token a link carries, which needs the same
// letter classes, which is a smaller duplication and still a duplication.
//
// It has already cost once. The band class was widened here to read N and S as
// latitude bands, the webapp kept the older narrower one, and a UTM link the
// server had just issued failed the webapp's check. That failure is silent by
// construction: the click handler reads a null payload as "not one of ours",
// stands aside, and the browser follows the link to the standalone page. The
// page renders correctly, so it looks like a routing choice rather than a
// rejected payload, and nothing is logged on either side.
func TestWebappBandClassMatches(t *testing.T) {
	source := readWebappSource(t, "format.ts")

	// The webapp writes the class once and builds both grid patterns from it,
	// for the same reason bandBody is written once here.
	m := regexp.MustCompile(`const BAND = '\[([^\]]+)\]'`).FindStringSubmatch(source)
	if m == nil {
		t.Fatal("no `const BAND = '[...]'` in the webapp's format.ts; " +
			"if it was renamed, point this test at the new name rather than deleting it")
	}

	if m[1] != bandBody {
		t.Errorf("the webapp band class is %q and this package's is %q.\n"+
			"They must agree: a link this side issues and that side rejects opens the "+
			"standalone page instead of the sidebar, silently.", m[1], bandBody)
	}
}

// The column and row classes are the same duplication, one step further in, and
// they have to be in the right POSITION as well as present.
//
// An earlier version searched for each class as a substring anywhere in the
// file. Swapping the two in the mgrs pattern left both substrings present and
// the test green, while the webapp then rejected every column letter after V
// and accepted row letters the server never emits. That failure is silent by
// construction: the click handler reads a null payload as "not ours" and the
// browser follows the link to the standalone page.
//
// So this parses the mgrs pattern itself and checks each group in order.
func TestWebappGridPatternUsesTheRightClasses(t *testing.T) {
	source := readWebappSource(t, "format.ts")

	m := regexp.MustCompile(
		`mgrs: new RegExp\(` + "`" + `\^\(\$\{ZONE\}\)\(\$\{BAND\}\)\(\[([^\]]+)\]\)\(\[([^\]]+)\]\)`,
	).FindStringSubmatch(source)
	if m == nil {
		t.Fatal("could not parse the webapp's mgrs pattern; if its shape changed, " +
			"point this test at the new one rather than deleting it")
	}

	if m[1] != colBody {
		t.Errorf("the webapp's COLUMN class is %q and this package's is %q", m[1], colBody)
	}
	if m[2] != rowBody {
		t.Errorf("the webapp's ROW class is %q and this package's is %q", m[2], rowBody)
	}
}

func TestWebappAreaAlphabetsMatch(t *testing.T) {
	source := readWebappSource(t, "format.ts")

	for _, tc := range []struct {
		name string
		want string
	}{
		{"GEOREF_ZONE", georefZoneBody},
		{"GEOREF_BAND", georefBandBody},
		{"GEOREF_UNIT", georefUnitBody},
		{"GARS_LETTER", garsLetterBody},
		{"OLC_CHAR", olcBody},
	} {
		m := regexp.MustCompile(`const ` + tc.name + ` = '\[([^\]]+)\]'`).FindStringSubmatch(source)
		if m == nil {
			t.Errorf("no `const %s = '[...]'` in the webapp's format.ts; if it was renamed, "+
				"point this test at the new name rather than deleting it", tc.name)
			continue
		}

		if m[1] != tc.want {
			t.Errorf("the webapp's %s class is %q and this package's is %q.\n"+
				"They must agree: a link this side issues and that side rejects opens the "+
				"standalone page instead of the sidebar, silently.", tc.name, m[1], tc.want)
		}
	}
}

// And both grid patterns must be built FROM the shared class rather than
// spelling a band class of their own, or widening BAND would fix nothing.
func TestWebappGridPatternsUseTheSharedBandClass(t *testing.T) {
	source := readWebappSource(t, "format.ts")

	for _, name := range []string{"mgrs", "utm"} {
		if !regexp.MustCompile(name + `: new RegExp\(` + "`" + `\^\(\$\{ZONE\}\)\(\$\{BAND\}\)`).MatchString(source) {
			t.Errorf("the webapp's %s pattern does not use ${BAND}", name)
		}
	}
}

func TestWebappAreaResolutionFixturesMatch(t *testing.T) {
	source := readWebappSource(t, "format.spec.ts")

	found := regexp.MustCompile(
		`\['(georef|gars|pluscode)', '([^']*)', '([^']*)'\],`).FindAllStringSubmatch(source, -1)
	if len(found) == 0 {
		t.Fatal("no area fixtures parsed out of the webapp's format.spec.ts; if the table's " +
			"shape changed, point this test at the new one rather than deleting it")
	}

	// The webapp table carries one extra row, the four-letter GEOREF quadrangle
	// it must refuse, which has no Go counterpart because Parse declines it.
	var comparable [][]string
	for _, m := range found {
		if m[3] != "" {
			comparable = append(comparable, m)
		}
	}

	if len(comparable) != len(areaFixtures) {
		t.Fatalf("the webapp has %d area fixtures with a resolution and this package has %d",
			len(comparable), len(areaFixtures))
	}

	for i, m := range comparable {
		want := areaFixtures[i]

		if got := Format(m[1]); got != want.format {
			t.Errorf("area fixture %d is format %q in the webapp and %q here", i+1, got, want.format)
		}
		if m[2] != want.canonical {
			t.Errorf("area fixture %d is %q in the webapp and %q here", i+1, m[2], want.canonical)
		}
		if m[3] != want.resolution {
			t.Errorf("area fixture %d (%s): resolution is %q in the webapp and %q here",
				i+1, want.canonical, m[3], want.resolution)
		}
	}
}

func readWebappSource(t *testing.T, name string) string {
	t.Helper()

	path := filepath.Join("..", "..", "..", "webapp", "src", "decorators", "location", name)
	raw, err := os.ReadFile(path) // #nosec G304 -- fixed, repo-relative source path
	if err != nil {
		t.Fatalf("could not read %s: %v", path, err)
	}

	return string(raw)
}

// The format list is duplicated in the webapp, and it has to be the same set.
//
// This was the one cross-language list nothing compared, and its failure was the
// quietest of the lot. `readPageData` refused a format id LOCATION_FORMATS did
// not carry, and the standalone page, which is what the mobile app opens, then
// rendered a wholly blank document: no rows, no note, nothing logged, while the
// conversion the server had already computed sat unread in the shell. So adding
// a tenth grammar in Go alone shipped blank pages with every test still green.
//
// The page degrades rather than blanking now, which is the real fix. This is the
// other half: a build whose two halves disagree about what a format id is should
// fail here, in Go, with the reason spelled out, rather than on somebody's phone.
//
// The ORDER is deliberately not compared. Nothing renders from this list, so
// only membership carries meaning.
func TestWebappFormatListMatches(t *testing.T) {
	path := filepath.Join("..", "..", "..", "webapp", "src", "decorators", "location", "format.ts")
	raw, err := os.ReadFile(path) // #nosec G304 -- fixed, repo-relative source path
	if err != nil {
		t.Fatalf("could not read %s: %v", path, err)
	}

	block := regexp.MustCompile(
		`export const LOCATION_FORMATS: readonly LocationFormat\[\] = \[([^\]]*)\]`).
		FindStringSubmatch(string(raw))
	if block == nil {
		t.Fatal("could not find LOCATION_FORMATS in format.ts")
	}

	var got []string
	for _, m := range regexp.MustCompile(`'([a-z]+)'`).FindAllStringSubmatch(block[1], -1) {
		got = append(got, m[1])
	}

	var want []string
	for _, f := range AllFormatIDs {
		want = append(want, string(f))
	}

	slices.Sort(got)
	slices.Sort(want)

	if !slices.Equal(got, want) {
		t.Errorf("LOCATION_FORMATS = %v, AllFormatIDs = %v", got, want)
	}
}

// The row catalog is duplicated in the webapp, and it has to be the same list
// in the same order.
//
// Three things name these rows: this package renders the standalone page from
// them, the panel renders itself from them in TypeScript, and a reader's stored
// settings hide them by id. A row present in two of those and not the third is
// invisible in a different way each time, and none of them is an error: a row
// the webapp has and this package does not cannot be hidden, because the server
// refuses the id; a row this package has and the webapp does not appears on the
// page and nowhere else.
//
// The ORDER matters too, since it is the order both surfaces render in and the
// order the editor lists.
func TestWebappRowCatalogMatches(t *testing.T) {
	source := readWebappSource(t, "rows.ts")

	found := regexp.MustCompile(
		`\{id: '([a-z]+)', label: '([^']*)', copyable: (true|false)`).FindAllStringSubmatch(source, -1)
	if len(found) == 0 {
		t.Fatal("no rows parsed out of the webapp's rows.ts; if the shape changed, " +
			"point this test at the new one rather than deleting it")
	}

	if len(found) != len(Rows) {
		t.Fatalf("the webapp has %d rows and this package has %d", len(found), len(Rows))
	}

	for i, m := range found {
		if m[1] != Rows[i].ID {
			t.Errorf("row %d is %q in the webapp and %q here; the two must be the same list "+
				"in the same order", i+1, m[1], Rows[i].ID)
		}

		// The label is what a reader matches the editor's checkbox against the
		// panel's row by, so the two saying different things is its own bug.
		if m[2] != Rows[i].Label {
			t.Errorf("row %q is labeled %q in the webapp and %q here", m[1], m[2], Rows[i].Label)
		}

		// And whether it carries a copy control, which is the field the two
		// surfaces most visibly disagree about: a copy button over "stated
		// confidence 9 (latitude)" on one and not the other.
		if copyable := m[3] == "true"; copyable != Rows[i].Copyable {
			t.Errorf("row %q is copyable=%v in the webapp and %v here", m[1], copyable, Rows[i].Copyable)
		}
	}
}

// The shared render fixtures are the same table on both sides.
//
// format_test.go says "Change one and change the other" about the table in
// webapp/src/decorators/location/format.spec.ts, and until now nothing enforced
// it: the two were independent literals, so editing or deleting a row on either
// side was green on both. The mechanism to check it already existed for the
// letter classes.
//
// Only the canonical token and the columns BOTH sides compute are compared. The
// position rows are Go's alone, because in the running plugin they are produced
// in Go alone.
func TestWebappRenderFixturesMatch(t *testing.T) {
	source := readWebappSource(t, "format.spec.ts")

	// [format, canonical, decimal, dms, ddm, usmtf, resolution]
	found := regexp.MustCompile(
		`\['(dd|ddh|dms|ddm|latd|latm|vlatm)', '([^']*)', '((?:[^'\\]|\\.)*)',\s*`+
			`'((?:[^'\\]|\\.)*)', '((?:[^'\\]|\\.)*)',\s*`+
			`'((?:[^'\\]|\\.)*)', '((?:[^'\\]|\\.)*)'\],`,
	).FindAllStringSubmatch(source, -1)

	if len(found) == 0 {
		t.Fatal("no fixtures parsed out of the webapp's format.spec.ts; if the table's " +
			"shape changed, point this test at the new one rather than deleting it")
	}

	if len(found) != len(renderFixtures) {
		t.Fatalf("the webapp has %d render fixtures and this package has %d",
			len(found), len(renderFixtures))
	}

	unescape := strings.NewReplacer(`\'`, `'`, `\"`, `"`, `\\`, `\`)

	for i, m := range found {
		want := renderFixtures[i]

		if got := Format(m[1]); got != want.format {
			t.Errorf("fixture %d is format %q in the webapp and %q here", i+1, got, want.format)
		}
		for _, col := range []struct {
			name      string
			got, want string
		}{
			{"canonical", m[2], want.canonical},
			{"decimal", unescape.Replace(m[3]), want.decimal},
			{"dms", unescape.Replace(m[4]), want.dms},
			{"ddm", unescape.Replace(m[5]), want.ddm},
			{"usmtf", unescape.Replace(m[6]), want.usmtf},
			{"resolution", unescape.Replace(m[7]), want.resolution},
		} {
			if col.got != col.want {
				t.Errorf("fixture %d (%s): %s is %q in the webapp and %q here",
					i+1, want.canonical, col.name, col.got, col.want)
			}
		}
	}
}

// The conversion payload is a wire contract between two languages, and it was
// the one seam here with nothing holding it together.
//
// The panel renders these six strings verbatim; it cannot check them and does
// not try, because `response.json() as Promise<Conversion>` is an unchecked
// cast. So a field renamed on either side compiles, type-checks and ships: the
// Go tag moves and the TypeScript property reads `undefined`, or the interface
// moves and does the same, and the row goes blank on every location link. The
// component test cannot catch it either, since its route stub supplies a
// payload built from the TypeScript type and is therefore self-consistent.
//
// That is the same silent-failure family as the band class above, which is why
// this reads the interface out of the webapp rather than trusting review.
func TestWebappConversionShapeMatches(t *testing.T) {
	source := readWebappSource(t, "convert.ts")

	block := regexp.MustCompile(`(?s)export interface Conversion \{(.*?)\n\}`).FindStringSubmatch(source)
	if block == nil {
		t.Fatal("no `export interface Conversion` in the webapp's convert.ts; if it was " +
			"renamed, point this test at the new name rather than deleting it")
	}

	var webapp []string
	for _, m := range regexp.MustCompile(`(?m)^\s+(\w+):\s*(\w+);`).FindAllStringSubmatch(block[1], -1) {
		webapp = append(webapp, m[1]+" "+m[2])
	}

	// Names AND types, not names alone. While every field was a string, name
	// agreement implied type agreement; with numbers on the wire a TypeScript
	// `lat: string` against a Go float64 type-checks, ships, and puts the pin
	// at NaN.
	var server []string
	for field := range reflect.TypeFor[Conversion]().Fields() {
		tag, ok := field.Tag.Lookup("json")
		if !ok {
			t.Fatalf("Conversion.%s has no json tag, so the webapp cannot read it", field.Name)
		}
		ts, ok := tsTypeFor(field.Type.Kind())
		if !ok {
			t.Fatalf("Conversion.%s is a %s, which this test does not know how to "+
				"compare against TypeScript", field.Name, field.Type.Kind())
		}
		server = append(server, strings.Split(tag, ",")[0]+" "+ts)
	}

	if !slices.Equal(server, webapp) {
		t.Errorf("the conversion payload is %v here and %v in the webapp.\n"+
			"They must agree field for field, type for type, and in order: a rename or a "+
			"changed type on either side is silent, and shows up as a permanently blank "+
			"row or a pin at NaN.",
			server, webapp)
	}
}

func tsTypeFor(k reflect.Kind) (string, bool) {
	switch k {
	case reflect.String:
		return "string", true
	case reflect.Float64, reflect.Float32:
		return "number", true
	case reflect.Bool:
		return "boolean", true
	default:
		return "", false
	}
}

// And the shared zone class has to mean 1 to 60, which is what gridPoint
// enforces on the way to a position.
//
// Both webapp grid patterns used a bare \d{1,2}, which admits 00 and 61 to 99.
// Go's scanning grammar is equally loose, so the two matched, but the server
// never ISSUES such a link: gridPoint refuses the zone, so the page answers 400
// and the decorator declines the token. The webapp accepting it was a split in
// the opposite direction from the band class, and just as silent.
func TestWebappGridZoneIsBounded(t *testing.T) {
	source := readWebappSource(t, "format.ts")

	m := regexp.MustCompile(`const ZONE = '([^']+)'`).FindStringSubmatch(source)
	if m == nil {
		t.Fatal("no `const ZONE = '...'` in the webapp's format.ts; if it was renamed, " +
			"point this test at the new name rather than deleting it")
	}

	// Asserted behaviourally rather than by comparing the expression text, so a
	// different but equivalent spelling is free to differ.
	zone := regexp.MustCompile(`^(?:` + m[1] + `)$`)

	for n := range 100 {
		// Both spellings, with nothing skipped. "0" and "00" are different
		// strings and both must be refused; an earlier version skipped the
		// padded one as redundant and so never tested "00" at all, which is
		// the zero-padded invalid zone this is most concerned with.
		for _, spelled := range []string{strconv.Itoa(n), fmt.Sprintf("%02d", n)} {
			want := n >= 1 && n <= 60

			if got := zone.MatchString(spelled); got != want {
				t.Errorf("the webapp ZONE class matches %q = %v, want %v; "+
					"gridPoint accepts zones 1 to 60 and nothing else", spelled, got, want)
			}
		}
	}
}

// The map's geometry constants live in two places, this package and span.ts,
// and every surface draws from span.ts. Go keeps them only as the anchors these
// are compared against, so a change on either side that does not reach the other
// fails here rather than in a browser.
func TestWebappMapConstantsMatch(t *testing.T) {
	span := readWebappSource(t, "map/span.ts")

	cases := []struct {
		name   string
		server float64
	}{
		{"MERCATOR_LIMIT", mapdata.MercatorLimit},
		{"DEGREE_METERS", degreeMeters},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if webapp := tsConstant(t, span, "map/span.ts", c.name); webapp != c.server {
				t.Errorf("%s is %v here and %v in the webapp", c.name, c.server, webapp)
			}
		})
	}
}

// Reads a `const NAME = <number>;` out of either language.
func tsConstant(t *testing.T, source, where, name string) float64 {
	t.Helper()

	m := regexp.MustCompile(`const ` + name + ` = ([0-9.]+);`).FindStringSubmatch(source)
	if m == nil {
		t.Fatalf("no `const %s` in %s; if it was renamed, point this test at the new "+
			"name rather than deleting it", name, where)
	}

	v, err := strconv.ParseFloat(m[1], 64)
	if err != nil {
		t.Fatalf("%s = %q, which is not a number", name, m[1])
	}

	return v
}

// The map is hideable and is not a row, so it is carried by its own constant on
// both sides. A rename is silent: the reader unticks Map, the server stores it,
// and the webapp's forgiving read drops it again on the next load.
func TestWebappMapSectionIDMatches(t *testing.T) {
	source := readWebappSource(t, "map/../rows.ts")

	m := regexp.MustCompile(`export const MAP_ID = '([a-z]+)';`).FindStringSubmatch(source)
	if m == nil {
		t.Fatal("no `export const MAP_ID` in rows.ts")
	}
	if m[1] != SectionMap {
		t.Errorf("the map section is %q here and %q in the webapp", SectionMap, m[1])
	}
}

// The map under a post is hideable, is not a row, and is a second thing a
// rename would break silently in exactly the way the panel map's would.
func TestWebappInlineSectionIDMatches(t *testing.T) {
	source := readWebappSource(t, "map/../rows.ts")

	m := regexp.MustCompile(`export const INLINE_ID = '([a-z]+)';`).FindStringSubmatch(source)
	if m == nil {
		t.Fatal("no `export const INLINE_ID` in rows.ts")
	}
	if m[1] != SectionInline {
		t.Errorf("the inline section is %q here and %q in the webapp", SectionInline, m[1])
	}
}

// The custom post type is a third cross-language string, and a mismatch is the
// quietest kind: the server stamps a type this build's webapp registered
// nothing for, Mattermost falls through to ordinary markdown, and the reader
// simply never sees a map. Nothing is logged on either side.
func TestWebappPostTypeMatches(t *testing.T) {
	source := readWebappSource(t, "index.ts")

	m := regexp.MustCompile(`postType: '([a-z_]+)',`).FindStringSubmatch(source)
	if m == nil {
		t.Fatal("no `postType:` in index.ts")
	}
	if m[1] != PostType {
		t.Errorf("the post type is %q here and %q in the webapp", PostType, m[1])
	}
}

// Posts.Type is VARCHAR(26). Post.IsValid checks the custom_ prefix and never
// the length, so an over-long type is a database error at save time, which is
// the author being unable to post at all.
func TestPostTypeFitsTheColumn(t *testing.T) {
	if len(PostType) > decorators.PostTypeMaxLen {
		t.Fatalf("PostType %q is %d bytes, over the %d the column holds",
			PostType, len(PostType), decorators.PostTypeMaxLen)
	}
	if decorators.StandalonePostType(&Decorator{}) != PostType {
		t.Fatal("the framework refused this package's own post type")
	}
}

// The shell attribute that tells a standalone page which map surfaces are on.
//
// The last cross-language seam in this package without a pin, and the one whose
// drift is quietest. Both halves are checked because they fail in opposite
// directions:
//
//   - Rename the ATTRIBUTE in Go and `root.dataset.maps` is undefined, which
//     mapsFrom deliberately reads as every surface on. So both standalone pages
//     keep drawing maps on an install that turned them off, which is the
//     opposite of what the admin asked for and the one thing the switch
//     promises.
//   - Rename a TOKEN and that surface reads off, so the page stops drawing a map
//     an admin left on.
//
// Neither is logged anywhere. Nothing else can catch this: the Go tests and the
// TypeScript tests each pin their own copy of these strings, so renaming one
// side together with its own test leaves both suites green.
func TestWebappMapSurfaceAttributeMatches(t *testing.T) {
	source := readPageSource(t, "payload.ts")

	t.Run("attribute", func(t *testing.T) {
		// dataset turns data-maps into .maps, which is the spelling to look for.
		want := strings.TrimPrefix(MapSurfacesAttr, "data-")
		if !regexp.MustCompile(`dataset\.` + want + `\b`).MatchString(source) {
			t.Errorf("the webapp never reads %q (as dataset.%s); an attribute it does not "+
				"read is absent, which it takes as every map surface being on",
				MapSurfacesAttr, want)
		}
	})

	for _, surface := range []string{SurfacePanel, SurfacePage} {
		t.Run(surface, func(t *testing.T) {
			if !strings.Contains(source, `has('`+surface+`')`) {
				t.Errorf("the webapp never matches the %q surface; a token it does not "+
					"recognise reads as that surface being off", surface)
			}
		})
	}
}

// And the separator, since the tokens are joined into one attribute value.
//
// Joining with ", " instead of "," would leave every token after the first
// unmatched, so the page would draw no map with nothing saying why. Neither
// side's own tests would notice: Go asserts the string it just built and the
// webapp asserts the string it wrote itself.
func TestWebappMapSurfaceSeparatorMatches(t *testing.T) {
	source := readPageSource(t, "payload.ts")

	if !strings.Contains(source, `split('`+mapSurfaceSeparator+`')`) {
		t.Errorf("the webapp does not split the surface list on %q", mapSurfaceSeparator)
	}
}

// readPageSource reads a file from the page bundle's source.
//
// A sibling of readWebappSource, which is rooted at the location decorator's own
// directory. The shell's reader lives in the page entry point instead, and
// reaching it through that helper needed a "map/../.." prefix that said nothing
// about where the file is.
func readPageSource(t *testing.T, name string) string {
	t.Helper()

	path := filepath.Join("..", "..", "..", "webapp", "src", "page", name)
	raw, err := os.ReadFile(path) // #nosec G304 -- fixed, repo-relative source path
	if err != nil {
		t.Fatalf("could not read %s: %v", path, err)
	}

	return string(raw)
}

/*
 * The package list reaches a page through the document rather than the API,
 * because a page has no session. Three things have to agree for that to work:
 * the attribute name, the separator, and that the webapp reads it at all.
 *
 * An attribute the webapp never reads is an absent one, and absence here means
 * NONE, so the pages would silently draw the global tier alone on an install
 * that has every area.
 */
func TestWebappPackagesAttributeMatches(t *testing.T) {
	source := readPageSource(t, "payload.ts")

	t.Run("attribute", func(t *testing.T) {
		// dataset turns data-packages into .packages.
		want := strings.TrimPrefix(PackagesAttr, "data-")
		if !regexp.MustCompile(`dataset\.` + want + `\b`).MatchString(source) {
			t.Errorf("the webapp never reads %q (as dataset.%s); an attribute it does not "+
				"read is absent, which it takes as no detail area being installed",
				PackagesAttr, want)
		}
	})

	t.Run("separator", func(t *testing.T) {
		if !strings.Contains(source, `split('`+packageSeparator+`')`) {
			t.Errorf("the webapp does not split on %q, so every name but one is discarded "+
				"by the grammar filter behind it", packageSeparator)
		}
	})
}
