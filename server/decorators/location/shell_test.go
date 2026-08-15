package location

import (
	"encoding/json"
	"html"
	"math"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// The shell carries the link's parameters and the conversion, and nothing else.
//
// Everything a reader sees is rendered by the page bundle from these, so a
// missing attribute is a page that renders nothing rather than a page that
// renders wrongly.
func TestShellCarriesWhatTheBundleNeeds(t *testing.T) {
	d := &Decorator{}
	rec := httptest.NewRecorder()
	d.RenderPage(rec, mustParams(t, d, `34°03'22"N 118°15'00"W`))

	body := rec.Body.String()

	for _, want := range []string{
		`id="root"`,
		`data-mode="location"`,
		`data-f="dms"`,
		`data-v="340322N1181500W"`,
		`data-r=`,
		`data-conversion=`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("the shell does not carry %s", want)
		}
	}
}

// The conversion in the shell is the one /api/v1/convert would have served.
//
// A public page has no session, so it cannot call that route. Handing over the
// same function's output is what lets the bundle render the rows that need the
// projection, and routing both through Convert is what stops the page and the
// sidebar disagreeing about a coordinate.
func TestShellConversionMatchesTheAPI(t *testing.T) {
	d := &Decorator{}
	rec := httptest.NewRecorder()
	d.RenderPage(rec, mustParams(t, d, "18S UJ 23478 06483"))

	var embedded Conversion
	if err := json.Unmarshal([]byte(shellConversion(t, rec.Body.String())), &embedded); err != nil {
		t.Fatalf("the embedded conversion is not JSON: %v", err)
	}

	want, ok := Convert(FormatMGRS, "18SUJ2347806483", "18S UJ 23478 06483")
	if !ok {
		t.Fatal("Convert refused a token the page accepted")
	}

	if embedded != want {
		t.Errorf("the page embedded\n  %+v\nand the API serves\n  %+v", embedded, want)
	}
}

// A position off the projection still gets a shell.
//
// The map is the bundle's business: it knows the Mercator limit and says so in
// place of a map, and every reading in the table is still true. Refusing to
// render the page would take the readings with it.
func TestShellIsServedForPolarPositions(t *testing.T) {
	loc, ok := Parse(FormatDDH, "89.9000N,12.0000E")
	if !ok {
		t.Fatal("polar fixture did not parse")
	}

	rec := httptest.NewRecorder()
	(&Decorator{}).RenderPage(rec, url.Values{
		paramFormat: {string(FormatDDH)},
		paramValue:  {loc.Canonical()},
	})

	if rec.Code != 200 {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `id="root"`) {
		t.Error("a polar coordinate got no shell, so it renders no readings either")
	}
}

// The author's own text is the one value here that came from a message, and
// this page declares PageMapping, which admits same-origin script. Escaping is
// the only thing between the two.
func TestShellEscapesTheAuthorsText(t *testing.T) {
	page := pageData{loc: mustParse(t, "3510N07901W"), raw: `" onload="alert(1)`}

	root := renderRoot(page, pageModeLocation)

	if strings.Contains(root, `onload="`) {
		t.Fatalf("the author's text broke out of its attribute: %s", root)
	}
	if !strings.Contains(root, html.EscapeString(`" onload="alert(1)`)) {
		t.Fatalf("the author's text was not escaped: %s", root)
	}
}

// The conversion is JSON inside an HTML attribute, which is two escaping
// contexts stacked. A quote surviving either one ends the attribute.
func TestShellConversionCannotEndItsAttribute(t *testing.T) {
	d := &Decorator{}
	rec := httptest.NewRecorder()
	d.RenderPage(rec, mustParams(t, d, "3510N07901W"))

	raw := regexp.MustCompile(`data-conversion="([^"]*)"`).FindStringSubmatch(rec.Body.String())
	if raw == nil {
		t.Fatal("the shell carries no conversion")
	}
	if !strings.Contains(raw[1], "&#34;") {
		t.Fatalf("the conversion's own quotes were not escaped: %s", raw[1])
	}

	if decoded := html.UnescapeString(raw[1]); !json.Valid([]byte(decoded)) {
		t.Fatalf("the conversion did not survive escaping: %s", decoded)
	}
}

func TestMapPageShellSaysItIsTheMap(t *testing.T) {
	rec := mapPageBody(t, FormatMGRS, "18SUJ2347806483")

	if !strings.Contains(rec.Body.String(), `data-mode="map"`) {
		t.Error("the map page does not tell the bundle which view to render")
	}
}

// Both pages load ONE bundle, and each names it relative to its own route.
// Getting the depth wrong is silent: the page renders its shell, the script
// 404s, and the reader sees an empty document.
func TestBothPagesNameTheBundleRelativeToTheirOwnRoute(t *testing.T) {
	d := &Decorator{}
	rec := httptest.NewRecorder()
	d.RenderPage(rec, mustParams(t, d, "18S UJ 23478 06483"))

	cases := []struct{ name, body, want string }{
		{"the map page", mapPageBody(t, FormatMGRS, "18SUJ2347806483").Body.String(), pageAppFromRoot},
		{"the readings page", rec.Body.String(), pageAppFromDecorate},
	}

	for _, c := range cases {
		if !strings.Contains(c.body, `<script src="`+c.want+`" defer>`) {
			t.Errorf("%s does not load %s", c.name, c.want)
		}
	}

	// /map sits at the plugin root and /decorate/<type> one below it, so the two
	// must differ by exactly one level or one of them points outside the bundle.
	if pageAppFromDecorate != "."+pageAppFromRoot {
		t.Errorf("the two paths are %q and %q, which are not one level apart",
			pageAppFromRoot, pageAppFromDecorate)
	}
}

// Both public pages run the bundle, so both give back part of default-src
// 'none'. The whole header is pinned rather than a directive at a time: this is
// the security posture of a public route that echoes author text, and the only
// way a future widening stays a decision is if it shows up here as a diff.
func TestMapPagesCarryExactlyTheMappingPolicy(t *testing.T) {
	const want = "default-src 'none'; style-src 'self' 'unsafe-inline'; script-src 'self'; " +
		"worker-src 'self'; img-src 'self' data: blob:; connect-src 'self'; " +
		"base-uri 'none'; form-action 'none'; frame-ancestors 'none'"

	d := &Decorator{}
	rec := httptest.NewRecorder()
	d.RenderPage(rec, mustParams(t, d, "18S UJ 23478 06483"))

	pages := map[string]string{
		"the readings page": rec.Header().Get("Content-Security-Policy"),
		"the map page":      mapPageBody(t, FormatMGRS, "18SUJ2347806483").Header().Get("Content-Security-Policy"),
	}

	for name, got := range pages {
		if got != want {
			t.Errorf("%s serves\n  %s\nwant\n  %s", name, got, want)
		}

		// font-src stays absent. Nothing loads a font, and its absence is what
		// keeps CSS url() from becoming an exfiltration channel.
		if strings.Contains(got, "font-src") {
			t.Errorf("%s gained font-src, which nothing on it needs", name)
		}
	}
}

func TestErrorPagesKeepTheStrictPolicy(t *testing.T) {
	rec := httptest.NewRecorder()
	(&Decorator{}).RenderPage(rec, mustValues(t, "f=dd&v=nonsense"))

	csp := rec.Header().Get("Content-Security-Policy")
	for _, directive := range []string{"'self'", "img-src", "connect-src", "worker-src"} {
		if strings.Contains(csp, directive) {
			t.Errorf("the rejection page CSP = %q carries %q. A page that renders no "+
				"coordinate has no reason to give anything back", csp, directive)
		}
	}
}

// The land/water edge is the entire content of the map, so it is a non-text
// graphic that conveys information and carries WCAG 1.4.11's 3:1.
//
// The palette lives in the webapp now, in one place for all three surfaces, so
// this reads it from there rather than from a second copy in Go. The first one
// was picked by eye and sat at 1.46:1 in dark and 1.28:1 in light, which
// filling a window read as a near-uniform slab with a dot on it.
func TestMapPaletteCarriesItsContrast(t *testing.T) {
	source := readWebappSource(t, "map/maplibre.ts")

	pairs := []struct {
		theme, name string
		a, b        string
		min         float64
	}{
		{"light", "land against water", "#76889f", "#eef2f7", 3},
		{"light", "the pin's edge against land", "#ffffff", "#76889f", 3},
		{"light", "the cell outline against land", "#0b2f7a", "#76889f", 3},
		{"dark", "land against water", "#5a6472", "#12161d", 3},
		{"dark", "the pin's edge against land", "#0b0e13", "#5a6472", 3},
		{"dark", "the cell outline against land", "#b3d1ff", "#5a6472", 3},
	}

	for _, p := range pairs {
		t.Run(p.theme+": "+p.name, func(t *testing.T) {
			if got := contrastRatio(t, p.a, p.b); got < p.min {
				t.Errorf("contrast is %.2f:1, want at least %.0f:1", got, p.min)
			}
			if !strings.Contains(source, p.a) {
				t.Errorf("%s is not in maplibre.ts, so this test is checking a colour "+
					"no surface draws", p.a)
			}
		})
	}
}

func contrastRatio(t *testing.T, a, b string) float64 {
	t.Helper()

	la, lb := relativeLuminance(t, a), relativeLuminance(t, b)
	if la < lb {
		la, lb = lb, la
	}

	return (la + 0.05) / (lb + 0.05)
}

func relativeLuminance(t *testing.T, hex string) float64 {
	t.Helper()

	v, err := strconv.ParseUint(strings.TrimPrefix(hex, "#"), 16, 32)
	if err != nil {
		t.Fatalf("%q is not a hex colour: %v", hex, err)
	}

	channel := func(c uint64) float64 {
		s := float64(c) / 255
		if s <= 0.03928 {
			return s / 12.92
		}
		return math.Pow((s+0.055)/1.055, 2.4)
	}

	return (0.2126 * channel((v>>16)&0xff)) +
		(0.7152 * channel((v>>8)&0xff)) +
		(0.0722 * channel(v&0xff))
}

// A coordinate on land must be drawn on land.
//
// An earlier renderer clipped filled rings to the view and closed each
// surviving arc, which filled the seaward side: every inland coordinate
// rendered with the ocean under it. Nothing caught it, because the palette put
// land and water within 1.5:1 and the whole map read as one slab either way.
//
// The projection lives in MapLibre now, so what is left to check here is the
// data it is handed: the bundled basemap must have land polygons where there is
// land and none where there is not.
func TestBasemapHasLandUnderLandPositions(t *testing.T) {
	rings := landRings(t)

	cases := []struct {
		name     string
		lat, lon float64
		onLand   bool
	}{
		{"Washington DC", 38.8895, -77.0353, true},
		{"Kansas, far inland", 38.5, -98.0, true},
		{"central Australia", -25.0, 133.0, true},
		{"the middle of the Pacific", 0, -140.0, false},
		{"the middle of the Atlantic", 30.0, -40.0, false},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := false
			for _, ring := range rings {
				if ringContainsLonLat(ring, c.lon, c.lat) {
					got = true
					break
				}
			}
			if got != c.onLand {
				t.Fatalf("the basemap puts land=%v at this position, want %v. A position "+
					"drawn with the wrong fill under it is a map that says the opposite "+
					"of the truth", got, c.onLand)
			}
		})
	}
}

func landRings(t *testing.T) [][][]float64 {
	t.Helper()

	raw, err := os.ReadFile(filepath.Join("..", "..", "..", "public", "map", "world.geo.json"))
	if err != nil {
		t.Fatalf("the bundled basemap is unreadable: %v. Run `make map-data`", err)
	}

	var doc struct {
		Features []struct {
			Properties struct {
				Layer string `json:"layer"`
			} `json:"properties"`
			Geometry struct {
				Type        string          `json:"type"`
				Coordinates json.RawMessage `json:"coordinates"`
			} `json:"geometry"`
		} `json:"features"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("the bundled basemap is not JSON: %v", err)
	}

	var rings [][][]float64
	for _, f := range doc.Features {
		if f.Properties.Layer != "land" {
			continue
		}
		rings = append(rings, outerRings(t, f.Geometry.Type, f.Geometry.Coordinates)...)
	}

	if len(rings) == 0 {
		t.Fatal("the bundled basemap carries no land at all")
	}

	return rings
}

func outerRings(t *testing.T, kind string, raw json.RawMessage) [][][]float64 {
	t.Helper()

	switch kind {
	case "Polygon":
		var p [][][]float64
		if err := json.Unmarshal(raw, &p); err != nil {
			t.Fatalf("bad Polygon: %v", err)
		}
		if len(p) > 0 {
			return p[:1]
		}
	case "MultiPolygon":
		var mp [][][][]float64
		if err := json.Unmarshal(raw, &mp); err != nil {
			t.Fatalf("bad MultiPolygon: %v", err)
		}
		var out [][][]float64
		for _, p := range mp {
			if len(p) > 0 {
				out = append(out, p[0])
			}
		}
		return out
	}

	return nil
}

func ringContainsLonLat(ring [][]float64, lon, lat float64) bool {
	inside := false
	for i, j := 0, len(ring)-1; i < len(ring); j, i = i, i+1 {
		if len(ring[i]) < 2 || len(ring[j]) < 2 {
			continue
		}
		xi, yi := ring[i][0], ring[i][1]
		xj, yj := ring[j][0], ring[j][1]

		if (yi > lat) != (yj > lat) && lon < ((xj-xi)*(lat-yi)/(yj-yi))+xi {
			inside = !inside
		}
	}

	return inside
}

func shellConversion(t *testing.T, body string) string {
	t.Helper()

	m := regexp.MustCompile(`data-conversion="([^"]*)"`).FindStringSubmatch(body)
	if m == nil {
		t.Fatal("the shell carries no conversion")
	}

	return html.UnescapeString(m[1])
}

func mustParse(t *testing.T, token string) Location {
	t.Helper()

	page, ok := validateParams(mustParams(t, &Decorator{}, token))
	if !ok {
		t.Fatalf("the parameters for %q did not validate", token)
	}

	return page.loc
}

func mustValues(t *testing.T, query string) url.Values {
	t.Helper()

	values, err := url.ParseQuery(query)
	if err != nil {
		t.Fatalf("bad query %q: %v", query, err)
	}

	return values
}
