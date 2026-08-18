import {expect, test} from '@playwright/test';

import {
    CACHE_TTL_MS,
    getState,
    loadPreferences,
    resetPreferencesSection,
    savePreferencesSection,
    subscribe,
    _resetForTesting,
    _setClockForTesting,
} from './store';

interface Call {
    url: string;
    method: string;
    body: unknown;
}

interface Reply {
    status: number;
    body: unknown;
}

const realFetch = globalThis.fetch;

/**
 * Replaces global fetch with a recorder.
 *
 * Only the three things the store touches are implemented (ok, status, json),
 * so anything else it starts relying on fails loudly here rather than quietly
 * reading undefined.
 */
function stubFetch(reply: (call: Call) => Reply): Call[] {
    const calls: Call[] = [];

    globalThis.fetch = ((url: string, init: {method: string; body?: string}) => {
        const call = {
            url,
            method: init.method,
            body: init.body === undefined ? undefined : JSON.parse(init.body),
        };
        calls.push(call);

        const {status, body} = reply(call);
        return Promise.resolve({
            ok: status >= 200 && status < 300,
            status,
            json: () => Promise.resolve(body),
        });
    }) as unknown as typeof globalThis.fetch;

    return calls;
}

const savedBlob = {
    dtg: {zones: [{iana: 'UTC'}, {iana: 'Asia/Tokyo', name: 'Yokota'}], urgent_within_minutes: 15},
    location: {hidden_rows: []},
};

test.beforeEach(() => {
    _resetForTesting();
});

test.afterEach(() => {
    globalThis.fetch = realFetch;
});

test('loads once and serves the rest from memory', async () => {
    const calls = stubFetch(() => ({status: 200, body: savedBlob}));

    await loadPreferences();
    await loadPreferences();
    await loadPreferences();

    expect(calls).toHaveLength(1);
    expect(calls[0].method).toBe('GET');
    expect(getState().preferences.dtg.zones).toEqual([{iana: 'UTC'}, {iana: 'Asia/Tokyo', name: 'Yokota'}]);
    expect(getState().preferences.dtg.urgentWithinMinutes).toBe(15);
});

// The panel and every hover card ask on mount, so without this a channel full
// of DTGs would open a request per link.
test('callers that arrive together share one request', async () => {
    const calls = stubFetch(() => ({status: 200, body: savedBlob}));

    await Promise.all([loadPreferences(), loadPreferences(), loadPreferences()]);

    expect(calls).toHaveLength(1);
});

test('asks the plugin route', async () => {
    const calls = stubFetch(() => ({status: 200, body: savedBlob}));

    await loadPreferences();

    expect(calls[0].url).toContain('/api/v1/preferences');
    expect(calls[0].url.startsWith('/plugins/')).toBe(true);
});

// View settings are not worth taking the panel down for.
test('a failed load leaves the defaults in place', async () => {
    stubFetch(() => ({status: 500, body: {message: 'Could not read your settings.'}}));

    await loadPreferences();

    expect(getState().preferences.dtg.zones).toEqual([]);
    expect(getState().preferences.dtg.urgentWithinMinutes).toBe(0);
    expect(getState().error).toBe('Could not read your settings.');
    expect(getState().loading).toBe(false);
});

test('a failed load is retried by the next caller', async () => {
    let calls = stubFetch(() => ({status: 500, body: null}));
    await loadPreferences();
    expect(calls).toHaveLength(1);

    calls = stubFetch(() => ({status: 200, body: savedBlob}));
    await loadPreferences();

    expect(calls).toHaveLength(1);
    expect(getState().preferences.dtg.zones).toEqual([{iana: 'UTC'}, {iana: 'Asia/Tokyo', name: 'Yokota'}]);
    expect(getState().error).toBeNull();
});

test('saving re-reads first, then sends the wire shape and adopts what came back', async () => {
    const calls = stubFetch(() => ({status: 200, body: savedBlob}));

    await savePreferencesSection('dtg', {zones: [{iana: 'UTC'}, {iana: 'Asia/Tokyo', name: 'Yokota'}], urgentWithinMinutes: 15});

    expect(calls.map((call) => call.method)).toEqual(['GET', 'PUT']);
    expect(calls[1].body).toEqual(savedBlob);
    expect(getState().preferences.dtg.zones).toEqual([{iana: 'UTC'}, {iana: 'Asia/Tokyo', name: 'Yokota'}]);
});

// The whole reason the save re-reads.
//
// loadPreferences fetches once per page load and never again, so the cached
// blob is as stale as the tab is old. A save built from it carried a snapshot
// from minutes ago back over the top of whatever had been written since — most
// visibly from the other decorator's editor, in another tab.
test('saving one section keeps what the server holds for the other', async () => {
    const calls = stubFetch((call) => ({
        status: 200,
        body: call.method === 'GET' ? {dtg: {zones: [{iana: 'Asia/Tokyo'}], urgent_within_minutes: 42},
                location: {hidden_rows: ['ddm']}} : savedBlob,
    }));

    // A stale cache: loaded before anything else changed it.
    await loadPreferences();
    expect(getState().preferences.location.hiddenRows).toEqual(['ddm']);

    await savePreferencesSection('dtg', {zones: [{iana: 'UTC'}], urgentWithinMinutes: 5});

    const put = calls[calls.length - 1];
    expect(put.method).toBe('PUT');
    expect(put.body).toEqual({
        dtg: {zones: [{iana: 'UTC'}], urgent_within_minutes: 5},
        location: {hidden_rows: ['ddm']},
    });
});

// A save that quietly did nothing would leave the reader believing their
// settings had been kept, so unlike the load this one has to reject.
test('a failed save rejects with the reason the server gave', async () => {
    stubFetch(() => ({status: 400, body: {message: 'unknown timezone "Mars/Olympus_Mons"'}}));

    await expect(savePreferencesSection('dtg', {zones: [{iana: 'Mars/Olympus_Mons'}], urgentWithinMinutes: 0})).
        rejects.toThrow('unknown timezone "Mars/Olympus_Mons"');
});

test('a failed save without a reason still rejects', async () => {
    stubFetch(() => ({status: 500, body: null}));

    await expect(savePreferencesSection('dtg', {zones: [], urgentWithinMinutes: 0})).
        rejects.toThrow('500');
});

test('saving means a later load does not go back to the server', async () => {
    const calls = stubFetch(() => ({status: 200, body: savedBlob}));

    await savePreferencesSection('dtg', {zones: [{iana: 'UTC'}], urgentWithinMinutes: 0});
    await loadPreferences();

    // The save's own read, then its write. Nothing after it.
    expect(calls.map((call) => call.method)).toEqual(['GET', 'PUT']);
});

// "Restore defaults" deletes the blob rather than writing today's defaults into
// it, so the reader goes back to tracking whatever the defaults become.
test('restoring defaults deletes when nothing is left', async () => {
    const calls = stubFetch((call) => ({
        status: 200,
        body: call.method === 'DELETE' ? {} : savedBlob,
    }));

    await loadPreferences();
    await resetPreferencesSection('dtg');

    expect(calls.map((call) => call.method)).toEqual(['GET', 'GET', 'DELETE']);
    expect(calls[2].body).toBeUndefined();
    expect(getState().preferences.dtg.zones).toEqual([]);
    expect(getState().preferences.dtg.urgentWithinMinutes).toBe(0);
});

// The bug this scoping exists for: "Restore defaults" under a legend reading
// "Rows to show" used to DELETE the whole blob and take the reader's timezone
// table with it. Now it writes the blob back with only its own section zeroed.
test('restoring one section leaves the other alone', async () => {
    const stored = {
        dtg: {zones: [{iana: 'Asia/Tokyo'}], urgent_within_minutes: 42},
        location: {hidden_rows: ['ddm']},
    };
    const calls = stubFetch(() => ({status: 200, body: stored}));

    await resetPreferencesSection('location');

    const write = calls[calls.length - 1];
    expect(write.method).toBe('PUT');
    expect(write.body).toEqual({
        dtg: {zones: [{iana: 'Asia/Tokyo'}], urgent_within_minutes: 42},
        location: {hidden_rows: []},
    });
});

test('restoring the last section left deletes rather than writing an empty blob', async () => {
    const calls = stubFetch((call) => ({
        status: 200,
        body: call.method === 'DELETE' ? {} : {location: {hidden_rows: ['ddm']}},
    }));

    await resetPreferencesSection('location');

    expect(calls.map((call) => call.method)).toEqual(['GET', 'DELETE']);
});

test('subscribers hear about a change, and stop when they unsubscribe', async () => {
    stubFetch(() => ({status: 200, body: savedBlob}));

    let notifications = 0;
    const unsubscribe = subscribe(() => {
        notifications++;
    });

    await loadPreferences();
    expect(notifications).toBeGreaterThan(0);

    const seen = notifications;
    unsubscribe();
    await savePreferencesSection('dtg', {zones: [], urgentWithinMinutes: 0});

    expect(notifications).toBe(seen);
});

// useSyncExternalStore compares snapshots by identity, so an unchanged read
// that handed back a fresh object would re-render forever.
test('the snapshot keeps its identity until something changes', async () => {
    stubFetch(() => ({status: 200, body: savedBlob}));

    await loadPreferences();
    const first = getState();

    await loadPreferences();

    expect(getState()).toBe(first);
});

/*
 * A save is authoritative and a load is not.
 *
 * A GET still in the air when a PUT lands used to overwrite the saved blob with
 * whatever the server held before the save. The reader watched their new table
 * revert, with nothing reported wrong and the server holding the right data.
 */
test('a load still in flight does not overwrite a save that landed', async () => {
    let releaseGet = (): void => {};
    const getReached = new Promise<void>((resolve) => {
        releaseGet = resolve;
    });

    // Only the FIRST read is held open: that is the load being raced. The save
    // does a read of its own, and holding that one too would simply deadlock
    // the test rather than exercise anything.
    let reads = 0;

    globalThis.fetch = ((url: string, init: {method: string; body?: string}) => {
        if (init.method === 'GET') {
            reads++;
            if (reads === 1) {
                return getReached.then(() => ({
                    ok: true,
                    status: 200,
                    json: () => Promise.resolve(savedBlob),
                }));
            }
            return Promise.resolve({
                ok: true,
                status: 200,
                json: () => Promise.resolve(savedBlob),
            });
        }
        return Promise.resolve({
            ok: true,
            status: 200,
            json: () => Promise.resolve({dtg: {zones: [{iana: 'Europe/Berlin'}], urgent_within_minutes: 99}}),
        });
    }) as unknown as typeof globalThis.fetch;

    // The load starts first and is still waiting.
    const loading = loadPreferences();

    await savePreferencesSection('dtg', {zones: [{iana: 'Europe/Berlin'}], urgentWithinMinutes: 99});
    expect(getState().preferences.dtg.urgentWithinMinutes).toBe(99);

    // Now let the older read land.
    releaseGet();
    await loading;

    expect(getState().preferences.dtg.urgentWithinMinutes).toBe(99);
    expect(getState().preferences.dtg.zones).toEqual([{iana: 'Europe/Berlin'}]);
});

// The same race on the failing path, which was the worse of the two: it
// replaced the save with the defaults and left `loaded` set, so nothing ever
// fetched again and the reader saw the built-in table for the rest of the
// session.
test('a load that fails after a save does not revert to the defaults', async () => {
    let failGet = (): void => {};
    const getReached = new Promise<void>((resolve) => {
        failGet = resolve;
    });

    // As above, only the first read is the one being raced.
    let reads = 0;

    globalThis.fetch = ((url: string, init: {method: string; body?: string}) => {
        if (init.method === 'GET') {
            reads++;
            if (reads === 1) {
                return getReached.then(() => ({
                    ok: false,
                    status: 503,
                    json: () => Promise.resolve({message: 'Not ready.'}),
                }));
            }
            return Promise.resolve({
                ok: true,
                status: 200,
                json: () => Promise.resolve({}),
            });
        }
        return Promise.resolve({
            ok: true,
            status: 200,
            json: () => Promise.resolve({dtg: {zones: [{iana: 'Europe/Berlin'}], urgent_within_minutes: 99}}),
        });
    }) as unknown as typeof globalThis.fetch;

    const loading = loadPreferences();

    await savePreferencesSection('dtg', {zones: [{iana: 'Europe/Berlin'}], urgentWithinMinutes: 99});

    failGet();
    await loading;

    expect(getState().preferences.dtg.urgentWithinMinutes).toBe(99);
    expect(getState().error).toBeNull();
});

/*
 * The cache has a lifetime, which it did not.
 *
 * A blob read on the first hover used to be kept for the life of the tab, so a
 * reader who changed their settings elsewhere saw the old ones here until they
 * reloaded the page, and a save built on that copy carried it back over the
 * newer one.
 */
test.describe('the cache expires', () => {
    test('serves from memory inside the lifetime', async () => {
        let clock = 1_000_000;
        _setClockForTesting(() => clock);
        const calls = stubFetch(() => ({status: 200, body: savedBlob}));

        await loadPreferences();
        clock += CACHE_TTL_MS - 1;
        await loadPreferences();

        expect(calls).toHaveLength(1);
    });

    test('reads again once the lifetime has passed', async () => {
        let clock = 1_000_000;
        _setClockForTesting(() => clock);
        const calls = stubFetch(() => ({status: 200, body: savedBlob}));

        await loadPreferences();
        clock += CACHE_TTL_MS;
        await loadPreferences();

        expect(calls.map((call) => call.method)).toEqual(['GET', 'GET']);
    });

    test('is thirty minutes', () => {
        expect(CACHE_TTL_MS).toBe(30 * 60 * 1000);
    });

    // A save is a read of the server's own answer, so it restarts the clock
    // rather than leaving a blob that expires on the old one's schedule.
    test('a save restarts the lifetime', async () => {
        let clock = 1_000_000;
        _setClockForTesting(() => clock);
        const calls = stubFetch(() => ({status: 200, body: savedBlob}));

        await loadPreferences();
        clock += CACHE_TTL_MS - 1;

        await savePreferencesSection('dtg', {zones: [{iana: 'UTC'}], urgentWithinMinutes: 0});
        const afterSave = calls.length;

        clock += 1;
        await loadPreferences();

        expect(calls).toHaveLength(afterSave);
    });

    // A REFRESH that fails keeps what was already read.
    //
    // The failure handler used to write the defaults whatever had gone before,
    // which was right for a first load and wrong for every one after it. Once
    // the cache grew a lifetime, the first mount past it starts a refresh, so a
    // single blip reverted the reader's hidden rows and timezone table on
    // screen with nothing saying why. And because a failed read deliberately
    // does not stamp the clock, every later mount retried, so the rows flipped
    // back and forth for as long as the server was unwell.
    test('a failed refresh keeps the settings already loaded', async () => {
        let clock = 1_000_000;
        _setClockForTesting(() => clock);

        let fail = false;
        stubFetch(() => (fail ? {status: 503, body: {message: 'Not ready.'}} : {status: 200, body: savedBlob}));

        await loadPreferences();
        expect(getState().preferences.dtg.urgentWithinMinutes).toBe(15);

        fail = true;
        clock += CACHE_TTL_MS;
        await loadPreferences();

        // The reason is reported, and the settings are still theirs.
        expect(getState().error).not.toBeNull();
        expect(getState().preferences.dtg.urgentWithinMinutes).toBe(15);
        expect(getState().preferences.dtg.zones).toHaveLength(2);
    });

    // But a first load that fails has nothing to keep, so it still degrades to
    // the defaults rather than leaving the panel with no shape to render.
    test('a first read that fails still falls back to the defaults', async () => {
        _setClockForTesting(() => 1_000_000);
        stubFetch(() => ({status: 503, body: {message: 'Not ready.'}}));

        await loadPreferences();

        expect(getState().error).not.toBeNull();
        expect(getState().preferences.dtg.zones).toHaveLength(0);
        expect(getState().preferences.dtg.urgentWithinMinutes).toBe(0);
    });

    // A read that failed must not stamp the clock, or a reader whose settings
    // were briefly unreachable would be stuck on the defaults for half an hour.
    test('a failed read does not start the lifetime', async () => {
        const clock = 1_000_000;
        _setClockForTesting(() => clock);
        const calls = stubFetch(() => ({status: 503, body: {message: 'Not ready.'}}));

        await loadPreferences();
        await loadPreferences();

        expect(calls.map((call) => call.method)).toEqual(['GET', 'GET']);
    });
});
