import {expect, test} from '@playwright/test';

import {
    getState,
    loadPreferences,
    resetPreferences,
    savePreferences,
    subscribe,
    _resetForTesting,
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

const savedBlob = {dtg: {zones: [{iana: 'UTC'}, {iana: 'Asia/Tokyo', name: 'Yokota'}], urgent_within_minutes: 15}};

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

test('saving sends the wire shape and adopts what came back', async () => {
    const calls = stubFetch(() => ({status: 200, body: savedBlob}));

    await savePreferences({dtg: {zones: [{iana: 'UTC'}, {iana: 'Asia/Tokyo', name: 'Yokota'}], urgentWithinMinutes: 15}});

    expect(calls).toHaveLength(1);
    expect(calls[0].method).toBe('PUT');
    expect(calls[0].body).toEqual(savedBlob);
    expect(getState().preferences.dtg.zones).toEqual([{iana: 'UTC'}, {iana: 'Asia/Tokyo', name: 'Yokota'}]);
});

// A save that quietly did nothing would leave the reader believing their
// settings had been kept, so unlike the load this one has to reject.
test('a failed save rejects with the reason the server gave', async () => {
    stubFetch(() => ({status: 400, body: {message: 'unknown timezone "Mars/Olympus_Mons"'}}));

    await expect(savePreferences({dtg: {zones: [{iana: 'Mars/Olympus_Mons'}], urgentWithinMinutes: 0}})).
        rejects.toThrow('unknown timezone "Mars/Olympus_Mons"');
});

test('a failed save without a reason still rejects', async () => {
    stubFetch(() => ({status: 500, body: null}));

    await expect(savePreferences({dtg: {zones: [], urgentWithinMinutes: 0}})).
        rejects.toThrow('500');
});

test('saving means a later load does not go back to the server', async () => {
    const calls = stubFetch(() => ({status: 200, body: savedBlob}));

    await savePreferences({dtg: {zones: [{iana: 'UTC'}], urgentWithinMinutes: 0}});
    await loadPreferences();

    expect(calls).toHaveLength(1);
});

// "Restore defaults" deletes the blob rather than writing today's defaults into
// it, so the reader goes back to tracking whatever the defaults become.
test('restoring defaults deletes', async () => {
    const calls = stubFetch((call) => ({
        status: 200,
        body: call.method === 'DELETE' ? {dtg: {zones: [], urgent_within_minutes: 0}} : savedBlob,
    }));

    await loadPreferences();
    await resetPreferences();

    expect(calls[1].method).toBe('DELETE');
    expect(calls[1].body).toBeUndefined();
    expect(getState().preferences.dtg.zones).toEqual([]);
    expect(getState().preferences.dtg.urgentWithinMinutes).toBe(0);
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
    await savePreferences({dtg: {zones: [], urgentWithinMinutes: 0}});

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
