import {expect, test} from '@playwright/test';

import {_geometryForTesting as geometryForTesting} from './CotMap';
import {SUMMARY_MAX_RUNES, TOO_LONG, oneLine} from './summary';
import {AFFILIATION_COLORS, COT_CLASSES, COT_PROPS_KEY, accuracyMeters, affiliationWord, fromProps, isLinkable, staleAfterPosting, statedColor} from './types';
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

test('a version 1 blob still reads, and simply carries no extensions', () => {
    const payload = fromProps(props({}, {callsign: 'DELTA1'}));

    expect(payload?.events[0].cotClass).toBe('');
    expect(payload?.events[0].detailUnknown).toBe('');
    expect(payload?.events[0].flow).toEqual([]);
    expect(payload?.events[0].detail.takvPlatform).toBe('');
});

test('reads the registry keys the server writes', () => {
    const payload = fromProps(props({}, {
        takv_platform: 'ATAK-CIV',
        status_battery: '87%',
        attitude_yaw: '183.5°',
        chatgrp_uid0: 'A',
        medevac_urgent: '0',
    }));

    const {detail} = payload!.events[0];
    expect(detail.takvPlatform).toBe('ATAK-CIV');
    expect(detail.statusBattery).toBe('87%');
    expect(detail.attitudeYaw).toBe('183.5°');
    expect(detail.chatgrpUid0).toBe('A');
    expect(detail.medevacUrgent).toBe('0');
});

test('a class this build does not know falls to the default layout', () => {
    for (const value of ['teleport', '', 42, null, 'CHAT']) {
        const payload = fromProps(props({}, {class: value}));
        expect(payload?.events[0].cotClass, String(value)).toBe('');
    }
});

test('every class the server writes is one this build lays out', () => {
    for (const value of COT_CLASSES) {
        const payload = fromProps(props({}, {class: value}));
        expect(payload?.events[0].cotClass, value).toBe(value);
    }
});

// The ordering IS the processing path, so it survives the trip through JSON.
test('the processing path keeps its order', () => {
    const payload = fromProps(props({}, {
        flow: [
            {system: 'alpha', time: 'T1'},
            {system: 'bravo', time: 'T2'},
        ],
    }));

    expect(payload?.events[0].flow.map((hop) => hop.system)).toEqual(['alpha', 'bravo']);
});

test('a malformed processing path reads as none rather than throwing', () => {
    for (const value of ['nope', 42, null, {system: 'a'}]) {
        const payload = fromProps(props({}, {flow: value}));
        expect(payload?.events[0].flow, String(value)).toEqual([]);
    }
});

test('a hop with no system is dropped and the rest survive', () => {
    const payload = fromProps(props({}, {
        flow: [{time: 'T1'}, {system: 'bravo', time: 'T2'}, 'nope'],
    }));

    expect(payload?.events[0].flow).toEqual([{system: 'bravo', time: 'T2'}]);
});

// React sets style values through setProperty without sanitising them, so this
// is the last gate before an author-derived string reaches one.
test('only a hex triple is offered as a colour', () => {
    const accepted = ['#ff0000', '#FF0000', '#0a0b0c'];
    const refused = ['url(https://attacker.example/px)', 'red', '#f00', '#ff00000', '', 'expression(1)'];

    for (const value of accepted) {
        const payload = fromProps(props({}, {color_argb: value}));
        expect(statedColor(payload!.events[0]), value).toBe(value);
    }

    for (const value of refused) {
        const payload = fromProps(props({}, {color_argb: value}));
        expect(statedColor(payload!.events[0]), value).toBeUndefined();
    }
});

// The sync guard checks that every key is read SOMEWHERE. It cannot see a
// transposition: `takvDevice: text(event,'takv_os')` type-checks, satisfies the
// guard, and mislabels an operator-facing row forever. This is what sees it.
const DETAIL_KEYS = [
    'archive', 'attitude_pitch', 'attitude_roll', 'attitude_yaw',
    'chat_group_owner', 'chat_id', 'chat_parent', 'chat_room', 'chat_sender',
    'chatgrp_id', 'chatgrp_uid0', 'chatgrp_uid1', 'color_argb', 'contact_endpoint',
    'medevac_ambulatory', 'medevac_casevac', 'medevac_equipment_detail',
    'medevac_equipment_none', 'medevac_freq', 'medevac_hlz_marking', 'medevac_litter',
    'medevac_medline_remarks', 'medevac_nationality', 'medevac_nbc', 'medevac_priority',
    'medevac_routine', 'medevac_security', 'medevac_terrain_none', 'medevac_title',
    'medevac_urgent', 'medevac_zone_prot_selection', 'precision_altsrc',
    'precision_geopointsrc', 'precision_hdop', 'precision_pdop', 'precision_vdop',
    'route_direction', 'route_method', 'route_order', 'route_planning', 'route_type',
    'sensor_azimuth', 'sensor_elevation', 'sensor_fov', 'sensor_model', 'sensor_range',
    'sensor_roll', 'sensor_vfov', 'status_battery', 'status_readiness', 'takv_device',
    'takv_os', 'takv_platform', 'takv_version', 'track_slope', 'uid_extra_droid',
    'usericon_iconsetpath', 'video_conn_address', 'video_conn_path', 'video_conn_port',
    'video_conn_protocol', 'video_uid', 'video_url',
];

function camel(key: string): string {
    const [head, ...rest] = key.split('_');
    return head + rest.map((part) => part.charAt(0).toUpperCase() + part.slice(1)).join('');
}

test('every registry key lands on the field named after it', () => {
    const blob: Record<string, unknown> = {};
    for (const key of DETAIL_KEYS) {
        blob[key] = `sentinel-${key}`;
    }

    const detail = fromProps(props({}, blob))!.events[0].detail as unknown as Record<string, string>;

    for (const key of DETAIL_KEYS) {
        expect(detail[camel(key)], key).toBe(`sentinel-${key}`);
    }

    // And nothing else: a field reading two keys would leave one unread.
    expect(Object.keys(detail).sort()).toEqual(DETAIL_KEYS.map(camel).sort());
});

// The cap is what makes "one line" reviewable. Popping to empty returned '',
// which made the caller render nothing: the line the cap exists to bound
// disappeared entirely.
test('the summary drops trailing readings rather than the whole line', () => {
    expect(oneLine(['a', 'b', 'c'])).toBe('a · b · c');
    expect(oneLine([])).toBe('');

    const long = 'x'.repeat(80);
    const trimmed = oneLine([long, 'second', 'third']);
    expect(trimmed).toContain(long);
    expect(trimmed).not.toContain('third');
    expect([...trimmed].length).toBeLessThanOrEqual(SUMMARY_MAX_RUNES);
});

// Neither blank nor clipped. Popping to empty erased the line the cap exists to
// bound; clipping put 90 runes of an author's value under a casualty label with
// the word itself cut off, which is not a reading of anything.
test('a single overlong reading points at the panel rather than being clipped', () => {
    const line = oneLine(['y'.repeat(SUMMARY_MAX_RUNES + 40) + ' urgent']);

    expect(line).toBe(TOO_LONG);
    expect(line).not.toContain('yyy');
    expect([...line].length).toBeLessThanOrEqual(SUMMARY_MAX_RUNES);
});

test('nothing stated still renders nothing', () => {
    expect(oneLine([])).toBe('');
    expect(oneLine(['', ''])).toBe('');
});

test('reads a drawn outline, and keeps its vertex order', () => {
    const payload = fromProps(props({}, {
        geometry: {
            kind: 'polyline',
            closed: 'stated',
            count: '3',
            points: [
                {lat: '1.0000', lon: '10.0000'},
                {lat: '2.0000', lon: '20.0000'},
                {lat: '3.0000', lon: '30.0000'},
            ],
        },
    }));

    const geometry = payload!.events[0].geometry!;
    expect(geometry.kind).toBe('polyline');
    expect(geometry.closed).toBe(true);
    expect(geometry.points.map((p) => p.lat)).toEqual([1, 2, 3]);
});

test('reads an ellipse, which carries axes rather than points', () => {
    const payload = fromProps(props({}, {
        geometry: {kind: 'ellipse', major: '100 m', minor: '50 m', angle: '-45°'},
    }));

    const geometry = payload!.events[0].geometry!;
    expect(geometry.kind).toBe('ellipse');
    expect(geometry.major).toBe('100 m');
    expect(geometry.points).toEqual([]);
});

// The server refuses a shape it will not stand behind and says so in `note`.
// This is the same refusal on the side that draws.
test('a shape left with fewer than two usable points is not a shape', () => {
    const payload = fromProps(props({}, {
        geometry: {kind: 'polyline', points: [{lat: '1.0000', lon: '2.0000'}, {lat: 'north', lon: 'x'}]},
    }));

    expect(payload!.events[0].geometry!.points).toEqual([]);
});

test('a malformed geometry reads as none rather than throwing', () => {
    for (const value of ['nope', 42, null, [], {kind: ''}, {points: []}]) {
        const payload = fromProps(props({}, {geometry: value}));
        expect(payload?.events[0].geometry, JSON.stringify(value)).toBeNull();
    }
});

test('an event that describes no shape carries none', () => {
    expect(fromProps(props({}, {}))!.events[0].geometry).toBeNull();
});

test.describe('what the map is asked to draw', () => {
    function geometryOf(overrides: Record<string, unknown>) {
        const payload = fromProps(props({}, {geometry: overrides}));
        return geometryForTesting(payload!.events[0]);
    }

    test('an ellipse is taken from the raw numbers, not from the rendered strings', () => {
        const drawn = geometryOf({
            kind: 'ellipse',
            major: '400 m',
minor: '250 m',
angle: '30°',
            major_m: '400',
minor_m: '250',
angle_deg: '30',
        });

        expect(drawn).toEqual({kind: 'ellipse', major: 400, minor: 250, angle: 30});
    });

    // The server refuses a shape it will not stand behind. Drawing one anyway
    // made the card say "not drawn" over a shape the map had drawn.
    test('a shape the server refused is not drawn', () => {
        const drawn = geometryOf({
            kind: 'ellipse',
major_m: '400',
minor_m: '250',
            note: 'A point in this shape is not one this build will stand behind.',
        });

        expect(drawn).toBeUndefined();
    });

    test('an ellipse whose axes did not survive is not drawn', () => {
        expect(geometryOf({kind: 'ellipse', major: '400 m'})).toBeUndefined();
        expect(geometryOf({kind: 'ellipse', major_m: '0', minor_m: '250'})).toBeUndefined();
    });

    test('an outline is drawn from its points', () => {
        const drawn = geometryOf({
            kind: 'polyline',
closed: 'stated',
            points: [{lat: '1.0000', lon: '2.0000'}, {lat: '3.0000', lon: '4.0000'}],
        });

        expect(drawn).toEqual({kind: 'outline', points: [{lat: 1, lon: 2}, {lat: 3, lon: 4}], closed: true});
    });

    test('an event with no shape asks for nothing', () => {
        expect(geometryForTesting(undefined)).toBeUndefined();
        expect(geometryForTesting(fromProps(props({}, {}))!.events[0])).toBeUndefined();
    });
});

// One bad vertex refuses the whole shape, which is the rule the server applies.
// Dropping the corner drew a different polygon as fact.
test('one unusable vertex refuses the whole outline', () => {
    for (const bad of [{lat: 'north', lon: '2'}, {lat: '95', lon: '2'}, {lat: '1', lon: '181'}, 'nope']) {
        const payload = fromProps(props({}, {
            geometry: {kind: 'polyline', points: [{lat: '1.0000', lon: '2.0000'}, bad, {lat: '3.0000', lon: '4.0000'}]},
        }));

        expect(payload!.events[0].geometry!.points, JSON.stringify(bad)).toEqual([]);
    }
});

test('a forged vertex list is capped at what the server can write', () => {
    const points = Array.from({length: 600}, (_, i) => ({lat: '1.0000', lon: `${i % 100}.0000`}));
    const payload = fromProps(props({}, {geometry: {kind: 'polyline', points}}));

    expect(payload!.events[0].geometry!.points.length).toBeLessThanOrEqual(512);
});
