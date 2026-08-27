package cot

import (
	_ "embed"
	"encoding/csv"
	"slices"
	"strings"
)

type affiliation struct {
	id    string
	label string
}

// The label is the adjectival form, because it is read in front of what
// follows it: "Friendly Ground Combat Unit". The id is not a label and must not
// follow it, since the card keys its affiliation color on that string.
var affiliations = map[byte]affiliation{
	'p': {"pending", "Pending"},
	'u': {"unknown", "Unknown"},
	'a': {"assumed-friend", "Assumed Friendly"},
	'f': {"friend", "Friendly"},
	'n': {"neutral", "Neutral"},
	's': {"suspect", "Suspect"},
	'h': {"hostile", "Hostile"},
	'j': {"joker", "Joker"},
	'k': {"faker", "Faker"},
	'o': {"none", "Unaffiliated"},
	'x': {"other", "Other"},
}

//go:embed data/types.csv
var atomTypesCSV string

// atomPaths is what an atom's code path below the affiliation means, keyed by
// the whole path rather than by each letter.
//
// Keyed by path because the letters do not compose into English on their own:
// "G-U-C" is Ground, Unit, Combat, and the catalog's name for it is a finished
// phrase. The longest matching path wins, so an unrecognized tail costs the
// letters after it and never the ones before.
//
// See server/cot/data/README.md for what the table holds and how to extend it.
var atomPaths = loadAtomPaths()

func loadAtomPaths() map[string]string {
	reader := csv.NewReader(strings.NewReader(atomTypesCSV))

	rows, err := reader.ReadAll()
	if err != nil || len(rows) < 2 {
		// Embedded at build time from a file this repository generates and
		// commits, so this is a broken build rather than anything a message can
		// cause. An empty table decodes every atom to its affiliation alone,
		// which is the same answer an unrecognized path already gets.
		return map[string]string{}
	}

	paths := make(map[string]string, len(rows)-1)
	for _, row := range rows[1:] {
		if len(row) == 2 && row[0] != "" && row[1] != "" {
			paths[row[0]] = row[1]
		}
	}

	return paths
}

// maxAtomCodes is the deepest path the table holds, read from the table so it
// cannot disagree with it. A type carries author text of any length, and
// walking past this can match nothing.
var maxAtomCodes = deepestAtomPath()

func deepestAtomPath() int {
	deepest := 0
	for path := range atomPaths {
		if codes := strings.Count(path, "-") + 1; codes > deepest {
			deepest = codes
		}
	}

	return deepest
}

// Title Case, matching the atom table, because both reach the same row on the
// card and a reader meets them side by side.
var wholeTypes = map[string]string{
	"b-t-f":          "Chat Message",
	"b-m-p-s-p-i":    "Designated Point",
	"b-m-p-s-m":      "Survey Point",
	"b-m-p-w":        "Waypoint",
	"b-m-r":          "Route",
	"b-a-o-tbl":      "Emergency Alert",
	"b-a-o-can":      "Emergency Canceled",
	"b-a-g":          "Geofence Breach",
	"b-r-f-h-c":      "Casualty Evacuation Request",
	"b-l-p-c":        "Sensor Point of Interest",
	"u-d-f":          "Drawn Shape",
	"u-d-c-c":        "Drawn Circle",
	"u-r-b-bullseye": "Bullseye",
	"t-x-takp-v":     "TAK Client Version",
	"t-x-c-t":        "Ping",
	"t-x-c-t-r":      "Ping Response",
}

type decodedType struct {
	Label       string
	Affiliation string
}

func decodeType(raw string) decodedType {
	if label, ok := wholeTypes[raw]; ok {
		return decodedType{Label: label}
	}

	if strings.HasPrefix(raw, atomPrefix) {
		return decodeAtom(raw)
	}

	// Longest prefix wins, explicitly. Ranging a map picks an arbitrary one of
	// several matching prefixes, and "t-x-c-t" and "t-x-c-t-r" both match
	// "t-x-c-t-r-1", so the label would have been decided by map iteration
	// order and then written permanently into the post.
	best, bestLen := "", 0
	for prefix, label := range wholeTypes {
		if len(prefix) > bestLen && strings.HasPrefix(raw, prefix+"-") {
			best, bestLen = label, len(prefix)
		}
	}

	if best != "" {
		return decodedType{Label: best}
	}

	return decodedType{}
}

func decodeAtom(raw string) decodedType {
	parts := strings.Split(raw, "-")
	if len(parts) < 2 || len(parts[1]) != 1 {
		return decodedType{}
	}

	aff, ok := affiliations[parts[1][0]]
	if !ok {
		return decodedType{}
	}

	decoded := decodedType{Affiliation: aff.id, Label: aff.label}

	if path := longestAtomPath(parts[2:]); path != "" {
		decoded.Label = aff.label + " " + path
	}

	return decoded
}

// longestAtomPath walks the code letters and keeps the deepest path that is
// actually known, so an unrecognized tail costs the letters after it and not
// the ones before.
func longestAtomPath(codes []string) string {
	if len(codes) > maxAtomCodes {
		codes = codes[:maxAtomCodes]
	}

	best := ""
	var path strings.Builder

	for i, code := range codes {
		if i > 0 {
			path.WriteByte('-')
		}
		path.WriteString(code)

		if label, ok := atomPaths[path.String()]; ok {
			best = label
		}
	}

	return best
}

var howMethods = map[byte]string{
	'm': "Machine",
	'h': "Human",
}

var howSources = map[string]string{
	"m-g": "GPS",
	"m-p": "Measured",
	"m-i": "Inertial",
	"m-f": "Fused",
	"m-c": "Configured",
	"m-r": "Relayed",
	"m-a": "Derived",
	"m-s": "Simulated",
	"m-n": "Not Stated",
	"h-e": "Estimated",
	"h-c": "Calculated",
	"h-t": "Transcribed",
	"h-g": "Map Plotted",
	"h-p": "Precision Plotted",
	"h-i": "Mensurated",
}

func decodeHow(raw string) string {
	if raw == "" {
		return ""
	}

	if source, ok := howSources[raw]; ok {
		method := howMethods[raw[0]]
		if method == "" {
			return source
		}
		return method + ", " + source
	}

	if method, ok := howMethods[raw[0]]; ok && len(raw) > 1 && raw[1] == '-' {
		return method
	}

	return ""
}

// AffiliationIDs is every affiliation this package can decode, sorted.
//
// Exported for the sync test that holds the webapp's word table to it. A word
// is the only channel a map label has, so an affiliation named here and absent
// there is a marker a screen reader cannot tell from any other.
func AffiliationIDs() []string {
	ids := make([]string, 0, len(affiliations))
	for _, aff := range affiliations {
		ids = append(ids, aff.id)
	}
	slices.Sort(ids)
	return ids
}
