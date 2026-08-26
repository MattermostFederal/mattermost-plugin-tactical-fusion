package location

import (
	"fmt"
	"math"
	"math/rand"
	"regexp"
	"strings"
	"testing"
)

// acceptedTokens is the positive corpus.
//
// The USMTF rows are the vectors from mattermost-plugin-aocanywhere's
// server/model/usmtf2004/sets/location_test.go, reused deliberately: the two
// plugins must agree about what a LATM is, and borrowing the sibling's cases is
// cheaper and more honest than inventing parallel ones.
var acceptedTokens = []struct {
	format    Format
	token     string
	canonical string
	lat, lon  float64
}{
	// Signed decimal degrees.
	{FormatDD, "34.0561, -118.2500", "34.0561,-118.2500", 34.0561, -118.25},
	{FormatDD, "-33.8688,151.2093", "-33.8688,151.2093", -33.8688, 151.2093},
	{FormatDD, "+34.0561,+118.2500", "34.0561,118.2500", 34.0561, 118.25},

	// A zero half beside a real one is an ordinary coordinate. Only the pair of
	// zeroes is declined, and a truthiness check would drop both.
	{FormatDD, "0.0000,32.5000", "0.0000,32.5000", 0, 32.5},
	{FormatDD, "12.3456,0.0000", "12.3456,0.0000", 12.3456, 0},

	// Decimal degrees with hemisphere letters.
	{FormatDDH, "34.0561 N, 118.2500 W", "34.0561N,118.2500W", 34.0561, -118.25},
	{FormatDDH, "34.0561°N 118.2500°W", "34.0561N,118.2500W", 34.0561, -118.25},
	{FormatDDH, "N34.0561 W118.2500", "34.0561N,118.2500W", 34.0561, -118.25},
	{FormatDDH, "34.05S, 18.42E", "34.05S,18.42E", -34.05, 18.42},

	// Degrees, minutes, seconds, in every spelling.
	{FormatDMS, "34°03'22\"N 118°15'00\"W", "340322N1181500W", 34.0561111, -118.25},
	{FormatDMS, "34° 03' 22\" N, 118° 15' 00\" W", "340322N1181500W", 34.0561111, -118.25},
	{FormatDMS, "34 03 22 N 118 15 00 W", "340322N1181500W", 34.0561111, -118.25},
	{FormatDMS, "400948N1221400W", "400948N1221400W", 40.1633333, -122.2333333},

	// Smart quotes, which a phone keyboard produces unbidden.
	{FormatDMS, "34°03′22″N 118°15′00″W", "340322N1181500W", 34.0561111, -118.25},

	// USMTF LATDS and DMPID: seconds with a fraction.
	{FormatDMS, "331000.0N1183000.0W", "331000.0N1183000.0W", 33.1666667, -118.5},
	{FormatDMS, "641230.0N1683045.0W", "641230.0N1683045.0W", 64.2083333, -168.5125},

	// Degrees and decimal minutes, which USMTF calls GEOK.
	{FormatDDM, "34°03.366'N 118°15.000'W", "3403.366N11815.000W", 34.0561, -118.25},
	{FormatDDM, "3403.366N 11815.000W", "3403.366N11815.000W", 34.0561, -118.25},
	{FormatDDM, "3510.234N07901.123W", "3510.234N07901.123W", 35.1705667, -79.0187167},

	// The USMTF fixed-width family.
	{FormatLATD, "35N079W", "35N079W", 35, -79},
	{FormatLATD, "40N122W", "40N122W", 40, -122},
	{FormatLATM, "3510N07901W", "3510N07901W", 35.1666667, -79.0166667},
	{FormatLATM, "2130N15730W", "2130N15730W", 21.5, -157.5},
	{FormatLATM, "2621N12746E", "2621N12746E", 26.35, 127.7666667},
	{FormatVLATM, "3510N9-07901W7", "3510N9-07901W7", 35.1666667, -79.0166667},

	// The grid grammars. Positions are the center of the square, which is what
	// a grid reference names, so they are a regression pin on the projection
	// rather than an independent check of it; geodesy_test.go holds the
	// projection itself to figures with an authority outside this repository.
	{FormatMGRS, "18S UJ 23478 06483", "18SUJ2347806483", 38.8895024, -77.0352950},
	{FormatMGRS, "18SUJ2347806483", "18SUJ2347806483", 38.8895024, -77.0352950},
	{FormatMGRS, "18s uj 23478 06483", "18SUJ2347806483", 38.8895024, -77.0352950},
	{FormatMGRS, "11S LT 12345 67890", "11SLT1234567890", 34.0349117, -119.0327116},
	{FormatMGRS, "32U MV 12 34", "32UMV1234", 49.0571522, 7.8023224},

	// The southern hemisphere, whose northing is measured from a false origin
	// 10,000 km south of the equator.
	{FormatMGRS, "56H LH 34900 52288", "56HLH3490052288", -33.8568023, 151.2152992},

	// A square the plain formula does not produce. Five degrees east is zone 31
	// by the formula; south-west Norway was widened to zone 32 in 1950, so this
	// square exists only with that exception implemented, and its center lands
	// on the nose at 60 north, 5 east.
	{FormatMGRS, "32V KM 76979 58157", "32VKM7697958157", 60.0000024, 4.9999920},

	{FormatUTM, "33U 291000 5628000", "33U2910005628000", 50.7660671, 12.0361164},
	{FormatUTM, "33U2910005628000", "33U2910005628000", 50.7660671, 12.0361164},

	{FormatGEOREF, "GJNJ5753", "GJNJ5753", 38.8916667, -77.0416667},
	{FormatGEOREF, "gjnj5753", "GJNJ5753", 38.8916667, -77.0416667},
	{FormatGEOREF, "GJNJ575337", "GJNJ575337", 38.5625, -77.0408333},
	{FormatGEOREF, "GJNJ57533752", "GJNJ57533752", 38.6254167, -77.0410833},

	{FormatGARS, "206LT", "206LT", 38.75, -77.25},
	{FormatGARS, "206LT2", "206LT2", 38.875, -77.125},
	{FormatGARS, "206LT26", "206LT26", 38.875, -77.0416667},
	{FormatGARS, "006ag39", "006AG39", -86.9583333, -177.2916667},

	{FormatPlusCode, "8FVC2222+22", "8FVC2222+22", 47.0000625, 8.0000625},
	{FormatPlusCode, "8fvc2222+22", "8FVC2222+22", 47.0000625, 8.0000625},
	{FormatPlusCode, "849VCWC8+R9", "849VCWC8+R9", 37.4220625, -122.0840625},
	{FormatPlusCode, "849VCWC8+R9C", "849VCWC8+R9C", 37.4220625, -122.084109375},
	{FormatPlusCode, "849VCWC8+", "849VCWC8+", 37.42125, -122.08375},
	{FormatPlusCode, "849V0000+", "849V0000+", 37.5, -122.5},
}

func TestParseAcceptsAndCanonicalizes(t *testing.T) {
	for _, tc := range acceptedTokens {
		t.Run(tc.token, func(t *testing.T) {
			loc, ok := Parse(tc.format, tc.token)
			if !ok {
				t.Fatalf("Parse(%s, %q) rejected a valid token", tc.format, tc.token)
			}

			if got := loc.Canonical(); got != tc.canonical {
				t.Fatalf("Canonical() = %q, want %q", got, tc.canonical)
			}

			// Point rather than Lat.Decimal, because the grid grammars keep no
			// Axis at all: their position is projected from the token's letters
			// and digits, which is the whole difference between the two halves
			// of this corpus.
			lat, lon, ok := loc.Point()
			if !ok {
				t.Fatalf("Point() failed for a token Parse had just accepted")
			}

			if math.Abs(lat-tc.lat) > 1e-6 {
				t.Errorf("latitude = %v, want %v", lat, tc.lat)
			}
			if math.Abs(lon-tc.lon) > 1e-6 {
				t.Errorf("longitude = %v, want %v", lon, tc.lon)
			}
		})
	}
}

// The round trip is what lets a link be validated against nothing but itself,
// so it has to be exact and it has to be reachable from the canonical form.
//
// This is the test a float64 intermediate fails: 340322N parses to
// 34.05611111..., and rebuilding whole seconds from that can land on
// 21.99999999, which comes back out as 340321N. A token accepted at decoration
// would then be rejected by the page behind it.
func TestCanonicalRoundTripsExactly(t *testing.T) {
	for _, tc := range acceptedTokens {
		t.Run(tc.canonical, func(t *testing.T) {
			reparsed, ok := Parse(tc.format, tc.canonical)
			if !ok {
				t.Fatalf("Parse(%s, %q) rejected its own canonical form", tc.format, tc.canonical)
			}

			if got := reparsed.Canonical(); got != tc.canonical {
				t.Fatalf("canonical is not idempotent: %q became %q", tc.canonical, got)
			}
		})
	}
}

// Each half keeps its own digits, and only the RESOLUTION is the coarser half.
//
// An earlier version wrote both halves at the pair's coarser count, which moved
// the finer one: "34.1 N, 118.2500 W" canonicalized to "34.1N,118.2W", putting
// the stored longitude about 4 km from the one the author typed. That is silent
// corruption of the thing the link is supposed to identify, so the two concepts
// are kept apart.
func TestCanonicalKeepsEachHalfsOwnDigits(t *testing.T) {
	cases := []struct {
		format    Format
		token     string
		canonical string
		digits    int
	}{
		{FormatDD, "34.0561,-118.25000", "34.0561,-118.25000", 4},
		{FormatDDH, "34.1 N, 118.2500 W", "34.1N,118.2500W", 1},
		{FormatDD, "34.05619,-118.2501", "34.05619,-118.2501", 4},
	}

	for _, tc := range cases {
		t.Run(tc.token, func(t *testing.T) {
			loc, ok := Parse(tc.format, tc.token)
			if !ok {
				t.Fatalf("Parse(%s, %q) rejected a valid token", tc.format, tc.token)
			}

			if got := loc.Canonical(); got != tc.canonical {
				t.Fatalf("Canonical() = %q, want %q", got, tc.canonical)
			}
			if got := loc.Digits(); got != tc.digits {
				t.Fatalf("Digits() = %d, want %d", got, tc.digits)
			}

			// And it still round-trips, which is what a link depends on.
			again, ok := Parse(tc.format, tc.canonical)
			if !ok || again.Canonical() != tc.canonical {
				t.Fatalf("canonical is not a fixed point for %q", tc.canonical)
			}
		})
	}
}

// Every rune the grammars can put inside a token must be one the "r" parameter
// accepts.
//
// These are two hand-maintained whitelists that have to describe the same set.
// When they did not, a coordinate split across a line break was decorated and
// its own page then refused the newline, leaving a permanently dead link in
// somebody's message.
func TestGrammarAlphabetMatchesRawAlphabet(t *testing.T) {
	// Every literal separator the expressions can emit, plus the symbol
	// variants and the digits and letters the fields are made of.
	//
	// The letters are the whole alphabet in both cases because the grid
	// grammars draw band and 100 km square letters from nearly all of it, and
	// accept lower case the way every other grammar here does.
	const emitted = "0123456789" +
		"ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz" +
		".,-+'\" \u00b0\u00ba\u2032\u2019\u00b4\u2033\u201d"

	for _, r := range emitted {
		if !allowedRawRunes(string(r)) {
			t.Errorf("the grammars can produce %q but allowedRawRunes rejects it", r)
		}
	}

	// And the whitespace the grammars must NOT admit, because the r alphabet
	// cannot carry it.
	for _, r := range "\t\n\r\f" {
		if allowedRawRunes(string(r)) {
			t.Errorf("allowedRawRunes accepts %q; if that is intended the grammars may use it too", r)
		}
	}
}

func TestVerifiedTokenKeepsConfidenceOutOfThePosition(t *testing.T) {
	loc, ok := Parse(FormatVLATM, "3510N9-07901W7")
	if !ok {
		t.Fatal("Parse rejected a valid VLATM")
	}

	lat, lon, ok := loc.Confidence()
	if !ok {
		t.Fatal("Confidence() reported none, want the token's digits")
	}
	if lat != 9 || lon != 7 {
		t.Fatalf("Confidence() = (%d, %d), want (9, 7)", lat, lon)
	}

	// The same position without the digits, so confidence cannot have leaked in.
	plain, _ := Parse(FormatLATM, "3510N07901W")
	if loc.Lat.Decimal() != plain.Lat.Decimal() || loc.Lon.Decimal() != plain.Lon.Decimal() {
		t.Fatal("confidence digits changed the position")
	}
}

// rejectedTokens is the negative corpus, and it is the design.
//
// Each row is a shape that must never be rewritten into a link, because doing
// so permanently edits something a user wrote.
var rejectedTokens = []struct {
	name   string
	format Format
	token  string
}{
	{"fewer than four decimals", FormatDD, "34.05, -118.25"},
	{"integers", FormatDD, "12, 34"},
	{"no comma", FormatDD, "34.0561 -118.2500"},
	{"null island", FormatDD, "0.0000, 0.0000"},
	{"latitude out of range", FormatDD, "91.0000, 10.0000"},
	{"longitude out of range", FormatDD, "10.0000, 181.0000"},
	{"epoch seconds", FormatDD, "1723385400.1234, 1723385400.5678"},
	{"comma as a decimal separator", FormatDD, "34,0561, -118,2500"},
	{"more fractional digits than a float can reproduce", FormatDD, "34.0561000000,-118.2500000000"},

	{"newtons and watts", FormatDDH, "12 N, 5 W"},
	{"no decimal point", FormatDDH, "34 N, 118 W"},
	{"sign and hemisphere together", FormatDDH, "-34.0561N,118.2500W"},

	{"minutes past 59", FormatDMS, "346022N1181500W"},
	{"seconds past 59", FormatDMS, "340360N1181500W"},
	{"degrees past 90", FormatDMS, "910000N1181500W"},
	{"past the pole", FormatDMS, "900001N1181500W"},
	{"no hemisphere letters", FormatDMS, "34 03 22 118 15 00"},

	{"minutes past 59 in ddm", FormatDDM, "3460.000N11815.000W"},

	{"aocanywhere accepts this, and it is latitude 99.98", FormatLATM, "9999N99999W"},
	{"letters where digits belong", FormatLATM, "35XXN07901W"},

	// The grid grammars, where most of the validation is geometric rather than
	// textual.
	{
		// aocanywhere's pattern for this field is `^\d{1,2}[A-Z]{3}\d{6}$`,
		// which accepts any three letters. Only eight of the twenty-four
		// possible column letters are legal in a given zone, so two thirds of
		// what that pattern matches names no square at all. Zone 32 takes
		// J to R, and D is not among them.
		"a 100 km column letter from another zone's set", FormatMGRS, "32WDL123123",
	},
	{"a row letter that is not one", FormatMGRS, "18S UI 23478 06483"},
	{"a band letter that is not one", FormatMGRS, "18O UJ 23478 06483"},
	{"zone 0", FormatMGRS, "0S UJ 23478 06483"},
	{"zone 61", FormatMGRS, "61S UJ 23478 06483"},
	{"halves of different widths", FormatMGRS, "18S UJ 2347 06483"},
	{"an odd number of digits", FormatMGRS, "18SUJ234780648"},
	{"more digits than the notation has", FormatMGRS, "18SUJ234780648312"},
	{"no digits at all, which is a 100 km square", FormatMGRS, "18SUJ"},
	{"a three-digit zone", FormatMGRS, "118SUJ2347806483"},
	{
		// The square is real and the band is real, but not together: UJ in zone
		// 18 is in the northern hemisphere and band C is between 80 and 72
		// south.
		"a square that cannot be in the band it claims", FormatMGRS, "18C UJ 23478 06483",
	},

	{
		// N and S are read as bands rather than hemispheres, so what declines
		// them is the same thing that declines any other letter: the position
		// has to be inside the band the token names. 3769000 is 34 north, which
		// is band S, so band N is refused for it.
		"a northing outside band N", FormatUTM, "11N 385000 3769000",
	},
	{"a northing outside band S", FormatUTM, "11S 385000 1200000"},
	{"an easting with too few digits", FormatUTM, "33U 91000 5628000"},
	{"a northing with too few digits", FormatUTM, "33U 291000 628000"},
	{"a northing past the pole", FormatUTM, "33U 291000 9999999"},
	{"a band the northing cannot be in", FormatUTM, "33C 291000 5628000"},

	{"an I in a GEOREF zone, which reads as a 1", FormatGEOREF, "IJNJ5753"},
	{"a GEOREF band past M, which is past the pole", FormatGEOREF, "GNNJ5753"},
	{"a GEOREF degree unit past Q, which is past the zone", FormatGEOREF, "GJRJ5753"},
	{"GEOREF minutes past 59", FormatGEOREF, "GJNJ6053"},
	{"GEOREF tenths past 599", FormatGEOREF, "GJNJ600533"},
	{"an odd number of GEOREF digits", FormatGEOREF, "GJNJ57533"},
	{"a bare GEOREF quadrangle, which is four letters and therefore a word", FormatGEOREF, "GJNJ"},

	{"GARS band 0", FormatGARS, "000LT"},
	{"GARS band past 720", FormatGARS, "721LT"},
	{"GARS letters past the 360 bands that exist", FormatGARS, "206ZZ"},
	{"a GARS quadrant past 4", FormatGARS, "206LT5"},
	{"a GARS keypad cell of 0", FormatGARS, "206LT20"},

	{"a Plus Code separator in the wrong place", FormatPlusCode, "849VCW+C8R9"},
	{"a Plus Code with a letter outside the alphabet", FormatPlusCode, "849VCWA8+R9"},
	{"a Plus Code of nine significant characters", FormatPlusCode, "849VCWC8+R"},
	{"a Plus Code past fifteen characters", FormatPlusCode, "849VCWC8+R9CVWXQ2"},
	{"a Plus Code whose latitude pair is off the top of the world", FormatPlusCode, "F49VCWC8+R9"},
	{"a Plus Code padded with an odd number of zeroes", FormatPlusCode, "849VC000+"},
	{"a padded Plus Code carrying detail after the separator", FormatPlusCode, "849V0000+R9"},
	{"a short Plus Code", FormatPlusCode, "CWC8+R9"},
}

func TestParseDeclines(t *testing.T) {
	for _, tc := range rejectedTokens {
		t.Run(tc.name, func(t *testing.T) {
			if _, ok := Parse(tc.format, tc.token); ok {
				t.Fatalf("Parse(%s, %q) accepted a token that must be declined", tc.format, tc.token)
			}
		})
	}
}

// A format id is a closed enum. Trying every grammar until one parses would
// make it decorative and let a crafted link claim a notation its token was
// never written in.
func TestParseRejectsUnknownFormat(t *testing.T) {
	for _, id := range []Format{"usng", "olc", "mgrs2", ""} {
		if _, ok := Parse(id, "34.0561,-118.2500"); ok {
			t.Errorf("Parse accepted the unknown format id %q", id)
		}
		if KnownFormat(id) {
			t.Errorf("KnownFormat accepted %q, an id this build does not implement", id)
		}
	}
}

// A grammar must not accept a token belonging to another one, or the format id
// on the page would be a guess.
func TestGrammarsDoNotAcceptEachOthersTokens(t *testing.T) {
	for _, tc := range acceptedTokens {
		for _, f := range AllFormatIDs {
			if f == tc.format {
				continue
			}
			// dd and ddh both describe decimal degrees, and the fixed-width
			// USMTF shapes are distinguished by digit count alone, so this is a
			// real check rather than a tautology.
			if _, ok := Parse(f, tc.token); ok {
				t.Errorf("Parse(%s, %q) accepted a token that is %s", f, tc.token, tc.format)
			}
		}
	}
}

// Every canonical form must be plain ASCII, which keeps non-ASCII off a public
// HTML page and keeps the anchored patterns narrow.
func TestCanonicalIsASCII(t *testing.T) {
	for _, tc := range acceptedTokens {
		loc, ok := Parse(tc.format, tc.token)
		if !ok {
			t.Fatalf("Parse(%s, %q) rejected a valid token", tc.format, tc.token)
		}

		for _, r := range loc.Canonical() {
			if r < 0x20 || r > 0x7e {
				t.Fatalf("Canonical() of %q contains %q, want printable ASCII only", tc.token, r)
			}
		}
	}
}

// The anchored token expression is what validates the "r" parameter, so it must
// accept exactly what the scanner would have matched and nothing around it.
func TestAnchoredTokenGrammarRejectsSurroundingProse(t *testing.T) {
	const coordinate = "34.0561, -118.2500"

	if !MatchesTokenGrammar(FormatDD, coordinate) {
		t.Fatal("MatchesTokenGrammar rejected a token the scanner accepts")
	}

	for _, spoof := range []string{
		"PRIORITY TARGET " + coordinate + " CONFIRMED",
		coordinate + " CONFIRMED",
		"see " + coordinate,
		strings.Repeat(coordinate, 2),
	} {
		if MatchesTokenGrammar(FormatDD, spoof) {
			t.Errorf("MatchesTokenGrammar accepted %q, want only the bare token", spoof)
		}
	}
}

// Nothing shared between the grammars may introduce a capture group.
//
// A single "(?:" typed as "(" would break two things at once and neither
// loudly. One alternate's group numbering shifts, so minutes get filed as
// degrees in one spelling only; and every bare scanning pattern gains a group,
// at which point Pattern.Value starts labeling links with a fragment of the
// token instead of the token.
func TestSharedSubExpressionsHaveNoCaptureGroups(t *testing.T) {
	for name, expr := range map[string]string{
		"sp": sp, "degSep": degSep, "minSep": minSep, "secEnd": secEnd, "half": half,
		"degSym": degSym, "minSym": minSym, "secSym": secSym, "ns": ns, "ew": ew,
	} {
		if n := regexp.MustCompile(expr).NumSubexp(); n != 0 {
			t.Errorf("%s has %d capture groups, want 0", name, n)
		}
	}
}

// A bare scanning pattern must capture nothing, or the framework hands Parse
// and the link label a fragment rather than the whole token.
func TestBareScanningPatternsHaveNoCaptureGroups(t *testing.T) {
	for _, f := range AllFormatIDs {
		if n := regexp.MustCompile(scanExpr(f)).NumSubexp(); n != 0 {
			t.Errorf("the scanning expression for %s has %d capture groups, want 0", f, n)
		}
	}
}

// The two spellings of a format are read with the same group indices, so their
// group counts have to agree. They do today; nothing else notices if they stop.
func TestAlternateParsersAgreeOnGroupCount(t *testing.T) {
	pairs := []struct {
		name string
		a, b *regexp.Regexp
	}{
		{"dms", dmsSepRe, dmsCompactRe},
		{"ddm", ddmSepRe, ddmCompactRe},
		{"ddh", ddhSuffixRe, ddhPrefixRe},
	}

	for _, p := range pairs {
		if p.a.NumSubexp() != p.b.NumSubexp() {
			t.Errorf("%s: the two spellings capture %d and %d groups; the parser reads both with one set of indices",
				p.name, p.a.NumSubexp(), p.b.NumSubexp())
		}
	}
}

// N and S after a zone number are read as latitude BANDS, not hemispheres.
//
// This is the one place in the package where an ambiguity is resolved by
// convention rather than removed, so it is pinned by the position rather than
// by acceptance: the two readings of "11S 384640 3769080" are 90 degrees of
// latitude apart, and a test that only checked the token parsed would pass
// under either.
func TestUTMBandLettersAreBandsNotHemispheres(t *testing.T) {
	loc, ok := Parse(FormatUTM, "11S 384640 3769080")
	if !ok {
		t.Fatal("Parse declined a military UTM position")
	}

	lat, lon, ok := loc.Point()
	if !ok {
		t.Fatal("the position could not be derived")
	}

	// Band S is 32 to 40 north. The hemisphere reading of the same token is
	// 56.2 south, so anything negative here means the convention flipped.
	if lat < 32 || lat > 40 {
		t.Fatalf("latitude = %.4f, want it inside band S (32 to 40 north); "+
			"a negative value means S was read as a hemisphere", lat)
	}
	if lon > -114 || lon < -120 {
		t.Fatalf("longitude = %.4f, want it inside zone 11", lon)
	}
}

// Reading N as a band can decline a token but can never misplace one.
//
// Hemisphere-north and band N both use the northing as written, so wherever the
// two readings both accept, they agree. The band reading is simply the stricter
// of the two, and this is what says so: every accepted band-N token lands in
// the first 8 degrees north, which is northern under either reading.
//
// The asymmetry matters because it is the whole reason S is the letter with a
// cost and N is not.
func TestBandNCannotMisplaceAPosition(t *testing.T) {
	for _, northing := range []string{"0000100", "0250000", "0500000", "0884000"} {
		token := "11N 384640 " + northing

		loc, ok := Parse(FormatUTM, token)
		if !ok {
			t.Errorf("Parse declined %q", token)
			continue
		}

		lat, _, ok := loc.Point()
		if !ok {
			t.Errorf("%q gave no position", token)
			continue
		}

		if lat < 0 || lat > 8 {
			t.Errorf("%q is at latitude %.4f, outside band N", token, lat)
		}
	}
}

// How often reading S as a band misplaces a civilian southern-hemisphere token.
//
// Measured rather than argued, because the number is the whole basis of the
// decision. A civilian "zone Z, southern hemisphere" token is misread only when
// its northing ALSO lands inside band S under the northern reading, so most of
// them are declined by the band check rather than silently relocated.
//
// This is a property of the notation, not of this code, so the assertion is a
// loose bound: it exists to fail if a change to the band check quietly widens
// the window, not to pin a particular percentage.
func TestSouthernHemisphereTokensMostlyDecline(t *testing.T) {
	rng := rand.New(rand.NewSource(20260812)) //nolint:gosec // deterministic on purpose, not a security context

	const samples = 20000
	accepted := 0

	for range samples {
		// A civilian southern-hemisphere pair: zone, "S", and a northing
		// measured from the false origin, which is what 80 south to the equator
		// spans.
		zone := rng.Intn(60) + 1
		easting := rng.Intn(maxGridEasting-minGridEasting) + minGridEasting
		northing := rng.Intn(10000000-1100000) + 1100000

		token := fmt.Sprintf("%dS %06d %07d", zone, easting, northing)
		if _, ok := Parse(FormatUTM, token); ok {
			accepted++
		}
	}

	rate := float64(accepted) / float64(samples)
	t.Logf("%d of %d southern-hemisphere tokens survive the band S check (%.1f%%)",
		accepted, samples, rate*100)

	if rate > 0.2 {
		t.Fatalf("%.1f%% of civilian southern tokens are accepted as band S; "+
			"the band containment check is not doing its job", rate*100)
	}
}

// The axis letters an author writes on a UTM pair are accepted and carry no
// meaning of their own: the position is the same with or without them.
func TestUTMAxisLettersAreAccepted(t *testing.T) {
	plain, ok := Parse(FormatUTM, "11S 384640 3769080")
	if !ok {
		t.Fatal("Parse declined the unsuffixed form")
	}

	for _, token := range []string{
		"11S 384640E 3769080N",
		"11S 384640mE 3769080mN",
		"11s 384640e 3769080n",
		"11S 384640E 3769080",
		"11S 384640 3769080N",
	} {
		t.Run(token, func(t *testing.T) {
			loc, ok := Parse(FormatUTM, token)
			if !ok {
				t.Fatalf("Parse declined %q", token)
			}

			// Same canonical form, which is what makes the letters display
			// only: the link carries the same identity either way.
			if loc.Canonical() != plain.Canonical() {
				t.Fatalf("canonical = %q, want %q", loc.Canonical(), plain.Canonical())
			}
		})
	}
}

// decimalDegrees refuses either half independently.
//
// Only reachable directly: the scanning pattern bounds the digit counts before
// this is called, so no message can present it with 99 degrees of latitude. The
// range check is what makes the bound a property of the parser rather than of
// the pattern, which is the bug this package deliberately does not inherit from
// mattermost-plugin-aocanywhere, where "9999N99999W" parses to latitude 99.98.
func TestDecimalDegreesRefusesEitherHalfOutOfRange(t *testing.T) {
	for _, tc := range []struct {
		name                             string
		latHemi, latDeg, lonHemi, lonDeg string
		want                             bool
	}{
		{"both in range", "N", "34", "W", "118", true},
		{"latitude past the pole", "N", "99", "W", "118", false},
		{"longitude past the antimeridian", "N", "34", "W", "999", false},
		{"both out of range", "N", "99", "W", "999", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, ok := decimalDegrees(tc.latHemi, tc.latDeg, "0561", tc.lonHemi, tc.lonDeg, "2500")
			if ok != tc.want {
				t.Fatalf("decimalDegrees(%s%s, %s%s) ok = %v, want %v",
					tc.latDeg, tc.latHemi, tc.lonDeg, tc.lonHemi, ok, tc.want)
			}
		})
	}
}

// upperByte guards an empty capture rather than indexing it.
//
// Every caller passes a mandatory one-character group, so this is unreachable
// from a message today. It runs inside MessageWillBePosted, where an index
// panic is one grammar edit away from stopping somebody posting.
func TestUpperByteGuardsAnEmptyCapture(t *testing.T) {
	for _, tc := range []struct {
		name string
		in   string
		want byte
	}{
		{"a lower-case letter is raised", "n", 'N'},
		{"an upper-case one is left alone", "N", 'N'},
		{"an empty capture yields no byte rather than panicking", "", 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := upperByte(tc.in); got != tc.want {
				t.Fatalf("upperByte(%q) = %d, want %d", tc.in, got, tc.want)
			}
		})
	}
}

// The pattern offers one digit, so the range check is what makes 0..9 true of
// the type rather than only of the caller.
func TestConfidenceDigitBoundsWhatItAccepts(t *testing.T) {
	for _, tc := range []struct {
		name string
		in   string
		want int8
	}{
		{"the lowest digit", "0", 0},
		{"the highest", "9", 9},
		{"past the highest", "10", NoConfidence},
		{"not a number at all", "x", NoConfidence},

		// atoi reads an absent capture as zero rather than as a failure, so an
		// empty string here is the digit 0 and not "no digit". Every caller
		// passes a group that matched, so nothing reaches this today; it is
		// recorded because the two readings are easy to confuse.
		{"an empty capture reads as the digit zero", "", 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := confidenceDigit(tc.in); got != tc.want {
				t.Fatalf("confidenceDigit(%q) = %d, want %d", tc.in, got, tc.want)
			}
		})
	}
}

func TestAxesOutsideTheirRangeDecline(t *testing.T) {
	for _, tc := range []struct {
		name   string
		format Format
		token  string
	}{
		{"dms latitude past 90", FormatDMS, "990000N0790000W"},
		{"dms longitude past 180", FormatDMS, "350000N9990000W"},
		{"ddm latitude past 90", FormatDDM, "9900.000N07901.000W"},
		{"ddm longitude past 180", FormatDDM, "3510.000N99901.000W"},
		{"latd latitude past 90", FormatLATD, "99N079W"},
		{"latd longitude past 180", FormatLATD, "35N999W"},
		{"latm latitude past 90", FormatLATM, "9910N07901W"},
		{"latm longitude past 180", FormatLATM, "3510N99901W"},
		{"vlatm latitude past 90", FormatVLATM, "9910N9-07901W7"},
		{"vlatm longitude past 180", FormatVLATM, "3510N9-99901W7"},
		{"utm zone past 60", FormatUTM, "99S3234784306483"},
		{"utm zone zero", FormatUTM, "00S3234784306483"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if loc, ok := Parse(tc.format, tc.token); ok {
				t.Errorf("Parse(%s, %q) = %v, want a refusal", tc.format, tc.token, loc)
			}
		})
	}
}

func TestHemiAxisRefusesAnEmptyHemisphere(t *testing.T) {
	if a, ok := hemiAxis("", "35", "10", "", "", FracNone, maxLatDeg); ok {
		t.Errorf("hemiAxis with no hemisphere = %v, want a refusal", a)
	}
}
