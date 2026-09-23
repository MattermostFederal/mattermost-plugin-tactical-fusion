import {expect, test} from '@playwright/test';

import {focusOn} from './focus';

test('nothing to focus on is null', () => {
    expect(focusOn([], 1)).toBeNull();
});

test('a single position centers with no box', () => {
    expect(focusOn([{lat: 21.3353, lon: -157.9483}], 3)).toEqual({
        seq: 3, box: null, center: {lat: 21.3353, lon: -157.9483},
    });
});

test('several positions frame a box around all of them', () => {
    const focus = focusOn([{lat: 21.33, lon: -157.95}, {lat: 21.37, lon: -157.90}, {lat: 21.35, lon: -157.92}], 1);

    expect(focus?.box).toEqual([[-157.95, 21.33], [-157.90, 21.37]]);
    expect(focus?.center).toEqual({lat: 21.35, lon: -157.925});
});

test('a shape across the antimeridian is framed on the short way round', () => {
    const focus = focusOn([{lat: 0, lon: 179.5}, {lat: 1, lon: -179.5}], 1);

    expect(focus?.box).toEqual([[179.5, 0], [180.5, 1]]);
});

test('a line along a meridian keeps its box, so it is fitted rather than zoomed to its midpoint', () => {
    const focus = focusOn([{lat: 21, lon: -158}, {lat: 22, lon: -158}], 1);

    expect(focus?.box).toEqual([[-158, 21], [-158, 22]]);
});
