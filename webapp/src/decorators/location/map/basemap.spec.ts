import fs from 'fs';
import path from 'path';

import {expect, test} from '@playwright/test';
import manifest from 'manifest';

import {basemapUrl, loadBasemap, loadPackages, packageUrl, _resetForTesting} from './basemap';
import {DATA_MAX_ZOOM, SEAM_ZOOM} from './span';

/*
 * The real archive, off disk.
 *
 * Only its first 127 bytes are ever read, but they are the real ones: a
 * hand-written header could only ever test the rejection branches, and the point
 * of this module is that it accepts the archive this build actually ships.
 */
const HEADER_BYTES = 127;

const REAL_ARCHIVE = fs.readFileSync(
    path.resolve(__dirname, '../../../../../public/map/world.pmtiles'),
);

function realHeader(): ArrayBuffer {
    const slice = REAL_ARCHIVE.subarray(0, HEADER_BYTES);

    return slice.buffer.slice(slice.byteOffset, slice.byteOffset + slice.byteLength) as ArrayBuffer;
}

/** The real header with one byte changed, which is how each rejection is provoked. */
function headerWith(offset: number, value: number): ArrayBuffer {
    const bytes = new Uint8Array(realHeader());
    bytes[offset] = value;

    return bytes.buffer as ArrayBuffer;
}

function bytesOf(value: string): ArrayBuffer {
    return new TextEncoder().encode(value).buffer as ArrayBuffer;
}

interface Reply {
    ok?: boolean;
    status?: number;
    bytes?: ArrayBuffer;
    throws?: boolean;

    /** What the server claims it is sending, which is how byte serving is detected. */
    contentLength?: string | null;
}

const realFetch = globalThis.fetch;

/**
 * Replaces global fetch and records every call.
 *
 * Only `ok`, `status` and `arrayBuffer` are implemented, so a module that starts
 * reading anything else fails loudly here rather than quietly seeing undefined.
 */
function stubFetch(reply: (n: number) => Reply): string[] {
    const calls: string[] = [];

    globalThis.fetch = ((url: string) => {
        const answer = reply(calls.length);
        calls.push(String(url));

        if (answer.throws) {
            return Promise.reject(new Error('offline'));
        }

        const length = answer.contentLength === undefined ? String(HEADER_BYTES) : answer.contentLength;

        return Promise.resolve({
            ok: answer.ok ?? true,
            status: answer.status ?? 206,
            headers: {get: (name: string) => (name === 'Content-Length' ? length : null)},
            arrayBuffer: () => Promise.resolve(answer.bytes ?? realHeader()),
        });
    }) as unknown as typeof globalThis.fetch;

    return calls;
}

type Global = {window?: {basename?: string}};

test.beforeEach(() => {
    _resetForTesting();
});

test.afterEach(() => {
    globalThis.fetch = realFetch;
    delete (globalThis as Global).window;
});

test.describe('basemapUrl', () => {
    test('points at the bundled archive', () => {
        expect(basemapUrl()).toBe(
            `/plugins/${manifest.id}/public/map/world.pmtiles?v=${encodeURIComponent(manifest.version)}`,
        );
    });

    // The version is what busts a reader's cache when the archive is
    // regenerated, so a build that stopped carrying it would serve last
    // release's world from disk forever.
    test('carries the build version', () => {
        expect(basemapUrl()).toContain(`v=${encodeURIComponent(manifest.version)}`);
    });

    test('inherits the subpath Mattermost is served from', () => {
        (globalThis as Global).window = {basename: '/mattermost'};

        expect(basemapUrl()).toContain(`/mattermost/plugins/${manifest.id}/`);
    });
});

test.describe('probing', () => {
    test('accepts the archive this build ships', async () => {
        const calls = stubFetch(() => ({}));

        const archive = await loadBasemap();

        expect(calls).toHaveLength(1);
        expect(archive?.minZoom).toBe(0);
        expect(archive?.maxZoom).toBeGreaterThanOrEqual(DATA_MAX_ZOOM);
        expect(archive?.url).toBe(basemapUrl());
    });

    // Only the header is wanted, and the archive is a great deal larger than it.
    test('asks for the header alone', async () => {
        let range = '';
        globalThis.fetch = ((_url: string, init: {headers?: Record<string, string>}) => {
            range = init.headers?.Range ?? '';
            return Promise.resolve({
                ok: true,
                status: 206,
                headers: {get: () => String(HEADER_BYTES)},
                arrayBuffer: () => Promise.resolve(realHeader()),
            });
        }) as unknown as typeof globalThis.fetch;

        await loadBasemap();

        expect(range).toBe(`bytes=0-${HEADER_BYTES - 1}`);
    });

    test('sends no session with a static asset', async () => {
        let credentials = '';
        globalThis.fetch = ((_url: string, init: {credentials?: string}) => {
            credentials = init.credentials ?? '';
            return Promise.resolve({
                ok: true,
                status: 206,
                headers: {get: () => String(HEADER_BYTES)},
                arrayBuffer: () => Promise.resolve(realHeader()),
            });
        }) as unknown as typeof globalThis.fetch;

        await loadBasemap();

        expect(credentials).toBe('omit');
    });

    // A 200 whose body is only the requested bytes is a well-behaved backend
    // answering without the 206 status. That is usable.
    test('accepts a 200 that still honored the range', async () => {
        stubFetch(() => ({ok: true, status: 200, contentLength: String(HEADER_BYTES)}));

        expect(await loadBasemap()).not.toBeNull();
    });

    // Module state with no TTL: the archive is immutable for the life of an
    // install, so a channel full of coordinates makes one request rather than
    // one per panel.
    test('is probed once and served from memory thereafter', async () => {
        const calls = stubFetch(() => ({}));

        const first = await loadBasemap();
        const second = await loadBasemap();
        const third = await loadBasemap();

        expect(calls).toHaveLength(1);
        expect(second).toBe(first);
        expect(third).toBe(first);
    });

    // Two panels opening at once must not each start a request.
    test('concurrent callers share one request', async () => {
        const calls = stubFetch(() => ({}));

        const [first, second] = await Promise.all([loadBasemap(), loadBasemap()]);

        expect(calls).toHaveLength(1);
        expect(second).toBe(first);
    });
});

/*
 * A definitive failure is a property of the deploy and will not change, so it is
 * remembered: a broken deploy must not become a request loop from every open
 * panel.
 */
test.describe('a definitive failure is remembered', () => {
    const cases: Array<[string, Reply]> = [
        ['a missing archive', {ok: false, status: 404}],

        // What a 404 page, an SPA index or a captive portal actually returns.
        // None of these are an archive and none should reach MapLibre.
        ['a body that is not an archive', {bytes: bytesOf('<!doctype html><title>404</title>')}],
        ['a truncated header', {bytes: bytesOf('PMTiles')}],
        ['a spec version this reader does not know', {bytes: headerWith(7, 4)}],

        // Raster tiles in a style that declares vector layers draw nothing at
        // all, silently.
        ['an archive of raster tiles', {bytes: headerWith(99, 2)}],

        // A backend without byte serving answers 200 with the WHOLE archive.
        // It looks healthy and is not: the pmtiles reader applies this same
        // test to every tile and throws, so accepting it would pass a deploy
        // whose every tile then fails, drawing water with no land.
        ['a backend that does not support byte serving',
            {ok: true, status: 200, contentLength: '25000953'}],
        ['a 200 with no Content-Length at all', {ok: true, status: 200, contentLength: null}],

        // Both of these read as 0 through Number(), which is finite and under
        // the header size, so a bare Number() comparison ACCEPTED them as a
        // valid range response and every tile then threw. NaN is the only
        // honest reading of a length that is not a number.
        ['a 200 whose Content-Length is not a number',
            {ok: true, status: 200, contentLength: 'gzip'}],
        ['a 200 whose Content-Length is blank', {ok: true, status: 200, contentLength: '  '}],

        // Shallower than the camera allows, so the reader would zoom into blank.
        ['an archive shallower than the style asks for', {bytes: headerWith(101, DATA_MAX_ZOOM - 1)}],
    ];

    for (const [name, reply] of cases) {
        test(name, async () => {
            const calls = stubFetch(() => reply);

            expect(await loadBasemap()).toBeNull();
            expect(await loadBasemap()).toBeNull();
            expect(calls).toHaveLength(1);
        });
    }
});

/*
 * A transient failure is NOT remembered. Latching on one stalled fetch would
 * tell a reader on a DDIL link that the map is broken for the rest of the
 * session, with a reload the only way back, in the environment this plugin is
 * built for.
 */
test.describe('a transient failure is not remembered', () => {
    test('a network throw is retried by the next caller', async () => {
        const calls = stubFetch((n) => (n === 0 ? {throws: true} : {}));

        expect(await loadBasemap()).toBeNull();
        expect(await loadBasemap()).not.toBeNull();
        expect(calls).toHaveLength(2);
    });
});

/*
 * The OpenStreetMap detail tier, which is optional and whose absence is a
 * configuration rather than a fault.
 */
const PILOT = 'indopacom-hawaii';

const REAL_DETAIL = fs.readFileSync(
    path.resolve(__dirname, `../../../../../public/map/packages/${PILOT}.pmtiles`),
);

function realDetailHeader(): ArrayBuffer {
    const slice = REAL_DETAIL.subarray(0, HEADER_BYTES);

    return slice.buffer.slice(slice.byteOffset, slice.byteOffset + slice.byteLength) as ArrayBuffer;
}

function detailHeaderWith(offset: number, value: number): ArrayBuffer {
    const bytes = new Uint8Array(realDetailHeader());
    bytes[offset] = value;

    return bytes.buffer as ArrayBuffer;
}

// eslint-disable-next-line no-console
const realWarn = console.warn;

function captureWarnings(): string[] {
    const lines: string[] = [];

    // eslint-disable-next-line no-console
    console.warn = (...args: unknown[]) => {
        lines.push(args.map(String).join(' '));
    };

    return lines;
}

test.afterEach(() => {
    // eslint-disable-next-line no-console
    console.warn = realWarn;
});

test.describe('the detail tier', () => {
    test('points at the route that serves both bundled and dropped-in areas', () => {
        expect(packageUrl(PILOT)).toBe(
            `/plugins/${manifest.id}/packages/${PILOT}.pmtiles?v=${encodeURIComponent(manifest.version)}`,
        );
    });

    /*
     * The name is whitelisted on the server and again here. A name that reaches
     * a URL from a filename on somebody's disk is the one value in this module
     * that did not come from this plugin, and the two checks agreeing by
     * accident would be a request for something else entirely.
     */
    test('refuses a name that is not <command>-<area>', async () => {
        stubFetch(() => ({bytes: realDetailHeader()}));

        expect(await loadPackages(['../world', 'INDOPACOM-HAWAII', 'hawaii', ''])).toEqual([]);
    });

    // One bad archive must not take the others off the map: an install with six
    // areas and one half-copied file draws the other five.
    test('a package that does not answer is dropped, not fatal to the set', async () => {
        stubFetch((n) => (n === 0 ? {ok: false, status: 404} : {bytes: realDetailHeader()}));

        const loaded = await loadPackages(['indopacom-broken', PILOT]);

        expect(loaded.map((entry) => entry.name)).toEqual([PILOT]);
    });

    test('accepts the archive this build ships', async () => {
        stubFetch(() => ({bytes: realDetailHeader()}));

        const [loaded] = await loadPackages([PILOT]);

        expect(loaded).toBeTruthy();
        expect(loaded.name).toBe(PILOT);
        expect(loaded.archive.minZoom).toBe(SEAM_ZOOM);
        expect(loaded.archive.maxZoom).toBeGreaterThanOrEqual(SEAM_ZOOM);
    });

    /*
     * The two probes are one machine with two accept rules, so the rules are
     * what has to be shown not to be interchangeable: the global archive must
     * start at 0 and the detail archive at the seam, and each rejects the
     * other's shape. Without this the shared machine could be reading one rule
     * for both and nothing would say so.
     */
    test('refuses the global archive, and the global probe refuses this one', async () => {
        stubFetch(() => ({bytes: realHeader()}));
        expect(await loadPackages([PILOT])).toEqual([]);

        _resetForTesting();

        stubFetch(() => ({bytes: realDetailHeader()}));
        expect(await loadBasemap()).toBeNull();
    });

    // A FLOOR, not an equality. An operator may ship a shallower profile than
    // the deepest one this pipeline can build, and rejecting it would leave
    // them with no detail and, by the silence rule below, nothing saying so.
    test('accepts an archive shallower than the one this build ships', async () => {
        stubFetch(() => ({bytes: detailHeaderWith(101, SEAM_ZOOM + 2)}));

        const [loaded] = await loadPackages([PILOT]);

        expect(loaded).toBeTruthy();
        expect(loaded.archive.maxZoom).toBe(SEAM_ZOOM + 2);
    });

    test('refuses an archive that starts below the seam, which would draw twice', async () => {
        stubFetch(() => ({bytes: detailHeaderWith(100, SEAM_ZOOM - 1)}));

        expect(await loadPackages([PILOT])).toEqual([]);
    });

    /*
     * A 404 is a global-only build, which is supported, so it says nothing to
     * anybody and is remembered: a missing file will not appear, and one probe
     * per panel for the life of the tab is a request loop for no answer.
     */
    test('a missing archive is silent, and is asked for once', async () => {
        const warnings = captureWarnings();
        const calls = stubFetch(() => ({ok: false, status: 404}));

        expect(await loadPackages([PILOT])).toEqual([]);
        expect(await loadPackages([PILOT])).toEqual([]);

        expect(calls).toHaveLength(1);
        expect(warnings).toEqual([]);
    });

    /*
     * A transient failure is the opposite, and is the one case where the two
     * look identical on screen: the style is built once inside the map's
     * creation effect and the map is created once and then moved, so a
     * timed-out probe yields a panel with no detail source for the whole of its
     * life, rendering exactly like a correct global-only install. It is
     * therefore logged, and it is not remembered, so the next map retries.
     */
    test('an unreachable archive says so, and is retried', async () => {
        const warnings = captureWarnings();
        const calls = stubFetch((n) => (n === 0 ? {throws: true} : {bytes: realDetailHeader()}));

        expect(await loadPackages([PILOT])).toEqual([]);
        expect(warnings).toHaveLength(1);
        expect(warnings[0]).toContain('detail');

        expect(await loadPackages([PILOT])).toHaveLength(1);
        expect(calls).toHaveLength(2);
    });

    // The global archive keeps its own silence on both branches, which is what
    // makes the logging above a property of the detail tier rather than of the
    // shared machine.
    test('the global probe stays silent on the same failures', async () => {
        const warnings = captureWarnings();
        stubFetch(() => ({throws: true}));

        expect(await loadBasemap()).toBeNull();
        expect(warnings).toEqual([]);
    });
});
