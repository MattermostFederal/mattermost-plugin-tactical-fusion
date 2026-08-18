import type {Map as MapLibreMap} from 'maplibre-gl';
import React, {useEffect, useRef, useState} from 'react';

import type {Conversion} from './convert';
import {_resetConversionsForTesting} from './convert';
import {parseCanonical} from './format';
import type {LocationFormat} from './format';
import LocationInline from './LocationInline';
import {_setMapObserverForTesting} from './map/LocationMap';

import {_resetForTesting as resetFeatures} from '../../features/store';
import {featuresReply, isFeaturesRequest, setStubbedFeatures} from '../../features/stub_fetch';
import type {Features} from '../../features/types';
import {_resetForTesting as resetPreferences} from '../../preferences/store';

import type {LocationPayload} from './index';

/**
 * Harness for the map drawn under a post.
 *
 * Borrows the hover harness's fetch stub, including the part that matters most:
 * every request that is not a conversion is delegated to the real `fetch`, or
 * the archive, the glyph ranges and the worker would all be swallowed and no
 * map would ever load.
 *
 * What it adds is a viewport. The component mounts the map only while its post
 * is near the screen, so the harness puts a tall spacer above it and lets the
 * test scroll, which is the only way to drive an IntersectionObserver.
 */

const LOS_ANGELES: Conversion = {
    mgrs: '11S LT 8463 6908',
    utm: '11S 384640E 3769080N',
    decimal: '34.0561° N, 118.2500° W',
    dms: '34°03\'22.0"N 118°15\'00.0"W',
    ddm: '34°03.366\'N 118°15.000\'W',
    usmtf: '340322.0N1181500.0W',
    georef: 'EJBE45000336',
    gars: '124LJ47',
    pluscode: '85633Q42+C2R',
    region: 'United States of America (Natural Earth 110m)',
    lat: 34.0561,
    lon: -118.25,
};

type Outcome = 'ok' | 'reject';

const realFetch = window.fetch.bind(window);

let settle: (() => void) | null = null;
let outcome: Outcome = 'ok';
let conversions = 0;

window.fetch = ((input: RequestInfo | URL, init?: RequestInit) => {
    const url = String(input);
    if (isFeaturesRequest(url)) {
        return Promise.resolve(featuresReply());
    }
    if (!url.includes('/api/v1/convert')) {
        return realFetch(input as RequestInfo, init);
    }

    conversions += 1;

    return new Promise((resolve) => {
        settle = () => {
            if (outcome === 'reject') {
                resolve({ok: false, status: 400, json: () => Promise.resolve({})} as Response);
                return;
            }
            resolve({ok: true, status: 200, json: () => Promise.resolve(LOS_ANGELES)} as Response);
        };
    });
}) as typeof fetch;

/** How tall the spacer is, so the post starts well outside the mount margin. */
const SPACER_PX = 4000;

interface Props {
    format?: LocationFormat;
    canonical?: string;
    outcome?: Outcome;

    /** Start with the post already on screen, for the tests about the map. */
    inView?: boolean;

    /** Map surfaces the stubbed admin has left on. Every one, by default. */
    features?: Partial<Features>;
}

const LocationInlineHarness: React.FC<Props> = ({
    format = 'dd',
    canonical = '34.0561,-118.2500',
    outcome: requested = 'ok',
    inView = false,
    features = {},
}) => {
    outcome = requested;
    setStubbedFeatures(features);

    // At render rather than in an effect, because child effects run before
    // parent ones: the component's own useFeatures would have already loaded
    // and cached, and resetting afterwards left the store empty with nothing
    // left to re-trigger a load. Every surface then read as switched off and no
    // map was ever mounted.
    useState(() => {
        resetFeatures();
        return null;
    });

    const live = useRef<MapLibreMap | null>(null);
    const [maps, setMaps] = useState({built: 0, released: 0});

    useEffect(() => {
        conversions = 0;
        _resetConversionsForTesting();
        resetPreferences();
        _setMapObserverForTesting((instance) => {
            live.current = instance;
            setMaps((seen) => (instance ?
                {...seen, built: seen.built + 1} :
                {...seen, released: seen.released + 1}));
        });

        return () => _setMapObserverForTesting(null);
    }, []);

    const payload: LocationPayload = {
        coord: parseCanonical(format, canonical),
        format,
        canonical,
        raw: canonical,
    };

    return (
        <div>
            {!inView && <div style={{height: SPACER_PX}}/>}
            <LocationInline payload={payload}/>
            <div style={{height: SPACER_PX}}/>
            <div style={{position: 'fixed', top: 0, right: 0, background: '#fff'}}>
                <button
                    type='button'
                    onClick={() => settle?.()}
                >{'answer the conversion'}</button>
                <output data-testid='maps-built'>{String(maps.built)}</output>
                <output data-testid='maps-released'>{String(maps.released)}</output>
                <output data-testid='conversions'>{String(conversions)}</output>
            </div>
        </div>
    );
};

export default LocationInlineHarness;
