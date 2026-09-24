package cyber

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/decorators/cyber/intel"
)

type compiledDataset struct {
	compiled string
	rows     []string
}

func compiledDatasets(t *testing.T, files map[string]compiledDataset) *intel.Set {
	t.Helper()

	dir := t.TempDir()
	for name, file := range files {
		body := fmt.Sprintf("%s%d\t%s\t%s\ttest\n%s\n", intel.SchemaPrefix, intel.SchemaVersion, name, file.compiled, strings.Join(file.rows, "\n"))
		if err := os.WriteFile(filepath.Join(dir, name+intel.Suffix), []byte(body), 0o600); err != nil {
			t.Fatalf("writing %s: %v", name, err)
		}
	}

	set, problems := intel.Open([]string{dir})
	for _, problem := range problems {
		t.Fatalf("unexpected problem: %v", problem)
	}
	t.Cleanup(set.Close)
	return set
}

func oldestOf(t *testing.T, d Details) string {
	t.Helper()
	oldest, ok := OldestCompiled(d.Freshness)
	if !ok {
		return ""
	}
	return CompiledText(oldest)
}

func TestFreshnessIsTheOldestCompileStampNotADateInTheRows(t *testing.T) {
	set := compiledDatasets(t, map[string]compiledDataset{
		intel.NameCVE: {"2026-09-24T13:00:00Z", []string{strings.Join([]string{
			"CVE-2021-44228", "2021-12-10", "2022-01-01", "10.0", "Critical", "CVSS:3.1/AV:N", "", "Log4Shell",
		}, "\t")}},
		intel.NameEPSS: {"2026-09-23T06:30:00Z", []string{"CVE-2021-44228\t0.975\t0.9998\t2020-01-01"}},
		intel.NameKEV: {"2026-09-24T01:00:00Z", []string{strings.Join([]string{
			"CVE-2021-44228", "2019-12-10", "2019-12-24", "Known", "Apache Log4j2", "Apply updates.",
		}, "\t")}},
	})

	d := Describe(KindCVE, "CVE-2021-44228", set)

	if got := oldestOf(t, d); got != "2026-09-23 06:30 UTC" {
		t.Errorf("the panel is current as of %q, want the EPSS file's compile time", got)
	}
	if len(d.Freshness) != 3 {
		t.Errorf("the sources are %+v, want the three installed datasets", d.Freshness)
	}
}

func TestFreshnessCountsOnlyTheDatasetsThatAnswerTheKind(t *testing.T) {
	set := compiledDatasets(t, map[string]compiledDataset{
		intel.NameEPSS:    {"2020-01-01T00:00:00Z", []string{"CVE-2021-44228\t0.975\t0.9998\t2026-09-01"}},
		intel.NameMalware: {"2026-09-20T00:00:00Z", []string{}},
	})

	d := Describe(KindHash, strings.Repeat("a", 64), set)

	if got := oldestOf(t, d); got != "2026-09-20 00:00 UTC" {
		t.Errorf("a hash is current as of %q, want the malware file's time and not the EPSS file's", got)
	}
}

func TestFreshnessAddsTheWatchlistWhenItIsInstalled(t *testing.T) {
	set := compiledDatasets(t, map[string]compiledDataset{
		intel.NameWatchlist: {"2026-01-01T00:00:00Z", []string{}},
	})

	d := Describe(KindHash, strings.Repeat("a", 64), set)

	if got := oldestOf(t, d); got != "2026-01-01 00:00 UTC" {
		t.Errorf("the watchlist's compile time is not counted: %q", got)
	}
}

func TestFreshnessSkipsAStampWithNoDate(t *testing.T) {
	set := compiledDatasets(t, map[string]compiledDataset{
		intel.NameMalware: {"yesterday", []string{}},
	})

	if got := Describe(KindHash, strings.Repeat("a", 64), set).Freshness; len(got) != 0 {
		t.Errorf("an undated stamp became a source: %+v", got)
	}
}

func TestNoDatasetMeansNoFreshness(t *testing.T) {
	if got := Describe(KindCVE, "CVE-2021-44228", nil).Freshness; len(got) != 0 {
		t.Errorf("a panel with nothing installed claims a date: %+v", got)
	}
}

func TestTheCatalogsCarryTheirCompileTime(t *testing.T) {
	for name, kind := range map[string]Kind{"attack": KindAttack, "cwe": KindCWE} {
		value := "T1059"
		if kind == KindCWE {
			value = "CWE-79"
		}
		d := Describe(kind, value, nil)
		if len(d.Freshness) != 1 || d.Freshness[0].Compiled.IsZero() {
			t.Errorf("the %s catalog carries no compile time: %+v", name, d.Freshness)
		}
	}
}

func TestACatalogStampIsReadAndAnUnstampedCatalogStillLoads(t *testing.T) {
	header := "id\tname\tabstraction\tstatus\tsummary\tparents\nCWE-1\tOne\tBase\tDraft\tA weakness.\t\n"
	stamped := intel.SchemaPrefix + "1\tcwe\t2026-09-24T13:51:39Z\tcwe-1000.csv\n" + header

	if got := catalogCompiled(stamped); !got.Equal(time.Date(2026, 9, 24, 13, 51, 39, 0, time.UTC)) {
		t.Errorf("the stamp reads as %v", got)
	}
	if got := catalogCompiled(header); !got.IsZero() {
		t.Errorf("an unstamped catalog claims %v", got)
	}
	for _, source := range []string{stamped, header} {
		if parsed, err := parseWeaknesses(source); err != nil || len(parsed) != 1 {
			t.Errorf("the catalog did not load: %v %v", parsed, err)
		}
	}
}

func TestThePageSaysWhenItsDataWasCompiled(t *testing.T) {
	body := renderBody(Describe(KindCWE, "CWE-79", nil))

	if !strings.Contains(body, "Current as of "+CompiledText(cweCompiled)) {
		t.Errorf("the page does not say when its data was compiled")
	}
}
