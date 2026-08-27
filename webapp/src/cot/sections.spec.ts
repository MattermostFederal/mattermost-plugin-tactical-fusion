import {expect, test} from '@playwright/test';

import {SECTIONS, isSectionID, isSectionVisible, sectionLabel} from './sections';

test('every section has a label, and it is the catalog\'s', () => {
    for (const section of SECTIONS) {
        expect(sectionLabel(section.id), section.id).toBe(section.label);
        expect(section.label).not.toBe('');
    }
});

test('every section has a hint of its own', () => {
    const hints = SECTIONS.map((section) => section.hint);

    expect(new Set(hints).size).toBe(SECTIONS.length);
    for (const hint of hints) {
        expect(hint).not.toBe('');
    }
});

test('the catalog names each section once', () => {
    const ids = SECTIONS.map((section) => section.id);

    expect(new Set(ids).size).toBe(ids.length);
});

test('an id this build does not have is not a section', () => {
    expect(isSectionID('telepathy')).toBe(false);
    expect(isSectionID('map')).toBe(true);
});

test('a hidden id this build does not have hides nothing', () => {
    expect(isSectionVisible(['telepathy'], 'map')).toBe(true);
    expect(isSectionVisible(['map'], 'map')).toBe(false);
});
