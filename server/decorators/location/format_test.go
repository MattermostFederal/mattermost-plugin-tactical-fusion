package location

import (
	"fmt"
	"math"
	"math/rand"
	"strings"
	"testing"
	"time"
)

// renderFixtures is the shared expectation table.
//
// The page is Go and the panel is TypeScript, so they cannot share a render
// function. What keeps them from drifting is this table: the same inputs and
// the same expected strings are asserted here and in
// webapp/src/decorators/location/format.spec.ts. Change one and change the
// other, or the sidebar and the standalone page will describe the same
// coordinate two different ways.
var renderFixtures = []struct {
	format     Format
	canonical  string
	decimal    string
	dms        string
	ddm        string
	usmtf      string
	resolution string
}{
	{
		format:     FormatDD,
		canonical:  "34.0561,-118.2500",
		decimal:    "34.0561° N, 118.2500° W",
		dms:        `34°03'22.0"N 118°15'00.0"W`,
		ddm:        "34°03.366'N 118°15.000'W",
		usmtf:      "340322.0N1181500.0W",
		resolution: "about 11 m",
	},
	{
		format:     FormatLATD,
		canonical:  "35N079W",
		decimal:    "35° N, 79° W",
		dms:        "35°N 79°W",
		ddm:        "35°N 79°W",
		usmtf:      "35N079W",
		resolution: "about 111.3 km",
	},
	{
		format:     FormatLATM,
		canonical:  "3510N07901W",
		decimal:    "35.17° N, 79.02° W",
		dms:        `35°10'N 79°01'W`,
		ddm:        `35°10'N 79°01'W`,
		usmtf:      "3510N07901W",
		resolution: "about 1.9 km",
	},
	{
		format:     FormatDMS,
		canonical:  "340322N1181500W",
		decimal:    "34.0561° N, 118.2500° W",
		dms:        `34°03'22"N 118°15'00"W`,
		ddm:        "34°03.37'N 118°15.00'W",
		usmtf:      "340322N1181500W",
		resolution: "about 31 m",
	},

	// The halves need not have been written to the same precision, and each is
	// rendered at its OWN. Rendering the fine half at the coarse one's count
	// moved it 4.9 km, which is the whole reason these two rows are here.
	//
	// The resolution stays the COARSER half's, because that is how well the
	// pair is known. A value row and the resolution row disagreeing about how
	// many digits to show is the point rather than a defect: "34.0561° N" is
	// what the author wrote and "about 11.1 km" is what the pair is worth.
	{
		format:     FormatDDH,
		canonical:  "34.0561N,118.2W",
		decimal:    "34.0561° N, 118.2° W",
		dms:        `34°03'22.0"N 118°12'W`,
		ddm:        "34°03.366'N 118°12'W",
		usmtf:      "3403N11812W",
		resolution: "about 11.1 km",
	},
	{
		format:     FormatDMS,
		canonical:  "340322.5N1181500W",
		decimal:    "34.05625° N, 118.2500° W",
		dms:        `34°03'22.5"N 118°15'00"W`,
		ddm:        "34°03.375'N 118°15.00'W",
		usmtf:      "340322N1181500W",
		resolution: "about 31 m",
	},
}

// gridFixtures is the same idea for the two grid grammars, kept apart because
// the TypeScript half can assert far less about them.
//
// The panel has no projection, so it computes only the grid reference itself
// and the resolution and takes the rest from the conversion endpoint. Those two
// are what webapp/src/decorators/location/format.spec.ts holds to this table;
// the position rows are asserted in Go alone, because in the running plugin
// they are produced in Go alone.
var gridFixtures = []struct {
	format     Format
	canonical  string
	grid       string
	resolution string

	// Asserted in Go only.
	decimal string
	mgrs    string
	utm     string
	usmtf   string
}{
	{
		format:     FormatMGRS,
		canonical:  "18SUJ2347806483",
		grid:       "18S UJ 23478 06483",
		resolution: "1 m grid, at center",
		decimal:    "38.889502° N, 77.035295° W",
		mgrs:       "18S UJ 23478 06483",
		utm:        "18S 323478E 4306483N",
		usmtf:      "385322.21N0770207.06W",
	},
	{
		format:     FormatMGRS,
		canonical:  "32UMV1234",
		grid:       "32U MV 12 34",
		resolution: "1 km grid, at center",
		decimal:    "49.057° N, 7.802° E",

		// The UTM row is the same numbers as the grid reference, because the
		// digits of a grid reference are its south-west corner on the UTM grid.
		// Quoting the square's center here instead would put the easting at
		// 412500 against a square that starts at 412000.
		mgrs:  "32U MV 12 34",
		utm:   "32U 412000E 5434000N",
		usmtf: "490326N0074808E",
	},
	{
		// The same position as the MGRS fixture above, written as UTM, and the
		// link that found the webapp reading band S with an older, narrower
		// class of its own. Paired with a row in format.spec.ts.
		format:     FormatUTM,
		canonical:  "18S3234784306483",
		grid:       "18S 323478E 4306483N",
		resolution: "1 m",
		decimal:    "38.889498° N, 77.035301° W",
		mgrs:       "18S UJ 23478 06483",
		utm:        "18S 323478E 4306483N",
		usmtf:      "385322.19N0770207.08W",
	},
	{
		format:     FormatUTM,
		canonical:  "33U2910005628000",
		grid:       "33U 291000E 5628000N",
		resolution: "1 m",
		decimal:    "50.766067° N, 12.036116° E",
		mgrs:       "33U TS 91000 28000",
		utm:        "33U 291000E 5628000N",
		usmtf:      "504557.84N0120210.02E",
	},
}

func TestGridRenderingMatchesTheSharedFixtures(t *testing.T) {
	for _, tc := range gridFixtures {
		t.Run(tc.canonical, func(t *testing.T) {
			loc, ok := Parse(tc.format, tc.canonical)
			if !ok {
				t.Fatalf("Parse(%s, %q) rejected its own canonical form", tc.format, tc.canonical)
			}

			if got := gridText(loc.Grid); got != tc.grid {
				t.Errorf("gridText() = %q, want %q", got, tc.grid)
			}
			if got := loc.ResolutionText(); got != tc.resolution {
				t.Errorf("ResolutionText() = %q, want %q", got, tc.resolution)
			}
			if got := loc.DecimalText(); got != tc.decimal {
				t.Errorf("DecimalText() = %q, want %q", got, tc.decimal)
			}
			if got := loc.MGRSText(); got != tc.mgrs {
				t.Errorf("MGRSText() = %q, want %q", got, tc.mgrs)
			}
			if got := loc.USMTFText(); got != tc.usmtf {
				t.Errorf("USMTFText() = %q, want %q", got, tc.usmtf)
			}
			if got := loc.UTMText(); got != tc.utm {
				t.Errorf("UTMText() = %q, want %q", got, tc.utm)
			}
		})
	}
}

// The grid rows a textual coordinate earns, at its own resolution and no finer.
//
// This is where the honesty rule is easiest to break: every conversion tool in
// existence hands back a ten-figure grid reference whatever it was given, and a
// 111 km token rendered as a one meter square is a claim about a position
// nobody wrote down.
func TestDerivedGridRowsKeepTheTokensResolution(t *testing.T) {
	for _, tc := range []struct {
		format    Format
		canonical string
		mgrs      string
		utm       string
	}{
		// Four decimals is about 11 m, so a 10 m square.
		{FormatDD, "34.0561,-118.2500", "11S LT 8463 6908", "11S 384640E 3769080N"},

		// Whole degrees: a 100 km square, and no digits at all after the
		// letters.
		{FormatLATD, "35N079W", "17S PU", "17S 700000E 3900000N"},

		// Whole minutes is about 1.9 km, so a 1 km square.
		{FormatLATM, "3510N07901W", "17S PU 80 93", "17S 681000E 3893000N"},

		// Whole seconds is about 31 m, so a 10 m square.
		{FormatDMS, "340322N1181500W", "11S LT 8463 6908", "11S 384640E 3769080N"},
	} {
		t.Run(tc.canonical, func(t *testing.T) {
			loc, ok := Parse(tc.format, tc.canonical)
			if !ok {
				t.Fatalf("Parse(%s, %q) rejected a valid token", tc.format, tc.canonical)
			}

			if got := loc.MGRSText(); got != tc.mgrs {
				t.Errorf("MGRSText() = %q, want %q", got, tc.mgrs)
			}
			if got := loc.UTMText(); got != tc.utm {
				t.Errorf("UTMText() = %q, want %q", got, tc.utm)
			}
		})
	}
}

// Outside the latitudes UTM covers there is no grid reference to give, and the
// rows are left out rather than filled with something approximate.
func TestPolarCoordinatesHaveNoGridRows(t *testing.T) {
	loc, ok := Parse(FormatDD, "85.1234,10.0000")
	if !ok {
		t.Fatal("Parse rejected a valid coordinate")
	}

	if got := loc.MGRSText(); got != "" {
		t.Errorf("MGRSText() = %q for a polar coordinate, want empty", got)
	}
	if got := loc.UTMText(); got != "" {
		t.Errorf("UTMText() = %q for a polar coordinate, want empty", got)
	}

	// The rows that do not need a projection are unaffected.
	if got := loc.DecimalText(); got != "85.1234° N, 10.0000° E" {
		t.Errorf("DecimalText() = %q", got)
	}
}

func TestRenderingMatchesTheSharedFixtures(t *testing.T) {
	for _, tc := range renderFixtures {
		t.Run(tc.canonical, func(t *testing.T) {
			loc, ok := Parse(tc.format, tc.canonical)
			if !ok {
				t.Fatalf("Parse(%s, %q) rejected its own canonical form", tc.format, tc.canonical)
			}

			if got := loc.DecimalText(); got != tc.decimal {
				t.Errorf("DecimalText() = %q, want %q", got, tc.decimal)
			}
			if got := loc.DMSText(); got != tc.dms {
				t.Errorf("DMSText() = %q, want %q", got, tc.dms)
			}
			if got := loc.DDMText(); got != tc.ddm {
				t.Errorf("DDMText() = %q, want %q", got, tc.ddm)
			}
			if got := loc.USMTFText(); got != tc.usmtf {
				t.Errorf("USMTFText() = %q, want %q", got, tc.usmtf)
			}
			if got := loc.ResolutionText(); got != tc.resolution {
				t.Errorf("ResolutionText() = %q, want %q", got, tc.resolution)
			}
		})
	}
}

// Rounding, not truncation.
//
// The reference implementation in mattermost-plugin-aocanywhere truncates
// minutes when it converts a decimal back, which biases every result up to a
// whole minute south and west. 34.0561 degrees is 34°03'21.96", so a rendering
// that truncates shows 21.9 where this must show 22.0.
func TestRenderingRoundsRatherThanTruncating(t *testing.T) {
	loc, ok := Parse(FormatDD, "34.0561,-118.2500")
	if !ok {
		t.Fatal("Parse rejected a valid token")
	}

	const want = `34°03'22.0"N 118°15'00.0"W`
	if got := loc.DMSText(); got != want {
		t.Fatalf("DMSText() = %q, want %q", got, want)
	}
}

// Padding a field the token never carried would be a claim about a second the
// author did not write.
func TestCoarseTokensDoNotGrowFields(t *testing.T) {
	loc, ok := Parse(FormatLATD, "35N079W")
	if !ok {
		t.Fatal("Parse rejected a valid token")
	}

	if got := loc.DMSText(); got != "35°N 79°W" {
		t.Fatalf("DMSText() = %q, want degrees only", got)
	}
}

func TestConfidenceIsSeparateFromResolution(t *testing.T) {
	loc, ok := Parse(FormatVLATM, "3510N9-07901W7")
	if !ok {
		t.Fatal("Parse rejected a valid token")
	}

	if got := loc.ResolutionText(); got != "about 1.9 km" {
		t.Errorf("ResolutionText() = %q, want the LATM resolution", got)
	}
	if got := loc.ConfidenceText(); got != "stated confidence 9 (latitude), 7 (longitude)" {
		t.Errorf("ConfidenceText() = %q", got)
	}
}

// "about 0 m" would read as a claim of infinite precision rather
// than as the very fine figure it is, and an eight-decimal token reaches it.
//
// The wording must match resolutionText in
// webapp/src/decorators/location/format.ts.
func TestVeryFineTokensDoNotClaimZeroMeters(t *testing.T) {
	for _, tc := range []struct {
		format Format
		token  string
	}{
		{FormatDD, "34.12345678,-118.12345678"},
		{FormatDMS, "340322.1234N1181500.1234W"},
		{FormatDDM, "3403.123456N11815.123456W"},
	} {
		loc, ok := Parse(tc.format, tc.token)
		if !ok {
			t.Fatalf("Parse(%s, %q) rejected a valid token", tc.format, tc.token)
		}

		const want = "finer than 0.01 m"
		if got := loc.ResolutionText(); got != want {
			t.Errorf("ResolutionText() for %q = %q, want %q", tc.token, got, want)
		}
	}
}

// The rungs between "about 11 m" and "finer than 0.01 m", which is where every
// other fixture in this file steps straight over.
//
// humanMeters has three arms and the sub-meter one had never been taken: the
// shared fixtures stop at four decimals (11 m) and the test above starts at
// eight (below a centimeter, which returns before humanMeters is called at
// all). Six decimals is not an edge case, it is what an ordinary phone emits,
// and it lands at 0.11 m squarely in the gap.
//
// Whole numbers stay whole ("1 m", not "1.00 m") because the arms are chosen by
// magnitude and trimZeroes runs on the sub-meter one.
func TestResolutionTextBelowAMeter(t *testing.T) {
	for _, tc := range []struct {
		token, want string
	}{
		{"34.05611N,118.25000W", "about 1 m"},
		{"34.056111N,118.250000W", "about 0.11 m"},
		{"34.0561111N,118.2500000W", "about 0.01 m"},
	} {
		t.Run(tc.token, func(t *testing.T) {
			loc, ok := Parse(FormatDDH, tc.token)
			if !ok {
				t.Fatalf("Parse rejected %q", tc.token)
			}

			if got := loc.ResolutionText(); got != tc.want {
				t.Fatalf("ResolutionText() = %q, want %q", got, tc.want)
			}
		})
	}
}

// The carry, which is what stops a rounded field being rendered as 60.
//
// Every rendering path here rounds rather than truncates, so a value a hair
// under a whole degree rounds its minutes up to exactly 60, and the field below
// has to be emptied and the one above incremented. Left undone, DMSText would
// render "33°60'00\"N" and the USMTF row "3360N", neither of which any grammar
// in this package accepts, so the failure would be a value a reader could paste
// into an ATO and have refused by the next tool along.
//
// Driven against the helpers rather than through a token, deliberately. A
// first-order carry is not reachable from the parser: a minute-resolution
// token's minutes field is 0-59 by construction, and a one-decimal degree token
// cannot exceed 54 minutes. The helpers are called with derived floats from
// Decimal(), which has no such bound, so this is the layer where the carry is
// both reachable and load-bearing.
func TestRoundingCarriesIntoTheNextField(t *testing.T) {
	t.Run("degMin", func(t *testing.T) {
		deg, minutes := degMin(33.999999)
		if deg != 34 || minutes != 0 {
			t.Fatalf("degMin(33.999999) = %d°%d', want 34°0'", deg, minutes)
		}
	})

	t.Run("degDecimalMin", func(t *testing.T) {
		deg, minutes := degDecimalMin(33.999999, 2)
		if deg != 34 || minutes != 0 {
			t.Fatalf("degDecimalMin(33.999999, 2) = %d°%v', want 34°0'", deg, minutes)
		}
	})

	// Both carries at once: the seconds round to 60, which carries into minutes
	// and takes those to 60 as well. The minutes arm only ever runs as this
	// second-order case, since the minutes are truncated rather than rounded.
	t.Run("degMinSec", func(t *testing.T) {
		deg, minutes, seconds := degMinSec(33.99999999, 2)
		if deg != 34 || minutes != 0 || seconds != 0 {
			t.Fatalf(`degMinSec(33.99999999, 2) = %d°%d'%v", want 34°0'0"`, deg, minutes, seconds)
		}
	})
}

// A row whose position cannot be derived is blank, never a zero.
//
// Point() documents that every caller has to handle its failure "rather than
// rendering whatever a failed conversion left behind", and nothing held them to
// it. The failure mode is not a missing row: it is "0.0000° N, 0.0000° E" with
// a copy button beside it, which is a position, and a wrong one.
//
// Grid is exported with exported fields and no constructor, so a caller really
// can hold one that names nowhere; the same reasoning as
// TestOversizedFractionDoesNotHangFormatting below. Zone 0 is outside the 1-60
// UTM zones, so gridPoint refuses it.
func TestRowsAreBlankWhenThePositionCannotBeDerived(t *testing.T) {
	for _, tc := range []struct {
		name string
		loc  Location

		// The token's OWN grid row is not among the rows that go blank: a grid
		// reference is its own answer and is rendered straight back from the
		// value, without a projection to fail. Only the derived rows are at
		// stake here, which is why this names the other grid row per case.
		otherGridRow func(Location) string
	}{
		{
			name: "an MGRS square in no zone",
			loc: Location{
				Format: FormatMGRS,
				Grid:   Grid{Format: FormatMGRS, Zone: 0, Band: 'S', Col: 'U', Row: 'J', Digits: 5},
			},
			otherGridRow: Location.UTMText,
		},
		{
			name: "a UTM position in no zone",
			loc: Location{
				Format: FormatUTM,
				Grid:   Grid{Format: FormatUTM, Zone: 0, Band: 'S', Easting: 323478, Northing: 4306483},
			},
			otherGridRow: Location.MGRSText,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, _, ok := tc.loc.Point(); ok {
				t.Fatal("Point() resolved a grid that names nowhere, so this proves nothing")
			}

			for _, row := range []struct {
				label  string
				render func(Location) string
			}{
				{"Lat / lon", Location.DecimalText},
				{"DMS", Location.DMSText},
				{"DDM", Location.DDMText},
				{"USMTF", Location.USMTFText},
				{"the derived grid row", tc.otherGridRow},
			} {
				if got := row.render(tc.loc); got != "" {
					t.Errorf("%s rendered %q from a position that cannot be derived", row.label, got)
				}
			}
		})
	}
}

// Nothing computed from a token may reach a formatter as an unbounded width.
// Axis is exported with exported fields, so a Location can be built without
// going through Parse and its digit cap.
func TestOversizedFractionDoesNotHangFormatting(t *testing.T) {
	loc := Location{
		Format: FormatDD,
		Lat:    Axis{Deg: 34, Frac: strings.Repeat("1", 400), FracUnit: FracDegrees, Conf: NoConfidence},
		Lon:    Axis{Deg: 118, Frac: strings.Repeat("1", 400), FracUnit: FracDegrees, Neg: true, Conf: NoConfidence},
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = loc.DecimalText()
		_ = loc.DMSText()
		_ = loc.DDMText()
		_ = loc.ResolutionText()
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("formatting did not return for an oversized fraction")
	}
}

// The USMTF row is a token this package would accept back.
//
// It is the only derived row whose output is also an input grammar, so it can
// be held to a standard the others cannot: whatever it renders must parse, and
// must land where it said it would. A row producing a shape nothing here
// recognizes would be a value a reader could copy into an ATO and have refused
// by the next tool along.
//
// Driven by a GENERATED CORPUS rather than by the fixture table, and that is the
// whole point of it. An earlier version ran the same assertions over the ten
// hand-picked fixtures, which made it a universal claim checked against inputs
// chosen to satisfy it: the implementation violated the property for about one
// two-decimal coordinate in 800 and the test stayed green for months of edits.
// The property is the test; the fixtures are a different test.
func TestUSMTFRowIsATokenThisPackageAccepts(t *testing.T) {
	rng := rand.New(rand.NewSource(20260812)) //nolint:gosec // deterministic on purpose, not a security context

	// Two decimals is where the negative-zero defect concentrated, so the
	// corpus is weighted onto the coarse end rather than the fine one. A finer
	// token is a strictly easier case for this property.
	for _, places := range []int{1, 2, 3, 4} {
		t.Run(fmt.Sprintf("%d decimal places", places), func(t *testing.T) {
			for range 20000 {
				lat := rng.Float64()*180 - 90
				lon := rng.Float64()*360 - 180

				token := fmt.Sprintf("%.*f%s,%.*f%s",
					places, math.Abs(lat), hemisphere(lat, "N", "S"),
					places, math.Abs(lon), hemisphere(lon, "E", "W"))

				loc, ok := Parse(FormatDDH, token)
				if !ok {
					continue
				}

				assertUSMTFRoundTrips(t, loc, token)
			}
		})
	}

	// And the fixtures, which cover the shapes a generator reaches rarely: the
	// grid grammars, and the coarse USMTF forms.
	for _, tc := range renderFixtures {
		t.Run(tc.canonical, func(t *testing.T) {
			loc, ok := Parse(tc.format, tc.canonical)
			if !ok {
				t.Fatalf("Parse(%s, %q) declined its own canonical form", tc.format, tc.canonical)
			}
			assertUSMTFRoundTrips(t, loc, tc.canonical)
		})
	}
	for _, tc := range gridFixtures {
		t.Run(tc.canonical, func(t *testing.T) {
			loc, ok := Parse(tc.format, tc.canonical)
			if !ok {
				t.Fatalf("Parse(%s, %q) declined its own canonical form", tc.format, tc.canonical)
			}
			assertUSMTFRoundTrips(t, loc, tc.canonical)
		})
	}
}

func hemisphere(v float64, pos, neg string) string {
	if math.Signbit(v) {
		return neg
	}
	return pos
}

// assertUSMTFRoundTrips holds one rendering to the property.
func assertUSMTFRoundTrips(t *testing.T, loc Location, from string) {
	t.Helper()

	usmtf := loc.USMTFText()
	if usmtf == "" {
		t.Fatalf("USMTFText() is empty for %q", from)
	}

	// LATD, LATM and the DMS family cover the whole ladder. Parse selects the
	// grammar rather than guessing at it, so each is tried in turn.
	var back Location
	var parsed bool
	for _, f := range []Format{FormatLATD, FormatLATM, FormatDMS} {
		if back, parsed = Parse(f, usmtf); parsed {
			break
		}
	}
	if !parsed {
		t.Fatalf("USMTFText() = %q from %q, which no USMTF grammar accepts", usmtf, from)
	}

	// And it names the same place, to within the resolution it was rendered at.
	// Half a step is the most a correct rounding can move a value, so anything
	// past a whole step is a conversion error rather than the rounding this row
	// is supposed to do.
	wantLat, wantLon, _ := loc.Point()
	gotLat, gotLon, _ := back.Point()

	tolerance := loc.resolutionDegrees()
	if math.Abs(gotLat-wantLat) > tolerance || math.Abs(gotLon-wantLon) > tolerance {
		t.Errorf("%q (from %q) reads back as %.6f, %.6f, want %.6f, %.6f within %g",
			usmtf, from, gotLat, gotLon, wantLat, wantLon, tolerance)
	}
}

// No rendered field may carry a sign.
//
// The sibling of the test above, and the one that names the defect directly:
// every value row is a magnitude plus a hemisphere letter, so a "-" anywhere in
// one is wrong by construction whether or not the result happens to re-parse.
// DMSText showed "0°01'-0\"N" while USMTFText showed "0001-0N", and only the
// second failed to parse, so a round-trip test alone would have left the DMS
// row broken.
func TestNoRenderedFieldCarriesASign(t *testing.T) {
	rng := rand.New(rand.NewSource(20260813)) //nolint:gosec // deterministic on purpose, not a security context

	for range 100000 {
		lat := rng.Float64()*180 - 90
		lon := rng.Float64()*360 - 180
		places := rng.Intn(4) + 1

		token := fmt.Sprintf("%.*f%s,%.*f%s",
			places, math.Abs(lat), hemisphere(lat, "N", "S"),
			places, math.Abs(lon), hemisphere(lon, "E", "W"))

		loc, ok := Parse(FormatDDH, token)
		if !ok {
			continue
		}

		for _, row := range []struct{ name, value string }{
			{"DMS", loc.DMSText()},
			{"DDM", loc.DDMText()},
			{"USMTF", loc.USMTFText()},
		} {
			if strings.Contains(row.value, "-") {
				t.Fatalf("%s row for %q rendered %q, which carries a sign", row.name, token, row.value)
			}
		}
	}
}

// The confidence digits never reach the derived row.
//
// A verified token states how well its position is KNOWN. That is a property of
// the measurement rather than of the arithmetic, so a reading derived from it
// cannot inherit the claim. The token keeps its own Confidence row.
func TestUSMTFRowNeverClaimsConfidence(t *testing.T) {
	loc, ok := Parse(FormatVLATM, "3510N9-07901W7")
	if !ok {
		t.Fatal("Parse declined a verified token")
	}

	if got := loc.USMTFText(); got != "3510N07901W" {
		t.Fatalf("USMTFText() = %q, want the plain shape with no confidence digits", got)
	}

	// The claim is still on screen, in the row that owns it.
	if loc.ConfidenceText() == "" {
		t.Error("the confidence row lost the digits the token carried")
	}
}

// The shape follows the token's resolution, which is the whole rule.
//
// Pinned separately from the fixtures because it is the decision the row is
// built on: padding LATM onto a degrees-only token, or emitting LATS from one
// written to minutes, would both be claims about fields nobody wrote.
func TestUSMTFShapeFollowsResolution(t *testing.T) {
	for _, tc := range []struct {
		name      string
		format    Format
		canonical string
		want      string
	}{
		{"whole degrees is LATD", FormatLATD, "35N079W", "35N079W"},
		{"whole minutes is LATM", FormatLATM, "3510N07901W", "3510N07901W"},
		{"whole seconds is LATS", FormatDMS, "340322N1181500W", "340322N1181500W"},
		{"finer than a second is LATDS", FormatDMS, "340322.5N1181500.5W", "340322.5N1181500.5W"},

		// A decimal token lands on the ladder by its resolution rather than by
		// its spelling: four decimals is about 11 m, which is finer than a
		// second, so it renders LATDS.
		{"four decimal degrees is LATDS", FormatDD, "34.0561,-118.2500", "340322.0N1181500.0W"},

		// And a coarse one comes out coarse rather than padded.
		{"two decimal degrees is LATS", FormatDDH, "34.05N,118.25W", "340300N1181500W"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			loc, ok := Parse(tc.format, tc.canonical)
			if !ok {
				t.Fatalf("Parse(%s, %q) declined", tc.format, tc.canonical)
			}
			if got := loc.USMTFText(); got != tc.want {
				t.Fatalf("USMTFText() = %q, want %q", got, tc.want)
			}
		})
	}
}
