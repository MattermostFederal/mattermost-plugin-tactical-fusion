import {expect, test} from '@playwright/test';

import {_resetForTesting as reset, _setFetchTimeoutForTesting as setFetchTimeout, readReport, request} from './client';
import {HONOLULU_METAR, propsFor} from './report_fixtures';
import {AVREPORT_PROPS_KEY} from './types';

const WIRE = propsFor(HONOLULU_METAR)[AVREPORT_PROPS_KEY] as Record<string, unknown>;

const PAYLOAD = {v: HONOLULU_METAR.src, t: HONOLULU_METAR.issuedAt};

function stubFetch(handler: (url: string) => Promise<Response> | Response): () => void {
    const real = globalThis.fetch;
    globalThis.fetch = (async (input: RequestInfo | URL) => handler(String(input))) as unknown as typeof globalThis.fetch;
    return () => {
        globalThis.fetch = real;
    };
}

function ok(body: unknown): Response {
    return {status: 200, ok: true, json: async () => body} as unknown as Response;
}

test.beforeEach(() => {
    reset();
    (globalThis as {window?: unknown}).window = {location: {origin: 'https://example.com'}};
});

test('readReport refuses an answer about a different report', () => {
    expect(() => readReport(WIRE, {...PAYLOAD, v: 'METAR KJFK 221651Z 28012KT'})).toThrow();
    expect(() => readReport(WIRE, {...PAYLOAD, t: '1'})).toThrow();
    expect(() => readReport({}, PAYLOAD)).toThrow();
    expect(readReport(WIRE, PAYLOAD).station).toBe('PHNL');
});

test('asks once for a report however many times it is wanted', async () => {
    let asked = 0;
    const restore = stubFetch(() => {
        asked += 1;
        return ok(WIRE);
    });
    try {
        expect((await request(PAYLOAD)).status).toBe('ready');
        expect((await request(PAYLOAD)).status).toBe('ready');
        expect(asked).toBe(1);
    } finally {
        restore();
    }
});

test('several callers at once share one request', async () => {
    let asked = 0;
    const restore = stubFetch(() => {
        asked += 1;
        return ok(WIRE);
    });
    try {
        const answers = await Promise.all([request(PAYLOAD), request(PAYLOAD), request(PAYLOAD)]);
        expect(answers.every((a) => a.status === 'ready')).toBe(true);
        expect(asked).toBe(1);
    } finally {
        restore();
    }
});

test('the request carries the report and its instant', async () => {
    const urls: string[] = [];
    const restore = stubFetch((url) => {
        urls.push(url);
        return ok(WIRE);
    });
    try {
        await request(PAYLOAD);
        expect(urls[0]).toContain('/api/v1/avreport?');
        expect(new URL(urls[0], 'https://example.com').searchParams.get('v')).toBe(PAYLOAD.v);
        expect(new URL(urls[0], 'https://example.com').searchParams.get('t')).toBe(PAYLOAD.t);
    } finally {
        restore();
    }
});

test('a 400 is rejected and remembered, and never asked again', async () => {
    let asked = 0;
    const restore = stubFetch(() => {
        asked += 1;
        return {status: 400, ok: false} as Response;
    });
    try {
        expect((await request(PAYLOAD)).status).toBe('rejected');
        expect((await request(PAYLOAD)).status).toBe('rejected');
        expect(asked).toBe(1);
    } finally {
        restore();
    }
});

test('never remembers an outage', async () => {
    let asked = 0;
    const restore = stubFetch(() => {
        asked += 1;
        throw new Error('offline');
    });
    try {
        expect((await request(PAYLOAD)).status).toBe('failed');
        expect((await request(PAYLOAD)).status).toBe('failed');
        expect(asked).toBe(2);
    } finally {
        restore();
    }
});

test('a server error and an answer about another report both fail', async () => {
    let restore = stubFetch(() => ({status: 500, ok: false} as Response));
    try {
        expect((await request(PAYLOAD)).status).toBe('failed');
    } finally {
        restore();
    }
    restore = stubFetch(() => ok({...WIRE, src: 'METAR KJFK 221651Z 28012KT'}));
    try {
        expect((await request(PAYLOAD)).status).toBe('failed');
    } finally {
        restore();
    }
});

test('a hung request is abandoned rather than pinning the report', async () => {
    setFetchTimeout(20);
    let calls = 0;
    const real = globalThis.fetch;
    globalThis.fetch = (async (_input: RequestInfo | URL, init?: RequestInit) => {
        calls += 1;
        return new Promise<Response>((_resolve, rejectIt) => {
            init?.signal?.addEventListener('abort', () => rejectIt(new Error('aborted')));
        });
    }) as typeof globalThis.fetch;

    try {
        expect((await request(PAYLOAD)).status).toBe('failed');

        globalThis.fetch = (async () => {
            calls += 1;
            return ok(WIRE);
        }) as typeof globalThis.fetch;
        expect((await request(PAYLOAD)).status).toBe('ready');
        expect(calls).toBe(2);
    } finally {
        globalThis.fetch = real;
    }
});
