import {isSectionID} from '../cot/sections';
import type {SectionID} from '../cot/sections';
import type {ZoneSelection} from '../decorators/dtg/zones';
import {isHideableID} from '../decorators/location/rows';
import type {HideableID} from '../decorators/location/rows';
import {isSectionID as isGeoJsonSectionID} from '../geojson/sections';
import type {SectionID as GeoJsonSectionID} from '../geojson/sections';

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
    hiddenRows: HideableID[];
}

/**
 * One reader's view of a Cursor on Target event. Mirrors CotPreferences in
 * server/preferences.go.
 */
export interface CotPreferences {

    /**
     * The groups to leave out of the sidebar panel, by id. Empty means every
     * section.
     *
     * The HIDDEN sections rather than the shown ones, for the reason
     * LocationPreferences.hiddenRows records: empty means "all of them", so a
     * reader who never chose is stored as nothing at all, and a section added
     * in a later version appears for everybody.
     */
    hiddenSections: SectionID[];
}

export interface GeoJsonPreferences {
    hiddenSections: GeoJsonSectionID[];
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
    cot: CotPreferences;
    geojson: GeoJsonPreferences;
}

/**
 * The blob of a reader who has customized nothing.
 *
 * A module constant rather than a factory: it is the fallback snapshot for
 * `useSyncExternalStore`, which compares snapshots by identity and would
 * re-render forever on a fresh object each time.
 */
export const EMPTY_PREFERENCES: Preferences = {
    dtg: {zones: [], urgentWithinMinutes: 0},
    location: {hiddenRows: []},
    cot: {hiddenSections: []},
    geojson: {hiddenSections: []},
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
    const blob = (typeof raw === 'object' && raw !== null ? raw : {}) as {dtg?: unknown; location?: unknown; cot?: unknown; geojson?: unknown};
    const dtg = (typeof blob.dtg === 'object' && blob.dtg !== null ? blob.dtg : {}) as Record<string, unknown>;
    const location = (typeof blob.location === 'object' && blob.location !== null ? blob.location : {}) as Record<string, unknown>;
    const cot = (typeof blob.cot === 'object' && blob.cot !== null ? blob.cot : {}) as Record<string, unknown>;
    const geojson = (typeof blob.geojson === 'object' && blob.geojson !== null ? blob.geojson : {}) as Record<string, unknown>;

    return {
        dtg: {
            zones: asZones(dtg.zones),
            urgentWithinMinutes: asCount(dtg.urgent_within_minutes),
        },
        location: {
            hiddenRows: asRowIDs(location.hidden_rows),
        },
        cot: {
            hiddenSections: asSectionIDs(cot.hidden_sections),
        },
        geojson: {
            hiddenSections: asGeoJsonSectionIDs(geojson.hidden_sections),
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
function asRowIDs(value: unknown): HideableID[] {
    if (!Array.isArray(value)) {
        return [];
    }

    const ids: HideableID[] = [];
    for (const raw of value) {
        if (typeof raw === 'string' && isHideableID(raw) && !ids.includes(raw)) {
            ids.push(raw);
        }
    }

    return ids;
}

/**
 * The section ids in an array, dropping anything this build does not render.
 *
 * Forgiving on the way in and strict on the way out, for the reason asRowIDs is.
 */
function asSectionIDs(value: unknown): SectionID[] {
    if (!Array.isArray(value)) {
        return [];
    }

    const ids: SectionID[] = [];
    for (const raw of value) {
        if (typeof raw === 'string' && isSectionID(raw) && !ids.includes(raw)) {
            ids.push(raw);
        }
    }

    return ids;
}

/**
 * The GeoJSON section ids in an array, dropping anything this build does not
 * render. Forgiving on the way in, for the reason asRowIDs is.
 */
function asGeoJsonSectionIDs(value: unknown): GeoJsonSectionID[] {
    if (!Array.isArray(value)) {
        return [];
    }

    const ids: GeoJsonSectionID[] = [];
    for (const raw of value) {
        if (typeof raw === 'string' && isGeoJsonSectionID(raw) && !ids.includes(raw)) {
            ids.push(raw);
        }
    }

    return ids;
}

/** Builds the JSON the server expects. The wire names are snake_case. */
export function toWire(preferences: Preferences): unknown {
    return {
        dtg: sectionToWire('dtg', preferences.dtg),
        location: sectionToWire('location', preferences.location),
        cot: sectionToWire('cot', preferences.cot),
        geojson: sectionToWire('geojson', preferences.geojson),
    };
}

function sectionToWire<K extends keyof Preferences>(section: K, value: Preferences[K]): unknown {
    if (section === 'dtg') {
        const dtg = value as DtgPreferences;
        return {zones: dtg.zones, urgent_within_minutes: dtg.urgentWithinMinutes};
    }
    if (section === 'location') {
        return {hidden_rows: (value as LocationPreferences).hiddenRows};
    }
    if (section === 'geojson') {
        return {hidden_sections: (value as GeoJsonPreferences).hiddenSections};
    }

    return {hidden_sections: (value as CotPreferences).hiddenSections};
}

/**
 * The blob the server sent, with ONE section replaced.
 *
 * A PUT replaces the whole blob, so a save has to send back the sections it did
 * not touch. Rebuilding them from the PARSED shape silently rewrote them:
 * fromWire drops an id this build does not know, deliberately, so that retiring
 * one cannot lock a reader out. Sending the parsed shape back turned that
 * forgiving read into a destructive write, and a reader on a cached older
 * bundle lost a hidden id by saving an unrelated section. The untouched
 * sections now travel back exactly as they arrived.
 */
export function withSection<K extends keyof Preferences>(
    base: unknown, section: K, value: Preferences[K],
): unknown {
    const blob = typeof base === 'object' && base !== null ? {...base as Record<string, unknown>} : {};
    blob[section] = sectionToWire(section, value);

    return blob;
}

/**
 * Whether a blob the server sent records no choice at all.
 *
 * Read off the WIRE rather than the parsed shape, for the reason withSection
 * exists: a hidden id this build does not know parses away to nothing, and
 * deciding to DELETE on that would throw away the very settings the forgiving
 * read exists to preserve.
 */
export function wireHasNoChoices(base: unknown): boolean {
    if (typeof base !== 'object' || base === null) {
        return true;
    }

    const blob = base as {dtg?: unknown; location?: unknown; cot?: unknown; geojson?: unknown};
    const dtg = record(blob.dtg);
    const minutes = dtg.urgent_within_minutes;

    return listLength(dtg.zones) === 0 &&
        (typeof minutes !== 'number' || minutes === 0) &&
        listLength(record(blob.location).hidden_rows) === 0 &&
        listLength(record(blob.cot).hidden_sections) === 0 &&
        listLength(record(blob.geojson).hidden_sections) === 0;
}

function record(value: unknown): Record<string, unknown> {
    return typeof value === 'object' && value !== null ? value as Record<string, unknown> : {};
}

function listLength(value: unknown): number {
    return Array.isArray(value) ? value.length : 0;
}
