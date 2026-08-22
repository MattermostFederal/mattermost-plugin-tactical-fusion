package main

import (
	"cmp"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/errcode"
)

// A detail package is one region's OpenStreetMap archive, named
// <command>-<area>. The name reaches a URL and a filesystem path, so it is
// matched against a whitelist rather than cleaned: no dot, no slash and no
// separator survives this, which is what makes traversal impossible rather than
// merely unlikely.
var packageNamePattern = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)+$`)

const (
	packageSuffix = ".pmtiles"

	// Where the seam is. The archive must start exactly here or the two tiers
	// leave a blank band or draw the same road twice; TestSeamZoomMatchesTheWebapp
	// holds this to SEAM_ZOOM in map/span.ts.
	seamZoom = 10

	packageHeaderBytes = 127
	packageMagic       = "PMTiles"
	packageSpecVersion = 3
	packageTileTypeMVT = 1
)

// bundledPackageDir is where packages that ship inside the bundle live,
// relative to the bundle root.
var bundledPackageDir = filepath.Join("public", "map", "packages")

type mapPackage struct {
	Name string `json:"name"`

	// Where the bytes are. Never sent to a client: the client is given a route
	// and this is what that route reads.
	path string

	// Whether this one came from the operator's directory rather than the
	// bundle, which is the same question as whether removePackage can unlink
	// it. A bundled area is inside the plugin and only a release replaces it.
	dropIn bool
}

/*
 * Every detail package this install has, bundled and dropped in, validated.
 *
 * The dropped-in directory is read SECOND and wins on a name collision, so an
 * operator can replace a bundled package with a newer build of the same area
 * without waiting for a plugin release. That is the whole point of the
 * directory existing, and the alternative ordering would make a shipped
 * package permanently unreplaceable.
 *
 * A package that fails validation is dropped and logged rather than served. The
 * failure modes here are an operator's: a half-copied file, an archive built to
 * the wrong depth, a raster archive, or something that is not an archive at all.
 * Every one of them draws nothing and says nothing if it reaches a client, so
 * the log line is the only thing that will ever explain it.
 */
func (p *Plugin) mapPackages() []mapPackage {
	dirs := p.packageDirs()
	if cached := p.cachedPackages(dirs); cached != nil {
		return cached
	}

	found := map[string]mapPackage{}

	// packageDirs returns the bundle first and the operator's directory second,
	// so the later write is what makes a drop-in override a bundled package of
	// the same name, and what makes dropIn true for it.
	dropInDir := strings.TrimSpace(p.getConfiguration().LocationMapPackagesDir)
	for _, dir := range dirs {
		for _, pkg := range p.packagesIn(dir) {
			pkg.dropIn = dropInDir != "" && dir == dropInDir
			found[pkg.Name] = pkg
		}
	}

	packages := make([]mapPackage, 0, len(found))
	for _, pkg := range found {
		packages = append(packages, pkg)
	}
	slices.SortFunc(packages, func(a, b mapPackage) int { return cmp.Compare(a.Name, b.Name) })

	p.storePackages(dirs, packages)

	return packages
}

/*
 * Discovery is memoised, because it is on the tile path.
 *
 * MapLibre asks for one byte range per tile, and every one of those reached
 * servePackage, which read both directories and opened, read and closed every
 * archive to re-validate its header. With the thirteen areas the release ships
 * that is thirteen file opens per tile, and a reader panning a map issues tiles
 * continuously. packageNames is on the same path for every page render.
 *
 * The TTL is what keeps "drop a file in and it appears, with no restart", which
 * is most of what the directory is for. It is far shorter than the client's own
 * minute so an operator never waits on this layer, and installPackage and
 * removePackage drop it outright so the System Console reflects a change at once.
 *
 * Keyed on the directories as well as the clock: LocationMapPackagesDir can
 * change under a running plugin, and a stale list from the previous directory
 * would outlive the setting.
 */
const packageCacheTTL = 5 * time.Second

type packageCache struct {
	dirs     []string
	packages []mapPackage
	at       time.Time
}

func (p *Plugin) cachedPackages(dirs []string) []mapPackage {
	p.packageLock.Lock()
	defer p.packageLock.Unlock()

	c := p.packageCache
	if c == nil || !slices.Equal(c.dirs, dirs) || time.Since(c.at) > packageCacheTTL {
		return nil
	}

	return slices.Clone(c.packages)
}

func (p *Plugin) storePackages(dirs []string, packages []mapPackage) {
	p.packageLock.Lock()
	defer p.packageLock.Unlock()

	p.packageCache = &packageCache{
		dirs:     slices.Clone(dirs),
		packages: slices.Clone(packages),
		at:       time.Now(),
	}
}

// forgetPackages drops the memo, for the two writes that know the directory
// changed and should not wait out the TTL.
func (p *Plugin) forgetPackages() {
	p.packageLock.Lock()
	defer p.packageLock.Unlock()

	p.packageCache = nil
}

/*
 * The packages removePackage can actually unlink.
 *
 * A bundled area lives inside the plugin, so Remove cannot touch it and the
 * System Console should not offer it: it used to, and the attempt surfaced
 * os.Remove's ENOENT to an admin as "the package could not be removed", which
 * reads as a broken feature rather than a thing that was never possible.
 */
func (p *Plugin) removablePackages() []string {
	names := []string{}
	for _, pkg := range p.mapPackages() {
		if pkg.dropIn {
			names = append(names, pkg.Name)
		}
	}

	return names
}

// packageNames is mapPackages reduced to what a client is told.
func (p *Plugin) packageNames() []string {
	packages := p.mapPackages()
	names := make([]string, 0, len(packages))
	for _, pkg := range packages {
		names = append(names, pkg.Name)
	}

	return names
}

// The directories packages are read from, in precedence order: the bundle
// first, the operator's directory second so it overrides.
// logWarn drops the line when there is no API to take it, which packageDirs
// already treats as a reachable state: ServeHTTP can run before OnActivate, and
// a discovery warning is not worth a nil dereference taking the process down.
func (p *Plugin) logWarn(message string, pairs ...any) {
	if p.API == nil {
		return
	}

	p.API.LogWarn(message, pairs...)
}

/*
 * warnOnce keeps a rejected package to one log line rather than one per
 * discovery.
 *
 * Discovery used to run per tile request, so a single malformed file wrote a
 * line for every tile a reader panned past. The memo above cuts that to one per
 * TTL, which over an hour is still hundreds, and the comment on packagesIn
 * promises the operator a line naming the file rather than a stream of them.
 * Keyed by path, so replacing a bad archive with another bad one under the same
 * name is silent until the plugin restarts; the alternative is stat-ing every
 * rejected file on every pass to notice, which is the cost this exists to avoid.
 */
func (p *Plugin) warnOnce(path, message string, pairs ...any) {
	p.packageLock.Lock()
	if p.warned == nil {
		p.warned = map[string]bool{}
	}
	seen := p.warned[path]
	p.warned[path] = true
	p.packageLock.Unlock()

	if !seen {
		p.logWarn(message, pairs...)
	}
}

func (p *Plugin) packageDirs() []string {
	var dirs []string

	// Reachable before OnActivate has run, since ServeHTTP and the page
	// renderers are wired the moment the plugin loads. Without an API there is
	// no bundle to find and nothing to log the failure to, so the honest answer
	// is the configured directory alone.
	if p.API == nil {
		if dir := strings.TrimSpace(p.getConfiguration().LocationMapPackagesDir); dir != "" {
			return []string{dir}
		}

		return nil
	}

	if bundle, err := p.API.GetBundlePath(); err == nil {
		dirs = append(dirs, filepath.Join(bundle, bundledPackageDir))
	} else {
		p.API.LogWarn("cannot locate the plugin bundle, so no bundled map packages are served",
			"error_code", errcode.PackagesNoBundlePath, "error", err.Error())
	}

	if dir := strings.TrimSpace(p.getConfiguration().LocationMapPackagesDir); dir != "" {
		dirs = append(dirs, dir)
	}

	return dirs
}

func (p *Plugin) packagesIn(dir string) []mapPackage {
	entries, err := os.ReadDir(dir)
	if err != nil {
		// Not an error worth a log line on every call: a bundle without the
		// directory is an ordinary global-only build, and an operator who has
		// not created theirs yet has not done anything wrong.
		return nil
	}

	var packages []mapPackage
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name, ok := strings.CutSuffix(entry.Name(), packageSuffix)
		if !ok {
			continue
		}

		path := filepath.Join(dir, entry.Name())
		if !packageNamePattern.MatchString(name) {
			p.warnOnce(path, "ignoring a map package whose name is not <command>-<area>",
				"error_code", errcode.PackagesBadName, "file", path)
			continue
		}

		if err := validPackageHeader(path); err != nil {
			p.warnOnce(path, "ignoring a map package that is not the archive it claims to be",
				"error_code", errcode.PackagesBadArchive, "file", path, "error", err.Error())
			continue
		}

		packages = append(packages, mapPackage{Name: name, path: path})
	}

	return packages
}

// The package with this name, or false. The name is validated here rather than
// trusted from a route, so this is the only way to turn a request into a path.
func (p *Plugin) mapPackage(name string) (mapPackage, bool) {
	if !packageNamePattern.MatchString(name) {
		return mapPackage{}, false
	}

	for _, pkg := range p.mapPackages() {
		if pkg.Name == name {
			return pkg, true
		}
	}

	return mapPackage{}, false
}

/*
 * Whether a file is the archive a reader will assume it is.
 *
 * The same four questions basemap.ts asks on arrival, asked here because a
 * client cannot report what it found: a bad archive draws nothing, and the
 * reader sees an area that is simply missing rather than an error. Checking at
 * discovery is what turns that into one log line naming the file.
 */
func validPackageHeader(path string) error {
	file, err := os.Open(path) // #nosec G304 -- a path built from a whitelisted name
	if err != nil {
		return err
	}
	defer func() { _ = file.Close() }()

	header := make([]byte, packageHeaderBytes)
	if _, err := file.Read(header); err != nil {
		return err
	}

	if string(header[:len(packageMagic)]) != packageMagic {
		return errcode.Errorf(errcode.PackagesBadArchive, "not a PMTiles archive")
	}
	if header[7] != packageSpecVersion {
		return errcode.Errorf(errcode.PackagesBadArchive,
			"PMTiles spec version %d, want %d", header[7], packageSpecVersion)
	}
	if header[99] != packageTileTypeMVT {
		return errcode.Errorf(errcode.PackagesBadArchive,
			"tile type %d, want %d (MVT)", header[99], packageTileTypeMVT)
	}
	if header[100] != seamZoom {
		return errcode.Errorf(errcode.PackagesBadArchive,
			"starts at zoom %d, want %d, which is where the detail tier takes over",
			header[100], seamZoom)
	}

	return nil
}

/*
 * Installing a package from the System Console.
 *
 * This writes into the SAME directory the drop-in path reads, because there is
 * nowhere else the server can range-read from: plugin.API.ReadFile and GetFile
 * both return the whole file, so a package stored through them would be pulled
 * into memory for every tile a reader scrolls past.
 *
 * Two consequences follow and neither is a defect to be fixed here. In a
 * cluster the upload lands on ONE node, so shared storage or a copy per node is
 * required exactly as it is for a file copied by hand. And an upload crosses
 * Mattermost's own request limits and whatever proxy sits in front of it, which
 * a large area will exceed: the directory, not this route, is the mechanism for
 * those.
 */
const maxUploadBytes = 512 << 20

func (p *Plugin) installPackage(name string, body io.Reader) error {
	if !packageNamePattern.MatchString(name) {
		return errcode.Errorf(errcode.PackagesUploadBadName,
			"a package is named <command>-<area>, lower case, such as indopacom-hawaii")
	}

	dir := strings.TrimSpace(p.getConfiguration().LocationMapPackagesDir)
	if dir == "" {
		return errcode.Errorf(errcode.PackagesUploadNoDir,
			"no package directory is configured, so there is nowhere to put this")
	}

	if err := os.MkdirAll(dir, 0o750); err != nil {
		return errcode.Errorf(errcode.PackagesUploadWriteFailed,
			"the package directory cannot be created: %v", err)
	}

	// Written beside the target and renamed, so a reader never sees a
	// half-written archive: discovery lists the directory on every request, and
	// a partial file would be validated, rejected and logged while the upload
	// was still in progress.
	temp, err := os.CreateTemp(dir, "."+name+".*.part")
	if err != nil {
		return errcode.Errorf(errcode.PackagesUploadWriteFailed,
			"cannot write to the package directory: %v", err)
	}
	tempPath := temp.Name()

	defer func() {
		_ = temp.Close()
		_ = os.Remove(tempPath)
	}()

	written, err := io.Copy(temp, io.LimitReader(body, maxUploadBytes+1))
	if err != nil {
		return errcode.Errorf(errcode.PackagesUploadWriteFailed, "the upload failed: %v", err)
	}
	if written > maxUploadBytes {
		return errcode.Errorf(errcode.PackagesUploadTooLarge,
			"a package uploaded this way is limited to %d MB; copy larger areas into the "+
				"package directory instead", maxUploadBytes>>20)
	}
	if err := temp.Close(); err != nil {
		return errcode.Errorf(errcode.PackagesUploadWriteFailed, "the upload failed: %v", err)
	}

	// Validated BEFORE it is put in place, so a file that is not an archive is
	// refused with a reason rather than accepted and then silently skipped by
	// discovery, which is where every other bad package ends up.
	if err := validPackageHeader(tempPath); err != nil {
		return errcode.Errorf(errcode.PackagesUploadNotAnArchive,
			"that is not a map archive built for zoom %d and up: %v", seamZoom, err)
	}

	if err := os.Rename(tempPath, filepath.Join(dir, name+packageSuffix)); err != nil {
		return errcode.Errorf(errcode.PackagesUploadWriteFailed,
			"the package could not be put in place: %v", err)
	}

	p.forgetPackages()

	return nil
}

// Removing one, which is the other half of being able to install one: an area
// that can only ever be added is a directory an admin has to reach by hand
// anyway.
func (p *Plugin) removePackage(name string) error {
	dir := strings.TrimSpace(p.getConfiguration().LocationMapPackagesDir)
	if !packageNamePattern.MatchString(name) || dir == "" {
		return errcode.Errorf(errcode.PackagesUploadBadName, "no such package")
	}

	// Said plainly rather than left to os.Remove's ENOENT, which an admin reads
	// as a failure of the button rather than as the answer.
	if !slices.Contains(p.removablePackages(), name) {
		return errcode.Errorf(errcode.PackagesUploadBadName,
			"that area ships inside the plugin and is replaced by a release, not from here")
	}

	if err := os.Remove(filepath.Join(dir, name+packageSuffix)); err != nil {
		return errcode.Errorf(errcode.PackagesUploadWriteFailed,
			"the package could not be removed: %v", err)
	}

	p.forgetPackages()

	return nil
}
