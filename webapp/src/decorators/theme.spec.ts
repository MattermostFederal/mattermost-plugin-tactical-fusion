import {expect, test} from '@playwright/test';

import {detectTheme, themeForBackground, withTheme} from './theme';

test('classifies Mattermost default backgrounds', () => {
    // Denim and the other light themes.
    expect(themeForBackground('#ffffff')).toBe('light');

    // Onyx, which is very nearly black, and Indigo.
    expect(themeForBackground('#090a0b')).toBe('dark');
    expect(themeForBackground('#151e2e')).toBe('dark');
});

test('accepts the color forms a theme variable can hold', () => {
    expect(themeForBackground('#fff')).toBe('light');
    expect(themeForBackground('#000')).toBe('dark');
    expect(themeForBackground('rgb(255, 255, 255)')).toBe('light');
    expect(themeForBackground('rgba(9, 10, 11, 1)')).toBe('dark');
    expect(themeForBackground('  #ffffff  ')).toBe('light');
});

// Green is bright to the eye even at full saturation, so a naive average would
// misread it. Luma weighting is what keeps mid-tones on the right side.
test('weights channels by perceived brightness', () => {
    expect(themeForBackground('#00ff00')).toBe('light');
    expect(themeForBackground('#0000ff')).toBe('dark');
});

// Returning null lets the caller leave the theme unstated rather than guess,
// so the page falls back to the operating system preference.
test('returns null for anything it cannot read', () => {
    expect(themeForBackground('')).toBeNull();
    expect(themeForBackground('inherit')).toBeNull();
    expect(themeForBackground('var(--something)')).toBeNull();
    expect(themeForBackground('#12345')).toBeNull();
});

/*
 * Reading the theme off the live DOM, and the ways that can come up empty.
 *
 * The whole point of returning null rather than a default is that the page then
 * falls back to the operating system preference, which is a worse answer than
 * the sidebar's but a better one than stating the wrong theme as fact.
 */
type Global = {window?: unknown; document?: unknown};

interface FakeElement {
    background: string;
}

function withDom(elements: {documentElement?: FakeElement | null; body?: FakeElement | null}, origin = 'https://mm.example'): void {
    const holder = globalThis as Global;

    holder.document = {
        documentElement: elements.documentElement ?? null,
        body: elements.body ?? null,
    };
    holder.window = {
        location: {origin},
        getComputedStyle: (element: FakeElement) => ({
            getPropertyValue: () => element.background,
        }),
    };
}

test.afterEach(() => {
    const holder = globalThis as Global;
    delete holder.window;
    delete holder.document;
});

test.describe('detectTheme', () => {
    test('has nothing to read outside a browser', () => {
        expect(detectTheme()).toBeNull();
    });

    // A window that cannot compute styles is not a browser this can read, and
    // it must degrade rather than throw on the way past.
    test('has nothing to read without getComputedStyle', () => {
        (globalThis as Global).window = {};

        expect(detectTheme()).toBeNull();
    });

    test('reads the document element', () => {
        withDom({documentElement: {background: '#090a0b'}});

        expect(detectTheme()).toBe('dark');
    });

    // Both elements are consulted because a theme may be set on either, and an
    // absent one is skipped rather than being read as no theme at all.
    test('falls through a missing document element to the body', () => {
        withDom({documentElement: null, body: {background: '#ffffff'}});

        expect(detectTheme()).toBe('light');
    });

    test('falls through a document element stating nothing', () => {
        withDom({documentElement: {background: ''}, body: {background: '#090a0b'}});

        expect(detectTheme()).toBe('dark');
    });

    test('leaves the theme unstated when neither element says', () => {
        withDom({documentElement: {background: 'inherit'}, body: {background: ''}});

        expect(detectTheme()).toBeNull();
    });
});

test.describe('withTheme', () => {
    test('adds the theme and stays root-relative', () => {
        withDom({documentElement: {background: '#090a0b'}});

        expect(withTheme('/plugins/x/map?f=dd&v=1')).toBe('/plugins/x/map?f=dd&v=1&_theme=dark');
    });

    test('replaces a theme already on the link', () => {
        withDom({documentElement: {background: '#ffffff'}});

        expect(withTheme('/plugins/x/map?_theme=dark')).toBe('/plugins/x/map?_theme=light');
    });

    // An undetectable theme leaves the link exactly as written, so the page
    // follows the operating system rather than being told the wrong thing.
    test('leaves a link alone when there is no theme to state', () => {
        expect(withTheme('/plugins/x/map?f=dd')).toBe('/plugins/x/map?f=dd');
    });

    // The href is this plugin's own, so this is a guard rather than a case that
    // arises: it must return something usable rather than take the caller down.
    test('leaves a link alone when it cannot be parsed', () => {
        withDom({documentElement: {background: '#090a0b'}}, 'not an origin');

        expect(withTheme('/plugins/x/map?f=dd')).toBe('/plugins/x/map?f=dd');
    });
});
