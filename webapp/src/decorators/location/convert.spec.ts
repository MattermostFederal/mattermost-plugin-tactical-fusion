import {expect, test} from '@playwright/test';

import {
    asConversion,
    _requestForTesting as requestForTesting,
    _resetConversionsForTesting as resetConversions,
} from './convert';

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

/*
 * The conversion cache, which is what let the location decorator have a hover.
 *
 * A hover fires on pointer movement rather than on a click, so an uncached
 * conversion puts a request behind every coordinate a cursor crosses in a busy
 * channel. These pin the three properties that makes acceptable: one request per
 * token, one request when several components ask at once, and no remembering of
 * an outage.
 */

function stubFetch(): {calls: () => number; restore: () => void; fail: (why: 'net' | 'reject' | null) => void} {
    const real = globalThis.fetch;
    let count = 0;
    let mode: 'net' | 'reject' | null = null;

    globalThis.fetch = (async () => {
        count++;
        if (mode === 'net') {
            throw new Error('offline');
        }
        if (mode === 'reject') {
            return {status: 400, ok: false} as Response;
        }

        return {status: 200, ok: true, json: async () => VALID} as unknown as Response;
    }) as typeof globalThis.fetch;

    return {
        calls: () => count,
        restore: () => {
            globalThis.fetch = real;
        },
        fail: (why) => {
            mode = why;
        },
    };
}

test.describe('the conversion cache', () => {
    test.beforeEach(() => resetConversions());
    test.afterEach(() => resetConversions());

    test('asks once for a token however many times it is wanted', async () => {
        const stub = stubFetch();
        try {
            await requestForTesting('mgrs', '11S LT 8463 6908', '');
            await requestForTesting('mgrs', '11S LT 8463 6908', '');
            await requestForTesting('mgrs', '11S LT 8463 6908', '');

            expect(stub.calls()).toBe(1);
        } finally {
            stub.restore();
        }
    });

    // The click that follows a hover arrives while the hover's fetch is still
    // outstanding. It has to join that request rather than start a second.
    test('several askers at once share one request', async () => {
        const stub = stubFetch();
        try {
            await Promise.all([
                requestForTesting('mgrs', '11S LT 8463 6908', ''),
                requestForTesting('mgrs', '11S LT 8463 6908', ''),
                requestForTesting('mgrs', '11S LT 8463 6908', ''),
            ]);

            expect(stub.calls()).toBe(1);
        } finally {
            stub.restore();
        }
    });

    test('keeps a different token apart', async () => {
        const stub = stubFetch();
        try {
            await requestForTesting('mgrs', '11S LT 8463 6908', '');
            await requestForTesting('mgrs', '11S LT 8463 6909', '');

            expect(stub.calls()).toBe(2);
        } finally {
            stub.restore();
        }
    });

    /*
     * An outage is never remembered, and this is the important one.
     *
     * Caching a failure would mean one bad minute costs every coordinate in the
     * channel for the life of the tab, with a reload the only way back. A
     * verdict is different: the server looked at the token and answered about
     * it, and that answer cannot change under us.
     */
    test('does not remember an outage, and does remember a verdict', async () => {
        const stub = stubFetch();
        try {
            stub.fail('net');
            expect((await requestForTesting('dd', '34.0561,-118.2500', '')).status).toBe('failed');
            expect((await requestForTesting('dd', '34.0561,-118.2500', '')).status).toBe('failed');
            expect(stub.calls()).toBe(2);

            stub.fail(null);
            expect((await requestForTesting('dd', '34.0561,-118.2500', '')).status).toBe('ready');
            expect(stub.calls()).toBe(3);

            await requestForTesting('dd', '34.0561,-118.2500', '');
            expect(stub.calls()).toBe(3);

            stub.fail('reject');
            expect((await requestForTesting('dd', '9,9', '')).status).toBe('rejected');
            await requestForTesting('dd', '9,9', '');
            expect(stub.calls()).toBe(4);
        } finally {
            stub.restore();
        }
    });
});
