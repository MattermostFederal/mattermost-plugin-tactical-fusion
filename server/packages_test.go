package main

import (
	"bytes"
	"compress/gzip"
	"encoding/binary"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/mattermost/mattermost/server/public/plugin"
)

// The bytes of the pilot package, which is a real archive this build ships.
// A hand-written header could only ever exercise the rejection branches.
func realPackage(t *testing.T) []byte {
	t.Helper()

	raw, err := os.ReadFile("../public/map/packages/indopacom-hawaii.pmtiles")
	if err != nil {
		t.Skipf("no pilot package to read: %v", err)
	}

	return raw
}

func writePackage(t *testing.T, dir, name string, body []byte) string {
	t.Helper()

	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, body, 0o600); err != nil {
		t.Fatalf("cannot write %s: %v", path, err)
	}

	return path
}

func packagePlugin(t *testing.T, dir string) (*Plugin, *fakeAPI) {
	t.Helper()

	api := &fakeAPI{}
	p := &Plugin{}
	p.SetAPI(api)
	p.setConfiguration(&configuration{LocationMapPackagesDir: dir})

	return p, api
}

/*
 * The name is a whitelist and not a cleaning step, which is what makes
 * traversal impossible rather than merely unlikely. Every rejected case here is
 * a file somebody could put in the directory.
 */
func TestOnlyWellNamedPackagesAreServed(t *testing.T) {
	dir := t.TempDir()
	body := realPackage(t)

	// Alphabetical, because mapPackages sorts and the panel adds one style
	// source per name: an unstable order would reshuffle the layer list on
	// every discovery.
	good := []string{"centcom-persian-gulf", "eucom-baltics", "indopacom-hawaii"}
	bad := []string{
		"hawaii",              // no command
		"INDOPACOM-HAWAII",    // upper case reaches a case-sensitive path
		"indopacom_hawaii",    // separator this pattern does not admit
		"indopacom--hawaii",   // empty segment
		"-hawaii",             // leading separator
		"hawaii-",             // trailing separator
		"indopacom-hawaii.v2", // a dot is what a traversal needs
	}

	for _, name := range append(append([]string{}, good...), bad...) {
		writePackage(t, dir, name+".pmtiles", body)
	}
	writePackage(t, dir, "notes.txt", []byte("not an archive"))

	p, _ := packagePlugin(t, dir)
	got := p.packageNames()

	if strings.Join(got, ",") != strings.Join(good, ",") {
		t.Errorf("packages = %v, want %v", got, good)
	}
}

/*
 * A package that is not the archive it claims to be is dropped at discovery
 * rather than served and left to fail in a browser. A client cannot report what
 * it found: a bad archive draws nothing, and the reader sees an area that is
 * simply missing.
 */
func TestPackagesThatAreNotArchivesAreDropped(t *testing.T) {
	body := realPackage(t)

	shallow := append([]byte(nil), body[:127]...)
	shallow[100] = seamZoom - 1

	raster := append([]byte(nil), body[:127]...)
	raster[99] = 0

	cases := []struct {
		name string
		body []byte
	}{
		{"eucom-html", []byte("<html>a captive portal</html>")},
		{"eucom-truncated", body[:40]},
		{"eucom-shallow", shallow},
		{"eucom-raster", raster},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			writePackage(t, dir, tc.name+".pmtiles", tc.body)

			p, api := packagePlugin(t, dir)

			if got := p.packageNames(); len(got) != 0 {
				t.Errorf("packages = %v, want none", got)
			}
			if len(api.warnings) == 0 {
				t.Error("nothing was logged, so an operator has no way to learn why " +
					"their area never appeared")
			}
		})
	}
}

// The directory wins, or a shipped package could never be replaced without a
// plugin release, which is most of what the directory is for.
func TestADroppedInPackageReplacesABundledOne(t *testing.T) {
	body := realPackage(t)

	bundle := t.TempDir()
	if err := os.MkdirAll(filepath.Join(bundle, bundledPackageDir), 0o750); err != nil {
		t.Fatalf("cannot build a fake bundle: %v", err)
	}
	bundled := writePackage(t, filepath.Join(bundle, bundledPackageDir), "indopacom-hawaii.pmtiles", body)

	dir := t.TempDir()
	dropped := writePackage(t, dir, "indopacom-hawaii.pmtiles", body)

	api := &fakeAPI{bundlePath: bundle}
	p := &Plugin{}
	p.SetAPI(api)
	p.setConfiguration(&configuration{LocationMapPackagesDir: dir})

	packages := p.mapPackages()
	if len(packages) != 1 {
		t.Fatalf("packages = %v, want exactly one", packages)
	}
	if packages[0].path == bundled {
		t.Error("the bundled copy won, so an operator cannot replace a shipped area")
	}
	if packages[0].path != dropped {
		t.Errorf("path = %q, want %q", packages[0].path, dropped)
	}
}

/*
 * The route answers byte ranges, which is not a nicety: PMTiles is read by
 * range, and the client refuses a 200 that carries the whole archive because
 * the pmtiles reader applies the same test to every tile and throws. A route
 * that ignored Range would pass a smoke test and then fail every tile.
 */
func TestPackageRouteAnswersByteRanges(t *testing.T) {
	dir := t.TempDir()
	body := realPackage(t)
	writePackage(t, dir, "indopacom-hawaii.pmtiles", body)

	p, _ := packagePlugin(t, dir)

	req := httptest.NewRequest(http.MethodGet, "/packages/indopacom-hawaii.pmtiles", nil)
	req.Header.Set("Range", "bytes=0-126")
	rec := httptest.NewRecorder()
	p.ServeHTTP(&plugin.Context{}, rec, req)

	if rec.Code != http.StatusPartialContent {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusPartialContent)
	}
	if got := rec.Body.Len(); got != 127 {
		t.Errorf("body = %d bytes, want 127", got)
	}
	if got := rec.Body.String()[:7]; got != "PMTiles" {
		t.Errorf("body starts %q, want the archive header", got)
	}
	if got := rec.Header().Get("Content-Range"); got != "bytes 0-126/"+strconv.Itoa(len(body)) {
		t.Errorf("Content-Range = %q", got)
	}
}

// No session, deliberately: MapLibre fetches tiles from a worker, so a route
// that could redirect to a login would answer a tile request with an HTML page
// and the reader would see a map that half drew.
func TestPackageRouteNeedsNoSession(t *testing.T) {
	dir := t.TempDir()
	writePackage(t, dir, "indopacom-hawaii.pmtiles", realPackage(t))

	p, _ := packagePlugin(t, dir)

	req := httptest.NewRequest(http.MethodGet, "/packages/indopacom-hawaii.pmtiles", nil)
	rec := httptest.NewRecorder()
	p.ServeHTTP(&plugin.Context{}, rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d; a tile request cannot follow a redirect to a login",
			rec.Code, http.StatusOK)
	}
}

func TestPackageRouteRefusesAnythingElse(t *testing.T) {
	dir := t.TempDir()
	writePackage(t, dir, "indopacom-hawaii.pmtiles", realPackage(t))

	p, _ := packagePlugin(t, dir)

	paths := []string{
		"/packages/",
		"/packages/indopacom-hawaii",
		"/packages/eucom-baltics.pmtiles",
		"/packages/../plugin.json",
		"/packages/nested/indopacom-hawaii.pmtiles",
	}

	for _, path := range paths {
		t.Run(path, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, path, nil)
			rec := httptest.NewRecorder()
			p.ServeHTTP(&plugin.Context{}, rec, req)

			if rec.Code != http.StatusNotFound {
				t.Errorf("status = %d, want %d", rec.Code, http.StatusNotFound)
			}
		})
	}
}

/*
 * The seam is one number in three languages: this constant, SEAM_ZOOM in
 * map/span.ts, and MINZ in the generator. The generator half is held by
 * TestDetailPackagesStartAtTheSeam, which can see the archive; this is the half
 * that can see the Go constant.
 */
func TestSeamZoomMatchesTheWebapp(t *testing.T) {
	raw, err := os.ReadFile("../webapp/src/decorators/location/map/span.ts")
	if err != nil {
		t.Fatalf("cannot read span.ts: %v", err)
	}

	m := regexp.MustCompile(`const SEAM_ZOOM = ([0-9]+);`).FindSubmatch(raw)
	if m == nil {
		t.Fatal("no `const SEAM_ZOOM` in map/span.ts; if it was renamed, point this " +
			"test at the new name rather than deleting it")
	}

	want, err := strconv.Atoi(string(m[1]))
	if err != nil {
		t.Fatalf("SEAM_ZOOM is not a number: %v", err)
	}

	if seamZoom != want {
		t.Errorf("seamZoom is %d and SEAM_ZOOM is %d. The server would then accept an "+
			"archive the style cannot draw, or refuse one it can", seamZoom, want)
	}
}

/*
 * Discovery is memoised because it was on the tile path, and the memo is only
 * safe while a write is visible through it. An upload the System Console cannot
 * see for five seconds reads as a failed upload.
 */
func TestAnUploadIsVisibleThroughTheMemoAtOnce(t *testing.T) {
	dir := t.TempDir()
	p, _ := packagePlugin(t, dir)

	if got := len(p.mapPackages()); got != 0 {
		t.Fatalf("a fresh directory holds %d packages, want 0", got)
	}

	if err := p.installPackage("indopacom-hawaii", strings.NewReader(string(realPackage(t)))); err != nil {
		t.Fatalf("cannot install: %v", err)
	}

	names := p.packageNames()
	if len(names) != 1 || names[0] != "indopacom-hawaii" {
		t.Errorf("after an upload the memo still answers %v, want [indopacom-hawaii]", names)
	}
}

func TestARemovalIsVisibleThroughTheMemoAtOnce(t *testing.T) {
	dir := t.TempDir()
	p, _ := packagePlugin(t, dir)
	writePackage(t, dir, "indopacom-hawaii.pmtiles", realPackage(t))

	if got := len(p.mapPackages()); got != 1 {
		t.Fatalf("the directory holds %d packages, want 1", got)
	}

	if err := p.removePackage("indopacom-hawaii"); err != nil {
		t.Fatalf("cannot remove: %v", err)
	}

	if names := p.packageNames(); len(names) != 0 {
		t.Errorf("after a removal the memo still answers %v, want none", names)
	}
}

/*
 * The memo is keyed on the directories as well as the clock, because
 * LocationMapPackagesDir changes under a running plugin and a list from the
 * previous directory would otherwise outlive the setting.
 */
func TestChangingThePackageDirectoryIsNotServedFromTheMemo(t *testing.T) {
	first, second := t.TempDir(), t.TempDir()
	p, _ := packagePlugin(t, first)
	writePackage(t, first, "indopacom-hawaii.pmtiles", realPackage(t))

	if got := len(p.mapPackages()); got != 1 {
		t.Fatalf("the first directory holds %d packages, want 1", got)
	}

	p.setConfiguration(&configuration{LocationMapPackagesDir: second})

	if names := p.packageNames(); len(names) != 0 {
		t.Errorf("after the directory changed the memo still answers %v, want none", names)
	}
}

func packageRequest(t *testing.T, p *Plugin, rangeHeader, ifNoneMatch string) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequest(http.MethodGet, "/packages/indopacom-hawaii.pmtiles", nil)
	if rangeHeader != "" {
		req.Header.Set("Range", rangeHeader)
	}
	if ifNoneMatch != "" {
		req.Header.Set("If-None-Match", ifNoneMatch)
	}
	rec := httptest.NewRecorder()
	p.ServeHTTP(&plugin.Context{}, rec, req)

	return rec
}

/*
 * A package is replaceable, by upload and by copying over it, under a URL that
 * carries only the plugin version. PMTiles offsets read from one file are
 * meaningless against another, so the ETag is the only thing that lets the
 * client notice; pmtiles.js already recovers when it moves.
 */
func TestAReplacedPackageIsServedWithADifferentETag(t *testing.T) {
	dir := t.TempDir()
	body := realPackage(t)
	path := writePackage(t, dir, "indopacom-hawaii.pmtiles", body)

	p, _ := packagePlugin(t, dir)

	before := packageRequest(t, p, "bytes=0-126", "").Header().Get("ETag")
	if before == "" {
		t.Fatal("no ETag, so a client cannot tell one archive from another")
	}

	// Same bytes at a later modification time is the case a digest of the
	// contents would miss and an operator can produce by rebuilding.
	later := time.Now().Add(2 * time.Second)
	if err := os.Chtimes(path, later, later); err != nil {
		t.Fatalf("cannot age the file: %v", err)
	}
	p.forgetPackages()

	if after := packageRequest(t, p, "bytes=0-126", "").Header().Get("ETag"); after == before {
		t.Errorf("ETag is %q before and after the file changed", after)
	}
}

func TestAnUnchangedPackageRevalidatesWithoutResending(t *testing.T) {
	dir := t.TempDir()
	writePackage(t, dir, "indopacom-hawaii.pmtiles", realPackage(t))

	p, _ := packagePlugin(t, dir)

	etag := packageRequest(t, p, "", "").Header().Get("ETag")
	rec := packageRequest(t, p, "", etag)

	if rec.Code != http.StatusNotModified {
		t.Errorf("status = %d, want %d, so a revalidation re-sends the archive", rec.Code, http.StatusNotModified)
	}
	if rec.Body.Len() != 0 {
		t.Errorf("a 304 carried %d bytes", rec.Body.Len())
	}
}

// Bounded to the client's own package-list lifetime, so a replaced archive is
// stale for at most that long rather than the five minutes it was.
func TestThePackageRouteBoundsHowLongAReplacementStaysStale(t *testing.T) {
	dir := t.TempDir()
	writePackage(t, dir, "indopacom-hawaii.pmtiles", realPackage(t))

	p, _ := packagePlugin(t, dir)

	if got := packageRequest(t, p, "", "").Header().Get("Cache-Control"); got != "private, max-age=60" {
		t.Errorf("Cache-Control = %q", got)
	}
}

func bundledAndDropIn(t *testing.T, bundledName, dropInName string) (*Plugin, string) {
	t.Helper()

	body := realPackage(t)

	bundle := t.TempDir()
	if err := os.MkdirAll(filepath.Join(bundle, bundledPackageDir), 0o750); err != nil {
		t.Fatalf("cannot build a fake bundle: %v", err)
	}
	if bundledName != "" {
		writePackage(t, filepath.Join(bundle, bundledPackageDir), bundledName+".pmtiles", body)
	}

	dir := t.TempDir()
	if dropInName != "" {
		writePackage(t, dir, dropInName+".pmtiles", body)
	}

	p := &Plugin{}
	p.SetAPI(&fakeAPI{bundlePath: bundle})
	p.setConfiguration(&configuration{LocationMapPackagesDir: dir})

	return p, dir
}

/*
 * Remove is offered for what Remove can do. A bundled area is inside the plugin
 * and a release replaces it, so listing it beside a drop-in with the same button
 * surfaced os.Remove's ENOENT to an admin as "the package could not be removed".
 */
func TestOnlyADroppedInPackageIsRemovable(t *testing.T) {
	p, _ := bundledAndDropIn(t, "indopacom-hawaii", "indopacom-guam")

	if got, want := p.packageNames(), []string{"indopacom-guam", "indopacom-hawaii"}; !slices.Equal(got, want) {
		t.Fatalf("packages = %v, want %v", got, want)
	}
	if got, want := p.removablePackages(), []string{"indopacom-guam"}; !slices.Equal(got, want) {
		t.Errorf("removable = %v, want %v", got, want)
	}
}

func TestRemovingABundledPackageSaysWhyRatherThanFailing(t *testing.T) {
	p, _ := bundledAndDropIn(t, "indopacom-hawaii", "")

	err := p.removePackage("indopacom-hawaii")
	if err == nil {
		t.Fatal("removing a bundled package reported success")
	}
	if !strings.Contains(err.Error(), "ships inside the plugin") {
		t.Errorf("error = %q, which does not tell an admin why", err)
	}
}

/*
 * Removing a drop-in that shadows a bundled package of the same name succeeds
 * and the name stays listed, because the bundled one underneath resurfaces.
 * That reads as a failed delete unless the row stops offering Remove, which is
 * what dropping out of removablePackages does.
 */
func TestRemovingAShadowingPackageLeavesTheBundledOneListedButNotRemovable(t *testing.T) {
	p, _ := bundledAndDropIn(t, "indopacom-hawaii", "indopacom-hawaii")

	if got := p.removablePackages(); !slices.Equal(got, []string{"indopacom-hawaii"}) {
		t.Fatalf("removable = %v before the removal", got)
	}

	if err := p.removePackage("indopacom-hawaii"); err != nil {
		t.Fatalf("cannot remove the shadowing package: %v", err)
	}

	if got := p.packageNames(); !slices.Equal(got, []string{"indopacom-hawaii"}) {
		t.Errorf("packages = %v, want the bundled one still listed", got)
	}
	if got := p.removablePackages(); len(got) != 0 {
		t.Errorf("removable = %v, want none once only the bundled one is left", got)
	}
}

func readWebappFile(t *testing.T, parts ...string) string {
	t.Helper()

	path := filepath.Join(append([]string{"..", "webapp", "src"}, parts...)...)
	raw, err := os.ReadFile(path) // #nosec G304 -- fixed, repo-relative source path
	if err != nil {
		t.Fatalf("could not read %s: %v", path, err)
	}

	return string(raw)
}

/*
 * The package name is a whitelist in two languages, and it reaches a URL from
 * one and a filesystem path from the other. Agreeing only by accident is a
 * request for something else, or a file this server serves and that client
 * refuses.
 */
func TestWebappPackageNameGrammarMatches(t *testing.T) {
	source := readWebappFile(t, "decorators", "location", "map", "basemap.ts")

	m := regexp.MustCompile(`export const PACKAGE_NAME = /(.+)/;`).FindStringSubmatch(source)
	if m == nil {
		t.Fatal("could not find PACKAGE_NAME in basemap.ts; if it was renamed, point this " +
			"test at the new name rather than deleting it")
	}

	if got, want := m[1], packageNamePattern.String(); got != want {
		t.Errorf("the webapp accepts %q and this server %q", got, want)
	}
}

// The list is cached on both sides, and the client waiting longer than the
// server means an operator watching for a file they just copied waits twice.
func TestWebappPackageCacheLifetimeMatches(t *testing.T) {
	source := readWebappFile(t, "packages", "store.ts")

	m := regexp.MustCompile(`const CACHE_TTL_MS = (\d+) \* 1000;`).FindStringSubmatch(source)
	if m == nil {
		t.Fatal("could not find CACHE_TTL_MS in the webapp package store; if it was renamed " +
			"or written differently, point this test at the new form rather than deleting it")
	}

	seconds, err := strconv.Atoi(m[1])
	if err != nil {
		t.Fatalf("unparsable webapp cache lifetime %q: %v", m[1], err)
	}

	if seconds != packageListCacheSeconds {
		t.Errorf("the webapp caches the package list for %ds and this server sends max-age=%d",
			seconds, packageListCacheSeconds)
	}
}

// Rewrites an archive's metadata name, which is where build.sh stamps the map
// schema. The new blob is appended and the header repointed at it, so the tile
// data behind it never moves and the length is free to change.
func restamp(t *testing.T, path, from, to string) {
	t.Helper()

	raw, err := os.ReadFile(path) // #nosec G304 -- a temp file this test wrote
	if err != nil {
		t.Fatalf("cannot read %s: %v", path, err)
	}

	offset := binary.LittleEndian.Uint64(raw[24:32])
	length := binary.LittleEndian.Uint64(raw[32:40])
	plain, err := io.ReadAll(mustGunzip(t, raw[offset:offset+length]))
	if err != nil {
		t.Fatalf("cannot read metadata: %v", err)
	}

	swapped := strings.Replace(string(plain), from, to, 1)
	if swapped == string(plain) {
		t.Fatalf("metadata does not contain %q", from)
	}

	var out bytes.Buffer
	zw := gzip.NewWriter(&out)
	if _, err := zw.Write([]byte(swapped)); err != nil {
		t.Fatalf("cannot compress metadata: %v", err)
	}
	_ = zw.Close()

	// The rewritten blob is appended and the header repointed at it, which is
	// simpler than fitting it back into the original span.
	binary.LittleEndian.PutUint64(raw[24:32], uint64(len(raw)))
	binary.LittleEndian.PutUint64(raw[32:40], uint64(out.Len()))
	if err := os.WriteFile(path, append(raw, out.Bytes()...), 0o600); err != nil {
		t.Fatalf("cannot write %s: %v", path, err)
	}
}

func mustGunzip(t *testing.T, raw []byte) io.Reader {
	t.Helper()

	zr, err := gzip.NewReader(bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("metadata is not gzip: %v", err)
	}

	return zr
}

/*
 * Areas are portable across plugin versions, which is the point: an operator
 * upgrading should not have to move gigabytes of map data. The stamp exists
 * only for the change that makes an old archive wrong rather than shallower.
 */
func TestAnArchiveWithNoStampIsTheOriginalSchema(t *testing.T) {
	dir := t.TempDir()
	path := writePackage(t, dir, "indopacom-hawaii.pmtiles", realPackage(t))
	p, _ := packagePlugin(t, dir)

	// Every archive published before the stamp existed looks like this.
	if err := validPackageHeader(path); err != nil {
		t.Fatalf("an unstamped archive was rejected: %v", err)
	}
	if got := len(p.mapPackages()); got != 1 {
		t.Errorf("discovered %d packages, want 1", got)
	}
}

func TestAnArchiveStampedForThisSchemaIsAccepted(t *testing.T) {
	dir := t.TempDir()
	path := writePackage(t, dir, "indopacom-hawaii.pmtiles", realPackage(t))
	restamp(t, path, `"OpenMapTiles"`, `"`+schemaPrefix+`1"`)

	if err := validPackageHeader(path); err != nil {
		t.Fatalf("an archive stamped for schema %d was rejected: %v", mapSchemaVersion, err)
	}
}

func TestAnArchiveFromANewerSchemaSaysToUpgradeThePlugin(t *testing.T) {
	dir := t.TempDir()
	path := writePackage(t, dir, "indopacom-hawaii.pmtiles", realPackage(t))
	restamp(t, path, `"OpenMapTiles"`, `"`+schemaPrefix+`9"`)

	err := validPackageHeader(path)
	if err == nil {
		t.Fatal("an archive from a newer map schema was accepted")
	}
	if !strings.Contains(err.Error(), "upgrade the plugin") {
		t.Errorf("error = %q, which does not say which side is behind", err)
	}

	p, _ := packagePlugin(t, dir)
	if got := p.mapPackages(); len(got) != 0 {
		t.Errorf("discovery served %v despite the schema mismatch", got)
	}
}

// The generator writes the stamp and this package checks it, so the two
// constants are one value in two languages.
func TestMapSchemaMatchesTheGenerator(t *testing.T) {
	raw, err := os.ReadFile("../build/maposm/build.sh") // #nosec G304 -- a fixed, repo-relative path
	if err != nil {
		t.Fatalf("cannot read the generator: %v", err)
	}

	m := regexp.MustCompile(`MAP_SCHEMA="\$\{MAP_SCHEMA:-(\d+)\}"`).FindStringSubmatch(string(raw))
	if m == nil {
		t.Fatal("no MAP_SCHEMA default in build/maposm/build.sh; if it was renamed, point " +
			"this test at the new name rather than deleting it")
	}

	built, err := strconv.Atoi(m[1])
	if err != nil {
		t.Fatalf("unparsable MAP_SCHEMA %q: %v", m[1], err)
	}

	if built != mapSchemaVersion {
		t.Errorf("the generator stamps schema %d and this server draws %d", built, mapSchemaVersion)
	}

	if !strings.Contains(string(raw), "SCHEMA_PREFIX="+strings.TrimSuffix(schemaPrefix, "/")) {
		t.Errorf("the generator does not stamp the %q prefix this server reads", schemaPrefix)
	}
}
