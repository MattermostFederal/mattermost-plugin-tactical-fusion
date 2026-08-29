import type {FeatureCollection} from 'geojson';
import type {Map as MapLibreMap} from 'maplibre-gl';

import {
    MARKER_PIXEL_RATIO,
    accuracyFeature, cellFeature, crosshairImage, ellipseFeature, emptyCollection,
    mapColors, markedPoints, markerImageID, outlineFeature, pointFeature, shapesFeature,
} from './maplibre';
import type {MapShape} from './paint';
import {styleOf} from './paint';
import {cellBounds} from './span';
import type {View} from './view';

/**
 * One marked point.
 *
 * Named rather than left as `readonly MapMarker[] | undefined`, because every helper below took
 * that shape and a helper in its own module cannot reach into the component's
 * props for a type without the two importing each other.
 */
export interface MapMarker {
    lat: number;
    lon: number;
    color: string;
    size?: string;
}

/** What a surface may ask this map to draw beyond its pin. */
export type MapGeometry =
    | {kind: 'outline'; points: ReadonlyArray<{lat: number; lon: number}>; closed: boolean}
    | {kind: 'ellipse'; major: number; minor: number; angle: number};

/**
 * The cell, or nothing, where nothing means only an unknown position or a token
 * that carries no resolution at all.
 */
export function drawableCell(current: View): FeatureCollection {
    const {lat, lon, cellDegLat, cellDegLon} = current;
    if (lat === null || lon === null || !(cellDegLat > 0) || !(cellDegLon > 0)) {
        return emptyCollection();
    }

    // No minimum size, and no threshold below which the cell is dropped. These
    // surfaces zoom, so there is no one scale to test against: a meter-wide cell
    // is invisible until the reader zooms far enough to see it, which is more
    // honest than a number guessing on their behalf.
    //
    // There was a 6px floor here, measured once at the OPENING camera and never
    // recomputed, because nothing listens for zoom. It dropped every square
    // finer than about 45km permanently, including a 10km grid reference that
    // would have been 11px across at maximum zoom.
    return cellFeature(cellBounds(lat, lon, cellDegLat, cellDegLon));
}

/**
 * What to draw at the marked points, or the single pin when nothing asked for
 * markers, which is every location surface.
 *
 * Only ever called on the positioned path. An extent-only map draws its markers
 * directly and never falls back to a pin, because the position it would fall
 * back to is one no caller stated.
 */
export function drawableMarkers(
    markers: readonly MapMarker[] | undefined, lat: number, lon: number,
): FeatureCollection {
    if (!hasMarkers(markers)) {
        return pointFeature(lat, lon);
    }

    return markerFeatures(markers);
}

/**
 * simplestyle's three marker sizes as a scale on the one reticle.
 *
 * A closed set matched by name, so a size this build does not know is drawn at
 * the theme's rather than turned into a number. Scaling one image beats an
 * image per size: the reticles are generated per color already, and three sizes
 * would treble that for a difference a multiplier expresses exactly.
 */
const MARKER_SCALES: Record<string, number> = {small: 0.7, medium: 1, large: 1.5};

function markerScale(size: string | undefined): number | undefined {
    // Object.hasOwn, not a bare index. A bare index walks the prototype chain,
    // so a props blob naming "constructor" or "__proto__" yields a function or
    // Object.prototype where TypeScript promises a number, and the comment
    // above calling this a closed set was false. cot/types.ts guards its two
    // tables the same way; this was the lookup that skipped it.
    if (size === undefined || !Object.hasOwn(MARKER_SCALES, size)) {
        return undefined;
    }

    return MARKER_SCALES[size];
}

/**
 * The markers, as features. The ONE builder, used by both write paths.
 *
 * `applyView` had two: the positioned branch called `drawableMarkers` and the
 * extent-only branch hand-rolled its own `markedPoints` call that omitted
 * `scale`. GeoJSON is always extent-only, so a stated `marker-size` was carried
 * the whole way and then dropped at the last hop, on the only surface that
 * draws it.
 */
export function markerFeatures(markers: readonly MapMarker[] | undefined): FeatureCollection {
    return markedPoints((markers ?? []).map((marker) => ({
        lat: marker.lat,
        lon: marker.lon,
        icon: markerImageID(marker.color),
        scale: markerScale(marker.size),
    })));
}

/**
 * Everything the overlays draw, as one string.
 *
 * The effect that redraws them compares this instead of object identity, so a
 * caller may build its geometry inline without re-framing the camera, and a
 * marker set that changed over an unchanged position still redraws.
 */
export function overlayDigest(
    markers: readonly MapMarker[] | undefined, geometry: MapGeometry | undefined, geometryColor: string | undefined,
    geometries?: readonly MapShape[] | undefined,
): string {
    const pins = (markers ?? []).
        map((marker) => `${marker.lat},${marker.lon},${marker.color},${marker.size ?? ''}`).join('|');

    // The STYLE is part of it, not only the geometry. A document redrawn with
    // the same shapes in different colors is a different picture, and the shape
    // half alone said it was the same one.
    const plural = (geometries ?? []).
        map((shape) => `${shape.closed}:${shape.color ?? ''}:${shape.width ?? ''}:` +
            `${shape.lineOpacity ?? ''}:${shape.fillOpacity ?? ''}:${shape.rings.
                map((ring) => ring.map((point) => `${point.lat},${point.lon}`).join(' ')).
                join(';')}`).
        join('|');

    if (geometry === undefined) {
        return `${pins}#${plural}#`;
    }

    const shape = geometry.kind === 'ellipse' ? `e:${geometry.major},${geometry.minor},${geometry.angle}` : `o:${geometry.closed}:${geometry.points.map((point) => `${point.lat},${point.lon}`).join(' ')}`;

    return `${pins}#${plural}#${shape}#${geometryColor ?? ''}`;
}

/**
 * Both halves of the overlay, as one collection on one source.
 *
 * One source and one layer pair rather than N, so a document of 256 features
 * costs the same setData a single shape does.
 */
export function drawableOverlay(
    geometry: MapGeometry | undefined,
    geometries: readonly MapShape[] | undefined,
    lat: number,
    lon: number,
    color?: string,
): FeatureCollection {
    if ((geometries ?? []).length === 0) {
        return drawableGeometry(geometry, lat, lon, color);
    }

    const plural = shapesFeature((geometries ?? []).map((shape) => ({
        rings: shape.rings,
        closed: shape.closed,
        style: styleOf(shape),
    })));
    const singular = drawableGeometry(geometry, lat, lon, color);

    return {
        type: 'FeatureCollection',
        features: [...plural.features, ...singular.features],
    };
}

function drawableGeometry(
    geometry: MapGeometry | undefined, lat: number, lon: number, color?: string,
): FeatureCollection {
    if (geometry === undefined) {
        return emptyCollection();
    }

    if (geometry.kind === 'ellipse') {
        return ellipseFeature(lat, lon, geometry.major, geometry.minor, geometry.angle, styleOf({color}));
    }

    return outlineFeature(geometry.points, geometry.closed, styleOf({color}));
}

export function hasMarkers(markers: readonly MapMarker[] | undefined): boolean {
    return markers !== undefined && markers.length > 0;
}

/**
 * Registers one reticle per color the markers ask for.
 *
 * Per color rather than per marker, so a block of twenty friendly tracks costs
 * one image. Nothing to do for a map with no markers, which is every location
 * surface: those draw the circle layer and never reference an image.
 */
export function addMarkerImages(instance: MapLibreMap, markers: readonly MapMarker[] | undefined): void {
    if (!hasMarkers(markers)) {
        return;
    }

    const edge = mapColors().pinEdge;
    for (const color of new Set((markers ?? []).map((marker) => marker.color))) {
        const id = markerImageID(color);
        const image = crosshairImage(color, edge);

        if (instance.hasImage(id)) {
            instance.updateImage(id, image);
        } else {
            instance.addImage(id, image, {pixelRatio: MARKER_PIXEL_RATIO});
        }
    }
}

export function drawableAccuracy(lat: number, lon: number, meters: number | undefined): FeatureCollection {
    if (meters === undefined || !(meters > 0)) {
        return emptyCollection();
    }

    return accuracyFeature(lat, lon, meters);
}
