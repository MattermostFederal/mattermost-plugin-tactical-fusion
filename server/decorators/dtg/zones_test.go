package dtg

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"
)

func TestZoneOffsets(t *testing.T) {
	cases := []struct {
		letter byte
		want   int
	}{
		{'Z', 0},
		{'A', 1},
		{'B', 2},
		{'H', 8},
		{'K', 10},
		{'M', 12},
		{'N', -1},
		{'R', -5},
		{'Y', -12},
	}

	for _, tc := range cases {
		got, ok := zoneOffsetHours(tc.letter)
		if !ok {
			t.Fatalf("zoneOffsetHours(%c) reported unknown, want %+d", tc.letter, tc.want)
		}
		if got != tc.want {
			t.Fatalf("zoneOffsetHours(%c) = %+d, want %+d", tc.letter, got, tc.want)
		}
	}
}

// I is skipped in the military alphabet and J is the observer's own local time,
// which cannot be resolved to a single instant for every reader.
func TestZoneLettersIAndJAreUnknown(t *testing.T) {
	for _, letter := range []byte{'I', 'J'} {
		if _, ok := zoneOffsetHours(letter); ok {
			t.Fatalf("zoneOffsetHours(%c) reported known, want unknown", letter)
		}
	}
}

func TestZoneLetterCoverage(t *testing.T) {
	// 26 letters minus I and J.
	if len(zoneOffsets) != 24 {
		t.Fatalf("zoneOffsets has %d entries, want 24", len(zoneOffsets))
	}
}

// The abbreviations are fixed labels, but the times come from the IANA zone, so
// a daylight-saving zone has to shift with the season.
func TestPacificRowFollowsDaylightSaving(t *testing.T) {
	loc, err := time.LoadLocation("America/Los_Angeles")
	if err != nil {
		t.Fatalf("time.LoadLocation failed: %v", err)
	}

	summer := time.Date(2026, time.August, 9, 16, 30, 0, 0, time.UTC).In(loc)
	winter := time.Date(2026, time.January, 9, 16, 30, 0, 0, time.UTC).In(loc)

	if got := summer.Format("15:04"); got != "09:30" {
		t.Fatalf("summer = %s, want 09:30", got)
	}
	if got := winter.Format("15:04"); got != "08:30" {
		t.Fatalf("winter = %s, want 08:30", got)
	}
}

// Proves the embedded tzdata in server/main.go is doing its job. Without it,
// UTC keeps resolving while every other zone fails, which is exactly the kind
// of breakage a casual smoke test misses.
func TestDisplayZonesResolve(t *testing.T) {
	sawNonUTC := false

	for _, zone := range DisplayZones {
		loc, err := time.LoadLocation(zone.IANA)
		if err != nil {
			t.Fatalf("time.LoadLocation(%q) failed: %v (is _ \"time/tzdata\" still imported in server/main.go?)", zone.IANA, err)
		}
		if zone.IANA != "UTC" {
			sawNonUTC = true

			// A zone that resolved but has no real offset would mean the
			// database is present but empty.
			reference := time.Date(2026, time.August, 9, 12, 0, 0, 0, time.UTC)
			if _, offset := reference.In(loc).Zone(); offset == 0 && zone.IANA != "Atlantic/Reykjavik" {
				t.Fatalf("zone %q resolved with a zero offset, which suggests an empty timezone database", zone.IANA)
			}
		}
	}

	if !sawNonUTC {
		t.Fatal("DisplayZones contains only UTC, so it cannot prove tzdata is present")
	}
}

// The order is the one asserted for the sidebar in
// webapp/src/decorators/dtg/zones.spec.ts. The two must agree, or the same DTG
// lists its zones two different ways depending on where you open it.
func TestOrderedZonesRunWestToEast(t *testing.T) {
	summer := time.Date(2026, time.August, 9, 16, 30, 0, 0, time.UTC)

	want := []string{
		"Honolulu",
		"San Diego",
		"Colorado Springs",
		"Washington, DC",
		"Zulu (UTC)",
		"Ramstein",
		"Al Udeid",
		"Yokota",
		"Andersen, Guam",
	}

	got := make([]string, 0, len(want))
	for _, zone := range OrderedZones(DisplayZones, summer) {
		got = append(got, zone.Name)
	}

	if len(got) != len(want) {
		t.Fatalf("OrderedZones returned %d zones, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("position %d = %q, want %q (full order: %v)", i, got[i], want[i], got)
		}
	}
}

// The offsets are measured at the instant, not looked up, so the order has to
// move with daylight saving rather than freeze at whatever August looked like.
//
// The zones in DisplayZones all shift together, so their order never changes
// and cannot show this. Halifax and Santiago can: they are on opposite
// hemispheres, so they swap outright rather than tying. A pair that merely tied
// would be testing the name tiebreak instead of the measurement. The same pair
// is asserted in webapp/src/decorators/dtg/zones.spec.ts.
func TestOrderedZonesFollowTheSeason(t *testing.T) {
	summer := time.Date(2026, time.August, 9, 16, 30, 0, 0, time.UTC)
	winter := time.Date(2026, time.January, 9, 16, 30, 0, 0, time.UTC)

	zones := []DisplayZone{
		{Name: "Halifax", IANA: "America/Halifax", Abbr: "AT"},
		{Name: "Santiago", IANA: "America/Santiago", Abbr: "CLT"},
	}

	inSummer := OrderedZones(zones, summer)
	if inSummer[0].Name != "Santiago" || inSummer[1].Name != "Halifax" {
		t.Fatalf("summer order = %s, %s; want Santiago, Halifax", inSummer[0].Name, inSummer[1].Name)
	}

	inWinter := OrderedZones(zones, winter)
	if inWinter[0].Name != "Halifax" || inWinter[1].Name != "Santiago" {
		t.Fatalf("winter order = %s, %s; want Halifax, Santiago", inWinter[0].Name, inWinter[1].Name)
	}
}

// Several bases share a zone, so ties are routine rather than exotic: Ramstein
// and Stuttgart keep the same clock and a reader may well have picked both. The
// order they land in has to be decided by something, and it is the name.
//
// Must agree with orderedZones in webapp/src/decorators/dtg/zones.ts, which is
// the whole reason the tiebreak is spelled out rather than left to the sort.
func TestOrderedZonesBreakOffsetTiesOnName(t *testing.T) {
	instant := time.Date(2026, time.August, 9, 16, 30, 0, 0, time.UTC)

	// Listed out of order so a pass cannot come from the input order surviving.
	zones := []DisplayZone{
		{Name: "Stuttgart", IANA: "Europe/Berlin", Abbr: "CET"},
		{Name: "Ramstein", IANA: "Europe/Berlin", Abbr: "CET"},
		{Name: "Aviano", IANA: "Europe/Rome", Abbr: "CET"},
	}

	ordered := OrderedZones(zones, instant)

	want := []string{"Aviano", "Ramstein", "Stuttgart"}
	for i, name := range want {
		if ordered[i].Name != name {
			t.Fatalf("position %d = %q, want %q", i, ordered[i].Name, name)
		}
	}
}

// Identity is the (IANA, Name) pair, so two entries can share a name and still
// be distinct rows. The identifier is the last tiebreak, and without it the
// order of such a pair would depend on the input order.
func TestOrderedZonesBreakNameTiesOnIdentifier(t *testing.T) {
	instant := time.Date(2026, time.August, 9, 16, 30, 0, 0, time.UTC)

	zones := []DisplayZone{
		{Name: "Central Europe", IANA: "Europe/Rome", Abbr: "CET"},
		{Name: "Central Europe", IANA: "Europe/Berlin", Abbr: "CET"},
	}

	ordered := OrderedZones(zones, instant)

	if ordered[0].IANA != "Europe/Berlin" || ordered[1].IANA != "Europe/Rome" {
		t.Fatalf("order = %s, %s; want Europe/Berlin, Europe/Rome", ordered[0].IANA, ordered[1].IANA)
	}
}

// Treating an unknown offset as zero would file it under UTC, which is a claim
// rather than an admission.
func TestOrderedZonesPutUnresolvableZonesLast(t *testing.T) {
	instant := time.Date(2026, time.August, 9, 16, 30, 0, 0, time.UTC)

	zones := []DisplayZone{
		{Name: "Nowhere", IANA: "Mars/Olympus_Mons", Abbr: ""},
		{Name: "Yokota", IANA: "Asia/Tokyo", Abbr: "JST"},
		{Name: "Zulu (UTC)", IANA: "UTC", Abbr: "Z"},
	}

	ordered := OrderedZones(zones, instant)

	want := []string{"Zulu (UTC)", "Yokota", "Nowhere"}
	for i, name := range want {
		if ordered[i].Name != name {
			t.Fatalf("position %d = %q, want %q", i, ordered[i].Name, name)
		}
	}
}

// The page renders whatever OrderedZones returns, so this is what a reader
// actually sees.
func TestPageRendersZonesInOrder(t *testing.T) {
	instant := time.Date(2026, time.August, 9, 16, 30, 0, 0, time.UTC)
	body := renderBody(pageData{
		instant:   instant,
		canonical: "091630ZAUG26",
		zoneLabel: "Z",
	})

	previous := -1
	for _, zone := range OrderedZones(DisplayZones, instant) {
		at := strings.Index(body, zone.Name)
		if at < 0 {
			t.Fatalf("the page is missing %q", zone.Name)
		}
		if at <= previous {
			t.Fatalf("%q appears out of order in the page", zone.Name)
		}
		previous = at
	}
}

// The two sides render the same table, so a divergence would mean the RHS panel
// and the standalone page disagree about the same DTG.
func TestDisplayZonesMatchTheWebappList(t *testing.T) {
	path := filepath.Join("..", "..", "..", "webapp", "src", "decorators", "dtg", "zones.ts")

	source, err := os.ReadFile(path) // #nosec G304 -- fixed, repo-relative test fixture path
	if err != nil {
		t.Fatalf("could not read %s: %v", path, err)
	}

	// Scoped to the DISPLAY_ZONES literal. That file also lists the military
	// bases the picker offers, which are a webapp-only convenience with no Go
	// counterpart, and matching those too would compare this table against a
	// list it was never meant to equal.
	blockRe := regexp.MustCompile(`(?s)export const DISPLAY_ZONES: DisplayZone\[\] = \[(.*?)\];`)
	block := blockRe.FindStringSubmatch(string(source))
	if block == nil {
		t.Fatalf("could not find the DISPLAY_ZONES table in %s; has it been renamed?", path)
	}

	ianaRe := regexp.MustCompile(`iana:\s*'([^']+)'`)
	matches := ianaRe.FindAllStringSubmatch(block[1], -1)

	got := make([]string, 0, len(matches))
	for _, m := range matches {
		got = append(got, m[1])
	}

	if len(got) != len(DisplayZones) {
		t.Fatalf("webapp has %d zones, Go has %d; the two tables must match", len(got), len(DisplayZones))
	}

	for i, zone := range DisplayZones {
		if got[i] != zone.IANA {
			t.Fatalf("zone %d: webapp has %q, Go has %q", i, got[i], zone.IANA)
		}
	}
}
