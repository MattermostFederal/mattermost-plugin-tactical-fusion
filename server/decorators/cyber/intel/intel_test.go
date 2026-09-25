package intel

import (
	"errors"
	"fmt"
	"net/netip"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func stamp(name string) string {
	return fmt.Sprintf("%s%d\t%s\t2026-09-01T00:00:00Z\ttest\n", SchemaPrefix, SchemaVersion, name)
}

func writeDataset(t *testing.T, dir, name string, rows ...string) string {
	t.Helper()

	path := filepath.Join(dir, name+Suffix)
	body := stamp(name) + strings.Join(rows, "\n")
	if len(rows) > 0 {
		body += "\n"
	}

	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("writing %s: %v", path, err)
	}

	return path
}

func cveRow(id string, summary string) string {
	return strings.Join([]string{id, "2021-12-10", "2022-01-01", "10.0", "Critical", "AV:N", "CWE-502", summary}, "\t")
}

func openIn(t *testing.T, dirs ...string) *Set {
	t.Helper()

	set, problems := Open(dirs)
	for _, problem := range problems {
		t.Fatalf("unexpected problem: %v", problem)
	}
	t.Cleanup(set.Close)

	return set
}

func TestLookupFindsEveryRowIncludingTheEdges(t *testing.T) {
	dir := t.TempDir()

	var rows []string
	for i := 1; i <= 200; i++ {
		rows = append(rows, cveRow(fmt.Sprintf("CVE-2021-%04d", i), fmt.Sprintf("row %d", i)))
	}
	writeDataset(t, dir, NameCVE, rows...)

	set := openIn(t, dir)

	for i := 1; i <= 200; i++ {
		id := fmt.Sprintf("CVE-2021-%04d", i)
		record, err := set.CVE(id)
		if err != nil {
			t.Fatalf("%s was not found", id)
		}
		if record.Summary != fmt.Sprintf("row %d", i) {
			t.Fatalf("%s returned %q", id, record.Summary)
		}
	}
}

func TestLookupDeclinesAKeyThatIsNotThere(t *testing.T) {
	dir := t.TempDir()
	writeDataset(t, dir, NameCVE,
		cveRow("CVE-2021-0002", "b"),
		cveRow("CVE-2021-0004", "d"),
		cveRow("CVE-2021-0006", "f"),
	)

	set := openIn(t, dir)

	for _, id := range []string{
		"CVE-2021-0001",
		"CVE-2021-0003",
		"CVE-2021-0005",
		"CVE-2021-0007",
		"CVE-1999-0001",
		"CVE-2099-9999",
	} {
		if _, err := set.CVE(id); err == nil {
			t.Errorf("%s was found and is not in the file", id)
		}
	}
}

func TestAnEmptyBodyFindsNothing(t *testing.T) {
	dir := t.TempDir()
	writeDataset(t, dir, NameCVE)

	set := openIn(t, dir)

	if _, err := set.CVE("CVE-2021-0001"); err == nil {
		t.Fatalf("an empty dataset answered")
	}
	if !set.Has(NameCVE) {
		t.Fatalf("an empty dataset should still be installed")
	}
}

func TestASingleRowIsFound(t *testing.T) {
	dir := t.TempDir()
	writeDataset(t, dir, NameCVE, cveRow("CVE-2021-44228", "log4j"))

	set := openIn(t, dir)

	record, err := set.CVE("CVE-2021-44228")
	if err != nil {
		t.Fatalf("the only row was not found")
	}
	if record.Severity != "Critical" || len(record.Weaknesses) != 1 || record.Weaknesses[0] != "CWE-502" {
		t.Fatalf("row read back as %+v", record)
	}
	if _, err := set.CVE("CVE-2021-44227"); err == nil {
		t.Fatalf("a key below the only row was found")
	}
	if _, err := set.CVE("CVE-2021-44229"); err == nil {
		t.Fatalf("a key above the only row was found")
	}
}

func TestASchemaStampIsRequired(t *testing.T) {
	cases := map[string]string{
		"no stamp":        "CVE-2021-0001\ta\tb\tc\td\te\tf\tg\n",
		"another schema":  "#tactical-fusion-cyber/99\tcve\t2026-09-01T00:00:00Z\ttest\n",
		"another dataset": "#tactical-fusion-cyber/1\tkev\t2026-09-01T00:00:00Z\ttest\n",
		"too few fields":  "#tactical-fusion-cyber/1\tcve\n",
		"no newline":      "#tactical-fusion-cyber/1\tcve\t2026-09-01T00:00:00Z\ttest",
	}

	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, NameCVE+Suffix)
			if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
				t.Fatalf("writing: %v", err)
			}

			set, problems := Open([]string{dir})
			t.Cleanup(set.Close)

			if len(problems) != 1 {
				t.Fatalf("%d problems, want 1", len(problems))
			}
			if problems[0].Class != ErrorSchema {
				t.Fatalf("class %v, want ErrorSchema", problems[0].Class)
			}
			if set.Has(NameCVE) {
				t.Fatalf("a stampless file was used anyway")
			}
		})
	}
}

func TestAnUnknownDatasetNameIsRefused(t *testing.T) {
	dir := t.TempDir()
	writeDataset(t, dir, "nonsense")

	set, problems := Open([]string{dir})
	t.Cleanup(set.Close)

	if len(problems) != 1 || problems[0].Class != ErrorName {
		t.Fatalf("problems: %+v", problems)
	}
}

func TestTheStampCarriesWhenItWasGenerated(t *testing.T) {
	dir := t.TempDir()
	writeDataset(t, dir, NameKEV)

	set := openIn(t, dir)

	if got := set.Generated(NameKEV); got != "2026-09-01T00:00:00Z" {
		t.Fatalf("generated %q", got)
	}
	if got := set.Generated(NameCVE); got != "" {
		t.Fatalf("an absent dataset reported %q", got)
	}
}

func TestALaterDirectoryOverridesAnEarlierOneByName(t *testing.T) {
	bundled, dropIn := t.TempDir(), t.TempDir()

	writeDataset(t, bundled, NameCVE, cveRow("CVE-2021-44228", "from the bundle"))
	writeDataset(t, dropIn, NameCVE, cveRow("CVE-2021-44228", "from the directory"))

	set := openIn(t, bundled, dropIn)

	record, err := set.CVE("CVE-2021-44228")
	if err != nil {
		t.Fatalf("not found")
	}
	if record.Summary != "from the directory" {
		t.Fatalf("the bundled file won: %q", record.Summary)
	}
}

func TestStatusesNameEveryDatasetPresentOrNot(t *testing.T) {
	dir := t.TempDir()
	writeDataset(t, dir, NameKEV)

	set := openIn(t, dir)

	statuses := set.Statuses()
	if len(statuses) != len(Names) {
		t.Fatalf("%d statuses for %d datasets", len(statuses), len(Names))
	}

	for i, status := range statuses {
		if status.Name != Names[i] {
			t.Fatalf("status %d is %q, want %q", i, status.Name, Names[i])
		}
		if status.Present != (status.Name == NameKEV) {
			t.Fatalf("%s present=%v", status.Name, status.Present)
		}
	}
}

func TestANilSetAnswersNothingAndStillReports(t *testing.T) {
	var set *Set

	if _, err := set.CVE("CVE-2021-44228"); err == nil {
		t.Fatalf("a nil set answered")
	}
	if set.Has(NameCVE) || set.Generated(NameCVE) != "" || set.Watchlist("x") != nil {
		t.Fatalf("a nil set claimed to hold something")
	}
	if len(set.Statuses()) != len(Names) {
		t.Fatalf("a nil set did not report every dataset as absent")
	}
	if record, _ := set.IP(netip.MustParseAddr("203.0.113.7")); !record.Empty() {
		t.Fatalf("a nil set described an address")
	}
}

func ipRow(start, end netip.Addr, asn, name, country, region, city string) string {
	return strings.Join([]string{IPKey(start), IPKey(end), asn, name, country, region, city}, "\t")
}

func TestIPRangesAreFoundForBothFamilies(t *testing.T) {
	dir := t.TempDir()

	writeDataset(t, dir, NameIP,
		ipRow(netip.MustParseAddr("8.8.8.0"), netip.MustParseAddr("8.8.8.255"),
			"AS15169", "Google LLC", "US", "California", "Mountain View"),
		ipRow(netip.MustParseAddr("203.0.113.0"), netip.MustParseAddr("203.0.113.255"),
			"AS64496", "Example", "AU", "", ""),
		ipRow(netip.MustParseAddr("2001:db8::"), netip.MustParseAddr("2001:db8::ffff"),
			"AS64497", "Example Six", "NZ", "", ""),
	)

	set := openIn(t, dir)

	cases := []struct {
		addr string
		asn  string
	}{
		{"8.8.8.8", "AS15169"},
		{"8.8.8.0", "AS15169"},
		{"8.8.8.255", "AS15169"},
		{"203.0.113.7", "AS64496"},
		{"2001:db8::1", "AS64497"},
	}

	for _, tc := range cases {
		t.Run(tc.addr, func(t *testing.T) {
			record, err := set.IP(netip.MustParseAddr(tc.addr))
			if err != nil {
				t.Fatalf("%s was not found: %v", tc.addr, err)
			}
			if record.ASN != tc.asn {
				t.Fatalf("%s gave %q, want %q", tc.addr, record.ASN, tc.asn)
			}
			if len(record.Sources) != 1 || record.Sources[0] != NameIP {
				t.Fatalf("sources %v", record.Sources)
			}
		})
	}
}

func TestAnAddressOutsideEveryRangeDescribesNothing(t *testing.T) {
	dir := t.TempDir()

	writeDataset(t, dir, NameIP,
		ipRow(netip.MustParseAddr("8.8.8.0"), netip.MustParseAddr("8.8.8.255"),
			"AS15169", "Google LLC", "US", "", ""),
	)

	set := openIn(t, dir)

	for _, addr := range []string{"8.8.7.255", "8.8.9.0", "1.1.1.1", "2001:db8::1"} {
		if record, _ := set.IP(netip.MustParseAddr(addr)); !record.Empty() {
			t.Errorf("%s was described as %+v", addr, record)
		}
	}
}

func TestTheIPKeySortsBothFamiliesTogether(t *testing.T) {
	lower := IPKey(netip.MustParseAddr("8.8.8.0"))
	upper := IPKey(netip.MustParseAddr("8.8.8.255"))
	sixish := IPKey(netip.MustParseAddr("2001:db8::"))

	if lower >= upper {
		t.Fatalf("%q does not sort before %q", lower, upper)
	}
	if len(lower) != 32 || len(sixish) != 32 {
		t.Fatalf("keys are %d and %d characters", len(lower), len(sixish))
	}
	if IPKey(netip.MustParseAddr("::ffff:8.8.8.0")) != lower {
		t.Fatalf("a mapped address keys differently from its own family")
	}
}

func TestTheWatchlistCarriesEveryRowForAnIndicator(t *testing.T) {
	dir := t.TempDir()

	writeDataset(t, dir, NameWatchlist,
		strings.Join([]string{"203.0.113.7", "ip", "malicious", "internal", "beaconing", "2026-08-01"}, "\t"),
		strings.Join([]string{"203.0.113.7", "ip", "Under review", "partner", "", "2026-08-09"}, "\t"),
		strings.Join([]string{"44d88612fea8a8f36de82e1278abb02f", "hash", "benign", "internal", "eicar", "2026-08-02"}, "\t"),
	)

	set := openIn(t, dir)

	entries := set.Watchlist("203.0.113.7")
	if len(entries) != 2 {
		t.Fatalf("%d entries, want 2", len(entries))
	}
	if entries[0].Verdict != "malicious" || entries[1].Source != "partner" {
		t.Fatalf("entries %+v", entries)
	}
	if got := set.Watchlist("198.51.100.1"); got != nil {
		t.Fatalf("an indicator with no rows returned %+v", got)
	}

	if !KnownVerdict("malicious") || !KnownVerdict("  Benign ") {
		t.Fatalf("a recognized verdict was not recognized")
	}
	if KnownVerdict("Under review") {
		t.Fatalf("free text was treated as a recognized verdict")
	}
}

func TestChangedNoticesANewFileAndANewDirectoryList(t *testing.T) {
	dir := t.TempDir()
	writeDataset(t, dir, NameKEV)

	set := openIn(t, dir)

	if set.Changed([]string{dir}) {
		t.Fatalf("an untouched directory reported a change")
	}
	if !set.Changed([]string{dir, t.TempDir()}) {
		t.Fatalf("a new directory list did not report a change")
	}

	writeDataset(t, dir, NameCVE, cveRow("CVE-2021-44228", "new"))
	if !set.Changed([]string{dir}) {
		t.Fatalf("a dropped-in dataset did not report a change")
	}
}

func TestChangedNoticesAReplacedFile(t *testing.T) {
	dir := t.TempDir()
	path := writeDataset(t, dir, NameCVE, cveRow("CVE-2021-0001", "before"))

	set := openIn(t, dir)
	if set.Changed([]string{dir}) {
		t.Fatalf("an untouched file reported a change")
	}

	body := stamp(NameCVE) + cveRow("CVE-2021-0001", "after this one is longer") + "\n"
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("rewriting: %v", err)
	}

	if !set.Changed([]string{dir}) {
		t.Fatalf("a replaced file did not report a change")
	}
}

func TestARowOfTheWrongShapeIsRefused(t *testing.T) {
	dir := t.TempDir()
	writeDataset(t, dir, NameWatchlist, "203.0.113.7\tip\tmalicious")

	set, problems := Open([]string{dir})
	t.Cleanup(set.Close)

	if len(problems) != 1 || problems[0].Class != ErrorUnreadable {
		t.Fatalf("problems: %+v", problems)
	}
}

func TestAMissingDirectoryIsNotAProblem(t *testing.T) {
	set, problems := Open([]string{filepath.Join(t.TempDir(), "absent"), ""})
	t.Cleanup(set.Close)

	if len(problems) != 0 {
		t.Fatalf("problems: %+v", problems)
	}
	if len(set.Statuses()) != len(Names) {
		t.Fatalf("statuses were not reported")
	}
}

func TestTrailingBlankLinesDoNotHideRows(t *testing.T) {
	long := strings.Repeat("z", 10000)

	cases := map[string]struct {
		rows    []string
		trailer string
	}{
		"one blank after one row":    {[]string{"CVE-2021-0001"}, "\n"},
		"one blank after two rows":   {[]string{"CVE-2021-0001", "CVE-2021-0002"}, "\n"},
		"two blanks":                 {[]string{"CVE-2021-0001", "CVE-2021-0002"}, "\n\n"},
		"a blank after a long row":   {[]string{"CVE-2021-0001", "CVE-2021-0002", "CVE-2021-0003"}, "\n"},
		"no trailing newline at all": {[]string{"CVE-2021-0001", "CVE-2021-0002"}, ""},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()

			body := stamp(NameCVE)
			for i, id := range tc.rows {
				tail := "summary"
				if name == "a blank after a long row" && i == len(tc.rows)-1 {
					tail = long
				}
				body += cveRow(id, tail) + "\n"
			}
			if tc.trailer == "" {
				body = strings.TrimSuffix(body, "\n")
			} else {
				body += strings.TrimPrefix(tc.trailer, "\n")
			}

			path := filepath.Join(dir, NameCVE+Suffix)
			if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
				t.Fatalf("writing: %v", err)
			}

			set := openIn(t, dir)

			for _, id := range tc.rows {
				if _, err := set.CVE(id); err != nil {
					t.Errorf("%s is in the file and reports as absent", id)
				}
			}
		})
	}
}

func TestABodyOfBlankLinesFindsNothing(t *testing.T) {
	dir := t.TempDir()

	path := filepath.Join(dir, NameCVE+Suffix)
	if err := os.WriteFile(path, []byte(stamp(NameCVE)+"\n\n\n"), 0o600); err != nil {
		t.Fatalf("writing: %v", err)
	}

	set := openIn(t, dir)

	if _, err := set.CVE("CVE-2021-0001"); err == nil {
		t.Fatalf("a body of blank lines answered")
	}
}

func TestASkippedFileDoesNotForceAPermanentReopen(t *testing.T) {
	cases := map[string]func(dir string){
		"a file with no stamp": func(dir string) {
			_ = os.WriteFile(filepath.Join(dir, NameCVE+Suffix), []byte("CVE-2021-0001\ta\n"), 0o600)
		},
		"a name this build does not read": func(dir string) {
			writeDataset(t, dir, "notes")
		},
		"a vendor database this build cannot classify": func(dir string) {
			_ = os.WriteFile(filepath.Join(dir, "something"+MMDBSuffix), []byte("not a database"), 0o600)
		},
	}

	for name, write := range cases {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			writeDataset(t, dir, NameKEV)
			write(dir)

			set, _ := Open([]string{dir})
			t.Cleanup(set.Close)

			if set.Changed([]string{dir}) {
				t.Fatalf("an untouched directory reports as changed, so every request reopens")
			}
		})
	}
}

func TestTheDocumentedOverrideIsNotAPermanentChange(t *testing.T) {
	bundled, dropIn := t.TempDir(), t.TempDir()

	writeDataset(t, bundled, NameKEV)
	writeDataset(t, dropIn, NameKEV)

	set := openIn(t, bundled, dropIn)

	if set.Changed([]string{bundled, dropIn}) {
		t.Fatalf("the documented override reports as changed on an untouched directory")
	}
}

func TestAReplacedVendorDatabaseIsNoticed(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "GeoLite2-ASN"+MMDBSuffix)

	if err := os.WriteFile(path, []byte("not a database"), 0o600); err != nil {
		t.Fatalf("writing: %v", err)
	}

	set, _ := Open([]string{dir})
	t.Cleanup(set.Close)

	if set.Changed([]string{dir}) {
		t.Fatalf("an untouched vendor database reports as changed")
	}

	if err := os.WriteFile(path, []byte("not a database, and longer than before"), 0o600); err != nil {
		t.Fatalf("rewriting: %v", err)
	}

	if !set.Changed([]string{dir}) {
		t.Fatalf("a replaced vendor database was not noticed")
	}
}

func TestAReadFailureIsNotAMissingRow(t *testing.T) {
	dir := t.TempDir()
	writeDataset(t, dir, NameCVE, strings.Join([]string{
		"CVE-2021-44228", "2021-12-10", "2021-12-14", "10.0", "Critical", "AV:N", "CWE-502", "Log4Shell",
	}, "\t"))

	set := openIn(t, dir)

	if _, err := set.CVE("CVE-2021-44228"); err != nil {
		t.Fatalf("the row was not readable to begin with: %v", err)
	}

	if err := set.datasets[NameCVE].handle.Close(); err != nil {
		t.Fatalf("closing: %v", err)
	}

	_, err := set.CVE("CVE-2021-44228")
	switch {
	case err == nil:
		t.Fatalf("a closed file answered")
	case errors.Is(err, ErrNotFound):
		t.Fatalf("a read failure was reported as a missing row")
	case errors.Is(err, ErrNoDataset):
		t.Fatalf("a read failure was reported as a missing dataset")
	}
}

func TestAMissingDatasetAndAMissingRowAreDifferentErrors(t *testing.T) {
	dir := t.TempDir()
	writeDataset(t, dir, NameCVE, strings.Join([]string{
		"CVE-2021-44228", "2021-12-10", "2021-12-14", "10.0", "Critical", "AV:N", "CWE-502", "Log4Shell",
	}, "\t"))

	set := openIn(t, dir)

	if _, err := set.CVE("CVE-2021-44227"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("a key the file does not hold gave %v", err)
	}
	if _, err := set.KEV("CVE-2021-44228"); !errors.Is(err, ErrNoDataset) {
		t.Fatalf("a dataset that is not installed gave %v", err)
	}
	if _, err := set.IP(netip.MustParseAddr("203.0.113.7")); !errors.Is(err, ErrNoDataset) {
		t.Fatalf("an address with no dataset at all gave %v", err)
	}
}

func openDescriptors(t *testing.T) int {
	t.Helper()

	entries, err := os.ReadDir("/proc/self/fd")
	if err != nil {
		t.Skipf("no /proc/self/fd on this platform: %v", err)
	}

	return len(entries)
}

func settleBelow(t *testing.T, target int) bool {
	t.Helper()

	deadline := time.Now().Add(5 * time.Second)
	for {
		runtime.GC()

		if openDescriptors(t) <= target {
			return true
		}
		if !time.Now().Before(deadline) {
			return false
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// A stale set is dropped rather than closed, because Close munmaps the vendor
// databases and a request already reading through one would not survive it.
// Dropping is only correct if the runtime then releases the handles.
func TestADroppedSetReleasesItsHandles(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("the descriptor count is read from /proc")
	}

	dir := t.TempDir()
	writeDataset(t, dir, NameCVE, strings.Join([]string{
		"CVE-2021-44228", "2021-12-10", "2021-12-14", "10.0", "Critical", "AV:N", "CWE-502", "Log4Shell",
	}, "\t"))
	writeDataset(t, dir, NameKEV, strings.Join([]string{
		"CVE-2021-44228", "2021-12-10", "2021-12-24", "Known", "Apache Log4j2", "Apply updates.",
	}, "\t"))
	writeMMDB(t, filepath.Join(dir, "GeoLite2-ASN"+MMDBSuffix), "GeoLite2-ASN", asnRecord(t, 15169, "Google LLC"))

	const (
		rounds    = 50
		perSetFDs = 2
	)

	dropOne := func() {
		set, problems := Open([]string{dir})
		if len(problems) != 0 {
			t.Fatalf("opening: %v", problems)
		}
		if _, err := set.CVE("CVE-2021-44228"); err != nil {
			t.Fatalf("reading the dataset: %v", err)
		}
		if _, err := set.IP(netip.MustParseAddr("8.8.8.8")); err != nil {
			t.Fatalf("reading the vendor database: %v", err)
		}
	}

	dropOne()
	runtime.GC()
	runtime.GC()
	before := openDescriptors(t)

	for range rounds {
		dropOne()
	}

	if !settleBelow(t, before+perSetFDs) {
		t.Fatalf("%d descriptors are still open after %d dropped sets, against %d before; "+
			"a dropped set is meant to release its handles once the runtime collects it",
			openDescriptors(t), rounds, before)
	}
}

func TestRowsLongerThanAReadBlockAreFoundWholeAnywhereInTheFile(t *testing.T) {
	pieces := []string{"overflow ", "é", "漢字", "🔥", "A"}

	var rows []string
	want := map[string]string{}
	for i := range 60 {
		id := fmt.Sprintf("CVE-2021-%04d", 1000+i*2)

		var summary strings.Builder
		for summary.Len() < blockBytes+500+i*97 {
			summary.WriteString(pieces[(summary.Len()+i)%len(pieces)])
		}

		rows = append(rows, cveRow(id, summary.String()))
		want[id] = summary.String()
	}

	dir := t.TempDir()
	writeDataset(t, dir, NameCVE, rows...)
	set := openIn(t, dir)

	for id, summary := range want {
		record, err := set.CVE(id)
		if err != nil {
			t.Fatalf("%s: %v", id, err)
		}
		if record.Summary != summary {
			t.Fatalf("%s came back %d bytes, want %d", id, len(record.Summary), len(summary))
		}
	}

	for _, absent := range []string{"CVE-2021-0999", "CVE-2021-1001", "CVE-2021-1059", "CVE-2021-9999"} {
		if _, err := set.CVE(absent); !errors.Is(err, ErrNotFound) {
			t.Fatalf("%s, which falls between long rows, gave %v", absent, err)
		}
	}
}
