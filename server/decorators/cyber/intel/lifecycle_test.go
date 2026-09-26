package intel

import (
	"errors"
	"net/netip"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const log4shellKEV = "CVE-2021-44228\t2021-12-10\t2021-12-24\tKnown\tApache Log4j2\tApply updates."

func openLifecycleSet(t *testing.T) *Set {
	t.Helper()

	dir := t.TempDir()
	writeDataset(t, dir, NameCVE, cveRow("CVE-2021-44228", "Log4Shell"))
	writeDataset(t, dir, NameKEV, log4shellKEV)
	writeMMDB(t, filepath.Join(dir, "GeoLite2-ASN"+MMDBSuffix), "GeoLite2-ASN", asnRecord(t, 15169, "Google LLC"))

	return openIn(t, dir)
}

func TestCloseWaitsForAReadAlreadyInFlight(t *testing.T) {
	set := openLifecycleSet(t)

	set.guard.RLock()

	closed := make(chan struct{})
	go func() {
		set.Close()
		close(closed)
	}()

	select {
	case <-closed:
		set.guard.RUnlock()
		t.Fatal("Close released the handles while a read was still using them")
	case <-time.After(50 * time.Millisecond):
	}

	row, err := set.datasets[NameCVE].file.Lookup("CVE-2021-44228")
	if err != nil || row[0] != "CVE-2021-44228" {
		set.guard.RUnlock()
		t.Fatalf("the in-flight read failed while Close waited: %v %v", row, err)
	}
	var record IPRecord
	set.enrichFromDatabases(netip.MustParseAddr("8.8.8.8"), &record)
	if record.ASN != "AS15169" {
		set.guard.RUnlock()
		t.Fatalf("the in-flight vendor lookup failed while Close waited: %+v", record)
	}

	set.guard.RUnlock()

	select {
	case <-closed:
	case <-time.After(5 * time.Second):
		t.Fatal("Close never finished once the read was done")
	}

	if _, err := set.datasets[NameCVE].handle.Stat(); !errors.Is(err, os.ErrClosed) {
		t.Fatalf("the dataset handle is still open after Close: %v", err)
	}
}

func TestAClosedSetAnswersRetiredRatherThanReadingFreedHandles(t *testing.T) {
	set := openLifecycleSet(t)
	set.Close()
	set.Close()

	if _, err := set.CVE("CVE-2021-44228"); !errors.Is(err, ErrRetired) {
		t.Fatalf("CVE on a closed set gave %v, want ErrRetired", err)
	}
	if _, err := set.KEV("CVE-2021-44228"); !errors.Is(err, ErrRetired) {
		t.Fatalf("KEV on a closed set gave %v, want ErrRetired", err)
	}
	if _, err := set.IP(netip.MustParseAddr("8.8.8.8")); !errors.Is(err, ErrRetired) {
		t.Fatalf("IP on a closed set gave %v, want ErrRetired", err)
	}
	for _, entry := range set.Inventory().Datasets {
		if !errors.Is(entry.CountErr, ErrRetired) {
			t.Fatalf("counting %s on a closed set gave %v, want ErrRetired", entry.Name, entry.CountErr)
		}
	}
}

func TestARetiredSetIsNeitherAMissingRowNorAMissingDataset(t *testing.T) {
	if errors.Is(ErrRetired, ErrNotFound) || errors.Is(ErrRetired, ErrNoDataset) {
		t.Fatal("a retired set would be reported as an answer about the indicator")
	}
}

func TestARowLongerThanTheCapIsARefusalNotAMissingRow(t *testing.T) {
	dir := t.TempDir()
	huge := cveRow("CVE-2021-0002", strings.Repeat("x", maxLineBytes+1))
	writeDataset(t, dir, NameCVE, cveRow("CVE-2021-0001", "short"), huge, cveRow("CVE-2021-0003", "short"))
	set := openIn(t, dir)

	_, err := set.CVE("CVE-2021-0002")
	if !errors.Is(err, ErrLineTooLong) {
		t.Fatalf("the over-long row gave %v, want ErrLineTooLong", err)
	}

	for _, id := range []string{"CVE-2021-0001", "CVE-2021-0003"} {
		if _, err := set.CVE(id); err != nil && !errors.Is(err, ErrLineTooLong) {
			t.Fatalf("%s beside an over-long row gave %v, want its row or ErrLineTooLong", id, err)
		}
	}
}

func TestReadLineStopsAtTheCapRatherThanBufferingTheFile(t *testing.T) {
	dir := t.TempDir()
	path := writeDataset(t, dir, NameCVE, strings.Repeat("y", 2*maxLineBytes))

	set, _ := Open([]string{dir})
	t.Cleanup(set.Close)

	dataset, ok := set.datasets[NameCVE]
	if !ok {
		t.Fatalf("%s did not open", path)
	}
	if _, _, err := dataset.file.readLine(dataset.file.bodyStart); !errors.Is(err, ErrLineTooLong) {
		t.Fatalf("readLine gave %v, want ErrLineTooLong", err)
	}
	if _, err := dataset.file.firstLineStartAtOrAfter(dataset.file.bodyStart + 10); !errors.Is(err, ErrLineTooLong) {
		t.Fatalf("firstLineStartAtOrAfter gave %v, want ErrLineTooLong", err)
	}
	if _, err := dataset.file.previousLineStart(dataset.file.size); !errors.Is(err, ErrLineTooLong) {
		t.Fatalf("previousLineStart gave %v, want ErrLineTooLong", err)
	}
}

func TestAWatchlistWithAnOverLongRowIsSkippedAsUnreadable(t *testing.T) {
	dir := t.TempDir()
	writeDataset(t, dir, NameWatchlist, strings.Repeat("z", maxLineBytes+1))

	set, problems := Open([]string{dir})
	t.Cleanup(set.Close)

	if set.Has(NameWatchlist) {
		t.Fatal("a watchlist with an over-long row was loaded")
	}
	if len(problems) != 1 || problems[0].Class != ErrorUnreadable || !errors.Is(problems[0].Err, ErrLineTooLong) {
		t.Fatalf("problems = %v, want one unreadable over-long row", problems)
	}
}

func TestAWatchlistLargerThanTheCapIsSkippedWithItsOwnClass(t *testing.T) {
	dir := t.TempDir()
	path := writeDataset(t, dir, NameWatchlist)
	if err := os.Truncate(path, MaxWatchlistBytes+int64(len(stamp(NameWatchlist)))+2); err != nil {
		t.Fatalf("growing: %v", err)
	}

	set, problems := Open([]string{dir})
	t.Cleanup(set.Close)

	if set.Has(NameWatchlist) {
		t.Fatal("a watchlist over the cap was loaded")
	}
	if len(problems) != 1 || problems[0].Class != ErrorTooLarge || !errors.Is(problems[0].Err, ErrWatchlistTooLarge) {
		t.Fatalf("problems = %v, want one ErrorTooLarge", problems)
	}
}

func TestAWatchlistUnderTheCapStillLoads(t *testing.T) {
	dir := t.TempDir()
	row := "198.51.100.7\tip\tmalicious\tsoc\tseen\t2026-09-01"
	writeDataset(t, dir, NameWatchlist, row)

	set := openIn(t, dir)
	if entries := set.Watchlist("198.51.100.7"); len(entries) != 1 {
		t.Fatalf("a small watchlist did not load: %+v", entries)
	}
}
