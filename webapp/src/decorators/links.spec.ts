import {expect, test} from '@playwright/test';
import manifest from 'manifest';

import {decoratorLinks} from './inline';

const PREFIX = `/plugins/${manifest.id}/decorate`;

const A = `[PHIK](${PREFIX}/airport?v=PHIK)`;
const B = `[HNL](${PREFIX}/airport?i=HNL)`;

test.beforeEach(() => {
    (globalThis as {window?: unknown}).window = {location: {origin: 'https://example.com'}};
});

test('finds every decorator link in message order with its span', () => {
    const message = `${A} then ${B}//`;
    const links = decoratorLinks(message);

    expect(links.map((l) => l.type)).toEqual(['airport', 'airport']);
    expect(links[0].params.get('v')).toBe('PHIK');
    expect(links[1].params.get('i')).toBe('HNL');
    expect(links[0].label).toBe('PHIK');
    expect(message.slice(links[0].start, links[0].end)).toBe(A);
    expect(message.slice(links[1].start, links[1].end)).toBe(B);
});

test('skips a link that is not a decorator link', () => {
    const links = decoratorLinks(`[docs](https://example.com) ${A} [x](/elsewhere)`);

    expect(links).toHaveLength(1);
    expect(links[0].params.get('v')).toBe('PHIK');
});

test('unescapes the label the tagger escaped', () => {
    const links = decoratorLinks(`[a\\_b](${PREFIX}/airport?v=PHIK)`);

    expect(links[0].label).toBe('a_b');
});

test('finds nothing in plain text', () => {
    expect(decoratorLinks('DEPLOC:PHIK ARRLOC:PGUA')).toEqual([]);
});

test('refuses a link whose href is not this origin, whatever its path says', () => {
    for (const href of [
        'javascript:alert(1)', // eslint-disable-line no-script-url
        `//evil.example${PREFIX}/airport?v=PHIK`,
        `https://evil.example${PREFIX}/airport?v=PHIK`,
        `data:text/html,${PREFIX}/airport?v=PHIK`,
    ]) {
        expect(decoratorLinks(`[PHIK](${href})`), href).toEqual([]);
    }
});
