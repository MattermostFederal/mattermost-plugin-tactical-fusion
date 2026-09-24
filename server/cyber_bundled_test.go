package main

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"net/netip"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/decorators/cyber"
	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/decorators/cyber/intel"
)

const maxBundledCyberBytes = 16 << 20

func bundledCyberFiles(t *testing.T) []os.DirEntry {
	t.Helper()

	entries, err := os.ReadDir(filepath.Join("..", bundledCyberDir))
	if err != nil {
		t.Fatal(err)
	}
	shipped := entries[:0]
	for _, entry := range entries {
		name := entry.Name()
		if strings.HasSuffix(name, intel.Suffix) || strings.HasSuffix(name, intel.ArchiveSuffix) {
			if _, err := os.Stat(filepath.Join("..", bundledCyberDir, name+".gz")); err == nil {
				continue
			}
			shipped = append(shipped, entry)
		}
	}
	return shipped
}

func openBundledCyber(t *testing.T) *intel.Set {
	t.Helper()

	dir := t.TempDir()
	for _, entry := range bundledCyberFiles(t) {
		raw, err := os.ReadFile(filepath.Join("..", bundledCyberDir, entry.Name()))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, entry.Name()), raw, 0o600); err != nil {
			t.Fatal(err)
		}
	}

	set, problems := intel.Open([]string{dir})
	for _, problem := range problems {
		t.Fatalf("the bundled datasets do not open: %v", problem)
	}
	t.Cleanup(set.Close)
	return set
}

func kevIDs(t *testing.T) []string {
	t.Helper()

	handle, err := os.Open(filepath.Join("..", bundledCyberDir, "kev.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = handle.Close() }()

	var ids []string
	scanner := bufio.NewScanner(handle)
	for scanner.Scan() {
		if line := scanner.Text(); !strings.HasPrefix(line, intel.SchemaPrefix) {
			id, _, _ := strings.Cut(line, "\t")
			ids = append(ids, id)
		}
	}
	return ids
}

func TestTheBundledCVEDatasetsAreMarkedAsTheKEVSlice(t *testing.T) {
	set := openBundledCyber(t)

	for _, name := range []string{intel.NameCVE, intel.NameCVEDetail} {
		if !set.IsKEVSlice(name) {
			t.Errorf("the bundled %s.tsv is not stamped as the KEV slice, so a miss would read as a CVE that does not exist", name)
		}
	}
}

func TestEveryKEVEntryHasItsCVEBundled(t *testing.T) {
	set := openBundledCyber(t)

	for _, id := range kevIDs(t) {
		if _, err := set.CVE(id); err != nil {
			t.Errorf("%s is in KEV and not in the bundled vulnerability dataset: %v", id, err)
		}
	}
}

func TestACVEOutsideTheBundledSliceSaysTheSliceHoldsOnlyKEV(t *testing.T) {
	d := cyber.Describe(cyber.KindCVE, "CVE-1999-0001", openBundledCyber(t))

	if !strings.Contains(d.Status, "holds only the CVEs in CISA KEV") {
		t.Errorf("a CVE outside the slice reads %q", d.Status)
	}
}

func TestTheBundledIPDatasetAnswersAPublicAddress(t *testing.T) {
	record, err := openBundledCyber(t).IP(netip.MustParseAddr("8.8.8.8"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(record.ASN, "15169") {
		t.Errorf("8.8.8.8 reads as %+v, want AS15169", record)
	}
}

func TestTheBundledCyberDataStaysInsideItsBudget(t *testing.T) {
	var total int
	for _, entry := range bundledCyberFiles(t) {
		raw, err := os.ReadFile(filepath.Join("..", bundledCyberDir, entry.Name()))
		if err != nil {
			t.Fatal(err)
		}
		if strings.HasSuffix(entry.Name(), intel.ArchiveSuffix) {
			total += len(raw)
			continue
		}
		var compressed bytes.Buffer
		writer := gzip.NewWriter(&compressed)
		_, _ = writer.Write(raw)
		_ = writer.Close()
		total += compressed.Len()
	}

	if total > maxBundledCyberBytes {
		t.Errorf("the bundled cyber data is %d bytes compressed, over the %d byte budget", total, maxBundledCyberBytes)
	}
}
