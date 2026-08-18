import {useEffect, useSyncExternalStore} from 'react';

import type {Preferences} from './types';
import {EMPTY_PREFERENCES, fromWire, toWire} from './types';

import {pluginBaseUrl} from '../plugin_url';

export interface PreferencesState {

    /** What the reader has saved. Empty fields mean the defaults. */
    preferences: Preferences;

    /** True while the first read is still in flight. */
    loading: boolean;

    /** Why the last read failed, or null. */
    error: string | null;
}

const INITIAL_STATE: PreferencesState = {
    preferences: EMPTY_PREFERENCES,
    loading: false,
    error: null,
};

/*
 * Module state, not React state, and deliberately so.
 *
 * The panel and every hover card ask for the same blob, and they mount and
 * unmount constantly as the reader moves around a channel. Caching here means
 * one request per page load however many of them appear, which is the whole
 * point of pairing this with the server-side cache rather than reading the KV
 * store on every hover.
 */
let state = INITIAL_STATE;
let inflight: Promise<void> | null = null;

/**
 * How long a loaded blob is served before it is read again.
 *
 * Thirty minutes rather than forever, which is what this was. A blob loaded on
 * the first hover was kept for the life of the tab, so a reader who changed
 * their settings in another tab, or on their phone, saw the old ones here until
 * they reloaded the page. Worse, a save built on that copy carried it back over
 * the top of the newer one; savePreferencesSection re-reads for exactly that
 * reason, and this bounds how wrong the cached copy can get in the meantime.
 *
 * Equal to the server's own, and that is a decision rather than a coincidence:
 * TestWebappCacheLifetimeMatches reads this constant and fails if either moves
 * alone. The two mean different things, though. The server's is a backstop for
 * a lost cluster event and is invalidated by every write, so it corrects
 * itself; this one has no invalidation to hear, so the timer is the only thing
 * that ever refreshes it. Shortening it would buy freshness a reader cannot
 * perceive at the cost of a request per panel open.
 */
export const CACHE_TTL_MS = 30 * 60 * 1000;

/** When the current blob was read, or null when there is nothing cached. */
let loadedAt: number | null = null;

/**
 * The clock, so a test can drive the TTL rather than wait out thirty minutes.
 *
 * Swappable because the alternative is a test that pokes `loadedAt` directly
 * and so pins the implementation rather than the duration.
 */
let now = (): number => Date.now();

/** Whether the cached blob is still young enough to serve. */
function isFresh(): boolean {
    return loadedAt !== null && now() - loadedAt < CACHE_TTL_MS;
}
const listeners = new Set<() => void>();

/*
 * Counts completed writes, so a load that started before one can tell that it
 * did and keep its hands off the state.
 *
 * A save is authoritative and a load is not. Without this, a GET still in
 * flight when a PUT lands overwrites the saved blob with what the server held
 * before the save, and the reader watches their new table revert with nothing
 * reported wrong. The failed-load path is worse: it replaces the save with the
 * defaults, and the save has already stamped `loadedAt`, so nothing fetches
 * again until the TTL runs out.
 */
let writes = 0;

function setState(next: PreferencesState): void {
    state = next;
    listeners.forEach((listener) => listener());
}

/** Subscribes to changes. Returns the unsubscribe function. */
export function subscribe(listener: () => void): () => void {
    listeners.add(listener);
    return () => {
        listeners.delete(listener);
    };
}

/**
 * The current snapshot.
 *
 * Identity only changes when something changed, which is what
 * `useSyncExternalStore` needs to avoid re-rendering forever.
 */
export function getState(): PreferencesState {
    return state;
}

function endpoint(): string {
    return `${pluginBaseUrl()}/api/v1/preferences`;
}

function messageOf(error: unknown): string {
    return error instanceof Error && error.message ? error.message : 'Something went wrong.';
}

async function request(method: string, body?: Preferences): Promise<Preferences> {
    const response = await fetch(endpoint(), {
        method,
        credentials: 'same-origin',
        headers: {

            // Mattermost accepts session-cookie authentication only for
            // requests that could not have been a cross-site form post, so a
            // write without this header is rejected as a CSRF attempt.
            'X-Requested-With': 'XMLHttpRequest',
            'Content-Type': 'application/json',
        },
        body: body === undefined ? undefined : JSON.stringify(toWire(body)),
    });

    // The server explains a rejected save in terms the reader can act on, such
    // as a timezone it could not resolve. Losing that for a generic message
    // would leave the panel able only to say that something went wrong.
    const payload = await response.json().catch(() => null);
    if (!response.ok) {
        const detail = (payload as {message?: string} | null)?.message;
        throw new Error(detail || `The server returned ${response.status}.`);
    }

    return fromWire(payload);
}

/**
 * Loads the blob, sharing one request between every caller and serving a cached
 * copy until it is CACHE_TTL_MS old.
 *
 * Never rejects. A reader whose settings could not be loaded gets the defaults
 * and an explanation, because a failed preference read is not worth taking the
 * panel down for.
 */
export function loadPreferences(): Promise<void> {
    if (isFresh()) {
        return Promise.resolve();
    }
    if (inflight) {
        return inflight;
    }

    setState({...state, loading: true, error: null});

    const startedAt = writes;

    inflight = request('GET').
        then((preferences) => {
            // A write finished while this was in the air, so it already knows
            // better than the answer being carried here.
            if (writes !== startedAt) {
                return;
            }
            loadedAt = now();
            setState({preferences, loading: false, error: null});
        }).
        catch((error: unknown) => {
            if (writes !== startedAt) {
                return;
            }

            // loadedAt stays where it was, so a failed read does not refresh
            // the clock and the next panel to open tries again.
            //
            // The last good blob is KEPT rather than replaced with the
            // defaults. This used to write EMPTY_PREFERENCES unconditionally,
            // which was right for a first load and wrong for every one after
            // it: the cache has a lifetime now, so the first mount past it
            // starts a refresh, and one blip reverted the reader's hidden rows
            // and timezone table on screen with nothing saying why. Because a
            // failed read deliberately does not stamp the clock, every
            // subsequent mount retried, so the rows flipped back and forth for
            // as long as the server was unwell. Degrading to the defaults is
            // still correct when nothing has ever loaded.
            setState({
                preferences: loadedAt === null ? EMPTY_PREFERENCES : state.preferences,
                loading: false,
                error: messageOf(error),
            });
        }).
        finally(() => {
            inflight = null;
        });

    return inflight;
}

/**
 * Saves ONE decorator's settings, leaving every other decorator's alone.
 *
 * Section-scoped rather than whole-blob, and it re-reads immediately before
 * writing, because both halves of that are load-bearing:
 *
 * - A `PUT` replaces the entire blob, so a caller that built one from its own
 *   state would delete whatever the reader had chosen in another decorator's
 *   editor. Spreading the cached blob fixes that only while the cache is right.
 * - `loadPreferences` fetches once per page load and never again, so the cache
 *   is as stale as the tab is old. Two tabs, or one tab left open while the
 *   reader changed something elsewhere, and the spread copy carries a snapshot
 *   from minutes ago straight back over the top of the newer one.
 *
 * The re-read narrows that window from the lifetime of the tab to the length of
 * one request. It does not close it: two saves landing inside that window still
 * resolve last-write-wins, and closing it properly needs a revision the server
 * checks. The narrow version is worth having on its own, and is the reason
 * neither editor may call this with a blob it assembled itself.
 *
 * Rejects on failure, unlike the load: a save that quietly did nothing would
 * leave the reader believing their settings had been kept.
 */
export async function savePreferencesSection<K extends keyof Preferences>(
    section: K,
    value: Preferences[K],
): Promise<void> {
    const current = await request('GET');
    const preferences = await request('PUT', {...current, [section]: value});

    writes++;
    loadedAt = now();
    setState({preferences, loading: false, error: null});
}

/**
 * Restores ONE decorator's defaults, which is what "Restore defaults" does.
 *
 * The zero value of a section is what "no choice made" looks like, so writing it
 * is a restore rather than a write of today's defaults: an empty row list is not
 * today's rows, it is the absence of a preference, and the reader keeps tracking
 * whatever the rows become. That is the property CLAUDE.md describes, and it
 * survives being a write.
 *
 * The blob is DELETED only when every section is back to its zero value, which
 * keeps the store from accumulating entries that say nothing. Before this was
 * scoped, "Restore defaults" under a legend reading "Rows to show" deleted the
 * whole blob and took the reader's timezone table with it.
 */
export async function resetPreferencesSection<K extends keyof Preferences>(section: K): Promise<void> {
    const current = await request('GET');
    const next = {...current, [section]: EMPTY_PREFERENCES[section]};

    const preferences = hasNoChoices(next) ? await request('DELETE') : await request('PUT', next);

    writes++;
    loadedAt = now();
    setState({preferences, loading: false, error: null});
}

/**
 * Whether a blob records no choice at all.
 *
 * Spelled out per section rather than deep-compared, so that adding a decorator
 * is a compile error here rather than a silently missed case: a new section this
 * does not mention would keep the blob alive forever.
 */
function hasNoChoices(preferences: Preferences): boolean {
    return preferences.dtg.zones.length === 0 &&
        preferences.dtg.urgentWithinMinutes === 0 &&
        preferences.location.hiddenRows.length === 0;
}

/**
 * Subscribes a component to the reader's preferences, loading them on first
 * use.
 *
 * Safe to call from as many components as you like: they share one request and
 * one snapshot.
 */
export function usePreferences(): PreferencesState {
    const current = useSyncExternalStore(subscribe, getState, getState);

    // Safe to fire and forget: loadPreferences never rejects, and it is a no-op
    // while the cached blob is still fresh.
    //
    // On mount only, deliberately. Panels and hovers mount constantly as a
    // reader moves around a channel, so this is what turns the TTL into a
    // refresh: the first one to open after it lapses does the read, and the
    // rest of that half hour is served from memory.
    useEffect(() => {
        loadPreferences();
    }, []);

    return current;
}

/** @internal exported for tests */
export function _resetForTesting(): void { // eslint-disable-line no-underscore-dangle, @typescript-eslint/naming-convention
    state = INITIAL_STATE;
    loadedAt = null;
    inflight = null;
    writes = 0;
    now = () => Date.now();
    listeners.clear();
}

/**
 * Drives the clock the cache ages against.
 *
 * @internal exported for tests
 */
export function _setClockForTesting(clock: () => number): void { // eslint-disable-line no-underscore-dangle, @typescript-eslint/naming-convention
    now = clock;
}
