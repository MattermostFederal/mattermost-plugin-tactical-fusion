package airport

import (
	"strings"
	"testing"

	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/decorators/location"
)

func TestRunwaysAndFrequenciesArePinned(t *testing.T) {
	d, ok := Describe("PHNL")
	if !ok {
		t.Fatal("PHNL is not in the data")
	}

	if len(d.Runways) != 6 {
		t.Fatalf("PHNL has %d runways, want 6", len(d.Runways))
	}
	var long Runway
	for _, r := range d.Runways {
		if r.Designation == "08L/26R" {
			long = r
		}
	}
	if long.Designation == "" {
		t.Fatal("PHNL has no runway 08L/26R")
	}
	if long.Length != "12,360 ft" || long.Width != "200 ft" || long.Surface != "Asphalt" {
		t.Errorf("08L/26R = %+v", long)
	}
	if long.Ends == nil {
		t.Fatal("08L/26R has no ends, so it draws no line")
	}
	if long.Ends[0].Token != "21.3252,-157.9430" || long.Ends[1].Token != "21.3252,-157.9070" {
		t.Errorf("ends = %v", *long.Ends)
	}
	if got := RunwayLine(long); got != "12,360 x 200 ft, Asphalt" {
		t.Errorf("RunwayLine = %q", got)
	}

	var tower Frequency
	for _, f := range d.Frequencies {
		if f.Type == "TWR" {
			tower = f
		}
	}
	if tower.MHz != "118.100" {
		t.Errorf("tower = %+v, want 118.100", tower)
	}
}

func TestAnAirfieldWithoutRunwaysOrFrequenciesCarriesNone(t *testing.T) {
	d, ok := Describe("AAXX")
	if !ok {
		t.Fatal("AAXX is not in the data")
	}
	if len(d.Runways) != 0 || len(d.Frequencies) != 0 {
		t.Errorf("AAXX carries %d runways and %d frequencies, want none", len(d.Runways), len(d.Frequencies))
	}
}

func TestTheMilitaryDesignatorIsAWholeWordOfTheName(t *testing.T) {
	count := 0
	for ident, a := range airfields {
		if a.Military == "" {
			continue
		}
		count++
		if !strings.Contains(a.Name, a.Military) {
			t.Fatalf("%s carries designator %q, which is not in %q", ident, a.Military, a.Name)
		}
	}
	if count < 100 {
		t.Fatalf("only %d airfields carry a designator, which does not match the measurement", count)
	}

	d, _ := Describe("PHIK")
	if d.Military != "Air Force Base" || useText(d) != "Military (Air Force Base)" {
		t.Errorf("PHIK renders %q", useText(d))
	}
	d, _ = Describe("KIND")
	if d.Military != "" || useText(d) != "" {
		t.Errorf("KIND renders %q, want no use row", useText(d))
	}
}

func TestEveryRunwayEndIsAcceptedByLocation(t *testing.T) {
	drawn := 0
	for ident := range airfields {
		for _, r := range runwayDetails(runwaysOf(ident)) {
			if r.Ends == nil {
				continue
			}
			drawn++
			for _, end := range r.Ends {
				parsed, ok := location.Parse(location.Format(end.Format), end.Token)
				if !ok || parsed.Canonical() != end.Token {
					t.Fatalf("%s %s: end %q is not a canonical coordinate", ident, r.Designation, end.Token)
				}
			}
		}
	}
	if drawn < 5000 {
		t.Fatalf("only %d runways carry both ends, far fewer than the data holds", drawn)
	}
}

func TestEveryRunwayAndFrequencyFieldPassesTheWhitelist(t *testing.T) {
	for ident, list := range runways {
		for _, r := range list {
			for _, field := range []string{r.LowIdent, r.HighIdent, r.Surface} {
				if !validText(field) {
					t.Fatalf("%s runway %s carries %q", ident, r.LowIdent, field)
				}
			}
		}
	}
	for ident, list := range frequencies {
		for _, f := range list {
			for _, field := range []string{f.Type, f.Description} {
				if !validText(field) {
					t.Fatalf("%s frequency %s carries %q", ident, f.MHz, field)
				}
			}
		}
	}
}

func TestNoAirfieldExceedsTheRunwayOrFrequencyCap(t *testing.T) {
	for ident, list := range runways {
		if len(list) > MaxRunwaysPerAirfield {
			t.Errorf("%s has %d runways", ident, len(list))
		}
	}
	for ident, list := range frequencies {
		if len(list) > MaxFrequenciesPerAirfield {
			t.Errorf("%s has %d frequencies", ident, len(list))
		}
	}
}

func TestRunwayLineOmitsWhatTheDatabaseDoesNotState(t *testing.T) {
	for _, tc := range []struct {
		runway Runway
		want   string
	}{
		{Runway{Length: "5,000 ft", Width: "75 ft", Surface: "Turf", Lighted: "Lighted", Closed: "Closed"}, "5,000 x 75 ft, Turf, lighted, closed"},
		{Runway{Length: "5,000 ft"}, "5,000 ft"},
		{Runway{Width: "75 ft"}, "75 ft wide"},
		{Runway{Surface: "Gravel"}, "Gravel"},
		{Runway{}, ""},
	} {
		if got := RunwayLine(tc.runway); got != tc.want {
			t.Errorf("RunwayLine(%+v) = %q, want %q", tc.runway, got, tc.want)
		}
	}
}

func TestARunwayWithOneEndMissingDrawsNoLine(t *testing.T) {
	out := runwayDetails([]runwayRecord{{LowIdent: "09", HighIdent: "27"}})
	if len(out) != 1 || out[0].Ends != nil {
		t.Fatalf("runwayDetails = %+v, want one runway with no ends", out)
	}

	out = runwayDetails([]runwayRecord{{LowIdent: "09", HasEnds: true, LowLat: 91, LowLon: 0, HighLat: 0, HighLon: 1}})
	if out[0].Ends != nil {
		t.Fatal("an end outside the grammar still drew a line")
	}
}

func TestParseRunwaysRefusesMalformedData(t *testing.T) {
	const header = "ident,le_ident,he_ident,length_ft,width_ft,surface,lighted,closed,le_lat,le_lon,he_lat,he_lon,le_heading\n"
	fields := map[string]Airport{"KIND": {Ident: "KIND"}}

	for name, tc := range map[string]struct{ source, want string }{
		"unknown ident":  {header + "QZQZ,09,27,5000,75,Asphalt,1,0,,,,,\n", "not an airfield this build holds"},
		"no designation": {header + "KIND,,27,5000,75,Asphalt,1,0,,,,,\n", "no designation"},
		"bad length":     {header + "KIND,09,27,long,75,Asphalt,1,0,,,,,\n", "length"},
		"half an end":    {header + "KIND,09,27,5000,75,Asphalt,1,0,39.7,,,,\n", "half stated"},
		"bad end":        {header + "KIND,09,27,5000,75,Asphalt,1,0,north,-86,39,-86,\n", "end"},
		"hostile text":   {header + "KIND,09,27,5000,75,@here,1,0,,,,,\n", "whitelist"},
		"wrong width":    {"ident,le_ident\nKIND,09\n", "want 13"},
		"empty":          {"", "empty"},
	} {
		t.Run(name, func(t *testing.T) {
			parsed, err := parseRunways(tc.source, fields)
			if err == nil {
				t.Fatalf("parsed %d airfields, want an error", len(parsed))
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error = %q, want it to mention %q", err, tc.want)
			}
		})
	}

	var many strings.Builder
	many.WriteString(header)
	for range MaxRunwaysPerAirfield + 1 {
		many.WriteString("KIND,09,27,5000,75,Asphalt,1,0,,,,,\n")
	}
	if _, err := parseRunways(many.String(), fields); err == nil || !strings.Contains(err.Error(), "more than") {
		t.Errorf("past the cap: err = %v", err)
	}
}

func TestParseFrequenciesRefusesMalformedData(t *testing.T) {
	const header = "ident,type,description,mhz\n"
	fields := map[string]Airport{"KIND": {Ident: "KIND"}}

	for name, tc := range map[string]struct{ source, want string }{
		"unknown ident": {header + "QZQZ,TWR,,118.100\n", "not an airfield this build holds"},
		"no type":       {header + "KIND,,,118.100\n", "no type"},
		"bad value":     {header + "KIND,TWR,,tower\n", "frequency"},
		"hostile text":  {header + "KIND,TWR,see www.example.com,118.100\n", "whitelist"},
		"wrong width":   {"ident,type\nKIND,TWR\n", "want 4"},
	} {
		t.Run(name, func(t *testing.T) {
			parsed, err := parseFrequencies(tc.source, fields)
			if err == nil {
				t.Fatalf("parsed %d airfields, want an error", len(parsed))
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error = %q, want it to mention %q", err, tc.want)
			}
		})
	}
}

func TestBuildIATAIndexRefusesADuplicate(t *testing.T) {
	_, err := buildIATAIndex(map[string]Airport{
		"AAAA": {Ident: "AAAA", IATA: "AAA"},
		"BBBB": {Ident: "BBBB", IATA: "AAA"},
	})
	if err == nil || !strings.Contains(err.Error(), "names both") {
		t.Fatalf("err = %v, want a duplicate refusal", err)
	}

	_, err = buildIATAIndex(map[string]Airport{"AAAA": {Ident: "AAAA", IATA: "aaa"}})
	if err == nil {
		t.Fatal("a lower-case IATA code was indexed")
	}
}

func TestTheTableCarriesRunwaysAndFrequencies(t *testing.T) {
	d, _ := DescribeFields("PHNL")
	table := airfieldTable(HREF, "", d)

	if !strings.Contains(table, "| Runways | ") || !strings.Contains(table, "08L/26R 12,360 x 200 ft, Asphalt; ") {
		t.Errorf("the runways row is missing or misrendered:\n%s", table)
	}
	if !strings.Contains(table, "| Frequencies | ") || !strings.Contains(table, "TWR 118.100") {
		t.Errorf("the frequencies row is missing or misrendered:\n%s", table)
	}

	d, _ = DescribeFields("PHIK")
	table = airfieldTable(HREF, "", d)
	if !strings.Contains(table, "| Use | Military (Air Force Base) |") {
		t.Errorf("the use row is missing:\n%s", table)
	}

	d, _ = DescribeFields("AAXX")
	table = airfieldTable(HREF, "", d)
	for _, absent := range []string{"Runways", "Frequencies", "Use"} {
		if strings.Contains(table, absent) {
			t.Errorf("%s was rendered for a field that states none:\n%s", absent, table)
		}
	}
}

func TestThePageCarriesRunwaysAndFrequencies(t *testing.T) {
	d, _ := Describe("PHNL")
	body := renderBody("PHNL", d, true)

	for _, want := range []string{"<h2>Runways</h2>", "08L/26R", "12,360 x 200 ft, Asphalt", "<h2>Frequencies</h2>", "TWR", "118.100"} {
		if !strings.Contains(body, want) {
			t.Errorf("the page does not carry %q", want)
		}
	}

	d, _ = Describe("PHIK")
	body = renderBody("PHIK", d, true)
	if !strings.Contains(body, "Military (Air Force Base)") {
		t.Error("the page does not carry the military designator")
	}

	d, _ = Describe("AAXX")
	body = renderBody("AAXX", d, true)
	if strings.Contains(body, "<h2>") {
		t.Error("a section heading was rendered for an airfield with nothing under it")
	}
}

func TestThePageEscapesRunwayAndFrequencyText(t *testing.T) {
	body := renderBody("KIND", Details{
		Ident:       "KIND",
		Name:        "x",
		Runways:     []Runway{{Designation: "<b>09</b>", Surface: "<i>Turf</i>"}},
		Frequencies: []Frequency{{Type: "<u>TWR</u>", Description: "<s>x</s>", MHz: "118.100"}},
	}, true)

	for _, tag := range []string{"<b>", "<i>", "<u>", "<s>"} {
		if strings.Contains(body, tag) {
			t.Errorf("%s reached the page as markup", tag)
		}
	}
}
