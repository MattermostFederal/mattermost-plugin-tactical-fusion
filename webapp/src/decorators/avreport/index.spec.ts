import {expect, test} from '@playwright/test';

import decorator, {MAX_SOURCE_RUNES, fromParams} from './index';

const METAR = 'METAR PHNL 221651Z 07012G18KT 10SM FEW025 SCT045 27/19 A3010';

test('the type is the path segment the server routes', () => {
    expect(decorator.type).toBe('avreport');
    expect(decorator.Hover).toBeDefined();
    expect(decorator.postType).toBeUndefined();
});

test('reads the report and its instant', () => {
    expect(fromParams(new URLSearchParams({v: METAR, t: '1790095860000'}))).toEqual({v: METAR, t: '1790095860000'});
    expect(fromParams(new URLSearchParams({v: '!JFK 09/001 JFK TWY A CLSD', t: '0'}))).toEqual({v: '!JFK 09/001 JFK TWY A CLSD', t: '0'});
});

test('refuses a link missing either half', () => {
    expect(fromParams(new URLSearchParams({v: METAR}))).toBeNull();
    expect(fromParams(new URLSearchParams({t: '1790095860000'}))).toBeNull();
    expect(fromParams(new URLSearchParams({v: '', t: '1'}))).toBeNull();
});

test('refuses an instant that is not a number', () => {
    expect(fromParams(new URLSearchParams({v: METAR, t: 'soon'}))).toBeNull();
    expect(fromParams(new URLSearchParams({v: METAR, t: '1.5'}))).toBeNull();
    expect(fromParams(new URLSearchParams({v: METAR, t: '9'.repeat(18)}))).toBeNull();
});

test('refuses what the server would refuse: an over-long report', () => {
    expect(fromParams(new URLSearchParams({v: `${METAR}\nRMK NONE`, t: '1'}))).toEqual({v: `${METAR}\nRMK NONE`, t: '1'});
    expect(fromParams(new URLSearchParams({v: 'x'.repeat(MAX_SOURCE_RUNES + 1), t: '1'}))).toBeNull();
    expect(fromParams(new URLSearchParams({v: 'x'.repeat(MAX_SOURCE_RUNES), t: '1'}))).not.toBeNull();
});
