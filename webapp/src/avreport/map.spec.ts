import {expect, test} from '@playwright/test';

import {METERS_PER_NAUTICAL_MILE, REPORT_COLOR, RESTRICTION_COLOR, areaFocus, drawsNothing, hasArea, mapLabel, placed, radiusEllipse, reportColor, reportShapes} from './map';
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

test('a TFR area becomes one closed ring in the report color', () => {
    const tfr = {...NOTAM_WITH_RADIUS, radiusNm: '', area: ['43.6167,-116.2000', '43.7500,-116.0000', '43.5000,-115.9167']};

    expect(reportShapes(tfr)).toEqual([{
        rings: [[{lat: 43.6167, lon: -116.2}, {lat: 43.75, lon: -116}, {lat: 43.5, lon: -115.9167}]],
        closed: true,
        color: REPORT_COLOR,
    }]);
    expect(mapLabel(tfr)).toContain('restricted area');
});

test('an area with a vertex this build cannot read, or too few vertices, draws no shape', () => {
    expect(reportShapes({...NOTAM_WITH_RADIUS, area: ['43.6167,-116.2000', 'nope', '43.5000,-115.9167']})).toEqual([]);
    expect(reportShapes({...NOTAM_WITH_RADIUS, area: ['43.6167,-116.2000', '43.7500,-116.0000']})).toEqual([]);
    expect(reportShapes(NOTAM_WITH_RADIUS)).toEqual([]);
});

test('a circle focuses on a box one radius around its center', () => {
    const focus = areaFocus(NOTAM_WITH_RADIUS, 3);
    const center = placed(NOTAM_WITH_RADIUS)!;

    expect(hasArea(NOTAM_WITH_RADIUS)).toBe(true);
    expect(focus?.seq).toBe(3);
    const [[west, south], [east, north]] = focus!.box!;
    expect(north - center.lat).toBeCloseTo((5 * METERS_PER_NAUTICAL_MILE) / 111320, 5);
    expect(center.lat - south).toBeCloseTo(north - center.lat, 9);
    expect(east - center.lon).toBeGreaterThan(north - center.lat);
    expect(center.lon - west).toBeCloseTo(east - center.lon, 9);
});

test('a polygon focuses on the box around its ring', () => {
    const tfr = {...NOTAM_WITH_RADIUS, radiusNm: '', area: ['43.6167,-116.2000', '43.7500,-116.0000', '43.5000,-115.9167']};

    expect(areaFocus(tfr, 1)?.box).toEqual([[-116.2, 43.5], [-115.9167, 43.75]]);
});

test('a report with no area cannot be focused', () => {
    expect(hasArea(HONOLULU_METAR)).toBe(false);
    expect(areaFocus(HONOLULU_METAR, 1)).toBeNull();
});

test('a flight restriction is drawn in red, marker, circle and area alike', () => {
    const tfr = {...NOTAM_WITH_RADIUS, rows: [{label: 'Restriction', value: 'temporary flight restriction'}, ...NOTAM_WITH_RADIUS.rows]};
    const area = {...tfr, radiusNm: '', area: ['43.6167,-116.2000', '43.7500,-116.0000', '43.5000,-115.9167']};

    expect(reportColor(tfr)).toBe(RESTRICTION_COLOR);
    expect(radiusEllipse(tfr)?.color).toBe(RESTRICTION_COLOR);
    expect(reportShapes(area)[0].color).toBe(RESTRICTION_COLOR);
});

test('any other report keeps the report color', () => {
    expect(reportColor(NOTAM_WITH_RADIUS)).toBe(REPORT_COLOR);
    expect(radiusEllipse(NOTAM_WITH_RADIUS)?.color).toBe(REPORT_COLOR);
});
