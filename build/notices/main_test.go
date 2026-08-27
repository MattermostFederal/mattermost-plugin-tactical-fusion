package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func write(t *testing.T, path, body string) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestALicenseFileIsFoundWhateverItIsCalled(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{
		"LICENSE", "LICENSE.txt", "LICENSE.md", "LICENCE", "COPYING",
		"NOTICE", "NOTICE.txt", "LICENSE-MIT", "LICENSE.libyaml", "copying.txt",
	} {
		write(t, filepath.Join(dir, name), "terms of "+name)
	}

	found, err := noticesIn(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(found) != 10 {
		var names []string
		for _, n := range found {
			names = append(names, n.File)
		}
		t.Errorf("found %d notice file(s) %v, want all 10", len(found), names)
	}
}

func TestASourceFileNamedLikeALicenseIsNotOne(t *testing.T) {
	// github.com/lib/pq ships notice.go and notice_test.go beside its real
	// LICENSE. Reading them put Go source into the notices file.
	dir := t.TempDir()
	write(t, filepath.Join(dir, "LICENSE"), "the real terms")
	for _, name := range []string{"notice.go", "notice_test.go", "license.ts", "licenses.json"} {
		write(t, filepath.Join(dir, name), "package main")
	}

	found, err := noticesIn(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(found) != 1 || found[0].File != "LICENSE" {
		t.Errorf("found %v, want only LICENSE", found)
	}
}

func TestAnEmptyLicenseFileCountsAsNoLicenseAtAll(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, "LICENSE"), "   \n\n")

	found, err := noticesIn(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(found) != 0 {
		t.Errorf("an empty license file was counted as text: %v", found)
	}
}

func TestTheMainModuleIsNotListedAsThirdParty(t *testing.T) {
	// The plugin's own module is the one entry go list reports with no version,
	// and its code is neither third-party nor open source.
	dir := t.TempDir()
	dep := filepath.Join(dir, "dep")
	write(t, filepath.Join(dep, "LICENSE"), "MIT terms")

	list := filepath.Join(dir, "modules.txt")
	write(t, list, "example.com/plugin||"+dir+"\nexample.com/dep|v1.2.3|"+dep+"\n")

	deps, err := readGoModules(list)
	if err != nil {
		t.Fatal(err)
	}
	if len(deps) != 1 || deps[0].Name != "example.com/dep" {
		t.Fatalf("read %v, want only the versioned dependency", deps)
	}
	if deps[0].Version != "v1.2.3" {
		t.Errorf("version = %q, want v1.2.3", deps[0].Version)
	}
}

func TestAComponentWithNoLicenseTextIsRefusedRatherThanOmitted(t *testing.T) {
	deps := []dependency{{Name: "bare", Version: "1.0.0", Ecosystem: "npm package"}}

	_, err := applyFallbacks(policy{}, deps)
	if err == nil {
		t.Fatal("a component with no license text was written into the notices file silently")
	}
	for _, want := range []string{"bare", "noticeFallbacks"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the refusal never mentions %q: %v", want, err)
		}
	}
}

func TestARecordedFallbackStandsInForMissingText(t *testing.T) {
	deps := []dependency{{Name: "bare", Version: "1.0.0", Ecosystem: "npm package"}}
	pol := policy{NoticeFallbacks: []fallback{{Component: "bare", License: "MIT", Text: "stated in package.json"}}}

	out, err := applyFallbacks(pol, deps)
	if err != nil {
		t.Fatal(err)
	}
	if out[0].Fallback != "stated in package.json" {
		t.Errorf("fallback = %q, want the recorded text", out[0].Fallback)
	}
}

func TestAFallbackForSomethingThatNoLongerNeedsOneIsRefused(t *testing.T) {
	// Otherwise a hand-written stand-in outlives the gap it filled, and keeps
	// standing in front of a license file the package has since started shipping.
	deps := []dependency{{Name: "fine", Version: "1.0.0", Notices: []notice{{File: "LICENSE", Text: "terms"}}}}
	pol := policy{NoticeFallbacks: []fallback{{Component: "gone", License: "MIT", Text: "x"}}}

	if _, err := applyFallbacks(pol, deps); err == nil {
		t.Error("a stale fallback was accepted")
	}
}

func TestTheNoticesFileNamesEveryComponentAndItsTerms(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "THIRD-PARTY-NOTICES.txt")

	deps := []dependency{
		{Name: "github.com/a/one", Version: "v1.0.0", Ecosystem: "Go module",
			Notices: []notice{{File: "LICENSE", Text: "Copyright one. MIT."}}},
		{Name: "github.com/b/two", Version: "v2.0.0", Ecosystem: "Go module",
			Notices: []notice{{File: "LICENSE", Text: "Apache terms"}, {File: "NOTICE", Text: "Copyright two."}}},
		{Name: "three", Version: "3.0.0", Ecosystem: "npm package", Fallback: "declared MIT, no file shipped"},
	}

	if err := writeNotices(out, "test bundle", deps); err != nil {
		t.Fatal(err)
	}

	raw, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	body := string(raw)

	for _, want := range []string{
		"THIRD-PARTY SOFTWARE NOTICES", "test bundle",
		"github.com/a/one v1.0.0", "Copyright one. MIT.",
		"github.com/b/two v2.0.0", "Apache terms", "Copyright two.", "--- NOTICE ---",
		"three 3.0.0", "declared MIT, no file shipped",
		"3 component(s) listed",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("the notices file never mentions %q", want)
		}
	}
}

func TestTheNoticesFileSaysWhatItDoesNotCover(t *testing.T) {
	// The map data and the fonts carry their own notices beside the data they
	// belong to, and a reader who finds only this file has to be told that.
	dir := t.TempDir()
	out := filepath.Join(dir, "n.txt")

	deps := []dependency{{Name: "a", Version: "1", Notices: []notice{{File: "LICENSE", Text: "t"}}}}
	if err := writeNotices(out, "test", deps); err != nil {
		t.Fatal(err)
	}

	raw, _ := os.ReadFile(out)
	for _, want := range []string{"LICENSE-OSM.txt", "fonts/LICENSE.txt", "not open source"} {
		if !strings.Contains(string(raw), want) {
			t.Errorf("the preamble never mentions %q", want)
		}
	}
}
