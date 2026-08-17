import {expect, test} from '@playwright/test';

import {
    DATA_MAX_ZOOM, MAX_ZOOM, MERCATOR_LIMIT, TARGET_SPAN_METERS, cellBounds, isRenderable, zoomForSpan,
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
    // zoom level further OUT. Mirrors viewFor in mapsvg.go, where the half-width
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
