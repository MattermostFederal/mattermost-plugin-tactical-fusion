import {expect, test} from '@playwright/test';

import {themeForBackground} from './theme';

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
