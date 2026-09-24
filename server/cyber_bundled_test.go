package main

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/decorators/cyber"
	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/decorators/cyber/intel"
)

func openBundledCyber(t *testing.T) *intel.Set {
	t.Helper()

	set, problems := intel.Open([]string{filepath.Join("..", bundledCyberDir)})
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
