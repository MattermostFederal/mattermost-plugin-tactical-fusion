/**
 * The map's shared geometry constants, and the arithmetic that turns a target
 * ground span into a zoom level.
 *
 * DEGREE_METERS is pinned against Go by `webapp_sync_test.go`, which reads it
 * out of this file; there is no Go renderer any more, and the rest of these are
 * TypeScript-only. The page
 * and the panel draw the same coordinate at the same scale, or a reader who
 * opens both sees two different maps of one position.
 */

/** Where Web Mercator runs out. Beyond this there is no map, and no pin. */
export const MERCATOR_LIMIT = 85.0511287798066;

/**
 * How much ground the map tries to show across its width, in metres.
 *
 * This decides what a reader sees first, and it used to be 2,400 km, which
 * opened the panel at about z3.4 at the equator: two and a half zoom levels
 * below the map's own ceiling, with a one metre grid reference framed exactly
 * like a whole-degree one. 400 km frames a region instead, which is the
 * question somebody with a coordinate is usually asking. Zooming out to the
 * whole world is still one gesture away.
 */
export const TARGET_SPAN_METERS = 400000;

/**
 * Past this the basemap stops being honest about what it is.
 *
 * Natural Earth 10m carries roughly 5 km of positional accuracy, and a 512 px
 * tile puts 78271.5/2^z metres in a pixel. So that error is about 16 px at z8,
 * 33 px at z9 and 65 px at z10. At 16 px it reads as generalisation; at 65 px it
 * reads as fact to a reader with no way to tell, which for an audience acting on
 * grid references is the wrong way to be wrong.
 *
 * This was 8 for exactly that reason and is now 9, which is a deliberate trade
 * rather than a change of mind about the arithmetic. What z9 buys is narrow and
 * worth stating so nobody re-derives it as generous: the sources are the same
 * 10m files, so z9 carries no geometry z8 did not, and vector tiles magnify
 * without blurring. It buys halved coordinate quantization inside a tile
 * (a 4096-unit extent over a z8 tile is about 38 m at the equator, and about
 * 19 m at z9) and more room for the collision index to place labels. It does
 * not buy accuracy, and 33 px of possible error is the price.
 *
 * The archive is built to exactly this depth, and
 * TestArchiveDepthMatchesTheData holds the two together: built shallower and a
 * reader zooms into tiles that do not exist, built deeper and every install
 * carries zoom levels nothing can display. MAXZ in build/maptiles/build.sh is
 * the other half.
 *
 * This bounds the DATA and no longer bounds the CAMERA; see MAX_ZOOM.
 */
export const DATA_MAX_ZOOM = 9;

/**
 * How far the camera may go, which is deliberately past where the data stops.
 *
 * The two used to be one number, so a coordinate could never be inspected at
 * its own resolution: the cell drawn around a pin carries the token's precision
 * in its SIZE, and at z9 a 10 m grid reference is about a third of a pixel. The
 * reader could see that a cell existed and never how small it was, which is the
 * one thing it is drawn to say.
 *
 * 17 is where a 10 m cell reaches about 20 px, which covers the fine end of what
 * the grammars actually produce: 10 m MGRS, and four decimal places of a degree
 * (about 11 m). A 1 m grid reference is still about 2 px, and going deeper was
 * declined: it would need z20, where the basemap is magnified 2048 times and is
 * a flat colour with a straight line for a coastline.
 *
 * Past DATA_MAX_ZOOM MapLibre overzooms, which for vector tiles magnifies
 * without blurring, so lines stay crisp and only their GENERALISATION is wrong.
 * That is exactly the failure the ceiling used to exist to prevent, and nothing
 * on the map states it in words: a notice saying so was drawn here and then
 * removed, leaving the zoom readout as the only hint. What keeps that a choice
 * rather than something done to a reader is that `zoomForSpan` still clamps to
 * DATA_MAX_ZOOM, so nothing ever OPENS into overzoom.
 */
export const MAX_ZOOM = 17;

export const SEAM_ZOOM = 10;

/** A degree of latitude, the same approximation the Go side makes. */
export const DEGREE_METERS = 111320;

/** MapLibre's tile size, which fixes what a zoom level means. */
const TILE_PIXELS = 512;

/**
 * Whether a position can be drawn at all.
 *
 * The grammars validate latitude to 90 while the projection stops at about 85,
 * so this is reachable from ordinary text. Clamping instead would put the pin
 * up to 550 km from what the author wrote.
 */
export function isRenderable(lat: number): boolean {
    return Number.isFinite(lat) && Math.abs(lat) <= MERCATOR_LIMIT;
}

/**
 * The zoom that shows TARGET_SPAN_METERS across `widthPx` at this latitude.
 *
 * A constant zoom is not a constant answer: Mercator scale is 1/cos(lat), so
 * the same width is about 2,940 km at the equator and about 1,000 km at 70
 * north. Holding the ground span is what makes "what region contains this" get
 * the same answer everywhere.
 */
export function zoomForSpan(lat: number, widthPx: number): number {
    const cos = Math.max(Math.cos((lat * Math.PI) / 180), 1e-6);
    const worldPx = (360 * widthPx * DEGREE_METERS * cos) / TARGET_SPAN_METERS;
    const zoom = Math.log2(worldPx / TILE_PIXELS);

    return Math.max(0, Math.min(DATA_MAX_ZOOM, zoom));
}

/** The bounding rectangle of a cell, as MapLibre wants it. */
export function cellBounds(
    lat: number, lon: number, dLat: number, dLon: number,
): [[number, number], [number, number]] {
    const north = Math.min(MERCATOR_LIMIT, lat + (dLat / 2));
    const south = Math.max(-MERCATOR_LIMIT, lat - (dLat / 2));

    return [[lon - (dLon / 2), south], [lon + (dLon / 2), north]];
}
