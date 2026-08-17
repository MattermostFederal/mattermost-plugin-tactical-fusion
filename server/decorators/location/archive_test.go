package location

import (
	"bytes"
	"compress/gzip"
	"encoding/binary"
	"encoding/json"
	"os"
	"regexp"
	"slices"
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

const (
	archiveHeaderBytes = 127
	archiveMagic       = "PMTiles"
	archiveSpecVersion = 3
	archiveTileTypeMVT = 1
)

func readArchiveHeader(t *testing.T) []byte {
	t.Helper()

	raw, err := os.ReadFile(archivePath)
	if err != nil {
		t.Fatalf("the basemap archive is missing: %v. Run 'make map-tiles'", err)
	}
	if len(raw) < archiveHeaderBytes {
		t.Fatalf("the basemap archive is %d bytes, shorter than its own header", len(raw))
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
 * The camera and the data have to stop at the same zoom, and this is the only
 * place that can see both: MAX_ZOOM is webapp-only and the archive is a binary.
 *
 * Both directions are errors. Built shallower than the camera allows and a
 * reader zooms into blank tiles, which reads as a rendering bug rather than a
 * data one. Built deeper and the extra zoom levels are bytes in every install
 * that nothing can ever display, which usually means one of the two was raised
 * without the other.
 */
func TestArchiveDepthMatchesTheCameraCeiling(t *testing.T) {
	raw := readArchiveHeader(t)

	maxZoom := int(tsConstant(t, readWebappSource(t, "map/span.ts"), "map/span.ts", "MAX_ZOOM"))

	if got := int(raw[101]); got != maxZoom {
		t.Errorf("the archive stops at zoom %d and the camera at %d. Rebuild with "+
			"'make map-tiles' or move MAX_ZOOM in map/span.ts, but not one alone",
			got, maxZoom)
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
	// Still loose on purpose: this catches an order-of-magnitude mistake, such as
	// a tier accidentally built at full detail across every zoom, not a layer
	// added on purpose.
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
	inArchive := archiveLayers(t)
	inStyle := styleSourceLayers(t)

	for _, name := range inStyle {
		if !slices.Contains(inArchive, name) {
			t.Errorf("the style draws source-layer %q, which the archive does not carry. "+
				"Either the layer was dropped from build/maptiles/build.sh or the name "+
				"is misspelled in maplibre.ts; it draws nothing either way", name)
		}
	}

	for _, name := range inArchive {
		if !slices.Contains(inStyle, name) {
			t.Errorf("the archive carries layer %q, which no style layer draws. It is "+
				"bytes in every install that nobody can see: draw it in maplibre.ts or "+
				"drop it from build/maptiles/build.sh", name)
		}
	}
}

// The vector_layers the archive declares, out of its own metadata block.
func archiveLayers(t *testing.T) []string {
	t.Helper()

	raw, err := os.ReadFile(archivePath)
	if err != nil {
		t.Fatalf("the basemap archive is missing: %v. Run 'make map-tiles'", err)
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

// The source-layers maplibre.ts names, read out of the style it builds.
func styleSourceLayers(t *testing.T) []string {
	t.Helper()

	source := readWebappSource(t, "map/maplibre.ts")
	found := regexp.MustCompile(`'source-layer': '([a-z0-9_]+)'`).FindAllStringSubmatch(source, -1)
	if found == nil {
		t.Fatal("no `'source-layer'` in map/maplibre.ts; if the style stopped naming " +
			"them, point this test at the new shape rather than deleting it")
	}

	var names []string
	for _, m := range found {
		if !slices.Contains(names, m[1]) {
			names = append(names, m[1])
		}
	}
	slices.Sort(names)

	return names
}
