package location

import (
	"bytes"
	"compress/gzip"
	"encoding/binary"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"
)

// The vector basemap every map surface draws. Its bytes are never read by Go at
// runtime: the browser fetches it directly from public/. What is worth asserting
// here is that the committed artifact is the shape the webapp's reader and the
// style both assume, because every way it can be wrong is silent. A shallow
// archive renders blank at the zooms it lacks, a raster one draws nothing at
// all under vector layers, and either reads as a rendering bug rather than a
// data one.
const archivePath = "../../../public/map/world.pmtiles"

/*
 * The OpenStreetMap detail tier, which takes over at the seam.
 *
 * Optional, and its absence is a configuration rather than a fault: a
 * global-only build is a supported shipping profile and the style simply omits
 * the source. Every assertion about it therefore skips rather than fails when
 * it is not there, and the pilot region is committed precisely so that on this
 * branch they run instead.
 */
const detailPackageGlob = "../../../public/map/packages/*.pmtiles"

// The detail packages this build ships, by path. Empty is legitimate and is
// what a global-only build looks like, so every assertion over these skips
// rather than fails; the pilot area is committed so that on this branch they
// run instead.
func detailPackages(t *testing.T) []string {
	t.Helper()

	paths, err := filepath.Glob(detailPackageGlob)
	if err != nil {
		t.Fatalf("cannot list detail packages: %v", err)
	}
	slices.Sort(paths)

	return paths
}

const (
	maptilesGenerator = "build/maptiles/build.sh"
	maposmGenerator   = "build/maposm/build.sh"
	maposmRegions     = "build/maposm/regions.txt"
	bundledProfile    = "bundled"
)

// A `NAME="${NAME:-N}"` default out of a generator, which is where a build
// depth is declared once so it cannot move for one layer and miss another.
func shellDefault(t *testing.T, script, name string) int {
	t.Helper()

	raw, err := os.ReadFile("../../../" + script) // #nosec G304 -- a generator path named by this test
	if err != nil {
		t.Fatalf("cannot read %s: %v", script, err)
	}

	m := regexp.MustCompile(name + `="\$\{` + name + `:-([0-9]+)\}"`).FindSubmatch(raw)
	if m == nil {
		t.Fatalf("no `%s=\"${%s:-N}\"` in %s; if it was renamed, point this test at the "+
			"new name rather than deleting it", name, name, script)
	}

	v, err := strconv.Atoi(string(m[1]))
	if err != nil {
		t.Fatalf("%s in %s is not a number", name, script)
	}

	return v
}

const (
	archiveHeaderBytes = 127
	archiveMagic       = "PMTiles"
	archiveSpecVersion = 3
	archiveTileTypeMVT = 1
)

func readArchiveHeader(t *testing.T) []byte {
	t.Helper()

	return readArchive(t, archivePath)
}

func buildCommandFor(path string) string {
	if path == archivePath {
		return "make map-tiles"
	}

	return "make map-osm"
}

func readArchive(t *testing.T, path string) []byte {
	t.Helper()

	raw, err := os.ReadFile(path) // #nosec G304 -- an archive path named by this test
	if err != nil {
		t.Fatalf("%s is missing: %v. Run %q", path, err, buildCommandFor(path))
	}
	if len(raw) < archiveHeaderBytes {
		t.Fatalf("%s is %d bytes, shorter than its own header", path, len(raw))
	}

	return raw
}

func TestArchiveIsTheShapeTheReaderAssumes(t *testing.T) {
	raw := readArchiveHeader(t)

	if got := string(raw[:len(archiveMagic)]); got != archiveMagic {
		t.Errorf("magic = %q, want %q", got, archiveMagic)
	}
	if got := raw[7]; got != archiveSpecVersion {
		t.Errorf("spec version = %d, want %d", got, archiveSpecVersion)
	}
	if got := raw[99]; got != archiveTileTypeMVT {
		t.Errorf("tile type = %d, want %d (MVT)", got, archiveTileTypeMVT)
	}
	if got := raw[100]; got != 0 {
		t.Errorf("min zoom = %d, want 0", got)
	}
}

/*
 * The archive and DATA_MAX_ZOOM have to agree, and this is the only place that
 * can see both: the constant is webapp-only and the archive is a binary.
 *
 * Both directions are errors. Built shallower than the constant claims and a
 * reader zooms into tiles that do not exist, which reads as a rendering bug
 * rather than a data one. Built deeper and the extra zoom levels are bytes in
 * every install that nothing can ever display, which usually means one of the
 * two was raised without the other.
 *
 * The CAMERA is deliberately not held to this any more. MAX_ZOOM runs past the
 * data on purpose, so that the cell drawn around a pin can be inspected at the
 * resolution its token carried; past DATA_MAX_ZOOM MapLibre overzooms and the
 * map says so on screen. What still has to hold is that the camera never stops
 * SHORT of the data, which would ship zoom levels nothing can reach.
 */
func TestArchiveDepthMatchesTheData(t *testing.T) {
	raw := readArchiveHeader(t)

	source := readWebappSource(t, "map/span.ts")
	dataMax := int(tsConstant(t, source, "map/span.ts", "DATA_MAX_ZOOM"))
	cameraMax := int(tsConstant(t, source, "map/span.ts", "MAX_ZOOM"))

	if got := int(raw[101]); got != dataMax {
		t.Errorf("the archive stops at zoom %d and DATA_MAX_ZOOM is %d. Rebuild with "+
			"'make map-tiles' or move DATA_MAX_ZOOM in map/span.ts, but not one alone",
			got, dataMax)
	}

	if cameraMax < dataMax {
		t.Errorf("the camera stops at zoom %d and the data goes to %d, so every install "+
			"carries zoom levels no reader can reach", cameraMax, dataMax)
	}
}

/*
 * The detail archive begins exactly at the seam and goes as deep as its
 * generator says.
 *
 * Three constants have to agree and no two of them are in the same language:
 * SEAM_ZOOM in map/span.ts, which is where the style stops drawing Natural
 * Earth and starts drawing OpenStreetMap; MINZ and MAXZ in build/maposm's
 * build.sh, which is what the archive was cut to; and the archive's own header,
 * which is what the browser reads on arrival.
 *
 * The minimum is an EQUALITY because a gap or an overlap there is a visible
 * defect: one zoom short and z10 draws neither tier, one zoom long and both
 * draw the same road with kilometers between them. The maximum is checked
 * against the generator rather than against a client constant, because there is
 * deliberately no client-side maximum: an operator may ship a shallower profile
 * than this pipeline can build and the reader must simply get what is there.
 */
func TestDetailPackagesStartAtTheSeam(t *testing.T) {
	packages := detailPackages(t)
	if len(packages) == 0 {
		t.Skip("no detail packages; a global-only build is a supported profile")
	}

	seam := int(tsConstant(t, readWebappSource(t, "map/span.ts"), "map/span.ts", "SEAM_ZOOM"))
	if want := shellDefault(t, maposmGenerator, "MINZ"); want != seam {
		t.Errorf("%s builds from zoom %d and SEAM_ZOOM is %d", maposmGenerator, want, seam)
	}
	maxz := shellDefault(t, maposmGenerator, "MAXZ")

	for _, path := range packages {
		t.Run(filepath.Base(path), func(t *testing.T) {
			raw := readArchive(t, path)

			if got := int(raw[100]); got != seam {
				t.Errorf("starts at zoom %d and SEAM_ZOOM is %d. At a gap neither tier "+
					"draws; at an overlap both draw the same road. Rebuild with "+
					"'make map-osm' or move SEAM_ZOOM, but not one alone", got, seam)
			}
			if got := int(raw[101]); got != maxz {
				t.Errorf("stops at zoom %d and %s declares %d. Rebuild with 'make map-osm'",
					got, maposmGenerator, maxz)
			}
		})
	}
}

/*
 * A package's NAME reaches a URL and a filesystem path, and is whitelisted in
 * three places that cannot see each other: build.sh writes it, packages.go
 * serves it, and basemap.ts requests it. This is the only test that can look at
 * what was actually committed.
 */
func TestDetailPackagesAreNamedForTheirCommand(t *testing.T) {
	packages := detailPackages(t)
	if len(packages) == 0 {
		t.Skip("no detail packages; a global-only build is a supported profile")
	}

	named := regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)+\.pmtiles$`)
	for _, path := range packages {
		if base := filepath.Base(path); !named.MatchString(base) {
			t.Errorf("%q is not <command>-<area>.pmtiles, so nothing will serve it", base)
		}
	}
}

func TestOnlyBundledRegionsAreCommitted(t *testing.T) {
	raw, err := os.ReadFile("../../../" + maposmRegions) // #nosec G304 -- a generator path named by this test
	if err != nil {
		t.Fatalf("cannot read %s: %v", maposmRegions, err)
	}

	bundled := map[string]bool{}
	for line := range strings.SplitSeq(string(raw), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 || strings.HasPrefix(fields[0], "#") {
			continue
		}
		if slices.Contains(strings.Split(fields[1], ","), bundledProfile) {
			bundled[fields[0]] = true
		}
	}

	if len(bundled) == 0 {
		t.Fatalf("no row in %s carries the %q profile; if that token was renamed, point "+
			"this test at the new one rather than deleting it", maposmRegions, bundledProfile)
	}

	for _, path := range detailPackages(t) {
		name := strings.TrimSuffix(filepath.Base(path), ".pmtiles")
		if !bundled[name] {
			t.Errorf("%s is committed but its %s row does not carry the %q profile. "+
				"Regions without it build into build/maposm/out/ and ship as release "+
				"assets; delete this file rather than raising a budget to fit it",
				path, maposmRegions, bundledProfile)
		}
	}
}

func TestDetailPackagesAreTheShapeTheReaderAssumes(t *testing.T) {
	packages := detailPackages(t)
	if len(packages) == 0 {
		t.Skip("no detail packages; a global-only build is a supported profile")
	}

	for _, path := range packages {
		t.Run(filepath.Base(path), func(t *testing.T) {
			raw := readArchive(t, path)

			if got := string(raw[:len(archiveMagic)]); got != archiveMagic {
				t.Errorf("magic = %q, want %q", got, archiveMagic)
			}
			if got := raw[7]; got != archiveSpecVersion {
				t.Errorf("spec version = %d, want %d", got, archiveSpecVersion)
			}
			if got := raw[99]; got != archiveTileTypeMVT {
				t.Errorf("tile type = %d, want %d (MVT)", got, archiveTileTypeMVT)
			}
			if clustered := raw[96]; clustered != 1 {
				t.Errorf("clustered = %d, want 1", clustered)
			}
		})
	}
}

/*
 * What the two archives weigh together, which is the figure that actually gates
 * an install and which nothing checked before.
 *
 * Mattermost gates plugin upload through the file API, so FileSettings'
 * MaxFileSize (100 MiB by default) is the ceiling, and the bundle is the two
 * archives plus the glyph ranges plus both webpack bundles plus the server
 * binaries. The per-archive budgets above catch an order-of-magnitude mistake
 * in one tier; only this catches two tiers that are each individually plausible
 * and together do not fit.
 *
 * Loose on purpose, exactly as the per-archive budgets are. It is not an
 * allowance to spend: the room under it is the point of it, and a budget raised
 * to fit whatever was just built stops catching anything.
 */
func TestArchivesFitTheBundleTogether(t *testing.T) {
	const budget = 64 * 1024 * 1024

	info, err := os.Stat(archivePath)
	if err != nil {
		t.Fatalf("the basemap archive is missing: %v. Run 'make map-tiles'", err)
	}
	total := info.Size()

	for _, path := range detailPackages(t) {
		pkg, err := os.Stat(path)
		if err != nil {
			t.Fatalf("cannot stat %s: %v", path, err)
		}
		total += pkg.Size()
	}

	if total > budget {
		t.Errorf("the basemap archives are %d bytes together, over their %d budget. "+
			"They are most of the plugin bundle, which Mattermost refuses past "+
			"FileSettings.MaxFileSize", total, budget)
	}
}

/*
 * The archive stays clustered, and its directory layout is recorded rather than
 * constrained.
 *
 * It used to carry a single root directory and no leaves, which kept a
 * from-scratch PMTiles reader down to a header parse, one directory decode and
 * an inflate. That was written down as the escape hatch if the pmtiles
 * dependency ever had to be dropped.
 *
 * Enriching the basemap to z8 ended it: at roughly 32,000 tile entries the
 * writer spills into leaf directories, which is ordinary PMTiles v3 and not a
 * fault. Avoiding it would mean capping zoom or dropping layers, which is the
 * opposite of what the archive grew for, so the guarantee was given up
 * deliberately rather than tuned around.
 *
 * What it costs, so nobody has to rediscover it: the fallback reader now needs a
 * second directory level, and a cold tile lookup can take two range requests
 * instead of one. Directory caching in the client absorbs the second for every
 * tile after the first few.
 *
 * Clustered still matters and is still asserted: it is what lets a reader walk
 * tile data in order rather than seeking per tile.
 */
func TestArchiveIsClustered(t *testing.T) {
	raw := readArchiveHeader(t)

	if clustered := raw[96]; clustered != 1 {
		t.Errorf("clustered = %d, want 1", clustered)
	}
}

func TestArchiveFitsItsBudget(t *testing.T) {
	// Raised from 8 MiB when the basemap gained roads, railways, urban areas,
	// states, rivers and populated places, and its ceiling moved from z6 to z8.
	// Deliberately NOT raised again when the ceiling moved to z9, which cost
	// +66% and left 8.8 MiB under this figure: the room it has left is the whole
	// point of it, and a budget raised to fit whatever was just built stops
	// catching anything. Still loose on purpose: this catches an
	// order-of-magnitude mistake, such as a tier accidentally built at full
	// detail across every zoom, not a layer added on purpose.
	const budget = 48 * 1024 * 1024

	info, err := os.Stat(archivePath)
	if err != nil {
		t.Fatalf("the basemap archive is missing: %v. Run 'make map-tiles'", err)
	}

	if info.Size() > budget {
		t.Errorf("the basemap archive is %d bytes, over its %d budget", info.Size(), budget)
	}
}

/*
 * Every layer the archive carries is drawn, and every layer drawn is carried.
 *
 * This is the one place that can see both sides. The style names its
 * source-layers in TypeScript and the archive declares its own in a gzipped
 * metadata block, so each half is invisible to the other's tests, and both
 * failure directions are silent: a source-layer the archive does not carry
 * draws nothing and reads as a data problem, and a layer built into the archive
 * that no style names is bytes in every install that nobody can ever see.
 *
 * It replaces a TypeScript test that compared the style against a hardcoded
 * list of ten strings and never opened the archive, so a --named-layer added to
 * or dropped from build.sh passed it unchanged.
 */
func TestArchiveCarriesEveryLayerTheStyleDraws(t *testing.T) {
	inStyle := styleSourceLayers(t)

	t.Run("basemap", func(t *testing.T) {
		inArchive := archiveLayers(t, archivePath)

		for _, name := range inStyle["basemap"] {
			if !slices.Contains(inArchive, name) {
				t.Errorf("the style draws source-layer %q, which %s does not carry. "+
					"Either the layer was dropped from %s or the name is misspelled "+
					"in maplibre.ts; it draws nothing either way",
					name, archivePath, maptilesGenerator)
			}
		}

		for _, name := range inArchive {
			if !slices.Contains(inStyle["basemap"], name) {
				t.Errorf("%s carries layer %q, which no style layer draws. It is bytes "+
					"in every install that nobody can see: draw it in maplibre.ts or "+
					"drop it from %s", archivePath, name, maptilesGenerator)
			}
		}
	})

	/*
	 * The detail layers cannot be scraped the way the basemap ones are: their
	 * source is one per package, so the style names it with a variable rather
	 * than a literal and there is nothing constant to match. maplibre.ts
	 * therefore declares the set once, as DETAIL_SOURCE_LAYERS, and a TypeScript
	 * test holds the built style to it. This holds every committed package to
	 * the same list, which is the half no TypeScript test can see.
	 */
	wanted := detailSourceLayers(t)
	for _, path := range detailPackages(t) {
		t.Run(filepath.Base(path), func(t *testing.T) {
			inArchive := archiveLayers(t, path)

			for _, name := range wanted {
				if !slices.Contains(inArchive, name) {
					t.Errorf("the style draws source-layer %q, which %s does not carry. "+
						"Either it was dropped from %s or the archive predates it; it "+
						"draws nothing either way", name, path, maposmGenerator)
				}
			}

			for _, name := range inArchive {
				if !slices.Contains(wanted, name) {
					t.Errorf("%s carries layer %q, which no style layer draws. It is "+
						"bytes in every install that nobody can see: draw it in "+
						"maplibre.ts or drop it from %s", path, name, maposmGenerator)
				}
			}
		})
	}
}

// The source-layers the detail tier draws, out of the one place maplibre.ts
// declares them.
func detailSourceLayers(t *testing.T) []string {
	t.Helper()

	source := readWebappSource(t, "map/maplibre.ts")
	m := regexp.MustCompile(`(?s)DETAIL_SOURCE_LAYERS = \[(.*?)\]`).FindStringSubmatch(source)
	if m == nil {
		t.Fatal("no `DETAIL_SOURCE_LAYERS` in map/maplibre.ts; if it was renamed, point " +
			"this test at the new name rather than deleting it")
	}

	var names []string
	for _, quoted := range regexp.MustCompile(`'([a-z0-9_]+)'`).FindAllStringSubmatch(m[1], -1) {
		names = append(names, quoted[1])
	}
	if len(names) == 0 {
		t.Fatal("DETAIL_SOURCE_LAYERS is empty, so nothing holds a package to the style")
	}
	slices.Sort(names)

	return names
}

// The vector_layers an archive declares, out of its own metadata block.
func archiveLayers(t *testing.T, path string) []string {
	t.Helper()

	raw, err := os.ReadFile(path) // #nosec G304 -- an archive path named by this test
	if err != nil {
		t.Fatalf("%s is missing: %v. Run %q", path, err, buildCommandFor(path))
	}

	offset := binary.LittleEndian.Uint64(raw[24:32])
	length := binary.LittleEndian.Uint64(raw[32:40])
	if offset+length > uint64(len(raw)) {
		t.Fatalf("the metadata block runs past the end of the archive")
	}

	zr, err := gzip.NewReader(bytes.NewReader(raw[offset : offset+length]))
	if err != nil {
		t.Fatalf("the metadata block is not gzipped: %v", err)
	}
	defer func() { _ = zr.Close() }()

	var meta struct {
		VectorLayers []struct {
			ID string `json:"id"`
		} `json:"vector_layers"`
	}
	if err := json.NewDecoder(zr).Decode(&meta); err != nil {
		t.Fatalf("the metadata block is not JSON: %v", err)
	}

	names := make([]string, 0, len(meta.VectorLayers))
	for _, l := range meta.VectorLayers {
		names = append(names, l.ID)
	}
	if len(names) == 0 {
		t.Fatal("the archive declares no vector layers at all")
	}
	slices.Sort(names)

	return names
}

/*
 * The source-layers maplibre.ts names, grouped by the source they are drawn
 * from.
 *
 * Grouping is what makes this usable with more than one archive. It used to
 * return a flat list scraped from the whole file, which was right while there
 * was one vector source and wrong the moment there were two: each archive would
 * report the other's layers as ones it fails to carry, in both directions at
 * once, and the test would fail with two lists of things that are all perfectly
 * correct.
 *
 * A layer's `source` always precedes its `'source-layer'` inside the same
 * object literal, so the nearest preceding `source:` is the one it belongs to.
 * A `'source-layer'` with no `source:` before it at all is a style that cannot
 * draw, and is reported rather than silently filed under the empty string.
 */
func styleSourceLayers(t *testing.T) map[string][]string {
	t.Helper()

	source := readWebappSource(t, "map/maplibre.ts")
	layerRe := regexp.MustCompile(`'source-layer': '([a-z0-9_]+)'`)
	sourceRe := regexp.MustCompile(`source: '([a-z0-9_]+)'`)

	found := layerRe.FindAllStringSubmatchIndex(source, -1)
	if found == nil {
		t.Fatal("no `'source-layer'` in map/maplibre.ts; if the style stopped naming " +
			"them, point this test at the new shape rather than deleting it")
	}

	bySource := map[string][]string{}
	for _, m := range found {
		name := source[m[2]:m[3]]

		owners := sourceRe.FindAllStringSubmatch(source[:m[0]], -1)
		if len(owners) == 0 {
			t.Fatalf("source-layer %q in map/maplibre.ts has no `source:` before it, "+
				"so nothing says which archive it is drawn from", name)
		}
		owner := owners[len(owners)-1][1]

		if !slices.Contains(bySource[owner], name) {
			bySource[owner] = append(bySource[owner], name)
		}
	}

	for _, names := range bySource {
		slices.Sort(names)
	}

	return bySource
}
