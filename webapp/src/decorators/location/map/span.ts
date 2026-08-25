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

/**
 * How much of the canvas the map's own chrome covers, per edge, in CSS pixels.
 *
 * Every corner has something in it: the Reset button top left, MapLibre's zoom
 * buttons top right, the zoom readout bottom left, and the scale bar bottom
 * right. A block of events framed with one uniform padding put markers under
 * all four, because a number small enough not to waste the canvas is smaller
 * than the tallest thing on it.
 *
 * Corner chrome only has to be cleared on ONE axis, and which axis is chosen by
 * which is cheaper on a box this shape. The zoom buttons are 29 wide and 58
 * tall, so clearing them sideways costs 12% of a 320px width where clearing
 * them downwards costs 34% of a 200px height; the Reset button and the readout
 * are the other way round, wide and short, so they are cleared vertically. That
 * is why these four numbers are not symmetrical and must not be tidied into
 * one.
 *
 * Each includes half a marker, since a crosshair is drawn centred on its point
 * and would otherwise hang over the edge it was just cleared of. That allowance
 * is not a rounding: at 40 the top edge of a marker landed exactly on the
 * bottom edge of the Reset button. The reach of each control, measured from the
 * edge it is anchored to, is 32 for Reset (8 + 24), 39 for the zoom buttons
 * (10 + 29), 30 for the readout (8 + 22) and 28 for the scale bar (10 + 18);
 * every number above is that plus half a marker plus a little. The BOTTOM edge
 * is sized against the readout's 30 rather than the scale bar's 28, since both
 * sit on it and the taller one is what has to be cleared.
 *
 * span.spec.ts holds them to MARKER_SIZE, and the browser test 'no marker opens
 * underneath a control' holds them to where MapLibre actually puts a marker on
 * a panel-width canvas, which is the only thing that catches a graze.
 */
const CHROME_PADDING_PX = {top: 48, right: 52, bottom: 46, left: 26} as const;

/**
 * The same, for a surface that draws no chrome at all.
 *
 * The hover card is `preview`: no controls, no readout, no Reset. It still
 * needs half a marker plus enough that a pin does not sit against the frame.
 */
const BARE_PADDING_PX = {top: 24, right: 24, bottom: 24, left: 24} as const;

/**
 * The most of one axis padding may take.
 *
 * Padding is in pixels and the canvas is not: the panel map is 200px tall at
 * its minimum, where the chrome and its marker allowance want 88 of them. That
 * is 44% of the shortest map this component draws, which is why the share is a
 * half rather than something more comfortable: the chrome genuinely covers that
 * much of a short canvas, and a lower ceiling would scale the padding back down
 * into the controls it exists to clear.
 *
 * Past this share the padding is scaled down rather than allowed to invert the
 * viewport, which MapLibre answers with a camera nobody can read.
 */
export const MAX_PADDING_SHARE = 0.5;

export interface FitPadding {
    top: number;
    right: number;
    bottom: number;
    left: number;
}

/**
 * What to pad a multi-marker fit by, so no marker lands under the chrome.
 */
export function fitPadding(widthPx: number, heightPx: number, hasChrome: boolean): FitPadding {
    const want = hasChrome ? CHROME_PADDING_PX : BARE_PADDING_PX;
    const [left, right] = withinAxis(want.left, want.right, widthPx);
    const [top, bottom] = withinAxis(want.top, want.bottom, heightPx);

    return {top, right, bottom, left};
}

/**
 * Both edges of one axis, scaled together if they would not fit.
 *
 * Scaled in proportion rather than clamped individually, because the asymmetry
 * is the whole point: halving both keeps the wide edge wide, while clipping
 * each to a ceiling would flatten them into the uniform padding this replaced.
 */
function withinAxis(near: number, far: number, sizePx: number): [number, number] {
    const total = near + far;
    const budget = Math.max(0, sizePx) * MAX_PADDING_SHARE;

    if (total <= budget || total === 0) {
        return [near, far];
    }

    const scale = budget / total;
    return [Math.floor(near * scale), Math.floor(far * scale)];
}
