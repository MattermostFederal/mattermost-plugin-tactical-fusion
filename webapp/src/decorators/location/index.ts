import {isCanonical, isGridFormat, parseCanonical, LOCATION_FORMATS} from './format';
import type {Coordinate, LocationFormat} from './format';
import LocationPanel from './LocationPanel';
import LocationTitle, {PANEL_TITLE} from './LocationTitle';

import type {Decorator} from '../types';

/**
 * A coordinate, already matched, validated and parsed by the server.
 *
 * The identity of a location is the pair (format, canonical token). Latitude,
 * longitude and every alternative notation are derived from those two, so
 * nothing carried here can disagree with the token it came from.
 */
export interface LocationPayload {

    /**
     * The reading, sliced out of the canonical token.
     *
     * Null for the two grid formats, whose position is a projection rather than
     * a division and therefore arrives from the conversion endpoint. Null is
     * not a failure here and the panel must not render it as one: the token is
     * still valid, and most of the panel is still computable without it.
     */
    coord: Coordinate | null;

    /** Which grammar the server matched. */
    format: LocationFormat;

    /** The canonical token, which is what the link was validated against. */
    canonical: string;

    /**
     * The author's own text for the same token.
     *
     * Falls back to the canonical form when the link carries no `r`, which is
     * the common case for the USMTF compact family: the author already typed
     * the canonical form. An absent `r` is not a missing value.
     */
    raw: string;
}

/** Mirrors maxRawBytes in the Go package. */
const MAX_RAW_LENGTH = 64;

/**
 * The characters `r` may contain.
 *
 * A whitelist, never a blacklist: digits, letters, the separators the grammars
 * use, and the four typographic variants the server accepts. No `<`, no `&`, no
 * control characters, no other non-ASCII.
 *
 * The whole alphabet rather than only the hemisphere letters, matching
 * `allowedRawRunes` in Go: a grid reference carries a band letter and two
 * 100 km square letters drawn from most of it. The tight checks on `r` are the
 * server's grammar match and canonical round trip, neither of which can be done
 * here; this is the coarse gate, and it is the same coarse gate on both sides.
 */
const RAW_ALLOWED = /^[0-9A-Za-z.,\-+'" °º′’´″”]*$/;

/**
 * Builds the payload from the link's query params.
 *
 * Params come from a URL a user could have hand-edited, so everything is
 * re-validated. There is no coordinate grammar here by design: the server did
 * that, and duplicating it in TypeScript would be a permanent source of drift.
 *
 * What this can check is shape. It cannot re-derive the position from the token
 * without the grammar, so a link whose `v` is well-formed but was never issued
 * by this plugin renders here while the server-rendered page rejects it. That
 * limit is real and is why the page, not the panel, is the round-trip-validated
 * surface.
 */
export function fromParams(params: URLSearchParams): LocationPayload | null {
    const name = params.get('f');
    if (!name || !LOCATION_FORMATS.includes(name as LocationFormat)) {
        return null;
    }
    const format = name as LocationFormat;

    const canonical = params.get('v') ?? '';
    if (!isCanonical(format, canonical)) {
        return null;
    }

    // Null for a grid format, which is expected rather than a rejection: the
    // shape check above is what vouched for the token, and a projection is not
    // something this side of the plugin does.
    const coord = parseCanonical(format, canonical);
    if (!coord && !isGridFormat(format)) {
        return null;
    }

    // An empty r reads as absent, not as "the author wrote nothing". The
    // server does the same; treating '' as present left the panel showing a
    // blank "Original text" row beside a spurious "Normalized" one.
    const raw = params.get('r');
    if (raw === null || raw === '') {
        return {coord, format, canonical, raw: canonical};
    }

    // Gates 1 and 2 of the server's four. Gates 3 and 4 need the grammar and
    // are the server's alone; a length-capped, alphabet-restricted string
    // rendered as a text node is safe to display without them.
    if (raw.length > MAX_RAW_LENGTH || !RAW_ALLOWED.test(raw)) {
        return null;
    }

    return {coord, format, canonical, raw};
}

const decorator: Decorator<LocationPayload> = {
    type: 'location',
    fromParams,

    // The fallback when the header cannot render a component. LocationTitle is
    // what the sidebar normally shows, and it follows the panel into the editor.
    summary: () => PANEL_TITLE,

    // Teal, matching the coordinate color in mattermost-plugin-aocanywhere, so
    // the two plugins agree about what a coordinate looks like in a channel.
    style: {color: '#388ea6', background: 'rgba(56, 148, 166, 0.12)'},

    Panel: LocationPanel,
    Title: LocationTitle,

    // Still no Hover, and the reason has changed rather than gone away.
    //
    // The thing worth a glance is the grid reference, and the projection for it
    // now exists, but only on the server. A hover fires on pointer movement
    // rather than on a click, so wiring one to the conversion endpoint would
    // put a request behind every link a reader's cursor crosses in a busy
    // channel. That wants the shared module cache `preferences/store.ts` has,
    // which is a piece of work in its own right and not one this phase claimed.
    // A hover that merely restated the token under the cursor would fail the
    // bar the framework sets, so there is none until it can show the grid.
};

export default decorator;
