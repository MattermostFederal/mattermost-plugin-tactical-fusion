import React, {useState} from 'react';

import {_resetForTesting as resetEditing} from './editing';
import GeoJsonCard from './GeoJsonCard';
import {registerGeoJsonPanel} from './panel';
import type {GeoJsonFeature, GeoJsonPayload} from './types';

import {RhsTitle, RhsView} from '../components/rhs/RhsView';
import {_resetForTesting as resetDecorators} from '../decorators/registry';
import {clearSelection} from '../decorators/selection';
import {_resetForTesting as resetFeaturesStore} from '../features/store';
import {_resetForTesting as resetPreferencesStore} from '../preferences/store';

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
    features?: Array<Partial<GeoJsonFeature>>;
    note?: string;
    propertiesDropped?: boolean;
    src?: string;
    counts?: Partial<GeoJsonPayload['counts']>;

    /**
     * A second card over its own payload.
     *
     * The sidebar keeps one panel mounted across a change of selection, so a
     * second card is the only way to exercise what the panel does when the
     * reader clicks a different document while it is open.
     */
    second?: Array<Partial<GeoJsonFeature>>;
}

const POINT_PART = {
    kind: 'Point' as const,
    rings: [[{lon: '-118.250000', lat: '34.056100', alt: ''}]],
    ringCounts: [],
};

function build(over: Partial<GeoJsonFeature>, index: number): GeoJsonFeature {
    return {
        name: `Feature ${index + 1}`,
        kind: 'Point',
        note: '',
        format: '',
        value: '',
        length: '',
        area: '',
        color: '',
        width: '',
        lineOpacity: '',
        fillOpacity: '',
        markerSize: '',
        parts: [POINT_PART],
        properties: [],
        ...over,
    };
}

/**
 * The card and the sidebar together, so a test can click the card's button and
 * assert what the sidebar then shows.
 *
 * The map is never reachable: the features store is reset, so `mapPanel` and
 * `mapInline` are both false and neither surface mounts `GeoJsonMap`. That
 * keeps these tests free of WebGL; the map has its own tests on `LocationMap`.
 *
 * The preferences store is reset because it caches the blob for the life of the
 * page and a panel that reads a reader's hidden sections needs its own. A test
 * that cares what is hidden stubs the route as well; one that does not gets the
 * defaults, which is every section shown.
 */
const GeoJsonPanelHarness: React.FC<Props> = ({
    features = [{}],
    note = '',
    propertiesDropped = false,
    src = '{"type":"Point","coordinates":[-118.250000,34.056100]}',
    counts = {},
    second,
}) => {
    useState(() => {
        resetDecorators();
        clearSelection();
        resetEditing();
        resetPreferencesStore();
        resetFeaturesStore();
        registerGeoJsonPanel();
        return true;
    });

    const payload: GeoJsonPayload = {
        source: 'fence',
        lead: '',
        trail: '',
        src,
        fileId: '',
        fileName: '',
        note,
        unplaceable: false,
        propertiesDropped,
        counts: {
            features: features.length,
            points: features.length,
            lines: 0,
            polygons: 0,
            collections: 0,
            unlocated: 0,
            undrawable: 0,
            ...counts,
        },
        features: features.map(build),
    };

    return (
        <div data-testid='harness'>
            <div data-testid='rhs-title'><RhsTitle/></div>
            <div data-testid='rhs'><RhsView/></div>
            <GeoJsonCard payload={payload}/>
            {second && (
                <div data-testid='second-card'>
                    <GeoJsonCard
                        payload={{
                            ...payload,
                            counts: {...payload.counts, features: second.length, points: second.length},
                            features: second.map(build),
                        }}
                    />
                </div>
            )}
        </div>
    );
};

export default GeoJsonPanelHarness;
