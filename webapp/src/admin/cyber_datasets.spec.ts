import {expect, test} from '@playwright/test';

import {asDatasets, sizeText, stampText} from './CyberDatasets';

test('sizes read in the largest whole unit', () => {
    expect(sizeText(0)).toBe('0 bytes');
    expect(sizeText(1023)).toBe('1023 bytes');
    expect(sizeText(1024)).toBe('1.0 KB');
    expect(sizeText(185738016)).toBe('177.1 MB');
    expect(sizeText(5 * 1024 * 1024 * 1024 * 1024)).toBe('5120.0 GB');
});

test('a dataset list of the wrong shape is refused whole', () => {
    const ok = {directories: [], datasets: [], databases: [], missing: [], replaced: [], skipped: []};
    expect(asDatasets(ok)).toEqual(ok);

    for (const bad of [
        null,
        [],
        {...ok, datasets: undefined},
        {...ok, datasets: [{name: 'cve'}]},
        {...ok, replaced: [1]},
        {...ok, databases: [{path: 'x', kind: 'k', type: 't', built: '', size: '1'}]},
    ]) {
        expect(asDatasets(bad)).toBeNull();
    }
});

test('a stamp reads as a date and a minute in UTC, and anything else as written', () => {
    expect(stampText('2026-09-23T20:57:13Z')).toBe('2026-09-23 20:57 UTC');
    expect(stampText('2026-09-23T20:57:13.500Z')).toBe('2026-09-23 20:57 UTC');
    expect(stampText('2026-09-23T20:57Z')).toBe('2026-09-23 20:57 UTC');
    expect(stampText('2026-09-23T20:57:13+02:00')).toBe('2026-09-23T20:57:13+02:00');
    expect(stampText('')).toBe('');
});
