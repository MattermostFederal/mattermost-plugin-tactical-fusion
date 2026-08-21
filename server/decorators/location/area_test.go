package location

import (
	"math"
	"math/rand"
	"testing"
)

// The decoders are the second layer of validation, and every one of these
// refusals is unreachable through Parse: the anchored grammar rejects the token
// first. They are tested by calling the decoder directly, past the regex, for
// exactly that reason. A guard that has never run is a guess, and this is the
// layer that has to hold the day the grammar is widened, which it already has
// been once per phase.
//
// decodeArea is the reachable half. Location and Area are exported with
// exported fields and no constructor, so a hand-built one reaches it without
// passing a pattern at all.
func TestAreaDecodersRefuseWhatTheGrammarWouldHaveCaught(t *testing.T) {
	t.Run("georef", func(t *testing.T) {
		for _, tc := range []struct {
			name string
			code string
		}{
			{"shorter than the four letters", "GJN"},
			{"empty", ""},
			{"an I in the longitude zone", "IJNJ5753"},
			{"a latitude band past M", "GNNJ5753"},
			{"a longitude unit past Q", "GRNJ5753"},
			{"a latitude unit past Q", "GJNR5753"},
			{"an odd number of digits", "GJNJ575"},
			{"two digits, one per axis", "GJNJ57"},
			{"ten digits, five per axis", "GJNJ5753375312"},
			{"longitude minutes past 59", "GJNJ6053"},
			{"latitude minutes past 59", "GJNJ5760"},
		} {
			t.Run(tc.name, func(t *testing.T) {
				if _, ok := georefCell(tc.code); ok {
					t.Errorf("georefCell(%q) accepted a code that names nothing", tc.code)
				}
			})
		}

		// And the shape the grammar declines but the decoder must still read,
		// because a derived row renders it for a whole-degree coordinate.
		if _, ok := georefCell("GJMF"); !ok {
			t.Error("georefCell rejected the bare quadrangle, which GEOREFText emits")
		}
	})

	t.Run("gars", func(t *testing.T) {
		for _, tc := range []struct {
			name string
			code string
		}{
			{"shorter than a band and a letter pair", "206L"},
			{"longer than the keypad allows", "206LT263"},
			{"a letter where the band digits go", "20ALT"},
			{"a leading space", " 06LT"},
			{"band 000", "000LT"},
			{"band past 720", "721LT"},
			{"an I in the first letter", "206IT"},
			{"an I in the second letter", "206LI"},
			{"a letter pair past the 360 bands that exist", "206ZZ"},
			{"quadrant 0", "206LT0"},
			{"quadrant past 4", "206LT5"},
			{"keypad 0", "206LT20"},
		} {
			t.Run(tc.name, func(t *testing.T) {
				if _, ok := garsCell(tc.code); ok {
					t.Errorf("garsCell(%q) accepted a code that names nothing", tc.code)
				}
			})
		}
	})

	t.Run("pluscode", func(t *testing.T) {
		for _, tc := range []struct {
			name string
			code string
		}{
			{"no separator", "849VCWC8R9"},
			{"two separators", "849VCWC+8+R9"},
			{"the separator in the wrong place", "849VCW+C8R9"},
			{"a character outside the alphabet", "849VCWA8+R9"},
			{"a grid character outside the alphabet", "849VCWC8+RA"},
			{"nine significant characters", "849VCWC8+R"},
			{"sixteen significant characters", "849VCWC8+R9CVWXQ2"},
			{"an odd run of padding", "849VC000+"},
			{"padding with detail after the separator", "849V0000+R9"},
			{"two significant characters", "84000000+"},
			{"a latitude pair off the top of the world", "F49VCWC8+R9"},
			{"a longitude pair past the antimeridian", "8X9VCWC8+R9"},
		} {
			t.Run(tc.name, func(t *testing.T) {
				if _, ok := olcCell(tc.code); ok {
					t.Errorf("olcCell(%q) accepted a code that names nothing", tc.code)
				}
			})
		}
	})

	t.Run("decodeArea", func(t *testing.T) {
		// The branch a hand-built Location reaches, since nothing outside this
		// package has to go through Parse to build one.
		for _, tc := range []struct {
			name string
			area Area
		}{
			{"a format that keeps no Area", Area{Format: FormatMGRS, Code: "18SUJ2347806483"}},
			{"the zero value", Area{}},
			{"an area format carrying a code of another", Area{Format: FormatGARS, Code: "GJNJ5753"}},
		} {
			t.Run(tc.name, func(t *testing.T) {
				if _, ok := decodeArea(tc.area); ok {
					t.Errorf("decodeArea(%+v) accepted an area it cannot read", tc.area)
				}

				// And the two derived values must refuse rather than invent one.
				if _, _, ok := areaCenter(tc.area); ok {
					t.Error("areaCenter reported a position for an unreadable area")
				}
				if got := areaResolutionDegrees(tc.area); got != 1 {
					t.Errorf("areaResolutionDegrees = %v for an unreadable area, want the "+
						"coarsest figure this package has rather than a zero", got)
				}
			})
		}
	})
}

// clampOLCLength normalizes a length no caller passes today, since
// plusCodeLengthFor only ever returns one off the ladder. It is tested because
// the cost of it being wrong is an index panic inside MessageWillBePosted,
// where the hook's recover would turn it into decoration silently dying across
// the workspace rather than into a failure anybody sees.
func TestEveryPlusCodeLengthProducesAReadableCode(t *testing.T) {
	const (
		lat = 37.4220625
		lon = -122.0840625
	)

	for length := -3; length <= 20; length++ {
		code := plusCodeAt(lat, lon, length)

		back, ok := Parse(FormatPlusCode, code)
		if !ok {
			t.Fatalf("plusCodeAt(length %d) rendered %q, which this package will not read back",
				length, code)
		}

		cell, ok := decodeArea(back.Area)
		if !ok {
			t.Fatalf("decodeArea failed for %q", code)
		}
		if lat < cell.Lat || lat >= cell.Lat+cell.LatSize ||
			lon < cell.Lon || lon >= cell.Lon+cell.LonSize {
			t.Errorf("plusCodeAt(length %d) = %q, which does not contain the position", length, code)
		}
	}

	// The clamp itself, stated directly: below the shortest length, above the
	// longest, and the odd values the notation has no shape for.
	for _, tc := range []struct {
		in, want int
	}{
		{-1, 4},
		{0, 4},
		{3, 4},
		{4, 4},
		{5, 4},
		{7, 6},
		{9, 8},
		{10, 10},
		{15, 15},
		{16, 15},
		{99, 15},
	} {
		if got := clampOLCLength(tc.in); got != tc.want {
			t.Errorf("clampOLCLength(%d) = %d, want %d", tc.in, got, tc.want)
		}
	}
}

func TestAreaCodesReportAPositionInsideTheirOwnCell(t *testing.T) {
	for _, tc := range acceptedTokens {
		switch tc.format {
		case FormatGEOREF, FormatGARS, FormatPlusCode:
		default:
			continue
		}

		t.Run(tc.canonical, func(t *testing.T) {
			loc, ok := Parse(tc.format, tc.canonical)
			if !ok {
				t.Fatalf("Parse(%s, %q) rejected a valid token", tc.format, tc.canonical)
			}

			cell, ok := decodeArea(loc.Area)
			if !ok {
				t.Fatal("decodeArea failed for a token Parse had just accepted")
			}

			lat, lon, ok := loc.Point()
			if !ok {
				t.Fatal("Point() failed for a token Parse had just accepted")
			}

			if lat < cell.Lat || lat >= cell.Lat+cell.LatSize {
				t.Errorf("latitude %v is outside [%v, %v)", lat, cell.Lat, cell.Lat+cell.LatSize)
			}
			if lon < cell.Lon || lon >= cell.Lon+cell.LonSize {
				t.Errorf("longitude %v is outside [%v, %v)", lon, cell.Lon, cell.Lon+cell.LonSize)
			}
		})
	}
}

func TestGEOREFIsLongitudeFirst(t *testing.T) {
	loc, ok := Parse(FormatGEOREF, "GJNJ5753")
	if !ok {
		t.Fatal("Parse rejected a valid GEOREF code")
	}

	lat, lon, ok := loc.Point()
	if !ok {
		t.Fatal("Point() failed")
	}

	const (
		zoneWest, zoneEast   = -78.0, -77.0
		bandSouth, bandNorth = 38.0, 39.0

		wantLat = 38.8916667
		wantLon = -77.0416667
	)

	if lon > zoneEast || lon < zoneWest {
		t.Errorf("longitude = %v, want between %v and %v", lon, zoneWest, zoneEast)
	}
	if lat < bandSouth || lat > bandNorth {
		t.Errorf("latitude = %v, want between %v and %v", lat, bandSouth, bandNorth)
	}

	if math.Abs(lon-wantLon) > 1e-6 || math.Abs(lat-wantLat) > 1e-6 {
		t.Errorf("position = %v, %v, want %v, %v", lat, lon, wantLat, wantLon)
	}
}

func TestAreaRowsAreTokensThisPackageAccepts(t *testing.T) {
	rng := rand.New(rand.NewSource(20260814)) // #nosec G404 -- test corpus, not security

	const (
		samples         = 3000
		latSpan, latMin = 178.0, -89.0
		lonSpan, lonMin = 358.0, -179.0
	)

	// Driven off the ladders themselves rather than off a corpus of source
	// tokens. Every dd token carries at least four decimals, so a sweep built
	// from those lands on the finest rung of all three ladders every time,
	// which is a universal claim measured against inputs chosen to satisfy it.
	// That is how a transposed Plus Code grid survived a passing round trip.
	//
	// The coarsest GEOREF rung is excluded and pinned separately by
	// TestTheCoarsestGEOREFRowIsNotAnInputToken: four letters is a word, so
	// that one shape is deliberately not an input token.
	for range samples {
		lat := rng.Float64()*latSpan + latMin
		lon := rng.Float64()*lonSpan + lonMin

		for _, step := range georefLadder {
			if step.value == 0 {
				continue
			}
			assertCellContains(t, FormatGEOREF, georefAt(lat, lon, step.value), lat, lon)
		}
		for _, step := range garsLadder {
			assertCellContains(t, FormatGARS, garsAt(lat, lon, step.value), lat, lon)
		}
		for _, step := range olcLadder {
			assertCellContains(t, FormatPlusCode, plusCodeAt(lat, lon, step.value), lat, lon)
		}
	}
}

func assertCellContains(t *testing.T, f Format, code string, lat, lon float64) {
	t.Helper()

	if code == "" {
		t.Fatalf("%s rendered nothing for %v, %v", f, lat, lon)
	}

	back, ok := Parse(f, code)
	if !ok {
		t.Fatalf("%s rendered %q, which this package will not read back", f, code)
	}

	cell, ok := decodeArea(back.Area)
	if !ok {
		t.Fatalf("decodeArea failed for %q", code)
	}

	// A hair of slack on each edge, because the decoded bound is rebuilt by a
	// division and a subtraction and lands within an ulp of the value the
	// encoder was given: a source of 84.3 decodes its own cell as starting at
	// 84.30000000000001. The defect this guards against is a whole cell wide.
	// Absolute rather than relative to the cell: the error is one ulp at the
	// coordinate's own magnitude, about 1.4e-14 near 84 degrees, and does not
	// shrink with the cell. A micron of slack sits a thousand times above that
	// and a thousandth below the finest cell this notation has.
	const edgeSlack = 1e-11

	if lat < cell.Lat-edgeSlack || lat >= cell.Lat+cell.LatSize+edgeSlack {
		t.Fatalf("%q does not contain latitude %v; cell is [%v, %v)",
			code, lat, cell.Lat, cell.Lat+cell.LatSize)
	}
	if lon < cell.Lon-edgeSlack || lon >= cell.Lon+cell.LonSize+edgeSlack {
		t.Fatalf("%q does not contain longitude %v; cell is [%v, %v)",
			code, lon, cell.Lon, cell.Lon+cell.LonSize)
	}
}

func TestRoundCoordinatesLandInTheCellTheyAreOn(t *testing.T) {
	rng := rand.New(rand.NewSource(20260815)) // #nosec G404 -- test corpus, not security

	const samples = 2000

	// Positions that sit exactly ON a cell boundary, which is where a bare
	// Floor puts the point one cell south or west: the product lands an ulp
	// low, so 35 degrees 10 minutes becomes minute 09. Whole minutes, whole
	// seconds and round decimals are nearly everything this plugin decorates,
	// so this is the common case rather than an edge of it.
	for range samples {
		for _, source := range []struct {
			lat, lon float64
		}{
			{float64(rng.Intn(170)-85) + float64(rng.Intn(60))/60, float64(rng.Intn(350) - 175)},
			{float64(rng.Intn(170)-85) + float64(rng.Intn(3600))/3600, float64(rng.Intn(350) - 175)},
			{float64(rng.Intn(17000)-8500) / 100, float64(rng.Intn(35000)-17500) / 100},
			{float64(rng.Intn(170)-85) + float64(rng.Intn(12))/12, float64(rng.Intn(350) - 175)},
		} {
			for _, step := range georefLadder {
				if step.value == 0 {
					continue
				}
				assertCellContains(t, FormatGEOREF,
					georefAt(source.lat, source.lon, step.value), source.lat, source.lon)
			}
			for _, step := range garsLadder {
				assertCellContains(t, FormatGARS,
					garsAt(source.lat, source.lon, step.value), source.lat, source.lon)
			}
			for _, step := range olcLadder {
				assertCellContains(t, FormatPlusCode,
					plusCodeAt(source.lat, source.lon, step.value), source.lat, source.lon)
			}
		}
	}
}

func TestEveryRowAgreesAboutTheMinute(t *testing.T) {
	loc, ok := Parse(FormatLATM, "3510N07901W")
	if !ok {
		t.Fatal("Parse rejected a valid whole-minute token")
	}

	const (
		wantDMS    = `35°10'N 79°01'W`
		wantUSMTF  = "3510N07901W"
		wantGEOREF = "GJLF5910"
	)

	if got := loc.DMSText(); got != wantDMS {
		t.Errorf("DMSText() = %q, want %q", got, wantDMS)
	}
	if got := loc.USMTFText(); got != wantUSMTF {
		t.Errorf("USMTFText() = %q, want %q", got, wantUSMTF)
	}
	if got := loc.GEOREFText(); got != wantGEOREF {
		t.Errorf("GEOREFText() = %q, want %q; the GEOREF row disagreed with the DMS and "+
			"USMTF rows about the minute on the same page", got, wantGEOREF)
	}
}

func TestTheCoarsestGEOREFRowIsNotAnInputToken(t *testing.T) {
	loc, ok := Parse(FormatLATD, "35N079W")
	if !ok {
		t.Fatal("Parse rejected a valid whole-degree token")
	}

	const coarsest = "GJMF"

	if got := loc.GEOREFText(); got != coarsest {
		t.Fatalf("GEOREFText() = %q, want %q", got, coarsest)
	}
	if _, ok := Parse(FormatGEOREF, coarsest); ok {
		t.Errorf("Parse accepted %q; four letters is a word, and this shape is "+
			"reachable as a derived row only", coarsest)
	}

	for _, tc := range []struct {
		format Format
		code   string
	}{
		{FormatGARS, loc.GARSText()},
		{FormatPlusCode, loc.PlusCodeText()},
	} {
		if _, ok := Parse(tc.format, tc.code); !ok {
			t.Errorf("the coarsest %s row %q does not read back", tc.format, tc.code)
		}
	}
}

func TestPlusCodeGridRefinementIsFiveRowsByFourColumns(t *testing.T) {
	const (
		gridRows    = 5
		gridColumns = 4

		pairPrecision     = 8000
		wantLatPrecision  = pairPrecision * gridRows * gridRows * gridRows * gridRows * gridRows
		wantLonPrecision  = pairPrecision * gridColumns * gridColumns * gridColumns * gridColumns * gridColumns
		longestCodeLength = 15
	)

	loc, ok := Parse(FormatPlusCode, "849VCWC8+R9CVWXQ")
	if !ok {
		t.Fatal("Parse rejected a fifteen-character Plus Code")
	}

	cell, ok := decodeArea(loc.Area)
	if !ok {
		t.Fatal("decodeArea failed for a token Parse had just accepted")
	}

	if got := math.Round(1 / cell.LatSize); got != wantLatPrecision {
		t.Errorf("latitude precision at %d characters = %.0f, want %d",
			longestCodeLength, got, wantLatPrecision)
	}
	if got := math.Round(1 / cell.LonSize); got != wantLonPrecision {
		t.Errorf("longitude precision at %d characters = %.0f, want %d",
			longestCodeLength, got, wantLonPrecision)
	}

	lat, lon, ok := loc.Point()
	if !ok {
		t.Fatal("Point() failed")
	}
	if lat < -90 || lat > 90 || lon < -180 || lon > 180 {
		t.Errorf("position %v, %v is off the world", lat, lon)
	}
}

func TestPlusCodeMatchesPublishedVectors(t *testing.T) {
	for _, tc := range []struct {
		code     string
		lat, lon float64
	}{
		{"8FVC2222+22", 47.0000625, 8.0000625},
		{"849VCWC8+R9", 37.4220625, -122.0840625},
		{"849VCWC8+R9C", 37.4220625, -122.0841094},
	} {
		t.Run(tc.code, func(t *testing.T) {
			loc, ok := Parse(FormatPlusCode, tc.code)
			if !ok {
				t.Fatalf("Parse rejected %q", tc.code)
			}

			lat, lon, ok := loc.Point()
			if !ok {
				t.Fatal("Point() failed")
			}

			if math.Abs(lat-tc.lat) > 1e-6 || math.Abs(lon-tc.lon) > 1e-6 {
				t.Errorf("center = %.7f, %.7f, want %.7f, %.7f", lat, lon, tc.lat, tc.lon)
			}
		})
	}
}

func TestAreaEncodersTruncateRatherThanRound(t *testing.T) {
	const intoTheCell = 0.9

	for _, tc := range []struct {
		format Format
		code   string
		encode func(lat, lon float64) string
	}{
		{FormatGEOREF, "GJNJ5753", func(lat, lon float64) string { return georefAt(lat, lon, 2) }},
		{FormatGARS, "206LT26", func(lat, lon float64) string { return garsAt(lat, lon, 5) }},
		{FormatPlusCode, "849VCWC8+", func(lat, lon float64) string { return plusCodeAt(lat, lon, 8) }},
	} {
		t.Run(tc.code, func(t *testing.T) {
			loc, ok := Parse(tc.format, tc.code)
			if !ok {
				t.Fatalf("Parse(%s, %q) rejected a valid token", tc.format, tc.code)
			}

			cell, ok := decodeArea(loc.Area)
			if !ok {
				t.Fatal("decodeArea failed for a token Parse had just accepted")
			}

			lat := cell.Lat + intoTheCell*cell.LatSize
			lon := cell.Lon + intoTheCell*cell.LonSize

			if got := tc.encode(lat, lon); got != tc.code {
				t.Errorf("a position in the upper corner of %q encoded as %q", tc.code, got)
			}
		})
	}
}

func TestAreaRowsKeepTheTokensResolution(t *testing.T) {
	for _, tc := range []struct {
		format    Format
		canonical string
		georef    string
		gars      string
		pluscode  string
	}{
		{FormatLATD, "35N079W", "GJMF", "203LL", "87730000+"},
		{FormatLATM, "3510N07901W", "GJLF5910", "202LL43", "87725X8M+"},
		{FormatDD, "34.0561,-118.2500", "EJBE45000336", "124LJ47", "85633Q42+C2R"},
		{FormatMGRS, "18SUJ2347806483", "GJNJ57885337", "206LT26", "87C4VXQ7+RV44"},
	} {
		t.Run(tc.canonical, func(t *testing.T) {
			loc, ok := Parse(tc.format, tc.canonical)
			if !ok {
				t.Fatalf("Parse(%s, %q) rejected a valid token", tc.format, tc.canonical)
			}

			if got := loc.GEOREFText(); got != tc.georef {
				t.Errorf("GEOREFText() = %q, want %q", got, tc.georef)
			}
			if got := loc.GARSText(); got != tc.gars {
				t.Errorf("GARSText() = %q, want %q", got, tc.gars)
			}
			if got := loc.PlusCodeText(); got != tc.pluscode {
				t.Errorf("PlusCodeText() = %q, want %q", got, tc.pluscode)
			}
		})
	}
}

func TestAreaRowsRenderWhereTheGridRowsCannot(t *testing.T) {
	for _, canonical := range []string{"85.1234,10.0000", "-85.1234,-10.0000"} {
		t.Run(canonical, func(t *testing.T) {
			loc, ok := Parse(FormatDD, canonical)
			if !ok {
				t.Fatalf("Parse rejected %q", canonical)
			}

			if loc.MGRSText() != "" || loc.UTMText() != "" {
				t.Fatal("this coordinate is supposed to be outside the UTM latitudes")
			}

			for _, row := range []struct {
				name  string
				value string
			}{
				{"GEOREF", loc.GEOREFText()},
				{"GARS", loc.GARSText()},
				{"Plus Code", loc.PlusCodeText()},
			} {
				if row.value == "" {
					t.Errorf("the %s row is empty at a polar latitude", row.name)
				}
			}
		})
	}
}

func TestAreaEncodersSurviveTheCorners(t *testing.T) {
	for _, corner := range []struct{ lat, lon float64 }{
		{90, 180}, {90, -180}, {-90, 180}, {-90, -180}, {0, 0},
	} {
		for _, got := range []string{
			georefAt(corner.lat, corner.lon, 4),
			garsAt(corner.lat, corner.lon, 5),
			plusCodeAt(corner.lat, corner.lon, 11),
		} {
			if got == "" {
				t.Errorf("empty code at %v, %v", corner.lat, corner.lon)
			}
		}
	}
}

func TestAreaLaddersPickTheCoarsestCellThatFits(t *testing.T) {
	for _, step := range georefLadder {
		if got := georefDigitsFor(step.size); got != step.value {
			t.Errorf("georefDigitsFor(%v) = %d, want %d", step.size, got, step.value)
		}
	}
	for _, step := range garsLadder {
		if got := garsMinutesFor(step.size); got != step.value {
			t.Errorf("garsMinutesFor(%v) = %d, want %d", step.size, got, step.value)
		}
	}
	for _, step := range olcLadder {
		if got := plusCodeLengthFor(step.size); got != step.value {
			t.Errorf("plusCodeLengthFor(%v) = %d, want %d", step.size, got, step.value)
		}
	}

	const finerThanTheFinestStep = 1e-12

	if got := plusCodeLengthFor(finerThanTheFinestStep); got != 15 {
		t.Errorf("plusCodeLengthFor(%v) = %d, want 15", finerThanTheFinestStep, got)
	}
	if got := georefDigitsFor(finerThanTheFinestStep); got != 4 {
		t.Errorf("georefDigitsFor(%v) = %d, want 4", finerThanTheFinestStep, got)
	}
}

func TestPaddedPlusCodesAreTheCoarseOnes(t *testing.T) {
	for _, tc := range []struct {
		code  string
		size  float64
		reads bool
	}{
		{"849V0000+", 1, true},
		{"849VCW00+", 0.05, true},
		{"849VCWC8+", 0.0025, true},
		{"84000000+", 0, false},
		{"849VC000+", 0, false},
	} {
		t.Run(tc.code, func(t *testing.T) {
			loc, ok := Parse(FormatPlusCode, tc.code)
			if ok != tc.reads {
				t.Fatalf("Parse(pluscode, %q) = %v, want %v", tc.code, ok, tc.reads)
			}
			if !tc.reads {
				return
			}

			if got := loc.resolutionDegrees(); math.Abs(got-tc.size) > 1e-12 {
				t.Errorf("resolution = %v degrees, want %v", got, tc.size)
			}
		})
	}
}

func TestAPlusCodeGridCharacterOutsideTheAlphabetRefuses(t *testing.T) {
	for _, code := range []string{"8FVC2222+22A", "8FVC2222+22E", "8FVC2222+22ZZI"} {
		if cell, ok := decodeArea(Area{Format: FormatPlusCode, Code: code}); ok {
			t.Errorf("decodeArea(%q) = %v, want a refusal", code, cell)
		}
	}
}
