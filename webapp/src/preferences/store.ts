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
let loaded = false;
let inflight: Promise<void> | null = null;
const listeners = new Set<() => void>();

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
            'X-Requested-With': 'XMLHttpRequest', // eslint-disable-line @typescript-eslint/naming-convention
            'Content-Type': 'application/json', // eslint-disable-line @typescript-eslint/naming-convention
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
 * Loads the blob once, sharing one request between every caller.
 *
 * Never rejects. A reader whose settings could not be loaded gets the defaults
 * and an explanation, because a failed preference read is not worth taking the
 * panel down for.
 */
export function loadPreferences(): Promise<void> {
    if (loaded) {
        return Promise.resolve();
    }
    if (inflight) {
        return inflight;
    }

    setState({...state, loading: true, error: null});

    inflight = request('GET').
        then((preferences) => {
            loaded = true;
            setState({preferences, loading: false, error: null});
        }).
        catch((error: unknown) => {
            // loaded stays false, so the next panel to open tries again.
            setState({preferences: EMPTY_PREFERENCES, loading: false, error: messageOf(error)});
        }).
        finally(() => {
            inflight = null;
        });

    return inflight;
}

/**
 * Saves the blob and adopts what the server stored.
 *
 * Rejects on failure, unlike the load: a save that quietly did nothing would
 * leave the reader believing their settings had been kept.
 */
export async function savePreferences(next: Preferences): Promise<void> {
    const preferences = await request('PUT', next);
    loaded = true;
    setState({preferences, loading: false, error: null});
}

/**
 * Deletes the blob, which is what "Restore defaults" does.
 *
 * A delete rather than a write of today's defaults, so the reader goes back to
 * tracking whatever the defaults become.
 */
export async function resetPreferences(): Promise<void> {
    const preferences = await request('DELETE');
    loaded = true;
    setState({preferences, loading: false, error: null});
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
    // once something else has already loaded.
    useEffect(() => {
        loadPreferences();
    }, []);

    return current;
}

/** @internal exported for tests */
export function _resetForTesting(): void { // eslint-disable-line no-underscore-dangle, @typescript-eslint/naming-convention
    state = INITIAL_STATE;
    loaded = false;
    inflight = null;
    listeners.clear();
}
