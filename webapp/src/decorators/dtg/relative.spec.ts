import {expect, test} from '@playwright/test';

import {formatRelative, isUrgent} from './relative';

const now = new Date(Date.UTC(2026, 7, 9, 12, 0, 0));

function at(offsetSeconds: number): Date {
    return new Date(now.getTime() + (offsetSeconds * 1000));
}

// This table is duplicated in server/decorators/dtg/page_test.go on purpose.
// The sidebar, the server-rendered page and the page's countdown script all
// format the same instant, so a divergence would show the same DTG two
// different ways depending on where you looked.
const cases: Array<[string, number, string]> = [
    ['same instant', 0, 'now'],
    ['under a second', 0, 'now'],
    ['seconds only', 15, 'in 15s'],
    ['minutes keep seconds', 65, 'in 1m 5s'],
    ['hours keep everything below', (3 * 3600) + (20 * 60) + 15, 'in 3h 20m 15s'],
    ['zero units are kept once a larger one appears', 3600, 'in 1h 0m 0s'],
    ['days keep everything below', (2 * 86400) + (2 * 3600), 'in 2d 2h 0m 0s'],
    ['past instants count up', -90 * 60, '1h 30m 0s ago'],
    ['past seconds', -5, '5s ago'],
];

for (const [name, offset, want] of cases) {
    test(name, () => {
        expect(formatRelative(now, at(offset))).toBe(want);
    });
}

// Sub-second differences round toward "now" rather than showing "in 0s", so the
// display never sits on a value that reads as though nothing is happening.
test('sub-second offsets read as now', () => {
    expect(formatRelative(now, new Date(now.getTime() + 400))).toBe('now');
    expect(formatRelative(now, new Date(now.getTime() - 400))).toBe('now');
});

// This table is duplicated in server/decorators/dtg/page_test.go on purpose.
// The sidebar, the page and the page's countdown script all decide urgency
// independently, so a divergence would flash in one place and not another.
const urgencyCases: Array<[string, number, boolean]> = [
    ['at the instant', 0, true],
    ['well inside, ahead', 5 * 60, true],
    ['well inside, behind', -5 * 60, true],
    ['one second inside, ahead', (30 * 60) - 1, true],
    ['exactly on the threshold, ahead', 30 * 60, true],
    ['exactly on the threshold, behind', -30 * 60, true],
    ['one second outside, ahead', (30 * 60) + 1, false],
    ['one second outside, behind', -((30 * 60) + 1), false],
    ['far away', 48 * 3600, false],
];

for (const [name, offset, want] of urgencyCases) {
    test(`urgency ${name}`, () => {
        expect(isUrgent(now, at(offset))).toBe(want);
    });
}
