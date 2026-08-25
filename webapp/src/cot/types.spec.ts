import {expect, test} from '@playwright/test';

import {AFFILIATION_COLORS, COT_PROPS_KEY, accuracyMeters, affiliationWord, fromProps, isLinkable, staleAfterPosting} from './types';
import type {CotEvent} from './types';

function props(overrides: Record<string, unknown> = {}, event: Record<string, unknown> = {}) {
    return {
        [COT_PROPS_KEY]: {
            version: 1,
            source: 'fence',
            lead: '',
            trail: '',
            src: '<event/>',
            event: {uid: 'ANDROID-1', cot_type: 'a-f-G-U-C', ...event},
            ...overrides,
        },
    };
}

test('reads a well formed blob', () => {
    const payload = fromProps(props({lead: 'note'}, {callsign: 'DELTA1', value: '1.0000,2.0000', format: 'dd'}));

    expect(payload).not.toBeNull();
    expect(payload?.lead).toBe('note');
    expect(payload?.events).toHaveLength(1);
    expect(payload?.events[0].callsign).toBe('DELTA1');
    expect(payload?.events[0].cotType).toBe('a-f-G-U-C');
});

test('refuses anything that is not this plugin s blob', () => {
    const cases: Record<string, unknown> = {
        'not an object': 'nope',
        null: null,
        'an array': [],
        'no blob': {other: {version: 1}},
        'blob is not an object': {[COT_PROPS_KEY]: 'nope'},
    };

    for (const [name, value] of Object.entries(cases)) {
        expect(fromProps(value), name).toBeNull();
    }
});

test('refuses a version it does not know', () => {
    expect(fromProps(props({version: 3}))).toBeNull();
    expect(fromProps(props({version: 'one'}))).toBeNull();
    expect(fromProps(props({version: undefined}))).toBeNull();
});

test('refuses a blob with no event or no uid', () => {
    expect(fromProps(props({event: undefined}))).toBeNull();
    expect(fromProps(props({event: 'nope'}))).toBeNull();
    expect(fromProps(props({}, {uid: ''}))).toBeNull();
    expect(fromProps(props({}, {uid: 42}))).toBeNull();
});

test('refuses a source it does not know', () => {
    expect(fromProps(props({source: 'somewhere'}))).toBeNull();
    expect(fromProps(props({source: ''}))).toBeNull();
});

test('reads an absent field as empty rather than throwing', () => {
    const payload = fromProps(props());

    expect(payload?.events[0].callsign).toBe('');
    expect(payload?.events[0].positionNote).toBe('');
});

test('a position is linkable only when both halves of its identity are present', () => {
    const base = {format: '', value: ''} as CotEvent;

    expect(isLinkable({...base})).toBe(false);
    expect(isLinkable({...base, format: 'dd'})).toBe(false);
    expect(isLinkable({...base, value: '1.0,2.0'})).toBe(false);
    expect(isLinkable({...base, format: 'dd', value: '1.0000,2.0000'})).toBe(true);
});

test('an unstated or nonsense accuracy draws no circle', () => {
    const base = {ceMeters: ''} as CotEvent;

    expect(accuracyMeters({...base})).toBeUndefined();
    expect(accuracyMeters({...base, ceMeters: '0'})).toBeUndefined();
    expect(accuracyMeters({...base, ceMeters: '-5'})).toBeUndefined();
    expect(accuracyMeters({...base, ceMeters: 'wat'})).toBeUndefined();
    expect(accuracyMeters({...base, ceMeters: '45.3'})).toBe(45.3);
});

test('the freshness reading uses two server side values and never the local clock', () => {
    const createAt = 1_700_000_000_000;
    const event = {staleAt: String(createAt + 120_000)} as CotEvent;

    expect(staleAfterPosting(event, createAt)).toBe('stale 2m after posting');
    expect(staleAfterPosting({staleAt: String(createAt + 45_000)} as CotEvent, createAt)).
        toBe('stale 45s after posting');
    expect(staleAfterPosting({staleAt: String(createAt + 5_400_000)} as CotEvent, createAt)).
        toBe('stale 1h 30m after posting');
});

test('an event already stale when it was posted says so', () => {
    const createAt = 1_700_000_000_000;

    expect(staleAfterPosting({staleAt: String(createAt - 1000)} as CotEvent, createAt)).
        toBe('already stale when it was posted');
});

test('the freshness reading stands down when either value is missing', () => {
    expect(staleAfterPosting({staleAt: ''} as CotEvent, 1_700_000_000_000)).toBe('');
    expect(staleAfterPosting({staleAt: 'wat'} as CotEvent, 1_700_000_000_000)).toBe('');
    expect(staleAfterPosting({staleAt: '1700000000000'} as CotEvent, 0)).toBe('');
});

test('reads the events array a version 2 blob carries', () => {
    const payload = fromProps({
        [COT_PROPS_KEY]: {
            version: 2,
            source: 'fence',
            events: [{uid: 'a', callsign: 'ALPHA'}, {uid: 'b', callsign: 'BRAVO'}],
        },
    });

    expect(payload?.events).toHaveLength(2);
    expect(payload?.events[0].callsign).toBe('ALPHA');
    expect(payload?.events[1].callsign).toBe('BRAVO');
});

// Posts stamped before the array existed are still out there and still render.
test('reads the single event a version 1 blob carries', () => {
    const payload = fromProps({
        [COT_PROPS_KEY]: {
            version: 1,
            source: 'fence',
            event: {uid: 'a', callsign: 'ALPHA'},
        },
    });

    expect(payload?.events).toHaveLength(1);
    expect(payload?.events[0].callsign).toBe('ALPHA');
});

test('refuses a version neither shape knows', () => {
    for (const version of [0, 3, 99, 'two']) {
        expect(fromProps({
            [COT_PROPS_KEY]: {version, source: 'fence', events: [{uid: 'a'}]},
        }), String(version)).toBeNull();
    }
});

test('refuses a blob carrying no events at all', () => {
    expect(fromProps({[COT_PROPS_KEY]: {version: 2, source: 'fence', events: []}})).toBeNull();
    expect(fromProps({[COT_PROPS_KEY]: {version: 2, source: 'fence'}})).toBeNull();
});

// One unreadable event spoils the blob, for the reason the parser refuses the
// source: a list showing two of three is a list that is wrong about the post.
test('refuses the blob when any event is unreadable', () => {
    expect(fromProps({
        [COT_PROPS_KEY]: {
            version: 2,
            source: 'fence',
            events: [{uid: 'a'}, {callsign: 'no uid'}],
        },
    })).toBeNull();
});

test('reads the relations an event declares', () => {
    const payload = fromProps({
        [COT_PROPS_KEY]: {
            version: 2,
            source: 'fence',
            events: [{uid: 'a', parent: 'ALPHA', related: 'ANDROID-9'}],
        },
    });

    expect(payload?.events[0].parent).toBe('ALPHA');
    expect(payload?.events[0].related).toBe('ANDROID-9');
});

/*
 * Every affiliation this build colours can also be NAMED.
 *
 * The two tables exist for one reason between them: colour is never the only
 * channel. A map's accessible label is the surface where the colour is worth
 * nothing, so an affiliation with a colour and no word would be a marker a
 * screen reader cannot tell from any other on the one surface where telling
 * them apart is the whole job.
 */
test('every affiliation with a colour has a word', () => {
    const ids = Object.keys(AFFILIATION_COLORS);
    expect(ids.length).toBeGreaterThan(0);

    for (const affiliation of ids) {
        const word = affiliationWord({affiliation} as CotEvent);
        expect(word, `${affiliation} has a colour but no word`).not.toBe('unstated');
        expect(word).not.toBe('');
    }
});

test('an affiliation this build does not know is named rather than dropped', () => {
    expect(affiliationWord({affiliation: 'zzz'} as CotEvent)).toBe('unstated');
    expect(affiliationWord({affiliation: ''} as CotEvent)).toBe('unstated');
});
