import type {GeoJSONSource, Map as MapLibreMap} from 'maplibre-gl';
import React, {useEffect, useRef, useState} from 'react';

import type {Conversion} from './convert';
import {_resetConversionsForTesting} from './convert';
import {parseCanonical} from './format';
import type {LocationFormat} from './format';
import LocationHover from './LocationHover';
import {_setMapObserverForTesting} from './map/LocationMap';

import {_resetForTesting as resetFeatures} from '../../features/store';
import {featuresReply, isFeaturesRequest, setStubbedFeatures} from '../../features/stub_fetch';
import type {Features} from '../../features/types';

import type {LocationPayload} from './index';

/**
 * Harness for the hover card.
 *
 * Two things have to be observable at once, which is why this borrows from both
 * of the harnesses either side of it: the conversion has to be held open the way
 * `LocationPanelHarness` holds it, and MapLibre's own state has to be readable
 * the way `LocationMapHarness` reads it. The hover contributes neither a table
 * nor a control, so everything it decides reaches the reader through the map's
 * pin, its cell and its accessible label, and nothing else.
 *
 * The critical difference from `LocationPanelHarness`: this one **delegates**
 * every request that is not a conversion to the real `fetch`. That harness
 * answers the preferences route and treats everything else as a conversion,
 * which would swallow the archive, the glyph ranges and the worker, and no map
 * would ever load.
 */

/*
 * 'reject' is the server saying the link is not one it issued (HTTP 400), which
 * is a VERDICT about a fixed token; 'fail' is an outage. The card treats them
 * differently and only had a way to reach one of them, so the branch that
 * refuses a hand-edited link was unreachable from the suite.
 */
type Outcome = 'ok' | 'fail' | 'reject';

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

/*
 * Keyed by the canonical token in the request, so a grid link is answered with
 * the position belonging to the token that asked for it. Both of these are the
 * same place, which is the point: a grid reference and the lat/lon spelling of
 * it must put the pin in the same square.
 */
const ANSWERS: Record<string, Conversion> = {
    '34.0561,-118.2500': LOS_ANGELES,
    '11SLT84636908': LOS_ANGELES,
    '89.9000,12.0000': {
        mgrs: '33X VN 00000 00000',
        utm: '33X 500000E 9987000N',
        decimal: '89.9000° N, 12.0000° E',
        dms: '89°54\'00.0"N 12°00\'00.0"E',
        ddm: '89°54.000\'N 12°00.000\'E',
        usmtf: '895400.0N0120000.0E',
        georef: 'NMNQ00005400',
        gars: '385QZ14',
        pluscode: 'CFXJW222+222',
        region: '',
        lat: 89.9,
        lon: 12,
    },
};

const realFetch = window.fetch.bind(window);

let settle: (() => void) | null = null;
let outcome: Outcome = 'ok';

/*
 * How many conversions the card has asked for, rendered into the DOM.
 *
 * The hover's claim that a switched-off card costs nothing was prose until this
 * existed: LocationInline proves the same property with a counter, and the hover
 * asserted only that nothing renders, which is also true while a request is in
 * flight.
 */
let conversions = 0;

window.fetch = ((input: RequestInfo | URL, init?: RequestInit) => {
    const url = String(input);
    if (isFeaturesRequest(url)) {
        return Promise.resolve(featuresReply());
    }
    if (!url.includes('/api/v1/convert')) {
        return realFetch(input as RequestInfo, init);
    }

    const value = new URL(url, 'http://localhost').searchParams.get('v') ?? '';
    conversions += 1;

    return new Promise((resolve, rejectRequest) => {
        settle = () => {
            if (outcome === 'fail') {
                rejectRequest(new Error('offline'));
                return;
            }

            if (outcome === 'reject') {
                resolve({ok: false, status: 400, json: () => Promise.resolve({})} as Response);
                return;
            }

            resolve({
                ok: true,
                status: 200,
                json: () => Promise.resolve(ANSWERS[value] ?? LOS_ANGELES),
            } as Response);
        };
    });
}) as typeof fetch;

/** How many features a source is carrying, or -1 when there is no map to ask. */
function countIn(map: MapLibreMap | null, id: string): number {
    const source = map?.getSource<GeoJSONSource>(id);
    if (!source) {
        return -1;
    }

    const data = (source.serialize() as {data?: {features?: unknown[]}}).data;

    return data?.features?.length ?? -1;
}

/**
 * The drawn cell's north-south extent in degrees, or -1 when there is none.
 *
 * The count alone cannot tell a 10 m square from a 100 km one, and the size is
 * the whole of what the rectangle states. `cellBounds` puts the pin at the
 * middle of the box, so this reads back exactly the `cellDegLat` the hover
 * passed in.
 */
function cellSpanIn(map: MapLibreMap | null): number {
    const source = map?.getSource<GeoJSONSource>('cell');
    if (!source) {
        return -1;
    }

    const data = (source.serialize() as {
        data?: {features?: Array<{geometry?: {coordinates?: number[][][]}}>};
    }).data;
    const ring = data?.features?.[0]?.geometry?.coordinates?.[0];
    if (!ring) {
        return -1;
    }

    const lats = ring.map((point) => point[1]);

    return Math.max(...lats) - Math.min(...lats);
}

interface Props {
    format?: LocationFormat;
    canonical?: string;
    raw?: string;
    outcome?: Outcome;

    /** Map surfaces the stubbed admin has left on. Every one, by default. */
    features?: Partial<Features>;
}

const LocationHoverHarness: React.FC<Props> = ({
    format = 'dd',
    canonical = '34.0561,-118.2500',
    raw,
    outcome: requested = 'ok',
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
        conversions = 0;
        return null;
    });

    const last = useRef<MapLibreMap | null>(null);
    const [reading, setReading] = useState({pin: -1, cell: -1, span: -1, center: 'none'});

    useEffect(() => {
        // Cleared here rather than in a test, so a token answered by one test
        // cannot arrive already cached in the next one and skip the loading
        // state every test in this file is about.
        _resetConversionsForTesting();
        _setMapObserverForTesting((instance) => {
            if (instance) {
                last.current = instance;
            }
        });

        return () => _setMapObserverForTesting(null);
    }, []);

    const read = () => {
        const map = last.current;
        const at = map?.getCenter() ?? null;

        setReading({
            pin: countIn(map, 'pin'),
            cell: countIn(map, 'cell'),
            span: cellSpanIn(map),
            center: at ? `${at.lat.toFixed(3)},${at.lng.toFixed(3)}` : 'none',
        });
    };

    const payload: LocationPayload = {
        coord: parseCanonical(format, canonical),
        format,
        canonical,
        raw: raw ?? canonical,
    };

    return (
        <div>
            <LocationHover payload={payload}/>
            <button
                type='button'
                onClick={() => settle?.()}
            >{'answer the conversion'}</button>
            <button
                type='button'
                onClick={read}
            >{'read the map'}</button>
            <output data-testid='pin-features'>{String(reading.pin)}</output>
            <output data-testid='cell-features'>{String(reading.cell)}</output>
            <output data-testid='cell-span'>{String(reading.span)}</output>
            <output data-testid='camera'>{reading.center}</output>
            <output data-testid='conversions'>{String(conversions)}</output>
        </div>
    );
};

export default LocationHoverHarness;
