package airport

import (
	"strings"
	"testing"
)

func TestEveryAirfieldIsUsable(t *testing.T) {
	if Count() < 18000 {
		t.Fatalf("only %d airfields, which is far fewer than the data should hold", Count())
	}

	for ident, a := range airfields {
		if !MatchesIdentShape(ident) {
			t.Fatalf("ident %q is not four upper-case letters", ident)
		}
		if a.Ident != ident {
			t.Fatalf("%s is keyed as %q", a.Ident, ident)
		}
		if a.Name == "" {
			t.Errorf("%s has no name", ident)
		}
	}
}

// Every airfield has to convert, or a panel opens on a table of blank rows with
// nothing saying why. This is the check that lets Details carry no "found but
// unconvertible" case in practice, and it runs over the whole file rather than
// over a sample, because one bad row is one permanently broken link.
func TestEveryAirfieldConverts(t *testing.T) {
	for ident := range airfields {
		d, ok := Describe(ident)
		if !ok {
			t.Fatalf("%s is in the data and not describable", ident)
		}
		if !d.HasPosition {
			t.Fatalf("%s does not convert to a position", ident)
		}
	}
}

// The place is what carries the link to the position on both surfaces, so an
// airfield with no place would be one a reader cannot get a position for. None
// exist today, and this is here so a refreshed database cannot introduce one
// silently.
func TestEveryAirfieldHasAPlaceToHangTheLinkOn(t *testing.T) {
	for ident := range airfields {
		d, ok := Describe(ident)
		if !ok {
			t.Fatalf("%s is in the data and not describable", ident)
		}
		if d.Place == "" {
			t.Fatalf("%s has no place, so neither surface can offer its position", ident)
		}
	}
}

// ZZZZ is a real airfield upstream and is deliberately absent, because in a
// flight plan it means "aerodrome not listed". See data/README.md.
func TestReservedIdentsAreAbsent(t *testing.T) {
	for _, ident := range []string{"ZZZZ", "AFIL"} {
		if _, ok := Lookup(ident); ok {
			t.Errorf("%s is reserved and must not resolve to an airfield", ident)
		}
	}
}

func TestLookupPins(t *testing.T) {
	tests := []struct {
		ident     string
		name      string
		place     string
		elevation string
		iata      string
	}{
		{"KIND", "Indianapolis International Airport", "Indianapolis, IN, US", "797 ft", "IND"},
		{"KLAX", "Los Angeles International Airport", "Los Angeles, CA, US", "125 ft", "LAX"},
		{"EGLL", "London Heathrow Airport", "London, ENG, GB", "83 ft", "LHR"},
		{"PHIK", "Hickam Air Force Base", "Honolulu, HI, US", "13 ft", "HIK"},
		{"USCG", "Chelyabinsk Shagol Airport", "CHE, RU", "830 ft", ""},
	}

	for _, tc := range tests {
		t.Run(tc.ident, func(t *testing.T) {
			d, ok := Describe(tc.ident)
			if !ok {
				t.Fatalf("not found")
			}
			if d.Name != tc.name {
				t.Errorf("name = %q, want %q", d.Name, tc.name)
			}
			if d.Place != tc.place {
				t.Errorf("place = %q, want %q", d.Place, tc.place)
			}
			if d.Elevation != tc.elevation {
				t.Errorf("elevation = %q, want %q", d.Elevation, tc.elevation)
			}
			if d.IATA != tc.iata {
				t.Errorf("iata = %q, want %q", d.IATA, tc.iata)
			}
		})
	}
}

// NZSP is Amundsen-Scott at latitude exactly -90.0000, which is past Mercator
// and past both grid bands. It still has to yield a usable coordinate link:
// which rows come back blank there is the location decorator's business, and
// TestAreaRowsRenderWhereTheGridRowsCannot is what covers it.
func TestThePoleStillYieldsACoordinate(t *testing.T) {
	d, ok := Describe("NZSP")
	if !ok {
		t.Fatal("NZSP is not in the data")
	}
	if !d.HasPosition {
		t.Fatal("the pole yields no coordinate, so the panel would offer no link")
	}
	if d.Token != "-90.0000,0.0000" {
		t.Errorf("token = %q, want -90.0000,0.0000", d.Token)
	}
	if d.Format != "dd" {
		t.Errorf("format = %q, want dd", d.Format)
	}
}

// The token has to be exactly what the location grammar canonicalizes to, or
// every surface links to a page that refuses it. No space after the comma.
func TestTheTokenIsTheCanonicalCoordinate(t *testing.T) {
	d, ok := Describe("KIND")
	if !ok {
		t.Fatal("KIND is not in the data")
	}
	if d.Token != "39.7173,-86.2944" {
		t.Errorf("token = %q, want 39.7173,-86.2944", d.Token)
	}
	if strings.Contains(d.Token, " ") {
		t.Error("the token carries a space, which the canonical form does not")
	}
}

// Sea level is a real elevation and an absent one is a different thing. This is
// the truthiness defect this plugin does not inherit from the sibling plugin.
func TestElevationDistinguishesSeaLevelFromAbsent(t *testing.T) {
	zero := 0
	if got := elevationText(Airport{ElevationFt: &zero}); got != "0 ft" {
		t.Errorf("sea level renders %q, want %q", got, "0 ft")
	}
	if got := elevationText(Airport{}); got != "" {
		t.Errorf("an absent elevation renders %q, want empty", got)
	}
}

func TestElevationGroupsThousands(t *testing.T) {
	for _, tc := range []struct {
		feet int
		want string
	}{{7, "7 ft"}, {797, "797 ft"}, {9300, "9,300 ft"}, {13123, "13,123 ft"}, {-210, "-210 ft"}} {
		feet := tc.feet
		if got := elevationText(Airport{ElevationFt: &feet}); got != tc.want {
			t.Errorf("%d renders %q, want %q", tc.feet, got, tc.want)
		}
	}
}

func TestTypeTextIsTitleCase(t *testing.T) {
	for raw, want := range map[string]string{
		"large_airport":  "Large Airport",
		"seaplane_base":  "Seaplane Base",
		"closed":         "Closed",
		"":               "",
		"large__airport": "Large  Airport",
		"_closed":        " Closed",
	} {
		if got := typeText(raw); got != want {
			t.Errorf("typeText(%q) = %q, want %q", raw, got, want)
		}
	}
}

func TestParseAirfieldsRefusesMalformedData(t *testing.T) {
	const header = "ident,type,name,municipality,country,region,iata,elevation_ft,lat,lon\n"

	tests := []struct {
		name   string
		source string
		want   string
	}{
		{
			name:   "a ragged record",
			source: header + "KIND,large_airport,Indianapolis,Indianapolis,US,US-IN,IND,797\n",
			want:   "reading the airfield data",
		},
		{
			name:   "nothing at all",
			source: "",
			want:   "the airfield data is empty",
		},
		{
			name:   "a header and no records",
			source: header,
			want:   "the airfield data is empty",
		},
		{
			name:   "a record of the wrong width",
			source: "ident,name,lat\nKIND,Indianapolis,39.7173\n",
			want:   "a record has 3 fields, want 10",
		},
		{
			name:   "an unreadable latitude",
			source: header + "KIND,large_airport,Indianapolis,Indianapolis,US,US-IN,IND,797,north,-86.2944\n",
			want:   "KIND: latitude",
		},
		{
			name:   "an unreadable longitude",
			source: header + "KIND,large_airport,Indianapolis,Indianapolis,US,US-IN,IND,797,39.7173,west\n",
			want:   "KIND: longitude",
		},
		{
			name:   "an unreadable elevation",
			source: header + "KIND,large_airport,Indianapolis,Indianapolis,US,US-IN,IND,high,39.7173,-86.2944\n",
			want:   "KIND: elevation",
		},
		{
			name: "the same ident twice",
			source: header +
				"KIND,large_airport,Indianapolis,Indianapolis,US,US-IN,IND,797,39.7173,-86.2944\n" +
				"KIND,large_airport,Indianapolis,Indianapolis,US,US-IN,IND,797,39.7173,-86.2944\n",
			want: `duplicate ident "KIND"`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			parsed, err := parseAirfields(tc.source)
			if err == nil {
				t.Fatalf("parsed %d airfields, want an error", len(parsed))
			}
			if parsed != nil {
				t.Errorf("returned %d airfields beside the error, want none", len(parsed))
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error = %q, want it to mention %q", err, tc.want)
			}
		})
	}
}

func TestBadEmbeddedDataPanicsAtInit(t *testing.T) {
	defer func() {
		recovered := recover()
		if recovered == nil {
			t.Fatal("a broken airfield file parsed without panicking")
		}
		message, ok := recovered.(string)
		if !ok {
			t.Fatalf("panicked with %T, want the string mustParseAirfields builds", recovered)
		}
		if !strings.HasPrefix(message, "airport: ") {
			t.Errorf("panic = %q, want it to name the package", message)
		}
	}()

	mustParseAirfields("")
}

func TestPositionReportsNoPositionRatherThanAZeroOneWhenLocationRefuses(t *testing.T) {
	token, region, ok := position(Airport{Ident: "ZZZZ", Lat: 91, Lon: 0})
	if ok {
		t.Fatalf("a latitude of 91 converted to %q, want a refusal", token)
	}
	if token != "" || region != "" {
		t.Errorf("refused with token %q and region %q, want both empty", token, region)
	}
}

// The whitelist is the first layer, and it exists for what output escaping
// cannot reach. "@here" is the case: backslash-escaping it suppresses the
// rendered link, but Mattermost's mention scan reads the raw message and treats
// a backslash as a separator, so the notification fires from a message whose
// author typed four letters.
func TestTheTextWhitelistRefusesWhatEscapingCannotFix(t *testing.T) {
	for _, field := range []string{
		"Ramstein AB @here Ops",
		"Cape Town :fire: Intl",
		"Kandahar #RWY09",
		"see www.example.com",
		"a | b",
		`a \ b`,
		"a\nb",
		"a\rb",
		"a<b>c",
		"a~b",
		"a*b",
	} {
		if validText(field) {
			t.Errorf("validText(%q) = true, want it refused", field)
		}
	}
}

func TestTheTextWhitelistAcceptsWhatTheDatabaseUses(t *testing.T) {
	for _, field := range []string{
		"Cameri Air Base [MIL]",
		"Hotel Sant`anna Heliport",
		"São Tomé & Príncipe",
		`Warren "Bud" Woods Palmer Municipal Airport`,
		"Cam+Motor Airport",
		"Bill & Hillary Clinton National Airport/Adams Field",
		"Zhukovsky–Ramenskoye",
		"large_airport",
		"US-IN",
	} {
		if !validText(field) {
			t.Errorf("validText(%q) = false, want it accepted", field)
		}
	}
}
