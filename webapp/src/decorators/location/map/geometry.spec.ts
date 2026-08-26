import {expect, test} from '@playwright/test';

import {_frameBoundsForTesting as frameBounds} from './LocationMap';
import {accuracyFeature, ellipseFeature, outlineFeature} from './maplibre';
import {DEGREE_METERS} from './span';

const OUTLINE = [{lat: 1, lon: 10}, {lat: 2, lon: 20}, {lat: 3, lon: 30}];

// A linear ring needs four positions. Two points closed made three.
test('a two point shape stays a line even when it says it is closed', () => {
    const drawn = outlineFeature([{lat: 1, lon: 10}, {lat: 2, lon: 20}], true);
    expect(drawn.features[0].geometry.type).toBe('LineString');
});

test('an open outline is a line, and a closed one is a polygon that meets itself', () => {
    const line = outlineFeature(OUTLINE, false);
    expect(line.features[0].geometry.type).toBe('LineString');

    const area = outlineFeature(OUTLINE, true);
    expect(area.features[0].geometry.type).toBe('Polygon');

    const ring = (area.features[0].geometry as {coordinates: number[][][]}).coordinates[0];
    expect(ring[0]).toEqual(ring[ring.length - 1]);
});

// A shape needs somewhere to be. One point is a position, and the event has one.
test('fewer than two points is not an outline', () => {
    expect(outlineFeature([{lat: 1, lon: 2}], false).features).toHaveLength(0);
    expect(outlineFeature([], true).features).toHaveLength(0);
});

test('a point that is not a number is dropped rather than coerced', () => {
    const drawn = outlineFeature([...OUTLINE, {lat: NaN, lon: 5}], false);
    const line = (drawn.features[0].geometry as {coordinates: number[][]}).coordinates;

    expect(line).toHaveLength(OUTLINE.length);
});

// The axes are rotated in metres and only then converted, or the longitude
// scaling would shear the tilt and the ellipse would point somewhere the event
// did not say.
function ringOf(lat: number, lon: number, major: number, minor: number, angle: number) {
    const drawn = ellipseFeature(lat, lon, major, minor, angle);
    return (drawn.features[0].geometry as {coordinates: number[][][]}).coordinates[0];
}

/** Where the ring reaches furthest, as a bearing clockwise from north. */
function majorAxisBearing(ring: number[][], lat: number, lon: number): number {
    let best = -1;
    let at: number[] = ring[0];
    for (const [x, y] of ring) {
        const east = (x - lon) * Math.cos((lat * Math.PI) / 180);
        const north = y - lat;
        const d = Math.hypot(east, north);
        if (d > best) {
            best = d;
            at = [east, north];
        }
    }
    return ((((Math.atan2(at[0], at[1]) * 180) / Math.PI) + 360) % 360);
}

// At latitude 0 the cos(lat) division is a no-op, so a test there cannot see
// whether the conversion is geodesic at all. This one is away from the equator.
test('an ellipse is geodesic, so its axes are metres on the ground', () => {
    const lat = 60;
    const ring = ringOf(lat, 0, 1000, 1000, 0);

    const north = Math.max(...ring.map(([, y]) => y)) - lat;
    expect(north * DEGREE_METERS).toBeCloseTo(1000, 3);

    // East has to be scaled by cos(lat), or the circle is an egg.
    const east = Math.max(...ring.map(([x]) => x));
    expect(east * DEGREE_METERS * Math.cos((lat * Math.PI) / 180)).toBeCloseTo(1000, 3);
});

// The bearing is clockwise from north. A counter-clockwise matrix mirrors it,
// which is right at 0 and 90 and wrong everywhere else.
test('the major axis lies along the bearing the event stated', () => {
    for (const bearing of [0, 30, 45, 90, 135, 200]) {
        const ring = ringOf(45, 10, 2000, 200, bearing);
        const got = majorAxisBearing(ring, 45, 10);

        // An axis is a line, so either end of it satisfies the claim.
        const diff = got - bearing;
        const off = Math.min(Math.abs(diff), Math.abs(diff - 180), Math.abs(diff + 180));
        expect(off, `bearing ${bearing} drawn at ${got.toFixed(1)}`).toBeLessThan(2);
    }
});

test('the major axis is the long one', () => {
    const ring = ringOf(45, 10, 2000, 200, 0);
    const northSouth = Math.max(...ring.map(([, y]) => y)) - Math.min(...ring.map(([, y]) => y));
    const eastWest = Math.max(...ring.map(([x]) => x)) - Math.min(...ring.map(([x]) => x));

    expect(northSouth * DEGREE_METERS).toBeCloseTo(4000, 2);
    expect(eastWest).toBeLessThan(northSouth);
});

// The accuracy circle now goes through the same ring, so it has to be unmoved.
test('the accuracy circle is the ring with equal axes', () => {
    const circle = accuracyFeature(37.5, -122.5, 750);
    const ellipse = ellipseFeature(37.5, -122.5, 750, 750, 0);

    expect(circle).toEqual(ellipse);
});

test('an ellipse with no usable axes draws nothing', () => {
    for (const [major, minor] of [[0, 10], [10, 0], [NaN, 10], [-5, 10]]) {
        expect(ellipseFeature(0, 0, major, minor, 0).features, `${major},${minor}`).toHaveLength(0);
    }
});

test.describe('the frame', () => {
    const MARKERS = [{lat: 0, lon: 0, color: '#fff'}, {lat: 1, lon: 1, color: '#fff'}];

    test('is the union of the markers and the shape', () => {
        const bounds = frameBounds(MARKERS, {kind: 'outline', points: OUTLINE, closed: false}, 0, 0);

        expect(bounds).not.toBeNull();

        // The outline reaches 3,30 and the markers only 1,1.
        expect(bounds![1]).toEqual([30, 3]);
        expect(bounds![0]).toEqual([0, 0]);
    });

    // A single marker frames nothing today, and a shape around it still has to.
    test('frames a shape even when there is only one marker', () => {
        const one = [{lat: 0, lon: 0, color: '#fff'}];

        expect(frameBounds(one, undefined, 0, 0)).toBeNull();
        expect(frameBounds(one, {kind: 'outline', points: OUTLINE, closed: false}, 0, 0)).not.toBeNull();
    });

    test('an ellipse contributes its reach', () => {
        const bounds = frameBounds(undefined, {kind: 'ellipse', major: 1000, minor: 500, angle: 0}, 0, 0);

        expect(bounds).not.toBeNull();
        expect(bounds![1][1] * DEGREE_METERS).toBeCloseTo(1000, 3);
    });

    test('is nothing to frame when there is neither', () => {
        expect(frameBounds(undefined, undefined, 0, 0)).toBeNull();
    });
});
