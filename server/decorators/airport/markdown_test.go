package airport

import (
	"strings"
	"testing"
)

const HREF = "/plugins/tf/decorate/airport?v=KIND"

// The escaping itself, on the function that does it. Kept separate from the
// table because the table legitimately writes unescaped pipes as cell
// separators and unescaped brackets as its one link, so asking whether the
// rendered table contains a raw "[" answers the wrong question.
func TestMdCellEscapesWhatMarkdownWouldRead(t *testing.T) {
	for _, raw := range []string{"|", "`", "*", "_", "[", "]", "<", ">", "~", `\`} {
		// The WHOLE output, not a substring: an escaper that handled only the
		// first occurrence would pass a Contains check.
		in := raw + "a" + raw
		want := `\` + raw + "a" + `\` + raw
		if got := mdCell(in); got != want {
			t.Errorf("mdCell(%q) = %q, want %q", in, got, want)
		}
	}

	// A newline would end the row and turn the rest of the value into a
	// paragraph after the table. The build filter already refuses one, so this
	// is the second layer rather than the first.
	// Both line endings, and the carriage return matters more than it looks:
	// the build filter refuses one in a DATABASE value, but the Code row also
	// carries the author's own trailing text, where mdCell is the only layer.
	for in, want := range map[string]string{
		"a\nb":   "a b",
		"a\rb":   "a b",
		"a\r\nb": "a  b",
	} {
		if got := mdCell(in); got != want {
			t.Errorf("mdCell(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestAirfieldTableEscapesWhatTheDatabaseCarries(t *testing.T) {
	table := airfieldTable(HREF, "", Details{
		Ident:     "KIND",
		Name:      "Sant`anna | Heliport",
		Place:     "a [place](x), _somewhere_",
		Type:      "*Large* Airport",
		Elevation: "1 ft",
		IATA:      "I~D",
	})

	// The body rows carry no markdown of their own, so anything unescaped in
	// them came from the database.
	for line := range strings.SplitSeq(table, "\n") {
		if strings.HasPrefix(line, "| Airfield |") || line == "|:--|:--|" {
			continue
		}
		for _, ch := range []byte{'`', '*', '_', '[', ']', '~'} {
			if hasUnescaped(line, ch) {
				t.Errorf("unescaped %q reached %q", string(ch), line)
			}
		}
	}

	// The name is escaped inside the link label, where an unescaped bracket
	// would end the label early and break the link.
	if !strings.Contains(table, "[Sant\\`anna \\| Heliport]("+HREF+")") {
		t.Errorf("the name was not escaped inside the link label:\n%s", table)
	}

	// And the row shape holds, which is what a cell that ate its own pipe
	// would actually violate.
	for line := range strings.SplitSeq(table, "\n") {
		if got := unescapedPipes(line); got != 3 {
			t.Errorf("row %q has %d unescaped pipes, want 3", line, got)
		}
	}
}

// hasUnescaped reports whether a character appears where markdown would read it
// as syntax rather than as text.
func hasUnescaped(s string, ch byte) bool {
	for i := range len(s) {
		if s[i] != ch {
			continue
		}
		if i > 0 && s[i-1] == '\\' {
			continue
		}
		return true
	}
	return false
}

// unescapedPipes counts the pipes a markdown parser would treat as cell
// separators, which is every one not preceded by a backslash.
func unescapedPipes(line string) int {
	count := 0
	for i, r := range line {
		if r != '|' {
			continue
		}
		if i > 0 && line[i-1] == '\\' {
			continue
		}
		count++
	}
	return count
}

func TestAirfieldTableOmitsWhatTheDatabaseDoesNotState(t *testing.T) {
	table := airfieldTable(HREF, "", Details{Ident: "YARD", Name: "Argadargada Airport", Place: "NT, AU", Type: "Small Airport"})

	for _, absent := range []string{"Elevation", "IATA"} {
		if strings.Contains(table, absent) {
			t.Errorf("%s was rendered for a field that states none:\n%s", absent, table)
		}
	}
	if !strings.Contains(table, "| Place | NT, AU |") {
		t.Errorf("the place is missing:\n%s", table)
	}
}

// Empty means the database states none, which is a different thing from sea
// level. Nothing here may test elevation for truthiness.
func TestAirfieldTableShowsSeaLevel(t *testing.T) {
	table := airfieldTable(HREF, "", Details{Ident: "KIND", Name: "x", Place: "y", Elevation: "0 ft"})

	if !strings.Contains(table, "| Elevation | 0 ft |") {
		t.Errorf("sea level was dropped as though it were absent:\n%s", table)
	}
}

// The link is the airfield's NAME, and the author's own token survives as the
// Code row.
//
// Both halves matter. Relabelling the link is what this decorator refused while
// the stored message was only a link, and the table concedes it because the name
// is in the message either way. The Code row is what is not conceded: without
// it the string the author typed appears nowhere, and searching for the code
// stops finding the post.
func TestAirfieldTableLinksTheNameAndKeepsTheCode(t *testing.T) {
	table := airfieldTable(HREF, "", Details{Ident: "KIND", Name: "Indianapolis International Airport", Place: "y"})

	if !strings.Contains(table, "| Airfield | [Indianapolis International Airport]("+HREF+") |") {
		t.Errorf("the name is not the link:\n%s", table)
	}
	if !strings.Contains(table, "| Code | KIND |") {
		t.Errorf("the author's own token is missing:\n%s", table)
	}
	if strings.Count(table, HREF) != 1 {
		t.Errorf("the destination appears %d times, want once:\n%s", strings.Count(table, HREF), table)
	}
}

// A USMTF set line ends "//". The author wrote it, so it travels with the token
// it belonged to rather than being dropped.
func TestAirfieldTableKeepsTheSetTerminatorWithTheCode(t *testing.T) {
	table := airfieldTable(HREF, "//", Details{Ident: "KIND", Name: "x", Place: "y"})

	if !strings.Contains(table, "| Code | KIND// |") {
		t.Errorf("the set terminator was lost:\n%s", table)
	}
}

// An unnamed airfield has nothing to label the link with but its own code.
func TestAirfieldTableLinksTheIdentWhenThereIsNoName(t *testing.T) {
	table := airfieldTable(HREF, "", Details{Ident: "KIND", Place: "y"})

	if !strings.Contains(table, "| Airfield | [KIND]("+HREF+") |") {
		t.Errorf("an unnamed airfield lost its link:\n%s", table)
	}
}

// Every row in the shipped database, because one malformed table is one
// permanently mangled message.
func TestEveryAirfieldTableIsWellFormed(t *testing.T) {
	for ident := range airfields {
		details, ok := Describe(ident)
		if !ok {
			t.Fatalf("Describe(%q) refused a shipped ident", ident)
		}

		table := airfieldTable(HREF, "", details)
		lines := strings.Split(table, "\n")
		if len(lines) < 3 {
			t.Fatalf("%s rendered %d lines, want a header, a rule and at least one row:\n%s",
				ident, len(lines), table)
		}
		if lines[1] != "|:--|:--|" {
			t.Fatalf("%s has no delimiter row: %q", ident, lines[1])
		}

		for _, line := range lines {
			if got := unescapedPipes(line); got != 3 {
				t.Fatalf("%s row %q has %d unescaped pipes, want 3", ident, line, got)
			}
		}
	}
}

// The corpus test above is a SHAPE check, not escaping coverage, and this is
// what makes that honest: the shipped file carries none of the characters
// mdCell escapes, so reducing mdCell to the identity function leaves the corpus
// green. Assert the invariant that silence depends on, so a refreshed database
// carrying a pipe fails here rather than rendering a broken table.
func TestTheShippedDatabaseCarriesNothingTheTableCannotEscape(t *testing.T) {
	for ident := range airfields {
		d, ok := DescribeFields(ident)
		if !ok {
			t.Fatalf("DescribeFields(%q) refused a shipped ident", ident)
		}

		for _, field := range []string{d.Name, d.Place, d.Type, d.Elevation, d.IATA} {
			if strings.ContainsAny(field, "|\r\n\\") {
				t.Errorf("%s carries a character the table cannot survive: %q", ident, field)
			}
		}
	}
}

// The real hostile rows, so a refreshed database that changes the escaper is
// caught against values that exist rather than only against invented ones.
func TestTheShippedHostileRowsStillRender(t *testing.T) {
	for ident, want := range map[string]string{
		"LIMN": `Cameri Air Base \[MIL\]`,
		"SDCP": "Hotel Sant\\`anna Heliport",
	} {
		d, ok := DescribeFields(ident)
		if !ok {
			t.Fatalf("%s is no longer in the database; pick another hostile row rather than deleting this", ident)
		}

		if got := mdCell(d.Name); got != want {
			t.Errorf("mdCell(%q) = %q, want %q", d.Name, got, want)
		}
	}
}
