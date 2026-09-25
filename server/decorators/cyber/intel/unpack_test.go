package intel

import (
	"bytes"
	"compress/gzip"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func stampedBody(name, generated string, rows ...string) string {
	var body strings.Builder
	fmt.Fprintf(&body, "%s%d\t%s\t%s\ttest\n", SchemaPrefix, SchemaVersion, name, generated)
	for _, row := range rows {
		body.WriteString(row + "\n")
	}
	return body.String()
}

func gzipped(t *testing.T, body string) []byte {
	t.Helper()

	var buf bytes.Buffer
	writer := gzip.NewWriter(&buf)
	if _, err := writer.Write([]byte(body)); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}

	return buf.Bytes()
}

func writeFile(t *testing.T, path string, content []byte) {
	t.Helper()

	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatal(err)
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()

	raw, err := os.ReadFile(path) // #nosec G304 -- a test's own temporary file
	if err != nil {
		t.Fatal(err)
	}

	return string(raw)
}

func noStagingLeft(t *testing.T, dir string) {
	t.Helper()

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), unpackingSuffix) {
			t.Fatalf("a staging file was left behind: %s", entry.Name())
		}
	}
}

func openReporting(t *testing.T, dirs ...string) (*Set, []*FileError) {
	t.Helper()

	set, problems := Open(dirs)
	t.Cleanup(set.Close)

	return set, problems
}

func TestAGzippedDatasetIsUnpackedBesideItselfAndServed(t *testing.T) {
	dir := t.TempDir()
	body := stampedBody(NameCVE, "2026-09-01T00:00:00Z", cveRow("CVE-2021-44228", "Log4Shell"))
	writeFile(t, filepath.Join(dir, "cve"+ArchiveSuffix), gzipped(t, body))

	set, problems := openReporting(t, dir)
	if len(problems) != 0 {
		t.Fatalf("problems: %v", problems)
	}

	if got := readFile(t, filepath.Join(dir, "cve"+Suffix)); got != body {
		t.Fatalf("unpacked to %q, want %q", got, body)
	}
	if record, err := set.CVE("CVE-2021-44228"); err != nil || record.Summary != "Log4Shell" {
		t.Fatalf("lookup gave %+v, %v", record, err)
	}
	noStagingLeft(t, dir)
}

func TestAnArchiveIsNotUnpackedAgainWhileItsStampMatches(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "cve"+ArchiveSuffix),
		gzipped(t, stampedBody(NameCVE, "2026-09-01T00:00:00Z", cveRow("CVE-2021-44228", "Log4Shell"))))
	openReporting(t, dir)

	unpacked := filepath.Join(dir, "cve"+Suffix)
	long := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	if err := os.Chtimes(unpacked, long, long); err != nil {
		t.Fatal(err)
	}

	if _, problems := openReporting(t, dir); len(problems) != 0 {
		t.Fatalf("problems: %v", problems)
	}

	info, err := os.Stat(unpacked)
	if err != nil {
		t.Fatal(err)
	}
	if !info.ModTime().Equal(long) {
		t.Fatalf("the file was rewritten although its stamp matched the archive's")
	}
}

func TestANewerArchiveReplacesTheUnpackedFile(t *testing.T) {
	dir := t.TempDir()
	archive := filepath.Join(dir, "cve"+ArchiveSuffix)

	writeFile(t, archive, gzipped(t, stampedBody(NameCVE, "2026-09-01T00:00:00Z", cveRow("CVE-2021-0001", "old"))))
	openReporting(t, dir)

	newer := stampedBody(NameCVE, "2026-09-02T00:00:00Z", cveRow("CVE-2021-0002", "new"))
	writeFile(t, archive, gzipped(t, newer))
	set, problems := openReporting(t, dir)
	if len(problems) != 0 {
		t.Fatalf("problems: %v", problems)
	}

	if got := readFile(t, filepath.Join(dir, "cve"+Suffix)); got != newer {
		t.Fatalf("unpacked to %q, want the newer archive", got)
	}
	if _, err := set.CVE("CVE-2021-0002"); err != nil {
		t.Fatalf("the newer row is not served: %v", err)
	}
	if _, err := set.CVE("CVE-2021-0001"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("the older row is still served: %v", err)
	}
}

func TestACorruptArchiveLeavesTheExistingFileAlone(t *testing.T) {
	dir := t.TempDir()
	good := stampedBody(NameCVE, "2026-09-01T00:00:00Z", cveRow("CVE-2021-44228", "Log4Shell"))
	writeFile(t, filepath.Join(dir, "cve"+Suffix), []byte(good))

	rows := make([]string, 0, 2000)
	for i := range 2000 {
		rows = append(rows, cveRow(fmt.Sprintf("CVE-2021-%05d", i), strings.Repeat("filler ", 20)))
	}
	whole := gzipped(t, stampedBody(NameCVE, "2026-09-02T00:00:00Z", rows...))
	writeFile(t, filepath.Join(dir, "cve"+ArchiveSuffix), whole[:len(whole)/2])

	set, problems := openReporting(t, dir)
	if len(problems) != 1 || problems[0].Class != ErrorUnpack {
		t.Fatalf("problems: %v, want one unpack failure", problems)
	}

	if got := readFile(t, filepath.Join(dir, "cve"+Suffix)); got != good {
		t.Fatalf("a truncated archive replaced the good file")
	}
	if _, err := set.CVE("CVE-2021-44228"); err != nil {
		t.Fatalf("the good file stopped being served: %v", err)
	}
	noStagingLeft(t, dir)
}

func TestAnArchiveNamingAnotherDatasetIsRefusedBeforeItTouchesAnything(t *testing.T) {
	dir := t.TempDir()
	good := stampedBody(NameCVE, "2026-09-01T00:00:00Z", cveRow("CVE-2021-44228", "Log4Shell"))
	writeFile(t, filepath.Join(dir, "cve"+Suffix), []byte(good))
	writeFile(t, filepath.Join(dir, "cve"+ArchiveSuffix),
		gzipped(t, stampedBody(NameKEV, "2026-09-02T00:00:00Z", "CVE-2021-44228\t2021-12-10\t2021-12-24\tKnown\tLog4j\tPatch")))

	_, problems := openReporting(t, dir)
	if len(problems) != 1 || problems[0].Class != ErrorSchema {
		t.Fatalf("problems: %v, want one schema mismatch", problems)
	}
	if got := readFile(t, filepath.Join(dir, "cve"+Suffix)); got != good {
		t.Fatalf("an archive stamped for another dataset overwrote cve.tsv")
	}
}

func TestAnArchiveOverTheLimitIsRefusedAndLeavesNothingBehind(t *testing.T) {
	dir := t.TempDir()
	archive := filepath.Join(dir, "cve"+ArchiveSuffix)
	target := filepath.Join(dir, "cve"+Suffix)
	writeFile(t, archive, gzipped(t, stampedBody(NameCVE, "2026-09-01T00:00:00Z", cveRow("CVE-2021-0001", strings.Repeat("x", 4096)))))

	err := unpack(archive, target, 1024)
	if !errors.Is(err, ErrTooLarge) {
		t.Fatalf("got %v, want ErrTooLarge", err)
	}
	if _, err := os.Stat(target); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("an oversized archive still produced %s", target)
	}
	noStagingLeft(t, dir)
}

func TestAnArchiveWithAnUnknownNameIsReported(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "notes"+ArchiveSuffix), gzipped(t, stampedBody("notes", "2026-09-01T00:00:00Z")))

	_, problems := openReporting(t, dir)
	if len(problems) != 1 || problems[0].Class != ErrorName {
		t.Fatalf("problems: %v, want one bad name", problems)
	}
	if _, err := os.Stat(filepath.Join(dir, "notes"+Suffix)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("an archive with an unknown name was unpacked")
	}
}

func TestAnUnwritableDirectoryIsReportedRatherThanFatal(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root writes through directory permissions")
	}

	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "cve"+ArchiveSuffix),
		gzipped(t, stampedBody(NameCVE, "2026-09-01T00:00:00Z", cveRow("CVE-2021-44228", "Log4Shell"))))
	readOnly := os.Chmod(dir, 0o500) // #nosec G302 -- a test directory made read-only on purpose
	if readOnly != nil {
		t.Fatal(readOnly)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o700) }) // #nosec G302 -- restores the test directory so it can be removed

	set, problems := openReporting(t, dir)
	if len(problems) != 1 || problems[0].Class != ErrorUnpack {
		t.Fatalf("problems: %v, want one unpack failure", problems)
	}
	if set.Has(NameCVE) {
		t.Fatalf("a dataset was reported installed although nothing could be unpacked")
	}
}

func TestDroppingAnArchiveIsNoticedAndThenSettles(t *testing.T) {
	dir := t.TempDir()
	writeDataset(t, dir, NameKEV)

	set, _ := openReporting(t, dir)
	writeFile(t, filepath.Join(dir, "cve"+ArchiveSuffix),
		gzipped(t, stampedBody(NameCVE, "2026-09-01T00:00:00Z", cveRow("CVE-2021-44228", "Log4Shell"))))

	if !set.Changed([]string{dir}) {
		t.Fatalf("a newly dropped archive was not noticed")
	}

	reopened, _ := openReporting(t, dir)
	if reopened.Changed([]string{dir}) {
		t.Fatalf("the set reported a change straight after unpacking, so it would reopen on every check")
	}
}
