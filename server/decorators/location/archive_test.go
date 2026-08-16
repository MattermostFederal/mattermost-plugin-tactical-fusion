package location

import (
	"os"
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
