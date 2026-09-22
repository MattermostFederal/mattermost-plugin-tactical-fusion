import React from 'react';

import {useAirport} from './airport';

import type {AirportPayload} from './index';

const styles: Record<string, React.CSSProperties> = {
    name: {
        fontSize: '14px',
        fontWeight: 600,
        color: 'var(--center-channel-color)',
        margin: 0,
    },
    place: {
        fontSize: '12px',
        opacity: 0.7,
        color: 'var(--center-channel-color)',
        margin: '2px 0 0',
    },
};

const AirportHover: React.FC<{payload: AirportPayload}> = ({payload}) => {
    const state = useAirport(payload);

    const answer = state.status === 'ready' ? state.data : null;
    const airport = answer?.airport;
    if (!answer || !airport) {
        return null;
    }

    return (
        <>
            <p style={styles.name}>{airport.name || answer.ident}</p>
            {airport.place !== '' && <p style={styles.place}>{airport.place}</p>}
        </>
    );
};

export default AirportHover;
