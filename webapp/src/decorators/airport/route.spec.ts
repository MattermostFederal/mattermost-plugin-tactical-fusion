import {expect, test} from '@playwright/test';

import {
    AIRFIELDS_POST_TYPE,
    AIRFIELDS_PROPS_KEY,
    AIRFIELDS_PROPS_VERSION,
    MAX_ROUTE_AIRFIELDS,
    airfieldsFromProps,
    drawsNothing,
    payloadFor,
    routeLabel,
    routeLegs,
    routeMarkers,
} from './route';

const HICKAM = {ident: 'PHIK', code: 'PHIK', name: 'Hickam Air Force Base', format: 'dd', value: '21.3353,-157.9483'};
const HONOLULU = {ident: 'PHNL', code: 'HNL', name: 'Daniel K. Inouye International Airport', format: 'dd', value: '21.3184,-157.9257'};
const ANDERSEN = {ident: 'PGUA', code: 'PGUA', name: 'Andersen Air Force Base', format: 'dd', value: '13.5840,144.9300'};
const NOWHERE = {ident: 'QZQZ', code: 'QZQZ', name: 'Somewhere Unplaced', format: '', value: ''};

function props(airfields: unknown[], version: unknown = AIRFIELDS_PROPS_VERSION): Record<string, unknown> {
    return {[AIRFIELDS_PROPS_KEY]: {version, airfields}};
}

test('the constants are the ones the server writes', () => {
    expect(AIRFIELDS_POST_TYPE).toBe('custom_tf_airfields');
    expect(AIRFIELDS_PROPS_KEY).toBe('tactical_fusion_airfields');
    expect(MAX_ROUTE_AIRFIELDS).toBe(64);
});

test.describe('airfieldsFromProps', () => {
    test('reads what the server writes, in order', () => {
        const payload = airfieldsFromProps(props([HICKAM, HONOLULU, ANDERSEN]));

        expect(payload).not.toBeNull();
        expect(payload?.airfields.map((a) => a.ident)).toEqual(['PHIK', 'PHNL', 'PGUA']);
        expect(payload?.airfields[1].code).toBe('HNL');
    });

    test('keeps an entry with no position, so a leg can stop there', () => {
        const payload = airfieldsFromProps(props([HICKAM, NOWHERE]));
        expect(payload?.airfields[1].format).toBe('');
    });

    test('refuses a blob that is not the shape', () => {
        for (const blob of [
            null,
            undefined,
            'PHIK',
            {},
            {[AIRFIELDS_PROPS_KEY]: 'x'},
            {[AIRFIELDS_PROPS_KEY]: []},
            props([]),
            props([HICKAM], 2),
            props([HICKAM], '1'),
            props([null]),
            props([{...HICKAM, ident: 'phik'}]),
            props([{...HICKAM, name: 4}]),
            props([{...HICKAM, code: undefined}]),
            props([{...HICKAM, format: 'dd', value: ''}]),
            props([{...HICKAM, format: '', value: '1,2'}]),
            {[AIRFIELDS_PROPS_KEY]: {version: 1, airfields: 'PHIK'}},
        ]) {
            expect(airfieldsFromProps(blob), JSON.stringify(blob)).toBeNull();
        }
    });

    test('refuses a route past the cap rather than slicing it', () => {
        const many = Array.from({length: MAX_ROUTE_AIRFIELDS + 1}, () => HICKAM);
        expect(airfieldsFromProps(props(many))).toBeNull();
        expect(airfieldsFromProps(props(many.slice(0, MAX_ROUTE_AIRFIELDS)))).not.toBeNull();
    });
});

test.describe('the drawing', () => {
    test('marks every placed airfield and joins consecutive ones', () => {
        const payload = {airfields: [HICKAM, HONOLULU, ANDERSEN], postId: ''};

        expect(routeMarkers(payload)).toHaveLength(3);
        const legs = routeLegs(payload);
        expect(legs).toHaveLength(1);
        expect(legs[0].rings[0]).toHaveLength(3);
        expect(legs[0].closed).toBe(false);
        expect(routeLabel(payload)).toBe('3 airfields and 2 legs in message order');
    });

    test('never draws a leg across an airfield that did not place', () => {
        const payload = {airfields: [HICKAM, NOWHERE, ANDERSEN], postId: ''};

        expect(routeMarkers(payload)).toHaveLength(2);
        expect(routeLegs(payload)).toHaveLength(0);
        expect(routeLabel(payload)).toBe('2 airfields');
    });

    test('a coordinate this build cannot read places nothing', () => {
        const payload = {airfields: [{...HICKAM, value: 'nowhere'}], postId: ''};

        expect(routeMarkers(payload)).toHaveLength(0);
        expect(drawsNothing(payload)).toBe(true);
    });

    test('one airfield is a marker and no leg', () => {
        const payload = {airfields: [HICKAM], postId: ''};

        expect(routeMarkers(payload)).toHaveLength(1);
        expect(routeLegs(payload)).toHaveLength(0);
        expect(routeLabel(payload)).toBe('1 airfield');
        expect(drawsNothing(payload)).toBe(false);
    });
});

test('payloadFor opens an IATA entry by its code and an ICAO entry by its ident', () => {
    expect(payloadFor(HONOLULU)).toEqual({key: 'iata', code: 'HNL'});
    expect(payloadFor(HICKAM)).toEqual({key: 'icao', code: 'PHIK'});
});
