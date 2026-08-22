import manifest from 'manifest';

import {DATA_MAX_ZOOM, SEAM_ZOOM} from './span';

import {pluginBaseUrl} from '../../../plugin_url';

/**
 * The bundled basemap archives, probed once and kept.
 *
 * Module state with no TTL, unlike the preferences cache. That was once because
 * these archives were immutable for the life of an install, and for the bundled
 * basemap it still is: it ships inside the plugin and its URL carries the plugin
 * version, so an upgrade is a different URL. A **detail package is not**. The
 * System Console uploader and the drop-in directory both replace one in place,
 * under a URL that changes with nothing, so what is memoised here is a header
 * that can go stale.
 *
 * The consequence is bounded and is not corruption: a map already built keeps
 * the zooms and bounds it opened with, and the next page load reads them again.
 * The dangerous half is the tile reader rather than this memo, since PMTiles
 * offsets taken from one file are meaningless against another, and that is
 * handled where it belongs: the server sends an ETag over size and modification
 * time, and pmtiles.js re-reads the directory when it moves. Adding a timer here
 * would refresh a header nothing re-reads without also rebuilding the style,
 * which the map's creation effect does once.
 *
 * A channel full of coordinates makes one request each, not one per panel.
 *
 * Only each archive's 127-byte header is read here. The tiles themselves are
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
    bounds: Bounds;
}

/** west, south, east, north, in degrees. */
export type Bounds = [number, number, number, number];

type ZoomRule = (minZoom: number, maxZoom: number) => boolean;

export function basemapUrl(): string {
    return `${pluginBaseUrl()}/public/map/world.pmtiles?v=${encodeURIComponent(manifest.version)}`;
}

/*
 * One detail package's archive.
 *
 * Served by the plugin's own route rather than out of public/, because a
 * package may have been dropped into a directory on the server that the bundle
 * knows nothing about. The route reads both, so this one URL shape covers a
 * bundled area and an operator's alike and the client never has to know which
 * it is looking at.
 *
 * The name is built into a URL here, so it is checked here too. It arrives from
 * a filename on a server and is whitelisted there, but this is a second
 * language reading the same value and the cost of them agreeing only by
 * accident is a request for something else.
 */
export const PACKAGE_NAME = /^[a-z0-9]+(-[a-z0-9]+)+$/;

export function packageUrl(name: string): string {
    return `${pluginBaseUrl()}/packages/${name}.pmtiles?v=${encodeURIComponent(manifest.version)}`;
}

/**
 * The global tier goes to the bottom and at least as deep as the style asks.
 * The detail tier starts exactly where the seam is, and its depth is a FLOOR
 * rather than an equality: an install may ship a shallower profile than the
 * deepest one this pipeline can build, and rejecting it would leave that
 * operator with no detail and, by the silence rule below, nothing saying so.
 */
const globalZooms: ZoomRule = (minZoom, maxZoom) => minZoom === 0 && maxZoom >= DATA_MAX_ZOOM;
const detailZooms: ZoomRule = (minZoom, maxZoom) => minZoom === SEAM_ZOOM && maxZoom >= SEAM_ZOOM;

interface Probe {
    load: () => Promise<Archive | null>;
    reset: () => void;
}

/**
 * One fetch-and-latch machine, used for the global tier and once per package.
 *
 * The definitive/transient split, the timeout, and the in-flight identity check
 * below are the parts of this module whose comments record having been got
 * wrong; a second copy of them is a second chance to get them wrong again. What
 * actually differs between archives is the URL, the zoom rule, and whether a
 * transient failure is worth saying anything about.
 */
function createProbe(url: () => string, zooms: ZoomRule, onTransient?: (url: string) => void): Probe {
    let cached: Archive | null = null;
    let inFlight: Promise<Archive | null> | null = null;
    let failed = false;

    const load = (): Promise<Archive | null> => {
        if (cached) {
            return Promise.resolve(cached);
        }
        if (failed) {
            return Promise.resolve(null);
        }
        if (inFlight) {
            return inFlight;
        }

        const target = url();
        const attempt = probeArchive(target, zooms).then((result) => {
            if (result.archive) {
                cached = result.archive;
            } else if (result.definitive) {
                failed = true;
            } else {
                onTransient?.(target);
            }
            return result.archive;
        });

        inFlight = attempt;

        // Identity-checked, so a settling attempt cannot null out a newer one
        // that replaced it after a reset.
        attempt.finally(() => {
            if (inFlight === attempt) {
                inFlight = null;
            }
        }).catch(() => {
            // probeArchive never rejects; this satisfies the no-floating-promise rule.
        });

        return attempt;
    };

    return {
        load,
        reset: () => {
            cached = null;
            inFlight = null;
            failed = false;
        },
    };
}

const globalProbe = createProbe(basemapUrl, globalZooms);

/** One probe per package, kept for the life of the tab as the global one is. */
const packageProbes = new Map<string, Probe>();

function packageProbe(name: string): Probe {
    let probe = packageProbes.get(name);
    if (!probe) {
        probe = createProbe(() => packageUrl(name), detailZooms, (target) => {
            // eslint-disable-next-line no-console
            console.warn('[tactical-fusion] detail package unreachable, retrying on the next map', target);
        });
        packageProbes.set(name, probe);
    }

    return probe;
}

/**
 * Probes the global basemap, or resolves null.
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
export function loadBasemap(): Promise<Archive | null> {
    return globalProbe.load();
}

export interface DetailArchive {
    name: string;
    archive: Archive;
}

/*
 * Probes every package this install says it has, and resolves the ones that
 * answered.
 *
 * A package that does not answer is DROPPED rather than failing the set. An
 * install with six areas and one bad archive draws the other five, which is the
 * only way this can degrade that an operator would want: the alternative is one
 * half-copied file taking every area off the map at once.
 *
 * A missing package is a CONFIGURATION rather than a fault, so a 404 says
 * nothing: a global-only install is supported, and so is a list naming an area
 * this node has not been given yet. A TRANSIENT failure is the opposite and is
 * logged, because it is the one case that looks identical on screen: the style
 * is built once inside the map's creation effect and the map is created once
 * and then moved, so a timed-out probe leaves a panel missing that area for the
 * whole of its life and renders exactly like an install that never had it.
 */
export async function loadPackages(names: readonly string[]): Promise<DetailArchive[]> {
    const wanted = names.filter((name) => PACKAGE_NAME.test(name));

    const probed = await Promise.all(wanted.map(async (name) => {
        const archive = await packageProbe(name).load();

        return archive === null ? null : {name, archive};
    }));

    return probed.filter((entry): entry is DetailArchive => entry !== null);
}

interface Attempt {
    archive: Archive | null;
    definitive: boolean;
}

function settled(archive: Archive | null): Attempt {
    return {archive, definitive: true};
}

async function probeArchive(url: string, zooms: ZoomRule): Promise<Attempt> {
    const controller = new AbortController();
    const timer = setTimeout(() => controller.abort(), FETCH_TIMEOUT_MS);

    try {
        const response = await fetch(url, {

            // A static asset needs no session, and sending one is not free.
            credentials: 'omit',
            signal: controller.signal,
            headers: {Range: `bytes=0-${HEADER_BYTES - 1}`},
        });

        if (!response.ok) {
            return settled(null);
        }

        // A 200 here means the server ignored Range and is sending the whole
        // archive. That is NOT usable, however healthy it looks: the pmtiles
        // reader applies this same test to every tile request and throws
        // ("Check that your storage backend supports HTTP Byte Serving"), so
        // accepting it would pass a deploy whose every tile then fails, and
        // the map would draw water with no land and say nothing.
        //
        // Reading Content-Length rather than the body is also what keeps this
        // from pulling 24 MB to look at 127 bytes, which is the whole reason
        // the old MAX_BYTES cap existed.
        // A non-numeric Content-Length has to fail this too. `Number('gzip')`
        // is NaN and every comparison against NaN is false, so a 200 carrying
        // one was ACCEPTED as a valid range response, which is the one thing
        // this branch exists to refuse.
        const header = response.headers.get('Content-Length');
        const length = header === null || header.trim() === '' ? NaN : Number(header);
        if (response.status === 200 && (!Number.isFinite(length) || length > HEADER_BYTES)) {
            return settled(null);
        }

        return settled(readHeader(url, await response.arrayBuffer(), zooms));
    } catch {
        // A timeout or a network throw, which may not repeat.
        return {archive: null, definitive: false};
    } finally {
        clearTimeout(timer);
    }
}

/**
 * Validates an archive header, or returns null.
 *
 * The zoom range is read rather than assumed so the camera and the data cannot
 * drift apart: an archive built shallower than the style asks for would
 * otherwise render blank at the zooms it lacks, which looks like a rendering
 * bug rather than a data one.
 */
function readHeader(url: string, header: ArrayBuffer, zooms: ZoomRule): Archive | null {
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
    if (!zooms(minZoom, maxZoom)) {
        return null;
    }

    const degrees = new DataView(header);
    const bounds: Bounds = [
        degrees.getInt32(102, true) / 1e7,
        degrees.getInt32(106, true) / 1e7,
        degrees.getInt32(110, true) / 1e7,
        degrees.getInt32(114, true) / 1e7,
    ];

    return {url, minZoom, maxZoom, bounds};
}

/** Test hook. Module state outlives a component, so a test must be able to clear it. */
export function _resetForTesting(): void { // eslint-disable-line no-underscore-dangle, @typescript-eslint/naming-convention
    globalProbe.reset();
    packageProbes.clear();
}
