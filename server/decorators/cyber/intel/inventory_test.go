package intel

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func inventoryEntry(t *testing.T, inv Inventory, name string) DatasetEntry {
	t.Helper()
	for _, entry := range inv.Datasets {
		if entry.Name == name {
			return entry
		}
	}
	t.Fatalf("%s is not in the inventory: %+v", name, inv.Datasets)
	return DatasetEntry{}
}

func TestTheInventoryCountsEveryRecordOfEveryLoadedDataset(t *testing.T) {
	dir := t.TempDir()
	writeDataset(t, dir, NameCVE, cveRow("CVE-2021-0001", "a"), cveRow("CVE-2021-0002", "b"), cveRow("CVE-2021-0003", "c"))

	entry := inventoryEntry(t, openIn(t, dir).Inventory(), NameCVE)

	if entry.Records != 3 || entry.CountErr != nil {
		t.Fatalf("records %d, %v, want 3", entry.Records, entry.CountErr)
	}
	if entry.Path != filepath.Join(dir, NameCVE+Suffix) || entry.Size == 0 || entry.Generated == "" {
		t.Fatalf("entry %+v", entry)
	}
}

func TestALastRowWithNoLineEndIsStillCounted(t *testing.T) {
	dir := t.TempDir()
	body := stamp(NameCVE) + cveRow("CVE-2021-0001", "a") + "\n" + cveRow("CVE-2021-0002", "b")
	if err := os.WriteFile(filepath.Join(dir, NameCVE+Suffix), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}

	if entry := inventoryEntry(t, openIn(t, dir).Inventory(), NameCVE); entry.Records != 2 {
		t.Fatalf("records %d, want 2", entry.Records)
	}
}

func TestAStampWithNoRowsCountsNone(t *testing.T) {
	dir := t.TempDir()
	writeDataset(t, dir, NameKEV)

	if entry := inventoryEntry(t, openIn(t, dir).Inventory(), NameKEV); entry.Records != 0 {
		t.Fatalf("records %d, want 0", entry.Records)
	}
}

func TestCountingSpansReadsLongerThanOneChunk(t *testing.T) {
	rows := make([]string, 0, 30000)
	for i := range 30000 {
		rows = append(rows, cveRow(fmt.Sprintf("CVE-2021-%05d", i), strings.Repeat("x", 40)))
	}
	dir := t.TempDir()
	writeDataset(t, dir, NameCVE, rows...)

	if entry := inventoryEntry(t, openIn(t, dir).Inventory(), NameCVE); entry.Records != 30000 {
		t.Fatalf("records %d, want 30000 across chunks of %d bytes", entry.Records, countChunk)
	}
}

func TestTheInventoryNamesTheBundledFileTheConfiguredOneReplaces(t *testing.T) {
	bundled, configured := t.TempDir(), t.TempDir()
	writeDataset(t, bundled, NameKEV)
	writeDataset(t, configured, NameKEV)

	inv := openIn(t, bundled, configured).Inventory()

	if entry := inventoryEntry(t, inv, NameKEV); entry.Path != filepath.Join(configured, NameKEV+Suffix) {
		t.Fatalf("the loaded kev is %s, want the configured one", entry.Path)
	}
	if len(inv.Replaced) != 1 || inv.Replaced[0] != filepath.Join(bundled, NameKEV+Suffix) {
		t.Fatalf("replaced %v", inv.Replaced)
	}
}

func TestTheInventoryKeepsWhatWasSkippedAndWhy(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "notes"+Suffix), []byte("hello\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	set, _ := Open([]string{dir})
	t.Cleanup(set.Close)

	inv := set.Inventory()
	if len(inv.Problems) != 1 || inv.Problems[0].Class != ErrorName {
		t.Fatalf("problems %+v", inv.Problems)
	}
}

func TestANilSetHasAnEmptyInventory(t *testing.T) {
	var set *Set
	if inv := set.Inventory(); len(inv.Datasets) != 0 || len(inv.Problems) != 0 {
		t.Fatalf("inventory %+v", inv)
	}
}

func TestABrokenConfiguredFileFallsBackToTheBundledOne(t *testing.T) {
	bundled, configured := t.TempDir(), t.TempDir()
	writeDataset(t, bundled, NameKEV)
	if err := os.WriteFile(filepath.Join(configured, NameKEV+Suffix), []byte("not a stamp\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	set, problems := Open([]string{bundled, configured})
	t.Cleanup(set.Close)

	if !set.Has(NameKEV) {
		t.Fatal("kev is not installed although the bundled file is good")
	}
	inv := set.Inventory()
	if entry := inventoryEntry(t, inv, NameKEV); entry.Path != filepath.Join(bundled, NameKEV+Suffix) {
		t.Fatalf("the loaded kev is %s, want the bundled one", entry.Path)
	}
	if len(inv.Replaced) != 0 {
		t.Errorf("the bundled file is reported as replaced by a file that did not open: %v", inv.Replaced)
	}
	if len(problems) != 1 || problems[0].Path != filepath.Join(configured, NameKEV+Suffix) || problems[0].Class != ErrorSchema {
		t.Errorf("problems %v, want the broken configured file", problems)
	}
}
