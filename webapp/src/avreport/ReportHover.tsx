import React from 'react';

import type {ReportLinkPayload} from './client';
import {useReport} from './client';
import {headingOf} from './types';

const styles: Record<string, React.CSSProperties> = {
    name: {fontSize: '14px', fontWeight: 600, color: 'var(--center-channel-color)', margin: 0},
};

const ReportHover: React.FC<{payload: ReportLinkPayload}> = ({payload}) => {
    const state = useReport(payload);

    const report = state.status === 'ready' ? state.data : null;
    if (!report) {
        return null;
    }

    return (
        <p style={styles.name}>{report.stationName === '' ? headingOf(report) : `${headingOf(report)}, ${report.stationName}`}</p>
    );
};

export default ReportHover;
