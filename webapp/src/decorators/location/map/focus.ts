import {unwrapLongitudes} from './span';

export const POINT_FOCUS_ZOOM = 14;

export interface MapFocus {
    seq: number;
    box: [[number, number], [number, number]] | null;
    center: {lat: number; lon: number};
}

export function focusOn(positions: ReadonlyArray<{lat: number; lon: number}>, seq: number): MapFocus | null {
    if (positions.length === 0) {
        return null;
    }

    const lons = unwrapLongitudes(positions.map((position) => position.lon));
    const lats = positions.map((position) => position.lat);
    const west = Math.min(...lons);
    const east = Math.max(...lons);
    const south = Math.min(...lats);
    const north = Math.max(...lats);
    const center = {lat: (south + north) / 2, lon: (west + east) / 2};

    if (west === east && south === north) {
        return {seq, box: null, center};
    }

    return {seq, box: [[west, south], [east, north]], center};
}
