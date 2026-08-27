import React, {useState} from 'react';

import CotCard from './CotCard';
import {_resetForTesting as resetEditing} from './editing';
import type {CotChecklist, CotPayload} from './types';
import {emptyDetail} from './types';

import {RhsTitle, RhsView} from '../components/rhs/RhsView';
import {_resetForTesting as resetDecorators} from '../decorators/registry';
import {clearSelection} from '../decorators/selection';
import {_resetForTesting as resetFeaturesStore} from '../features/store';
import {_resetForTesting as resetPreferencesStore} from '../preferences/store';

import {registerCotPanel} from './index';

// Stubbed at module scope for the reason CopyButtonHarness records.
export let copied = '';

Object.defineProperty(navigator, 'clipboard', {
    configurable: true,
    get: () => ({
        writeText: (value: string) => {
            copied = value;
            return Promise.resolve();
        },
    }),
});

interface Props {
    event?: Record<string, unknown>;

    /** Several events, each merged over the same baseline. Overrides `event`. */
    events?: Array<Record<string, unknown>>;

    /** Registry keys, camelCased, merged over an empty block. */
    detail?: Record<string, string>;
    source?: string;
    fileName?: string;
    checklist?: CotChecklist | null;
    src?: string;

    /**
     * A second card, over its own payload.
     *
     * The sidebar keeps one panel mounted across a change of selection, so a
     * second card is the only way to exercise what the panel does when the
     * reader clicks a different event while it is open.
     */
    second?: Record<string, unknown>;
}

/**
 * The card and the sidebar together, so a test can click the card's button and
 * assert what the sidebar then shows.
 *
 * The map is never reachable: every fixture leaves the position unlinkable, so
 * neither surface mounts `CotMap` and the component tests stay free of WebGL.
 *
 * The preferences store is reset on mount, because it caches the blob for the
 * life of the page and a panel that reads a reader's hidden sections needs its
 * own. A test that cares what is hidden stubs the route as well; one that does
 * not gets the defaults, which is every section shown.
 */
const CotPanelHarness: React.FC<Props> = ({
    event = {},
    events,
    detail = {},
    checklist = null,
    source = 'fence',
    fileName = '',
    src = '<event uid="ANDROID-1"/>',
    second,
}) => {
    useState(() => {
        resetDecorators();
        clearSelection();
        resetEditing();
        resetPreferencesStore();
        resetFeaturesStore();
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
            checklist,
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
            <CotCard payload={payload}/>
            {second && (
                <div data-testid='second-card'>
                    <CotCard payload={{...payload, events: [{...payload.events[0], ...second}]}}/>
                </div>
            )}
        </div>
    );
};

export default CotPanelHarness;
