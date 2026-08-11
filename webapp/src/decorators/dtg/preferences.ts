import {DEFAULT_URGENT_WITHIN_MS} from './relative';
import type {ZoneChoice, ZoneSelection} from './zones';
import {DEFAULT_SELECTION, orderedZones, zoneKey} from './zones';

import type {DtgPreferences} from '../../preferences/types';

/**
 * Turns a saved selection into the rows to render, ordered west to east.
 *
 * An empty selection means the built-in table, not an empty panel. That is the
 * whole reason "Restore defaults" can be a delete: the absence of a choice is
 * itself a valid state, and it keeps tracking whatever the defaults become.
 *
 * Because the order is always computed from the offsets, the order a selection
 * happens to be stored in carries no meaning. Nothing should read it as though
 * it did.
 */
export function resolvedZones(preferences: DtgPreferences, instant: Date): ZoneChoice[] {
    const entries = preferences.zones.length > 0 ? preferences.zones : DEFAULT_SELECTION;
    return orderedZones(entries, instant);
}

/** Turns a saved threshold into milliseconds. Zero means the built-in one. */
export function resolvedUrgentWithinMs(preferences: DtgPreferences): number {
    if (preferences.urgentWithinMinutes <= 0) {
        return DEFAULT_URGENT_WITHIN_MS;
    }
    return preferences.urgentWithinMinutes * 60 * 1000;
}

/**
 * The zone list to show in the editor.
 *
 * A reader who has chosen nothing is editing the defaults, so the editor starts
 * from the rows they can actually see rather than from an empty list with no
 * hint of what removing something would do.
 */
export function editableZoneIds(preferences: DtgPreferences): ZoneSelection[] {
    return preferences.zones.length > 0 ? [...preferences.zones] : [...DEFAULT_SELECTION];
}

/**
 * Collapses a selection that is just the defaults back to "unset".
 *
 * Without this, opening the editor and pressing Save would freeze the reader's
 * table at whatever the defaults happen to be today, and they would never see a
 * zone added to the built-in list again. Saving without changing anything
 * should change nothing.
 *
 * Compared as a set, not as a sequence, because the rows are always ordered by
 * offset at render time. Two selections holding the same entries are the same
 * selection however they happen to be stored.
 */
export function normalizeZoneSelection(entries: ZoneSelection[]): ZoneSelection[] {
    const chosen = new Set(entries.map(zoneKey));
    const isDefaults = chosen.size === DEFAULT_SELECTION.length &&
        DEFAULT_SELECTION.every((entry) => chosen.has(zoneKey(entry)));

    return isDefaults ? [] : entries;
}
