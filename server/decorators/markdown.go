package decorators

import "strings"

var tableCellEscaper = strings.NewReplacer(
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

func TableCell(value string) string {
	return tableCellEscaper.Replace(value)
}

const (
	DetailsRowLabel  = "Details"
	DetailsLinkLabel = "Open details"
)

func TableDetailsRow(href string) string {
	return "| " + DetailsRowLabel + " | [" + DetailsLinkLabel + "](" + href + ") |"
}
