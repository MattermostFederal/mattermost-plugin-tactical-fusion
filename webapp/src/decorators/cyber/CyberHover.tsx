import React from 'react';

import {useCyber} from './cyber';

import type {CyberPayload} from './index';

const styles: Record<string, React.CSSProperties> = {
    line: {
        fontSize: '13px',
        color: 'var(--center-channel-color)',
        whiteSpace: 'nowrap',
    },
};

const CyberHover: React.FC<{payload: CyberPayload}> = ({payload}) => {
    const state = useCyber(payload.kind, payload.value);

    if (state.status !== 'ready' || !state.data) {
        return null;
    }

    return <span style={styles.line}>{state.data.headline}</span>;
};

export default CyberHover;
