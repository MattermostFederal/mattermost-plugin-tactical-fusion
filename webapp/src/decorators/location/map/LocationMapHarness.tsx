import type {GeoJSONSource, Map as MapLibreMap} from 'maplibre-gl';
import React, {useEffect, useRef, useState} from 'react';

import LocationMap, {_setMapObserverForTesting} from './LocationMap';
import type {View} from './LocationMap';

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

const VIEWS = {
    'Los Angeles': LOS_ANGELES,
    Washington: WASHINGTON,
    unknown: UNKNOWN,
    'too far north': TOO_FAR_NORTH,
    'too far south': TOO_FAR_SOUTH,
    'no cell': NO_CELL,
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
}

const LocationMapHarness: React.FC<Props> = ({
    start = 'Los Angeles', region = '', pending = false, pageHref, fill, noWebGL,
}) => {
    // Assigned during render, never installed during render: the patch above is
    // already in place, so this only has to be decided before the child probes
    // from its own effect, and child effects run before the parent's.
    webgl2Allowed = !noWebGL;

    const [name, setName] = useState<ViewName>(start);
    const [mounted, setMounted] = useState(true);
    const [live, setLive] = useState(false);
    const [created, setCreated] = useState(0);
    const [reading, setReading] = useState({pin: -1, cell: -1, center: 'none', removed: false});

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
            center: at ? `${at.lat.toFixed(3)},${at.lng.toFixed(3)}` : 'none',
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
            <output data-testid='selected'>{name}</output>
            <output data-testid='live-map'>{live ? 'yes' : 'no'}</output>
            <output data-testid='maps-created'>{String(created)}</output>
            <output data-testid='pin-features'>{String(reading.pin)}</output>
            <output data-testid='cell-features'>{String(reading.cell)}</output>
            <output data-testid='camera'>{reading.center}</output>
            <output data-testid='removed'>{reading.removed ? 'yes' : 'no'}</output>
        </div>
    );
};

export default LocationMapHarness;
