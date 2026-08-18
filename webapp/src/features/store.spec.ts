import {expect, test} from '@playwright/test';

import {
    getState,
    loadFeatures,
    _resetForTesting,
    _setClockForTesting,
    _setFetchTimeoutForTesting,
} from './store';
import {fromWire} from './types';

import {CACHE_TTL_MS} from '../preferences/store';

/*
 * The store in front of /api/v1/features.
 *
 * What matters here is the pair of opposite defaults. "Not answered yet" and
 * "could not be answered" are different states with opposite right answers, and
 * collapsing them either way is a real failure: holding maps back forever hides
 * a feature nobody switched off, and assuming maps on before the answer lands
 * pulls the basemap archive on exactly the installs the switch exists for.
 */

const ON = {map_panel: true, map_inline: true, map_page: true};

function stubFetch(reply: (init?: RequestInit) => Promise<Response>): () => number {
    let calls = 0;
    globalThis.fetch = ((_input: RequestInfo | URL, init?: RequestInit) => {
        calls += 1;
        return reply(init);
    }) as typeof fetch;

    return () => calls;
}

function ok(body: unknown): Promise<Response> {
    return Promise.resolve({
        ok: true,
        status: 200,
        json: () => Promise.resolve(body),
    } as Response);
}

test.beforeEach(() => {
    _resetForTesting();
});

test('holds every surface back until the server answers', () => {
    expect(getState().features).toEqual({mapPanel: false, mapInline: false, mapPage: false});
});

test('reports what the server said', async () => {
    stubFetch(() => ok({map_panel: true, map_inline: false, map_page: true}));

    await loadFeatures();

    expect(getState().features).toEqual({mapPanel: true, mapInline: false, mapPage: true});
    expect(getState().error).toBeNull();
});

/*
 * A failure degrades to every surface ON, the opposite of the loading default.
 * Nothing may fail the panel into hiding a feature the admin is paying for, and
 * a reader whose server is unreachable has a broken map either way, which the
 * map says for itself.
 */
test('a failed read degrades to every surface on', async () => {
    stubFetch(() => Promise.reject(new Error('offline')));

    await loadFeatures();

    expect(getState().features).toEqual({mapPanel: true, mapInline: true, mapPage: true});
    expect(getState().error).toBe('offline');
});

test('a rejected status degrades the same way', async () => {
    stubFetch(() => Promise.resolve({
        ok: false,
        status: 500,
        json: () => Promise.resolve({message: 'Broken.'}),
    } as Response));

    await loadFeatures();

    expect(getState().features.mapPanel).toBe(true);
    expect(getState().error).toBe('Broken.');
});

/*
 * A shape disagreement is a failure rather than a set of falses. `Boolean` of a
 * missing field is a confident `false`, so a coercing reader would turn a
 * renamed wire key into "the admin turned maps off" and give nobody anywhere a
 * reason. TestWebappFeatureShapeMatches is what stops it happening; this is what
 * makes it legible when it does.
 */
test('a payload missing a field is a failure, not every surface off', async () => {
    stubFetch(() => ok({map_panel: true, map_page: true}));

    await loadFeatures();

    expect(getState().features.mapPanel).toBe(true);
    expect(getState().error).toContain('map_inline');
});

test('refuses a non-boolean field', () => {
    expect(() => fromWire({map_panel: 'yes', map_inline: true, map_page: true})).toThrow();
});

// One request for the whole tab, however many panels, hovers and posts ask.
// Mattermost renders on the order of thirty posts at a time, so without this the
// first paint of a channel full of coordinates would issue thirty of them.
test('shares one request between every caller', async () => {
    const calls = stubFetch(() => ok(ON));

    await Promise.all([loadFeatures(), loadFeatures(), loadFeatures()]);

    expect(calls()).toBe(1);
});

test('serves the cached answer until the TTL lapses', async () => {
    const calls = stubFetch(() => ok(ON));

    let clock = 1000;
    _setClockForTesting(() => clock);

    await loadFeatures();
    await loadFeatures();
    expect(calls()).toBe(1);

    clock += CACHE_TTL_MS + 1;
    await loadFeatures();
    expect(calls()).toBe(2);
});

/*
 * A failed REFRESH keeps the last good answer rather than falling back.
 *
 * This is the defect preferences/store.ts documents having fixed, reintroduced
 * here with the opposite constant and a worse consequence. Without it: an
 * install with maps off gets a good all-off answer, the cache lapses half an
 * hour later, one failed refresh flips every surface on, and every panel, hover
 * and coordinate-only post starts pulling the basemap archive. And because a
 * failure does not stamp the clock, every following mount retried, so it flapped
 * for as long as the server was unwell.
 */
test('a failed refresh keeps the last good answer', async () => {
    let failing = false;
    stubFetch(() => (failing ?
        Promise.reject(new Error('blip')) :
        ok({map_panel: false, map_inline: false, map_page: false})));

    let clock = 1000;
    _setClockForTesting(() => clock);

    await loadFeatures();
    expect(getState().features.mapPanel).toBe(false);

    clock += CACHE_TTL_MS + 1;
    failing = true;
    await loadFeatures();

    expect(getState().features).toEqual({mapPanel: false, mapInline: false, mapPage: false});
    expect(getState().error).toBe('blip');
});

// The other half: with nothing good to keep, a failure still degrades to every
// surface on, so a first read that fails cannot hide a feature nobody switched
// off. The two constants only mean different things because of this pair.
test('a first read that fails still degrades to every surface on', async () => {
    stubFetch(() => Promise.reject(new Error('offline')));

    await loadFeatures();

    expect(getState().features).toEqual({mapPanel: true, mapInline: true, mapPage: true});
});

/*
 * A request that HANGS is treated as failed rather than pinning the store.
 *
 * A stalled fetch never rejects, so without the timeout `inflight` is never
 * cleared, every later caller joins that same pending promise, and the store
 * sits on NO_FEATURES for the life of the tab: every map off, on every surface,
 * indistinguishable from the admin having switched them off. That is the one
 * direction this store must never fail in, and it is the same failure
 * loadMapLibre and basemap.ts are bounded against.
 */
test('a hung read is abandoned rather than pinning every surface off', async () => {
    // Never settles on its own. The abort signal is the only thing that can end
    // it, which is what makes this a test of the timeout rather than of a slow
    // reply, and is why the stub has to honour the signal the way fetch does.
    _setFetchTimeoutForTesting(20);

    const calls = stubFetch((init) => new Promise((_, reject) => {
        init?.signal?.addEventListener('abort', () => reject(new Error('aborted')));
    }));

    await loadFeatures();

    expect(getState().features.mapPanel).toBe(true);
    expect(getState().error).not.toBeNull();

    // And the store is not pinned: the next caller issues a fresh request rather
    // than joining the abandoned one forever.
    stubFetch(() => ok(ON));
    await loadFeatures();
    expect(getState().features.mapInline).toBe(true);
    expect(calls()).toBe(1);
});

// A failed read must not stamp the clock, or one bad minute leaves a reader on
// the fallback for half an hour with a reload the only way back.
test('retries immediately after a failure rather than caching it', async () => {
    let failing = true;
    const calls = stubFetch(() => (failing ? Promise.reject(new Error('offline')) : ok(ON)));

    await loadFeatures();
    expect(calls()).toBe(1);

    failing = false;
    await loadFeatures();

    expect(calls()).toBe(2);
    expect(getState().error).toBeNull();
});
