import {expect, test} from '@playwright/test';

import decorator, {PANEL_TITLE, fromParams} from './index';

function params(entries: Record<string, string>): URLSearchParams {
    return new URLSearchParams(entries);
}

test.describe('fromParams', () => {
    test('accepts an ICAO ident the server produces', () => {
        expect(fromParams(params({v: 'KIND'}))).toEqual({key: 'icao', code: 'KIND'});
    });

    test('accepts an IATA code the server produces', () => {
        expect(fromParams(params({i: 'HNL'}))).toEqual({key: 'iata', code: 'HNL'});
    });

    test('reads only v and i, so a stray parameter changes nothing', () => {
        expect(fromParams(params({v: 'KIND', f: 'dd', r: 'KIND'}))).toEqual({key: 'icao', code: 'KIND'});
    });

    test('refuses a link naming both codes or neither', () => {
        expect(fromParams(params({v: 'KIND', i: 'IND'}))).toBeNull();
        expect(fromParams(params({}))).toBeNull();
        expect(fromParams(params({x: 'KIND'}))).toBeNull();
    });

    test('rejects each mutation of a valid ident link', () => {
        const cases: Array<[string, Record<string, string>]> = [
            ['empty value', {v: ''}],
            ['three letters', {v: 'KIN'}],
            ['five letters', {v: 'KINDX'}],
            ['a digit', {v: 'K1ND'}],
            ['the USMTF terminator', {v: 'KIND//'}],
            ['a leading space', {v: ' KIND'}],
            ['a trailing space', {v: 'KIND '}],
            ['an inner space', {v: 'KI ND'}],
            ['a newline', {v: 'KIND\n'}],
            ['non-ASCII', {v: 'KINƉ'}],
        ];

        for (const [name, entries] of cases) {
            expect(fromParams(params(entries)), name).toBeNull();
        }
    });

    test('rejects each mutation of a valid IATA link', () => {
        for (const [name, code] of [['empty', ''], ['two letters', 'HN'], ['four letters', 'HNLL'], ['a digit', 'H1L'], ['lower case', 'hnl'], ['a space', 'HN L']]) {
            expect(fromParams(params({i: code})), name).toBeNull();
        }
    });

    test('refuses any case but upper, which is what makes one airfield one URL', () => {
        for (const ident of ['kind', 'Kind', 'kIND', 'KINd']) {
            expect(fromParams(params({v: ident})), ident).toBeNull();
        }
    });

    test('accepts a well formed code this build may not hold, leaving that to the server', () => {
        expect(fromParams(params({v: 'QZQZ'}))?.code).toBe('QZQZ');
        expect(fromParams(params({i: 'QQQ'}))?.code).toBe('QQQ');
    });
});

test.describe('the decorator', () => {
    test('claims the airport type', () => {
        expect(decorator.type).toBe('airport');
        expect(decorator.fromParams).toBe(fromParams);
    });

    test('heads the sidebar with the panel title', () => {
        expect(decorator.summary({key: 'icao', code: 'KIND'})).toBe(PANEL_TITLE);
    });

    test('declares a hover', () => {
        expect(decorator.Hover).toBeTruthy();
    });
});
