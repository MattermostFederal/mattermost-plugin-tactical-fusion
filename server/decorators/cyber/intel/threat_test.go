package intel

import (
	"errors"
	"strings"
	"testing"
)

func TestThreatReportsJoinTheAdvisoriesAndTheFeeds(t *testing.T) {
	set := openIn(t, "testdata")

	reports, err := set.ThreatReports("::ffff:203.0.113.10")
	if err != nil {
		t.Fatal(err)
	}

	var sources []string
	for _, report := range reports {
		sources = append(sources, report.Source)
	}
	if strings.Join(sources, "|") != "CISA AA99-001A|abuse.ch Feodo Tracker|abuse.ch ThreatFox" {
		t.Fatalf("sources %v, want the advisory first and then each feed", sources)
	}
	if reports[2].Ports != "443,8443" || reports[2].Confidence != "100" || reports[2].Category != CategoryMalicious {
		t.Fatalf("ThreatFox report %+v", reports[2])
	}
}

func TestAHashIsFoundWhateverItsCase(t *testing.T) {
	set := openIn(t, "testdata")

	reports, err := set.ThreatReports(strings.ToUpper("abcdef0123456789abcdef0123456789"))
	if err != nil || len(reports) != 1 || reports[0].Malware != "InventedBot" {
		t.Fatalf("reports %+v, %v", reports, err)
	}
}

func TestATorExitIsContextRatherThanAVerdict(t *testing.T) {
	set := openIn(t, "testdata")

	reports, err := set.ThreatReports("198.51.100.5")
	if err != nil || len(reports) != 1 || reports[0].Category != CategoryContext {
		t.Fatalf("reports %+v, %v", reports, err)
	}
}

func TestAnIndicatorNoFeedNamesIsNotFoundAndNoFeedIsNoDataset(t *testing.T) {
	if _, err := openIn(t, "testdata").ThreatReports("192.0.2.1"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("got %v, want ErrNotFound", err)
	}
	if _, err := openIn(t, t.TempDir()).ThreatReports("192.0.2.1"); !errors.Is(err, ErrNoDataset) {
		t.Fatalf("got %v, want ErrNoDataset", err)
	}
}

func TestIndicatorKeyAgreesWithTheGenerator(t *testing.T) {
	cases := map[string]string{
		"::ffff:203.0.113.10": "203.0.113.10",
		"2001:DB8::7":         "2001:db8::7",
		"ABCDEF":              "abcdef",
	}
	for value, want := range cases {
		if got := IndicatorKey(value); got != want {
			t.Errorf("IndicatorKey(%q) = %q, want %q", value, got, want)
		}
	}
}
