import {useEffect, useState} from 'react';

import type {AirportCoordinate, AirportDetails, AirportEnd, AirportFrequency, AirportResponse, AirportRunway} from './types';

import {pluginBaseUrl} from '../../plugin_url';
import {CACHE_TTL_MS} from '../../preferences/store';

import type {AirportPayload} from './index';

export const IDENT = /^[A-Z]{4}$/;

export const IATA = /^[A-Z]{3}$/;

export type AirportStatus = 'loading' | 'ready' | 'failed' | 'rejected';

export interface AirportState {
    status: AirportStatus;
    data: AirportResponse | null;
}

const LOADING: AirportState = {status: 'loading', data: null};

class RejectedError extends Error {}

function endpoint(payload: AirportPayload): string {
    const param = payload.key === 'iata' ? 'i' : 'v';
    return `${pluginBaseUrl()}/api/v1/airport?${new URLSearchParams({[param]: payload.code}).toString()}`;
}

function cacheKey(payload: AirportPayload): string {
    return `${payload.key}:${payload.code}`;
}

function requireString(wire: Record<string, unknown>, key: string, what: string): string {
    if (!Object.hasOwn(wire, key) || typeof wire[key] !== 'string') {
        throw new Error(`The server sent no ${key} for ${what}.`);
    }
    return wire[key] as string;
}

function asRecord(value: unknown, what: string): Record<string, unknown> {
    if (value === null || typeof value !== 'object' || Array.isArray(value)) {
        throw new Error(`The server did not return ${what}.`);
    }
    return value as Record<string, unknown>;
}

export function asAirport(body: unknown): AirportResponse {
    const wire = asRecord(body, 'an airfield');
    if (typeof wire.found !== 'boolean') {
        throw new Error('The server did not return an airfield.');
    }

    const ident = requireString(wire, 'ident', 'the airfield');
    const iata = requireString(wire, 'iata', 'the airfield');
    if (iata !== '' && !IATA.test(iata)) {
        throw new Error('The server returned a malformed IATA code.');
    }

    if (!wire.found) {
        if (ident !== '' && !IDENT.test(ident)) {
            throw new Error('The server returned a malformed airfield code.');
        }
        return {found: false, ident, iata};
    }

    if (!IDENT.test(ident)) {
        throw new Error('The server returned a malformed airfield code.');
    }

    const answer: AirportResponse = {found: true, ident, iata, airport: asDetails(wire.airport)};

    if (wire.coordinate !== undefined) {
        answer.coordinate = asCoordinate(wire.coordinate);
    }

    return answer;
}

function asDetails(value: unknown): AirportDetails {
    const wire = asRecord(value, 'an airfield');

    if (!Array.isArray(wire.runways) || !Array.isArray(wire.frequencies)) {
        throw new Error('The server sent no runways or frequencies.');
    }

    return {
        name: requireString(wire, 'name', 'the airfield'),
        type: requireString(wire, 'type', 'the airfield'),
        place: requireString(wire, 'place', 'the airfield'),
        elevation: requireString(wire, 'elevation', 'the airfield'),
        iata: requireString(wire, 'iata', 'the airfield'),
        military: requireString(wire, 'military', 'the airfield'),
        runways: wire.runways.map(asRunway),
        frequencies: wire.frequencies.map(asFrequency),
    };
}

function asRunway(value: unknown): AirportRunway {
    const wire = asRecord(value, 'a runway');

    const runway: AirportRunway = {
        designation: requireString(wire, 'designation', 'a runway'),
        summary: requireString(wire, 'summary', 'a runway'),
        length: requireString(wire, 'length', 'a runway'),
        width: requireString(wire, 'width', 'a runway'),
        surface: requireString(wire, 'surface', 'a runway'),
        lighted: requireString(wire, 'lighted', 'a runway'),
        closed: requireString(wire, 'closed', 'a runway'),
    };

    if (wire.ends !== undefined) {
        if (!Array.isArray(wire.ends) || wire.ends.length !== 2) {
            throw new Error('The server sent a runway with something other than two ends.');
        }
        runway.ends = [asEnd(wire.ends[0]), asEnd(wire.ends[1])];
    }

    return runway;
}

function asEnd(value: unknown): AirportEnd {
    const wire = asRecord(value, 'a runway end');
    const end = {format: requireString(wire, 'format', 'a runway end'), value: requireString(wire, 'value', 'a runway end')};
    if (end.format === '' || end.value === '') {
        throw new Error('The server returned an empty runway end.');
    }
    return end;
}

function asFrequency(value: unknown): AirportFrequency {
    const wire = asRecord(value, 'a frequency');
    return {
        type: requireString(wire, 'type', 'a frequency'),
        description: requireString(wire, 'description', 'a frequency'),
        mhz: requireString(wire, 'mhz', 'a frequency'),
    };
}

function asCoordinate(value: unknown): AirportCoordinate {
    const wire = asRecord(value, 'a coordinate');
    if (typeof wire.format !== 'string' || typeof wire.value !== 'string') {
        throw new Error('The server did not return a coordinate.');
    }
    if (wire.format === '' || wire.value === '') {
        throw new Error('The server returned an empty coordinate.');
    }
    if (typeof wire.region !== 'string') {
        throw new Error('The server did not return a region.');
    }

    return {format: wire.format, value: wire.value, region: wire.region};
}

let fetchTimeoutMs = 10000;

function answersTheQuestion(answer: AirportResponse, payload: AirportPayload): boolean {
    if (payload.key === 'iata') {
        return answer.iata === payload.code;
    }
    return answer.ident === payload.code;
}

async function fetchAirport(payload: AirportPayload): Promise<AirportResponse> {
    const controller = new AbortController();
    const timer = setTimeout(() => controller.abort(), fetchTimeoutMs);

    try {
        const response = await fetch(endpoint(payload), {
            credentials: 'same-origin',
            signal: controller.signal,
            headers: {'X-Requested-With': 'XMLHttpRequest'},
        });

        if (response.status === 400) {
            throw new RejectedError('not an airfield code this plugin issued');
        }
        if (!response.ok) {
            throw new Error(`The server returned ${response.status}.`);
        }

        const answer = asAirport(await response.json());

        if (!answersTheQuestion(answer, payload)) {
            throw new Error('The server answered about a different airfield.');
        }

        return answer;
    } finally {
        clearTimeout(timer);
    }
}

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

function remembered(state: AirportState): boolean {
    return state.status === 'ready' || state.status === 'rejected';
}

async function load(payload: AirportPayload): Promise<AirportState> {
    try {
        return {status: 'ready', data: await fetchAirport(payload)};
    } catch (error: unknown) {
        return {
            status: error instanceof RejectedError ? 'rejected' : 'failed',
            data: null,
        };
    }
}

export function request(payload: AirportPayload): Promise<AirportState> {
    const key = cacheKey(payload);

    const answer = fresh(key);
    if (answer) {
        return Promise.resolve(answer);
    }

    const pending = inflight.get(key);
    if (pending) {
        return pending;
    }

    const started: Promise<AirportState> = load(payload).then((state) => {
        if (inflight.get(key) !== started) {
            return state;
        }
        if (remembered(state)) {
            answers.set(key, {state, at: now()});
        }
        inflight.delete(key);

        return state;
    });

    inflight.set(key, started);

    return started;
}

export function useAirport(payload: AirportPayload): AirportState {
    const key = cacheKey(payload);
    const [state, setState] = useState<AirportState>(() => fresh(key) ?? LOADING);
    const [current, setCurrent] = useState(key);

    if (current !== key) {
        setCurrent(key);
        setState(fresh(key) ?? LOADING);
    }

    useEffect(() => {
        const answered = fresh(key);
        if (answered) {
            setState(answered);
            return undefined;
        }

        let live = true;

        request(payload).then((answer) => {
            if (live) {
                setState(answer);
            }
        });

        return () => {
            live = false;
        };
    }, [key]); // eslint-disable-line react-hooks/exhaustive-deps

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
