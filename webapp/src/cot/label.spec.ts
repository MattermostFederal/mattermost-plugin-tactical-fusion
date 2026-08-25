import {expect, test} from '@playwright/test';

import {_blockLabelForTesting as blockLabel, _drawableEventsForTesting as drawableEvents} from './CotMap';
import type {CotEvent} from './types';

function ev(affiliation: string, lat = '21.3353', lon = '-157.9483'): CotEvent {
    return {affiliation, lat, lon, format: 'dd', value: `${lat},${lon}`} as CotEvent;
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
 * An affiliation this build does not colour is still named, because the marker
 * is still drawn, in grey.
 */
test('names an affiliation that carries no colour', () => {
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
