import {fromParams} from '../decorators/location';
import type {LocationPayload} from '../decorators/location';
import {asConversion} from '../decorators/location/convert';
import type {ConversionState} from '../decorators/location/convert';
import {LOCATION_FORMATS} from '../decorators/location/format';
import type {LocationFormat} from '../decorators/location/format';

/**
 * What the server put in the shell.
 *
 * The page is rendered for a link the server has already validated, so these
 * are its own output rather than anything a reader supplied, and the conversion
 * is the one it would have served from /api/v1/convert to a reader who had a
 * session. Handing it over in the document is what lets a public page render
 * rows that need the projection without a public conversion route.
 */
export interface PageData {
    payload: LocationPayload;
    conversion: ConversionState;
    mode: 'location' | 'map';
}

export function readPageData(root: HTMLElement): PageData | null {
    const format = root.dataset.f ?? '';
    if (!LOCATION_FORMATS.includes(format as LocationFormat)) {
        return null;
    }

    const canonical = root.dataset.v ?? '';
    const raw = root.dataset.r ?? canonical;

    return {
        payload: payloadFor(format as LocationFormat, canonical, raw),
        conversion: conversionFrom(root.dataset.conversion),
        mode: root.dataset.mode === 'map' ? 'map' : 'location',
    };
}

/**
 * The payload, through the same parser the sidebar uses.
 *
 * A null from `fromParams` here does not mean the link is bad: the server
 * validated it against the real grammar, and this side keeps only a copy of the
 * canonical shapes. A disagreement between the two is the failure this file
 * cannot afford to inherit, because on a page there is nothing to fall through
 * to. So the shapes are still checked, and a mismatch degrades to a payload
 * with no local coordinate, where every row comes from the conversion the
 * server already worked out.
 */
function payloadFor(format: LocationFormat, canonical: string, raw: string): LocationPayload {
    const params = new URLSearchParams({f: format, v: canonical});
    if (raw !== canonical) {
        params.set('r', raw);
    }

    const parsed = fromParams(params);
    if (parsed) {
        return parsed;
    }

    // eslint-disable-next-line no-console
    console.warn(
        '[tactical-fusion] this build does not recognise a token the server issued; ' +
        'rendering from the server conversion alone',
        {format, canonical},
    );

    return {coord: null, format, canonical, raw};
}

/**
 * The conversion, re-validated rather than trusted.
 *
 * It is the server's own output, so this is not a trust boundary. It is the
 * same check the sidebar runs on the same shape, and it is what stops a
 * non-finite or out-of-range position reaching the map as a pin at NaN.
 *
 * A failure degrades rather than refusing: every row the token yields locally
 * is still true, and the rest reads "unavailable", which is the same thing the
 * sidebar shows a reader whose request did not arrive.
 */
function conversionFrom(encoded: string | undefined): ConversionState {
    if (!encoded) {
        return {status: 'failed', data: null};
    }

    try {
        return {status: 'ready', data: asConversion(JSON.parse(encoded))};
    } catch {
        return {status: 'failed', data: null};
    }
}
