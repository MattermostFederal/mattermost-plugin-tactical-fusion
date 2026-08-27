import {expect, test} from '@playwright/test';

import {MAX_TIMEOUT_MS, staleWait} from './stale';

const MINUTE = 60 * 1000;
const DAY = 24 * 60 * MINUTE;

test('a wait a browser can hold settles the countdown when it elapses', () => {
    expect(staleWait(90 * MINUTE)).toEqual({ms: 90 * MINUTE, settles: true});
});

test('a wait at the cap still settles, since nothing wrapped', () => {
    expect(staleWait(MAX_TIMEOUT_MS)).toEqual({ms: MAX_TIMEOUT_MS, settles: true});
});

/*
 * The stale time is the author's, so it is unbounded. A wait past the cap used
 * to be passed to setTimeout whole, wrap to a small number and fire at once,
 * and the panel opened on a year-long geofence already calling it Stale.
 */
test('a wait past the cap is held back rather than treated as elapsed', () => {
    const wait = staleWait(400 * DAY);

    expect(wait.ms).toBe(MAX_TIMEOUT_MS);
    expect(wait.settles).toBe(false);
});

test('the cap is what setTimeout can hold in a signed 32-bit integer', () => {
    expect(MAX_TIMEOUT_MS).toBe((2 ** 31) - 1);
});
