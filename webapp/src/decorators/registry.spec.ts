import {expect, test} from '@playwright/test';
import manifest from 'manifest';

import {all, decoratePathPrefix, get, register, _resetForTesting} from './registry';
import type {Decorator} from './types';

// A decorator defined entirely in the test file. If this works without touching
// anything under decorators/, so does a real one.
function fixture(type: string): Decorator<{value: string}> {
    return {
        type,
        fromParams: (params) => {
            const value = params.get('v');
            return value === null ? null : {value};
        },
        summary: (payload) => payload.value,
        style: {color: '#000', background: '#fff'},
        Panel: (() => null) as unknown as Decorator<{value: string}>['Panel'],
    };
}

test.beforeEach(() => {
    _resetForTesting();
});

test('register preserves registration order', () => {
    register(fixture('first'));
    register(fixture('second'));

    expect(all().map((d) => d.type)).toEqual(['first', 'second']);
});

test('register rejects a duplicate type', () => {
    register(fixture('dup'));

    expect(() => register(fixture('dup'))).toThrow(/already registered/);
});

test('register rejects an empty type', () => {
    expect(() => register(fixture(''))).toThrow(/must not be empty/);
});

test('get returns undefined for an unregistered type', () => {
    register(fixture('known'));

    expect(get('known')).toBeDefined();
    expect(get('unknown')).toBeUndefined();
});

test('decoratePathPrefix is root-relative when there is no basename', () => {
    expect(decoratePathPrefix()).toBe(`/plugins/${manifest.id}/decorate/`);
});
