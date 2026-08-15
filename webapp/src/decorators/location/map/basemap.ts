import type {FeatureCollection} from 'geojson';
import manifest from 'manifest';

import {BASEMAP_SHA256} from './basemap_digest';

import {pluginBaseUrl} from '../../../plugin_url';

/**
 * The bundled Natural Earth basemap, fetched once and kept.
 *
 * Module state with no TTL, unlike the preferences cache: this file is
 * immutable for the life of an install, so there is nothing for a timer to
 * refresh. A channel full of coordinates makes one request, not one per panel.
 */

const FETCH_TIMEOUT_MS = 10000;
const MAX_BYTES = 2 * 1024 * 1024;

export type Basemap = FeatureCollection;

let cached: Basemap | null = null;
let inFlight: Promise<Basemap | null> | null = null;
let failed = false;

export function basemapUrl(): string {
    return `${pluginBaseUrl()}/public/map/world.geo.json?v=${encodeURIComponent(manifest.version)}`;
}

/**
 * Loads the basemap, or resolves null.
 *
 * Never throws. A DEFINITIVE failure (a 404, an oversized body, a digest
 * mismatch, a body that is not GeoJSON) is a property of the deploy and will not
 * change, so it is remembered and no further request is made: a broken deploy
 * must not become a request loop from every open panel.
 *
 * A TRANSIENT failure (a timeout, a network throw) is NOT remembered. Latching
 * on those means one stalled fetch on a DDIL link tells a reader the map is
 * broken for the rest of the session, with a reload the only way back, in the
 * environment this plugin is built for. Retrying costs one request per panel
 * the reader opens, which is one per click.
 */
export async function loadBasemap(): Promise<Basemap | null> {
    if (cached) {
        return cached;
    }
    if (failed) {
        return null;
    }
    if (inFlight) {
        return inFlight;
    }

    const attempt = fetchBasemap().then((result) => {
        if (result.map) {
            cached = result.map;
        } else if (result.definitive) {
            failed = true;
        }
        return result.map;
    });

    inFlight = attempt;

    // Identity-checked, so a settling attempt cannot null out a newer one that
    // replaced it after a reset.
    attempt.finally(() => {
        if (inFlight === attempt) {
            inFlight = null;
        }
    }).catch(() => {
        // fetchBasemap never rejects; this satisfies the no-floating-promise rule.
    });

    return attempt;
}

interface Attempt {
    map: Basemap | null;
    definitive: boolean;
}

function settled(map: Basemap | null): Attempt {
    return {map, definitive: true};
}

async function fetchBasemap(): Promise<Attempt> {
    const controller = new AbortController();
    const timer = setTimeout(() => controller.abort(), FETCH_TIMEOUT_MS);

    try {
        const response = await fetch(basemapUrl(), {

            // A static asset needs no session, and sending one is not free.
            credentials: 'omit',
            signal: controller.signal,
        });
        if (!response.ok) {
            return settled(null);
        }

        const bytes = await response.arrayBuffer();
        if (bytes.byteLength > MAX_BYTES) {
            return settled(null);
        }
        if (!await hasExpectedDigest(bytes)) {
            return settled(null);
        }

        const parsed: unknown = JSON.parse(new TextDecoder().decode(bytes));
        if (!isBasemap(parsed)) {
            return settled(null);
        }

        return settled(parsed);
    } catch {
        // A timeout or a network throw, which may not repeat.
        return {map: null, definitive: false};
    } finally {
        clearTimeout(timer);
    }
}

/**
 * Whether the bytes are the basemap this build was generated against.
 *
 * As much an availability control as a security one: air-gapped installs have
 * no way to re-fetch, so a truncated or half-deployed asset would otherwise
 * render a silently wrong-shaped world.
 *
 * `crypto.subtle` is undefined on a plain-HTTP origin, which for on-prem and
 * air-gapped installs is the norm, so an unverifiable digest passes rather than
 * disabling the map. That is the same posture the copy buttons take.
 */
async function hasExpectedDigest(bytes: ArrayBuffer): Promise<boolean> {
    if (typeof crypto === 'undefined' || !crypto.subtle) {
        return true;
    }

    try {
        const digest = await crypto.subtle.digest('SHA-256', bytes);
        const hex = Array.from(new Uint8Array(digest)).
            map((b) => b.toString(16).padStart(2, '0')).
            join('');

        return hex === BASEMAP_SHA256;
    } catch {
        return true;
    }
}

function isBasemap(value: unknown): value is Basemap {
    if (typeof value !== 'object' || value === null) {
        return false;
    }
    const candidate = value as {type?: unknown; features?: unknown};

    return candidate.type === 'FeatureCollection' && Array.isArray(candidate.features);
}

/** Test hook. Module state outlives a component, so a test must be able to clear it. */
export function _resetForTesting(): void { // eslint-disable-line no-underscore-dangle, @typescript-eslint/naming-convention
    cached = null;
    inFlight = null;
    failed = false;
}
