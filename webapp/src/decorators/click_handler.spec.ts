import {expect, test} from '@playwright/test';

import {dispatchDecoratorClick, pageLinkRel} from './click_handler';
import {register, _resetForTesting as resetRegistry} from './registry';
import {getSelection, _resetForTesting as resetSelection} from './selection';
import type {Decorator} from './types';

const fixture: Decorator<{value: string}> = {
    type: 'fix',
    fromParams: (params) => {
        const value = params.get('v');
        return value === null ? null : {value};
    },
    summary: (payload) => payload.value,
    style: {color: '#000', background: '#fff'},
    Panel: (() => null) as unknown as Decorator<{value: string}>['Panel'],
};

test.beforeEach(() => {
    resetRegistry();
    resetSelection();
    register(fixture);
});

test('a known type sets the selection and reports handled', () => {
    const handled = dispatchDecoratorClick('fix', new URLSearchParams('v=hello'));

    expect(handled).toBe(true);
    expect(getSelection()).toEqual({type: 'fix', payload: {value: 'hello'}});
});

// An older bundle against a newer server must fall through to the
// server-rendered page rather than swallowing the click.
test('an unknown type reports unhandled and leaves the selection alone', () => {
    const handled = dispatchDecoratorClick('nope', new URLSearchParams('v=hello'));

    expect(handled).toBe(false);
    expect(getSelection()).toBeNull();
});

test('unusable params report unhandled', () => {
    const handled = dispatchDecoratorClick('fix', new URLSearchParams('other=1'));

    expect(handled).toBe(false);
    expect(getSelection()).toBeNull();
});

// The escape hatch for checking the server-rendered page from a desktop
// browser, where the mobile app is not available to test with.
test('the force-page flag reports unhandled so the browser navigates', () => {
    const handled = dispatchDecoratorClick('fix', new URLSearchParams('v=hello&_page=1'));

    expect(handled).toBe(false);
    expect(getSelection()).toBeNull();
});

test('the force-page flag wins even with a value the decorator could use', () => {
    // Any value, including an empty one, opts out.
    expect(dispatchDecoratorClick('fix', new URLSearchParams('v=hello&_page='))).toBe(false);
    expect(getSelection()).toBeNull();
});

// A named target is ignored outright if the link also demands a fresh browsing
// context, so both tokens have to come off for the shared window to work.
const rels: Array<[string | null, string | null]> = [
    [null, null],
    ['', null],
    ['noopener', null],
    ['noreferrer', null],
    ['noopener noreferrer', null],

    // Anything else the link was carrying is its own business.
    ['nofollow', 'nofollow'],
    ['noopener nofollow noreferrer', 'nofollow'],
    ['nofollow external', 'nofollow external'],

    // Ragged whitespace is still a valid attribute.
    ['  noopener   noreferrer  ', null],
    [' nofollow  noopener ', 'nofollow'],
];

for (const [rel, want] of rels) {
    test(`rel ${JSON.stringify(rel)} becomes ${JSON.stringify(want)}`, () => {
        expect(pageLinkRel(rel)).toBe(want);
    });
}
