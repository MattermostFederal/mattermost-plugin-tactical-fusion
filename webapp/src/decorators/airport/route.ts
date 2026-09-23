import {AIRFIELD_COLOR, positionPayload} from './map';

import type {MapMarker} from '../location/map/overlay';
import type {MapShape} from '../location/map/paint';
import {isRenderable} from '../location/map/span';

import type {AirportKey, AirportPayload} from './index';

export const AIRFIELDS_POST_TYPE = 'custom_tf_airfields';

export const AIRFIELDS_PROPS_KEY = 'tactical_fusion_airfields';

export const AIRFIELDS_PROPS_VERSION = 1;

export const MAX_ROUTE_AIRFIELDS = 64;

const ROUTE_COLOR = AIRFIELD_COLOR;

const LEG_WIDTH = '2';

export interface RouteAirfield {
    ident: string;
    code: string;
    name: string;
    format: string;
    value: string;
}

export interface RoutePayload {
    airfields: RouteAirfield[];
    postId: string;
}

function readString(raw: Record<string, unknown>, key: string): string | null {
    return typeof raw[key] === 'string' ? (raw[key] as string) : null;
}

function readEntry(value: unknown): RouteAirfield | null {
    if (value === null || typeof value !== 'object' || Array.isArray(value)) {
        return null;
    }
    const rawEntry = value as Record<string, unknown>;

    const ident = readString(rawEntry, 'ident');
    const code = readString(rawEntry, 'code');
    const name = readString(rawEntry, 'name');
    if (ident === null || code === null || name === null || !(/^[A-Z]{4}$/).test(ident) || !(/^[A-Z]{3,4}$/).test(code)) {
        return null;
    }

    const format = readString(rawEntry, 'format') ?? '';
    const position = readString(rawEntry, 'value') ?? '';
    if ((format === '') !== (position === '')) {
        return null;
    }

    return {ident, code, name, format, value: position};
}

export function airfieldsFromProps(props: unknown): RoutePayload | null {
    if (props === null || typeof props !== 'object') {
        return null;
    }
    const carried = (props as Record<string, unknown>)[AIRFIELDS_PROPS_KEY];
    if (carried === null || typeof carried !== 'object' || Array.isArray(carried)) {
        return null;
    }
    const rawBlob = carried as Record<string, unknown>;

    if (rawBlob.version !== AIRFIELDS_PROPS_VERSION) {
        return null;
    }
    if (!Array.isArray(rawBlob.airfields) || rawBlob.airfields.length === 0 || rawBlob.airfields.length > MAX_ROUTE_AIRFIELDS) {
        return null;
    }

    const airfields: RouteAirfield[] = [];
    for (const raw of rawBlob.airfields) {
        const entry = readEntry(raw);
        if (entry === null) {
            return null;
        }
        airfields.push(entry);
    }

    return {airfields, postId: ''};
}

export function payloadFor(airfield: RouteAirfield): AirportPayload {
    const key: AirportKey = airfield.code === airfield.ident ? 'icao' : 'iata';
    return {key, code: airfield.code};
}

function placed(airfield: RouteAirfield): {lat: number; lon: number} | null {
    if (airfield.format === '') {
        return null;
    }
    const coord = positionPayload({format: airfield.format, value: airfield.value})?.coord;
    if (!coord || !isRenderable(coord.lat.decimal)) {
        return null;
    }
    return {lat: coord.lat.decimal, lon: coord.lon.decimal};
}

export function routeMarkers(payload: RoutePayload): MapMarker[] {
    const markers: MapMarker[] = [];
    for (const airfield of payload.airfields) {
        const point = placed(airfield);
        if (point !== null) {
            markers.push({...point, color: ROUTE_COLOR});
        }
    }
    return markers;
}

export function routeLegs(payload: RoutePayload): MapShape[] {
    const legs: MapShape[] = [];
    let run: Array<{lat: number; lon: number}> = [];

    const flush = () => {
        if (run.length >= 2) {
            legs.push({rings: [run], closed: false, color: ROUTE_COLOR, width: LEG_WIDTH});
        }
        run = [];
    };

    for (const airfield of payload.airfields) {
        const point = placed(airfield);
        if (point === null) {
            flush();
            continue;
        }
        run.push(point);
    }
    flush();

    return legs;
}

export function drawsNothing(payload: RoutePayload): boolean {
    return routeMarkers(payload).length === 0;
}

export function routeLabel(payload: RoutePayload): string {
    const drawn = routeMarkers(payload).length;
    const legs = routeLegs(payload).reduce((n, leg) => n + (leg.rings[0].length - 1), 0);
    const airfields = `${drawn} airfield${drawn === 1 ? '' : 's'}`;
    if (legs === 0) {
        return airfields;
    }
    return `${airfields} and ${legs} leg${legs === 1 ? '' : 's'} in message order`;
}
