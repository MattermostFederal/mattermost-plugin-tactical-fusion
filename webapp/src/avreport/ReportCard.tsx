import React from 'react';

import {areaFocus, hasArea} from './map';
import {showReport} from './panel';
import ReportDetail, {StationLine} from './ReportDetail';
import ReportMap from './ReportMap';
import type {ReportPayload} from './types';
import {headingOf} from './types';

import ErrorBoundary from '../components/ErrorBoundary';
import {useInlineMapShown, useMapFocus} from '../decorators/location/map/use_map_focus';

export const CARD_KIND = 'Aviation report';

export const DETAIL_FAILED = 'The detail of this report could not be rendered. The report itself is shown above as it was posted.';

export const ROWS_DROPPED_NOTE = 'The decoded groups were omitted to fit the size limit. The report is shown as posted.';

const styles: Record<string, React.CSSProperties> = {
    text: {whiteSpace: 'pre-wrap'},
    card: {
        border: '1px solid rgba(var(--center-channel-color-rgb), 0.16)',
        borderRadius: 4,
        marginTop: 8,
        maxWidth: 640,
        overflow: 'hidden',
    },
    kind: {fontWeight: 700, margin: 0, padding: '8px 12px 0'},
    header: {alignItems: 'baseline', display: 'flex', flexWrap: 'wrap', gap: '0.5em', padding: '2px 12px 4px'},
    heading: {fontWeight: 600},
    source: {
        fontFamily: 'monospace',
        fontSize: '0.85em',
        margin: '0 12px 8px',
        whiteSpace: 'pre-wrap',
        wordBreak: 'break-word',
    },
    note: {opacity: 0.9, padding: '0 12px 8px', margin: 0},
    detail: {padding: '0 12px 8px'},
    actions: {display: 'flex', gap: '12px', padding: '0 12px 8px'},
    button: {
        background: 'none',
        border: 'none',
        color: 'var(--link-color)',
        cursor: 'pointer',
        font: 'inherit',
        padding: 0,
    },
};

export const ReportCard: React.FC<{payload: ReportPayload; compactDisplay?: boolean}> = ({payload, compactDisplay}) => {
    const [focus, show] = useMapFocus(areaFocus);
    const onShow = useInlineMapShown() && hasArea(payload) ? () => show(payload) : undefined;

    return (
        <div>
            {payload.lead !== '' && <span style={styles.text}>{payload.lead}</span>}
            <div
                style={styles.card}
                data-testid='avreport-card'
            >
                <p style={styles.kind}>{CARD_KIND}</p>
                <div style={styles.header}>
                    <span
                        style={styles.heading}
                        data-testid='avreport-heading'
                    >{headingOf(payload)}</span>
                    <StationLine report={payload}/>
                </div>

                <pre
                    style={styles.source}
                    data-testid='avreport-source'
                >{payload.src}</pre>

                {payload.rowsDropped && (
                <p
                    style={styles.note}
                    data-testid='avreport-degraded'
                >{ROWS_DROPPED_NOTE}</p>
            )}

                {!compactDisplay && (
                <ErrorBoundary fallback={<p style={styles.note}>{DETAIL_FAILED}</p>}>
                    <ReportMap
                        report={payload}
                        surface='card'
                        postId={payload.postId}
                        focus={focus}
                    />
                </ErrorBoundary>
            )}

                {!compactDisplay && (
                <ErrorBoundary fallback={<p style={styles.note}>{DETAIL_FAILED}</p>}>
                    <div style={styles.detail}>
                        <ReportDetail
                            report={payload}
                            onShow={onShow}
                        />
                    </div>
                </ErrorBoundary>
            )}

                <div style={styles.actions}>
                    <button
                        type='button'
                        style={styles.button}
                        onClick={() => showReport(payload)}
                    >{'Open details'}</button>
                </div>
            </div>
            {payload.trail !== '' && <span style={styles.text}>{payload.trail}</span>}
        </div>
    );
};

export default ReportCard;
