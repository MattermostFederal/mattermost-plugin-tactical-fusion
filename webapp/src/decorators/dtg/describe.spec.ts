import {expect, test} from '@playwright/test';

import {describeInstant, formatOffsetLabel} from './describe';
import {ZONE_OFFSETS as OFFSETS} from './zones';

// This table is duplicated in server/decorators/dtg/page_test.go on purpose.
// The sidebar and the standalone page describe the same instant, so a
// divergence would read as two different times for one message.
const cases: Array<[string, Date, string, string]> = [
    [
        'zulu is shown as stored',
        new Date(Date.UTC(2026, 7, 9, 16, 30)),
        'Z',
        '09 Aug 2026 16:30 Z',
    ],
    [
        'a positive zone is shown in its own local time',

        // 16:30Z is 18:30 in B, which is UTC+2.
        new Date(Date.UTC(2026, 7, 9, 16, 30)),
        'B',
        '09 Aug 2026 18:30 B',
    ],
    [
        'a negative zone is shown in its own local time',

        // 16:30Z is 11:30 in R, which is UTC-5.
        new Date(Date.UTC(2026, 7, 9, 16, 30)),
        'R',
        '09 Aug 2026 11:30 R',
    ],
    [
        'the zone offset can roll the date back',

        // 01:00Z in M, which is UTC+12, is the 9th at 13:00.
        new Date(Date.UTC(2026, 7, 9, 1, 0)),
        'M',
        '09 Aug 2026 13:00 M',
    ],
    [
        'the zone offset can roll the date forward',

        // 23:00Z in R, which is UTC-5, is still the 9th at 18:00.
        new Date(Date.UTC(2026, 7, 9, 23, 0)),
        'R',
        '09 Aug 2026 18:00 R',
    ],
    [
        'single digit days and months are padded',
        new Date(Date.UTC(2026, 0, 1, 5, 7)),
        'Z',
        '01 Jan 2026 05:07 Z',
    ],
];

for (const [name, instant, zone, want] of cases) {
    test(name, () => {
        expect(describeInstant(instant, OFFSETS[zone] * 60, zone)).toBe(want);
    });
}

// Mirrors FormatOffset in server/decorators/dtg/parse.go. This is the label on
// a token, so it reads the way the token was written, unlike the picker's
// UTC-prefixed form.
const offsets: Array<[number, string]> = [
    [0, 'Z'],
    [4 * 60, '+04:00'],
    [-5 * 60, '-05:00'],
    [330, '+05:30'],
    [-(9 * 60) - 30, '-09:30'],
    [14 * 60, '+14:00'],
];

for (const [minutes, want] of offsets) {
    test(`an offset of ${minutes} minutes reads as ${want}`, () => {
        expect(formatOffsetLabel(minutes)).toBe(want);
    });
}

// A timestamp is read in the offset it was written in, not in UTC.
test('describes a timestamp in its own offset', () => {
    const instant = new Date(Date.UTC(2026, 7, 9, 16, 30));

    expect(describeInstant(instant, 240, '+04:00')).toBe('09 Aug 2026 20:30 +04:00');
    expect(describeInstant(instant, 330, '+05:30')).toBe('09 Aug 2026 22:00 +05:30');
    expect(describeInstant(instant, -300, '-05:00')).toBe('09 Aug 2026 11:30 -05:00');
});
