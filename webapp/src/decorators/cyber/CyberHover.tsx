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
    credit: {display: 'block', fontSize: '11px', opacity: 0.72, marginTop: '2px', whiteSpace: 'nowrap'},
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

    return (
        <span style={styles.line}>
            {state.data.headline}
            {state.data.credits.map((credit) => (
                <span
                    key={credit.text}
                    style={styles.credit}
                    data-testid='cyber-credit'
                >{credit.text}</span>
            ))}
        </span>
    );
};

export default CyberHover;
