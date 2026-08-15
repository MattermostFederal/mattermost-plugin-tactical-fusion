import type {FeatureCollection} from 'geojson';
import type {StyleSpecification} from 'maplibre-gl';

import type {Basemap} from './basemap';

/**
 * Loading MapLibre, and the style it draws the bundled basemap with.
 *
 * The import is dynamic so the library costs nothing until a reader opens a
 * coordinate. That is a bytes-over-the-wire property and nothing more: it does
 * not change the SBOM surface, and any authenticated reader clicking any
 * coordinate loads it.
 */

export type MapLibre = typeof import('maplibre-gl');

let loading: Promise<MapLibre | null> | null = null;
let failed = false;

/**
 * Whether this browser can draw a WebGL map at all.
 *
 * MapLibre 6 requires WebGL2. Hardened VDI fleets, which is what this plugin
 * targets, disable it often enough that its absence is an ordinary degrade
 * rather than an error.
 */
let webgl2: boolean | null = null;

export function hasWebGL2(): boolean {
    // Memoised, and the probe context is released. A browser's WebGL2 support
    // does not change mid-session, and an un-released probe context is a real
    // driver allocation: this is called once per map effect run, so probing
    // afresh each time walks the browser's ~16 context cap and evicts the
    // oldest, which is the map the reader is looking at.
    if (webgl2 === null) {
        webgl2 = probeWebGL2();
    }

    return webgl2;
}

function probeWebGL2(): boolean {
    if (typeof document === 'undefined') {
        return false;
    }

    try {
        const gl = document.createElement('canvas').getContext('webgl2');
        gl?.getExtension('WEBGL_lose_context')?.loseContext();
        return Boolean(gl);
    } catch {
        return false;
    }
}

/** Loads MapLibre, or resolves null. Never throws, never retries. */
export async function loadMapLibre(): Promise<MapLibre | null> {
    if (failed || !hasWebGL2()) {
        return null;
    }
    if (!loading) {
        loading = loadAndConfigure().catch(() => {
            failed = true;
            return null;
        });
    }

    return loading;
}

/**
 * Imports MapLibre and points it at our own worker file.
 *
 * Both imports are dynamic. The worker one especially: it resolves to a URL
 * under webpack, but the unit-test runner is Node, where evaluating the worker
 * module throws on `self`. Nothing here is reached in Node, because the WebGL2
 * probe above returns false without a document.
 *
 * Without setWorkerUrl, MapLibre launders its worker through a blob: URL, which
 * needs `worker-src blob:` in whatever CSP the host serves the webapp under. A
 * same-origin file takes the plain `new Worker(url)` path instead, so a
 * hardened policy cannot break the map.
 */
async function loadAndConfigure(): Promise<MapLibre> {
    // The stylesheet is not optional: the zoom control's + and - glyphs are CSS
    // background images and the control container's corner placement is a CSS
    // rule, so without it the controls are invisible but still focusable.
    const [module, , worker] = await Promise.all([
        import('maplibre-gl'),
        import('maplibre-gl/dist/maplibre-gl.css'),
        import('maplibre-gl/dist/maplibre-gl-worker.mjs'),

        // Imported only so webpack emits it beside the worker, which loads it by
        // a fixed relative name. The ?copy marker is what keeps this an asset
        // copy rather than a rewrite of MapLibre's own import of the same file.
        import('maplibre-gl/dist/maplibre-gl-shared.mjs?copy'),
    ]);

    const url = assetUrl(worker);
    if (url !== null) {
        try {
            module.setWorkerUrl(url);
        } catch {
            // A build without the setter still works wherever blob: workers are
            // allowed.
        }
    }

    return module;
}

/**
 * The URL of an imported asset module, or null.
 *
 * Bundlers disagree about the shape. webpack's asset/resource emits CommonJS
 * (`module.exports = url`), so a dynamic import yields `{default: url}`; a
 * bundler that treats the file as real ESM instead EXECUTES it and yields the
 * worker's own exports, where there is no URL at all.
 *
 * Handing MapLibre a non-string is the worst of the three outcomes: it stores it
 * as WORKER_URL, `endsWith` throws inside the worker setup, the style never
 * finishes loading, and the panel sits on "Loading map…" forever with no error.
 * Returning null instead falls back to MapLibre's own blob: worker, which works
 * wherever the host CSP permits it.
 */
function assetUrl(imported: unknown): string | null {
    if (typeof imported === 'string') {
        return imported;
    }
    if (typeof imported === 'object' && imported !== null) {
        const value = (imported as {default?: unknown}).default;
        if (typeof value === 'string') {
            return value;
        }
    }

    return null;
}

/**
 * Reads a colour off the live DOM.
 *
 * The style is built here rather than shipped as a style.json because a static
 * file cannot follow the reader's theme, and this panel sits inside a Mattermost
 * whose colours are custom properties.
 */
function themeColor(name: string, fallback: string): string {
    if (typeof window === 'undefined' || typeof document === 'undefined') {
        return fallback;
    }

    const value = getComputedStyle(document.documentElement).getPropertyValue(name).trim();

    return value === '' ? fallback : value;
}

export interface MapColors {
    water: string;
    land: string;
    line: string;
    cell: string;
    cellFill: string;
    pin: string;
    pinEdge: string;
}

/**
 * The map's palette.
 *
 * The marker colours are NOT derived from the panel's background and text, on
 * purpose: tints of those are low-contrast by construction, and the pin has to
 * stay legible against both land and water in either theme.
 */
export function mapColors(): MapColors {
    const dark = isDarkTheme();

    // Every pair here is measured, not chosen by eye. The land/water edge IS the
    // content of this map, so it carries WCAG 1.4.11's 3:1 for non-text: the
    // first palette sat at 1.46:1 in dark and 1.28:1 in light, which read as a
    // near-uniform slab once the map filled a window. The pin is distinguished
    // by its outline against land rather than by its fill, which is what lets it
    // stay red.
    return {
        water: dark ? '#12161d' : '#eef2f7',
        land: dark ? '#5a6472' : '#76889f',
        line: dark ? 'rgba(255,255,255,0.45)' : 'rgba(20,24,32,0.55)',
        cell: dark ? '#b3d1ff' : '#0b2f7a',
        cellFill: dark ? 'rgba(179,209,255,0.18)' : 'rgba(11,47,122,0.16)',
        pin: dark ? '#ff6b6b' : '#c92a2a',
        pinEdge: dark ? '#0b0e13' : '#ffffff',
    };
}

function isDarkTheme(): boolean {
    const bg = themeColor('--center-channel-bg', '#ffffff');
    const match = (/^#?([0-9a-f]{6})$/i).exec(bg.replace('#', '#'));
    if (!match) {
        return false;
    }

    const n = parseInt(match[1], 16);
    const luma = (0.2126 * ((n >> 16) & 0xff)) + (0.7152 * ((n >> 8) & 0xff)) + (0.0722 * (n & 0xff));

    return luma < 128;
}

/**
 * The style object.
 *
 * No `glyphs` and no `sprite`, deliberately, and therefore no `symbol` layers.
 * Those are the two URLs a MapLibre style fetches from the network, and an
 * air-gapped install has nowhere for them to point. A future overlay that wants
 * a text label has to solve that first rather than discover it in the field.
 */
export function buildStyle(basemap: Basemap, colors: MapColors): StyleSpecification {
    return {
        version: 8,
        sources: {
            basemap: {type: 'geojson', data: basemap},
            cell: {type: 'geojson', data: emptyCollection()},
            pin: {type: 'geojson', data: emptyCollection()},
        },
        layers: [
            {id: 'water', type: 'background', paint: {'background-color': colors.water}},
            {
                id: 'land',
                type: 'fill',
                source: 'basemap',
                filter: ['==', ['get', 'layer'], 'land'],
                paint: {'fill-color': colors.land},
            },
            {
                id: 'lakes',
                type: 'fill',
                source: 'basemap',
                filter: ['==', ['get', 'layer'], 'lakes'],
                paint: {'fill-color': colors.water},
            },
            {
                id: 'borders',
                type: 'line',
                source: 'basemap',
                filter: ['==', ['get', 'layer'], 'borders'],
                paint: {'line-color': colors.line, 'line-width': 0.6},
            },
            {
                id: 'cell-fill',
                type: 'fill',
                source: 'cell',
                paint: {'fill-color': colors.cellFill},
            },
            {
                id: 'cell-outline',
                type: 'line',
                source: 'cell',
                paint: {'line-color': colors.cell, 'line-width': 1.5},
            },
            {
                id: 'pin',
                type: 'circle',
                source: 'pin',
                paint: {
                    'circle-radius': 4,
                    'circle-color': colors.pin,
                    'circle-stroke-color': colors.pinEdge,
                    'circle-stroke-width': 1.5,
                },
            },
        ],
    };
}

export function emptyCollection(): FeatureCollection {
    return {type: 'FeatureCollection', features: []};
}

export function pointFeature(lat: number, lon: number): FeatureCollection {
    return {
        type: 'FeatureCollection',
        features: [{
            type: 'Feature',
            properties: {},
            geometry: {type: 'Point', coordinates: [lon, lat]},
        }],
    };
}

export function cellFeature(
    bounds: [[number, number], [number, number]],
): FeatureCollection {
    const [[west, south], [east, north]] = bounds;

    return {
        type: 'FeatureCollection',
        features: [{
            type: 'Feature',
            properties: {},
            geometry: {
                type: 'Polygon',
                coordinates: [[
                    [west, south], [east, south], [east, north], [west, north], [west, south],
                ]],
            },
        }],
    };
}

/** @internal exported for tests */
export function _assetUrlForTesting(imported: unknown): string | null { // eslint-disable-line no-underscore-dangle, @typescript-eslint/naming-convention
    return assetUrl(imported);
}

/** Test hook, for the same reason basemap.ts has one. */
export function _resetForTesting(): void { // eslint-disable-line no-underscore-dangle, @typescript-eslint/naming-convention
    loading = null;
    failed = false;
    webgl2 = null;
}
