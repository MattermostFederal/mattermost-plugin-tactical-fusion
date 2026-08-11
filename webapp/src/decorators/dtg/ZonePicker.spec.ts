import {expect, test} from '@playwright/test';

import {optionLabel} from './ZonePicker';
import type {ZoneChoice} from './zones';

// optionLabel is pure and needs no DOM, so it is covered here rather than
// through the picker. How the label reads is a contract: several bases can sit
// behind one identifier, and the row has to say which one it is.

function choice(name: string, iana: string): ZoneChoice {
    return {name, iana, abbr: '', key: `${iana}|${name}`, offsetMinutes: 0, offsetLabel: 'UTC+00:00'};
}

// The name leads when there is one worth leading with, which for this audience
// is the base rather than the city.
test('a named base is followed by its identifier', () => {
    expect(optionLabel(choice('Ramstein', 'Europe/Berlin'))).toBe('Ramstein (Europe/Berlin)');
});

// Two bases sharing a zone are told apart by their names, which is the whole
// reason the identifier is not enough on its own.
test('two bases in one zone read differently', () => {
    const ramstein = optionLabel(choice('Ramstein', 'Europe/Berlin'));
    const stuttgart = optionLabel(choice('Stuttgart', 'Europe/Berlin'));

    expect(ramstein).not.toBe(stuttgart);
    expect(stuttgart).toBe('Stuttgart (Europe/Berlin)');
});

test('a name already carrying its identifier is left alone', () => {
    expect(optionLabel(choice('Zulu (UTC)', 'UTC'))).toBe('Zulu (UTC)');
});

// An unnamed zone would otherwise read "Europe/Paris (Europe/Paris)".
test('a name equal to the identifier says it once', () => {
    expect(optionLabel(choice('Europe/Paris', 'Europe/Paris'))).toBe('Europe/Paris');
});

// A zone named after its own last segment adds nothing the identifier does not
// already say, so the identifier wins.
test('a name equal to the city is dropped', () => {
    expect(optionLabel(choice('Paris', 'Europe/Paris'))).toBe('Europe/Paris');
});

// The city is the last segment with its underscores opened out, so the
// comparison has to survive that.
test('an underscored city still matches its identifier', () => {
    expect(optionLabel(choice('Los Angeles', 'America/Los_Angeles'))).toBe('America/Los_Angeles');
});

test('a multi-segment identifier compares on its last segment only', () => {
    expect(optionLabel(choice('Buenos Aires', 'America/Argentina/Buenos_Aires'))).
        toBe('America/Argentina/Buenos_Aires');

    // A different name still leads.
    expect(optionLabel(choice('Ushuaia', 'America/Argentina/Buenos_Aires'))).
        toBe('Ushuaia (America/Argentina/Buenos_Aires)');
});

// An identifier with no separator is its own city, so the same rule applies.
test('an identifier with no separator is its own city', () => {
    expect(optionLabel(choice('UTC', 'UTC'))).toBe('UTC');
    expect(optionLabel(choice('Coordinated', 'UTC'))).toBe('Coordinated (UTC)');
});

// A blob written before names existed holds a bare identifier, which reads as
// an unnamed zone. Nothing should invent a name for it.
test('an unnamed zone reads as its identifier', () => {
    expect(optionLabel(choice('Berlin', 'Europe/Berlin'))).toBe('Europe/Berlin');
});
