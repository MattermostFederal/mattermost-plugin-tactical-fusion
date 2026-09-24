import {expect, test} from '@playwright/test';

import {
    asCyber,
    request,
    _resetForTesting as reset,
    _setFetchTimeoutForTesting as setFetchTimeout,
} from './cyber';

const PUBLISHED_QUERY = 'a=&dtg=101015ZDEC21&t=1639131300000&z=Z';

const FOUND = {
    kind: 'cve',
    value: 'CVE-2021-44228',
    title: 'CVE-2021-44228',
    headline: '10.0 Critical, in KEV',
    summary: 'Remote code execution in a logging library.',
    status: '',
    rows: [
        {label: 'CVSS', value: '10.0 Critical', query: ''},
        {label: 'Published', value: '2021-12-10 10:15 UTC', query: PUBLISHED_QUERY},
    ],
    related: [{kind: 'cwe', value: 'CWE-502', label: 'CWE-502 Deserialization of Untrusted Data'}],
    watchlist: [{verdict: 'malicious', source: 'internal', note: '', updated: '2026-08-01', known: true}],
    datasets: [{name: 'cve', label: 'vulnerability', present: true, generated: '2026-09-01T00:00:00Z'}],
    score: '10.0',
    severity: 'critical',
    exploited: true,
    vector: [{metric: 'Attack vector', value: 'Network', severe: true}],
    affected: ['Apache Software Foundation Apache Log4j2: from 2.0-beta9 before 2.15.0'],
    configurations: ['apache log4j: from 2.0 before 2.3.1'],
    references: [{url: 'https://logging.apache.org/log4j/2.x/security.html', tags: 'Vendor Advisory, Patch'}],
    sections: [{title: 'Observed examples', items: [{head: 'CVE-2021-44228', text: 'x', kind: 'cve', value: 'CVE-2021-44228'}]}],
};

type Reply = (input?: RequestInfo | URL, init?: RequestInit) => Promise<Response>;

function stub(reply: Reply): () => void {
    const original = globalThis.fetch;
    globalThis.fetch = reply as unknown as typeof globalThis.fetch;

    return () => {
        globalThis.fetch = original;
    };
}

function jsonReply(body: unknown, status = 200): Response {
    return new Response(JSON.stringify(body), {
        status,
        headers: {'Content-Type': 'application/json'},
    });
}

test.beforeEach(() => {
    reset();
});

test.describe('asCyber', () => {
    test('accepts a well formed answer', () => {
        const parsed = asCyber({...FOUND});

        expect(parsed.value).toBe('CVE-2021-44228');
        expect(parsed.rows[0].label).toBe('CVSS');
        expect(parsed.related[0].value).toBe('CWE-502');
        expect(parsed.watchlist[0].known).toBe(true);
        expect(parsed.datasets[0].present).toBe(true);
        expect(parsed.affected).toEqual(FOUND.affected);
        expect(parsed.configurations).toEqual(FOUND.configurations);
        expect(parsed.references).toEqual(FOUND.references);
        expect(parsed.score).toBe('10.0');
        expect(parsed.severity).toBe('critical');
        expect(parsed.exploited).toBe(true);
        expect(parsed.vector).toEqual(FOUND.vector);
    });

    test('keeps a date-time group query the decorator accepts and drops any other', () => {
        const rows = [
            PUBLISHED_QUERY,
            'dtg=101015ZDEC21&t=-1&z=Z',
            'not=a&query',
            'a=&dtg=101015ZDEC21&t=1639131300000&z=Z&o=0',
            '',
        ].map((query) => ({label: query || 'none', value: 'v', query}));

        expect(asCyber({...FOUND, rows}).rows.map((row) => row.query)).toEqual([PUBLISHED_QUERY, '', '', '', '']);
    });

    test('keeps a section item link only when it names an indicator this plugin issues', () => {
        const items = [
            {head: 'a', text: '', kind: 'cve', value: 'CVE-2021-44228'},
            {head: 'b', text: '', kind: 'cve', value: '[REF-1]'},
            {head: 'c', text: '', kind: 'bogus', value: 'CVE-2021-44228'},
            {head: 'd', text: '', kind: '', value: ''},
        ];

        const parsed = asCyber({...FOUND, sections: [{title: 'Observed examples', items}]});

        expect(parsed.sections[0].items.map((item) => item.kind)).toEqual(['cve', '', '', '']);
        expect(parsed.sections[0].items.map((item) => item.value)).toEqual(['CVE-2021-44228', '', '', '']);
    });

    test('drops a severity it has no color for, so no badge claims one', () => {
        for (const severity of ['Critical', 'important', 'red', '']) {
            expect(asCyber({...FOUND, severity}).severity, severity).toBe('');
        }
    });

    test('reads anything but true as not exploited', () => {
        for (const exploited of ['yes', 1, null, undefined]) {
            expect(asCyber({...FOUND, exploited}).exploited, String(exploited)).toBe(false);
        }
    });

    test('keeps only web links as references, the same table the Go gate holds', () => {
        const cases = [
            // eslint-disable-next-line no-script-url
            'javascript:alert(1)',
            'data:text/html,x',
            'ftp://example.com/f',
            '//no-scheme.example/x',
            'not a url',
            'HTTPS://Example.com/Upper',
            'https://example.com/a',
            'http://example.org/b',
        ];

        const parsed = asCyber({...FOUND, references: cases.map((url) => ({url, tags: ''}))});

        expect(parsed.references.map((ref) => ref.url)).toEqual([
            'HTTPS://Example.com/Upper',
            'https://example.com/a',
            'http://example.org/b',
        ]);
    });

    test('refuses anything that is not the shape', () => {
        const cases: Array<[string, unknown]> = [
            ['null', null],
            ['a string', 'CVE-2021-44228'],
            ['an array', []],
            ['no kind', {...FOUND, kind: undefined}],
            ['a numeric value', {...FOUND, value: 7}],
            ['rows that are not an array', {...FOUND, rows: {}}],
            ['a row with no label', {...FOUND, rows: [{value: 'x', query: ''}]}],
            ['a row with no query', {...FOUND, rows: [{label: 'l', value: 'x'}]}],
            ['a link with no kind', {...FOUND, related: [{value: 'x', label: 'y'}]}],
            ['a watchlist entry with no verdict', {...FOUND, watchlist: [{source: 's', note: '', updated: '', known: true}]}],
            ['a dataset with no name', {...FOUND, datasets: [{label: 'l', present: true, generated: ''}]}],
            ['no affected list', {...FOUND, affected: undefined}],
            ['an affected entry that is not text', {...FOUND, affected: [7]}],
            ['configurations that are not an array', {...FOUND, configurations: 'apache'}],
            ['no references', {...FOUND, references: undefined}],
            ['no score', {...FOUND, score: undefined}],
            ['no vector', {...FOUND, vector: undefined}],
            ['no sections', {...FOUND, sections: undefined}],
            ['a section item with no text', {...FOUND, sections: [{title: 't', items: [{head: 'h', kind: '', value: ''}]}]}],
            ['a vector metric with no value', {...FOUND, vector: [{metric: 'Attack vector', severe: true}]}],
            ['a severity that is not text', {...FOUND, severity: 3}],
            ['a reference with no url', {...FOUND, references: [{tags: 'Patch'}]}],
        ];

        for (const [name, body] of cases) {
            expect(() => asCyber(body), name).toThrow();
        }
    });

    test('a missing present flag reads as absent rather than as true', () => {
        const parsed = asCyber({
            ...FOUND,
            datasets: [{name: 'cve', label: 'vulnerability', present: 'yes', generated: ''}],
        });

        expect(parsed.datasets[0].present).toBe(false);
    });
});

test.describe('request', () => {
    test('shares one request between callers asking at the same time', async () => {
        let calls = 0;
        const restore = stub(async () => {
            calls++;
            return jsonReply(FOUND);
        });

        try {
            const [first, second] = await Promise.all([
                request('cve', 'CVE-2021-44228'),
                request('cve', 'CVE-2021-44228'),
            ]);

            expect(calls).toBe(1);
            expect(first.status).toBe('ready');
            expect(second.status).toBe('ready');
        } finally {
            restore();
        }
    });

    test('remembers an answer the server decided', async () => {
        let calls = 0;
        const restore = stub(async () => {
            calls++;
            return jsonReply(FOUND);
        });

        try {
            await request('cve', 'CVE-2021-44228');
            await request('cve', 'CVE-2021-44228');

            expect(calls).toBe(1);
        } finally {
            restore();
        }
    });

    test('keys the cache on the kind as well as the value', async () => {
        let calls = 0;
        const restore = stub(async () => {
            calls++;
            return jsonReply({...FOUND, kind: 'cwe', value: 'CWE-79'});
        });

        try {
            await request('cwe', 'CWE-79');
            await request('cve', 'CWE-79');

            expect(calls).toBe(2);
        } finally {
            restore();
        }
    });

    test('a 400 is the server rejecting the link, and is remembered', async () => {
        let calls = 0;
        const restore = stub(async () => {
            calls++;
            return jsonReply({message: 'no'}, 400);
        });

        try {
            const state = await request('cve', 'CVE-2021-44228');
            await request('cve', 'CVE-2021-44228');

            expect(state.status).toBe('rejected');
            expect(calls).toBe(1);
        } finally {
            restore();
        }
    });

    test('an outage is never remembered', async () => {
        let calls = 0;
        const restore = stub(async () => {
            calls++;
            throw new Error('offline');
        });

        try {
            const state = await request('cve', 'CVE-2021-44228');
            await request('cve', 'CVE-2021-44228');

            expect(state.status).toBe('failed');
            expect(calls).toBe(2);
        } finally {
            restore();
        }
    });

    test('a 500 is an outage rather than a verdict', async () => {
        const restore = stub(async () => jsonReply({}, 500));

        try {
            expect((await request('cve', 'CVE-2021-44228')).status).toBe('failed');
        } finally {
            restore();
        }
    });

    test('refuses an answer about a different indicator', async () => {
        const restore = stub(async () => jsonReply({...FOUND, value: 'CVE-1999-0001'}));

        try {
            expect((await request('cve', 'CVE-2021-44228')).status).toBe('failed');
        } finally {
            restore();
        }
    });

    test('a request that never settles gives up rather than hanging forever', async () => {
        setFetchTimeout(20);

        const original = globalThis.fetch;
        globalThis.fetch = (async (_input: RequestInfo | URL, init?: RequestInit) => new Promise<Response>(
            (_resolve, rejectIt) => {
                init?.signal?.addEventListener('abort', () => rejectIt(new Error('aborted')));
            },
        )) as typeof globalThis.fetch;

        try {
            expect((await request('cve', 'CVE-2021-44228')).status).toBe('failed');

            globalThis.fetch = (async () => jsonReply(FOUND)) as typeof globalThis.fetch;

            expect((await request('cve', 'CVE-2021-44228')).status).toBe('ready');
        } finally {
            globalThis.fetch = original;
        }
    });

    test('asks the route the server serves, carrying both parameters', async () => {
        let seen = '';
        const restore = stub(async (input?: RequestInfo | URL) => {
            seen = String(input);
            return jsonReply(FOUND);
        });

        try {
            await request('cve', 'CVE-2021-44228');

            expect(seen).toContain('/api/v1/cyber?');
            expect(seen).toContain('k=cve');
            expect(seen).toContain('v=CVE-2021-44228');
        } finally {
            restore();
        }
    });
});
