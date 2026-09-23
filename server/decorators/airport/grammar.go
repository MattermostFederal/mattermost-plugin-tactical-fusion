package airport

import (
	"regexp"
	"strings"
)

var monikerPrefixes = []string{"DEPLOC", "ARRLOC", "ICAO", "LOC"}

const iataPrefix = "IATA"

const (
	identBody = `[A-Z]{4}`

	iataBody = `[A-Z]{3}`

	setTerminator = `(?://)?`

	separator = `[ \t]*:`
)

var identShape = regexp.MustCompile(`^` + identBody + `$`)

var iataShape = regexp.MustCompile(`^` + iataBody + `$`)

var monikerExpr = `(?:` + strings.Join(byLengthDescending(monikerPrefixes), `|`) + `)`

func byLengthDescending(prefixes []string) []string {
	sorted := append([]string(nil), prefixes...)
	for i := range sorted {
		for j := i + 1; j < len(sorted); j++ {
			if len(sorted[j]) > len(sorted[i]) {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}
	return sorted
}

var scanExpr = `(` + monikerExpr + separator + `(` + identBody + `))` + setTerminator

var iataScanExpr = `(` + iataPrefix + separator + `(` + iataBody + `))` + setTerminator

func MatchesIdentShape(v string) bool { return identShape.MatchString(v) }

func MatchesIATAShape(v string) bool { return iataShape.MatchString(v) }

func IdentBodyExpr() string { return identBody }

func IATABodyExpr() string { return iataBody }
