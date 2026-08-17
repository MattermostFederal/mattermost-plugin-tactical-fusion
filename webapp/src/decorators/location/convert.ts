import {useEffect, useState} from 'react';

import type {LocationFormat} from './format';

import {pluginBaseUrl} from '../../plugin_url';

/**
 * The rows the panel cannot work out for itself, already rendered.
 *
 * Exactly those and no others. `format.ts` is a full renderer, pinned against
 * `format.go` by paired fixtures, and it handles every textual grammar; what it
 * does not have is a projection. So the two grid rows are here because they
 * need one, and the three coordinate rows are here because a grid token has no
 * `Coordinate` to render from and its resolution is linear rather than angular.
 *
 * Mirrors `Conversion` in `server/decorators/location/convert.go`.
 */
export interface Conversion {
    mgrs: string;
    utm: string;
    decimal: string;
    dms: string;
    ddm: string;
    usmtf: string;
    region: string;

    lat: number;
    lon: number;
}

/**
 * `rejected` and `failed` are different answers and the panel treats them
 * differently.
 *
 * `failed` means the request did not arrive: offline, a proxy, a restart. The
 * link is presumed good and the panel keeps everything it worked out locally.
 *
 * `rejected` means the server looked at the link and said it is not one this
 * plugin issued. That is a verdict rather than an outage, and it is the only
 * way the panel can learn it, because two of the four checks on the author's
 * text need the token grammar and that lives in Go.
 */
export type ConversionStatus = 'loading' | 'ready' | 'failed' | 'rejected';

export interface ConversionState {
    status: ConversionStatus;
    data: Conversion | null;
}

/**
 * The author's own text, but only once the server has vouched for it.
 *
 * Two of the four gates on `r` need the token grammar, which lives in Go, so
 * this side can check its length and its alphabet and nothing more, and that
 * alphabet had to widen to the whole Latin alphabet for grid letters. A
 * hand-written link can therefore put a short run of words in `r` that nothing
 * here can tell from a coordinate: `r=DISREGARD USE 18SUJ11111111` passes both
 * local gates.
 *
 * Rendering it before the verdict arrives put that string on screen labeled as
 * the author's words, with a copy button beside it, on every open of every link
 * during loading, and permanently whenever the request failed. Falling back to
 * the canonical token means what is shown is always true: either the author's
 * text as the server confirmed it, or the token the link is built from.
 */
export function vouchedText(state: ConversionState, raw: string, canonical: string): string {
    return state.status === 'ready' ? raw : canonical;
}

/** Thrown for a 400, which is the server's verdict rather than an outage. */
class RejectedError extends Error {}

function endpoint(format: LocationFormat, canonical: string, raw: string): string {
    const query = new URLSearchParams({f: format, v: canonical});

    // Omitted when it would only repeat the token, exactly as the server omits
    // it from a link it writes.
    if (raw && raw !== canonical) {
        query.set('r', raw);
    }

    return `${pluginBaseUrl()}/api/v1/convert?${query.toString()}`;
}

async function fetchConversion(
    format: LocationFormat, canonical: string, raw: string): Promise<Conversion> {
    const response = await fetch(endpoint(format, canonical, raw), {
        credentials: 'same-origin',

        // Mattermost accepts session-cookie authentication only for requests
        // that could not have been a cross-site form post.
        headers: {'X-Requested-With': 'XMLHttpRequest'},
    });

    if (response.status === 400) {
        throw new RejectedError('not a coordinate this plugin issued');
    }
    if (!response.ok) {
        throw new Error(`The server returned ${response.status}.`);
    }

    return asConversion(await response.json());
}

/**
 * Turns a decoded body into a Conversion, or refuses it.
 *
 * `response.ok` is any 2xx, and a captive portal or a transparent proxy, which
 * is the ordinary DDIL failure, answers 200 with something else entirely. An
 * unchecked cast would leave `lat` undefined and put the pin at NaN.
 *
 * The position test is `Number.isFinite`, never a truthiness check: 0, 0 is a
 * real position, and dropping the equator and the prime meridian is a bug this
 * plugin deliberately did not inherit.
 */
type ConversionStringField = Exclude<keyof Conversion, 'lat' | 'lon'>;

const CONVERSION_STRING_FIELDS: readonly ConversionStringField[] =
    ['mgrs', 'utm', 'decimal', 'dms', 'ddm', 'usmtf', 'region'];

export function asConversion(body: unknown): Conversion {
    if (typeof body !== 'object' || body === null) {
        throw new Error('The server did not return a conversion.');
    }

    const value = body as Record<string, unknown>;
    for (const key of CONVERSION_STRING_FIELDS) {
        if (typeof value[key] !== 'string') {
            throw new Error('The server did not return a conversion.');
        }
    }

    const {lat, lon} = value;
    if (typeof lat !== 'number' || !Number.isFinite(lat) || Math.abs(lat) > 90) {
        throw new Error('The server did not return a position.');
    }
    if (typeof lon !== 'number' || !Number.isFinite(lon) || Math.abs(lon) > 180) {
        throw new Error('The server did not return a position.');
    }

    return value as unknown as Conversion;
}

/*
 * Module state, not React state, and the reason changed when hovers arrived.
 *
 * This deliberately had no cache while a conversion was one request per panel
 * open, triggered by a human clicking a link. A hover fires on pointer movement
 * instead, so an uncached conversion puts a request behind every coordinate a
 * cursor crosses in a busy channel, which is exactly why the location decorator
 * carried no hover until there was one of these.
 *
 * It needs no TTL, unlike the preferences store, and that is a property of what
 * is being cached rather than a shortcut. A conversion is a PURE FUNCTION of
 * (format, canonical, raw): the same token converts to the same readings
 * forever, because the projection is arithmetic and the region comes from
 * polygons compiled into the binary. There is nothing to go stale. A blob of
 * reader preferences can be changed from another tab; a grid reference cannot.
 */
const answers = new Map<string, ConversionState>();
const inflight = new Map<string, Promise<ConversionState>>();

/*
 * The cache key.
 *
 * The separator is written as an escape, not typed. It was two literal NUL
 * bytes, which made git classify this file as binary: `git diff` reported
 * `Bin 0 -> 5605 bytes`, `--numstat` gave no line counts at all, and grep
 * skipped it. The most safety-critical file in this decorator was the one
 * nobody could review as a diff. Keep it an escape.
 */
function cacheKey(format: LocationFormat, canonical: string, raw: string): string {
    return `${format}\u0000${canonical}\u0000${raw}`;
}

/*
 * Answers worth remembering are the ones the server DECIDED.
 *
 * `ready` and `rejected` are both verdicts about a fixed token and neither can
 * change under us. `failed` is an outage, so it is never cached: remembering it
 * would mean one bad minute costs every coordinate in the channel for the life
 * of the tab, with a reload the only way back. That is the same split
 * basemap.ts makes about the archive, for the same reason.
 */
function remembered(state: ConversionState): boolean {
    return state.status === 'ready' || state.status === 'rejected';
}

async function load(
    format: LocationFormat, canonical: string, raw: string): Promise<ConversionState> {
    try {
        return {status: 'ready', data: await fetchConversion(format, canonical, raw)};
    } catch (error: unknown) {
        return {
            status: error instanceof RejectedError ? 'rejected' : 'failed',
            data: null,
        };
    }
}

/*
 * One request per token, however many components ask at once.
 *
 * The in-flight map is what makes a hover and the click that follows it share a
 * single request rather than race: the panel opens while the hover's fetch is
 * still outstanding and joins it instead of issuing a second.
 */
function request(
    format: LocationFormat, canonical: string, raw: string): Promise<ConversionState> {
    const key = cacheKey(format, canonical, raw);

    // Answered already. Checked HERE and not only in the hook below: a cache
    // that only one caller consults is not a cache, and the next thing to want a
    // conversion would have gone to the network with every appearance of being
    // cached.
    const answer = answers.get(key);
    if (answer) {
        return Promise.resolve(answer);
    }

    const pending = inflight.get(key);
    if (pending) {
        return pending;
    }

    const started: Promise<ConversionState> = load(format, canonical, raw).then((state) => {
        // Identity-checked, the same way basemap.ts checks its own in-flight
        // slot: a request settling after the map was cleared must not write its
        // answer back under a key nothing is waiting for, and must not delete a
        // NEWER attempt that has since taken the slot, which would send the
        // next joiner to the network with every appearance of being cached.
        if (inflight.get(key) !== started) {
            return state;
        }
        if (remembered(state)) {
            answers.set(key, state);
        }
        inflight.delete(key);

        return state;
    });

    inflight.set(key, started);

    return started;
}

const LOADING: ConversionState = {status: 'loading', data: null};

/**
 * Fetches the derived readings for one coordinate, through the cache above.
 *
 * Never throws and never leaves the panel without something to show. A failed
 * conversion costs the grid rows on a lat/lon link and the position on a grid
 * link, and everything else on the panel was computed locally and is already on
 * screen.
 */
export function useConversion(
    format: LocationFormat, canonical: string, raw: string): ConversionState {
    const key = cacheKey(format, canonical, raw);
    const [state, setState] = useState<ConversionState>(() => answers.get(key) ?? LOADING);
    const [current, setCurrent] = useState(key);

    // Reset DURING RENDER, not in the effect below, and this is load-bearing.
    //
    // The sidebar keeps this panel mounted across a change of selection, so a
    // second coordinate arrives as a prop change. Effects run after commit and
    // after paint, so clearing the state there left one committed frame in
    // which the token, the heading and the locally-computed rows were the new
    // coordinate's while the server-derived rows were still the old one's. That
    // frame really rendered: a heading naming one grid square above a latitude
    // 3,800 km away, with a copy button armed on the wrong value.
    //
    // This is React's documented way to adjust state when a prop changes. It
    // re-renders immediately, before the browser paints anything, so no mixed
    // frame exists to be seen. Reading the cache here rather than dropping to
    // `loading` is what makes a coordinate the reader has already hovered open
    // with its grid rows already on screen.
    if (current !== key) {
        setCurrent(key);
        setState(answers.get(key) ?? LOADING);
    }

    useEffect(() => {
        // Read the cache AGAIN here, and adopt what it holds rather than
        // bailing on it. Effects flush in a scheduled task after commit, so a
        // reply can land in the window between the render above and this line:
        // the render saw nothing and left `loading`, and an early return here
        // would leave it there permanently, because these deps cannot change
        // again for the same token. The answer sat in the cache while the panel
        // said `converting…` forever.
        //
        // That window is the ordinary case rather than a corner: it is exactly
        // a hover starting the request and the click that follows it mounting
        // the panel before the reply arrives.
        //
        // setState with the value already in state is a React bail-out, so the
        // hit-on-render path costs nothing.
        const answered = answers.get(key);
        if (answered) {
            setState(answered);
            return undefined;
        }

        // A reply arriving after the reader has clicked a second coordinate
        // must not be written onto the first one's successor. Cleared by the
        // cleanup below rather than by comparing payloads, because the reply
        // carries nothing that identifies which request it answers.
        let live = true;

        request(format, canonical, raw).then((answer) => {
            if (live) {
                setState(answer);
            }
        });

        return () => {
            live = false;
        };
    }, [key, format, canonical, raw]);

    return state;
}

/** @internal exported so the cache can be exercised without mounting a component. */
export function _requestForTesting( // eslint-disable-line no-underscore-dangle, @typescript-eslint/naming-convention
    format: LocationFormat, canonical: string, raw: string): Promise<ConversionState> {
    return request(format, canonical, raw);
}

/** Test hook. Module state outlives a component, so a test must be able to clear it. */
export function _resetConversionsForTesting(): void { // eslint-disable-line no-underscore-dangle, @typescript-eslint/naming-convention
    answers.clear();
    inflight.clear();
}
