import {expect, test} from '@playwright/test';

import {
    asAirport,
    request,
    _resetForTesting as reset,
    _setFetchTimeoutForTesting as setFetchTimeout,
} from './airport';

const COORDINATE = {format: 'dd', value: '39.7173,-86.2944', region: 'United States of America (Natural Earth 110m)'};

const RUNWAY = {
    designation: '05L/23R',
    summary: '11,200 x 150 ft, Concrete',
    length: '11,200 ft',
    width: '150 ft',
    surface: 'Concrete',
    lighted: '',
    closed: '',
    ends: [{format: 'dd', value: '39.7009,-86.3054'}, {format: 'dd', value: '39.7263,-86.2733'}],
};

const FREQUENCY = {type: 'TWR', description: '', mhz: '120.900'};

const DETAILS = {
    name: 'Indianapolis International Airport',
    type: 'Large Airport',
    place: 'Indianapolis, IN, US',
    elevation: '797 ft',
    iata: 'IND',
    military: '',
    runways: [RUNWAY],
    frequencies: [FREQUENCY],
};

const FOUND = {found: true, ident: 'KIND', iata: 'IND', airport: DETAILS, coordinate: COORDINATE};

const ICAO = {key: 'icao' as const, code: 'KIND'};

/*
 * The wire validation. A captive portal or a transparent proxy answers 200 with
 * something else entirely, and an unchecked cast would render undefined as an
 * airfield name.
 */

test('accepts a well formed airfield', () => {
    const parsed = asAirport({...FOUND});
    expect(parsed.found).toBe(true);
    expect(parsed.airport?.name).toBe('Indianapolis International Airport');
    expect(parsed.coordinate?.value).toBe('39.7173,-86.2944');
});

test('accepts an ident this build does not hold', () => {
    const parsed = asAirport({found: false, ident: 'QZQZ', iata: ''});
    expect(parsed.found).toBe(false);
    expect(parsed.airport).toBeUndefined();
    expect(parsed.coordinate).toBeUndefined();
});

// The whole reason the shape is discriminated. A flat record would carry an
// empty coordinate, and an empty token would open a view that refuses it.
test('a not-found answer never carries a coordinate', () => {
    const parsed = asAirport({found: false, ident: 'QZQZ', iata: ''});
    expect(parsed.coordinate).toBeUndefined();
});

// An airfield with no usable position is a third state, not an error.
test('accepts a found airfield with no coordinate', () => {
    const parsed = asAirport({found: true, ident: 'KIND', iata: 'IND', airport: DETAILS});
    expect(parsed.found).toBe(true);
    expect(parsed.coordinate).toBeUndefined();
});

// An empty token would send the reader to a view that refuses it, which is
// worse than offering no link at all.
test('refuses an empty or malformed coordinate', () => {
    for (const coordinate of [
        {format: '', value: 'x', region: ''},
        {format: 'dd', value: '', region: ''},
        {format: 'dd', region: ''},
        {format: 'dd', value: '39.7173,-86.2944'},
        'dd',
    ]) {
        expect(() => asAirport({found: true, ident: 'KIND', iata: 'IND', airport: DETAILS, coordinate})).toThrow();
    }
});

test('accepts an empty elevation, which means the database states none', () => {
    const parsed = asAirport({...FOUND, airport: {...DETAILS, elevation: ''}});
    expect(parsed.airport?.elevation).toBe('');
});

test('refuses a body that is not an airfield', () => {
    for (const body of [null, undefined, 'KIND', 42, [], {}, {found: 'yes', ident: 'KIND'}]) {
        expect(() => asAirport(body)).toThrow();
    }
});

test('refuses a found airfield with a missing field', () => {
    for (const key of ['name', 'type', 'place', 'elevation', 'iata', 'military', 'runways', 'frequencies']) {
        const airport: Record<string, unknown> = {...DETAILS};
        delete airport[key];
        expect(() => asAirport({found: true, ident: 'KIND', iata: 'IND', airport})).toThrow();
    }
});

// A name that happens to exist on Object.prototype must read as absent rather
// than inherited. This repo has been bitten by exactly that once already.
test('refuses a field inherited from the prototype chain', () => {
    // The inherited key has to be one of the fields asDetails CHECKS, or the
    // test passes for the wrong reason: with toString on the prototype it
    // throws because the own `name` is missing, which a plain typeof check
    // would also do, so it never discriminates inherited from own.
    const airport = Object.create({name: 'inherited'}) as Record<string, unknown>;
    Object.assign(airport, {...DETAILS});
    delete airport.name;

    expect(() => asAirport({found: true, ident: 'KIND', iata: 'IND', airport})).toThrow();
});

/*
 * The one entry point that used to take the server's word for the code.
 *
 * Everything else enforces four upper-case letters: the Go scan pattern, Parse,
 * the page, the API, and fromParams on the way in from a link. A 200 from a
 * captive portal or a stale route could put an empty or mismatched code into
 * the panel's heading, its Code row and its copy button, while the hover beside
 * it read the code out of the link and showed the right one.
 */
test('refuses an answer whose code is not four upper-case letters', () => {
    for (const ident of ['', 'kind', 'KIN', 'KINDX', 'K1ND']) {
        expect(() => asAirport({...FOUND, ident}), ident).toThrow();
    }
});

test('accepts a not-found answer to an IATA question, which names no ident', () => {
    const parsed = asAirport({found: false, ident: '', iata: 'QQQ'});
    expect(parsed.found).toBe(false);
    expect(parsed.iata).toBe('QQQ');
});

test('refuses a malformed IATA code or ident in the answer', () => {
    expect(() => asAirport({...FOUND, iata: 'ind'})).toThrow();
    expect(() => asAirport({...FOUND, iata: 'INDY'})).toThrow();
    expect(() => asAirport({found: false, ident: 'kind', iata: ''})).toThrow();
    expect(() => asAirport({found: false, ident: 'KIND'})).toThrow();
});

test('accepts an airfield with no runways and no frequencies', () => {
    const parsed = asAirport({...FOUND, airport: {...DETAILS, runways: [], frequencies: []}});
    expect(parsed.airport?.runways).toEqual([]);
});

test('refuses a runway or frequency that is not the shape', () => {
    for (const runways of [
        'none',
        [null],
        [{...RUNWAY, summary: 5}],
        [{...RUNWAY, ends: [RUNWAY.ends[0]]}],
        [{...RUNWAY, ends: [RUNWAY.ends[0], {format: 'dd', value: ''}]}],
        [{...RUNWAY, ends: 'both'}],
    ]) {
        expect(() => asAirport({...FOUND, airport: {...DETAILS, runways}}), JSON.stringify(runways)).toThrow();
    }
    for (const frequencies of ['none', [null], [{type: 'TWR', mhz: '120.900'}], [{...FREQUENCY, mhz: 120.9}]]) {
        expect(() => asAirport({...FOUND, airport: {...DETAILS, frequencies}}), JSON.stringify(frequencies)).toThrow();
    }
});

test('a runway with no ends is kept without them', () => {
    const {ends, ...bare} = RUNWAY;
    expect(ends).toBeDefined();
    const parsed = asAirport({...FOUND, airport: {...DETAILS, runways: [bare]}});
    expect(parsed.airport?.runways[0].ends).toBeUndefined();
});

test('an IATA question is answered by the code asked for, not the ident', async () => {
    reset();

    const real = globalThis.fetch;
    const asked: string[] = [];
    globalThis.fetch = (async (input: RequestInfo | URL) => {
        asked.push(String(input));
        return {status: 200, ok: true, json: async () => FOUND} as unknown as Response;
    }) as unknown as typeof globalThis.fetch;

    try {
        expect((await request({key: 'iata', code: 'IND'})).status).toBe('ready');
        expect(asked[0]).toContain('i=IND');
        expect((await request({key: 'iata', code: 'HNL'})).status).toBe('failed');
    } finally {
        globalThis.fetch = real;
        reset();
    }
});

test('refuses an answer about a different airfield', async () => {
    reset();

    const real = globalThis.fetch;
    globalThis.fetch = (async () => ({
        status: 200,
        ok: true,
        json: async () => ({...FOUND, ident: 'KJFK'}),
    })) as unknown as typeof globalThis.fetch;

    try {
        // An outage rather than a verdict, so it is not cached and the next
        // caller asks again.
        expect((await request(ICAO)).status).toBe('failed');
    } finally {
        globalThis.fetch = real;
        reset();
    }
});

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

        return {status: 200, ok: true, json: async () => FOUND} as unknown as Response;
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

/*
 * A request that HANGS is abandoned rather than pinning the code forever.
 *
 * A stalled fetch never rejects, so without the timeout `inflight` is never
 * cleared: the hover starts the request, the click that follows joins the same
 * pending promise, and the panel sits on "Looking up this airfield…" for the
 * life of the tab with a reload the only way back. The panel has nothing local
 * to fall back to, so there is nothing else on screen while it does.
 */
test('a hung request is abandoned rather than pinning the code', async () => {
    reset();
    setFetchTimeout(20);

    const real = globalThis.fetch;
    let calls = 0;

    // Never settles on its own, so the abort signal is the only thing that can
    // end it. The stub has to honor the signal the way fetch does, or this
    // tests a slow reply rather than the timeout.
    globalThis.fetch = (async (_input: RequestInfo | URL, init?: RequestInit) => {
        calls++;
        return new Promise<Response>((_resolve, rejectIt) => {
            init?.signal?.addEventListener('abort', () => rejectIt(new Error('aborted')));
        });
    }) as typeof globalThis.fetch;

    try {
        expect((await request(ICAO)).status).toBe('failed');

        // And the code is not pinned: the next caller issues a fresh request
        // rather than joining the abandoned one.
        globalThis.fetch = (async () => {
            calls++;
            return {status: 200, ok: true, json: async () => FOUND} as unknown as Response;
        }) as typeof globalThis.fetch;

        expect((await request(ICAO)).status).toBe('ready');
        expect(calls).toBe(2);
    } finally {
        globalThis.fetch = real;
        reset();
    }
});

/*
 * Headers can arrive and the BODY still stall, and fetch resolves on the
 * headers. A bound that stops at the fetch therefore leaves the body read
 * unguarded, which is the same permanent-stall defect one step further down:
 * inflight is never cleared and every later caller for that code joins the
 * dead promise for the life of the tab.
 */
test('a stalled response body is abandoned too', async () => {
    reset();
    setFetchTimeout(20);

    const real = globalThis.fetch;
    globalThis.fetch = (async (_input: RequestInfo | URL, init?: RequestInit) => ({
        status: 200,
        ok: true,
        json: () => new Promise((_resolve, rejectIt) => {
            init?.signal?.addEventListener('abort', () => rejectIt(new Error('aborted')));
        }),
    })) as unknown as typeof globalThis.fetch;

    try {
        expect((await request(ICAO)).status).toBe('failed');
    } finally {
        globalThis.fetch = real;
        reset();
    }
});

test.describe('the airfield cache', () => {
    test.beforeEach(() => reset());
    test.afterEach(() => reset());

    test('asks once for a code however many times it is wanted', async () => {
        const stub = stubFetch();
        try {
            await request(ICAO);
            await request(ICAO);
            await request(ICAO);
            expect(stub.calls()).toBe(1);
        } finally {
            stub.restore();
        }
    });

    // The click that follows a hover arrives while the hover's own fetch is
    // still outstanding, and must join it rather than issue a second.
    test('several callers at once share one request', async () => {
        const stub = stubFetch();
        try {
            await Promise.all([request(ICAO), request(ICAO), request(ICAO)]);
            expect(stub.calls()).toBe(1);
        } finally {
            stub.restore();
        }
    });

    test('remembers a verdict about a code this build does not hold', async () => {
        const stub = stubFetch();
        try {
            stub.fail('reject');
            expect((await request(ICAO)).status).toBe('rejected');
            await request(ICAO);
            expect(stub.calls()).toBe(1);
        } finally {
            stub.restore();
        }
    });

    // Caching an outage would cost every airfield in the channel for the life
    // of the tab, with a reload the only way back.
    test('never remembers an outage', async () => {
        const stub = stubFetch();
        try {
            stub.fail('net');
            expect((await request(ICAO)).status).toBe('failed');
            expect((await request(ICAO)).status).toBe('failed');
            expect(stub.calls()).toBe(2);

            stub.fail(null);
            expect((await request(ICAO)).status).toBe('ready');
        } finally {
            stub.restore();
        }
    });

    test('asks separately for separate codes', async () => {
        const stub = stubFetch();
        try {
            await request(ICAO);
            await request({key: 'icao', code: 'KLAX'});
            expect(stub.calls()).toBe(2);
        } finally {
            stub.restore();
        }
    });
});
