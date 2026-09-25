package main

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/decorators/cyber/intel"
	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/errcode"
)

func datasetsAdminPlugin(t *testing.T, admin bool) (*Plugin, string) {
	t.Helper()

	dir := t.TempDir()
	p := &Plugin{}
	p.SetAPI(&fakeAPI{permitted: admin})
	p.setConfiguration(&configuration{CyberDatasetsDir: dir})

	return p, dir
}

func decodeDatasets(t *testing.T, body []byte) cyberDatasetsResponse {
	t.Helper()

	var got cyberDatasetsResponse
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("cannot decode %s: %v", body, err)
	}
	return got
}

func TestTheDatasetListIsForSystemAdminsOnly(t *testing.T) {
	p, _ := datasetsAdminPlugin(t, false)

	rec := call(p, http.MethodGet, cyberDatasetsPath, testUserID, "")

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", rec.Code)
	}
	assertCode(t, rec.Body.String(), errcode.APINotAuthorized)
}

func TestTheDatasetListRefusesAnythingButGet(t *testing.T) {
	p, _ := datasetsAdminPlugin(t, true)

	if rec := call(p, http.MethodPost, cyberDatasetsPath, testUserID, ""); rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want 405", rec.Code)
	}
}

func TestTheDatasetListShowsEachFileItsRecordsAndWhatIsMissing(t *testing.T) {
	p, dir := datasetsAdminPlugin(t, true)
	writeKEV(t, dir,
		strings.Join([]string{"CVE-2021-44228", "2021-12-10", "2021-12-24", "Known", "Log4j", "Patch."}, "\t"),
		strings.Join([]string{"CVE-2025-55182", "2025-12-05", "2025-12-12", "Known", "React", "Patch."}, "\t"),
	)
	if err := os.WriteFile(filepath.Join(dir, "notes"+intel.Suffix), []byte("hello\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	rec := call(p, http.MethodGet, cyberDatasetsPath, testUserID, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d (%s)", rec.Code, rec.Body.String())
	}
	if got := rec.Header().Get("Cache-Control"); got != "no-store" {
		t.Errorf("Cache-Control %q", got)
	}

	got := decodeDatasets(t, rec.Body.Bytes())

	if len(got.Datasets) != 1 {
		t.Fatalf("datasets %+v", got.Datasets)
	}
	kev := got.Datasets[0]
	if kev.Name != intel.NameKEV || kev.Records != 2 || kev.Kind != directoryConfigured || kev.Label == "" || kev.Size == 0 {
		t.Fatalf("kev %+v", kev)
	}

	if len(got.Missing) != len(intel.Names)-1 {
		t.Fatalf("missing %+v, want every dataset but kev", got.Missing)
	}
	for _, missing := range got.Missing {
		if missing.Name == intel.NameKEV || missing.Label == "" {
			t.Fatalf("missing %+v", missing)
		}
	}

	if len(got.Skipped) != 1 || !strings.Contains(got.Skipped[0].Reason, "TF-21003") || filepath.Base(got.Skipped[0].Path) != "notes"+intel.Suffix {
		t.Fatalf("skipped %+v", got.Skipped)
	}
	if len(got.Directories) != 1 || got.Directories[0].Kind != directoryConfigured {
		t.Fatalf("directories %+v", got.Directories)
	}
}

func TestAnEmptyDatasetListIsEmptyArraysRatherThanNull(t *testing.T) {
	p, _ := datasetsAdminPlugin(t, true)

	body := call(p, http.MethodGet, cyberDatasetsPath, testUserID, "").Body.String()

	for _, field := range []string{`"datasets":[]`, `"databases":[]`, `"replaced":[]`, `"skipped":[]`} {
		if !strings.Contains(body, field) {
			t.Errorf("%s is not an empty array in %s", field, body)
		}
	}
}
