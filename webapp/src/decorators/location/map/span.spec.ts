import {expect, test} from '@playwright/test';

import {MAX_ZOOM, MERCATOR_LIMIT, cellBounds, isRenderable, zoomForSpan} from './span';

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
    expect(zoomForSpan(85, 320)).toBeLessThanOrEqual(MAX_ZOOM);
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
