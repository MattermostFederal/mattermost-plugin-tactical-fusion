package dtg

import (
	"testing"
	"time"
)

// Every test pins the reference time, so nothing is flaky at a month boundary
// or on 29 February.
var ref = time.Date(2026, time.August, 9, 12, 0, 0, 0, time.UTC)

func TestParseAcceptedForms(t *testing.T) {
	cases := []struct {
		name    string
		value   string
		want    DTG
		instant time.Time
	}{
		{
			name:    "two digit year",
			value:   "091630ZAUG26",
			want:    DTG{Day: 9, Hour: 16, Minute: 30, Zone: 'Z', Month: time.August, Year: 2026},
			instant: time.Date(2026, time.August, 9, 16, 30, 0, 0, time.UTC),
		},
		{
			name:    "four digit year",
			value:   "091630ZAUG2026",
			want:    DTG{Day: 9, Hour: 16, Minute: 30, Zone: 'Z', Month: time.August, Year: 2026},
			instant: time.Date(2026, time.August, 9, 16, 30, 0, 0, time.UTC),
		},
		{
			name:  "short form infers month and year from the reference",
			value: "091630Z",
			want: DTG{
				Day: 9, Hour: 16, Minute: 30, Zone: 'Z',
				Month: time.August, Year: 2026,
				AssumedMonth: true, AssumedYear: true,
			},
			instant: time.Date(2026, time.August, 9, 16, 30, 0, 0, time.UTC),
		},
		{
			name:  "positive zone letter shifts back to UTC",
			value: "091630BAUG26",
			want:  DTG{Day: 9, Hour: 16, Minute: 30, Zone: 'B', Month: time.August, Year: 2026},
			// B is UTC+2, so 16:30B is 14:30Z.
			instant: time.Date(2026, time.August, 9, 14, 30, 0, 0, time.UTC),
		},
		{
			name:  "negative zone letter shifts forward to UTC",
			value: "091630RAUG26",
			want:  DTG{Day: 9, Hour: 16, Minute: 30, Zone: 'R', Month: time.August, Year: 2026},
			// R is UTC-5, so 16:30R is 21:30Z.
			instant: time.Date(2026, time.August, 9, 21, 30, 0, 0, time.UTC),
		},
		{
			name:  "zone offset can roll the date",
			value: "092300MAUG26",
			want:  DTG{Day: 9, Hour: 23, Minute: 0, Zone: 'M', Month: time.August, Year: 2026},
			// M is UTC+12, so 23:00M on the 9th is 11:00Z on the 9th.
			instant: time.Date(2026, time.August, 9, 11, 0, 0, 0, time.UTC),
		},
		{
			name:    "lowercase month is accepted",
			value:   "091630Zaug26",
			want:    DTG{Day: 9, Hour: 16, Minute: 30, Zone: 'Z', Month: time.August, Year: 2026},
			instant: time.Date(2026, time.August, 9, 16, 30, 0, 0, time.UTC),
		},
		{
			name:    "leap day is valid in a leap year",
			value:   "290000ZFEB24",
			want:    DTG{Day: 29, Hour: 0, Minute: 0, Zone: 'Z', Month: time.February, Year: 2024},
			instant: time.Date(2024, time.February, 29, 0, 0, 0, 0, time.UTC),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := parseDTG(tc.value, ref)
			if !ok {
				t.Fatalf("parseDTG(%q) rejected it, want accepted", tc.value)
			}
			if got != tc.want {
				t.Fatalf("parseDTG(%q) = %+v, want %+v", tc.value, got, tc.want)
			}
			if instant := got.resolveInstant(); !instant.Equal(tc.instant) {
				t.Fatalf("resolveInstant() = %s, want %s", instant, tc.instant)
			}
		})
	}
}

func TestParseRejections(t *testing.T) {
	cases := []struct {
		name  string
		value string
		why   string
	}{
		{"31 February", "311200ZFEB26", "normalising it would silently show 3 March"},
		{"31 April", "311200ZAPR26", "April has 30 days"},
		{"29 February in a common year", "290000ZFEB26", "2026 is not a leap year"},
		{"day zero", "001200ZAUG26", "there is no day 0"},
		{"day 32", "321200ZAUG26", "no month has 32 days"},
		{"hour 24", "092400ZAUG26", "hours run 00-23"},
		{"minute 60", "091660ZAUG26", "minutes run 00-59"},
		{"unknown month", "091630ZXXX26", "XXX is not a month"},
		{"zone letter I", "091630IAUG26", "I is skipped in the military alphabet"},
		{"zone letter J", "091630JAUG26", "J is the observer's local time and is reader-dependent"},
		{"short form with a non-Z zone", "091630B", "the bare form is too loose to claim for any letter"},
		{"short form with letter J", "091630J", "J is rejected everywhere"},
		{"too short", "09163Z", "not a DTG"},
		{"non-digit in the time", "09163AZAUG26", "the first six characters must be digits"},
		{"trailing junk", "091630ZAUG26X", "not one of the accepted forms"},
		{"month-only, no year", "091630ZAUG", "the year is not optional on the long form"},
		{"three digit year", "091630ZAUG263", "years are two or four digits"},

		// The canonical form carries two year digits. Accepting these would
		// canonicalise 2150 to "50", which reads back as 2050, so the link
		// would describe a different century from the author's text.
		{"year before the accepted century", "091630ZAUG1999", "the canonical form cannot represent it"},
		{"year after the accepted century", "091630ZAUG2150", "the canonical form cannot represent it"},
		{"far future year", "091630ZAUG9999", "the canonical form cannot represent it"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, ok := parseDTG(tc.value, ref); ok {
				t.Fatalf("parseDTG(%q) accepted it, want rejected: %s", tc.value, tc.why)
			}
		})
	}
}

// The adjacency case the pattern design exists to prevent: a date with no zone
// letter must not be read as a short-form DTG plus trailing garbage.
func TestJulPatternIsNotMisreadAsShortForm(t *testing.T) {
	if _, ok := parseDTG("091630J", ref); ok {
		t.Fatal("parseDTG(091630J) accepted it, want rejected")
	}
	if _, ok := parseDTG("091630JUL26", ref); ok {
		t.Fatal("parseDTG(091630JUL26) accepted it, want rejected because J is not a usable zone")
	}
}

func TestShortFormUsesReferenceMonthAndYear(t *testing.T) {
	december := time.Date(2027, time.December, 31, 23, 0, 0, 0, time.UTC)

	got, ok := parseDTG("011200Z", december)
	if !ok {
		t.Fatal("parseDTG(011200Z) rejected it, want accepted")
	}
	if got.Month != time.December || got.Year != 2027 {
		t.Fatalf("parseDTG() = %s %d, want December 2027 from the reference", got.Month, got.Year)
	}
	if !got.AssumedMonth || !got.AssumedYear {
		t.Fatalf("parseDTG() assumed flags = (%v, %v), want both true", got.AssumedMonth, got.AssumedYear)
	}
}

func TestTwoDigitYearMapsToTwentyFirstCentury(t *testing.T) {
	got, ok := parseDTG("091630ZAUG05", ref)
	if !ok {
		t.Fatal("parseDTG rejected a valid DTG")
	}
	if got.Year != 2005 {
		t.Fatalf("year = %d, want 2005", got.Year)
	}
}

// parse then re-format must round trip, which is what proves the components
// were not silently normalised into a different date.
func TestCanonicalRoundTrip(t *testing.T) {
	for _, value := range []string{"091630ZAUG26", "010000ZJAN00", "312359ZDEC99", "290000ZFEB24"} {
		t.Run(value, func(t *testing.T) {
			parsed, ok := parseDTG(value, ref)
			if !ok {
				t.Fatalf("parseDTG(%q) rejected it", value)
			}
			if got := parsed.canonical(); got != value {
				t.Fatalf("canonical() = %q, want %q", got, value)
			}
		})
	}
}

// The four-digit form is deliberately normalised to the canonical two-digit
// one. The link label keeps the author's original text either way.
func TestFourDigitYearNormalisesToCanonical(t *testing.T) {
	parsed, ok := parseDTG("091630ZAUG2026", ref)
	if !ok {
		t.Fatal("parseDTG rejected a valid four-digit-year DTG")
	}
	if got := parsed.canonical(); got != "091630ZAUG26" {
		t.Fatalf("canonical() = %q, want 091630ZAUG26", got)
	}
}

func TestShortFormCanonicalOmitsAssumedFields(t *testing.T) {
	parsed, ok := parseDTG("091630Z", ref)
	if !ok {
		t.Fatal("parseDTG rejected a valid short-form DTG")
	}
	if got := parsed.canonical(); got != "091630Z" {
		t.Fatalf("canonical() = %q, want 091630Z with no inferred month or year", got)
	}
}

func TestAssumedCode(t *testing.T) {
	cases := []struct {
		value string
		want  string
	}{
		{"091630ZAUG26", ""},
		{"091630Z", "my"},
	}

	for _, tc := range cases {
		t.Run(tc.value, func(t *testing.T) {
			parsed, ok := parseDTG(tc.value, ref)
			if !ok {
				t.Fatalf("parseDTG(%q) rejected it", tc.value)
			}
			if got := parsed.assumedCode(); got != tc.want {
				t.Fatalf("assumedCode() = %q, want %q", got, tc.want)
			}
		})
	}
}

// Anything this produces must be something the decorator recognises, so
// generated example text cannot drift away from the grammar.
func TestFormatZuluRoundTrips(t *testing.T) {
	instants := []time.Time{
		time.Date(2026, time.August, 9, 16, 30, 0, 0, time.UTC),
		time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC),
		time.Date(2026, time.December, 31, 23, 59, 0, 0, time.UTC),
		time.Date(2024, time.February, 29, 12, 0, 0, 0, time.UTC),

		// Seconds have nowhere to go in a DTG, so they are dropped and the
		// instant lands on the minute below.
		time.Date(2026, time.August, 9, 16, 30, 45, 0, time.UTC),

		// A non-UTC input still has to come out as Zulu.
		time.Date(2026, time.August, 9, 16, 30, 0, 0, time.FixedZone("test", 5*3600)),
	}

	for _, instant := range instants {
		t.Run(instant.String(), func(t *testing.T) {
			formatted := FormatZulu(instant)

			parsed, ok := parseDTG(formatted, ref)
			if !ok {
				t.Fatalf("FormatZulu produced %q, which the decorator declines", formatted)
			}

			want := instant.UTC().Truncate(time.Minute)
			if got := parsed.resolveInstant(); !got.Equal(want) {
				t.Fatalf("round trip = %s, want %s", got, want)
			}
			if parsed.Zone != 'Z' {
				t.Fatalf("zone = %c, want Z", parsed.Zone)
			}
		})
	}
}

func TestDaysInMonth(t *testing.T) {
	cases := []struct {
		year  int
		month time.Month
		want  int
	}{
		{2026, time.February, 28},
		{2024, time.February, 29},
		{2000, time.February, 29}, // divisible by 400
		{1900, time.February, 28}, // divisible by 100 but not 400
		{2026, time.April, 30},
		{2026, time.December, 31},
	}

	for _, tc := range cases {
		if got := daysInMonth(tc.year, tc.month); got != tc.want {
			t.Fatalf("daysInMonth(%d, %s) = %d, want %d", tc.year, tc.month, got, tc.want)
		}
	}
}
