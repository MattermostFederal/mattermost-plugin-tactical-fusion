import type {AirportCoordinate, AirportDetails, AirportEnd, AirportFrequency, AirportResponse, AirportRunway} from './types';

import {pluginBaseUrl} from '../../plugin_url';
import type {Answer, AnswerStatus} from '../cached_client';
import {createCachedClient} from '../cached_client';

import type {AirportPayload} from './index';

export const IDENT = /^[A-Z]{4}$/;

export const IATA = /^[A-Z]{3}$/;

export type AirportStatus = AnswerStatus;

export type AirportState = Answer<AirportResponse>;

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

function answersTheQuestion(answer: AirportResponse, payload: AirportPayload): boolean {
    if (payload.key === 'iata') {
        return answer.iata === payload.code;
    }
    return answer.ident === payload.code;
}

export function readAirport(body: unknown, payload: AirportPayload): AirportResponse {
    const answer = asAirport(body);
    if (!answersTheQuestion(answer, payload)) {
        throw new Error('The server answered about a different airfield.');
    }
    return answer;
}

const client = createCachedClient<AirportPayload, AirportResponse>({
    endpoint,
    cacheKey,
    read: readAirport,
});

export const {request, useAnswer: useAirport} = client;

export const {_resetForTesting, _setFetchTimeoutForTesting} = client; // eslint-disable-line @typescript-eslint/naming-convention
