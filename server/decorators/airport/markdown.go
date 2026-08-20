package airport

import (
	"strings"
)

const tableHeaderLabel = "Airfield"

var cellEscaper = strings.NewReplacer(
	`\`, `\\`,
	`|`, `\|`,
	"`", "\\`",
	`*`, `\*`,
	`_`, `\_`,
	`[`, `\[`,
	`]`, `\]`,
	`<`, `\<`,
	`>`, `\>`,
	`~`, `\~`,
	"\r", " ",
	"\n", " ",
)

// mdCell makes a database value safe to put in a table cell.
//
// The values come from a crowd-sourced third party and reach markdown for the
// first time here: every other surface renders them as escaped HTML or as a
// React text node. A pipe would end the cell and shift every value after it
// into the wrong column, and the emphasis, code and link characters would be
// read as formatting.
//
// The shipped file carries 19 names using a backtick as an apostrophe and two
// with brackets, so this is not hypothetical even before the database is
// refreshed.
func mdCell(value string) string {
	return cellEscaper.Replace(value)
}

// airfieldTable is the airfield rendered as a GitHub-flavored markdown table.
//
// The header row is the airfield's name AS THE LINK, so the table is the whole
// message and there is no separate line above it repeating the same
// destination.
//
// href is interpolated without mdCell, and that is safe rather than an
// oversight: it is this plugin's own generated URL, and both url.Values.Encode
// and url.URL.EscapedPath render a pipe as %7C, so it cannot end the cell. A
// paren CAN survive in it, from a SiteURL subpath, which breaks the markdown
// link for such an install exactly as it already breaks every other decorated
// link there.
//
// That relabels the link, which this decorator refused to do for as long as the
// stored message was only a link: the sibling plugin writes "Name (IDENT)" and
// the argument against it was that a name changes between builds and this hook
// rewrites stored text. The table concedes that already, since the name is in
// the message either way.
//
// What is NOT conceded is the author's own token, which is why the Code row
// exists. Without it the string the author typed appears nowhere in the message
// and searching for the code stops finding the post, which is the property this
// whole approach was chosen for.
func airfieldTable(href, trail string, d Details) string {
	name := d.Name
	if name == "" {
		name = d.Ident
	}

	var b strings.Builder
	b.WriteString("| " + tableHeaderLabel + " | [" + mdCell(name) + "](" + href + ") |\n")
	b.WriteString("|:--|:--|\n")

	for _, row := range []struct{ label, value string }{
		{"Code", d.Ident + trail},
		{"Place", d.Place},
		{"Type", d.Type},
		{"Elevation", d.Elevation},
		{"IATA", d.IATA},
	} {
		if row.value == "" {
			continue
		}
		b.WriteString("| " + row.label + " | " + mdCell(row.value) + " |\n")
	}

	return strings.TrimRight(b.String(), "\n")
}
