import React from 'react';

import type {ReportLinkPayload} from './client';
import {useReport} from './client';
import ReportDetail, {StationLine} from './ReportDetail';
import ReportMap from './ReportMap';
import type {Report, ReportPayload} from './types';
import {headingOf} from './types';

import ErrorBoundary from '../components/ErrorBoundary';
import LinkButton from '../components/LinkButton';
import Disclosure from '../cot/Disclosure';
import CopyButton from '../decorators/location/CopyButton';
import {docsUrl} from '../plugin_url';

export const PANEL_TITLE = 'Aviation report';

export const SOURCE_LABEL = 'As posted';

export const SECTION_FAILED = 'This report could not be rendered.';

export const STATUS_TEXT: Record<'loading' | 'failed' | 'rejected', string> = {
    loading: 'Decoding the report…',
    failed: 'The report could not be decoded right now. Check your connection and try again.',
    rejected: 'This link does not carry a report this plugin issued.',
};

const styles: Record<string, React.CSSProperties> = {
    heading: {margin: '0 0 4px', fontSize: '16px', fontWeight: 600},
    subhead: {margin: '0 0 8px', opacity: 0.85, fontSize: '13px'},
    summary: {margin: '0 0 12px', fontSize: '14px'},
    source: {
        fontFamily: 'monospace',
        fontSize: '0.85em',
        margin: '0 0 12px',
        maxHeight: 280,
        overflow: 'auto',
        whiteSpace: 'pre-wrap',
        wordBreak: 'break-word',
    },
    posted: {
        fontFamily: 'monospace',
        fontSize: '0.85em',
        margin: 0,
        maxHeight: 280,
        overflow: 'auto',
        whiteSpace: 'pre-wrap',
        wordBreak: 'break-word',
    },
    status: {margin: '0 0 12px', opacity: 0.85, fontSize: '13px'},
    footer: {
        borderTop: '1px solid rgba(var(--center-channel-color-rgb), 0.16)',
        fontSize: '13px',
        marginTop: '20px',
        paddingTop: '12px',
    },
};

const Footer: React.FC = () => (
    <div style={styles.footer}>
        <LinkButton href={docsUrl()}>{'Documentation'}</LinkButton>
    </div>
);

export const ReportView: React.FC<{report: Report; postId?: string}> = ({report, postId}) => (
    <div data-testid='avreport-panel'>
        <h2
            style={styles.heading}
            data-testid='avreport-heading'
        >{headingOf(report)}</h2>
        {report.stationName !== '' && <p style={styles.subhead}><StationLine report={report}/></p>}
        {report.summary !== '' && (
            <p
                style={styles.summary}
                data-testid='avreport-summary'
            >{report.summary}</p>
        )}

        <ErrorBoundary fallback={<p style={styles.status}>{SECTION_FAILED}</p>}>
            <ReportDetail report={report}/>
        </ErrorBoundary>

        {report.src !== '' && (
            <Disclosure
                label={SOURCE_LABEL}
                trailing={
                    <CopyButton
                        label='Copy the report as posted'
                        value={report.src}
                    />
                }
            >
                <pre
                    style={styles.posted}
                    tabIndex={0}
                    role='region'
                    aria-label='The report as it was posted'
                    data-testid='avreport-source'
                >{report.src}</pre>
            </Disclosure>
        )}

        <ErrorBoundary fallback={<p style={styles.status}>{SECTION_FAILED}</p>}>
            <ReportMap
                report={report}
                surface='panel'
                postId={postId}
            />
        </ErrorBoundary>

        <Footer/>
    </div>
);

export const ReportPanel: React.FC<{payload: ReportPayload}> = ({payload}) => (
    <ReportView
        report={payload}
        postId={payload.postId === '' ? undefined : payload.postId}
    />
);

export const ReportLinkPanel: React.FC<{payload: ReportLinkPayload}> = ({payload}) => {
    const state = useReport(payload);

    if (state.status !== 'ready' || state.data === null) {
        return (
            <div data-testid='avreport-panel'>
                <p
                    style={styles.status}
                    data-testid='avreport-status'
                >{STATUS_TEXT[state.status === 'ready' ? 'failed' : state.status]}</p>
                <pre
                    style={styles.source}
                    data-testid='avreport-source'
                >{payload.v}</pre>
                <Footer/>
            </div>
        );
    }

    return <ReportView report={state.data}/>;
};

export default ReportPanel;
