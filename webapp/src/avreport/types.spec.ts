import {expect, test} from '@playwright/test';

import {HONOLULU_METAR, NOTAM_WITH_RADIUS, propsFor} from './report_fixtures';
import {
    AVREPORT_POST_TYPE,
    AVREPORT_PROPS_KEY,
    AVREPORT_PROPS_VERSION,
    MAX_AREA_POINTS,
    MAX_REPORT_FLAGS,
    MAX_REPORT_PERIODS,
    MAX_REPORT_ROWS,
    fromProps,
    fromWire,
    headingOf,
    isPlaced,
} from './types';

test('the constants are the ones the server writes', () => {
    expect(AVREPORT_POST_TYPE).toBe('custom_tf_avreport');
    expect(AVREPORT_PROPS_KEY).toBe('tactical_fusion_avreport');
    expect(AVREPORT_PROPS_VERSION).toBe(1);
});

test.describe('fromProps', () => {
    test('reads what the server writes', () => {
        const payload = fromProps(propsFor(HONOLULU_METAR));

        expect(payload).toEqual(HONOLULU_METAR);
        expect(payload && headingOf(payload)).toBe('METAR PHNL');
        expect(payload && isPlaced(payload)).toBe(true);
    });

    test('reads a NOTAM with a radius and several lines', () => {
        const payload = fromProps(propsFor(NOTAM_WITH_RADIUS));

        expect(payload?.radiusNm).toBe('5');
        expect(payload?.src).toContain('\n');
    });

    test('refuses a version this build does not know, or one that is not a number', () => {
        expect(fromProps(propsFor(HONOLULU_METAR, 2))).toBeNull();
        expect(fromProps(propsFor(HONOLULU_METAR, 'one'))).toBeNull();
        expect(fromProps(propsFor(HONOLULU_METAR, '1'))).toBeNull();
        expect(fromProps(propsFor(HONOLULU_METAR, true))).toBeNull();
        expect(fromProps(propsFor(HONOLULU_METAR, [1]))).toBeNull();
    });

    test('refuses a text field that is present and not a string', () => {
        for (const key of ['lead', 'trail', 'rows_dropped']) {
            const blob = propsFor(HONOLULU_METAR);
            (blob[AVREPORT_PROPS_KEY] as Record<string, unknown>)[key] = 5;
            expect(fromProps(blob), key).toBeNull();
        }
    });

    test('refuses a source kind it does not know, and a missing blob', () => {
        const blob = propsFor(HONOLULU_METAR);
        (blob[AVREPORT_PROPS_KEY] as Record<string, unknown>).source = 'file';

        expect(fromProps(blob)).toBeNull();
        expect(fromProps({})).toBeNull();
        expect(fromProps(null)).toBeNull();
        expect(fromProps('text')).toBeNull();
    });

    test('keeps a date-time query the dtg route would accept', () => {
        const query = 'a=&dtg=231143ZAUG26&t=1787485380000&z=Z';
        const payload = fromProps(propsFor({...HONOLULU_METAR, issuedQuery: query, rows: [{label: 'Valid', value: '23 Aug', query}]}));

        expect(payload?.issuedQuery).toBe(query);
        expect(payload?.rows[0].query).toBe(query);
    });

    test('reads a forged date-time query as absent rather than linking it', () => {
        for (const query of ['x=1', 'dtg=231143ZAUG26&z=Z', 'a=&dtg=231143ZAUG26&t=1787485380000&z=Z&o=0', '../../api/v4/users/me']) {
            const payload = fromProps(propsFor({...HONOLULU_METAR, issuedQuery: query, rows: [{label: 'Valid', value: '23 Aug', query}]}));

            expect(payload?.issuedQuery, query).toBe('');
            expect(payload?.rows[0], query).toEqual({label: 'Valid', value: '23 Aug'});
        }
    });

    test('marks a degraded blob', () => {
        expect(fromProps(propsFor({...HONOLULU_METAR, rowsDropped: true, rows: []}))?.rowsDropped).toBe(true);
    });
});

test.describe('fromWire', () => {
    const wire = () => propsFor(HONOLULU_METAR)[AVREPORT_PROPS_KEY] as Record<string, unknown>;

    test('refuses a kind it does not know', () => {
        expect(fromWire({...wire(), kind: 'PIREP'})).toBeNull();
    });

    test('refuses a report with no text', () => {
        expect(fromWire({...wire(), src: ''})).toBeNull();
    });

    test('refuses a text field that is present and not a string, and reads an absent one as empty', () => {
        for (const key of ['station', 'station_name', 'summary', 'region', 'radius_nm', 'issued', 'format', 'value']) {
            expect(fromWire({...wire(), [key]: 5}), key).toBeNull();
        }
        const absent = {...wire()};
        delete absent.summary;
        expect(fromWire(absent)?.summary).toBe('');
    });

    test('caps the flags', () => {
        const report = fromWire({...wire(), flags: Array.from({length: MAX_REPORT_FLAGS + 3}, () => 'COR')});
        expect(report?.flags).toHaveLength(MAX_REPORT_FLAGS);
    });

    test('refuses half a position', () => {
        expect(fromWire({...wire(), format: 'dd', value: ''})).toBeNull();
        expect(fromWire({...wire(), format: '', value: '1,2'})).toBeNull();
    });

    test('accepts no position at all', () => {
        const report = fromWire({...wire(), format: '', value: ''});
        expect(report).not.toBeNull();
        expect(report && isPlaced(report)).toBe(false);
    });

    test('refuses an instant that is not a number', () => {
        expect(fromWire({...wire(), issued_at: 'soon'})).toBeNull();
        expect(fromWire({...wire(), issued_at: 1})).toBeNull();
    });

    test('refuses a malformed row', () => {
        expect(fromWire({...wire(), rows: [{label: '', value: 'x'}]})).toBeNull();
        expect(fromWire({...wire(), rows: ['Wind']})).toBeNull();
        expect(fromWire({...wire(), rows: 'none'})).toBeNull();
    });

    test('refuses a malformed period', () => {
        expect(fromWire({...wire(), periods: [{period: '', rows: []}]})).toBeNull();
        expect(fromWire({...wire(), periods: [{period: 'FM230600', rows: 'x'}]})).toBeNull();
    });

    test('caps rather than refuses a list past the limit', () => {
        const rows = Array.from({length: MAX_REPORT_ROWS + 10}, (_, i) => ({label: `Row ${i}`, value: 'x'}));
        const periods = Array.from({length: MAX_REPORT_PERIODS + 3}, (_, i) => ({period: `FM23${i}`, rows: []}));

        const report = fromWire({...wire(), rows, periods});
        expect(report?.rows).toHaveLength(MAX_REPORT_ROWS);
        expect(report?.periods).toHaveLength(MAX_REPORT_PERIODS);
    });

    test('reads inferred only from a true boolean', () => {
        expect(fromWire({...wire(), inferred: 'true'})?.inferred).toBe(false);
        expect(fromWire({...wire(), inferred: false})?.inferred).toBe(false);
    });
});

test.describe('lists the plugin RPC boundary turned into null', () => {
    test('an empty list stored as null, or missing, reads as empty', () => {
        const wire = {...propsFor(HONOLULU_METAR).tactical_fusion_avreport as Record<string, unknown>};
        for (const key of ['flags', 'periods', 'remarks', 'unknown', 'area']) {
            wire[key] = null;
        }
        delete wire.rows;

        const payload = fromProps({tactical_fusion_avreport: wire});
        expect(payload).not.toBeNull();
        expect(payload?.flags).toEqual([]);
        expect(payload?.rows).toEqual([]);
        expect(payload?.periods).toEqual([]);
        expect(payload?.area).toEqual([]);
    });

    test('a blob stored before date-time links and areas still reads', () => {
        const wire = {...propsFor(HONOLULU_METAR).tactical_fusion_avreport as Record<string, unknown>};
        delete wire.issued_query;
        delete wire.area;
        wire.rows = (wire.rows as Array<Record<string, unknown>>).map(({label, value}) => ({label, value}));

        const payload = fromProps({tactical_fusion_avreport: wire});
        expect(payload).not.toBeNull();
        expect(payload?.issuedQuery).toBe('');
        expect(payload?.area).toEqual([]);
        expect(payload?.rows.every((row) => row.query === undefined)).toBe(true);
    });

    test('an area past the cap is not drawn rather than cut short', () => {
        const wire = {...propsFor(HONOLULU_METAR).tactical_fusion_avreport as Record<string, unknown>};
        wire.area = Array.from({length: MAX_AREA_POINTS + 1}, () => '21.3000,-157.9000');

        expect(fromProps({tactical_fusion_avreport: wire})?.area).toEqual([]);
    });

    test('a list that is present and not a list is still refused', () => {
        for (const key of ['flags', 'rows', 'periods', 'remarks', 'unknown', 'area']) {
            const wire = {...propsFor(HONOLULU_METAR).tactical_fusion_avreport as Record<string, unknown>, [key]: 'nope'};
            expect(fromProps({tactical_fusion_avreport: wire}), key).toBeNull();
        }
    });
});
