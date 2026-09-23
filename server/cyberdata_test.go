package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/decorators/cyber"
	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/decorators/cyber/intel"
	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/errcode"
)

func writeKEV(t *testing.T, dir string, rows ...string) {
	t.Helper()

	var body strings.Builder
	body.WriteString(fmt.Sprintf("%s%d\t%s\t2026-09-01T00:00:00Z\ttest\n", intel.SchemaPrefix, intel.SchemaVersion, intel.NameKEV))
	for _, row := range rows {
		body.WriteString(row + "\n")
	}

	if err := os.WriteFile(filepath.Join(dir, "kev"+intel.Suffix), []byte(body.String()), 0o600); err != nil {
		t.Fatalf("writing: %v", err)
	}
}

func withDatasetDir(t *testing.T, dir string) *Plugin {
	t.Helper()

	p := newTestPlugin(t, "https://example.com", true)
	config := p.getConfiguration().Clone()
	config.CyberDatasetsDir = dir
	p.setConfiguration(config)

	return p
}

func TestTheConfiguredDirectoryReachesTheReader(t *testing.T) {
	dir := t.TempDir()
	writeKEV(t, dir, strings.Join([]string{
		"CVE-2021-44228", "2021-12-10", "2021-12-24", "Known", "Apache Log4j2", "Apply updates.",
	}, "\t"))

	p := withDatasetDir(t, dir)

	details := cyber.Describe(cyber.KindCVE, "CVE-2021-44228", p.cyberIntel())

	found := false
	for _, row := range details.Rows {
		if row.Label == "Known exploited" {
			found = true
		}
	}
	if !found {
		t.Fatalf("the dataset in CyberDatasetsDir did not reach the reader: %+v", details.Rows)
	}
}

func TestAnUnsetDirectoryReadsOnlyWhatIsBundled(t *testing.T) {
	p := newTestPlugin(t, "https://example.com", true)

	if dirs := p.cyberDirs(); len(dirs) != 0 {
		t.Fatalf("dirs = %v, want none when there is no bundle path and no setting", dirs)
	}
	if set := p.cyberIntel(); set.Has(intel.NameKEV) {
		t.Fatalf("a dataset was found with nothing configured")
	}
}

func TestADroppedInDatasetIsPickedUp(t *testing.T) {
	dir := t.TempDir()
	p := withDatasetDir(t, dir)

	if p.cyberIntel().Has(intel.NameKEV) {
		t.Fatalf("an empty directory reported a dataset")
	}

	writeKEV(t, dir)
	p.forgetCyberDatasets()

	if !p.cyberIntel().Has(intel.NameKEV) {
		t.Fatalf("a dataset dropped in was not picked up")
	}
}

func TestAConfigurationChangeDropsTheCachedDatasets(t *testing.T) {
	dir := t.TempDir()
	writeKEV(t, dir)

	p := withDatasetDir(t, dir)
	if !p.cyberIntel().Has(intel.NameKEV) {
		t.Fatalf("the configured directory was not read")
	}

	config := p.getConfiguration().Clone()
	config.CyberDatasetsDir = t.TempDir()
	p.setConfiguration(config)
	p.forgetCyberDatasets()

	if p.cyberIntel().Has(intel.NameKEV) {
		t.Fatalf("the old directory still answers after the setting changed")
	}
}

func TestEachUnusableFileReportsItsOwnCode(t *testing.T) {
	cases := map[string]struct {
		write func(dir string)
		code  int
	}{
		"no schema stamp": {
			write: func(dir string) {
				_ = os.WriteFile(filepath.Join(dir, "kev"+intel.Suffix), []byte("CVE-2021-0001\ta\n"), 0o600)
			},
			code: errcode.CyberDataSchemaMismatch,
		},
		"a name this build does not read": {
			write: func(dir string) {
				_ = os.WriteFile(filepath.Join(dir, "notes"+intel.Suffix), []byte("x\n"), 0o600)
			},
			code: errcode.CyberDataBadName,
		},
		"a vendor database that will not open": {
			write: func(dir string) {
				_ = os.WriteFile(filepath.Join(dir, "GeoLite2-ASN"+intel.MMDBSuffix), []byte("nope"), 0o600)
			},
			code: errcode.CyberDataMMDBUnreadable,
		},
		"an archive that will not unpack": {
			write: func(dir string) {
				_ = os.WriteFile(filepath.Join(dir, "cve"+intel.ArchiveSuffix), []byte("not gzip"), 0o600)
			},
			code: errcode.CyberDataUnpackFailed,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			tc.write(dir)

			p := withDatasetDir(t, dir)
			api := p.API.(*fakeAPI)

			p.cyberIntel()

			found := false
			for _, code := range api.warnCodes {
				if code == tc.code {
					found = true
				}
			}
			if !found {
				t.Fatalf("codes %v, want %d", api.warnCodes, tc.code)
			}
		})
	}
}

func TestABrokenFileIsReportedOnceAndTheRestStillAnswer(t *testing.T) {
	dir := t.TempDir()
	writeKEV(t, dir, strings.Join([]string{
		"CVE-2021-44228", "2021-12-10", "2021-12-24", "Known", "Apache Log4j2", "Apply updates.",
	}, "\t"))
	_ = os.WriteFile(filepath.Join(dir, "cve"+intel.Suffix), []byte("no stamp here\n"), 0o600)

	p := withDatasetDir(t, dir)
	api := p.API.(*fakeAPI)

	for i := range 3 {
		p.forgetCyberDatasets()

		set := p.cyberIntel()
		if !set.Has(intel.NameKEV) {
			t.Fatalf("the usable dataset stopped answering on pass %d", i)
		}
		if set.Has(intel.NameCVE) {
			t.Fatalf("the unusable dataset answered on pass %d", i)
		}
	}

	schema := 0
	for _, code := range api.warnCodes {
		if code == errcode.CyberDataSchemaMismatch {
			schema++
		}
	}
	if schema != 1 {
		t.Fatalf("the same broken file was reported %d times, want once", schema)
	}
}
