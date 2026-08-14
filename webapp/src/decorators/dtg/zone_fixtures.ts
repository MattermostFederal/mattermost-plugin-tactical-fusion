import type {ZoneChoice, ZoneGroup} from './zones';

/**
 * Hand-built zones for the picker's own tests.
 *
 * Test-only, and imported only from ZonePickerHarness and ZonePicker.pw.tsx, so
 * it never reaches the plugin bundle. It lives in a plain module rather than
 * beside the harness because Playwright CT rewrites every import from a
 * component file into a component handle, which a plain array cannot survive.
 *
 * Built by hand rather than taken from `availableZoneGroups` so the assertions
 * do not move with the host's tzdata or with the season. These tests are about
 * the picker's behavior, not about which zones exist.
 */
function choice(name: string, iana: string, offsetMinutes: number, abbr = ''): ZoneChoice {
    const sign = offsetMinutes < 0 ? '-' : '+';
    const abs = Math.abs(offsetMinutes);
    const hh = String(Math.floor(abs / 60)).padStart(2, '0');
    const mm = String(abs % 60).padStart(2, '0');
    return {
        name,
        iana,
        abbr,
        key: `${iana}|${name}`,
        offsetMinutes,
        offsetLabel: `UTC${sign}${hh}:${mm}`,
    };
}

// Ramstein and Stuttgart deliberately share a zone: two rows that read the same
// to the minute, which is what makes identity the pair rather than the zone.
export const BASES: ZoneChoice[] = [
    choice('Ramstein', 'Europe/Berlin', 120),
    choice('Stuttgart', 'Europe/Berlin', 120),
    choice('Yokota', 'Asia/Tokyo', 540),
];

export const ALL: ZoneChoice[] = [
    choice('Los Angeles', 'America/Los_Angeles', -420),
    choice('Zulu (UTC)', 'UTC', 0),
    choice('Kolkata', 'Asia/Kolkata', 330),
];

export const DEFAULT_GROUPS: ZoneGroup[] = [
    {label: 'Bases and common zones', zones: BASES},
    {label: 'All timezones', zones: ALL},
];

export const TOTAL_ZONES = BASES.length + ALL.length;
