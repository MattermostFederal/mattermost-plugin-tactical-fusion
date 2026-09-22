import React from 'react';

import type {Report, ReportRow} from './types';
import {issuedLabel} from './types';

import LinkButton from '../components/LinkButton';
import {openRhs, setSelection} from '../decorators/selection';

const styles: Record<string, React.CSSProperties> = {
    rows: {
        display: 'grid',
        gridTemplateColumns: 'max-content 1fr',
        gap: '2px 12px',
        margin: '4px 0 0',
        fontSize: '0.9em',
    },
    term: {opacity: 0.85},
    value: {margin: 0, wordBreak: 'break-word'},
    section: {margin: '12px 0 2px', fontSize: '12px', fontWeight: 600, textTransform: 'uppercase', letterSpacing: '0.04em', opacity: 0.85},
    unknown: {fontFamily: 'monospace', fontSize: '0.85em', margin: '2px 0 0', opacity: 0.85, wordBreak: 'break-word'},
    flags: {fontFamily: 'monospace', fontSize: '0.85em', opacity: 0.9},
};

export function openStation(station: string): void {
    setSelection({type: 'airport', payload: {key: 'icao', code: station}});
    openRhs();
}

export const Rows: React.FC<{rows: ReportRow[]; testId?: string}> = ({rows, testId}) => {
    if (rows.length === 0) {
        return null;
    }

    return (
        <dl
            style={styles.rows}
            data-testid={testId}
        >
            {rows.map((row, index) => (
                // eslint-disable-next-line react/no-array-index-key
                <React.Fragment key={`${row.label}-${index}`}>
                    <dt style={styles.term}>{row.label}</dt>
                    <dd style={styles.value}>{row.value}</dd>
                </React.Fragment>
            ))}
        </dl>
    );
};

export const StationLine: React.FC<{report: Report}> = ({report}) => {
    if (report.stationName === '') {
        return null;
    }

    return (
        <LinkButton onClick={() => openStation(report.station)}>{report.stationName}</LinkButton>
    );
};

export function issuedRows(report: Report): ReportRow[] {
    const rows: ReportRow[] = [];
    if (report.issued !== '') {
        rows.push({label: issuedLabel(report), value: report.issued});
    }
    if (report.flags.length > 0) {
        rows.push({label: 'Flags', value: report.flags.join(', ')});
    }
    return rows;
}

export function bodyRows(report: Report): ReportRow[] {
    return report.rows.filter((row) => !(report.kind === 'NOTAM' && row.label === 'Effective'));
}

export const ReportDetail: React.FC<{report: Report}> = ({report}) => (
    <>
        <Rows
            rows={[...issuedRows(report), ...bodyRows(report)]}
            testId='avreport-rows'
        />

        {report.periods.map((period, index) => (
            // eslint-disable-next-line react/no-array-index-key
            <React.Fragment key={`${period.period}-${index}`}>
                <h3 style={styles.section}>{period.period}</h3>
                <Rows rows={period.rows}/>
            </React.Fragment>
        ))}

        {report.remarks.length > 0 && (
            <>
                <h3 style={styles.section}>{'Remarks'}</h3>
                <Rows
                    rows={report.remarks}
                    testId='avreport-remarks'
                />
            </>
        )}

        {report.unknown.length > 0 && (
            <>
                <h3 style={styles.section}>{'Not decoded'}</h3>
                <p
                    style={styles.unknown}
                    data-testid='avreport-unknown'
                >{report.unknown.join(' ')}</p>
            </>
        )}
    </>
);

export default ReportDetail;
