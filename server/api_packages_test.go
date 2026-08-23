package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/mattermost/mattermost/server/public/model"

	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/errcode"
)

const packageAdminUserID = "admin1"

// packageAPIPlugin is a plugin whose only configured thing is a package
// directory, which is all the /api/v1/packages routes read. admin is what
// HasPermissionTo will answer.
func packageAPIPlugin(t *testing.T, admin bool) (*Plugin, *fakeAPI, string) {
	t.Helper()

	dir := t.TempDir()
	api := &fakeAPI{permitted: admin}
	p := &Plugin{}
	p.SetAPI(api)
	p.setConfiguration(&configuration{LocationMapPackagesDir: dir})

	return p, api, dir
}

func decodePackages(t *testing.T, rec *httptest.ResponseRecorder) packagesResponse {
	t.Helper()

	var payload packagesResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("cannot decode %s: %v", rec.Body.String(), err)
	}

	return payload
}

/*
 * The list carries two arrays because the System Console needs both: which
 * areas exist, and which of them a button may remove. A bundled area belongs in
 * the first and never in the second.
 */
func TestPackageListNamesEveryAreaAndWhichAreRemovable(t *testing.T) {
	p, _ := bundledAndDropIn(t, "indopacom-hawaii", "indopacom-guam")

	rec := call(p, http.MethodGet, packagesPathAPI, testUserID, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}

	payload := decodePackages(t, rec)
	if want := []string{"indopacom-guam", "indopacom-hawaii"}; !slices.Equal(payload.Packages, want) {
		t.Errorf("packages = %v, want %v", payload.Packages, want)
	}
	if want := []string{"indopacom-guam"}; !slices.Equal(payload.Removable, want) {
		t.Errorf("removable = %v, want %v", payload.Removable, want)
	}
}

// Both arrays are always arrays. A nil would reach the console as null, which
// is a different thing to iterate over than an empty list.
func TestAnInstallWithNoPackagesSendsEmptyArraysRatherThanNull(t *testing.T) {
	p, _, _ := packageAPIPlugin(t, false)

	rec := call(p, http.MethodGet, packagesPathAPI, testUserID, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	if got := strings.TrimSpace(rec.Body.String()); got != `{"packages":[],"removable":[]}` {
		t.Errorf("body = %s", got)
	}
}

func TestPackageListIsCachedForTheSameMinuteTheWebappCachesIt(t *testing.T) {
	p, _, _ := packageAPIPlugin(t, false)

	rec := call(p, http.MethodGet, packagesPathAPI, testUserID, "")
	if got, want := rec.Header().Get("Cache-Control"), "private, max-age=60"; got != want {
		t.Errorf("Cache-Control = %q, want %q", got, want)
	}
}

func TestPackageListNeedsASession(t *testing.T) {
	p, _, _ := packageAPIPlugin(t, false)

	rec := call(p, http.MethodGet, packagesPathAPI, "", "")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
	assertCode(t, rec.Body.String(), errcode.APINotAuthorized)
}

func TestPackageListRejectsANonGet(t *testing.T) {
	p, _, _ := packageAPIPlugin(t, false)

	rec := call(p, http.MethodPost, packagesPathAPI, testUserID, "")
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want 405", rec.Code)
	}
	assertCode(t, rec.Body.String(), errcode.APIMethodNotAllowed)
}

/*
 * Checked here rather than trusted from the fact that the request came from the
 * System Console, because the console is a client and a client is where a
 * request claims to be from rather than where it is from.
 */
func TestPackageAdminRefusesANonAdmin(t *testing.T) {
	p, api, dir := packageAPIPlugin(t, false)

	rec := call(p, http.MethodDelete, packagesPathAPI+"/indopacom-hawaii", testUserID, "")
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", rec.Code)
	}
	assertCode(t, rec.Body.String(), errcode.APINotAuthorized)

	if len(api.permissionsAsked) != 1 || api.permissionsAsked[0] != model.PermissionManageSystem {
		t.Errorf("permissions asked = %v, want one PermissionManageSystem", api.permissionsAsked)
	}

	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 0 {
		t.Errorf("the refused request still touched the directory: %v, %v", entries, err)
	}
}

func TestUploadingAPackageInstallsItAndReturnsTheNewList(t *testing.T) {
	p, api, dir := packageAPIPlugin(t, true)

	rec := call(p, http.MethodPost, packagesPathAPI+"/indopacom-hawaii",
		packageAdminUserID, string(realPackage(t)))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}

	payload := decodePackages(t, rec)
	if want := []string{"indopacom-hawaii"}; !slices.Equal(payload.Packages, want) {
		t.Errorf("packages = %v, want %v", payload.Packages, want)
	}
	if want := []string{"indopacom-hawaii"}; !slices.Equal(payload.Removable, want) {
		t.Errorf("removable = %v, want %v", payload.Removable, want)
	}

	if _, err := os.Stat(filepath.Join(dir, "indopacom-hawaii"+packageSuffix)); err != nil {
		t.Errorf("the archive did not land in the directory: %v", err)
	}

	// An install is an administrative act on shared storage, so it is worth a
	// line naming who did it rather than only appearing as a new row.
	if len(api.infos) != 1 {
		t.Errorf("logged %v, want one line recording the install", api.infos)
	}
}

// A refused upload leaves nothing behind: the archive is validated before it is
// renamed into place, so the directory an operator reads is never a directory
// this route half-wrote.
func TestARefusedUploadSaysWhyAndIsLogged(t *testing.T) {
	p, api, dir := packageAPIPlugin(t, true)

	rec := call(p, http.MethodPost, packagesPathAPI+"/indopacom-hawaii",
		packageAdminUserID, "<html>a captive portal</html>")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
	assertCode(t, rec.Body.String(), errcode.PackagesUploadNotAnArchive)

	if len(api.warnings) != 1 {
		t.Errorf("logged %v, want one line saying why the upload was refused", api.warnings)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("cannot read the package directory: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("the refused upload left %v behind", entries)
	}
}

func TestUploadingUnderANameThatIsNotAnAreaIsRefused(t *testing.T) {
	p, _, _ := packageAPIPlugin(t, true)

	rec := call(p, http.MethodPost, packagesPathAPI+"/Indopacom_Hawaii",
		packageAdminUserID, string(realPackage(t)))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
	assertCode(t, rec.Body.String(), errcode.PackagesUploadBadName)
}

func TestRemovingAPackageReturnsTheNewList(t *testing.T) {
	p, api, dir := packageAPIPlugin(t, true)
	path := writePackage(t, dir, "indopacom-hawaii"+packageSuffix, realPackage(t))

	rec := call(p, http.MethodDelete, packagesPathAPI+"/indopacom-hawaii", packageAdminUserID, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}

	if payload := decodePackages(t, rec); len(payload.Packages) != 0 {
		t.Errorf("packages = %v, want none once the only area is gone", payload.Packages)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("the archive is still on disk: %v", err)
	}
	if len(api.infos) != 1 {
		t.Errorf("logged %v, want one line recording the removal", api.infos)
	}
}

func TestRemovingAPackageThisInstallDoesNotHaveSaysSo(t *testing.T) {
	p, _, _ := packageAPIPlugin(t, true)

	rec := call(p, http.MethodDelete, packagesPathAPI+"/indopacom-hawaii", packageAdminUserID, "")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
	assertCode(t, rec.Body.String(), errcode.PackagesUploadBadName)
}

func TestPackageAdminRejectsAnythingElse(t *testing.T) {
	p, _, _ := packageAPIPlugin(t, true)

	for _, method := range []string{http.MethodGet, http.MethodPut} {
		t.Run(method, func(t *testing.T) {
			rec := call(p, method, packagesPathAPI+"/indopacom-hawaii", packageAdminUserID, "")
			if rec.Code != http.StatusMethodNotAllowed {
				t.Fatalf("status = %d, want 405", rec.Code)
			}
			assertCode(t, rec.Body.String(), errcode.APIMethodNotAllowed)
		})
	}
}

// The admin routes are behind the same session gate as everything else on the
// API, and the gate runs before the permission check.
func TestPackageAdminNeedsASession(t *testing.T) {
	p, api, _ := packageAPIPlugin(t, true)

	rec := call(p, http.MethodDelete, packagesPathAPI+"/indopacom-hawaii", "", "")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
	if len(api.permissionsAsked) != 0 {
		t.Errorf("a request with no session reached the permission check")
	}
}
