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

/**
 * Harness for the airfield surfaces.
 *
 * Everything both of them show comes from one route, so the whole harness is a
 * stub of it plus a switch for which surface to mount. The reply is held until
 * the test releases it, so the loading state is observable rather than raced.
 */
export type Reply =
    | 'found'
    | 'unknown'
    | 'no-position'
    | 'unreadable-position'
    | 'unnamed'
    | 'sea-level'
    | 'rejected'
    | 'failed'
    | 'hold';

interface Props {
    surface: 'panel' | 'hover';
    ident: string;
    reply: Reply;

    /** Which map surfaces the admin has left on. All of them by default. */
    features?: Partial<Features>;

    /** A second code the surface can be swapped to without remounting. */
    second?: string;
}

const DETAILS: AirportDetails = {
    name: 'Indianapolis International Airport',
    type: 'Large Airport',
    place: 'Indianapolis, IN, US',
    elevation: '797 ft',
    iata: 'IND',
};

const FOUND: AirportResponse = {
    found: true,
    ident: 'KIND',
    airport: DETAILS,
    coordinate: {format: 'dd', value: '39.7173,-86.2944', region: 'United States of America (Natural Earth 110m)'},
};

const SECOND: AirportResponse = {
    found: true,
    ident: 'KJFK',
    airport: {
        name: 'John F Kennedy International Airport',
        type: 'Large Airport',
        place: 'New York, NY, US',
        elevation: '13 ft',
        iata: 'JFK',
    },
    coordinate: {format: 'dd', value: '40.6398,-73.7789', region: 'United States of America (Natural Earth 110m)'},
};

/** Everything the first airfield can put on screen. */
const FIRST_MARKERS = ['Indianapolis International Airport', 'Indianapolis, IN, US', '797 ft', 'IND'];

/** Notified once per airfield request the stub answers. */
let onRequest: (() => void) | null = null;

/** How many harnesses have set themselves up, so a test can tell a re-render from a remount. */
let setups = 0;

function identOf(url: string): string {
    return new URLSearchParams(url.slice(url.indexOf('?') + 1)).get('v') ?? '';
}

function bodyFor(reply: Reply, ident: string): AirportResponse {
    if (ident === SECOND.ident) {
        return SECOND;
    }
    if (reply === 'unknown') {
        return {found: false, ident: 'QZQZ'};
    }
    if (reply === 'no-position') {
        return {found: true, ident: FOUND.ident, airport: FOUND.airport};
    }
    if (reply === 'unreadable-position') {
        return {...FOUND, coordinate: {format: 'dd', value: '999.0000,999.0000', region: ''}};
    }
    if (reply === 'unnamed') {
        return {...FOUND, airport: {...DETAILS, name: '', elevation: '', iata: ''}};
    }
    if (reply === 'sea-level') {
        return {...FOUND, airport: {...DETAILS, elevation: '0 ft'}};
    }

    return FOUND;
}

/** Counts committed frames still showing the first airfield after the code changed. */
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

/** What the sidebar switched to, so a swap is observable rather than inferred. */
function describeSelection(selection: Selection | null): string {
    if (!selection) {
        return '';
    }

    const payload = selection.payload as LocationPayload;
    return `${selection.type} ${payload.canonical}`;
}

const AirportHarness: React.FC<Props> = ({surface, ident, reply, features, second}) => {
    const [requests, setRequests] = React.useState(0);

    // Before the first render that mounts the surface, so no request escapes
    // and no answer is inherited from a previous test.
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

            // Everything that is not the airfield lookup is delegated, so the
            // basemap, the glyph ranges and the map worker reach the real
            // fetch. Answering them here would leave the map permanently on
            // "Loading map...".
            if (!url.includes('/api/v1/airport')) {
                return real(input, init);
            }

            onRequest?.();
            if (reply === 'hold') {
                // Never settles, which is what makes the in-flight state
                // observable rather than raced against a reply.
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
                json: async () => bodyFor(reply, identOf(url)),
            } as unknown as Response;
        }) as typeof globalThis.fetch;

        return setups;
    });

    const [selection, setSelection] = React.useState<Selection | null>(null);
    React.useEffect(() => subscribe(setSelection), []);

    // In the render body, not in the setup above, because the initializer does
    // not re-run on component.update(): a features prop passed to an update was
    // silently ignored and the surface kept the answer from its first mount.
    setStubbedFeatures(features ?? {mapPanel: true, mapInline: true, mapPage: true});

    const [shown, setShown] = React.useState(ident);
    const [ref, stale] = useStaleFrameCounter(shown);

    React.useEffect(() => () => {
        onRequest = null;
    }, []);

    if (!setup) {
        return null;
    }

    return (
        <div>
            <p data-testid='selection'>{describeSelection(selection)}</p>
            <div ref={ref}>
                {surface === 'panel' && <AirportPanel payload={{ident: shown}}/>}
                {surface === 'hover' && (
                    <div data-testid='card'><AirportHover payload={{ident: shown}}/></div>
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
