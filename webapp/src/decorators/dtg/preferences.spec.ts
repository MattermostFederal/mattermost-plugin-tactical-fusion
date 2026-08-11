import {expect, test} from '@playwright/test';

import {
    editableZoneIds,
    normalizeZoneSelection,
    resolvedUrgentWithinMs,
    resolvedZones,
} from './preferences';
import {DEFAULT_URGENT_WITHIN_MS} from './relative';
import type {ZoneSelection} from './zones';
import {DEFAULT_SELECTION, DEFAULT_ZONE_IDS, DISPLAY_ZONES, zoneKey} from './zones';

const instant = new Date(Date.UTC(2026, 7, 9, 16, 30, 0));

function prefs(zones: ZoneSelection[] = [], urgentWithinMinutes = 0) {
    return {zones, urgentWithinMinutes};
}

/** A bare zone, the way the "All timezones" group offers one. */
function bare(...ids: string[]): ZoneSelection[] {
    return ids.map((iana) => ({iana}));
}

// An empty selection is a valid state, not an empty panel. That is what lets
// "Restore defaults" be a delete.
test('no selection means the built-in table', () => {
    const rows = resolvedZones(prefs(), instant);

    expect(rows.map((row) => row.key).sort()).toEqual(DEFAULT_SELECTION.map(zoneKey).sort());
    expect(rows).toHaveLength(DISPLAY_ZONES.length);
});

// The order is computed from the offsets, so the order a selection is stored in
// carries no meaning and nothing may read it as though it did.
test('a selection is ordered west to east, not as it was given', () => {
    const rows = resolvedZones(prefs(bare('Asia/Tokyo', 'UTC')), instant);

    expect(rows.map((row) => row.iana)).toEqual(['UTC', 'Asia/Tokyo']);
});

test('the built-in table is ordered west to east too', () => {
    const rows = resolvedZones(prefs(), instant);

    expect(rows[0].name).toBe('Honolulu');
    expect(rows[rows.length - 1].name).toBe('Andersen, Guam');
});

// Picked bare out of "All timezones", so it is the zone that was chosen, not
// the base that happens to sit in it. The abbreviation is still the curated one.
test('a bare built-in zone reads as its city', () => {
    const [row] = resolvedZones(prefs(bare('America/Los_Angeles')), instant);

    expect(row.name).toBe('Los Angeles');
    expect(row.abbr).toBe('PT');
});

test('a zone the reader added is described from its identifier', () => {
    const [row] = resolvedZones(prefs(bare('Europe/Paris')), instant);

    expect(row.name).toBe('Paris');
    expect(row.iana).toBe('Europe/Paris');
    expect(row.abbr).not.toBe('');
});

test('no threshold means the built-in one', () => {
    expect(resolvedUrgentWithinMs(prefs())).toBe(DEFAULT_URGENT_WITHIN_MS);
});

test('a threshold is read as minutes', () => {
    expect(resolvedUrgentWithinMs(prefs([], 15))).toBe(15 * 60 * 1000);
    expect(resolvedUrgentWithinMs(prefs([], 1))).toBe(60 * 1000);
});

// Nothing should be able to produce a negative window, which would mean the
// countdown never flashes even at the instant itself.
test('a nonsensical threshold falls back to the built-in one', () => {
    expect(resolvedUrgentWithinMs(prefs([], -5))).toBe(DEFAULT_URGENT_WITHIN_MS);
});

// A reader who has chosen nothing is editing the defaults, so the editor starts
// from the rows they can actually see.
test('the editor starts from the defaults when nothing is saved', () => {
    expect(editableZoneIds(prefs())).toEqual(DEFAULT_SELECTION);
});

test('the editor works on a copy, not on the default list itself', () => {
    const editable = editableZoneIds(prefs());
    editable.push({iana: 'Europe/Paris'});

    expect(DEFAULT_SELECTION).toHaveLength(DEFAULT_ZONE_IDS.length);
    expect(editableZoneIds(prefs())).toEqual(DEFAULT_SELECTION);
});

test('the editor starts from a saved selection when there is one', () => {
    expect(editableZoneIds(prefs(bare('UTC')))).toEqual(bare('UTC'));
});

// Without this, opening the editor and pressing Save would freeze the reader's
// table at today's defaults, and they would never see a zone added to the
// built-in list again.
test('saving the defaults unchanged stores nothing', () => {
    expect(normalizeZoneSelection([...DEFAULT_SELECTION])).toEqual([]);
});

// Compared as a set: the rows are ordered by offset at render time, so two
// selections holding the same zones are the same selection.
test('the defaults in a different order are still the defaults', () => {
    expect(normalizeZoneSelection([...DEFAULT_SELECTION].reverse())).toEqual([]);
});

test('any real change is stored as given', () => {
    const dropped = DEFAULT_SELECTION.slice(0, -1);
    expect(normalizeZoneSelection(dropped)).toEqual(dropped);

    const added = [...DEFAULT_SELECTION, {iana: 'Europe/Paris'}];
    expect(normalizeZoneSelection(added)).toEqual(added);
});

// Same length as the defaults, but not the same zones. A count-only check
// would wrongly throw this away.
test('a same-sized selection of different zones is stored', () => {
    const swapped = [...DEFAULT_SELECTION.slice(0, -1), {iana: 'Europe/Paris'}];

    expect(normalizeZoneSelection(swapped)).toEqual(swapped);
});

test('an empty selection is already unset', () => {
    expect(normalizeZoneSelection([])).toEqual([]);
});

// The defaults now carry their names, so a fresh reader sees the curated
// labels rather than city names derived from the identifiers.
test('the default rows keep their curated names', () => {
    const rows = resolvedZones(prefs(), instant);

    expect(rows.map((row) => row.name)).toContain('San Diego');
    expect(rows.map((row) => row.name)).toContain('Andersen, Guam');
});

// Two bases in one zone are two rows, and the selection has to keep both.
test('two bases in one zone both survive', () => {
    const rows = resolvedZones(prefs([
        {iana: 'Europe/Berlin', name: 'Ramstein'},
        {iana: 'Europe/Berlin', name: 'USAG Stuttgart'},
    ]), instant);

    expect(rows).toHaveLength(2);
    expect(normalizeZoneSelection([
        {iana: 'Europe/Berlin', name: 'Ramstein'},
        {iana: 'Europe/Berlin', name: 'USAG Stuttgart'},
    ])).toHaveLength(2);
});
