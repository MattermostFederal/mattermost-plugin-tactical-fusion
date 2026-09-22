import {expect, test} from '@playwright/test';

import {PANEL_TITLE, headingFromSource} from './title';

test('reads the kind and station off each report shape', () => {
    for (const [source, want] of [
        ['METAR KJFK 221651Z 28012KT 10SM FEW250 24/12 A3012', 'METAR KJFK'],
        ['SPECI COR KJFK 221651Z 28012KT', 'SPECI KJFK'],
        ['KJFK 221651Z 28012KT 10SM', 'METAR KJFK'],
        ['TAF AMD PGUA 221720Z 2218/2324 07012KT P6SM SCT025', 'TAF PGUA'],
        ['TAF PGUA 221720Z 2218/2324 07012KT\n  FM230600 09008KT', 'TAF PGUA'],
        ['!HNL 09/123 HNL RWY 08L/26R CLSD', 'NOTAM HNL'],
        ['A1234/26 NOTAMN\nQ) PHZH/QMRLC/IV/NBO/A/000/999/2119N15755W005\nA) PHNL B) 2609221200', 'NOTAM PHNL'],
        ['A1234/26 NOTAMN\nQ) PHZH/QMRLC/IV/NBO/A/000/999/2119N15755W005', 'NOTAM'],
    ] as const) {
        expect(headingFromSource(source), source).toBe(want);
    }
});

test('falls back to the panel title for anything else', () => {
    for (const source of ['', 'hello there', 'KJFK ready for departure', '221651Z 28012KT']) {
        expect(headingFromSource(source), source).toBe(PANEL_TITLE);
    }
});
