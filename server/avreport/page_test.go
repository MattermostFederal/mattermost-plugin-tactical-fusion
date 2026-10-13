package avreport

import (
	"strings"
	"testing"
	"time"
)

func TestTheBodyRendersEverySection(t *testing.T) {
	body := renderBody(Report{
		Kind:        KindTAF,
		Station:     "KJFK",
		StationName: "John F Kennedy International",
		IssuedAt:    time.Date(2026, time.September, 22, 17, 20, 0, 0, time.UTC),
		Flags:       []string{"AMD", "COR"},
		Raw:         "TAF AMD KJFK 221720Z",
		Rows:        []Row{{Label: "Wind", Value: "280° at 12 kt"}},
		Periods:     []Period{{Period: "TEMPO (temporarily)", Rows: []Row{{Label: "Visibility", Value: "4,000 m"}}}},
		Remarks:     []Row{{Label: "Station", Value: "pressure rising rapidly"}},
		Unknown:     []string{"ZZZ", "QQQ"},
		Format:      "dd",
		Value:       "40.6398, -73.7789",
	})

	for _, want := range []string{
		`<p class="name">TAF KJFK</p>`,
		`<a href="airport?v=KJFK">John F Kennedy International</a>`,
		`<pre>TAF AMD KJFK 221720Z</pre>`,
		`<td>Issued</td><td class="value">22 Sep 2026 17:20Z</td>`,
		`<td>Flags</td><td class="value">AMD, COR</td>`,
		`<td>Wind</td><td class="value">280° at 12 kt</td>`,
		`<td>Position</td><td class="value"><a href="location?f=dd&amp;v=40.6398%2C+-73.7789">40.6398, -73.7789</a></td>`,
		`<h2>TEMPO (temporarily)</h2>`,
		`<td>Visibility</td><td class="value">4,000 m</td>`,
		`<h2>Remarks</h2>`,
		`<td>Station</td><td class="value">pressure rising rapidly</td>`,
		`<h2>Not decoded</h2><p class="note">ZZZ QQQ</p>`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("body is missing %s\n%s", want, body)
		}
	}
}

func TestABareReportRendersOnlyItsHeadingAndText(t *testing.T) {
	body := renderBody(Report{Kind: KindMETAR, Raw: "METAR"})

	if !strings.HasPrefix(body, `<p class="name">METAR</p><pre>METAR</pre>`) {
		t.Errorf("body = %s", body)
	}
	for _, absent := range []string{"airport?", "location?", "Issued", "Flags", "<h2>"} {
		if strings.Contains(body, absent) {
			t.Errorf("body carries %q with nothing to show:\n%s", absent, body)
		}
	}
}

func TestANotamWithAnIssueTimeShowsItsEffectiveRowOnce(t *testing.T) {
	body := renderBody(Report{
		Kind:     KindNOTAM,
		IssuedAt: time.Date(2026, time.September, 22, 17, 20, 0, 0, time.UTC),
		Rows:     []Row{{Label: "Effective", Value: "decoded twice"}, {Label: "Expires", Value: "permanent"}},
	})

	if got := strings.Count(body, "<td>Effective</td>"); got != 1 {
		t.Errorf("Effective rows = %d, want 1\n%s", got, body)
	}
	if strings.Contains(body, "decoded twice") {
		t.Errorf("the duplicate Effective row was rendered:\n%s", body)
	}
	if !strings.Contains(body, "<td>Expires</td>") {
		t.Errorf("the other rows were dropped:\n%s", body)
	}
}

func TestANotamWithoutAnIssueTimeKeepsItsEffectiveRow(t *testing.T) {
	body := renderBody(Report{Kind: KindNOTAM, Rows: []Row{{Label: "Effective", Value: "from the text"}}})

	if !strings.Contains(body, `<td>Effective</td><td class="value">from the text</td>`) {
		t.Errorf("body = %s", body)
	}
}

func TestTheBodyEscapesAuthorText(t *testing.T) {
	body := renderBody(Report{
		Kind:    KindMETAR,
		Station: "<b>",
		Raw:     `<script>alert(1)</script>`,
		Rows:    []Row{{Label: "<i>", Value: `"&"`}},
		Unknown: []string{"<x>"},
	})

	if strings.Contains(body, "<script>") || strings.Contains(body, "<b>") || strings.Contains(body, "<i>") || strings.Contains(body, "<x>") {
		t.Errorf("author text reached the page unescaped:\n%s", body)
	}
	if !strings.Contains(body, "&#34;&amp;&#34;") {
		t.Errorf("value was not escaped:\n%s", body)
	}
}
