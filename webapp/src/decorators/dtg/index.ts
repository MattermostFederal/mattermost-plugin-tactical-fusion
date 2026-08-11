import {formatOffsetLabel} from './describe';
import DtgHover from './DtgHover';
import DtgPanel from './DtgPanel';
import DtgTitle from './DtgTitle';
import {PANEL_TITLE} from './titles';
import {ZONE_OFFSETS} from './zones';

import type {Decorator} from '../types';

/**
 * A date-time group or an RFC 3339 timestamp, already parsed and resolved by
 * the server.
 *
 * Downstream of here the two are the same thing: an instant, the offset it was
 * written in, and a label for that offset. Only the shape of the URL tells them
 * apart, and only in `fromParams`.
 */
export interface Dtg {

    /** The resolved instant. */
    instant: Date;

    /** Canonical token string, for display. */
    canonical: string;

    /**
     * The offset the token was written in, in minutes from UTC.
     *
     * A military zone letter can only name a whole hour; an RFC 3339 offset can
     * be a half or a quarter of one, which is why this is minutes and not a
     * letter.
     */
    offsetMinutes: number;

    /** How that offset is written: a zone letter, or `Z` / `+05:30`. */
    zoneLabel: string;

    /** Whether month and year were inferred from the post date. */
    assumedMonth: boolean;
    assumedYear: boolean;
}

/** Matches the canonical date-time group grammar the server emits. */
const CANONICAL = /^\d{6}[A-Z]([A-Z]{3}\d{2})?$/;

/**
 * Matches the canonical timestamp the server emits: always seconds, never a
 * fraction, and a zero offset written as Z.
 */
const ISO_CANONICAL = /^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(Z|[+-]\d{2}:\d{2})$/;

/** Real zones run from -12:00 to +14:00. Mirrors maxOffsetMinutes in Go. */
const MAX_OFFSET_MINUTES = 14 * 60;

/**
 * The military zone letters the server accepts.
 *
 * I is skipped in the military alphabet and J is the observer's local time,
 * which cannot resolve to one instant for every reader. Accepting any letter
 * here would let a hand-crafted link be intercepted and rendered with a silent
 * UTC fallback instead of falling through to the server's error page.
 */
const ZONE_LETTERS = new Set([
    'Y', 'X', 'W', 'V', 'U', 'T', 'S', 'R', 'Q', 'P', 'O', 'N',
    'Z',
    'A', 'B', 'C', 'D', 'E', 'F', 'G', 'H', 'K', 'L', 'M',
]);

// Wide enough for any real DTG, narrow enough to reject garbage: 1970 to 2200.
const MIN_INSTANT_MS = 0;
const MAX_INSTANT_MS = 7258118400000;

/**
 * Builds the payload from the link's query params.
 *
 * Params come from a URL a user could have hand-edited, so everything is
 * re-validated. There is no parsing of the DTG grammar here by design: the
 * server did that, and duplicating it in TypeScript would be a permanent
 * source of drift.
 */
export function fromParams(params: URLSearchParams): Dtg | null {
    // Matched before Number(), which is not a validator here: it reads null as
    // 0 and "" as 0, and 0 is inside the accepted range, so a link with no "t"
    // at all would otherwise resolve to the Unix epoch and render a panel
    // counting down to 1970 beside whatever token "dtg" claimed. The Go side
    // rejects both through ParseInt; this keeps the two ends agreeing.
    const raw = params.get('t');
    if (raw === null || !(/^\d+$/).test(raw)) {
        return null;
    }

    const millis = Number(raw);
    if (!Number.isInteger(millis) || millis < MIN_INSTANT_MS || millis > MAX_INSTANT_MS) {
        return null;
    }
    const instant = new Date(millis);

    const canonical = params.get('dtg') ?? '';
    const assumed = params.get('a') ?? '';

    // Exactly one of the two zone parameters. A date-time group says "z", a
    // timestamp says "o", and a link carrying both is not one the server wrote.
    const zone = params.get('z');
    const offset = params.get('o');
    if ((zone === null) === (offset === null)) {
        return null;
    }

    if (offset !== null) {
        const offsetMinutes = Number(offset);
        if (!Number.isInteger(offsetMinutes) ||
            offsetMinutes < -MAX_OFFSET_MINUTES || offsetMinutes > MAX_OFFSET_MINUTES) {
            return null;
        }
        if (!ISO_CANONICAL.test(canonical)) {
            return null;
        }

        // A timestamp carries its own date, so nothing about it was assumed.
        if (assumed !== '') {
            return null;
        }

        return {
            instant,
            canonical,
            offsetMinutes,
            zoneLabel: formatOffsetLabel(offsetMinutes),
            assumedMonth: false,
            assumedYear: false,
        };
    }

    if (!CANONICAL.test(canonical)) {
        return null;
    }
    if (!ZONE_LETTERS.has(zone as string)) {
        return null;
    }
    if (zone !== canonical[6]) {
        return null;
    }
    if (!['', 'y', 'my'].includes(assumed)) {
        return null;
    }

    return {
        instant,
        canonical,
        offsetMinutes: (ZONE_OFFSETS[zone as string] ?? 0) * 60,
        zoneLabel: zone as string,
        assumedMonth: assumed.includes('m'),
        assumedYear: assumed.includes('y'),
    };
}

const decorator: Decorator<Dtg> = {
    type: 'dtg',
    fromParams,

    // The fallback when the header cannot render a component. DtgTitle is what
    // the sidebar actually shows, since the header has to follow the panel into
    // the editor and back.
    summary: () => PANEL_TITLE,
    style: {color: '#3d85c6', background: 'rgba(61, 133, 198, 0.12)'},
    Panel: DtgPanel,
    Title: DtgTitle,
    Hover: DtgHover,
};

export default decorator;
