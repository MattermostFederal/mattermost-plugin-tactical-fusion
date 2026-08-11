import {expect, test} from '@playwright/test';

import decorator, {fromParams} from './index';

// Kept in step with the Go tests: 091630ZAUG26 is 2026-08-09T16:30:00Z.
const INSTANT_MS = Date.UTC(2026, 7, 9, 16, 30, 0);

function params(overrides: Record<string, string> = {}): URLSearchParams {
    return new URLSearchParams({
        t: String(INSTANT_MS),
        dtg: '091630ZAUG26',
        z: 'Z',
        a: '',
        ...overrides,
    });
}

test('builds a payload from valid params', () => {
    const payload = fromParams(params());

    expect(payload).not.toBeNull();
    expect(payload!.canonical).toBe('091630ZAUG26');
    expect(payload!.zoneLabel).toBe('Z');
    expect(payload!.offsetMinutes).toBe(0);
    expect(payload!.instant.getTime()).toBe(INSTANT_MS);
    expect(payload!.assumedMonth).toBe(false);
    expect(payload!.assumedYear).toBe(false);
});

test('reads the assumed-fields code', () => {
    const payload = fromParams(params({dtg: '091630Z', a: 'my'}));

    expect(payload).not.toBeNull();
    expect(payload!.assumedMonth).toBe(true);
    expect(payload!.assumedYear).toBe(true);
});

test('accepts the short canonical form', () => {
    expect(fromParams(params({dtg: '091630Z'}))).not.toBeNull();
});

// Params come from a URL a user could have hand-edited, so all of these must be
// rejected rather than rendered.
test('rejects unusable params', () => {
    const cases: Array<[string, Record<string, string>]> = [
        ['non-numeric t', {t: 'abc'}],
        ['fractional t', {t: '123.5'}],
        ['negative t', {t: '-1'}],
        ['absurdly large t', {t: '99999999999999'}],
        ['malformed dtg', {dtg: 'not-a-dtg'}],
        ['dtg with markup', {dtg: '<script>alert(1)</script>'}],
        ['lowercase zone', {z: 'z'}],
        ['multi-character zone', {z: 'ZZ'}],
        ['unknown assumed code', {a: 'x'}],

        // The server rejects I and J. Accepting them here would intercept a
        // hand-crafted link and render it with a silent UTC fallback instead of
        // letting the browser reach the server's error page.
        ['zone letter I', {dtg: '091630IAUG26', z: 'I'}],
        ['zone letter J', {dtg: '091630JAUG26', z: 'J'}],

        // z must agree with the zone letter inside the canonical form.
        ['zone disagrees with dtg', {z: 'B'}],
    ];

    for (const [name, overrides] of cases) {
        expect(fromParams(params(overrides)), name).toBeNull();
    }
});

test('rejects missing params', () => {
    expect(fromParams(new URLSearchParams())).toBeNull();
    expect(fromParams(new URLSearchParams('t=' + INSTANT_MS))).toBeNull();
});

// The sidebar header names the category, not the value. The canonical DTG is
// already the first line of the panel. This must match pageTitle in
// server/decorators/dtg/dtg.go so the sidebar and the standalone page agree.
test('the sidebar header is the category, not the value', () => {
    const payload = fromParams(params());

    expect(decorator.summary(payload!)).toBe('Date/Time');
});

test('the header is the same for every DTG', () => {
    const long = fromParams(params())!;
    const short = fromParams(params({dtg: '091630Z', a: 'my'}))!;

    expect(decorator.summary(short)).toBe(decorator.summary(long));
});

/** The params the server emits for an RFC 3339 timestamp. */
function isoParams(overrides: Record<string, string> = {}): URLSearchParams {
    return new URLSearchParams({
        t: String(INSTANT_MS),
        dtg: '2026-08-09T20:30:00+04:00',
        o: '240',
        a: '',
        ...overrides,
    });
}

test('builds a payload from timestamp params', () => {
    const payload = fromParams(isoParams());

    expect(payload).not.toBeNull();
    expect(payload!.canonical).toBe('2026-08-09T20:30:00+04:00');
    expect(payload!.offsetMinutes).toBe(240);
    expect(payload!.zoneLabel).toBe('+04:00');
    expect(payload!.instant.getTime()).toBe(INSTANT_MS);
    expect(payload!.assumedMonth).toBe(false);
    expect(payload!.assumedYear).toBe(false);
});

test('labels a zero offset as Zulu', () => {
    const payload = fromParams(isoParams({dtg: '2026-08-09T16:30:00Z', o: '0'}));

    expect(payload!.zoneLabel).toBe('Z');
    expect(payload!.offsetMinutes).toBe(0);
});

// A military zone letter cannot say this, which is the whole reason the payload
// carries minutes rather than a letter.
test('accepts a half-hour offset', () => {
    const payload = fromParams(isoParams({dtg: '2026-08-09T22:00:00+05:30', o: '330'}));

    expect(payload!.offsetMinutes).toBe(330);
    expect(payload!.zoneLabel).toBe('+05:30');
});

// These params come off a URL somebody could have hand-edited.
const badIso: Array<[string, Record<string, string>]> = [
    ['a canonical that is not normalised', {dtg: '2026-08-09T20:30+04:00'}],
    ['a canonical with a fraction', {dtg: '2026-08-09T20:30:00.5+04:00'}],
    ['a date-time group in the canonical', {dtg: '091630ZAUG26'}],
    ['an offset that is not a number', {o: 'banana'}],
    ['an offset no place uses', {o: '1200'}],
    ['a fractional offset', {o: '240.5'}],
    ['an assumed flag a timestamp cannot have', {a: 'my'}],
];

for (const [name, overrides] of badIso) {
    test(`rejects ${name}`, () => {
        expect(fromParams(isoParams(overrides))).toBeNull();
    });
}

// Exactly one of the two says which form this is. Both, or neither, is not a
// link the server wrote.
test('rejects a link carrying both a zone letter and an offset', () => {
    expect(fromParams(isoParams({z: 'Z'}))).toBeNull();
});

test('rejects a link carrying neither', () => {
    const params = isoParams();
    params.delete('o');

    expect(fromParams(params)).toBeNull();
});

// Links already written into messages carry z and no o, and have to keep
// working exactly as they did.
test('still reads a date-time group link with no offset param', () => {
    const payload = fromParams(params());

    expect(payload).not.toBeNull();
    expect(payload!.offsetMinutes).toBe(0);
    expect(payload!.zoneLabel).toBe('Z');
});
