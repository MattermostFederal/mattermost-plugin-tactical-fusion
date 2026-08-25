import {expect, test} from '@playwright/test';

import {MARKER_SIZE} from './maplibre';
import {
    DATA_MAX_ZOOM, MAX_PADDING_SHARE, MAX_ZOOM, MERCATOR_LIMIT, TARGET_SPAN_METERS, cellBounds, fitPadding,
    isRenderable, zoomForSpan,
} from './span';

/*
 * The map's arithmetic, which is duplicated in Go and pinned to it by
 * TestWebappMapConstantsMatch. These cover the behaviour rather than the
 * numbers: that a constant ground span is not a constant zoom, and that a
 * position the projection cannot represent is refused rather than clamped.
 */

test('holds the ground span rather than the zoom as latitude rises', () => {
    // A degree of longitude is 111 km at the equator and 56 km at 60 north, so
    // showing the same 2,400 km there takes twice as many degrees, which is one
    // zoom level further OUT. This is the rule the deleted Go renderer applied
    // too (not to be confused with `viewFor` in map/view.ts, which is a different
    // thing entirely), where the half-width
    // doubles over the same range. A dropped cos factor shows a third of the
    // intended area at high latitude with no visible symptom.
    const equator = zoomForSpan(0, 320);
    const sixty = zoomForSpan(60, 320);

    expect(equator - sixty).toBeCloseTo(1, 2);
});

test('a wider viewport shows more ground at the same zoom step', () => {
    expect(zoomForSpan(0, 640) - zoomForSpan(0, 320)).toBeCloseTo(1, 6);
});

test('clamps to the honest range of the basemap', () => {
    expect(zoomForSpan(85, 320)).toBeLessThanOrEqual(DATA_MAX_ZOOM);
    expect(zoomForSpan(0, 1)).toBeGreaterThanOrEqual(0);
});

test('a position beyond the projection is refused rather than clamped', () => {
    // Clamping 89.9 to the limit would draw the pin about 550 km from what the
    // author wrote, silently.
    expect(isRenderable(MERCATOR_LIMIT)).toBe(true);
    expect(isRenderable(-MERCATOR_LIMIT)).toBe(true);
    expect(isRenderable(85.06)).toBe(false);
    expect(isRenderable(89.9)).toBe(false);
    expect(isRenderable(-89.9)).toBe(false);
});

test('a non-finite latitude is not renderable', () => {
    expect(isRenderable(NaN)).toBe(false);
    expect(isRenderable(Infinity)).toBe(false);
});

test('null island is renderable, because it is a position like any other', () => {
    expect(isRenderable(0)).toBe(true);
});

test('cell bounds are south-west first, north-east second', () => {
    // A swapped pair draws a rectangle somewhere else entirely and nothing else
    // in the pipeline would notice.
    const [[west, south], [east, north]] = cellBounds(35, -79, 1, 1);

    expect(west).toBeCloseTo(-79.5, 6);
    expect(east).toBeCloseTo(-78.5, 6);
    expect(south).toBeCloseTo(34.5, 6);
    expect(north).toBeCloseTo(35.5, 6);
});

test('a cell straddling the projection limit is clamped, not extended', () => {
    const [[, south], [, north]] = cellBounds(85, 0, 4, 4);

    expect(north).toBeLessThanOrEqual(MERCATOR_LIMIT);
    expect(south).toBeCloseTo(83, 6);
});

/*
 * The opening view, pinned.
 *
 * TARGET_SPAN_METERS decides what a reader sees first on every surface, and it
 * was asserted by nothing in either language: changing it by 25x left all 679
 * tests green, in a file reporting 100% of its lines and branches. Coverage
 * cannot see this, because every test here executes the constant and none of
 * them cared what it was.
 *
 * The value is a judgment rather than a derivation, which is exactly why it
 * wants a test: 400 km frames a region, and moving it is a decision about what
 * the map is for rather than a tuning tweak.
 */
test.describe('the opening view', () => {
    test('frames about 400 km across', () => {
        expect(TARGET_SPAN_METERS).toBe(400000);
    });

    // The consequence, so a reader of a failure sees what actually changed for
    // the reader rather than only which constant moved.
    test('opens a 320px panel near z6 at the equator', () => {
        expect(zoomForSpan(0, 320)).toBeGreaterThan(5.5);
        expect(zoomForSpan(0, 320)).toBeLessThan(6.5);
    });

    // Wide enough to be a real view rather than a clamp: if the opening zoom
    // ever reaches the ceiling, every coordinate opens fully zoomed in and
    // Reset view stops meaning anything.
    test('leaves room to zoom in on every surface', () => {
        for (const width of [320, 600, 1200]) {
            expect(zoomForSpan(0, width)).toBeLessThan(DATA_MAX_ZOOM);
        }
    });
});

/*
 * The camera goes past the data on purpose, and the opening view never does.
 *
 * These are two halves of one decision. If the ceiling stopped at the data a
 * cell could not be inspected at the resolution its token carried, which is the
 * one thing its size is drawn to say; and if the OPENING zoom could land past
 * the data, every coordinate would open onto a magnified basemap without the
 * reader ever asking for it. Overzoom has to be a gesture, not a default.
 */
test('the camera may overzoom and the opening view may not', () => {
    expect(MAX_ZOOM).toBeGreaterThan(DATA_MAX_ZOOM);

    for (const lat of [0, 35, 60, 85]) {
        for (const width of [200, 320, 640, 1000, 4000]) {
            expect(zoomForSpan(lat, width)).toBeLessThanOrEqual(DATA_MAX_ZOOM);
        }
    }
});

/*
 * The padding a block of events is framed with.
 *
 * The bug these cover: every corner of the map has chrome in it, and one
 * uniform number small enough not to waste the canvas is smaller than the
 * tallest of them, so markers opened underneath the zoom buttons and the scale
 * bar. What matters is that the padding is asymmetric, that it survives a small
 * canvas, and that it never inverts the viewport.
 */
test('padding clears the chrome on the edge each control sits against', () => {
    const padding = fitPadding(320, 200, true);

    // The zoom buttons are 10px in from the right and 29px wide.
    expect(padding.right).toBeGreaterThan(39);

    // The Reset button is 8px down and 24px tall, so it reaches 32. At the
    // bottom the readout reaches 30 and the scale bar 28, so the readout is the
    // one that has to be cleared.
    expect(padding.top).toBeGreaterThan(32);
    expect(padding.bottom).toBeGreaterThan(30);
});

/*
 * Half a marker, on a canvas with room for it.
 *
 * The precondition is the point. Below roughly 64px on an axis the share ceiling
 * scales the padding under half a marker and a crosshair does hang over the
 * edge, which is unavoidable: the chrome alone wants more than such a canvas
 * has. This used to be asserted at one size with no precondition stated, which
 * made it read as a guarantee the function does not give.
 */
test('padding clears half a marker wherever the canvas has room', () => {
    const half = MARKER_SIZE / 2;

    for (const chrome of [true, false]) {
        for (const [w, h] of [[320, 200], [360, 216], [640, 360], [120, 80], [64, 48]]) {
            const padding = fitPadding(w, h, chrome);
            for (const edge of [padding.top, padding.right, padding.bottom, padding.left]) {
                expect(edge, `${w}x${h} chrome=${chrome}`).toBeGreaterThanOrEqual(half);
            }
        }
    }
});

test('a canvas too small for the chrome degrades rather than inverting', () => {
    const padding = fitPadding(40, 200, true);

    // Still a usable viewport, still ordered, just no longer clearing a marker.
    expect(padding.left + padding.right).toBeLessThanOrEqual(40 * MAX_PADDING_SHARE);
    expect(padding.left).toBeGreaterThan(0);
    expect(padding.right).toBeGreaterThan(padding.left);
});

/*
 * A surface with no chrome does not pay for chrome it does not draw. The hover
 * card is 320x180 and has no controls, no readout and no Reset button.
 */
test('a bare surface is padded less than one carrying controls', () => {
    const bare = fitPadding(320, 180, false);
    const chromed = fitPadding(320, 180, true);

    expect(bare.right).toBeLessThan(chromed.right);
    expect(bare.top).toBeLessThan(chromed.top);
});

/*
 * Padding is in pixels and the canvas is not. Past the share it may take, both
 * edges scale together rather than one being clipped, which would flatten the
 * asymmetry that is the whole point.
 */
test('padding never takes more of an axis than it can afford', () => {
    for (const [width, height] of [[320, 200], [200, 120], [120, 80], [64, 48], [1, 1], [0, 0]]) {
        const padding = fitPadding(width, height, true);

        // Against the SHARE, not against the axis. Asserting only that padding
        // fits inside the canvas passes with MAX_PADDING_SHARE raised to 1,
        // which is the degenerate zero-width viewport this ceiling exists to
        // prevent.
        expect(padding.left + padding.right).toBeLessThanOrEqual(Math.max(width, 0) * MAX_PADDING_SHARE);
        expect(padding.top + padding.bottom).toBeLessThanOrEqual(Math.max(height, 0) * MAX_PADDING_SHARE);

        for (const edge of [padding.top, padding.right, padding.bottom, padding.left]) {
            expect(edge).toBeGreaterThanOrEqual(0);
            expect(Number.isFinite(edge)).toBe(true);
        }
    }
});

test('a squeezed axis keeps the wider edge wider', () => {
    const padding = fitPadding(80, 200, true);

    // right (52) started wider than left (26) and must still be.
    expect(padding.right).toBeGreaterThan(padding.left);
});
