import {useEffect, useSyncExternalStore} from 'react';

import type {Features} from './types';
import {ALL_FEATURES, NO_FEATURES, fromWire} from './types';

import {pluginBaseUrl} from '../plugin_url';
import {CACHE_TTL_MS} from '../preferences/store';

export interface FeaturesState {

    /** Which surfaces are on. NO_FEATURES until the server has answered. */
    features: Features;

    /** True while a read is in flight. */
    loading: boolean;

    /** Why the last read failed, or null. */
    error: string | null;
}

const INITIAL_STATE: FeaturesState = {
    features: NO_FEATURES,
    loading: false,
    error: null,
};

/*
 * Module state, not React state, for the reason preferences/store.ts is: every
 * panel, every hover card and every coordinate-only post in the rendered window
 * asks the same question, and they mount and unmount constantly as a reader
 * moves around a channel. One request per tab per TTL, however many of them
 * appear.
 *
 * The in-flight promise matters as much as the cached answer. Mattermost renders
 * on the order of thirty posts at a time, so without it the first paint of a
 * channel full of coordinates would issue thirty identical requests before any
 * of them landed.
 */
let state = INITIAL_STATE;
let inflight: Promise<void> | null = null;

/** When the answer was read, or null when there is nothing cached. */
let loadedAt: number | null = null;

/**
 * The clock, so a test can drive the TTL rather than wait out thirty minutes.
 */
let now = (): number => Date.now();

function isFresh(): boolean {
    return loadedAt !== null && now() - loadedAt < CACHE_TTL_MS;
}

const listeners = new Set<() => void>();

function setState(next: FeaturesState): void {
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
 * `useSyncExternalStore` needs to avoid re-rendering forever. Returning a fresh
 * object here would loop.
 */
export function getState(): FeaturesState {
    return state;
}

function endpoint(): string {
    return `${pluginBaseUrl()}/api/v1/features`;
}

function messageOf(error: unknown): string {
    return error instanceof Error && error.message ? error.message : 'Something went wrong.';
}

/**
 * How long a read may hang before it is treated as failed.
 *
 * The same bound `basemap.ts` puts on the archive header, for the same reason: a
 * fetch that STALLS never rejects, so without it `inflight` is never cleared,
 * every later caller joins the same pending promise, and the store sits on
 * NO_FEATURES for the life of the tab. That fails CLOSED, which is the one
 * direction this store must never fail in: every map off, on every surface, with
 * nothing distinguishing it from an admin having switched them off.
 *
 * Aborting turns the stall into a rejection, which the caller already degrades
 * correctly and does not cache.
 */
let fetchTimeoutMs = 10000;

async function request(): Promise<Features> {
    const controller = new AbortController();
    const timer = setTimeout(() => controller.abort(), fetchTimeoutMs);

    let response: Response;
    try {
        response = await fetch(endpoint(), {
            method: 'GET',
            credentials: 'same-origin',
            signal: controller.signal,
            headers: {'X-Requested-With': 'XMLHttpRequest'},
        });
    } finally {
        clearTimeout(timer);
    }

    const payload = await response.json().catch(() => null);
    if (!response.ok) {
        const detail = (payload as {message?: string} | null)?.message;
        throw new Error(detail || `The server returned ${response.status}.`);
    }

    return fromWire(payload);
}

/**
 * Loads which surfaces are on, sharing one request between every caller and
 * serving the cached answer until it is CACHE_TTL_MS old.
 *
 * Never rejects. A FIRST read that fails degrades to ALL_FEATURES rather than to
 * NO_FEATURES, for the reason those two constants record: a moment of not
 * knowing is not the same as an admin's decision, and hiding a feature nobody
 * switched off is the worse of the two ways to be wrong for longer than a
 * moment.
 *
 * A failed REFRESH keeps the last good answer instead, which is the same
 * distinction preferences/store.ts draws and for a worse reason. The cache
 * lapses every CACHE_TTL_MS, so on an install that turned maps off one failed
 * refresh would otherwise flip every surface back on and start the basemap
 * archive downloading: the exact transfer the switch exists to prevent, on the
 * exact installs that asked not to have it. And because a failure does not stamp
 * the clock, every following mount retried, so it flapped for as long as the
 * server was unwell.
 *
 * A failed read does not stamp the clock, so the next panel to open tries again
 * instead of a single bad minute lasting half an hour.
 */
export function loadFeatures(): Promise<void> {
    if (isFresh()) {
        return Promise.resolve();
    }
    if (inflight) {
        return inflight;
    }

    setState({...state, loading: true, error: null});

    inflight = request().
        then((features) => {
            loadedAt = now();
            setState({features, loading: false, error: null});
        }).
        catch((error: unknown) => {
            // loadedAt is the test for "has a good answer ever landed", and it
            // is deliberately not stamped here, so it still reads null after any
            // number of failures.
            setState({
                features: loadedAt === null ? ALL_FEATURES : state.features,
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
 * Subscribes a component to this install's feature switches, loading them on
 * first use.
 *
 * Safe to call from as many components as you like: they share one request and
 * one snapshot. On mount only, deliberately, which is what turns the TTL into a
 * refresh rather than a poll.
 */
export function useFeatures(): FeaturesState {
    const current = useSyncExternalStore(subscribe, getState, getState);

    useEffect(() => {
        loadFeatures();
    }, []);

    return current;
}

/** @internal exported for tests */
export function _resetForTesting(): void { // eslint-disable-line no-underscore-dangle, @typescript-eslint/naming-convention
    state = INITIAL_STATE;
    loadedAt = null;
    inflight = null;
    now = () => Date.now();
    fetchTimeoutMs = 10000;
    listeners.clear();
}

/**
 * Shortens the hang timeout, so the test for it does not wait ten seconds.
 *
 * @internal exported for tests
 */
export function _setFetchTimeoutForTesting(ms: number): void { // eslint-disable-line no-underscore-dangle, @typescript-eslint/naming-convention
    fetchTimeoutMs = ms;
}

/**
 * Drives the clock the cache ages against.
 *
 * @internal exported for tests
 */
export function _setClockForTesting(clock: () => number): void { // eslint-disable-line no-underscore-dangle, @typescript-eslint/naming-convention
    now = clock;
}
