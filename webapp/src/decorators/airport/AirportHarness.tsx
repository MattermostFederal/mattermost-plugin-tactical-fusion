import React from 'react';

import {_resetForTesting as resetAirports} from './airport';
import AirportHover from './AirportHover';
import AirportPanel from './AirportPanel';
import type {AirportDetails, AirportResponse} from './types';

import {_resetForTesting as resetFeatures} from '../../features/store';
import {featuresReply, isFeaturesRequest, setStubbedFeatures} from '../../features/stub_fetch';
import type {Features} from '../../features/types';
import type {LocationPayload} from '../location';
import type {Selection} from '../selection';
import {_resetForTesting as resetSelection, subscribe} from '../selection';

import type {AirportKey, AirportPayload} from './index';

export type Reply =
    | 'found'
    | 'unknown'
    | 'no-position'
    | 'unreadable-position'
    | 'unnamed'
    | 'sea-level'
    | 'military'
    | 'bare'
    | 'rejected'
    | 'failed'
    | 'hold';

interface Props {
    surface: 'panel' | 'hover';
    ident: string;
    reply: Reply;
    lookup?: AirportKey;
    features?: Partial<Features>;
    second?: string;
}

export const RUNWAYS: AirportDetails['runways'] = [
    {
        designation: '05L/23R',
        summary: '11,200 x 150 ft, Concrete, lighted',
        length: '11,200 ft',
        width: '150 ft',
        surface: 'Concrete',
        lighted: 'Lighted',
        closed: '',
        ends: [{format: 'dd', value: '39.7009,-86.3054'}, {format: 'dd', value: '39.7263,-86.2733'}],
    },
    {
        designation: '14/32',
        summary: '7,280 x 150 ft, Asphalt',
        length: '7,280 ft',
        width: '150 ft',
        surface: 'Asphalt',
        lighted: '',
        closed: '',
    },
];

export const FREQUENCIES: AirportDetails['frequencies'] = [
    {type: 'TWR', description: '', mhz: '120.900'},
    {type: 'ATIS', description: 'Arrival', mhz: '134.250'},
];

const DETAILS: AirportDetails = {
    name: 'Indianapolis International Airport',
    type: 'Large Airport',
    place: 'Indianapolis, IN, US',
    elevation: '797 ft',
    iata: 'IND',
    military: '',
    runways: RUNWAYS,
    frequencies: FREQUENCIES,
};

const FOUND: AirportResponse = {
    found: true,
    ident: 'KIND',
    iata: 'IND',
    airport: DETAILS,
    coordinate: {format: 'dd', value: '39.7173,-86.2944', region: 'United States of America (Natural Earth 110m)'},
};

const SECOND: AirportResponse = {
    found: true,
    ident: 'KJFK',
    iata: 'JFK',
    airport: {
        name: 'John F Kennedy International Airport',
        type: 'Large Airport',
        place: 'New York, NY, US',
        elevation: '13 ft',
        iata: 'JFK',
        military: '',
        runways: [],
        frequencies: [],
    },
    coordinate: {format: 'dd', value: '40.6398,-73.7789', region: 'United States of America (Natural Earth 110m)'},
};

const FIRST_MARKERS = ['Indianapolis International Airport', 'Indianapolis, IN, US', '797 ft', 'IND'];

let onRequest: (() => void) | null = null;

let setups = 0;

function codeOf(url: string): {key: AirportKey; code: string} {
    const params = new URLSearchParams(url.slice(url.indexOf('?') + 1));
    const iata = params.get('i');
    if (iata !== null) {
        return {key: 'iata', code: iata};
    }
    return {key: 'icao', code: params.get('v') ?? ''};
}

function bodyFor(reply: Reply, asked: {key: AirportKey; code: string}): AirportResponse {
    if (asked.code === SECOND.ident) {
        return SECOND;
    }
    if (reply === 'unknown') {
        return asked.key === 'iata' ? {found: false, ident: '', iata: asked.code} : {found: false, ident: asked.code, iata: ''};
    }
    if (reply === 'no-position') {
        return {found: true, ident: FOUND.ident, iata: FOUND.iata, airport: FOUND.airport};
    }
    if (reply === 'unreadable-position') {
        return {...FOUND, coordinate: {format: 'dd', value: '999.0000,999.0000', region: ''}};
    }
    if (reply === 'unnamed') {
        return {...FOUND, iata: '', airport: {...DETAILS, name: '', elevation: '', iata: ''}};
    }
    if (reply === 'sea-level') {
        return {...FOUND, airport: {...DETAILS, elevation: '0 ft'}};
    }
    if (reply === 'military') {
        return {...FOUND, airport: {...DETAILS, military: 'Air Force Base'}};
    }
    if (reply === 'bare') {
        return {...FOUND, airport: {...DETAILS, runways: [], frequencies: []}};
    }

    return FOUND;
}

function useStaleFrameCounter(code: string): [React.RefObject<HTMLDivElement | null>, number] {
    const ref = React.useRef<HTMLDivElement>(null);
    const [stale, setStale] = React.useState(0);
    const seen = React.useRef<string | null>(null);
    const opened = React.useRef(code);

    React.useLayoutEffect(() => {
        const node = ref.current;
        if (!node) {
            return;
        }

        const text = node.textContent ?? '';
        const key = `${code}\u0000${text}`;
        if (key === seen.current) {
            return;
        }
        seen.current = key;

        if (code !== opened.current && FIRST_MARKERS.some((m) => text.includes(m))) {
            setStale((n) => n + 1);
        }
    });

    return [ref, stale];
}

function describeSelection(selection: Selection | null): string {
    if (!selection) {
        return '';
    }

    const payload = selection.payload as LocationPayload;
    return `${selection.type} ${payload.canonical}`;
}

const AirportHarness: React.FC<Props> = ({surface, ident, reply, lookup, features, second}) => {
    const [requests, setRequests] = React.useState(0);

    const [setup] = React.useState(() => {
        setups += 1;
        resetAirports();
        resetSelection();
        resetFeatures();

        onRequest = () => setRequests((n) => n + 1);

        const real = globalThis.fetch;
        globalThis.fetch = (async (input: RequestInfo | URL, init?: RequestInit) => {
            const url = String(input);
            if (isFeaturesRequest(url)) {
                return featuresReply();
            }

            if (!url.includes('/api/v1/airport')) {
                return real(input, init);
            }

            onRequest?.();
            if (reply === 'hold') {
                return new Promise<Response>(() => {});
            }
            if (reply === 'failed') {
                throw new Error('offline');
            }
            if (reply === 'rejected') {
                return {status: 400, ok: false} as Response;
            }

            return {
                status: 200,
                ok: true,
                json: async () => bodyFor(reply, codeOf(url)),
            } as unknown as Response;
        }) as typeof globalThis.fetch;

        return setups;
    });

    const [selection, setSelection] = React.useState<Selection | null>(null);
    React.useEffect(() => subscribe(setSelection), []);

    setStubbedFeatures(features ?? {mapPanel: true, mapInline: true, mapPage: true});

    const [shown, setShown] = React.useState(ident);
    const [ref, stale] = useStaleFrameCounter(shown);

    React.useEffect(() => () => {
        onRequest = null;
    }, []);

    if (!setup) {
        return null;
    }

    const payload: AirportPayload = {key: lookup ?? 'icao', code: shown};

    return (
        <div>
            <p data-testid='selection'>{describeSelection(selection)}</p>
            <div ref={ref}>
                {surface === 'panel' && <AirportPanel payload={payload}/>}
                {surface === 'hover' && (
                    <div data-testid='card'><AirportHover payload={payload}/></div>
                )}
            </div>
            {second !== undefined && (
                <button
                    type='button'
                    onClick={() => setShown(second)}
                >
                    {'select the second airfield'}
                </button>
            )}
            <p data-testid='stale-frames'>{stale}</p>
            <p data-testid='setup'>{setup}</p>
            <p data-testid='requests'>{requests}</p>
        </div>
    );
};

export default AirportHarness;
