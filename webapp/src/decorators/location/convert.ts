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
export function _asConversionForTesting(body: unknown): Conversion { // eslint-disable-line no-underscore-dangle, @typescript-eslint/naming-convention
    return asConversion(body);
}

export function asConversion(body: unknown): Conversion {
    if (typeof body !== 'object' || body === null) {
        throw new Error('The server did not return a conversion.');
    }

    const value = body as Record<string, unknown>;
    for (const key of ['mgrs', 'utm', 'decimal', 'dms', 'ddm', 'usmtf', 'region']) {
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

/**
 * Fetches the derived readings for one coordinate.
 *
 * No cache, module-level or otherwise. This is one request per opened panel,
 * triggered by a human clicking a link, answering with a few hundred bytes the
 * browser is told it may keep for five minutes. The preferences store caches
 * because every hover card in a channel wants the same blob; nothing here is
 * asked for more than once by more than one component.
 *
 * Never throws and never leaves the panel without something to show. A failed
 * conversion costs the grid rows on a lat/lon link and the position on a grid
 * link, and everything else on the panel was computed locally and is already on
 * screen.
 */
export function useConversion(
    format: LocationFormat, canonical: string, raw: string): ConversionState {
    const [state, setState] = useState<ConversionState>({status: 'loading', data: null});

    // Which coordinate the state above describes.
    //
    // The separator is written as an escape, not typed. It was two literal NUL
    // bytes, which made git classify this file as binary: `git diff` reported
    // `Bin 0 -> 5605 bytes`, `--numstat` gave no line counts at all, and grep
    // skipped it. The most safety-critical file in this decorator was the one
    // nobody could review as a diff. Keep it an escape.
    const key = `${format}\u0000${canonical}\u0000${raw}`;
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
    // frame exists to be seen.
    if (current !== key) {
        setCurrent(key);
        setState({status: 'loading', data: null});
    }

    useEffect(() => {
        // A reply arriving after the reader has clicked a second coordinate
        // must not be written onto the first one's successor. Cleared by the
        // cleanup below rather than by comparing payloads, because the reply
        // carries nothing that identifies which request it answers.
        let live = true;

        fetchConversion(format, canonical, raw).then((data) => {
            if (live) {
                setState({status: 'ready', data});
            }
        }, (error: unknown) => {
            if (live) {
                setState({
                    status: error instanceof RejectedError ? 'rejected' : 'failed',
                    data: null,
                });
            }
        });

        return () => {
            live = false;
        };
    }, [format, canonical, raw]);

    return state;
}
