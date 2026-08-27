import {expect, test} from '@playwright/test';

import {
    _drawableCellForTesting as drawableCell,
    _positionNoteForTesting as positionNote,
} from './LocationMap';

/*
 * The cell is always drawn, and its SIZE carries the resolution.
 *
 * There was a 6px floor here once, measured at the OPENING camera and never
 * recomputed because nothing listens for zoom. It dropped every square finer
 * than about 45 km permanently: a 10 km grid reference drew a bare dot at every
 * zoom including the maximum, where it would have been 11px across. These
 * surfaces zoom, so there is no one scale to test a cell against.
 */

const AT = {lat: 38.8895, lon: -77.0353};

test.describe('every resolution yields a cell', () => {
    const cases: Array<[string, number]> = [
        ['a degree, about 111 km', 1],
        ['ten kilometers', 10000 / 111320],
        ['one kilometer', 1000 / 111320],
        ['one meter', 1 / 111320],
    ];

    for (const [name, degrees] of cases) {
        test(name, () => {
            const cell = drawableCell({...AT, cellDegLat: degrees, cellDegLon: degrees});

            expect(cell.features, name).toHaveLength(1);
        });
    }
});

test('a token with no resolution draws no cell', () => {
    expect(drawableCell({...AT, cellDegLat: 0, cellDegLon: 0}).features).toHaveLength(0);
});

test('a position that is not known draws no cell', () => {
    expect(drawableCell({
        lat: null, lon: null, cellDegLat: 1, cellDegLon: 1,
    }).features).toHaveLength(0);
});

/*
 * One expression decides what the reader is told. It was assigned from three
 * effects and two event handlers, and the load handler set it to null
 * unconditionally while applyView was clearing the pin, so the two disagreed:
 * a map with no pin and nothing saying why.
 */
test.describe('what the reader is told', () => {
    const TOO_FAR = 'This position is too far north for the map.';

    test('a position off the projection says so, whatever else is true', () => {
        expect(positionNote(TOO_FAR, false, true, false)).toBe(TOO_FAR);
        expect(positionNote(TOO_FAR, true, false, true)).toBe(TOO_FAR);
    });

    test('an unknown position distinguishes waiting from unavailable', () => {
        expect(positionNote(null, false, true, true)).toBe('Loading map…');
        expect(positionNote(null, false, false, true)).toBe(
            'The position for this coordinate is unavailable.');
    });

    // The regression: a map that finished loading after the reader moved on used
    // to leave this stuck, because readiness lived in a ref the note could not
    // re-render on.
    test('a known position waits for the map and then says nothing', () => {
        expect(positionNote(null, true, false, false)).toBe('Loading map…');
        expect(positionNote(null, true, false, true)).toBeNull();
    });
});
