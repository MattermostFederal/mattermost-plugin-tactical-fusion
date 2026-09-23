import {expect, test} from '@playwright/test';

import {OUTSIDE_BANDS} from './bands';

import decorator, {MAX_KHZ, MIN_KHZ, describe, fromParams, parseToken} from './index';

test('the type is the path segment the server routes', () => {
    expect(decorator.type).toBe('frequency');
    expect(decorator.Hover).toBeDefined();
    expect(decorator.postType).toBeUndefined();
});

test('normalizes every spelling to kilohertz', () => {
    for (const [token, khz] of [
        ['121.5', 121500],
        ['118.3', 118300],
        ['118.300', 118300],
        ['118.300 MHZ', 118300],
        ['118.300MHZ', 118300],
        ['8992', 8992],
        ['8992 KHZ', 8992],
        ['1300.0', MAX_KHZ],
        ['2.0', MIN_KHZ],
    ] as const) {
        expect(parseToken(token), token).toEqual({token, khz});
    }
});

test('refuses what the server refuses', () => {
    for (const token of ['', '1.999', '1300.001', '1999', '1300001', '121.5 KHZ', '8992 MHZ', ' 121.5', '121,5', '121.5 GHZ', '121.5678']) {
        expect(parseToken(token), token).toBeNull();
    }
    expect(fromParams(new URLSearchParams({}))).toBeNull();
    expect(fromParams(new URLSearchParams({v: '121.5'}))).toEqual({token: '121.5', khz: 121500});
});

test('describes the band, the channel and the use the way the server does', () => {
    expect(describe({token: '121.5', khz: 121500})).toEqual({
        token: '121.5', mhz: '121.500', khz: '121500', band: 'VHF air band', channel: '25 kHz channel', use: 'Aeronautical emergency',
    });
    expect(describe({token: '118.305', khz: 118305})).toMatchObject({band: 'VHF air band', channel: '8.33 kHz channel', use: ''});
    expect(describe({token: '8992 KHZ', khz: 8992})).toMatchObject({mhz: '8.992', band: 'HF aeronautical', channel: ''});
    expect(describe({token: '1090.0', khz: 1090000})).toMatchObject({band: OUTSIDE_BANDS, channel: '', use: ''});
    expect(describe({token: '243.0', khz: 243000})).toMatchObject({band: 'UHF military air band', use: 'UHF military emergency'});
});
