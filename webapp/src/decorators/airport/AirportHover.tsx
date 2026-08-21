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

/**
 * The card behind an airfield code.
 *
 * The name and where it is, one line each, and nothing else. That is the whole
 * bar a hover has to meet here: a four-letter code says nothing at all to a
 * reader, and naming it is exactly what a glance is asking. Every reading is
 * one click away in the panel, the way the location hover leaves its table to
 * the panel.
 *
 * It renders null for every state but a found airfield, and the framework's
 * `:empty` rule on the card turns that into no card at all rather than an empty
 * bordered box. A card that said "looking up" would flicker on every code a
 * cursor crossed.
 */
const AirportHover: React.FC<{payload: AirportPayload}> = ({payload}) => {
    const state = useAirport(payload.ident);

    const airport = state.status === 'ready' ? state.data?.airport : undefined;
    if (!airport) {
        return null;
    }

    return (
        <>
            <p style={styles.name}>{airport.name || payload.ident}</p>
            {airport.place !== '' && <p style={styles.place}>{airport.place}</p>}
        </>
    );
};

export default AirportHover;
