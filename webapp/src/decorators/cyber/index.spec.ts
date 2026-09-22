import {expect, test} from '@playwright/test';

import {KINDS, matchesShape} from './cyber';

import decorator, {PANEL_TITLE, fromParams} from './index';

function params(entries: Record<string, string>): URLSearchParams {
    return new URLSearchParams(entries);
}

test.describe('fromParams', () => {
    test('accepts what the server produces for every kind', () => {
        const cases: Array<[string, string]> = [
            ['cve', 'CVE-2021-44228'],
            ['cwe', 'CWE-79'],
            ['attack', 'T1059.001'],
            ['attack', 'TA0002'],
            ['ip', '203.0.113.7'],
            ['ip', '2001:db8::1'],
            ['hash', '44d88612fea8a8f36de82e1278abb02f'],
        ];

        for (const [kind, value] of cases) {
            const payload = fromParams(params({k: kind, v: value}));
            expect(payload, `${kind} ${value}`).not.toBeNull();
            expect(payload?.kind).toBe(kind);
            expect(payload?.value).toBe(value);
        }
    });

    test('rejects each mutation of a valid link', () => {
        const cases: Array<[string, Record<string, string>]> = [
            ['no kind', {v: 'CVE-2021-44228'}],
            ['no value', {k: 'cve'}],
            ['empty kind', {k: '', v: 'CVE-2021-44228'}],
            ['unknown kind', {k: 'domain', v: 'example.com'}],
            ['a kind that is not this value', {k: 'cwe', v: 'CVE-2021-44228'}],
            ['lower case cve', {k: 'cve', v: 'cve-2021-44228'}],
            ['a padded cwe', {k: 'cwe', v: 'CWE-079'}],
            ['upper case hash', {k: 'hash', v: '44D88612FEA8A8F36DE82E1278ABB02F'}],
            ['a 33 digit hash', {k: 'hash', v: 'd'.repeat(33)}],
            ['markup', {k: 'cve', v: '<script>'}],
            ['a newline', {k: 'cve', v: 'CVE-2021-44228\n'}],
            ['a leading space', {k: 'cve', v: ' CVE-2021-44228'}],
        ];

        for (const [name, entries] of cases) {
            expect(fromParams(params(entries)), name).toBeNull();
        }
    });

    test('reads only k and v, so a stray parameter changes nothing', () => {
        const payload = fromParams(params({k: 'cwe', v: 'CWE-79', title: 'anything'}));

        expect(payload?.kind).toBe('cwe');
        expect(payload?.value).toBe('CWE-79');
    });

    test('accepts a well formed id this build may not hold', () => {
        expect(fromParams(params({k: 'attack', v: 'T9999'}))?.value).toBe('T9999');
        expect(fromParams(params({k: 'cve', v: 'CVE-2099-9999'}))?.value).toBe('CVE-2099-9999');
    });
});

test.describe('the decorator', () => {
    test('names the type the server writes into the link', () => {
        expect(decorator.type).toBe('cyber');
    });

    test('summarizes by kind, and falls back to the panel title', () => {
        expect(decorator.summary({kind: 'cve', value: 'CVE-2021-44228'})).toBe('Vulnerability');
        expect(decorator.summary({kind: 'ip', value: '203.0.113.7'})).toBe('IP address');
        expect(decorator.summary({kind: 'nonsense', value: 'x'})).toBe(PANEL_TITLE);
    });

    test('carries a chip style and both surfaces', () => {
        expect(decorator.style.color).toBeTruthy();
        expect(decorator.style.background).toBeTruthy();
        expect(decorator.Panel).toBeTruthy();
        expect(decorator.Hover).toBeTruthy();
    });
});

test.describe('matchesShape', () => {
    test('has a shape for every kind it lists', () => {
        for (const kind of KINDS) {
            expect(matchesShape(kind, ''), kind).toBe(false);
        }
        expect(KINDS.length).toBe(5);
    });

    test('an unknown kind matches nothing', () => {
        expect(matchesShape('domain', 'example.com')).toBe(false);
    });
});
