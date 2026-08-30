import {fromParams} from '../decorators/location';
import type {LocationPayload} from '../decorators/location';
import {asConversion} from '../decorators/location/convert';
import type {ConversionState} from '../decorators/location/convert';
import type {LocationFormat} from '../decorators/location/format';
import {PACKAGE_NAME} from '../decorators/location/map/basemap';
import {ALL_FEATURES} from '../features/types';
import type {Features} from '../features/types';

/**
 * What the server put in the shell.
 *
 * The page is rendered for a link the server has already validated, so these
 * are its own output rather than anything a reader supplied, and the conversion
 * is the one it would have served from /api/v1/convert to a reader who had a
 * session. Handing it over in the document is what lets a public page render
 * rows that need the projection without a public conversion route.
 */
export interface LocationPageData {
    payload: LocationPayload;
    conversion: ConversionState;
    mode: 'location' | 'map';

    /**
     * Which map surfaces the admin has left on.
     *
     * Read out of the shell rather than fetched. A page is handed everything it
     * needs in one document by design, and /api/v1/features needs a session this
     * route does not require of anybody but the reader who already followed the
     * link. One less request, and one less way for a page to disagree with the
     * server that rendered it.
     */
    maps: Features;

    /** The detail areas this install has, from the shell rather than the API. */
    packages: string[];
}

export interface OverlayPageData {
    mode: 'overlay';
    kind: string;
    props: unknown;
    packages: string[];
}

export type PageData = LocationPageData | OverlayPageData;

/**
 * The shell's parameters, or null when there is no shell to read.
 *
 * An unrecognized format id DEGRADES rather than refusing, for the same reason
 * `payloadFor` degrades on a canonical-shape disagreement: on a page there is
 * nothing to fall through to. Refusing rendered a wholly blank document, on the
 * route the mobile app opens, with nothing logged, while the conversion the
 * server had already computed sat unread in the shell three attributes away.
 *
 * That was also the coarser half of a drift the finer half already handled: the
 * webapp keeps its own copy of the format list, so adding a grammar in Go alone
 * blanked every page for it. `TestWebappFormatListMatches` now pins the two
 * lists together, and this degrade is what keeps the failure legible if one is
 * ever added anyway.
 *
 * An EMPTY id is different and still refuses: it means the document was not
 * written by this plugin's shell at all, so there is nothing to render from.
 */
export function readPageData(root: HTMLElement): PageData | null {
    if (root.dataset.mode === 'overlay') {
        return overlayFrom(root);
    }

    const format = root.dataset.f ?? '';
    if (format === '') {
        return null;
    }

    const canonical = root.dataset.v ?? '';
    const raw = root.dataset.r ?? canonical;

    return {
        payload: payloadFor(format as LocationFormat, canonical, raw),
        conversion: conversionFrom(root.dataset.conversion),
        mode: root.dataset.mode === 'map' ? 'map' : 'location',
        maps: mapsFrom(root.dataset.maps),
        packages: packagesFrom(root.dataset.packages),
    };
}

/**
 * The overlay shell, or null when it carries nothing to draw.
 *
 * The blob is NOT validated here. It is handed straight to the card's reader,
 * which is the one place that decides what a props blob means and which already
 * refuses a version it cannot read. A check here would be a second opinion, and
 * the failure it would cause is the blank document `payloadFor` records.
 */
function overlayFrom(root: HTMLElement): OverlayPageData | null {
    const kind = root.dataset.overlayKind ?? '';
    const encoded = root.dataset.overlay ?? '';
    if (kind === '' || encoded === '') {
        return null;
    }

    let props: unknown;
    try {
        props = JSON.parse(encoded);
    } catch {
        return null;
    }

    return {
        mode: 'overlay',
        kind,
        props,
        packages: packagesFrom(root.dataset.packages),
    };
}

/*
 * Which detail areas the shell says this install has.
 *
 * A page has no session and /api/v1/packages requires one, so the list arrives
 * in the document rather than being fetched. Names are filtered against the
 * same pattern the request path is built from: this is the one value on the
 * page that started life as a filename on somebody's server.
 *
 * An absent attribute means NONE, which is the opposite of how `data-maps`
 * reads its own absence and is right for the same reason. There, absence has to
 * mean "an older shell" because failing to draw a map is the silent, costly
 * direction. Here the costly direction is the other way round: a name invented
 * from nothing would be a request for an archive that does not exist, on every
 * page load, and there is no list a bundle could guess.
 */
function packagesFrom(value: string | undefined): string[] {
    if (!value) {
        return [];
    }

    return value.split(',').filter((name) => PACKAGE_NAME.test(name));
}

/**
 * Which map surfaces the shell says are on.
 *
 * A closed set matched against a comma list, so a surface a later server knows
 * about and this bundle does not is ignored rather than drawn. `inline` is never
 * on the wire here: no page has posts in it, and reading one would be reading a
 * claim about a surface that does not exist on this document.
 *
 * An ABSENT attribute means every surface, not none. A missing attribute is a
 * shell this bundle cannot interpret rather than an admin decision, and the two
 * have opposite right answers: the failure to draw a map somebody is paying for
 * is silent, where drawing one is at worst a wasted fetch on a page the reader
 * is already looking at. The server writes the attribute unconditionally,
 * including empty, so "off" is always said out loud.
 */
function mapsFrom(value: string | undefined): Features {
    if (value === undefined) {
        return ALL_FEATURES;
    }

    const on = new Set(value.split(','));

    return {
        mapPanel: on.has('panel'),
        mapPage: on.has('page'),
        mapInline: false,
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
        '[tactical-fusion] this build does not recognize a token the server issued; ' +
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
