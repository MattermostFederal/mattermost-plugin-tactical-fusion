export const COT_POST_TYPE = 'custom_tf_cot';

export const COT_PROPS_KEY = 'tactical_fusion_cot';

/**
 * What the sidebar keys this panel on.
 *
 * Deliberately not a decorator type: there is no `/decorate/cot` route and no
 * link carrying this, so nothing parses it out of an href. It only ever reaches
 * the selection store from the card's own button.
 */
export const COT_PANEL_TYPE = 'cot';

/**
 * The shape the server writes today: a blob holding an `events` array.
 *
 * Version 1 held a single `event`, and posts stamped then are still out there
 * and still render, which is why `readEvents` reads both. A version NEITHER of
 * them knows is refused, and the card falls back to the post's own text rather
 * than guessing at a shape it has never seen.
 */
export const COT_PROPS_VERSION = 2;

const READABLE_VERSIONS = [1, 2];

export interface CotEvent {
    uid: string;
    callsign: string;
    cotType: string;
    typeLabel: string;
    affiliation: string;
    how: string;
    howLabel: string;
    time: string;
    timeQuery: string;
    start: string;
    startQuery: string;
    stale: string;
    staleQuery: string;
    staleAt: string;
    timeAt: string;
    format: string;
    value: string;
    lat: string;
    lon: string;
    positionNote: string;
    hae: string;
    ce: string;
    ceMeters: string;
    le: string;
    speed: string;
    course: string;
    group: string;
    role: string;
    remarks: string;
    parent: string;
    related: string;
}

export interface CotPayload {
    source: string;
    lead: string;
    trail: string;
    src: string;
    fileId: string;
    fileName: string;
    events: CotEvent[];
}

export const SOURCE_FENCE = 'fence';
export const SOURCE_FILE = 'file';

function text(blob: Record<string, unknown>, key: string): string {
    const value = Object.hasOwn(blob, key) ? blob[key] : undefined;
    return typeof value === 'string' ? value : '';
}

function record(value: unknown): Record<string, unknown> | null {
    if (typeof value !== 'object' || value === null || Array.isArray(value)) {
        return null;
    }
    return value as Record<string, unknown>;
}

export function fromProps(props: unknown): CotPayload | null {
    const outer = record(props);
    if (outer === null) {
        return null;
    }

    const blob = record(outer[COT_PROPS_KEY]);
    if (blob === null || !READABLE_VERSIONS.includes(Number(blob.version))) {
        return null;
    }

    const source = text(blob, 'source');
    if (source !== SOURCE_FENCE && source !== SOURCE_FILE) {
        return null;
    }

    const events = readEvents(blob);
    if (events === null) {
        return null;
    }

    return {
        source,
        lead: text(blob, 'lead'),
        trail: text(blob, 'trail'),
        src: text(blob, 'src'),
        fileId: text(blob, 'file_id'),
        fileName: text(blob, 'file_name'),
        events,
    };
}

/**
 * The events, from either shape the server has written.
 *
 * Null rather than an empty array for a blob carrying none, since a stamped
 * post with nothing in it is one the card cannot honour and should hand back to
 * the post's own text.
 */
function readEvents(blob: Record<string, unknown>): CotEvent[] | null {
    const raw = Array.isArray(blob.events) ? blob.events : [blob.event];

    const events: CotEvent[] = [];
    for (const entry of raw) {
        const event = record(entry);
        if (event === null) {
            return null;
        }

        const read = readEvent(event);
        if (read === null) {
            return null;
        }
        events.push(read);
    }

    return events.length === 0 ? null : events;
}

function readEvent(event: Record<string, unknown>): CotEvent | null {
    const uid = text(event, 'uid');
    if (uid === '') {
        return null;
    }

    return {
        uid,
            callsign: text(event, 'callsign'),
            cotType: text(event, 'cot_type'),
            typeLabel: text(event, 'type_label'),
            affiliation: text(event, 'affiliation'),
            how: text(event, 'how'),
            howLabel: text(event, 'how_label'),
            time: text(event, 'time'),
            timeQuery: text(event, 'time_q'),
            start: text(event, 'start'),
            startQuery: text(event, 'start_q'),
            stale: text(event, 'stale'),
            staleQuery: text(event, 'stale_q'),
            staleAt: text(event, 'stale_at'),
            timeAt: text(event, 'time_at'),
            format: text(event, 'format'),
            value: text(event, 'value'),
            lat: text(event, 'lat'),
            lon: text(event, 'lon'),
            positionNote: text(event, 'position_note'),
            hae: text(event, 'hae'),
            ce: text(event, 'ce'),
            ceMeters: text(event, 'ce_meters'),
            le: text(event, 'le'),
            speed: text(event, 'speed'),
            course: text(event, 'course'),
            group: text(event, 'group'),
            role: text(event, 'role'),
        remarks: text(event, 'remarks'),
        parent: text(event, 'parent'),
        related: text(event, 'related'),
    };
}

/**
 * What colour an affiliation is drawn in, or undefined for one this build does
 * not colour.
 *
 * Read by the dot beside the callsign AND by the map marker, so the two cannot
 * disagree about what a track is. Colour is never the only channel: the type
 * label always begins with the affiliation word wherever this returns a colour,
 * and the map states it in its accessible label.
 */
export const AFFILIATION_COLORS: Record<string, string> = {
    friend: '#3d85c6',
    'assumed-friend': '#3d85c6',
    hostile: '#c0392b',
    suspect: '#c0392b',
    neutral: '#3c8f3c',
    unknown: '#8a6d00',
    pending: '#8a6d00',
};

/**
 * What to CALL an affiliation, for the surfaces that cannot use its colour.
 *
 * Every affiliation the SERVER can decode, which is a wider set than
 * AFFILIATION_COLORS: this table names all eleven, and only some of them earn a
 * colour. `TestWebappAffiliationWordsMatch` holds it to the Go table.
 *
 * The wider set is the point. The four the colours leave out (joker, faker,
 * none, other) were falling through to `unstated`, so an event whose
 * affiliation this build was holding in a string was described as though
 * nothing were known about it, on the one surface where the word is the whole
 * channel. Saying "unstated" about a value we have is the ignorance the card
 * refuses to claim everywhere else.
 */
const AFFILIATION_WORDS: Record<string, string> = {
    friend: 'friendly',
    'assumed-friend': 'assumed friendly',
    hostile: 'hostile',
    suspect: 'suspect',
    neutral: 'neutral',
    unknown: 'unknown',
    pending: 'pending',
    joker: 'joker',
    faker: 'faker',
    none: 'unaffiliated',
    other: 'other',
};

/** The affiliation as a word, or 'unstated' when this build does not name it. */
export function affiliationWord(event: CotEvent): string {
    return Object.hasOwn(AFFILIATION_WORDS, event.affiliation) ? AFFILIATION_WORDS[event.affiliation] : 'unstated';
}

export function affiliationColor(event: CotEvent): string | undefined {
    return Object.hasOwn(AFFILIATION_COLORS, event.affiliation) ? AFFILIATION_COLORS[event.affiliation] : undefined;
}

export function isLinkable(event: CotEvent): boolean {
    return event.format !== '' && event.value !== '';
}

export function accuracyMeters(event: CotEvent): number | undefined {
    if (event.ceMeters === '') {
        return undefined;
    }

    const meters = Number(event.ceMeters);
    if (!Number.isFinite(meters) || meters <= 0) {
        return undefined;
    }

    return meters;
}

/**
 * How long the event says it is good for, from its own two timestamps.
 *
 * Both come from the event, so this reads the same on every machine.
 */
export function validFor(event: CotEvent): string {
    return spanBetween(event.timeAt, event.staleAt);
}

/**
 * How long after the post was written the event went stale.
 *
 * Both halves are server-side values, so the answer is the same on every
 * machine. Nothing here reads the reader's clock: a workstation twenty minutes
 * out would otherwise report a live track as expired.
 */
export function staleAfterPosting(event: CotEvent, createAt: number): string {
    const staleAt = Number(event.staleAt);
    if (!Number.isFinite(staleAt) || staleAt <= 0 || !Number.isFinite(createAt) || createAt <= 0) {
        return '';
    }

    const seconds = Math.round((staleAt - createAt) / 1000);
    if (seconds <= 0) {
        return 'already stale when it was posted';
    }

    return `stale ${compactDuration(seconds)} after posting`;
}

function spanBetween(fromMillis: string, toMillis: string): string {
    const from = Number(fromMillis);
    const to = Number(toMillis);
    if (!Number.isFinite(from) || !Number.isFinite(to) || from <= 0 || to <= from) {
        return '';
    }

    return compactDuration(Math.round((to - from) / 1000));
}

function compactDuration(seconds: number): string {
    const days = Math.floor(seconds / 86400);
    const hours = Math.floor((seconds % 86400) / 3600);
    const minutes = Math.floor((seconds % 3600) / 60);
    const remainder = seconds % 60;

    const parts: string[] = [];
    if (days > 0) {
        parts.push(`${days}d`);
    }
    if (hours > 0) {
        parts.push(`${hours}h`);
    }
    if (minutes > 0) {
        parts.push(`${minutes}m`);
    }
    if (remainder > 0 && days === 0 && hours === 0) {
        parts.push(`${remainder}s`);
    }

    return parts.join(' ');
}
