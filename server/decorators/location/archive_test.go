package location

import (
	"encoding/binary"
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

// A single root directory with no leaf directories is what keeps a from-scratch
// reader down to a header parse, one directory decode and an inflate. That is
// the recorded fallback if the pmtiles dependency ever has to be dropped, and it
// stops being available the moment the generator emits leaves.
func TestArchiveKeepsASingleRootDirectory(t *testing.T) {
	raw := readArchiveHeader(t)

	if leafLen := binary.LittleEndian.Uint64(raw[48:56]); leafLen != 0 {
		t.Errorf("leaf directory length = %d, want 0", leafLen)
	}
	if clustered := raw[96]; clustered != 1 {
		t.Errorf("clustered = %d, want 1", clustered)
	}
}

// The archive ships in the bundle and is fetched by every reader who opens a
// coordinate, so its size is a property worth noticing rather than discovering.
// The budget is deliberately loose: it exists to catch an order-of-magnitude
// change, such as a tier accidentally built at full detail across every zoom.
func TestArchiveFitsItsBudget(t *testing.T) {
	const budget = 8 * 1024 * 1024

	info, err := os.Stat(archivePath)
	if err != nil {
		t.Fatalf("the basemap archive is missing: %v. Run 'make map-tiles'", err)
	}

	if info.Size() > budget {
		t.Errorf("the basemap archive is %d bytes, over its %d budget", info.Size(), budget)
	}
}
