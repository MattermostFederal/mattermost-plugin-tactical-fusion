package dtg

import (
	"sort"
	"time"
)

// Military time zone letters map to whole-hour offsets from UTC.
//
// Z is UTC. A-H are UTC+1..+8, K-M are UTC+10..+12, N-Y are UTC-1..-12.
// There is deliberately no I: the letter is skipped in the military alphabet
// because it reads as a 1.
//
// J (Juliet, the observer's own local time) is absent on purpose. It would make
// the resolved instant reader-dependent, and this design bakes a single instant
// into the link, so a J DTG is rejected rather than guessed at.
var zoneOffsets = map[byte]int{
	'Y': -12, 'X': -11, 'W': -10, 'V': -9, 'U': -8, 'T': -7,
	'S': -6, 'R': -5, 'Q': -4, 'P': -3, 'O': -2, 'N': -1,
	'Z': 0,
	'A': 1, 'B': 2, 'C': 3, 'D': 4, 'E': 5, 'F': 6,
	'G': 7, 'H': 8, 'K': 10, 'L': 11, 'M': 12,
}

// zoneOffsetHours returns the UTC offset for a military zone letter.
func zoneOffsetHours(letter byte) (int, bool) {
	offset, ok := zoneOffsets[letter]
	return offset, ok
}

// DisplayZone is one row of the timezone table.
type DisplayZone struct {
	// Name is the human label.
	Name string

	// IANA is the tz database identifier. IANA names rather than fixed offsets,
	// so the standard library and Intl both handle DST.
	IANA string

	// Abbr is a short hint shown next to the name.
	//
	// Season-neutral for any zone that observes daylight saving, because this
	// label is hand-written and the row beside it is not: the time is measured
	// at the DTG's instant, so a standard-time-only abbreviation would say
	// "PST" next to a clock reading PDT for eight months of the year. American
	// English has neutral forms (ET, MT, PT) and European usage does not, which
	// is why Ramstein names both halves rather than inventing a "CE" nobody
	// writes. Do not "tidy" that into a single token.
	Abbr string
}

// DisplayZones is the table rendered by both the server page and the RHS panel.
//
// Keep this list in sync with webapp/src/decorators/dtg/zones.ts. zones_test.go
// reads that file and fails if the IANA identifiers diverge.
var DisplayZones = []DisplayZone{
	{Name: "Zulu (UTC)", IANA: "UTC", Abbr: "Z"},
	{Name: "Washington, DC", IANA: "America/New_York", Abbr: "ET"},
	{Name: "Colorado Springs", IANA: "America/Denver", Abbr: "MT"},
	{Name: "San Diego", IANA: "America/Los_Angeles", Abbr: "PT"},
	{Name: "Honolulu", IANA: "Pacific/Honolulu", Abbr: "HST"},
	{Name: "Ramstein", IANA: "Europe/Berlin", Abbr: "CET/CEST"},
	{Name: "Al Udeid", IANA: "Asia/Qatar", Abbr: "AST"},
	{Name: "Yokota", IANA: "Asia/Tokyo", Abbr: "JST"},
	{Name: "Andersen, Guam", IANA: "Pacific/Guam", Abbr: "ChST"},
}

// OrderedZones returns zones ordered west to east, the way a world clock reads.
//
// Measured at the instant rather than from a table of offsets, because half
// these zones observe daylight saving and would be an hour out for part of the
// year. Must agree with orderedZones in webapp/src/decorators/dtg/zones.ts,
// down to the tiebreak, or the sidebar and this page would list the same zones
// in two different orders.
//
// A zone that will not resolve sorts last rather than as UTC. The row still
// renders, as "n/a", which is what makes a missing timezone database visible
// instead of silently plausible.
func OrderedZones(zones []DisplayZone, instant time.Time) []DisplayZone {
	type positioned struct {
		zone    DisplayZone
		offset  int
		unknown bool
	}

	positions := make([]positioned, 0, len(zones))
	for _, zone := range zones {
		loc, err := time.LoadLocation(zone.IANA)
		if err != nil {
			positions = append(positions, positioned{zone: zone, unknown: true})
			continue
		}

		_, offset := instant.In(loc).Zone()
		positions = append(positions, positioned{zone: zone, offset: offset})
	}

	sort.SliceStable(positions, func(i, j int) bool {
		a, b := positions[i], positions[j]
		if a.unknown != b.unknown {
			return b.unknown
		}
		if a.offset != b.offset {
			return a.offset < b.offset
		}
		if a.zone.Name != b.zone.Name {
			return a.zone.Name < b.zone.Name
		}
		return a.zone.IANA < b.zone.IANA
	})

	ordered := make([]DisplayZone, 0, len(positions))
	for _, position := range positions {
		ordered = append(ordered, position.zone)
	}

	return ordered
}
