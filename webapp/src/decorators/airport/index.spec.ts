import {expect, test} from '@playwright/test';

import decorator, {PANEL_TITLE, fromParams} from './index';

function params(entries: Record<string, string>): URLSearchParams {
    return new URLSearchParams(entries);
}

test.describe('fromParams', () => {
    test('accepts what the server produces', () => {
        const payload = fromParams(params({v: 'KIND'}));

        expect(payload).not.toBeNull();
        expect(payload?.ident).toBe('KIND');
    });

    test('reads only v, so a stray parameter changes nothing', () => {
        const payload = fromParams(params({v: 'KIND', f: 'dd', r: 'KIND'}));

        expect(payload?.ident).toBe('KIND');
    });

    test('rejects each mutation of a valid link', () => {
        const cases: Array<[string, Record<string, string>]> = [
            ['missing value', {}],
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

    test('refuses any case but upper, which is what makes one airfield one URL', () => {
        for (const ident of ['kind', 'Kind', 'kIND', 'KINd']) {
            expect(fromParams(params({v: ident})), ident).toBeNull();
        }
    });

    test('accepts a well formed code this build may not hold, leaving that to the server', () => {
        expect(fromParams(params({v: 'QZQZ'}))?.ident).toBe('QZQZ');
    });
});

test.describe('the decorator', () => {
    test('claims the airport type', () => {
        expect(decorator.type).toBe('airport');
        expect(decorator.fromParams).toBe(fromParams);
    });

    test('heads the sidebar with the panel title', () => {
        expect(decorator.summary({ident: 'KIND'})).toBe(PANEL_TITLE);
    });

    test('declares a hover', () => {
        expect(decorator.Hover).toBeTruthy();
    });
});
