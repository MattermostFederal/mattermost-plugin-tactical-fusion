package intel

import (
	"errors"
	"strings"
	"testing"
)

func TestAnAddressJoinsItsAdvisoryAndTheRangesAroundIt(t *testing.T) {
	reports, err := openIn(t, "testdata").ThreatReports("::ffff:203.0.113.10")
	if err != nil || len(reports) != 2 {
		t.Fatalf("reports %+v, %v", reports, err)
	}
	if reports[0].Source != "CISA AA99-001A" || reports[0].Category != CategoryMalicious {
		t.Errorf("the advisory report is %+v", reports[0])
	}
	if reports[1].Threat != "Invented Cloud IP ranges" || reports[1].Category != CategoryContext {
		t.Errorf("the range report is %+v", reports[1])
	}
}

func TestARangeReportsEveryListCoveringTheAddress(t *testing.T) {
	reports, err := openIn(t, "testdata").ThreatReports("198.51.100.5")
	if err != nil || len(reports) != 2 {
		t.Fatalf("reports %+v, %v", reports, err)
	}
	for _, report := range reports {
		if report.Category != CategoryContext {
			t.Errorf("%+v is not context", report)
		}
	}
}

func TestAnAddressPastEveryRangeIsNotFound(t *testing.T) {
	if _, err := openIn(t, "testdata").ThreatReports("192.0.2.1"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("got %v, want ErrNotFound", err)
	}
	if _, err := openIn(t, "testdata").ThreatReports("2001:db9::1"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("an address past the last range: got %v, want ErrNotFound", err)
	}
}

func TestAKnownFileHashIsContext(t *testing.T) {
	reports, err := openIn(t, "testdata").ThreatReports("D41D8CD98F00B204E9800998ECF8427E")
	if err != nil || len(reports) != 1 || reports[0].Threat != "Hashes for empty files" || reports[0].Category != CategoryContext {
		t.Fatalf("reports %+v, %v", reports, err)
	}
}

func TestAHashIsFoundWhateverItsCase(t *testing.T) {
	reports, err := openIn(t, "testdata").ThreatReports(strings.Repeat("A", 64))
	if err != nil || len(reports) != 1 || reports[0].Source != "CISA AA99-001A" {
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

func TestALeftoverAbuseCHDatasetIsSkippedWithAReason(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"threat", "malware"} {
		writeDataset(t, dir, name, "203.0.113.10\t[]")
	}

	set, problems := Open([]string{dir})
	t.Cleanup(set.Close)

	if len(problems) != 2 {
		t.Fatalf("problems %v, want one per leftover file", problems)
	}
	for _, problem := range problems {
		if problem.Class != ErrorName {
			t.Errorf("%s was not skipped as a name this build does not read: %v", problem.Path, problem)
		}
	}
}
