import {expect, test} from '@playwright/test';
import manifest from 'manifest';

import {applyBasename} from './basename';

/*
 * Where the page thinks it is.
 *
 * Everything the bundle fetches goes through pluginBaseUrl, which reads this, so
 * a wrong answer sends the basemap request and the documentation link somewhere
 * else on the same origin. It is derived rather than configured because the page
 * renderers are pure functions of a query string and cannot see SiteURL.
 */

function withPath(pathname: string): string | undefined {
    const holder = globalThis as {window?: unknown};
    const previous = holder.window;

    holder.window = {location: {pathname}};
    applyBasename();
    const got = (holder.window as {basename?: string}).basename;

    holder.window = previous;

    return got;
}

// The guard exists so importing the page bundle's entry point outside a
// browser is not a crash. It is the first thing the entry point calls.
test('there is nothing to apply outside a browser', () => {
    const holder = globalThis as {window?: unknown};
    const previous = holder.window;
    delete holder.window;

    expect(() => applyBasename()).not.toThrow();

    holder.window = previous;
});

test('a root install has no basename', () => {
    expect(withPath(`/plugins/${manifest.id}/decorate/location`)).toBeUndefined();
});

test('a subpath install keeps its prefix', () => {
    expect(withPath(`/mattermost/plugins/${manifest.id}/map`)).toBe('/mattermost');
    expect(withPath(`/a/b/plugins/${manifest.id}/decorate/location`)).toBe('/a/b');
});

/*
 * The three refusals. This value decides where the browser fetches from, so a
 * prefix that is not a rooted path is dropped rather than used: the same rule
 * the server applies to SiteURL, and the same one staticBaseUrl applies before
 * handing a value to __webpack_public_path__.
 */
test.describe('refuses anything that is not a rooted path', () => {
    const cases: Array<[string, string]> = [
        ['protocol relative', `//evil.example/plugins/${manifest.id}/map`],
        ['an absolute url', `/https://evil.example/plugins/${manifest.id}/map`],
        ['no marker at all', '/plugins/some.other.plugin/map'],
    ];

    for (const [name, path] of cases) {
        test(name, () => {
            expect(withPath(path)).toBeUndefined();
        });
    }
});
