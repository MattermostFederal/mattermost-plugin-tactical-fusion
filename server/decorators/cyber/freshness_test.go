package cyber

import (
	"fmt"
	"maps"
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

func sourceDates(d Details) map[string]string {
	dates := map[string]string{}
	for _, source := range d.Freshness {
		dates[source.File] = CompiledText(source.Compiled)
	}
	return dates
}

func TestFreshnessIsEachFilesCompileStampNotADateInTheRows(t *testing.T) {
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

	want := map[string]string{
		"cve.tsv":  "2026-09-24 13:00 UTC",
		"epss.tsv": "2026-09-23 06:30 UTC",
		"kev.tsv":  "2026-09-24 01:00 UTC",
	}
	if got := sourceDates(d); !maps.Equal(got, want) {
		t.Errorf("the sources are %v, want %v", got, want)
	}
	if d.Freshness[0].Label != "vulnerability" {
		t.Errorf("the first source is labeled %q", d.Freshness[0].Label)
	}
}

func TestFreshnessCountsOnlyTheDatasetsThatAnswerTheKind(t *testing.T) {
	set := compiledDatasets(t, map[string]compiledDataset{
		intel.NameEPSS:    {"2020-01-01T00:00:00Z", []string{"CVE-2021-44228\t0.975\t0.9998\t2026-09-01"}},
		intel.NameMalware: {"2026-09-20T00:00:00Z", []string{}},
	})

	d := Describe(KindHash, strings.Repeat("a", 64), set)

	if got := sourceDates(d); !maps.Equal(got, map[string]string{"malware.tsv": "2026-09-20 00:00 UTC"}) {
		t.Errorf("a hash lists %v, want the malware file and not the EPSS file", got)
	}
}

func TestFreshnessAddsTheWatchlistWhenItIsInstalled(t *testing.T) {
	set := compiledDatasets(t, map[string]compiledDataset{
		intel.NameWatchlist: {"2026-01-01T00:00:00Z", []string{}},
	})

	d := Describe(KindHash, strings.Repeat("a", 64), set)

	if got := sourceDates(d); !maps.Equal(got, map[string]string{"watchlist.tsv": "2026-01-01 00:00 UTC"}) {
		t.Errorf("the watchlist is not listed: %v", got)
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
		if len(d.Freshness) != 1 || d.Freshness[0].Compiled.IsZero() || d.Freshness[0].File != name+".tsv (built in)" {
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

func TestThePageListsItsDataSources(t *testing.T) {
	body := renderBody(Describe(KindCWE, "CWE-79", nil))

	for _, want := range []string{"<h2>Data sources</h2>", "cwe.tsv (built in)", CompiledText(cweCompiled)} {
		if !strings.Contains(body, want) {
			t.Errorf("the page's data sources lack %q", want)
		}
	}
}
