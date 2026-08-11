import type {ZoneSelection} from '../decorators/dtg/zones';

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
 * The whole per-reader blob.
 *
 * Decorators get a key of their own rather than a flat namespace, so a second
 * decorator can add settings without migrating anybody's stored blob.
 */
export interface Preferences {
    dtg: DtgPreferences;
}

/**
 * The blob of a reader who has customised nothing.
 *
 * A module constant rather than a factory: it is the fallback snapshot for
 * `useSyncExternalStore`, which compares snapshots by identity and would
 * re-render forever on a fresh object each time.
 */
export const EMPTY_PREFERENCES: Preferences = {dtg: {zones: [], urgentWithinMinutes: 0}};

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

        entries.push(typeof entry.name === 'string' && entry.name !== '' ?
            {iana: entry.iana, name: entry.name} :
            {iana: entry.iana});
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
    const blob = (typeof raw === 'object' && raw !== null ? raw : {}) as {dtg?: unknown};
    const dtg = (typeof blob.dtg === 'object' && blob.dtg !== null ? blob.dtg : {}) as Record<string, unknown>;

    return {
        dtg: {
            zones: asZones(dtg.zones),
            urgentWithinMinutes: asCount(dtg.urgent_within_minutes),
        },
    };
}

/** Builds the JSON the server expects. The wire names are snake_case. */
export function toWire(preferences: Preferences): unknown {
    return {
        dtg: {
            zones: preferences.dtg.zones,
            urgent_within_minutes: preferences.dtg.urgentWithinMinutes, // eslint-disable-line @typescript-eslint/naming-convention
        },
    };
}
