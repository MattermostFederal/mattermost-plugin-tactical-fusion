import type {AirportCoordinate, AirportEnd} from './types';

import location from '../location';
import type {LocationPayload} from '../location';
import type {MapShape} from '../location/map/paint';
import {isRenderable} from '../location/map/span';

export const AIRPORT_MAP_KIND = 'airport';

export const AIRFIELD_COLOR = '#b8770f';

const RUNWAY_WIDTH = '3';

export interface AirportMapRunway {
    designation: string;
    ends: [AirportEnd, AirportEnd];
}

export interface AirportMapPayload {
    ident: string;
    name: string;
    coordinate: AirportCoordinate;
    runways: AirportMapRunway[];
}

export function positionPayload(coordinate: {format: string; value: string}): LocationPayload | null {
    return location.fromParams(new URLSearchParams({f: coordinate.format, v: coordinate.value}));
}

export function runwayShapes(runways: ReadonlyArray<{ends?: [AirportEnd, AirportEnd]}>): MapShape[] {
    const shapes: MapShape[] = [];

    for (const runway of runways) {
        if (!runway.ends) {
            continue;
        }

        const ring: Array<{lat: number; lon: number}> = [];
        for (const end of runway.ends) {
            const coord = positionPayload(end)?.coord;
            if (!coord || !isRenderable(coord.lat.decimal)) {
                break;
            }
            ring.push({lat: coord.lat.decimal, lon: coord.lon.decimal});
        }

        if (ring.length === 2) {
            shapes.push({rings: [ring], closed: false, color: AIRFIELD_COLOR, width: RUNWAY_WIDTH});
        }
    }

    return shapes;
}

export function runwayLabel(drawn: number): string {
    if (drawn === 0) {
        return 'the airfield';
    }
    return `the airfield, with ${drawn} runway${drawn === 1 ? '' : 's'} drawn`;
}

function asString(wire: Record<string, unknown>, key: string): string | null {
    return typeof wire[key] === 'string' ? (wire[key] as string) : null;
}

function asEnd(value: unknown): AirportEnd | null {
    if (value === null || typeof value !== 'object') {
        return null;
    }
    const wire = value as Record<string, unknown>;
    const format = asString(wire, 'format');
    const end = asString(wire, 'value');
    if (format === null || end === null || format === '' || end === '') {
        return null;
    }
    return {format, value: end};
}

export function airportMapFromBlob(blob: unknown): AirportMapPayload | null {
    if (blob === null || typeof blob !== 'object' || Array.isArray(blob)) {
        return null;
    }
    const wire = blob as Record<string, unknown>;

    const ident = asString(wire, 'ident');
    const name = asString(wire, 'name');
    if (ident === null || name === null || !(/^[A-Z]{4}$/).test(ident)) {
        return null;
    }

    if (wire.coordinate === null || typeof wire.coordinate !== 'object') {
        return null;
    }
    const coordinate = wire.coordinate as Record<string, unknown>;
    const format = asString(coordinate, 'format');
    const value = asString(coordinate, 'value');
    const region = asString(coordinate, 'region');
    if (format === null || value === null || region === null || format === '' || value === '') {
        return null;
    }

    if (!Array.isArray(wire.runways)) {
        return null;
    }
    const runways: AirportMapRunway[] = [];
    for (const raw of wire.runways) {
        if (raw === null || typeof raw !== 'object') {
            return null;
        }
        const runway = raw as Record<string, unknown>;
        const designation = asString(runway, 'designation');
        if (designation === null || !Array.isArray(runway.ends) || runway.ends.length !== 2) {
            return null;
        }
        const low = asEnd(runway.ends[0]);
        const high = asEnd(runway.ends[1]);
        if (low === null || high === null) {
            return null;
        }
        runways.push({designation, ends: [low, high]});
    }

    return {ident, name, coordinate: {format, value, region}, runways};
}
