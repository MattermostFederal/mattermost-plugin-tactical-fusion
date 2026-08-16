import fs from 'fs';
import path from 'path';

import {expect, test} from '@playwright/test';
import manifest from 'manifest';

import {basemapUrl, loadBasemap, _resetForTesting} from './basemap';
import {MAX_ZOOM} from './span';

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

        return Promise.resolve({
            ok: answer.ok ?? true,
            status: answer.status ?? 206,
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
        expect(archive?.maxZoom).toBeGreaterThanOrEqual(MAX_ZOOM);
        expect(archive?.url).toBe(basemapUrl());
    });

    // Only the header is wanted, and the archive is a great deal larger than it.
    test('asks for the header alone', async () => {
        let range = '';
        globalThis.fetch = ((_url: string, init: {headers?: Record<string, string>}) => {
            range = init.headers?.Range ?? '';
            return Promise.resolve({ok: true, status: 206, arrayBuffer: () => Promise.resolve(realHeader())});
        }) as unknown as typeof globalThis.fetch;

        await loadBasemap();

        expect(range).toBe(`bytes=0-${HEADER_BYTES - 1}`);
    });

    test('sends no session with a static asset', async () => {
        let credentials = '';
        globalThis.fetch = ((_url: string, init: {credentials?: string}) => {
            credentials = init.credentials ?? '';
            return Promise.resolve({ok: true, status: 206, arrayBuffer: () => Promise.resolve(realHeader())});
        }) as unknown as typeof globalThis.fetch;

        await loadBasemap();

        expect(credentials).toBe('omit');
    });

    // A server that does not implement Range answers 200 with the whole archive.
    // That is still perfectly usable and must not read as a broken deploy.
    test('accepts a server that ignores the range request', async () => {
        stubFetch(() => ({ok: true, status: 200}));

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

        // Shallower than the camera allows, so the reader would zoom into blank.
        ['an archive shallower than the style asks for', {bytes: headerWith(101, MAX_ZOOM - 1)}],
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
