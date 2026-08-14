package dtg

import (
	"strings"
	"testing"
	"time"
)

// A zone that will not load must still produce a row. Dropping it would leave
// the reader with a table that silently lost a location they chose, which reads
// as "no difference to show" rather than "this server cannot tell you".
//
// Only reachable when the embedded tzdata is missing, which the blank import in
// server/main.go exists to prevent, so nothing else exercises this.
func TestRenderRowDegradesWhenTheZoneWillNotLoad(t *testing.T) {
	instant := time.Date(2026, time.August, 9, 16, 30, 0, 0, time.UTC)

	row := renderRow(instant, DisplayZone{Name: "Nowhere", IANA: "Mars/Olympus_Mons", Abbr: "MOM"}, instant)

	if !strings.Contains(row, ">Nowhere<") {
		t.Fatalf("renderRow() = %q, want the name still rendered", row)
	}
	if !strings.Contains(row, ">MOM<") {
		t.Fatalf("renderRow() = %q, want the abbreviation still rendered", row)
	}
	if strings.Count(row, "n/a") != 2 {
		t.Fatalf("renderRow() = %q, want both the time and the date cells to read n/a", row)
	}
}

// The name and abbreviation are escaped on the degraded path too. They are
// plugin data today, but the escaping is what the reader's own labels will rely
// on if this table ever renders them.
func TestRenderRowEscapesOnBothPaths(t *testing.T) {
	instant := time.Date(2026, time.August, 9, 16, 30, 0, 0, time.UTC)

	cases := []struct {
		name string
		iana string
	}{
		{"resolvable zone", "UTC"},
		{"unresolvable zone", "Mars/Olympus_Mons"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			row := renderRow(instant, DisplayZone{Name: `<script>`, IANA: tc.iana, Abbr: `"&"`}, instant)

			if strings.Contains(row, "<script>") {
				t.Fatalf("renderRow() = %q, want the name escaped", row)
			}
			if !strings.Contains(row, "&lt;script&gt;") {
				t.Fatalf("renderRow() = %q, want the escaped name present", row)
			}
			if !strings.Contains(row, "&#34;&amp;&#34;") {
				t.Fatalf("renderRow() = %q, want the abbreviation escaped", row)
			}
		})
	}
}

// A short-form token borrows what it does not say from the post date, and the
// page has to admit which parts those were. "091630Z" borrows both; "091630ZAUG"
// borrows only the year.
func TestAssumedNote(t *testing.T) {
	cases := []struct {
		name    string
		assumed string
		want    string
	}{
		{
			name:    "month and year inferred",
			assumed: "my",
			want:    "Month and year were not in the original text; both were taken from the date the message was posted.",
		},
		{
			name:    "year inferred",
			assumed: "y",
			want:    "The year was not in the original text; it was taken from the date the message was posted.",
		},
		{"nothing inferred", "", ""},
		{"unrecognized code says nothing", "d", ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := assumedNote(tc.assumed); got != tc.want {
				t.Fatalf("assumedNote(%q) = %q, want %q", tc.assumed, got, tc.want)
			}
		})
	}
}

// The note has to survive into the page, or the reader never learns the month
// and year were inferred rather than written.
func TestPageCarriesTheAssumedNote(t *testing.T) {
	instant := time.Date(2026, time.August, 9, 16, 30, 0, 0, time.UTC)

	withNote := renderBody(pageData{
		instant:   instant,
		canonical: "091630ZAUG26",
		zoneLabel: "Z",
		assumed:   "y",
	})
	if !strings.Contains(withNote, "The year was not in the original text") {
		t.Fatal("renderBody() dropped the assumed-year note")
	}

	without := renderBody(pageData{
		instant:   instant,
		canonical: "091630ZAUG26",
		zoneLabel: "Z",
	})
	if strings.Contains(without, `class="note"`) {
		t.Fatal("renderBody() emitted a note paragraph for a fully written token")
	}
}
