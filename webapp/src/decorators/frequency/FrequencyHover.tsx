import React from 'react';

import type {FrequencyPayload} from './index';
import {describe} from './index';

const styles: Record<string, React.CSSProperties> = {
    band: {fontSize: '14px', fontWeight: 600, color: 'var(--center-channel-color)', margin: 0},
    use: {fontSize: '12px', opacity: 0.8, color: 'var(--center-channel-color)', margin: '2px 0 0'},
};

const FrequencyHover: React.FC<{payload: FrequencyPayload}> = ({payload}) => {
    const details = describe(payload);

    return (
        <>
            <p style={styles.band}>{`${details.mhz} MHz, ${details.band}`}</p>
            {details.use !== '' && <p style={styles.use}>{details.use}</p>}
        </>
    );
};

export default FrequencyHover;
