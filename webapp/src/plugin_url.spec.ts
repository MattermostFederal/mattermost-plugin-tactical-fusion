import {expect, test} from '@playwright/test';
import manifest from 'manifest';

import {docsUrl, pluginBaseUrl} from './plugin_url';

// `window.basename` is read at call time rather than at import time, which is
// what lets these tests set it around a single call.
type Global = {window?: {basename?: string}};

function withBasename(basename: string | undefined): void {
    (globalThis as Global).window = basename === undefined ? {} : {basename};
}

test.afterEach(() => {
    delete (globalThis as Global).window;
});

test.describe('pluginBaseUrl', () => {
    // The guard exists so the pure logic built on this stays testable outside a
    // browser, which is exactly the situation here.
    test('has no basename outside a browser', () => {
        expect(pluginBaseUrl()).toBe(`/plugins/${manifest.id}`);
    });

    test('has no basename when the window carries none', () => {
        withBasename(undefined);

        expect(pluginBaseUrl()).toBe(`/plugins/${manifest.id}`);
    });

    // The whole reason the function exists: a hardcoded root-relative path
    // would break every install served from a subpath.
    test('carries the subpath Mattermost is served from', () => {
        withBasename('/mattermost');

        expect(pluginBaseUrl()).toBe(`/mattermost/plugins/${manifest.id}`);
    });

    test('handles a deep subpath', () => {
        withBasename('/apps/chat');

        expect(pluginBaseUrl()).toBe(`/apps/chat/plugins/${manifest.id}`);
    });

    // An empty basename is falsy but not nullish, so `??` has to keep it rather
    // than fall through to its own default. Either way the answer is the same,
    // which is what makes the distinction safe to have.
    test('an empty basename reads as no subpath', () => {
        withBasename('');

        expect(pluginBaseUrl()).toBe(`/plugins/${manifest.id}`);
    });

    // Not normalized. Mattermost does not set a bare slash here, and trimming
    // it would be guessing at a case that does not arise; pinned so a change
    // to that is a decision.
    test('a bare slash basename is passed through as written', () => {
        withBasename('/');

        expect(pluginBaseUrl()).toBe(`//plugins/${manifest.id}`);
    });

    // The id is derived rather than written out, so the URL cannot drift from
    // whatever plugin.json says.
    test('names the plugin from the manifest', () => {
        expect(pluginBaseUrl()).toContain(manifest.id);
    });
});

test.describe('docsUrl', () => {
    // Mattermost serves the bundle's public/ directory; there is no route for
    // this in the server code.
    test('points at the built-in help page', () => {
        expect(docsUrl()).toBe(`/plugins/${manifest.id}/public/help/help.html`);
    });

    test('inherits the subpath', () => {
        withBasename('/mattermost');

        expect(docsUrl()).toBe(`/mattermost/plugins/${manifest.id}/public/help/help.html`);
    });

    test('builds on the plugin base rather than repeating it', () => {
        withBasename('/mattermost');

        expect(docsUrl()).toBe(`${pluginBaseUrl()}/public/help/help.html`);
    });
});
