import {useEffect, useState} from 'react';

import type {AirportCoordinate, AirportDetails, AirportResponse} from './types';

import {pluginBaseUrl} from '../../plugin_url';
import {CACHE_TTL_MS} from '../../preferences/store';

/**
 * Four upper-case letters, matching identBody in the Go grammar.
 *
 * Upper case rather than any case is what makes the code in a link
 * self-canonicalizing: there is no case to fold, so one airfield is one URL.
 * The webapp keeps this shape only, never the label alternation: the grammar
 * that decides what a token IS lives in Go.
 *
 * Held to the Go copy by TestWebappAirportIdentShapeMatches.
 */
export const IDENT = /^[A-Z]{4}$/;

/**
 * `rejected` and `failed` are different answers and both surfaces treat them
 * differently, the same split `convert.ts` makes.
 *
 * `failed` means the request did not arrive: offline, a proxy, a restart.
 * `rejected` means the server looked at the link and said it is not one this
 * plugin issued, which for this decorator can only mean a hand-edited `v`.
 *
 * `ready` covers both "here is the airfield" and "this build's database does
 * not hold that code". The second is an answer rather than a failure, so it is
 * carried in the payload and not in the status.
 */
export type AirportStatus = 'loading' | 'ready' | 'failed' | 'rejected';

export interface AirportState {
    status: AirportStatus;
    data: AirportResponse | null;
}

const LOADING: AirportState = {status: 'loading', data: null};

/** Thrown for a 400, which is the server's verdict rather than an outage. */
class RejectedError extends Error {}

function endpoint(ident: string): string {
    return `${pluginBaseUrl()}/api/v1/airport?${new URLSearchParams({v: ident}).toString()}`;
}

/**
 * Reads the server's answer, refusing anything that is not the shape.
 *
 * Strict rather than coerced, for the reason `fromWire` is: a captive portal or
 * a transparent proxy, which is the ordinary DDIL failure, answers 200 with
 * something else entirely, and an unchecked cast would render `undefined` as an
 * airfield name.
 */
export function asAirport(body: unknown): AirportResponse {
    if (body === null || typeof body !== 'object' || Array.isArray(body)) {
        throw new Error('The server did not return an airfield.');
    }

    const wire = body as Record<string, unknown>;
    if (typeof wire.found !== 'boolean' || typeof wire.ident !== 'string' || !IDENT.test(wire.ident)) {
        throw new Error('The server did not return an airfield.');
    }

    const answer: AirportResponse = {found: wire.found, ident: wire.ident};
    if (!answer.found) {
        return answer;
    }

    answer.airport = asDetails(wire.airport);

    // Absent rather than zeroed, which is the whole reason the shape is
    // discriminated. Only read when it is there.
    if (wire.coordinate !== undefined) {
        answer.coordinate = asCoordinate(wire.coordinate);
    }

    return answer;
}

function asDetails(value: unknown): AirportDetails {
    if (value === null || typeof value !== 'object') {
        throw new Error('The server did not return an airfield.');
    }

    const wire = value as Record<string, unknown>;
    const fields = ['name', 'type', 'place', 'elevation', 'iata'] as const;
    for (const key of fields) {
        // hasOwn, so a name that happens to exist on Object.prototype is read
        // as absent rather than inherited.
        if (!Object.hasOwn(wire, key) || typeof wire[key] !== 'string') {
            throw new Error(`The server sent no ${key}.`);
        }
    }

    return {
        name: wire.name as string,
        type: wire.type as string,
        place: wire.place as string,
        elevation: wire.elevation as string,
        iata: wire.iata as string,
    };
}

function asCoordinate(value: unknown): AirportCoordinate {
    if (value === null || typeof value !== 'object') {
        throw new Error('The server did not return a coordinate.');
    }

    const wire = value as Record<string, unknown>;
    if (typeof wire.format !== 'string' || typeof wire.value !== 'string') {
        throw new Error('The server did not return a coordinate.');
    }
    if (wire.format === '' || wire.value === '') {
        throw new Error('The server returned an empty coordinate.');
    }

    // Region may legitimately be empty, so it is checked for its type and not
    // for its content: a position in no country is an answer, not an outage.
    if (typeof wire.region !== 'string') {
        throw new Error('The server did not return a region.');
    }

    return {format: wire.format, value: wire.value, region: wire.region};
}

let fetchTimeoutMs = 10000;

async function fetchAirport(ident: string): Promise<AirportResponse> {
    const controller = new AbortController();
    const timer = setTimeout(() => controller.abort(), fetchTimeoutMs);

    // The body read is INSIDE the bound, not after it. fetch resolves when the
    // headers arrive, so clearing the timer there leaves a server that sends
    // headers and then stalls the body unbounded, which is the whole defect
    // this exists to close: inflight is never cleared and every later caller
    // joins the dead promise for the life of the tab.
    try {
        const response = await fetch(endpoint(ident), {
            credentials: 'same-origin',
            signal: controller.signal,

            // Mattermost accepts session-cookie authentication only for
            // requests that could not have been a cross-site form post.
            headers: {'X-Requested-With': 'XMLHttpRequest'},
        });

        if (response.status === 400) {
            throw new RejectedError('not an airfield code this plugin issued');
        }
        if (!response.ok) {
            throw new Error(`The server returned ${response.status}.`);
        }

        const answer = asAirport(await response.json());

        // The answer has to be about the code that was asked for. Without this
        // a proxy or a stale route can put one airfield's details under
        // another's code, and the panel and the hover would disagree about the
        // same link.
        if (answer.ident !== ident) {
            throw new Error('The server answered about a different airfield.');
        }

        return answer;
    } finally {
        clearTimeout(timer);
    }
}

/*
 * Module state, so a channel full of airfield codes makes one request per code
 * rather than one per hover. A hover fires on pointer movement, so without this
 * pointing along a line of a USMTF message would put a request behind every
 * code the cursor crossed.
 *
 * Unlike convert.ts this cache HAS a lifetime, and the difference is a property
 * of the data rather than a preference. A conversion is a pure function of its
 * token: the projection is arithmetic and the polygons are compiled in, so the
 * same token converts the same forever. An airfield answer is a function of
 * (ident, BUILD): the database is refreshed with the plugin, codes are retired
 * and positions corrected. Caching forever would mean a reader with an open tab
 * kept seeing "not in this build's database" after an upgrade added the code,
 * with nothing on screen suggesting a reload would help.
 *
 * CACHE_TTL_MS is the constant preferences/store.ts and features/store.ts
 * already share, and TestWebappCacheLifetimeMatches pins it across languages. A
 * second number here would be a second thing to keep in step.
 */
interface CachedAnswer {
    state: AirportState;
    at: number;
}

const answers = new Map<string, CachedAnswer>();
const inflight = new Map<string, Promise<AirportState>>();

function now(): number {
    return Date.now();
}

function fresh(key: string): AirportState | null {
    const cached = answers.get(key);
    if (!cached) {
        return null;
    }
    if (now() - cached.at >= CACHE_TTL_MS) {
        answers.delete(key);
        return null;
    }

    return cached.state;
}

/*
 * Answers worth remembering are the ones the server DECIDED.
 *
 * `failed` is an outage and is never cached: remembering it would mean one bad
 * minute costs every airfield in the channel until the tab is reloaded. That is
 * the same split convert.ts and basemap.ts make.
 */
function remembered(state: AirportState): boolean {
    return state.status === 'ready' || state.status === 'rejected';
}

async function load(ident: string): Promise<AirportState> {
    try {
        return {status: 'ready', data: await fetchAirport(ident)};
    } catch (error: unknown) {
        return {
            status: error instanceof RejectedError ? 'rejected' : 'failed',
            data: null,
        };
    }
}

/*
 * One request per code, however many components ask at once.
 *
 * The in-flight map is what makes a hover and the click that follows it share a
 * single request rather than race. The cache is checked HERE and not only in
 * the hook: a cache only one caller consults is not a cache.
 */
export function request(ident: string): Promise<AirportState> {
    const answer = fresh(ident);
    if (answer) {
        return Promise.resolve(answer);
    }

    const pending = inflight.get(ident);
    if (pending) {
        return pending;
    }

    const started: Promise<AirportState> = load(ident).then((state) => {
        // Identity-checked: a request settling late must not delete a newer
        // attempt that has since taken the slot, which would send the next
        // joiner to the network with every appearance of being cached.
        if (inflight.get(ident) !== started) {
            return state;
        }
        if (remembered(state)) {
            answers.set(ident, {state, at: now()});
        }
        inflight.delete(ident);

        return state;
    });

    inflight.set(ident, started);

    return started;
}

/**
 * Fetches one airfield, through the cache above.
 *
 * Never throws and never leaves a surface with nothing to say.
 */
export function useAirport(ident: string): AirportState {
    const [state, setState] = useState<AirportState>(() => fresh(ident) ?? LOADING);
    const [current, setCurrent] = useState(ident);

    // Reset DURING RENDER, not in the effect, for the reason useConversion
    // documents: the sidebar keeps the panel mounted across a change of
    // selection, and clearing in an effect leaves one committed frame showing
    // the new code above the old airfield's name.
    if (current !== ident) {
        setCurrent(ident);
        setState(fresh(ident) ?? LOADING);
    }

    useEffect(() => {
        // Read the cache again here and adopt what it holds rather than bailing
        // out: a reply can land between the render above and this line, which
        // is exactly a hover starting the request and the click after it
        // mounting the panel before the reply arrives.
        const answered = fresh(ident);
        if (answered) {
            setState(answered);
            return undefined;
        }

        let live = true;

        request(ident).then((answer) => {
            if (live) {
                setState(answer);
            }
        });

        return () => {
            live = false;
        };
    }, [ident]);

    return state;
}

/** @internal exported for tests, which must not inherit another test's cache. */
export function _resetForTesting(): void { // eslint-disable-line no-underscore-dangle, @typescript-eslint/naming-convention
    answers.clear();
    inflight.clear();
    fetchTimeoutMs = 10000;
}

/** @internal shortens the hang timeout, so the test for it does not wait ten seconds. */
export function _setFetchTimeoutForTesting(ms: number): void { // eslint-disable-line no-underscore-dangle, @typescript-eslint/naming-convention
    fetchTimeoutMs = ms;
}
