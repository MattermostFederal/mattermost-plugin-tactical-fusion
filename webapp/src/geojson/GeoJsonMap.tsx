import React, {useState} from 'react';

import {isSectionVisible} from './sections';
import type {GeoJsonFeature, GeoJsonPart, GeoJsonPayload, GeoJsonPosition} from './types';

import LocationMap, {MAP_HEIGHT} from '../decorators/location/map/LocationMap';
import {useNearViewport} from '../decorators/location/map/near_viewport';
import type {MapMarker} from '../decorators/location/map/overlay';
import {isMarkerSize} from '../decorators/location/map/overlay';
import type {MapShape} from '../decorators/location/map/paint';
import {styleOf} from '../decorators/location/map/paint';
import {isRenderable} from '../decorators/location/map/span';
import {overlayPageHref} from '../decorators/location/map/view';
import {INLINE_ID, isRowVisible} from '../decorators/location/rows';
import {useFeatures} from '../features/store';
import {usePreferences} from '../preferences/store';

export const GEOJSON_MAP_MAX_WIDTH_PX = 640;

const styles: Record<string, React.CSSProperties> = {
    frame: {maxWidth: GEOJSON_MAP_MAX_WIDTH_PX, padding: '0 12px 8px'},
    reserved: {height: MAP_HEIGHT},
};

/**
 * What every point in the document is drawn in.
 *
 * The fallback only. A feature that states a simplestyle color of its own is
 * drawn in that instead, validated by the server and again by `styleOf` before
 * it reaches a paint property.
 */
const MARKER_COLOR = '#3f7fbf';

/**
 * The color a feature asked for, or the default for its channel.
 *
 * Through the same gate the shapes use. A marker color is not a paint property
 * today (it names a drawn image, and `channels()` parses it numerically), so
 * nothing was exploitable, but the comments here and in types.ts both claimed
 * this path was validated twice and it was not. Running it through `styleOf`
 * makes the claim true rather than correcting it downwards.
 */
function colorFor(feature: GeoJsonFeature, fallback: string): string {
    return styleOf(statedStyle(feature)).color ?? fallback;
}

/**
 * The simplestyle a feature stated, in the shape `styleOf` gates.
 *
 * Empty strings become undefined here rather than inside `styleOf`, so the gate
 * sees one spelling of "stated nothing". The server writes no key at all for a
 * value it refused, and the reader turns an absent key into '', so both arrive.
 */
function statedStyle(feature: GeoJsonFeature): {
    color?: string;
    width?: string;
    lineOpacity?: string;
    fillOpacity?: string;
} {
    const stated = (value: string) => (value === '' ? undefined : value);

    return {
        color: stated(feature.color),
        width: stated(feature.width),
        lineOpacity: stated(feature.lineOpacity),
        fillOpacity: stated(feature.fillOpacity),
    };
}

/** A position the map will draw, or null. */
function usable(position: {lat: string; lon: string}): {lat: number; lon: number} | null {
    const lat = Number(position.lat);
    const lon = Number(position.lon);

    // Number('') is 0, so an empty lexeme would pass a finiteness test and pin
    // the Gulf of Guinea. The emptiness test is the one that matters.
    if (position.lat === '' || position.lon === '') {
        return null;
    }
    if (!Number.isFinite(lat) || !Number.isFinite(lon)) {
        return null;
    }

    // The server accepts any latitude to 90 and Web Mercator stops at about
    // 85.05. A position past that cannot be placed at all.
    if (!isRenderable(lat)) {
        return null;
    }

    return {lat, lon};
}

/** Whether a part is drawn as a marker rather than as a shape. */
function isPointPart(part: GeoJsonPart): boolean {
    return part.kind === 'Point' || part.kind === 'MultiPoint';
}

/**
 * Every point the document reports, as markers.
 *
 * Points go through `markers` and not through `geometries` because the geometry
 * source is drawn by a fill layer and a line layer, neither of which renders a
 * point. A GeoJSON Point is a position somebody reported, which is what a marker
 * means; a polygon's corners still are not.
 *
 * A GeometryCollection contributes each of its parts to whichever channel that
 * part's own kind selects, so one feature can appear in both.
 */
export function markersFor(features: readonly GeoJsonFeature[]): MapMarker[] {
    const markers: MapMarker[] = [];

    for (const feature of features) {
        // The server's verdict wins. It notes a feature it will not stand
        // behind, and drawing one anyway would make the card say "not drawn"
        // over something drawn.
        if (feature.note !== '') {
            continue;
        }

        for (const part of feature.parts) {
            if (!isPointPart(part)) {
                continue;
            }
            for (const ring of part.rings) {
                for (const position of ring) {
                    const point = usable(position);
                    if (point !== null) {
                        markers.push({
                            ...point,
                            color: colorFor(feature, MARKER_COLOR),
                            ...(isMarkerSize(feature.markerSize) ? {size: feature.markerSize} : {}),
                        });
                    }
                }
            }
        }
    }

    return markers;
}

/**
 * Every line and area the document describes, as ringed shapes.
 *
 * Closed is decided by the part's kind rather than by comparing its first and
 * last position: the server already refused a polygon whose rings do not close,
 * so a ring that reaches here and happens to be open is a line, not a polygon
 * this build should silently close.
 */
/**
 * One shape per member polygon, using the boundaries the server carried.
 *
 * A MultiPolygon arrives as ONE part whose rings are every ring of every member
 * in order, with `ringCounts` naming where each member ends. Drawing that as a
 * single shape makes ring 0 the exterior and every later ring a hole, so a
 * two-island MultiPolygon was drawn as island one with island two punched out
 * of it. `ringCounts` exists precisely to prevent that, and this is its only
 * consumer.
 *
 * Falling back to one member when it is absent matches `polygonArea`, so the
 * map and the measurement split a part the same way.
 */
function membersOf(part: GeoJsonPart): GeoJsonPosition[][][] {
    const counts = part.ringCounts.length > 0 ? part.ringCounts : [part.rings.length];

    const members: GeoJsonPosition[][][] = [];
    let at = 0;

    for (const count of counts) {
        const member = part.rings.slice(at, at + count);
        at += count;
        if (member.length > 0) {
            members.push(member);
        }
    }

    return members;
}

export function shapesFor(features: readonly GeoJsonFeature[]): MapShape[] {
    const shapes: MapShape[] = [];

    for (const feature of features) {
        if (feature.note !== '') {
            continue;
        }

        for (const part of feature.parts) {
            if (isPointPart(part)) {
                continue;
            }

            const closed = part.kind === 'Polygon' || part.kind === 'MultiPolygon';

            for (const member of membersOf(part)) {
                const rings = member.
                    map((ring) => ring.map(usable).filter((point) => point !== null)).
                    filter((ring) => ring.length >= 2);

                // Ring 0 is the exterior. If IT is the ring that lost its
                // positions, a hole would be promoted to the outer boundary and
                // drawn as the shape, so the whole member goes rather than the
                // wrong ring becoming the outline.
                if (rings.length === 0 || rings.length !== member.length) {
                    continue;
                }

                // Validated HERE as well as at the paint, so a MapShape never
                // carries a style the map would refuse. `styleOf` is idempotent
                // and this keeps the invariant true at the type boundary rather
                // than only at the last hop. The lexemes are carried forward,
                // not the parsed numbers: `styleOf` is where text stops being
                // text, and it runs again at the paint.
                const stated = statedStyle(feature);
                const drawn = styleOf(stated);

                shapes.push({
                    rings,
                    closed,
                    ...(drawn.color === undefined ? {} : {color: drawn.color}),
                    ...(drawn.width === undefined ? {} : {width: stated.width}),
                    ...(drawn.lineOpacity === undefined ? {} : {lineOpacity: stated.lineOpacity}),
                    ...(drawn.fill === undefined ? {} : {fillOpacity: stated.fillOpacity}),
                });
            }
        }
    }

    return shapes;
}

/** What the map is, for a reader who gets no picture. */
export function mapLabel(markers: number, shapes: number): string {
    const parts: string[] = [];
    if (markers > 0) {
        parts.push(`${markers} marked position${markers === 1 ? '' : 's'}`);
    }
    if (shapes > 0) {
        parts.push(`${shapes} drawn shape${shapes === 1 ? '' : 's'}`);
    }

    if (parts.length === 0) {
        return '';
    }

    return parts.join(' and ');
}

export function drawsNothing(payload: GeoJsonPayload): boolean {
    if (payload.unplaceable) {
        return true;
    }

    return markersFor(payload.features).length === 0 && shapesFor(payload.features).length === 0;
}

/**
 * The drawing, with nothing consulted. Exported for the reason CotMapCanvas is.
 */
export const GeoJsonMapCanvas: React.FC<{
    payload: GeoJsonPayload;
    pageEnabled?: boolean;
    fill?: boolean;
}> = ({payload, pageEnabled, fill}) => {
    const markers = markersFor(payload.features);
    const shapes = shapesFor(payload.features);

    if (drawsNothing(payload)) {
        return null;
    }

    // Extent-only, and no primary position anywhere. A document of nothing but
    // polygons reports no position at all, and passing a computed centroid as
    // one would pin a place nobody stated.
    return (
        <LocationMap
            lat={null}
            lon={null}
            cellDegLat={0}
            cellDegLon={0}
            region=''
            pending={false}
            extentLabel={mapLabel(markers.length, shapes.length)}
            markers={markers}
            geometries={shapes}
            pageHref={pageEnabled && payload.postId ? overlayPageHref(payload.postId) : undefined}
            fill={fill}
        />
    );
};

/**
 * The map of a GeoJSON document.
 *
 * The ADMIN switch is the location decorator's, not a second one: this is the
 * same map over the same basemap, and drawing it would pull the same archive on
 * exactly the installs that switch exists for.
 *
 * Which of the location decorator's switches differs BY SURFACE here, and that
 * is where this parts from CotMap. The card reads `mapInline`, the map under a
 * post; the panel reads `mapPanel`, because in the sidebar it is not a map
 * under a post. CoT reads `mapInline` on both, argued in cot.md "Switches".
 * The two comments used to say the same thing while doing different things.
 *
 * The READER switch differs by surface. In the channel this is the map under a
 * post and follows the same INLINE_ID a coordinate-only post does. In the
 * sidebar it is not a map under a post at all, so it follows this panel's own
 * `map` section; hiding the coordinate map under posts used to blank the
 * equivalent Cursor on Target map too, which read as a bug rather than a
 * setting.
 *
 * Both are read in the OUTER component so the inner one never mounts, and the
 * viewport gate is here for the reason it is on the other maps: browsers cap
 * live WebGL contexts at roughly sixteen and a channel of overlays is exactly
 * the shape that reaches it.
 */
const GeoJsonMap: React.FC<{payload: GeoJsonPayload; surface: 'card' | 'panel'}> = ({payload, surface}) => {
    const {preferences} = usePreferences();
    const {features} = useFeatures();
    const [box, setBox] = useState<HTMLDivElement | null>(null);
    const near = useNearViewport(box);

    // The server's own verdict, read as a flag rather than inferred from the
    // note. Any note used to blank the map, which contradicted the malformed
    // bbox note, whose sentence says the features are still drawn from their
    // own coordinates.
    if (payload.unplaceable) {
        return null;
    }

    const admin = surface === 'card' ? features.mapInline : features.mapPanel;
    const wanted = surface === 'card' ? isRowVisible(preferences.location.hiddenRows, INLINE_ID) : isSectionVisible(preferences.geojson.hiddenSections, 'map');

    if (!admin || !wanted) {
        return null;
    }

    return (
        <div
            ref={setBox}
            style={styles.frame}
            data-testid='geojson-map'
        >
            {near ? (
                <GeoJsonMapCanvas
                    payload={payload}
                    pageEnabled={features.mapPage}
                />
            ) : <div style={styles.reserved}/>}
        </div>
    );
};

export default GeoJsonMap;
