import {expect, test} from '@playwright/test';
import manifest from 'manifest';

import {all, decoratePathPrefix, get, parseDecoratorHref, register, _resetForTesting} from './registry';
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

/*
 * What counts as a decorator link.
 *
 * Shared so the click handler and the hover card cannot come to disagree, and
 * same-origin only: an absolute cross-origin URL carrying the right path is
 * somebody else's destination wearing our path, and would otherwise collect a
 * genuine hover card and, on the _page branch, have its rel stripped and be
 * opened in a named window with a live opener.
 */
test.describe('parseDecoratorHref', () => {
    const origin = 'https://mm.example';

    function withOrigin(value: string): void {
        (globalThis as {window?: unknown}).window = {location: {origin: value}};
    }

    test.afterEach(() => {
        delete (globalThis as {window?: unknown}).window;
    });

    test('reads the type and the params off a decorator link', () => {
        withOrigin(origin);

        const parsed = parseDecoratorHref(`/plugins/${manifest.id}/decorate/dtg?t=1&z=Z`);

        expect(parsed?.type).toBe('dtg');
        expect(parsed?.params.get('z')).toBe('Z');
    });

    test('accepts an absolute url on this origin', () => {
        withOrigin(origin);

        expect(parseDecoratorHref(`${origin}/plugins/${manifest.id}/decorate/dtg?t=1`)?.type).toBe('dtg');
    });

    test('refuses another origin wearing our path', () => {
        withOrigin(origin);

        expect(parseDecoratorHref(`https://evil.example/plugins/${manifest.id}/decorate/dtg?t=1`)).toBeNull();
    });

    // /map is deliberately outside the prefix, which is why the links pointing
    // at it carry the theme parameter themselves.
    test('refuses a path outside the decorate prefix', () => {
        withOrigin(origin);

        expect(parseDecoratorHref(`/plugins/${manifest.id}/map?f=dd`)).toBeNull();
        expect(parseDecoratorHref('/channels/town-square')).toBeNull();
    });

    test('refuses a prefix with no type after it', () => {
        withOrigin(origin);

        expect(parseDecoratorHref(`/plugins/${manifest.id}/decorate/`)).toBeNull();
    });

    test('refuses a type that is really a deeper path', () => {
        withOrigin(origin);

        expect(parseDecoratorHref(`/plugins/${manifest.id}/decorate/dtg/extra`)).toBeNull();
    });

    // The href comes off a link in a rendered message, so it can be anything at
    // all. Returning null is what makes the handler stand aside.
    test('refuses an href that cannot be parsed at all', () => {
        withOrigin('not an origin');

        expect(parseDecoratorHref(`/plugins/${manifest.id}/decorate/dtg`)).toBeNull();
    });
});
