import manifest from 'manifest';

import {MAX_ZOOM} from './span';

import {pluginBaseUrl} from '../../../plugin_url';

/**
 * The bundled Natural Earth basemap, probed once and kept.
 *
 * Module state with no TTL, unlike the preferences cache: this archive is
 * immutable for the life of an install, so there is nothing for a timer to
 * refresh. A channel full of coordinates makes one request, not one per panel.
 *
 * Only the archive's 127-byte header is read here. The tiles themselves are
 * fetched lazily by MapLibre through the pmtiles protocol, so there is never a
 * whole body in the browser and the whole-file SHA-256 this module used to
 * compute has no equivalent. That check now runs where the whole file is in
 * hand, when the bundle is assembled; it was always weakest here anyway, since
 * `crypto.subtle` is undefined on the plain-HTTP origins that on-prem and
 * air-gapped installs typically run on, so it silently passed on exactly the
 * installs it existed for.
 *
 * What is still worth asking on arrival is whether this is the archive we think
 * it is: a 404 page, a captive portal's HTML, or a half-copied file all fail the
 * magic bytes immediately, and none of them should reach MapLibre.
 */

const FETCH_TIMEOUT_MS = 10000;

const HEADER_BYTES = 127;
const MAGIC = 'PMTiles';
const SPEC_VERSION = 3;
const TILE_TYPE_MVT = 1;

export interface Archive {
    url: string;
    minZoom: number;
    maxZoom: number;
}

let cached: Archive | null = null;
let inFlight: Promise<Archive | null> | null = null;
let failed = false;

export function basemapUrl(): string {
    return `${pluginBaseUrl()}/public/map/world.pmtiles?v=${encodeURIComponent(manifest.version)}`;
}

/**
 * Probes the basemap, or resolves null.
 *
 * Never throws. A DEFINITIVE failure (a 404, a body that is not a PMTiles
 * archive, a tile type or zoom range this style cannot draw) is a property of
 * the deploy and will not change, so it is remembered and no further request is
 * made: a broken deploy must not become a request loop from every open panel.
 *
 * A TRANSIENT failure (a timeout, a network throw) is NOT remembered. Latching
 * on those means one stalled fetch on a DDIL link tells a reader the map is
 * broken for the rest of the session, with a reload the only way back, in the
 * environment this plugin is built for. Retrying costs one request per panel
 * the reader opens, which is one per click.
 */
export async function loadBasemap(): Promise<Archive | null> {
    if (cached) {
        return cached;
    }
    if (failed) {
        return null;
    }
    if (inFlight) {
        return inFlight;
    }

    const attempt = probeArchive().then((result) => {
        if (result.archive) {
            cached = result.archive;
        } else if (result.definitive) {
            failed = true;
        }
        return result.archive;
    });

    inFlight = attempt;

    // Identity-checked, so a settling attempt cannot null out a newer one that
    // replaced it after a reset.
    attempt.finally(() => {
        if (inFlight === attempt) {
            inFlight = null;
        }
    }).catch(() => {
        // probeArchive never rejects; this satisfies the no-floating-promise rule.
    });

    return attempt;
}

interface Attempt {
    archive: Archive | null;
    definitive: boolean;
}

function settled(archive: Archive | null): Attempt {
    return {archive, definitive: true};
}

async function probeArchive(): Promise<Attempt> {
    const controller = new AbortController();
    const timer = setTimeout(() => controller.abort(), FETCH_TIMEOUT_MS);
    const url = basemapUrl();

    try {
        const response = await fetch(url, {

            // A static asset needs no session, and sending one is not free.
            credentials: 'omit',
            signal: controller.signal,
            headers: {Range: `bytes=0-${HEADER_BYTES - 1}`},
        });

        // A server that ignores Range answers 200 with the whole archive. That
        // is still usable, so it is not a failure; only the header is read.
        if (!response.ok && response.status !== 206) {
            return settled(null);
        }

        return settled(readHeader(url, await response.arrayBuffer()));
    } catch {
        // A timeout or a network throw, which may not repeat.
        return {archive: null, definitive: false};
    } finally {
        clearTimeout(timer);
    }
}

/**
 * Validates the archive header, or returns null.
 *
 * `maxZoom` is read rather than assumed so the camera and the data cannot drift
 * apart: an archive built shallower than the style asks for would otherwise
 * render blank at the zooms it lacks, which looks like a rendering bug rather
 * than a data one.
 */
function readHeader(url: string, header: ArrayBuffer): Archive | null {
    if (header.byteLength < HEADER_BYTES) {
        return null;
    }

    const bytes = new Uint8Array(header);
    const magic = String.fromCharCode(...Array.from(bytes.subarray(0, MAGIC.length)));
    if (magic !== MAGIC || bytes[7] !== SPEC_VERSION) {
        return null;
    }
    if (bytes[99] !== TILE_TYPE_MVT) {
        return null;
    }

    const minZoom = bytes[100];
    const maxZoom = bytes[101];
    if (minZoom !== 0 || maxZoom < MAX_ZOOM) {
        return null;
    }

    return {url, minZoom, maxZoom};
}

/** Test hook. Module state outlives a component, so a test must be able to clear it. */
export function _resetForTesting(): void { // eslint-disable-line no-underscore-dangle, @typescript-eslint/naming-convention
    cached = null;
    inFlight = null;
    failed = false;
}
