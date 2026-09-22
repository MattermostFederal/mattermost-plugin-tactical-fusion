import {expect, test} from '@playwright/test';

import {AIRPORT_MAP_KIND, airportMapFromBlob, runwayLabel, runwayShapes} from './map';

const RUNWAY = {
    designation: '08L/26R',
    ends: [{format: 'dd', value: '21.3252,-157.9430'}, {format: 'dd', value: '21.3252,-157.9070'}] as [
        {format: string; value: string}, {format: string; value: string},
    ],
};

const BLOB = {
    ident: 'PHNL',
    name: 'Daniel K. Inouye International Airport',
    coordinate: {format: 'dd', value: '21.3184,-157.9257', region: 'United States of America (Natural Earth 110m)'},
    runways: [RUNWAY],
};

test('the map kind is the one the server writes', () => {
    expect(AIRPORT_MAP_KIND).toBe('airport');
});

test.describe('runwayShapes', () => {
    test('draws one open line per runway with two ends', () => {
        const shapes = runwayShapes([RUNWAY, {}, {ends: undefined}]);

        expect(shapes).toHaveLength(1);
        expect(shapes[0].closed).toBe(false);
        expect(shapes[0].rings).toHaveLength(1);
        expect(shapes[0].rings[0]).toHaveLength(2);
        expect(shapes[0].rings[0][0].lat).toBeCloseTo(21.3252, 4);
        expect(shapes[0].rings[0][1].lon).toBeCloseTo(-157.907, 4);
        expect(shapes[0].color).toMatch(/^#[0-9a-f]{6}$/i);
    });

    test('draws nothing for an end this build cannot read', () => {
        const shapes = runwayShapes([{ends: [{format: 'dd', value: 'nowhere'}, RUNWAY.ends[1]]}]);

        expect(shapes).toHaveLength(0);
    });
});

test.describe('runwayLabel', () => {
    test('counts what is drawn', () => {
        expect(runwayLabel(0)).toBe('the airfield');
        expect(runwayLabel(1)).toBe('the airfield, with 1 runway drawn');
        expect(runwayLabel(4)).toBe('the airfield, with 4 runways drawn');
    });
});

test.describe('airportMapFromBlob', () => {
    test('reads what the server writes', () => {
        const payload = airportMapFromBlob(BLOB);

        expect(payload).not.toBeNull();
        expect(payload?.ident).toBe('PHNL');
        expect(payload?.runways).toHaveLength(1);
        expect(payload?.coordinate.value).toBe('21.3184,-157.9257');
    });

    test('accepts an empty region, which is an answer rather than an outage', () => {
        expect(airportMapFromBlob({...BLOB, coordinate: {...BLOB.coordinate, region: ''}})).not.toBeNull();
    });

    test('refuses a blob that is not the shape', () => {
        for (const blob of [
            null,
            undefined,
            'PHNL',
            [],
            {},
            {...BLOB, ident: 'phnl'},
            {...BLOB, ident: 'PHN'},
            {...BLOB, name: 5},
            {...BLOB, coordinate: null},
            {...BLOB, coordinate: {format: 'dd', value: '', region: ''}},
            {...BLOB, coordinate: {format: 'dd', value: '1,2'}},
            {...BLOB, runways: 'none'},
            {...BLOB, runways: [null]},
            {...BLOB, runways: [{designation: '09'}]},
            {...BLOB, runways: [{designation: '09', ends: [RUNWAY.ends[0]]}]},
            {...BLOB, runways: [{designation: '09', ends: [RUNWAY.ends[0], {format: 'dd', value: ''}]}]},
        ]) {
            expect(airportMapFromBlob(blob), JSON.stringify(blob)).toBeNull();
        }
    });
});
