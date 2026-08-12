package location

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
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
		`mgrs: new RegExp\(` + "`" + `\^\(\\\\d\{1,2\}\)\(\$\{BAND\}\)\(\[([^\]]+)\]\)\(\[([^\]]+)\]\)`,
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

// And both grid patterns must be built FROM the shared class rather than
// spelling a band class of their own, or widening BAND would fix nothing.
func TestWebappGridPatternsUseTheSharedBandClass(t *testing.T) {
	source := readWebappSource(t, "format.ts")

	for _, name := range []string{"mgrs", "utm"} {
		if !regexp.MustCompile(name + `: new RegExp\(` + "`" + `\^\(\\\\d\{1,2\}\)\(\$\{BAND\}\)`).MatchString(source) {
			t.Errorf("the webapp's %s pattern does not use ${BAND}", name)
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
