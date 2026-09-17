import {MAX_ZOOM, MERCATOR_LIMIT, isRenderable} from './span';

export interface Camera {
    lat: number;
    lon: number;
    zoom: number;
}

const HASH_KEY = 'map';
const COORD_PLACES = 5;
const ZOOM_PLACES = 2;
const NUMBER = /^-?\d+(\.\d+)?$/;
const LAT_LIMIT = Math.floor(MERCATOR_LIMIT * (10 ** COORD_PLACES)) / (10 ** COORD_PLACES);

export function hashForCamera(camera: Camera): string {
    const zoom = camera.zoom.toFixed(ZOOM_PLACES);
    const lat = clampLatitude(camera.lat).toFixed(COORD_PLACES);
    const lon = wrapLongitude(camera.lon).toFixed(COORD_PLACES);

    return `#${HASH_KEY}=${zoom}/${lat}/${lon}`;
}

export function cameraFromHash(hash: string): Camera | null {
    const stated = hash.startsWith('#') ? hash.slice(1) : hash;
    if (!stated.startsWith(`${HASH_KEY}=`)) {
        return null;
    }

    const parts = stated.slice(HASH_KEY.length + 1).split('/');
    if (parts.length !== 3 || !parts.every((part) => NUMBER.test(part))) {
        return null;
    }

    const zoom = Number(parts[0]);
    const lat = Number(parts[1]);
    const lon = Number(parts[2]);

    if (zoom < 0 || zoom > MAX_ZOOM || !isRenderable(lat) || Math.abs(lon) > 180) {
        return null;
    }

    return {lat, lon, zoom};
}

export function openingCamera(): Camera | null {
    if (typeof window === 'undefined') {
        return null;
    }

    return cameraFromHash(window.location.hash);
}

function clampLatitude(lat: number): number {
    return Math.max(-LAT_LIMIT, Math.min(LAT_LIMIT, lat));
}

function wrapLongitude(lon: number): number {
    return ((((lon + 180) % 360) + 360) % 360) - 180;
}
