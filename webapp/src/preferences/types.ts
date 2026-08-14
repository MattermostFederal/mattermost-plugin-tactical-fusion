import type {ZoneSelection} from '../decorators/dtg/zones';
import {isRowID} from '../decorators/location/rows';
import type {RowID} from '../decorators/location/rows';

/**
 * One reader's view of a date-time group. Mirrors DTGPreferences in
 * server/preferences.go.
 *
 * Every field is optional in spirit: an empty list and a zero threshold both
 * mean "use the default". That is what lets "Restore defaults" delete the blob
 * rather than write today's defaults into it, so a reader who never chose keeps
 * tracking whatever the defaults become.
 */
export interface DtgPreferences {

    /**
     * The chosen zones, each optionally carrying the base name it was picked
     * by. Empty means the built-in table.
     */
    zones: ZoneSelection[];

    /** Minutes before the countdown flashes. Zero means the built-in threshold. */
    urgentWithinMinutes: number;
}

/**
 * One reader's view of a coordinate. Mirrors LocationPreferences in
 * server/preferences.go.
 */
export interface LocationPreferences {

    /**
     * The rows to leave out of the panel, by id. Empty means every row.
     *
     * The HIDDEN rows rather than the shown ones, and that direction is the
     * whole design: empty means "all of them", so a reader who never chose is
     * stored as nothing at all, and a row added in a later version appears for
     * everybody rather than being invisible to exactly the readers who cared
     * enough to choose.
     */
    hiddenRows: RowID[];
}

/**
 * The whole per-reader blob.
 *
 * Decorators get a key of their own rather than a flat namespace, so a second
 * decorator can add settings without migrating anybody's stored blob.
 */
export interface Preferences {
    dtg: DtgPreferences;
    location: LocationPreferences;
}

/**
 * The blob of a reader who has customised nothing.
 *
 * A module constant rather than a factory: it is the fallback snapshot for
 * `useSyncExternalStore`, which compares snapshots by identity and would
 * re-render forever on a fresh object each time.
 */
export const EMPTY_PREFERENCES: Preferences = {
    dtg: {zones: [], urgentWithinMinutes: 0},
    location: {hiddenRows: []},
};

/** A finite, non-negative integer, or 0 for anything else. */
function asCount(value: unknown): number {
    return typeof value === 'number' && Number.isInteger(value) && value > 0 ? value : 0;
}

/**
 * The selection entries in an array, dropping anything unusable.
 *
 * A bare string is accepted as well as an object, because that is what blobs
 * written before base names existed hold. They read as unnamed zones, which is
 * exactly what they were.
 */
function asZones(value: unknown): ZoneSelection[] {
    if (!Array.isArray(value)) {
        return [];
    }

    const entries: ZoneSelection[] = [];
    for (const raw of value) {
        if (typeof raw === 'string' && raw !== '') {
            entries.push({iana: raw});
            continue;
        }

        const entry = raw as {iana?: unknown; name?: unknown} | null;
        if (typeof entry?.iana !== 'string' || entry.iana === '') {
            continue;
        }

        entries.push(typeof entry.name === 'string' && entry.name !== '' ? {iana: entry.iana, name: entry.name} : {iana: entry.iana});
    }

    return entries;
}

/**
 * Reads the server's JSON.
 *
 * Defensive rather than trusting: this is the one place a shape from the
 * network becomes a typed value, and a malformed field must degrade to the
 * default instead of reaching `Intl` or a style calculation.
 */
export function fromWire(raw: unknown): Preferences {
    const blob = (typeof raw === 'object' && raw !== null ? raw : {}) as {dtg?: unknown; location?: unknown};
    const dtg = (typeof blob.dtg === 'object' && blob.dtg !== null ? blob.dtg : {}) as Record<string, unknown>;
    const location = (typeof blob.location === 'object' && blob.location !== null ? blob.location : {}) as Record<string, unknown>;

    return {
        dtg: {
            zones: asZones(dtg.zones),
            urgentWithinMinutes: asCount(dtg.urgent_within_minutes),
        },
        location: {
            hiddenRows: asRowIDs(location.hidden_rows),
        },
    };
}

/**
 * The row ids in an array, dropping anything this build does not render.
 *
 * Forgiving on the way in and strict on the way out, matching the server: a
 * stored id from a build that had a row this one does not simply hides nothing,
 * where refusing the blob would lock a reader out of their own settings over a
 * row that no longer exists.
 */
function asRowIDs(value: unknown): RowID[] {
    if (!Array.isArray(value)) {
        return [];
    }

    const ids: RowID[] = [];
    for (const raw of value) {
        if (typeof raw === 'string' && isRowID(raw) && !ids.includes(raw)) {
            ids.push(raw);
        }
    }

    return ids;
}

/** Builds the JSON the server expects. The wire names are snake_case. */
export function toWire(preferences: Preferences): unknown {
    return {
        dtg: {
            zones: preferences.dtg.zones,
            urgent_within_minutes: preferences.dtg.urgentWithinMinutes,
        },
        location: {
            hidden_rows: preferences.location.hiddenRows,
        },
    };
}
