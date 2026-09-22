import {expect, test} from '@playwright/test';

import {METERS_PER_NAUTICAL_MILE, drawsNothing, mapLabel, placed, radiusEllipse} from './map';
import {HONOLULU_METAR, NOTAM_WITH_RADIUS} from './report_fixtures';

test.beforeEach(() => {
    (globalThis as {window?: unknown}).window = {location: {origin: 'https://example.com'}};
});

test('places a report at its station', () => {
    expect(placed(HONOLULU_METAR)).toEqual({lat: 21.3184, lon: -157.9257});
    expect(drawsNothing(HONOLULU_METAR)).toBe(false);
});

test('draws nothing for a report with no position, or one this build cannot read', () => {
    expect(placed({...HONOLULU_METAR, format: '', value: ''})).toBeNull();
    expect(placed({...HONOLULU_METAR, format: 'nope', value: '1,2'})).toBeNull();
    expect(drawsNothing({...HONOLULU_METAR, format: '', value: ''})).toBe(true);
});

test('draws nothing past the Mercator limit rather than pinning at the edge', () => {
    expect(placed({...HONOLULU_METAR, value: '-90.0000,-1.0000'})).toBeNull();
    expect(placed({...HONOLULU_METAR, value: '85.0000,-1.0000'})).not.toBeNull();
});

test('a radius becomes an ellipse in meters', () => {
    expect(radiusEllipse(NOTAM_WITH_RADIUS)).toEqual({
        major: 5 * METERS_PER_NAUTICAL_MILE,
        minor: 5 * METERS_PER_NAUTICAL_MILE,
        angle: 0,
        color: '#2e7d9a',
    });
});

test('a radius that is empty, zero, negative or not a number draws no ellipse', () => {
    for (const radiusNm of ['', '0', '-3', 'five', '1e999', 'NaN']) {
        expect(radiusEllipse({...NOTAM_WITH_RADIUS, radiusNm}), radiusNm).toBeUndefined();
    }
});

test('the label names the station and only a radius that is drawn', () => {
    expect(mapLabel(HONOLULU_METAR)).toBe('Daniel K. Inouye International Airport (PHNL)');
    expect(mapLabel(NOTAM_WITH_RADIUS)).toBe('Daniel K. Inouye International Airport (PHNL), 5 NM radius');
    expect(mapLabel({...NOTAM_WITH_RADIUS, radiusNm: '0'})).toBe('Daniel K. Inouye International Airport (PHNL)');
    expect(mapLabel({...HONOLULU_METAR, stationName: ''})).toBe('METAR PHNL');
});
