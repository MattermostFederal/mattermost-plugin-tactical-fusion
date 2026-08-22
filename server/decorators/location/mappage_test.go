package location

import (
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
)

func mapPageBody(t *testing.T, format Format, canonical string) *httptest.ResponseRecorder {
	t.Helper()

	rec := httptest.NewRecorder()
	RenderMapPage(rec, url.Values{
		paramFormat: {string(format)},
		paramValue:  {canonical},
	}, nil)

	return rec
}

// The same gate as the readings page. A link one refuses must not render on the
// other, or the pair become two doors with different locks.
func TestMapPageRefusesWhatTheReadingsPageRefuses(t *testing.T) {
	cases := []struct {
		name   string
		params url.Values
	}{
		{"no format", url.Values{paramValue: {"18SUJ2347806483"}}},
		{"unknown format", url.Values{paramFormat: {"nope"}, paramValue: {"18SUJ2347806483"}}},
		{"not canonical", url.Values{paramFormat: {"mgrs"}, paramValue: {"18S UJ 234 064"}}},
		{"raw naming somewhere else", url.Values{
			paramFormat: {"mgrs"},
			paramValue:  {"18SUJ2347806483"},
			paramRaw:    {"18SUJ1111111111"},
		}},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			RenderMapPage(rec, c.params, nil)

			if rec.Code != 400 {
				t.Fatalf("status = %d, want 400: %s", rec.Code, rec.Body.String())
			}

			readings := httptest.NewRecorder()
			(&Decorator{}).RenderPage(readings, c.params)
			if readings.Code != 400 {
				t.Fatalf("the readings page allowed what the map page refused (%d)", readings.Code)
			}
		})
	}
}

// The shell is markup and a stylesheet: the basemap, the library and the
// readings all arrive as their own cacheable responses. It was 130 KB when the
// page carried a whole projected world inside it.
func TestMapPageFitsItsBudget(t *testing.T) {
	const budget = 8 * 1024

	worst := 0
	for _, c := range []struct{ lat, lon float64 }{{0, 0}, {51.5, -0.13}, {-33.9, 151.2}, {78.2, 15.6}} {
		rec := httptest.NewRecorder()
		RenderMapPage(rec, url.Values{
			paramFormat: {"dd"},
			paramValue:  {formatDD(c.lat, c.lon)},
		}, nil)
		if n := rec.Body.Len(); n > worst {
			worst = n
		}
	}

	t.Logf("worst map page = %d bytes", worst)
	if worst > budget {
		t.Fatalf("map page is %d bytes, over the %d budget", worst, budget)
	}
}

func formatDD(lat, lon float64) string {
	return strconv.FormatFloat(lat, 'f', 4, 64) + "," + strconv.FormatFloat(lon, 'f', 4, 64)
}

// The map page gets the window out of the way, and keeps its heading in the
// document for anyone reading it with assistive technology rather than eyes.
func TestMapPageGivesTheWindowToTheMap(t *testing.T) {
	rec := mapPageBody(t, FormatMGRS, "18SUJ2347806483")
	body := rec.Body.String()

	if !strings.Contains(body, "body { padding: 0; overflow: hidden; }") {
		t.Error("the map page does not clear the shell's page padding")
	}
	if !strings.Contains(body, "<h1>") {
		t.Error("the map page dropped its heading rather than hiding it")
	}
}
