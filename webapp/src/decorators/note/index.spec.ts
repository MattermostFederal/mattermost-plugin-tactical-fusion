import {expect, test} from '@playwright/test';

import {HOVER_MAX_WIDTH} from '../Tooltip';

import decorator, {MAX_NOTE_RUNES, NOTE_HOVER_MAX_WIDTH, firstLine, fromParams} from './index';

test('the type is the path segment the server routes', () => {
    expect(decorator.type).toBe('note');
    expect(decorator.Hover).toBeDefined();
    expect(decorator.postType).toBeUndefined();
});

test('reads the markdown whole, line breaks included', () => {
    const markdown = '| Tail | Fuel |\n|:--|--:|\n| 101 | 12,000 lb |';
    expect(fromParams(new URLSearchParams({v: markdown}))).toEqual({markdown});
});

test('refuses a missing, blank or over-long note', () => {
    expect(fromParams(new URLSearchParams())).toBeNull();
    expect(fromParams(new URLSearchParams({v: ' \n '}))).toBeNull();
    expect(fromParams(new URLSearchParams({v: 'é'.repeat(MAX_NOTE_RUNES + 1)}))).toBeNull();
    expect(fromParams(new URLSearchParams({v: '😀'.repeat(MAX_NOTE_RUNES)}))).not.toBeNull();
});

test('the summary is the first line of text with the markdown stripped', () => {
    expect(firstLine('**DCA**: Defensive Counter Air\n\nMore')).toBe('DCA: Defensive Counter Air');
    expect(firstLine('#### Squadron readiness\n\n| Tail |')).toBe('Squadron readiness');
    expect(firstLine('| Tail | Fuel |\n|:--|--:|')).toBe('Tail Fuel');
    expect(firstLine('- [x] Flight plan filed')).toBe('Flight plan filed');
    expect(decorator.summary({markdown: '> Expect turbulence'})).toBe('Note: Expect turbulence');
});

test('the hover card is a fifth wider than the framework default', () => {
    expect(NOTE_HOVER_MAX_WIDTH).toBe(HOVER_MAX_WIDTH * 1.2);
    expect(decorator.hoverMaxWidth).toBe(NOTE_HOVER_MAX_WIDTH);
});
