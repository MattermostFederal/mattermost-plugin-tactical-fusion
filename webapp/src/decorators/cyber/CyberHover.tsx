import React from 'react';

import Badges, {hasBadges} from './Badges';
import {useCyber} from './cyber';

import type {CyberPayload} from './index';

const styles: Record<string, React.CSSProperties> = {
    line: {
        fontSize: '13px',
        color: 'var(--center-channel-color)',
        whiteSpace: 'nowrap',
    },
    badges: {flexWrap: 'nowrap'},
};

const CyberHover: React.FC<{payload: CyberPayload}> = ({payload}) => {
    const state = useCyber(payload.kind, payload.value);

    if (state.status !== 'ready' || !state.data) {
        return null;
    }

    if (hasBadges(state.data)) {
        return (
            <Badges
                details={state.data}
                style={styles.badges}
            />
        );
    }

    return <span style={styles.line}>{state.data.headline}</span>;
};

export default CyberHover;
