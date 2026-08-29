import type {MapEllipse, MapMarker} from './overlay';
import type {MapShape} from './paint';
import {DEGREE_METERS, MERCATOR_LIMIT, spansTheWorld, unwrapLongitudes} from './span';
import type {View} from './view';

/**
 * The box the whole overlay occupies, so the camera frames all of it.
 *
 * Every marker and every ring of every shape goes through ONE unwrap, not one
 * per shape. Unwrapping each independently and taking a raw min and max
 * afterwards is safe only while at most one shape is drawn: two features at
 * 179..180 and -179..-178 each unwrap to themselves, union to a 359 degree box,
 * and slip under spansTheWorld's 360 test, so the camera frames the planet the
 * wrong way round with both features at the edges.
 *
 * The ellipse is unioned in afterwards rather than unwrapped with the rest,
 * because it is a radius around the primary position and carries no vertices.
 */
export function overlayBounds(
    markers: readonly MapMarker[] | undefined,
    ellipse: MapEllipse | undefined,
    geometries: readonly MapShape[] | undefined,
    lat: number | null,
    lon: number | null,
): [[number, number], [number, number]] | null {
    const points: Array<{lat: number; lon: number}> = [];

    for (const marker of markers ?? []) {
        points.push({lat: marker.lat, lon: marker.lon});
    }

    for (const shape of geometries ?? []) {
        for (const ring of shape.rings) {
            points.push(...ring);
        }
    }

    const usable = points.filter(
        (point) => Number.isFinite(point.lat) && Number.isFinite(point.lon),
    );

    let box: [[number, number], [number, number]] | null = null;

    if (usable.length > 0) {
        const lats = usable.map((point) => point.lat);
        const lons = unwrapLongitudes(usable.map((point) => point.lon));

        box = [
            [Math.min(...lons), Math.min(...lats)],
            [Math.max(...lons), Math.max(...lats)],
        ];
    }

    return unionOf(box, ellipseBounds(ellipse, lat, lon));
}

/** The box an ellipse reaches, which is stated in meters around the pin. */
function ellipseBounds(
    ellipse: MapEllipse | undefined, lat: number | null, lon: number | null,
): [[number, number], [number, number]] | null {
    if (ellipse === undefined || lat === null || lon === null) {
        return null;
    }

    const cosLat = Math.cos((lat * Math.PI) / 180);
    const reach = Math.max(ellipse.major, ellipse.minor);
    if (!Number.isFinite(reach) || reach <= 0 || Math.abs(cosLat) < 1e-9) {
        return null;
    }

    const dLat = reach / DEGREE_METERS;
    const dLon = reach / (DEGREE_METERS * cosLat);
    return [[lon - dLon, lat - dLat], [lon + dLon, lat + dLat]];
}

/**
 * Whether a box is too small to frame, which fitBounds answers badly.
 *
 * A single point, a due-north line and a zero-area polygon all produce a box
 * with no width or no height. fitBounds takes it to maxZoom, which is a
 * street-level view of a document that may be a country wide. The caller falls
 * back to zoomForSpan, which is the same answer the single-position camera has
 * always given.
 */
export function degenerate(box: [[number, number], [number, number]]): boolean {
    const [[west, south], [east, north]] = box;
    return east - west < 1e-9 || north - south < 1e-9;
}

export function centerOf(box: [[number, number], [number, number]]): {lat: number; lon: number} {
    const [[west, south], [east, north]] = box;
    return {lat: (south + north) / 2, lon: (west + east) / 2};
}

/**
 * What the camera frames, or null when there is nothing but the position.
 *
 * A single marker and no shape returns null, which is what keeps the ordinary
 * one-coordinate surfaces on the zoomForSpan camera they have always used
 * rather than on a fitBounds of a single point.
 */
export function frameBounds(
    markers: readonly MapMarker[] | undefined,
    ellipse: MapEllipse | undefined,
    geometries: readonly MapShape[] | undefined,
    lat: number | null,
    lon: number | null,
): [[number, number], [number, number]] | null {
    const nothingToFrame =
        (markers ?? []).length < 2 &&
        (geometries ?? []).length === 0 &&
        ellipse === undefined;

    if (nothingToFrame) {
        return null;
    }

    return withinMercator(overlayBounds(markers, ellipse, geometries, lat, lon));
}

function withinMercator(
    box: [[number, number], [number, number]] | null,
): [[number, number], [number, number]] | null {
    if (box === null) {
        return null;
    }

    const [[west, south], [east, north]] = box;
    if (![west, south, east, north].every((value) => Number.isFinite(value))) {
        return null;
    }

    // Latitude only. fitBounds throws past 90 rather than clamping, so that one
    // has to be brought in. Longitude is deliberately left continuous: a box
    // running 179 to 181 is the two degrees a shape crossing the antimeridian
    // occupies, and clamping it to 180 would collapse it to nothing.
    const inRange: [[number, number], [number, number]] = [
        [west, Math.max(-MERCATOR_LIMIT, south)],
        [east, Math.min(MERCATOR_LIMIT, north)],
    ];

    if (spansTheWorld(west, east)) {
        return [[-180, inRange[0][1]], [180, inRange[1][1]]];
    }

    return inRange;
}

function unionOf(
    a: [[number, number], [number, number]] | null,
    b: [[number, number], [number, number]] | null,
): [[number, number], [number, number]] | null {
    if (a === null) {
        return b;
    }
    if (b === null) {
        return a;
    }

    return [
        [Math.min(a[0][0], b[0][0]), Math.min(a[0][1], b[0][1])],
        [Math.max(a[1][0], b[1][0]), Math.max(a[1][1], b[1][1])],
    ];
}

/**
 * Where a map opens before applyView has run.
 *
 * The primary position when there is one, and otherwise the middle of whatever
 * the overlay covers. A map with neither opens at 0,0, which is the only
 * remaining case and is the same answer the ?? 0 fallbacks used to give
 * everybody.
 */
export function openingAnchor(
    view: View, markers: readonly MapMarker[] | undefined, geometries: readonly MapShape[] | undefined,
): {lat: number; lon: number} {
    if (view.lat !== null && view.lon !== null) {
        return {lat: view.lat, lon: view.lon};
    }

    const box = overlayBounds(markers, undefined, geometries, null, null);
    if (box === null) {
        return {lat: 0, lon: 0};
    }

    return centerOf(box);
}
