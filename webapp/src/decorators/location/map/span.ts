/**
 * The map's shared geometry constants, and the arithmetic that turns a target
 * ground span into a zoom level.
 *
 * These are pinned against the Go renderer by `webapp_sync_test.go`. The page
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
 * Natural Earth 10m carries roughly 5 km of positional accuracy. At z8 that is
 * about 16 px of possible error, which reads as generalisation; at z10 it is
 * 65 px, which reads as fact to a reader with no way to tell. For an audience
 * acting on grid references, the second is the wrong way to be wrong.
 *
 * The archive is built to exactly this depth, and
 * TestArchiveDepthMatchesTheCameraCeiling holds the two together: built
 * shallower and a reader zooms into blank tiles, built deeper and every install
 * carries zoom levels nothing can display.
 */
export const MAX_ZOOM = 8;

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

    return Math.max(0, Math.min(MAX_ZOOM, zoom));
}

/** The bounding rectangle of a cell, as MapLibre wants it. */
export function cellBounds(
    lat: number, lon: number, dLat: number, dLon: number,
): [[number, number], [number, number]] {
    const north = Math.min(MERCATOR_LIMIT, lat + (dLat / 2));
    const south = Math.max(-MERCATOR_LIMIT, lat - (dLat / 2));

    return [[lon - (dLon / 2), south], [lon + (dLon / 2), north]];
}
