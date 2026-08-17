import type {GeoJSONSource, Map as MapLibreMap} from 'maplibre-gl';
import React, {useEffect, useRef, useState} from 'react';

import LocationMap, {_setMapObserverForTesting} from './LocationMap';
import type {View} from './LocationMap';
import {DATA_MAX_ZOOM} from './span';

/**
 * Harness for the map.
 *
 * The panel keeps one map and moves it, so the harness changes the position
 * through state rather than remounting: a test that remounted would be testing
 * something the sidebar never does, and would hide the whole class of defect
 * that comes of a map outliving the coordinate it was built for.
 *
 * Most of what this component exists for is making MapLibre's own state
 * observable. The pin and the cell are GeoJSON sources drawn through WebGL and
 * the camera is a number nothing renders, so a test that watches the DOM sees
 * exactly the same thing whether or not `applyView` did any of its work. Every
 * `data-testid` below is one of those otherwise invisible facts, and each one
 * exists because a mutation of the product code survived without it.
 */

const LOS_ANGELES: View = {lat: 34.0561, lon: -118.25, cellDegLat: 0.0001, cellDegLon: 0.0001};
const WASHINGTON: View = {lat: 38.8895, lon: -77.0353, cellDegLat: 0.001, cellDegLon: 0.001};
const UNKNOWN: View = {lat: null, lon: null, cellDegLat: 0, cellDegLon: 0};
const TOO_FAR_NORTH: View = {lat: 89.9, lon: 12, cellDegLat: 0.0001, cellDegLon: 0.0001};
const TOO_FAR_SOUTH: View = {lat: -89.9, lon: 12, cellDegLat: 0.0001, cellDegLon: 0.0001};
const NO_CELL: View = {lat: 34.0561, lon: -118.25, cellDegLat: 0, cellDegLon: 0};

/*
 * Positions with a known answer to "is there land here".
 *
 * These carry the coordinates the Go suite used to check against the GeoJSON
 * basemap's polygons directly. The archive is vector tiles, so the equivalent
 * question can only be asked of a renderer, and it is a better question here
 * anyway: this asks what the reader is actually shown rather than what the
 * source data contains.
 */
const KANSAS: View = {lat: 38.5, lon: -98.0, cellDegLat: 0.01, cellDegLon: 0.01};
const CENTRAL_AUSTRALIA: View = {lat: -25.0, lon: 133.0, cellDegLat: 0.01, cellDegLon: 0.01};
const MID_PACIFIC: View = {lat: 0, lon: -140.0, cellDegLat: 0.01, cellDegLon: 0.01};
const MID_ATLANTIC: View = {lat: 30.0, lon: -40.0, cellDegLat: 0.01, cellDegLon: 0.01};

/*
 * A coordinate sitting on a named place.
 *
 * The opening camera always frames a fixed ground span, so whether a label is on
 * screen is decided by how near the coordinate is to one. This used to be
 * Natural Earth's label anchor for the United States, in Kansas, which worked
 * while the opening span was 2,400 km and stopped working the moment it became
 * 400 km: the anchor is farmland, and the nearest named towns are about 200 km
 * away, right at the frame edge.
 *
 * Paris is a rank-1 place, so it survives the zoom gating at every zoom the
 * layer is drawn at, and it is nowhere near a coastline, which keeps it out of
 * the way of the land-and-water fixtures.
 */
const ON_A_LABEL: View = {lat: 48.8566, lon: 2.3522, cellDegLat: 0.01, cellDegLon: 0.01};

const VIEWS = {
    'Los Angeles': LOS_ANGELES,
    'on a label': ON_A_LABEL,
    Washington: WASHINGTON,
    unknown: UNKNOWN,
    'too far north': TOO_FAR_NORTH,
    'too far south': TOO_FAR_SOUTH,
    'no cell': NO_CELL,
    Kansas: KANSAS,
    'central Australia': CENTRAL_AUSTRALIA,
    'mid Pacific': MID_PACIFIC,
    'mid Atlantic': MID_ATLANTIC,
} satisfies Record<string, View>;

export type ViewName = keyof typeof VIEWS;

/*
 * WebGL2.
 *
 * Installed once at module scope and gated by a flag, rather than installed on
 * demand and left in place. That is the shape `CopyButtonHarness` uses for
 * `navigator.clipboard`, and it matters twice: a render React discards cannot
 * leave a global patched, and the flag can go back, where a one-way latch could
 * not.
 */
const realGetContext = HTMLCanvasElement.prototype.getContext;

let webgl2Allowed = true;

HTMLCanvasElement.prototype.getContext = function patched(
    this: HTMLCanvasElement, id: string, ...rest: unknown[]
) {
    if (id === 'webgl2' && !webgl2Allowed) {
        return null;
    }
    return (realGetContext as (...args: unknown[]) => unknown).call(this, id, ...rest);
} as typeof HTMLCanvasElement.prototype.getContext;

/**
 * How many labels the map has actually drawn, or -1 when there is no map to ask.
 *
 * Rendered rather than sourced, deliberately: a label present in the tile but
 * dropped by collision, or one whose font never resolved, is not a label the
 * reader can see, and it is the reader's view this pins.
 *
 * Both label layers count, because which of them is drawing depends on the
 * zoom: countries name the view while it spans one, and hand over to towns
 * past z6. Naming only one here would make the test a test of the handover
 * rather than of whether any label reaches the reader.
 */
function labelsIn(map: MapLibreMap | null): number {
    if (!map) {
        return -1;
    }

    try {
        return map.queryRenderedFeatures({layers: ['country-label', 'place-label']}).length;
    } catch {
        // Asked before the style finished, which is not a failure to report.
        return -1;
    }
}

/**
 * Whether the map draws land under its own centre, or -1 when there is no map.
 *
 * Queried at the projected centre pixel rather than by testing geometry, because
 * what matters is the fill a reader sees at the coordinate: a basemap shifted or
 * clipped in tiling would still contain the right polygons somewhere.
 */
function landAtCentre(map: MapLibreMap | null): number {
    if (!map) {
        return -1;
    }

    try {
        const at = map.project(map.getCenter());

        return map.queryRenderedFeatures(at, {layers: ['land']}).length;
    } catch {
        return -1;
    }
}

/** How many features a source is carrying, or -1 when there is no map to ask. */
function countIn(map: MapLibreMap | null, id: string): number {
    const source = map?.getSource<GeoJSONSource>(id);
    if (!source) {
        return -1;
    }

    const data = (source.serialize() as {data?: {features?: unknown[]}}).data;

    return data?.features?.length ?? -1;
}

/*
 * MapLibre has no public "has this been removed" question, and `remove()` is
 * the only thing that gives the WebGL context back. Reading the private flag is
 * the honest way to assert it: the alternative is asserting on the canvas being
 * gone, which React does on its own and which therefore proves nothing.
 */
function wasRemoved(map: MapLibreMap | null): boolean {
    // eslint-disable-next-line no-underscore-dangle
    return Boolean((map as {_removed?: boolean} | null)?._removed);
}

interface Props {

    /** Which of the named views to open on. */
    start?: ViewName;

    region?: string;

    /** Whether a conversion is still in the air, which is loading rather than missing. */
    pending?: boolean;

    pageHref?: string;
    fill?: boolean;

    /** Makes this browser look like one with WebGL2 switched off. */
    noWebGL?: boolean;

    /** Renders the card-sized map: no controls, no gestures, no readout. */
    preview?: boolean;
}

const LocationMapHarness: React.FC<Props> = ({
    start = 'Los Angeles', region = '', pending = false, pageHref, fill, noWebGL, preview,
}) => {
    // Assigned during render, never installed during render: the patch above is
    // already in place, so this only has to be decided before the child probes
    // from its own effect, and child effects run before the parent's.
    webgl2Allowed = !noWebGL;

    const [name, setName] = useState<ViewName>(start);
    const [mounted, setMounted] = useState(true);
    const [live, setLive] = useState(false);
    const [created, setCreated] = useState(0);
    const [reading, setReading] = useState({
        pin: -1,
cell: -1,
labels: -1,
land: -1,
zoom: -1,
        tiles: 'pending',
center: 'none',
removed: false,
    });

    // The last instance, kept past the observer's null so the unmount test can
    // still ask it whether it was removed.
    const last = useRef<MapLibreMap | null>(null);

    useEffect(() => {
        _setMapObserverForTesting((instance) => {
            if (instance) {
                last.current = instance;
                setCreated((n) => n + 1);
            }
            setLive(Boolean(instance));
        });

        return () => _setMapObserverForTesting(null);
    }, []);

    const read = () => {
        const map = last.current;
        const at = map && !wasRemoved(map) ? map.getCenter() : null;

        setReading({
            pin: countIn(map, 'pin'),
            cell: countIn(map, 'cell'),
            labels: labelsIn(map),
            land: landAtCentre(map),
            tiles: map && !wasRemoved(map) && map.areTilesLoaded() ? 'loaded' : 'pending',
            center: at ? `${at.lat.toFixed(3)},${at.lng.toFixed(3)}` : 'none',
            zoom: map && !wasRemoved(map) ? Number(map.getZoom().toFixed(2)) : -1,
            removed: wasRemoved(map),
        });
    };

    const view = VIEWS[name];

    return (
        <div>
            {mounted && (
                <LocationMap
                    lat={view.lat}
                    lon={view.lon}
                    cellDegLat={view.cellDegLat}
                    cellDegLon={view.cellDegLon}
                    region={region}
                    pending={pending}
                    pageHref={pageHref}
                    fill={fill}
                    preview={preview}
                />
            )}
            {Object.keys(VIEWS).map((key) => (
                <button
                    key={key}
                    type='button'
                    onClick={() => setName(key as ViewName)}
                >{`select ${key}`}</button>
            ))}
            <button
                type='button'
                onClick={() => setMounted(false)}
            >{'unmount the map'}</button>
            <button
                type='button'
                onClick={read}
            >{'read the map'}</button>
            <button
                type='button'
                onClick={() => last.current?.fire('error', {error: new Error('style failed')})}
            >{'make the map fail'}</button>
            <button
                type='button'
                onClick={() => last.current?.setZoom(DATA_MAX_ZOOM + 3)}
            >{'zoom past the data'}</button>
            <button
                type='button'
                onClick={() => last.current?.setZoom(DATA_MAX_ZOOM - 1)}
            >{'zoom back within the data'}</button>
            <output data-testid='selected'>{name}</output>
            <output data-testid='live-map'>{live ? 'yes' : 'no'}</output>
            <output data-testid='maps-created'>{String(created)}</output>
            <output data-testid='pin-features'>{String(reading.pin)}</output>
            <output data-testid='cell-features'>{String(reading.cell)}</output>
            <output data-testid='labels-drawn'>{String(reading.labels)}</output>
            <output data-testid='land-at-centre'>{String(reading.land)}</output>
            <output data-testid='tiles'>{reading.tiles}</output>
            <output data-testid='camera'>{reading.center}</output>
            <output data-testid='zoom'>{String(reading.zoom)}</output>
            <output data-testid='removed'>{reading.removed ? 'yes' : 'no'}</output>
        </div>
    );
};

export default LocationMapHarness;
