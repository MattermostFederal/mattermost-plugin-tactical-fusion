import {expect, test} from '@playwright/test';

import {
    _blockLabelForTesting as blockLabel,
    _drawableEventsForTesting as drawableEvents,
    _outlinesForTesting as outlines,
} from './CotMap';
import type {CotEvent} from './types';

function ev(affiliation: string, lat = '21.3353', lon = '-157.9483'): CotEvent {
    return {
        affiliation, lat, lon, format: 'dd', value: `${lat},${lon}`, detail: {colorArgb: ''},
    } as unknown as CotEvent;
}

/*
 * The label a block of markers gets.
 *
 * Written because the browser test that consumes this rule hard-codes the
 * string blockLabel must produce ('1 hostile and 1 friendly'), so the consumer
 * was pinned and the producer was not. That is how a ratio clause that could
 * never fire shipped with the suite green.
 */
test('names the affiliations rather than only counting', () => {
    expect(blockLabel([ev('hostile'), ev('friend')], 2)).toBe('1 hostile and 1 friendly');
});

test('is the string the browser test expects for the same block', () => {
    // Keep this identical to markerLabel in LocationMap.pw.tsx's block test.
    expect(blockLabel([ev('hostile'), ev('friend')], 2)).toBe('1 hostile and 1 friendly');
});

test('tallies repeats instead of listing them', () => {
    expect(blockLabel([ev('friend'), ev('friend'), ev('hostile')], 3)).
        toBe('2 friendly and 1 hostile');
});

test('joins three or more with commas and a final and', () => {
    expect(blockLabel([ev('friend'), ev('hostile'), ev('neutral')], 3)).
        toBe('1 friendly, 1 hostile and 1 neutral');
});

test('a single marker needs no conjunction', () => {
    expect(blockLabel([ev('hostile')], 1)).toBe('1 hostile');
});

/*
 * The ratio, which is the whole reason the count is of events rather than of
 * markers. It must appear only when they differ.
 */
test('states the ratio when events could not be drawn', () => {
    expect(blockLabel([ev('friend'), ev('hostile')], 5)).
        toBe('2 of 5 events: 1 friendly and 1 hostile');
});

test('omits the ratio when every event was drawn', () => {
    expect(blockLabel([ev('friend'), ev('hostile')], 2)).not.toContain('events:');
});

/*
 * An affiliation this build does not color is still named, because the marker
 * is still drawn, in gray.
 */
test('names an affiliation that carries no color', () => {
    expect(blockLabel([ev('joker')], 1)).toBe('1 joker');
    expect(blockLabel([ev('zzz')], 1)).toBe('1 unstated');
});

/*
 * The filter. An event the server gave no position must not become a pin: its
 * lat is the empty string, and Number('') is 0, which is finite.
 */
test('an event with no position is not drawable', () => {
    const positionless = {affiliation: 'friend', lat: '', lon: '', format: '', value: ''} as CotEvent;

    expect(drawableEvents([positionless])).toHaveLength(0);
    expect(drawableEvents([positionless, ev('hostile')])).toHaveLength(1);
});

test('an event past the projection is not drawable', () => {
    expect(drawableEvents([ev('friend', '88.0000', '10.0000')])).toHaveLength(0);
});

function outlined(uid: string, points: Array<{lat: number; lon: number}>): CotEvent {
    return {
        ...ev('unknown'),
        uid,
        geometry: {kind: 'polyline', closed: true, count: String(points.length), points, note: ''},
    } as unknown as CotEvent;
}

const SQUARE = [
    {lat: 21.34, lon: -157.95},
    {lat: 21.35, lon: -157.94},
    {lat: 21.33, lon: -157.93},
];

/*
 * A drawn area beside the tracks inside it is the case this exists for, and it
 * arrives in a block rather than alone. Every outline in the block is drawn:
 * each one carries its own absolute vertices, so it lands where its event put
 * it whatever else is on the map.
 */
test('one outline in a block is drawn', () => {
    const drawn = [ev('friend'), ev('hostile'), outlined('AREA-1', SQUARE)];

    expect(outlines(drawn)).toHaveLength(1);
});

test('two outlines in a block are both drawn', () => {
    const drawn = [ev('friend'), outlined('AREA-1', SQUARE), outlined('AREA-2', SQUARE)];

    expect(outlines(drawn)).toHaveLength(2);
});

test('each outline keeps its own vertices rather than sharing one ring', () => {
    const elsewhere = [
        {lat: 35.0, lon: -118.0},
        {lat: 35.1, lon: -118.1},
        {lat: 35.2, lon: -117.9},
    ];
    const drawn = [outlined('AREA-1', SQUARE), outlined('AREA-2', elsewhere)];

    const rings = outlines(drawn).map((shape) => shape.rings[0][0].lat);

    expect(rings).toEqual([21.34, 35]);
});

test('a block with no outline draws none', () => {
    expect(outlines([ev('friend'), ev('hostile')])).toHaveLength(0);
});

function ringed(uid: string, lat: string, lon: string): CotEvent {
    return {
        ...ev('unknown', lat, lon),
        uid,
        geometry: {kind: 'ellipse', majorMeters: 400, minorMeters: 250, angleDegrees: 30, note: ''},
    } as unknown as CotEvent;
}

test('an ellipse in a block is drawn around its own event, not the first marker', () => {
    const shapes = outlines([ev('friend'), ringed('RING-1', '35.0000', '-118.0000')]);

    expect(shapes).toHaveLength(1);
    expect(shapes[0].closed).toBe(true);
    const lats = shapes[0].rings[0].map((point) => point.lat);
    expect(Math.min(...lats)).toBeGreaterThan(34.99);
    expect(Math.max(...lats)).toBeLessThan(35.01);
});

test('the lone event keeps its ellipse on the map rather than as a second outline', () => {
    const lone = ringed('RING-1', '35.0000', '-118.0000');

    expect(outlines([lone], lone)).toHaveLength(0);
});
