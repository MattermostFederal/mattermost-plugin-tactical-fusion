import {expect, test} from '@playwright/test';

import {EMPTY_PREFERENCES, fromWire, toWire, wireHasNoChoices} from './types';

// The payload comes off the network, so every shape that is not the expected
// one has to degrade to the defaults rather than reach Intl or a style
// calculation.
const junk: unknown[] = [
    null,
    undefined,
    0,
    'preferences',
    [],
    {},
    {dtg: null},
    {dtg: 'nope'},
    {dtg: []},
];

for (const raw of junk) {
    test(`degrades to the defaults for ${JSON.stringify(raw) ?? 'undefined'}`, () => {
        expect(fromWire(raw)).toEqual(EMPTY_PREFERENCES);
    });
}

test('reads a well-formed blob', () => {
    const raw = {dtg: {zones: [{iana: 'UTC'}, {iana: 'Asia/Tokyo', name: 'Yokota'}], urgent_within_minutes: 15}};

    expect(fromWire(raw)).toEqual({
        dtg: {zones: [{iana: 'UTC'}, {iana: 'Asia/Tokyo', name: 'Yokota'}], urgentWithinMinutes: 15},
        location: {hiddenRows: []},
        cot: {hiddenSections: []},
        geojson: {hiddenSections: []},
    });
});

test('keeps only the usable entries in a zone list', () => {
    const parsed = fromWire({dtg: {zones: [{iana: 'UTC'}, 7, null, {}, {iana: ''}, {iana: 'Asia/Tokyo'}]}});
    expect(parsed.dtg.zones).toEqual([{iana: 'UTC'}, {iana: 'Asia/Tokyo'}]);
});

// Blobs written before base names existed hold bare identifiers. Discarding
// them would silently wipe a reader's settings on upgrade.
test('reads a blob written before names existed', () => {
    expect(fromWire({dtg: {zones: ['UTC', 'Asia/Tokyo']}}).dtg.zones).
        toEqual([{iana: 'UTC'}, {iana: 'Asia/Tokyo'}]);
});

test('drops a name that is not a string', () => {
    expect(fromWire({dtg: {zones: [{iana: 'UTC', name: 7}]}}).dtg.zones).toEqual([{iana: 'UTC'}]);
});

test('a zone list that is not a list reads as empty', () => {
    expect(fromWire({dtg: {zones: 'UTC'}}).dtg.zones).toEqual([]);
});

// Zero already means "use the default", so anything unusable maps onto it
// rather than onto a threshold nobody chose.
const badThresholds: unknown[] = [-1, 0, 1.5, NaN, Infinity, '15', null];

for (const value of badThresholds) {
    test(`an unusable threshold (${String(value)}) means the default`, () => {
        expect(fromWire({dtg: {urgent_within_minutes: value}}).dtg.urgentWithinMinutes).toBe(0);
    });
}

// The wire names are a compatibility surface: renaming one silently discards
// everybody's saved settings.
test('writes the names the server reads', () => {
    expect(toWire({
        dtg: {zones: [{iana: 'UTC'}], urgentWithinMinutes: 15},
        location: {hiddenRows: ['ddm']},
        cot: {hiddenSections: ['payload']},
        geojson: {hiddenSections: ['properties']},
    })).toEqual({
        dtg: {zones: [{iana: 'UTC'}], urgent_within_minutes: 15},
        location: {hidden_rows: ['ddm']},
        cot: {hidden_sections: ['payload']},
        geojson: {hidden_sections: ['properties']},
    });
});

test('round-trips', () => {
    const original = {
        dtg: {
            zones: [{iana: 'UTC'}, {iana: 'Europe/Berlin', name: 'USAG Stuttgart'}],
            urgentWithinMinutes: 45,
        },
        location: {hiddenRows: ['ddm' as const, 'datum' as const]},
        cot: {hiddenSections: ['payload' as const, 'flow' as const]},
        geojson: {hiddenSections: ['properties' as const, 'source' as const]},
    };

    expect(fromWire(toWire(original))).toEqual(original);
});

// A blob from a build that had a row this one does not must not take the
// reader's other settings down with it: reading is forgiving where writing is
// strict, so an id nothing renders simply hides nothing.
test('drops a hidden row this build does not have', () => {
    const read = fromWire({location: {hidden_rows: ['ddm', 'sextant', 'ddm']}});
    expect(read.location.hiddenRows).toEqual(['ddm']);
});

test('an absent location key reads as nothing hidden', () => {
    expect(fromWire({dtg: {}}).location.hiddenRows).toEqual([]);
});

// The same forgiveness on the Cursor on Target section, for the same reason:
// retiring a section must not lock a reader out of the settings around it.
test('drops a hidden section this build does not have', () => {
    const read = fromWire({cot: {hidden_sections: ['payload', 'telepathy', 'payload']}});
    expect(read.cot.hiddenSections).toEqual(['payload']);
});

test('an absent cot key reads as nothing hidden', () => {
    expect(fromWire({dtg: {}}).cot.hiddenSections).toEqual([]);
});

test('a reader who has hidden only GeoJSON sections has made a choice', () => {
    expect(wireHasNoChoices({geojson: {hidden_sections: ['map']}})).toBe(false);
});

test('a blob with nothing chosen anywhere is still empty', () => {
    expect(wireHasNoChoices({
        dtg: {zones: [], urgent_within_minutes: 0},
        location: {hidden_rows: []},
        cot: {hidden_sections: []},
        geojson: {hidden_sections: []},
    })).toBe(true);
});
