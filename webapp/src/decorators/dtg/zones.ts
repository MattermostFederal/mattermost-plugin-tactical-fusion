/**
 * Military zone letters and their whole-hour offsets from UTC.
 *
 * Used only to interpret the DTG's own zone. Table rows always use IANA zones
 * so DST is handled properly.
 *
 * There is no I: the letter is skipped because it reads as a 1. J, the
 * observer's own local time, is rejected by the server and never reaches here.
 * Mirrors zoneOffsets in server/decorators/dtg/zones.go.
 */
export const ZONE_OFFSETS: Record<string, number> = {
    Y: -12,
    X: -11,
    W: -10,
    V: -9,
    U: -8,
    T: -7,
    S: -6,
    R: -5,
    Q: -4,
    P: -3,
    O: -2,
    N: -1,
    Z: 0,
    A: 1,
    B: 2,
    C: 3,
    D: 4,
    E: 5,
    F: 6,
    G: 7,
    H: 8,
    K: 10,
    L: 11,
    M: 12,
};

export interface DisplayZone {
    name: string;

    /**
     * IANA identifier rather than a fixed offset, so Intl handles DST.
     */
    iana: string;

    abbr: string;
}

/**
 * The timezone table shared with the server-rendered page.
 *
 * Keep this in sync with server/decorators/dtg/zones.go. A Go test reads this
 * file and fails if the IANA identifiers diverge, because a mismatch would mean
 * the RHS panel and the standalone page disagree about the same DTG.
 */
export const DISPLAY_ZONES: DisplayZone[] = [
    {name: 'Zulu (UTC)', iana: 'UTC', abbr: 'Z'},
    {name: 'Washington, DC', iana: 'America/New_York', abbr: 'ET'},
    {name: 'Colorado Springs', iana: 'America/Denver', abbr: 'MT'},
    {name: 'San Diego', iana: 'America/Los_Angeles', abbr: 'PT'},
    {name: 'Honolulu', iana: 'Pacific/Honolulu', abbr: 'HST'},
    {name: 'Ramstein', iana: 'Europe/Berlin', abbr: 'CET/CEST'},
    {name: 'Al Udeid', iana: 'Asia/Qatar', abbr: 'AST'},
    {name: 'Yokota', iana: 'Asia/Tokyo', abbr: 'JST'},
    {name: 'Andersen, Guam', iana: 'Pacific/Guam', abbr: 'ChST'},
];

/**
 * One entry in a reader's selection.
 *
 * The name travels with the zone rather than being derived from it, because
 * several bases share a zone and somebody at Stuttgart wants to see
 * "Stuttgart", not the Ramstein row that happens to keep the same clock. A
 * plain zone carries no name and is described from its identifier.
 *
 * The name is a snapshot taken when the reader chose, so a base renamed later
 * keeps its old label until they pick it again. That is the price of not
 * needing a base registry on the server as well as here.
 */
export interface ZoneSelection {
    iana: string;
    name?: string;
}

/** The defaults, as the entries a saved blob would hold. */
export const DEFAULT_SELECTION: ZoneSelection[] = DISPLAY_ZONES.map(
    (zone) => ({iana: zone.iana, name: zone.name}),
);

/** The default table as bare identifiers. */
export const DEFAULT_ZONE_IDS: string[] = DISPLAY_ZONES.map((zone) => zone.iana);

/**
 * Standard installations, offered by name in the picker.
 *
 * **Several bases may share a zone.** Two such rows read identically to the
 * minute, which is the accepted cost of letting each be named. The nine
 * defaults are in here too, so this is the whole named catalog rather than a
 * supplement to one.
 *
 * Names, not a claim about posture: several of these are rotational. Adding or
 * retiring one is a single line, and nothing else changes.
 */
export const MILITARY_BASES: ZoneSelection[] = [
    {name: 'Zulu (UTC)', iana: 'UTC'},

    {name: 'Pituffik Space Base', iana: 'America/Thule'},
    {name: 'JB Elmendorf-Richardson', iana: 'America/Anchorage'},
    {name: 'Honolulu', iana: 'Pacific/Honolulu'},
    {name: 'JB Pearl Harbor-Hickam', iana: 'Pacific/Honolulu'},

    {name: 'Travis AFB', iana: 'America/Los_Angeles'},
    {name: 'JB Lewis-McChord', iana: 'America/Los_Angeles'},
    {name: 'San Diego', iana: 'America/Los_Angeles'},
    {name: 'Nellis AFB', iana: 'America/Los_Angeles'},

    {name: 'Colorado Springs', iana: 'America/Denver'},
    {name: 'Buckley SFB', iana: 'America/Denver'},

    {name: 'Scott AFB', iana: 'America/Chicago'},
    {name: 'Offutt AFB', iana: 'America/Chicago'},
    {name: 'Fort Cavazos', iana: 'America/Chicago'},
    {name: 'Barksdale AFB', iana: 'America/Chicago'},

    {name: 'Washington, DC', iana: 'America/New_York'},
    {name: 'Fort Liberty', iana: 'America/New_York'},
    {name: 'Naval Station Norfolk', iana: 'America/New_York'},
    {name: 'MacDill AFB', iana: 'America/New_York'},

    {name: 'Soto Cano AB', iana: 'America/Tegucigalpa'},

    {name: 'Lajes Field', iana: 'Atlantic/Azores'},
    {name: 'Keflavik', iana: 'Atlantic/Reykjavik'},
    {name: 'RAF Lakenheath', iana: 'Europe/London'},
    {name: 'RAF Mildenhall', iana: 'Europe/London'},
    {name: 'SHAPE, Mons', iana: 'Europe/Brussels'},

    {name: 'Ramstein', iana: 'Europe/Berlin'},
    {name: 'USAG Stuttgart', iana: 'Europe/Berlin'},
    {name: 'Spangdahlem AB', iana: 'Europe/Berlin'},

    {name: 'Aviano AB', iana: 'Europe/Rome'},
    {name: 'NAS Sigonella', iana: 'Europe/Rome'},
    {name: 'Naval Station Rota', iana: 'Europe/Madrid'},
    {name: 'Redzikowo AB', iana: 'Europe/Warsaw'},
    {name: 'Souda Bay', iana: 'Europe/Athens'},
    {name: 'Deveselu AB', iana: 'Europe/Bucharest'},
    {name: 'Incirlik AB', iana: 'Europe/Istanbul'},

    {name: 'Camp Lemonnier', iana: 'Africa/Djibouti'},
    {name: 'Al Udeid', iana: 'Asia/Qatar'},
    {name: 'NSA Bahrain', iana: 'Asia/Bahrain'},
    {name: 'Ali Al Salem AB', iana: 'Asia/Kuwait'},
    {name: 'Camp Arifjan', iana: 'Asia/Kuwait'},
    {name: 'Prince Sultan AB', iana: 'Asia/Riyadh'},
    {name: 'Al Asad AB', iana: 'Asia/Baghdad'},
    {name: 'Al Dhafra AB', iana: 'Asia/Dubai'},
    {name: 'Diego Garcia', iana: 'Indian/Chagos'},

    {name: 'Singapore', iana: 'Asia/Singapore'},
    {name: 'Camp Humphreys', iana: 'Asia/Seoul'},
    {name: 'Osan AB', iana: 'Asia/Seoul'},
    {name: 'Yokota', iana: 'Asia/Tokyo'},
    {name: 'Kadena AB', iana: 'Asia/Tokyo'},
    {name: 'Yokosuka Naval Base', iana: 'Asia/Tokyo'},
    {name: 'RAAF Darwin', iana: 'Australia/Darwin'},
    {name: 'Andersen, Guam', iana: 'Pacific/Guam'},
    {name: 'Kwajalein Atoll', iana: 'Pacific/Kwajalein'},
    {name: 'Wake Island', iana: 'Pacific/Wake'},
];

/**
 * Identity for a selection entry.
 *
 * Two bases in one zone are two different rows, so the zone alone will not do:
 * this is what the picker keys its options on, what a removal matches, and what
 * duplicate detection compares.
 */
export function zoneKey(entry: ZoneSelection): string {
    return `${entry.iana}|${entry.name ?? ''}`;
}

// Only the nine curated rows are looked up by zone, and only for their
// hand-written abbreviations. Names are never inferred from a zone: they reach a
// row by being stored with it.
const BY_IANA = new Map(DISPLAY_ZONES.map((zone) => [zone.iana, zone]));

/**
 * A readable name for a zone with no entry in the table above.
 *
 * The last path segment, which is the city: `Europe/Paris` reads as `Paris` and
 * `America/Argentina/Buenos_Aires` as `Buenos Aires`. Better than the full
 * identifier in a narrow sidebar, and the identifier is still shown underneath
 * in the editor for anyone who needs to be sure which one they picked.
 */
function humanize(iana: string): string {
    const segments = iana.split('/');
    return (segments[segments.length - 1] ?? iana).replace(/_/g, ' ');
}

/**
 * The short zone name at a given instant, e.g. `PDT` or `GMT+4`.
 *
 * Computed per instant rather than stored, because the answer changes with
 * daylight saving. The built-in rows keep their hand-written labels instead,
 * which is why those are season-neutral: `San Diego` reads `PT` in either
 * season rather than claiming `PST` beside a clock showing PDT.
 */
function shortName(iana: string, instant: Date): string {
    try {
        const parts = new Intl.DateTimeFormat('en-US', {
            timeZone: iana,
            timeZoneName: 'short',
        }).formatToParts(instant);
        return parts.find((part) => part.type === 'timeZoneName')?.value ?? '';
    } catch {
        return '';
    }
}

/**
 * The row to render for a zone identifier.
 *
 * A built-in zone keeps its hand-written name and abbreviation. Anything the
 * reader added themselves is described from the identifier and from Intl, so
 * the picker is not limited to the nine locations this plugin happens to ship.
 */
export function displayZoneFor(entry: ZoneSelection, instant: Date): DisplayZone {
    const curated = BY_IANA.get(entry.iana);

    return {

        // No fall back to the curated name. A reader who picked the bare
        // Europe/Berlin out of "All timezones" chose the zone, not the base, and
        // labeling it "Ramstein" would put a second identical-looking row
        // beside the Ramstein they may already have.
        name: entry.name ?? humanize(entry.iana),

        iana: entry.iana,

        // Only the nine curated rows have hand-written abbreviations. Everything
        // else measures one, since the right answer moves with the season.
        abbr: curated?.abbr ?? shortName(entry.iana, instant),
    };
}

/**
 * The options that pin down a zone's wall clock to the second.
 *
 * `hourCycle: 'h23'` rather than `hour12: false`, because en-US reads the
 * latter as h24 and renders midnight as hour 24, which would put the offset a
 * day out for every zone once a day.
 */
const WALL_CLOCK_PARTS: Intl.DateTimeFormatOptions = {
    hourCycle: 'h23',
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit',
};

/**
 * A zone's offset from UTC at a given instant, in minutes.
 *
 * Measured rather than looked up, by asking Intl for the zone's wall clock and
 * subtracting. A table of offsets would be wrong twice a year in every zone
 * that observes daylight saving, and wrong permanently in any zone that changes
 * its rules.
 *
 * Null when the zone cannot be resolved, which a saved blob can outlive its
 * browser to produce.
 */
export function zoneOffsetMinutes(iana: string, instant: Date): number | null {
    let parts: Intl.DateTimeFormatPart[];
    try {
        parts = new Intl.DateTimeFormat('en-US', {...WALL_CLOCK_PARTS, timeZone: iana}).
            formatToParts(instant);
    } catch {
        return null;
    }

    const value = (type: string) => Number(parts.find((part) => part.type === type)?.value);
    const wall = Date.UTC(
        value('year'), value('month') - 1, value('day'),
        value('hour'), value('minute'), value('second'),
    );
    if (Number.isNaN(wall)) {
        return null;
    }

    // The parts carry no milliseconds, so the instant is truncated to the same
    // resolution before subtracting. Otherwise every offset would be off by the
    // millisecond component of whatever instant it was measured at.
    const truncated = Math.floor(instant.getTime() / 1000) * 1000;

    return Math.round((wall - truncated) / 60000);
}

/** An offset as `UTC+05:30`. */
export function formatZoneOffset(minutes: number): string {
    const sign = minutes < 0 ? '-' : '+';
    const absolute = Math.abs(minutes);
    const hours = String(Math.floor(absolute / 60)).padStart(2, '0');
    const rest = String(absolute % 60).padStart(2, '0');

    return `UTC${sign}${hours}:${rest}`;
}

/** A zone together with where it sits relative to UTC right now. */
export interface ZoneChoice extends DisplayZone {

    /** Identity, which the zone alone cannot provide once bases share one. */
    key: string;

    /** Minutes from UTC at the instant this was built for, or null if unknown. */
    offsetMinutes: number | null;

    /** The offset as `UTC+05:30`, or an empty string if unknown. */
    offsetLabel: string;
}

function toChoice(entry: ZoneSelection, instant: Date): ZoneChoice {
    const offsetMinutes = zoneOffsetMinutes(entry.iana, instant);

    return {
        ...displayZoneFor(entry, instant),
        key: zoneKey(entry),
        offsetMinutes,
        offsetLabel: offsetMinutes === null ? '' : formatZoneOffset(offsetMinutes),
    };
}

/**
 * Orders zones west to east, the way a world clock reads.
 *
 * Ties break on the name and then the identifier, so the order is total: two
 * zones sharing an offset must not swap places between renders.
 *
 * A zone whose offset could not be measured sorts last rather than as UTC,
 * which is what treating null as zero would silently do.
 */
function byOffset(a: ZoneChoice, b: ZoneChoice): number {
    if (a.offsetMinutes !== b.offsetMinutes) {
        if (a.offsetMinutes === null) {
            return 1;
        }
        if (b.offsetMinutes === null) {
            return -1;
        }
        return a.offsetMinutes - b.offsetMinutes;
    }

    return a.name.localeCompare(b.name) || a.iana.localeCompare(b.iana);
}

/** The rows for a selection, ordered west to east. */
export function orderedZones(entries: ZoneSelection[], instant: Date): ZoneChoice[] {
    return entries.map((entry) => toChoice(entry, instant)).sort(byOffset);
}

/**
 * Every timezone the reader can choose from, ordered west to east.
 *
 * `Intl.supportedValuesOf` is the browser's own IANA list, which is the only
 * honest source: offering a zone this browser cannot format would produce a row
 * of blanks. Where it is missing the picker falls back to the shortcut list, so
 * the editor still works rather than showing nothing at all.
 *
 * This measures a few hundred offsets, so build it once per instant and only
 * when the reader has actually opened the editor.
 */
export function availableZones(instant: Date): ZoneChoice[] {
    const supported = Intl as {supportedValuesOf?: (key: string) => string[]};

    let ids: string[];
    try {
        ids = supported.supportedValuesOf?.('timeZone') ?? [];
    } catch {
        ids = [];
    }

    if (ids.length === 0) {
        return orderedZones(MILITARY_BASES.map((base) => ({iana: base.iana})), instant);
    }

    // Union with the bases' zones: engines canonicalize, so several are
    // backward links their supported list leaves out (Asia/Bahrain links to
    // Asia/Qatar, Asia/Kuwait to Asia/Riyadh). Both still format correctly, and
    // a zone the picker could not offer would be a zone nobody could choose.
    const all = new Set([...ids, ...MILITARY_BASES.map((base) => base.iana)]);

    return orderedZones([...all].map((iana) => ({iana})), instant);
}

/** One labeled block of options in the picker. */
export interface ZoneGroup {
    label: string;
    zones: ZoneChoice[];
}

/**
 * The picker, split into the named bases and everything else.
 *
 * Several hundred identifiers with no grouping would bury the bases this plugin
 * exists to serve, and a native select prefixed with offsets has no useful
 * typeahead to find them with. A base's zone appears in the second group too,
 * unnamed: the second group is the complete list, and pruning it would make
 * "all timezones" a lie.
 */
export function availableZoneGroups(instant: Date): ZoneGroup[] {
    return [
        {label: 'Bases and common zones', zones: orderedZones(MILITARY_BASES, instant)},
        {label: 'All timezones', zones: availableZones(instant)},
    ];
}
