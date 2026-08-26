import React, {useState} from 'react';

import CotCard from './CotCard';
import type {CotPayload} from './types';
import {emptyDetail} from './types';

import {RhsTitle, RhsView} from '../components/rhs/RhsView';
import {_resetForTesting as resetDecorators} from '../decorators/registry';
import {clearSelection} from '../decorators/selection';

import {registerCotPanel} from './index';

interface Props {
    event?: Record<string, unknown>;

    /** Several events, each merged over the same baseline. Overrides `event`. */
    events?: Array<Record<string, unknown>>;

    /** Registry keys, camelCased, merged over an empty block. */
    detail?: Record<string, string>;
    source?: string;
    fileName?: string;
    src?: string;
}

/**
 * The card and the sidebar together, so a test can click the card's button and
 * assert what the sidebar then shows.
 *
 * The map is never reachable: every fixture leaves the position unlinkable, so
 * neither surface mounts `CotMap` and the component tests stay free of the
 * feature store and WebGL.
 */
const CotPanelHarness: React.FC<Props> = ({
    event = {},
    events,
    detail = {},
    source = 'fence',
    fileName = '',
    src = '<event uid="ANDROID-1"/>',
}) => {
    useState(() => {
        resetDecorators();
        clearSelection();
        registerCotPanel();
        return true;
    });

    const payload: CotPayload = {
        source,
        lead: '',
        trail: '',
        src,
        fileId: '',
        fileName,
        events: (events ?? [event]).map((each, index) => ({
            uid: `ANDROID-${index + 1}`,
            cotClass: '',
            detailUnknown: '',
            detailDropped: '',
            detail: {...emptyDetail(), ...detail},
            flow: [],
            geometry: null,
            callsign: '',
            cotType: 'a-f-G-U-C',
            typeLabel: 'Friend ground',
            affiliation: 'friend',
            how: 'm-g',
            howLabel: 'Machine, GPS',
            time: '',
            timeQuery: '',
            start: '',
            startQuery: '',
            stale: '',
            staleQuery: '',
            staleAt: '',
            timeAt: '',
            format: '',
            value: '',
            lat: '',
            lon: '',
            positionNote: '',
            hae: '',
            ce: '',
            ceMeters: '',
            le: '',
            speed: '',
            course: '',
            group: '',
            role: '',
            remarks: '',
            parent: '',
            related: '',
            ...each,
        })),
    };

    return (
        <div data-testid='harness'>
            <div data-testid='rhs-title'><RhsTitle/></div>
            <div data-testid='rhs'><RhsView/></div>
            <CotCard
                payload={payload}
                createAt={0}
            />
        </div>
    );
};

export default CotPanelHarness;
