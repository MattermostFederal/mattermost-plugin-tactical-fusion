import {expect, test} from '@playwright/test';

import {
    availableZoneGroups,
    availableZones,
    DEFAULT_ZONE_IDS,
    DISPLAY_ZONES,
    displayZoneFor,
    formatZoneOffset,
    MILITARY_BASES,
    orderedZones,
    DEFAULT_SELECTION,
    zoneKey,
    zoneOffsetMinutes,
} from './zones';

const summer = new Date(Date.UTC(2026, 7, 9, 16, 30, 0));
const winter = new Date(Date.UTC(2026, 0, 9, 16, 30, 0));

test('the identifier list matches the table', () => {
    expect(DEFAULT_ZONE_IDS).toEqual(DISPLAY_ZONES.map((zone) => zone.iana));
});

// The hand-written abbreviation is keyed off the zone; the name is not, so a
// bare Asia/Tokyo reads as Tokyo rather than as somebody else's base.
test('a built-in zone keeps its hand-written abbreviation', () => {
    expect(displayZoneFor({iana: 'Asia/Tokyo'}, summer)).toEqual({
        name: 'Tokyo',
        iana: 'Asia/Tokyo',
        abbr: 'JST',
    });
});

test('a chosen base keeps both its own name and the abbreviation', () => {
    expect(displayZoneFor({iana: 'Asia/Tokyo', name: 'Yokota'}, summer)).toEqual({
        name: 'Yokota',
        iana: 'Asia/Tokyo',
        abbr: 'JST',
    });
});

// The city, not the whole identifier: this has to fit in a narrow sidebar, and
// the identifier is shown alongside it in the editor anyway.
test('a zone the reader added is named after its city', () => {
    expect(displayZoneFor({iana: 'Europe/Paris'}, summer).name).toBe('Paris');
    expect(displayZoneFor({iana: 'America/Argentina/Buenos_Aires'}, summer).name).toBe('Buenos Aires');
});

// The label is computed per instant, unlike the hand-written ones, so it tells
// the truth about daylight saving.
test('an added zone abbreviates itself for the season', () => {
    const summerAbbr = displayZoneFor({iana: 'Europe/Paris'}, summer).abbr;
    const winterAbbr = displayZoneFor({iana: 'Europe/Paris'}, winter).abbr;

    expect(summerAbbr).not.toBe('');
    expect(winterAbbr).not.toBe('');
    expect(summerAbbr).not.toBe(winterAbbr);
});

// A saved blob outlives the browser that wrote it, so a zone this engine cannot
// format has to degrade to a row rather than throw the panel away.
test('an unresolvable zone still produces a row', () => {
    const row = displayZoneFor({iana: 'Mars/Olympus_Mons'}, summer);

    expect(row.iana).toBe('Mars/Olympus_Mons');
    expect(row.name).toBe('Olympus Mons');
    expect(row.abbr).toBe('');
});

// Measured against known offsets. A table of offsets would be wrong twice a
// year in every zone that observes daylight saving, which is most of them.
const offsets: Array<[string, Date, number]> = [
    ['UTC', summer, 0],
    ['Asia/Tokyo', summer, 9 * 60],
    ['Asia/Qatar', summer, 3 * 60],
    ['Pacific/Guam', summer, 10 * 60],
    ['America/New_York', summer, -4 * 60],
    ['Pacific/Honolulu', summer, -10 * 60],

    // Half-hour and three-quarter-hour zones exist, so minutes are not a
    // formality.
    ['Asia/Kolkata', summer, (5 * 60) + 30],
    ['Asia/Kathmandu', summer, (5 * 60) + 45],

    // The same zone either side of a daylight saving change.
    ['America/Los_Angeles', summer, -7 * 60],
    ['America/Los_Angeles', winter, -8 * 60],
    ['Europe/Berlin', summer, 2 * 60],
    ['Europe/Berlin', winter, 60],

    // Southern hemisphere, so the daylight saving runs the other way. These two
    // are the premise of the seasonal ordering test below.
    ['America/Halifax', summer, -3 * 60],
    ['America/Halifax', winter, -4 * 60],
    ['America/Santiago', summer, -4 * 60],
    ['America/Santiago', winter, -3 * 60],
];

for (const [iana, instant, want] of offsets) {
    test(`${iana} is ${want} minutes from UTC at ${instant.toISOString()}`, () => {
        expect(zoneOffsetMinutes(iana, instant)).toBe(want);
    });
}

// en-US reads hour12:false as h24 and renders midnight as hour 24, which would
// put every zone's offset a day out once a day.
test('midnight in the zone does not throw the offset a day out', () => {
    // 14:00Z is midnight the next day in Guam, which is UTC+10.
    const atMidnight = new Date(Date.UTC(2026, 7, 9, 14, 0, 0));
    expect(zoneOffsetMinutes('Pacific/Guam', atMidnight)).toBe(10 * 60);
});

// The wall clock parts carry no milliseconds, so the instant has to be
// truncated to match before subtracting.
test('a sub-second instant does not skew the offset', () => {
    const odd = new Date(Date.UTC(2026, 7, 9, 16, 30, 0, 750));
    expect(zoneOffsetMinutes('Asia/Tokyo', odd)).toBe(9 * 60);
});

test('an unresolvable zone has no offset', () => {
    expect(zoneOffsetMinutes('Mars/Olympus_Mons', summer)).toBeNull();
});

const formatted: Array<[number, string]> = [
    [0, 'UTC+00:00'],
    [60, 'UTC+01:00'],
    [-8 * 60, 'UTC-08:00'],
    [(5 * 60) + 30, 'UTC+05:30'],
    [(5 * 60) + 45, 'UTC+05:45'],
    [-(9 * 60) - 30, 'UTC-09:30'],
    [14 * 60, 'UTC+14:00'],
];

for (const [minutes, want] of formatted) {
    test(`${minutes} minutes reads as ${want}`, () => {
        expect(formatZoneOffset(minutes)).toBe(want);
    });
}

test('zones come back ordered west to east', () => {
    const ordered = orderedZones(DEFAULT_SELECTION, summer);

    expect(ordered.map((zone) => zone.name)).toEqual([
        'Honolulu',
        'San Diego',
        'Colorado Springs',
        'Washington, DC',
        'Zulu (UTC)',
        'Ramstein',
        'Al Udeid',
        'Yokota',
        'Andersen, Guam',
    ]);
});

// The order is measured, not stored, so it has to move with daylight saving
// rather than freeze at whatever the offsets were in August.
//
// Opposite hemispheres, so the two swap outright rather than tying: Halifax is
// on daylight time in August and Santiago in January. A pair that merely ties
// would be testing the name tiebreak instead of the measurement. The same pair
// is asserted in server/decorators/dtg/zones_test.go.
test('the order follows the season', () => {
    const pair = [{iana: 'America/Halifax'}, {iana: 'America/Santiago'}];

    expect(orderedZones(pair, summer).map((zone) => zone.iana)).
        toEqual(['America/Santiago', 'America/Halifax']);
    expect(orderedZones(pair, winter).map((zone) => zone.iana)).
        toEqual(['America/Halifax', 'America/Santiago']);
});

// Treating an unknown offset as zero would file it under UTC, which is a claim
// rather than an admission.
test('a zone with no offset sorts last', () => {
    const ordered = orderedZones([{iana: 'Mars/Olympus_Mons'}, {iana: 'Asia/Tokyo'}, {iana: 'UTC'}], summer);

    expect(ordered.map((zone) => zone.iana)).toEqual(['UTC', 'Asia/Tokyo', 'Mars/Olympus_Mons']);
    expect(ordered[2].offsetLabel).toBe('');
});

test('every ordered zone carries its offset label', () => {
    const ordered = orderedZones([{iana: 'Asia/Kolkata'}, {iana: 'America/Denver'}], summer);

    expect(ordered[0].offsetLabel).toBe('UTC-06:00');
    expect(ordered[1].offsetLabel).toBe('UTC+05:30');
});

// A base whose identifier does not resolve would render a row of blanks, and
// nothing else in the code would notice.
test('every base names a timezone this engine can resolve', () => {
    for (const base of MILITARY_BASES) {
        expect(zoneOffsetMinutes(base.iana, summer), `${base.name} (${base.iana})`).not.toBeNull();
    }
});

// Every base is separately selectable, so identity has to be the pair. Two
// entries with one key would collide in the picker and in React's keys.
test('no two bases share an identity', () => {
    const keys = MILITARY_BASES.map(zoneKey);
    expect(new Set(keys).size, `duplicate entry among ${keys.join(', ')}`).toBe(keys.length);
});

// Somebody at Stuttgart wants to see "Stuttgart", not the Ramstein row that
// keeps the same clock, so several bases may share a zone.
test('bases may share a zone', () => {
    const berlin = MILITARY_BASES.filter((base) => base.iana === 'Europe/Berlin');
    expect(berlin.map((base) => base.name)).toContain('Ramstein');
    expect(berlin.map((base) => base.name)).toContain('USAG Stuttgart');
});

test('the catalogue includes every default row', () => {
    const keys = new Set(MILITARY_BASES.map(zoneKey));
    for (const entry of DEFAULT_SELECTION) {
        expect(keys, `${entry.name} is missing from the catalogue`).toContain(zoneKey(entry));
    }
});

// The name reaches a row by being stored with it. Inferring one from the zone
// would label a bare Europe/Rome "Aviano AB", which the reader never chose.
test('a chosen name names the row', () => {
    expect(displayZoneFor({iana: 'Europe/Rome', name: 'Aviano AB'}, summer).name).toBe('Aviano AB');
    expect(displayZoneFor({iana: 'Europe/Rome'}, summer).name).toBe('Rome');
});

test('two bases in one zone keep their own names', () => {
    const rows = orderedZones([
        {iana: 'Europe/Berlin', name: 'Ramstein'},
        {iana: 'Europe/Berlin', name: 'USAG Stuttgart'},
    ], summer);

    expect(rows.map((row) => row.name)).toEqual(['Ramstein', 'USAG Stuttgart']);
    expect(rows[0].key).not.toBe(rows[1].key);

    // Same zone, so the same clock. That is the accepted cost of naming both.
    expect(rows[0].offsetMinutes).toBe(rows[1].offsetMinutes);
});

// The curated rows keep their hand-written abbreviations; everything else gets
// a measured one, since the right answer moves with the season.
test('a chosen zone abbreviates itself for the season', () => {
    expect(displayZoneFor({iana: 'Asia/Tokyo', name: 'Kadena AB'}, summer).abbr).toBe('JST');

    expect(displayZoneFor({iana: 'Europe/Rome'}, summer).abbr).not.toBe('');
    expect(displayZoneFor({iana: 'Europe/Rome'}, summer).abbr).
        not.toBe(displayZoneFor({iana: 'Europe/Rome'}, winter).abbr);
});

test('the picker offers Zulu', () => {
    expect(availableZones(summer).map((zone) => zone.iana)).toContain('UTC');
});

test('the picker offers every built-in zone', () => {
    const available = availableZones(summer).map((zone) => zone.iana);
    for (const id of DEFAULT_ZONE_IDS) {
        expect(available).toContain(id);
    }
});

// Engines canonicalise, so several bases are backward links their supported
// list leaves out. A base the picker cannot offer is a base nobody can choose.
test('the picker offers every base, canonical or not', () => {
    const available = availableZones(summer).map((zone) => zone.iana);
    for (const base of MILITARY_BASES) {
        expect(available, `${base.name} (${base.iana}) is missing`).toContain(base.iana);
    }
});

test('the picker leads with a shortcut group, then everything', () => {
    const groups = availableZoneGroups(summer);

    expect(groups.map((group) => group.label)).toEqual(['Bases and common zones', 'All timezones']);
    expect(groups[0].zones).toHaveLength(MILITARY_BASES.length);
    expect(groups[1].zones.length).toBeGreaterThan(groups[0].zones.length);
});

test('each group runs west to east on its own', () => {
    for (const group of availableZoneGroups(summer)) {
        const offsets = group.zones.map((zone) => zone.offsetMinutes ?? Infinity);
        expect([...offsets].sort((a, b) => a - b), group.label).toEqual(offsets);
    }
});

test('the picker is ordered by offset and free of duplicates', () => {
    const available = availableZones(summer);
    const ids = available.map((zone) => zone.iana);

    expect(new Set(ids).size).toBe(ids.length);

    const measured = available.map((zone) => zone.offsetMinutes ?? Infinity);
    expect([...measured].sort((a, b) => a - b)).toEqual(measured);
});

// If this ever collapses to the fallback, the picker silently shrinks to nine
// entries and "select your own timezones" stops meaning anything.
test('the picker is the browser list, not the fallback', () => {
    expect(availableZones(summer).length).toBeGreaterThan(DEFAULT_ZONE_IDS.length);
});
