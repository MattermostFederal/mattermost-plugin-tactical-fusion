import {expect, test} from '@playwright/test';

import {_asConversionForTesting as asConversion} from './convert';

/*
 * The wire validation. `response.ok` is any 2xx, and a captive portal or a
 * transparent proxy, which is the ordinary DDIL failure, answers 200 with
 * something else entirely. An unchecked cast leaves lat undefined and puts the
 * pin at NaN.
 */

const VALID = {
    mgrs: '11S LT 8463 6908',
    utm: '11S 384640E 3769080N',
    decimal: '34.0561° N, 118.2500° W',
    dms: '34°03\'22.0"N 118°15\'00.0"W',
    ddm: '34°03.366\'N 118°15.000\'W',
    usmtf: '340322.0N1181500.0W',
    region: 'United States of America (Natural Earth 110m)',
    lat: 34.0561,
    lon: -118.25,
};

test('accepts a well formed conversion', () => {
    expect(asConversion({...VALID}).lat).toBeCloseTo(34.0561, 6);
});

test('accepts null island, which is a position like any other', () => {
    // The bug this repo declined to inherit is a truthiness check that drops the
    // equator and the prime meridian. A tidy-up to `if (!lat || !lon)` reads as
    // equivalent and would fail only here.
    const parsed = asConversion({...VALID, lat: 0, lon: 0});

    expect(parsed.lat).toBe(0);
    expect(parsed.lon).toBe(0);
});

test('refuses a body with no position', () => {
    expect(() => asConversion({...VALID, lat: undefined})).toThrow();
    expect(() => asConversion({...VALID, lon: undefined})).toThrow();
});

test('refuses a non-finite position', () => {
    expect(() => asConversion({...VALID, lat: NaN})).toThrow();
    expect(() => asConversion({...VALID, lon: Infinity})).toThrow();
});

test('refuses a position outside the world', () => {
    expect(() => asConversion({...VALID, lat: 91})).toThrow();
    expect(() => asConversion({...VALID, lat: -91})).toThrow();
    expect(() => asConversion({...VALID, lon: 181})).toThrow();
});

test('refuses a body that is not a conversion at all', () => {
    expect(() => asConversion(null)).toThrow();
    expect(() => asConversion('<html>Sign in to the network</html>')).toThrow();
    expect(() => asConversion([])).toThrow();
});

test('refuses a reading that is not a string', () => {
    expect(() => asConversion({...VALID, mgrs: 42})).toThrow();
    expect(() => asConversion({...VALID, region: null})).toThrow();
});
