import {expect, test} from '@playwright/test';

import {
    ACCURACY_VERTICES, MARKER_PIXEL_RATIO, MARKER_SIZE,
    accuracyFeature, crosshairImage,
} from './maplibre';
import {DEGREE_METERS} from './span';

function ring(lat: number, lon: number, meters: number): Array<[number, number]> {
    const collection = accuracyFeature(lat, lon, meters);
    const geometry = collection.features[0]?.geometry;
    if (geometry?.type !== 'Polygon') {
        throw new Error('no polygon');
    }
    return geometry.coordinates[0] as Array<[number, number]>;
}

test('a stated accuracy becomes a closed ring', () => {
    const points = ring(0, 0, 100);

    expect(points).toHaveLength(ACCURACY_VERTICES + 1);
    expect(points[0]).toEqual(points[points.length - 1]);
});

/*
 * The whole point of the layer is that the drawn accuracy matches the stated
 * one. An equal-degree ring is right at the equator and wrong everywhere else,
 * which is the failure this asserts against.
 */
test('the ring is geodesic rather than equal degree', () => {
    const meters = 10000;

    const atEquator = ring(0, 0, meters);
    const atSixty = ring(60, 0, meters);

    const equatorHalfWidth = Math.abs(atEquator[0][0]);
    const sixtyHalfWidth = Math.abs(atSixty[0][0]);

    // cos(60) is 0.5, so a meter is twice as many degrees of longitude there.
    expect(sixtyHalfWidth / equatorHalfWidth).toBeCloseTo(2, 2);

    // Latitude does not vary with latitude, so both rings are the same height.
    const equatorHalfHeight = Math.max(...atEquator.map(([, lat]) => lat));
    const sixtyHalfHeight = Math.max(...atSixty.map(([, lat]) => lat)) - 60;
    expect(sixtyHalfHeight).toBeCloseTo(equatorHalfHeight, 6);
});

test('the radius is the stated number of meters', () => {
    const meters = 5000;
    const points = ring(0, 0, meters);

    const north = Math.max(...points.map(([, lat]) => lat));
    expect(north * DEGREE_METERS).toBeCloseTo(meters, 3);
});

test('an accuracy that states nothing draws nothing', () => {
    for (const meters of [0, -1, Number.NaN, Number.POSITIVE_INFINITY]) {
        expect(accuracyFeature(0, 0, meters).features, String(meters)).toHaveLength(0);
    }
});

test('a position at the pole draws nothing rather than a ring of infinite width', () => {
    expect(accuracyFeature(90, 0, 100).features).toHaveLength(0);
});

// meters was guarded for finiteness and the coordinates were not, so a NaN
// latitude produced a polygon of NaN positions rather than nothing.
test('a position that is not a number draws nothing', () => {
    const cases: Array<[number, number]> = [
        [Number.NaN, 0],
        [0, Number.NaN],
        [Number.POSITIVE_INFINITY, 0],
        [0, Number.NEGATIVE_INFINITY],
    ];

    for (const [lat, lon] of cases) {
        expect(accuracyFeature(lat, lon, 100).features, `${lat},${lon}`).toHaveLength(0);
    }
});

/*
 * The marker.
 *
 * A dot says "somewhere around here"; a reticle says "this point", which is what
 * a Cursor on Target event claims. It is a raw RGBA buffer so it can be asserted
 * against its pixels rather than against a screenshot, and so it needs no glyph
 * from a font range this bundle trims.
 */
test.describe('the crosshair marker', () => {
    const FILL = '#ff0000';
    const EDGE = '#0b0e13';

    function pixel(image: {width: number; data: Uint8Array}, x: number, y: number) {
        const at = ((y * image.width) + x) * 4;
        return {
            r: image.data[at],
g: image.data[at + 1],
b: image.data[at + 2],
            a: image.data[at + 3],
        };
    }

    test('is square, and sized for the pixel ratio it declares', () => {
        const image = crosshairImage(FILL, EDGE);

        expect(image.width).toBe(MARKER_SIZE * MARKER_PIXEL_RATIO);
        expect(image.height).toBe(image.width);
        expect(image.data).toHaveLength(image.width * image.height * 4);
    });

    // The reason the shape is drawn from distances rather than from a
    // yes-or-no test. Without partial coverage every curve steps, which at this
    // size reads as grain rather than as a line.
    test('is antialiased rather than stepped', () => {
        const image = crosshairImage(FILL, EDGE);

        const partial = new Set<number>();
        for (let i = 3; i < image.data.length; i += 4) {
            const alpha = image.data[i];
            if (alpha > 0 && alpha < 255) {
                partial.add(alpha);
            }
        }

        expect(partial.size, 'every pixel is fully on or fully off').toBeGreaterThan(8);
    });

    test('is a circle with a line across it and a line down it', () => {
        const image = crosshairImage(FILL, EDGE);
        const center = Math.floor(image.width / 2);

        // Both lines run well past the circle, which is what makes them read as
        // crosshairs rather than as spokes. Measured against the ring rather
        // than against the bitmap edge, which is arbitrary.
        const beyondRing = Math.round(center * 0.8);
        expect(pixel(image, center - beyondRing, center).a).toBeGreaterThan(0);
        expect(pixel(image, center + beyondRing, center).a).toBeGreaterThan(0);
        expect(pixel(image, center, center - beyondRing).a).toBeGreaterThan(0);
        expect(pixel(image, center, center + beyondRing).a).toBeGreaterThan(0);

        // And the circle is a ring, not a disc: on the diagonal, where neither
        // line runs, there is a gap between the center and the stroke.
        const gap: number[] = [];
        for (let step = 1; step < center; step++) {
            gap.push(pixel(image, center + step, center + step).a);
        }

        expect(Math.min(...gap), 'nothing is hollow, so this is a disc').toBe(0);
        expect(Math.max(...gap.slice(gap.indexOf(0))), 'no ring beyond the gap').toBeGreaterThan(0);
    });

    // The palette's note says the marker is told from the land by its outline
    // rather than by its fill, which is what lets the fill carry an affiliation.
    test('puts the outline outside the fill, not the other way round', () => {
        const image = crosshairImage(FILL, EDGE);
        const center = Math.floor(image.width / 2);

        const near = (a: {r: number; g: number; b: number}, hex: string) => {
            const want = [
                parseInt(hex.slice(1, 3), 16),
                parseInt(hex.slice(3, 5), 16),
                parseInt(hex.slice(5, 7), 16),
            ];
            return Math.abs(a.r - want[0]) + Math.abs(a.g - want[1]) + Math.abs(a.b - want[2]);
        };

        // Walk in from the edge along the horizontal line. The first ink met is
        // the outline; the middle of the stroke is the fill.
        let first = 0;
        while (first < center && pixel(image, first, center).a === 0) {
            first++;
        }

        const outermost = pixel(image, first, center);
        expect(near(outermost, EDGE)).toBeLessThan(near(outermost, FILL));

        const middle = pixel(image, center, center - Math.round(center * 0.02));
        expect(near(middle, FILL)).toBeLessThan(near(middle, EDGE));
    });

    test('takes the color it is given, so the map and the card agree', () => {
        const image = crosshairImage('#3d85c6', '#000000');

        // The most opaque pixels are the stroke's own middle, which is the
        // affiliation. Blends toward the outline live at the boundary.
        let solid = 0;
        let matching = 0;
        for (let i = 0; i < image.data.length; i += 4) {
            if (image.data[i + 3] !== 255) {
                continue;
            }
            solid++;
            if (image.data[i] === 61 && image.data[i + 1] === 133 && image.data[i + 2] === 198) {
                matching++;
            }
        }

        expect(solid).toBeGreaterThan(100);
        expect(matching / solid, 'the fill is not the color it was given').toBeGreaterThan(0.3);
    });
});
