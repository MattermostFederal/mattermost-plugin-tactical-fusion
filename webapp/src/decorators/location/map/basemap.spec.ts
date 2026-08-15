import fs from 'fs';
import path from 'path';

import {expect, test} from '@playwright/test';
import manifest from 'manifest';

import {basemapUrl, loadBasemap, _resetForTesting} from './basemap';

/*
 * The real basemap, off disk.
 *
 * These bytes are what BASEMAP_SHA256 was generated against, so the digest check
 * runs for real here rather than being stubbed away. A hand-written fixture
 * could only ever test the mismatch branch.
 */
const REAL_BASEMAP = fs.readFileSync(
    path.resolve(__dirname, '../../../../../public/map/world.geo.json'),
);

function bytesOf(value: string): ArrayBuffer {
    return new TextEncoder().encode(value).buffer as ArrayBuffer;
}

function realBytes(): ArrayBuffer {
    return REAL_BASEMAP.buffer.slice(
        REAL_BASEMAP.byteOffset,
        REAL_BASEMAP.byteOffset + REAL_BASEMAP.byteLength,
    ) as ArrayBuffer;
}

/** Valid GeoJSON, padded to a given total size. */
function paddedBasemap(bytes: number): string {
    const skeleton = JSON.stringify({type: 'FeatureCollection', features: [], pad: ''});

    return JSON.stringify({
        type: 'FeatureCollection',
        features: [],
        pad: 'a'.repeat(Math.max(0, bytes - skeleton.length)),
    });
}

function oversizedBasemap(): ArrayBuffer {
    return bytesOf(paddedBasemap((2 * 1024 * 1024) + 64));
}

interface Reply {
    ok?: boolean;
    bytes?: ArrayBuffer;
    throws?: boolean;
}

const realFetch = globalThis.fetch;
const realCrypto = Object.getOwnPropertyDescriptor(globalThis, 'crypto');

/**
 * Replaces global fetch and records every call.
 *
 * Only `ok` and `arrayBuffer` are implemented, so a module that starts reading
 * anything else fails loudly here rather than quietly seeing undefined.
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
            arrayBuffer: () => Promise.resolve(answer.bytes ?? realBytes()),
        });
    }) as unknown as typeof globalThis.fetch;

    return calls;
}

/** Removes `crypto`, which is what a plain-HTTP origin looks like. */
function withoutSubtleCrypto(): void {
    Object.defineProperty(globalThis, 'crypto', {value: undefined, configurable: true});
}

type Global = {window?: {basename?: string}};

test.beforeEach(() => {
    _resetForTesting();
});

test.afterEach(() => {
    globalThis.fetch = realFetch;
    delete (globalThis as Global).window;

    if (realCrypto) {
        Object.defineProperty(globalThis, 'crypto', realCrypto);
    }
});

test.describe('basemapUrl', () => {
    test('points at the bundled basemap', () => {
        expect(basemapUrl()).toBe(
            `/plugins/${manifest.id}/public/map/world.geo.json?v=${encodeURIComponent(manifest.version)}`,
        );
    });

    // The version is what busts a reader's cache when the basemap is
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

test.describe('loading', () => {
    test('returns the basemap the build was generated against', async () => {
        const calls = stubFetch(() => ({}));

        const map = await loadBasemap();

        expect(calls).toHaveLength(1);
        expect(map?.type).toBe('FeatureCollection');
        expect(map?.features.length).toBeGreaterThan(0);
    });

    test('sends no session with a static asset', async () => {
        let credentials = '';
        globalThis.fetch = ((_url: string, init: {credentials?: string}) => {
            credentials = init.credentials ?? '';
            return Promise.resolve({ok: true, arrayBuffer: () => Promise.resolve(realBytes())});
        }) as unknown as typeof globalThis.fetch;

        await loadBasemap();

        expect(credentials).toBe('omit');
    });

    // Module state with no TTL: the file is immutable for the life of an
    // install, so a channel full of coordinates makes one request rather than
    // one per panel.
    test('is fetched once and served from memory thereafter', async () => {
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
    test('a missing basemap', async () => {
        const calls = stubFetch(() => ({ok: false}));

        expect(await loadBasemap()).toBeNull();
        expect(await loadBasemap()).toBeNull();
        expect(calls).toHaveLength(1);
    });

    /*
     * The cap has to be the only thing standing between this body and success,
     * or the test passes on the digest instead and deleting the cap changes
     * nothing. So the payload is valid GeoJSON and the digest is unverifiable:
     * without the cap, this loads.
     */
    test('a body past the size cap', async () => {
        withoutSubtleCrypto();
        const calls = stubFetch(() => ({bytes: oversizedBasemap()}));

        expect(await loadBasemap()).toBeNull();
        expect(await loadBasemap()).toBeNull();
        expect(calls).toHaveLength(1);
    });

    // The counterpart, so the test above pins the cap rather than the shape:
    // the same payload just under the cap is accepted.
    test('the same body just under the cap is accepted', async () => {
        withoutSubtleCrypto();
        stubFetch(() => ({bytes: bytesOf(paddedBasemap(1024))}));

        expect(await loadBasemap()).not.toBeNull();
    });

    // As much an availability control as a security one: an air-gapped install
    // has no way to re-fetch, so a truncated or half-deployed asset would
    // otherwise render a silently wrong-shaped world.
    test('a digest that is not the one this build was generated against', async () => {
        const calls = stubFetch(() => ({
            bytes: bytesOf(JSON.stringify({type: 'FeatureCollection', features: []})),
        }));

        expect(await loadBasemap()).toBeNull();
        expect(await loadBasemap()).toBeNull();
        expect(calls).toHaveLength(1);
    });

    test('a body that is not GeoJSON', async () => {
        withoutSubtleCrypto();
        const calls = stubFetch(() => ({bytes: bytesOf(JSON.stringify({type: 'Feature'}))}));

        expect(await loadBasemap()).toBeNull();
        expect(await loadBasemap()).toBeNull();
        expect(calls).toHaveLength(1);
    });

    test('a FeatureCollection with no features array', async () => {
        withoutSubtleCrypto();
        const calls = stubFetch(() => ({
            bytes: bytesOf(JSON.stringify({type: 'FeatureCollection', features: 'no'})),
        }));

        expect(await loadBasemap()).toBeNull();
        expect(calls).toHaveLength(1);
    });

    test('a body that is not an object at all', async () => {
        withoutSubtleCrypto();
        const calls = stubFetch(() => ({bytes: bytesOf('null')}));

        expect(await loadBasemap()).toBeNull();
        expect(calls).toHaveLength(1);
    });
});

/*
 * `crypto.subtle` is undefined on a plain-HTTP origin, which for on-prem and
 * air-gapped installs is the norm. An unverifiable digest passes rather than
 * disabling the map, which is the same posture the copy buttons take.
 */
test.describe('an unverifiable digest', () => {
    test('passes when there is no crypto to verify with', async () => {
        withoutSubtleCrypto();
        stubFetch(() => ({}));

        expect(await loadBasemap()).not.toBeNull();
    });

    test('passes when the digest cannot be computed', async () => {
        Object.defineProperty(globalThis, 'crypto', {
            value: {subtle: {digest: () => Promise.reject(new Error('unavailable'))}},
            configurable: true,
        });
        stubFetch(() => ({}));

        expect(await loadBasemap()).not.toBeNull();
    });
});

/*
 * Latching on a transient failure means one stalled fetch on a DDIL link tells a
 * reader the map is broken for the rest of the session, with a reload the only
 * way back, in exactly the environment this plugin is built for.
 */
test.describe('a transient failure is not remembered', () => {
    test('a network throw is retried by the next caller', async () => {
        const calls = stubFetch((n) => (n === 0 ? {throws: true} : {}));

        expect(await loadBasemap()).toBeNull();
        expect(await loadBasemap()).not.toBeNull();
        expect(calls).toHaveLength(2);
    });

    test('the retry is cached like any other success', async () => {
        const calls = stubFetch((n) => (n === 0 ? {throws: true} : {}));

        await loadBasemap();
        await loadBasemap();
        await loadBasemap();

        expect(calls).toHaveLength(2);
    });
});
