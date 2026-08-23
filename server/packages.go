package main

import (
	"cmp"
	"compress/gzip"
	"encoding/binary"
	"errors"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/errcode"
)

var packageNamePattern = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)+$`)

const (
	packageSuffix = ".pmtiles"

	seamZoom = 10

	packageHeaderBytes = 127
	packageMagic       = "PMTiles"
	packageSpecVersion = 3
	packageTileTypeMVT = 1

	mapSchemaVersion = 1
	schemaPrefix     = "tactical-fusion-map/"

	schemaScanBytes = 4096

	compressionGzip = 2
)

var bundledPackageDir = filepath.Join("public", "map", "packages")

type mapPackage struct {
	Name string `json:"name"`

	path string

	dropIn bool
}

func (p *Plugin) mapPackages() []mapPackage {
	dirs := p.packageDirs()
	if cached := p.cachedPackages(dirs); cached != nil {
		return cached
	}

	found := map[string]mapPackage{}

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

func (p *Plugin) forgetPackages() {
	p.packageLock.Lock()
	defer p.packageLock.Unlock()

	p.packageCache = nil
}

func (p *Plugin) removablePackages() []string {
	names := []string{}
	for _, pkg := range p.mapPackages() {
		if pkg.dropIn {
			names = append(names, pkg.Name)
		}
	}

	return names
}

func (p *Plugin) packageNames() []string {
	packages := p.mapPackages()
	names := make([]string, 0, len(packages))
	for _, pkg := range packages {
		names = append(names, pkg.Name)
	}

	return names
}

func (p *Plugin) logWarn(message string, pairs ...any) {
	if p.API == nil {
		return
	}

	p.API.LogWarn(message, pairs...)
}

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

var archiveNamePattern = regexp.MustCompile(`"name"\s*:\s*"([^"]*)"`)

func archiveSchema(file io.ReadSeeker, header []byte) (int, error) {
	offset := int64(binary.LittleEndian.Uint64(header[24:32])) // #nosec G115 -- an offset from a validated header
	length := int64(binary.LittleEndian.Uint64(header[32:40])) // #nosec G115 -- same
	if length == 0 {
		return 1, nil
	}

	if _, err := file.Seek(offset, io.SeekStart); err != nil {
		return 0, err
	}

	blob := io.LimitReader(file, length)
	if header[97] == compressionGzip {
		zipped, err := gzip.NewReader(blob)
		if err != nil {
			return 0, errcode.Errorf(errcode.PackagesBadArchive, "unreadable metadata: %v", err)
		}
		defer func() { _ = zipped.Close() }()
		blob = zipped
	}

	head := make([]byte, schemaScanBytes)
	n, err := io.ReadFull(blob, head)
	if err != nil && !errors.Is(err, io.ErrUnexpectedEOF) && !errors.Is(err, io.EOF) {
		return 0, errcode.Errorf(errcode.PackagesBadArchive, "unreadable metadata: %v", err)
	}

	m := archiveNamePattern.FindSubmatch(head[:n])
	if m == nil {
		return 1, nil
	}

	stamp, found := strings.CutPrefix(string(m[1]), schemaPrefix)
	if !found {
		return 1, nil
	}

	schema, err := strconv.Atoi(stamp)
	if err != nil {
		return 0, errcode.Errorf(errcode.PackagesSchemaMismatch,
			"carries an unreadable map schema %q", m[1])
	}

	return schema, nil
}

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
	if schema, err := archiveSchema(file, header); err != nil {
		return err
	} else if schema != mapSchemaVersion {
		if schema < mapSchemaVersion {
			return errcode.Errorf(errcode.PackagesSchemaMismatch,
				"built for map schema %d and this build draws %d, so re-download the area",
				schema, mapSchemaVersion)
		}

		return errcode.Errorf(errcode.PackagesSchemaMismatch,
			"built for map schema %d and this build draws %d, so upgrade the plugin",
			schema, mapSchemaVersion)
	}

	if header[100] != seamZoom {
		return errcode.Errorf(errcode.PackagesBadArchive,
			"starts at zoom %d, want %d, which is where the detail tier takes over",
			header[100], seamZoom)
	}

	return nil
}

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

func (p *Plugin) removePackage(name string) error {
	dir := strings.TrimSpace(p.getConfiguration().LocationMapPackagesDir)
	if !packageNamePattern.MatchString(name) || dir == "" {
		return errcode.Errorf(errcode.PackagesUploadBadName, "no such package")
	}

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
