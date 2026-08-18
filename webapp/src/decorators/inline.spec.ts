import {expect, test} from '@playwright/test';
import manifest from 'manifest';

import {soleDecoratorLink, soleLead, soleLink} from './inline';

const PREFIX = `/plugins/${manifest.id}/decorate`;

const HREF = `${PREFIX}/location?f=ddh&v=34.0561N%2C118.2500W`;

function link(label: string, href = HREF): string {
    return `[${label}](${href})`;
}

test.beforeEach(() => {
    (globalThis as {window?: unknown}).window = {location: {origin: 'https://example.com'}};
});

test('a message that is only a link is one', () => {
    expect(soleLink(link('34.0561N,118.2500W'))?.href).toBe(HREF);
});

test('a label the decorator did not consume still leaves one link', () => {
    expect(soleLink(`MGRS: ${link('18SUJ2347806483')}`)?.label).toBe('18SUJ2347806483');
});

test('surrounding whitespace is trimmed before anything else', () => {
    expect(soleLink(`\n  ${link('x')}  \n`)?.href).toBe(HREF);
});

test('a link inside a sentence is not the whole message', () => {
    expect(soleLink(`target at ${link('x')}`)).toBeNull();
    expect(soleLink(`${link('x')} confirmed`)).toBeNull();
});

test('two links are not one link', () => {
    expect(soleLink(`${link('a')} ${link('b')}`)).toBeNull();
});

// The tagger escapes \ [ ] * _ ` ~ in a label, so an escaped bracket is a
// character rather than the end of the label.
test('an escaped bracket does not end the label', () => {
    expect(soleLink(link('a\\[b\\]c'))?.label).toBe('a[b]c');
});

test('an unescaped bracket in the label is refused', () => {
    expect(soleLink(link('a[b]c'))).toBeNull();
});

// buildURL percent-encodes ( and ) in the query, because an unbalanced paren
// would terminate a markdown destination. A destination carrying a bare one was
// never written by this plugin.
test('a destination carrying a paren is refused', () => {
    expect(soleLink(link('x', `${PREFIX}/location?f=dd&v=(1)`))).toBeNull();
});

test('a label of more than one word is not a label', () => {
    expect(soleLink(`see this: ${link('x')}`)).toBeNull();
});

test('a label with no colon is not a label', () => {
    expect(soleLink(`MGRS ${link('x')}`)).toBeNull();
});

test('a single letter is not a label', () => {
    expect(soleLink(`A: ${link('x')}`)).toBeNull();
});

test('a label longer than sixteen characters is not a label', () => {
    expect(soleLink(`ABCDEFGHIJKLMNOPQ: ${link('x')}`)).toBeNull();
    expect(soleLink(`ABCDEFGHIJKLMNOP: ${link('x')}`)).not.toBeNull();
});

test('soleLead reports the label the decorator left in the message', () => {
    expect(soleLead(`MGRS: ${link('x')}`)).toBe('MGRS:');
    expect(soleLead(link('x'))).toBe('');
});

test('soleDecoratorLink reads the type and params out of the destination', () => {
    const got = soleDecoratorLink(`MGRS: ${link('18SUJ2347806483')}`);

    expect(got?.type).toBe('location');
    expect(got?.params.get('f')).toBe('ddh');
    expect(got?.params.get('v')).toBe('34.0561N,118.2500W');
    expect(got?.lead).toBe('MGRS:');
    expect(got?.label).toBe('18SUJ2347806483');
});

test('a link outside the decorate prefix is not one of ours', () => {
    expect(soleDecoratorLink(link('x', '/plugins/other/decorate/location?f=dd'))).toBeNull();
});

test('another origin wearing our path is not one of ours', () => {
    expect(soleDecoratorLink(link('x', `https://evil.example${HREF}`))).toBeNull();
});
