import type {FeatureCollection} from 'geojson';
import type {StyleSpecification} from 'maplibre-gl';

import type {Archive} from './basemap';

import {pluginBaseUrl} from '../../../plugin_url';

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
let protocolRegistered = false;

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
    const [module, , worker, , pmtiles] = await Promise.all([
        import('maplibre-gl'),
        import('maplibre-gl/dist/maplibre-gl.css'),
        import('maplibre-gl/dist/maplibre-gl-worker.mjs'),

        // Imported only so webpack emits it beside the worker, which loads it by
        // a fixed relative name. The ?copy marker is what keeps this an asset
        // copy rather than a rewrite of MapLibre's own import of the same file.
        import('maplibre-gl/dist/maplibre-gl-shared.mjs?copy'),
        import('pmtiles'),
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

    // Registered here rather than at module scope, which would need maplibregl
    // at module scope and so un-lazy the chunk this whole function exists to
    // keep lazy. Behind the `loading` memo, so it happens exactly once, and
    // necessarily before any Map is constructed.
    //
    // The worker's own protocol table is empty, so a tile request made there
    // falls through to an actor message answered on this thread, where the
    // protocol is registered. That is why no worker-side code is needed.
    if (!protocolRegistered) {
        module.addProtocol('pmtiles', new pmtiles.Protocol().tile);
        protocolRegistered = true;
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
    label: string;
    labelHalo: string;
    urban: string;
    road: string;
    rail: string;
    adminLine: string;
    airport: string;
}

/**
 * Whether the map is drawn dark whatever theme the reader is in.
 *
 * It is, because the dark palette reads better as a map: the land/water edge
 * carries more of the frame and the pin and cell sit on a ground that is not
 * competing with the panel around them. The map therefore no longer follows
 * `--center-channel-bg`, and a light Mattermost gets a dark map inside a light
 * panel, which is the cost and was accepted deliberately.
 *
 * The light palette is deliberately KEPT rather than deleted, and stays
 * reachable through `palette(false)`: it is still held to its measured contrast
 * by TestMapPaletteCarriesItsContrast, which reads those hex values out of this
 * file, so it cannot rot while it waits. Flipping this one constant restores
 * theme-following everywhere, including on the pages, where `_theme` still
 * decides everything around the map.
 *
 * Annotated `boolean` rather than left to infer `true`, so the other branch
 * stays live code to the type checker instead of becoming unreachable.
 */
const ALWAYS_DARK: boolean = true;

/**
 * The map's palette.
 *
 * The marker colours are NOT derived from the panel's background and text, on
 * purpose: tints of those are low-contrast by construction, and the pin has to
 * stay legible against both land and water in either theme.
 */
export function mapColors(): MapColors {
    return palette(ALWAYS_DARK || isDarkTheme());
}

/** @internal exported so the unused half of the palette stays exercised. */
export function palette(dark: boolean): MapColors {
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
        label: dark ? '#e8edf5' : '#101418',
        labelHalo: dark ? '#12161d' : '#eef2f7',

        // Roads carry meaning once a reader is close, so they are held to the
        // same measured 3:1 the land/water edge is: 3.43:1 light, 3.69:1 dark.
        road: dark ? '#c2ccd9' : '#2b3542',

        // An airfield is a landmark rather than context, so it is held to the
        // same floor as roads: 3.73:1 light, 3.71:1 dark. It is the one hue on
        // the map that is neither the greys of the basemap nor the red of the
        // pin, which is what stops an aerodrome reading as a town.
        airport: dark ? '#edc67e' : '#382d12',

        // The rest are context and are deliberately BELOW that floor, between
        // 1.2:1 and 2.3:1 against land. They are there to be recognised when
        // looked for, not to be read, and the coordinate has to stay the
        // loudest thing on screen when a city's worth of them is drawn under
        // it. That is why they are absent from the contrast table in
        // shell_test.go rather than failing it.
        urban: dark ? '#68717f' : '#6a7c93',
        rail: dark ? '#98a3b1' : '#55637a',
        adminLine: dark ? '#7b8593' : '#8fa0b5',
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
 * `glyphs` and `sprite` are the only two style fields MapLibre resolves as
 * URLs. There is still no `sprite`, and `glyphs` points at bundled font ranges
 * under this plugin's own static path, so nothing here reaches off-origin.
 *
 * Glyph ranges load over fetch on the main thread, which CSP governs under
 * `connect-src`, and the page policy already grants `connect-src 'self'`. Adding
 * labels therefore widens no policy. A sprite would, because its image half
 * loads under `img-src`, which is why there still is not one.
 */
export function buildStyle(archive: Archive, colors: MapColors): StyleSpecification {
    return {
        version: 8,
        glyphs: `${pluginBaseUrl()}/public/map/fonts/{fontstack}/{range}.pbf`,
        sources: {
            basemap: {type: 'vector', url: `pmtiles://${archive.url}`},
            cell: {type: 'geojson', data: emptyCollection()},
            pin: {type: 'geojson', data: emptyCollection()},
        },
        layers: [
            {id: 'water', type: 'background', paint: {'background-color': colors.water}},
            {
                id: 'land',
                type: 'fill',
                source: 'basemap',
                'source-layer': 'land',
                paint: {'fill-color': colors.land},
            },
            {
                id: 'urban',
                type: 'fill',
                source: 'basemap',
                'source-layer': 'urban_areas',
                paint: {'fill-color': colors.urban},
            },
            {
                id: 'lakes',
                type: 'fill',
                source: 'basemap',
                'source-layer': 'lakes',
                paint: {'fill-color': colors.water},
            },
            {
                id: 'rivers',
                type: 'line',
                source: 'basemap',
                'source-layer': 'rivers',
                paint: {
                    'line-color': colors.water,
                    'line-width': ['interpolate', ['linear'], ['zoom'], 4, 0.4, 8, 1.4, 9, 2],
                },
            },
            {
                id: 'admin-1',
                type: 'line',
                source: 'basemap',
                'source-layer': 'admin_1_lines',
                paint: {
                    'line-color': colors.adminLine,
                    'line-width': 0.5,
                    'line-dasharray': [3, 2],
                },
            },
            {
                id: 'borders',
                type: 'line',
                source: 'basemap',
                'source-layer': 'boundary_lines',
                paint: {'line-color': colors.line, 'line-width': 0.6},
            },
            {
                id: 'railroads',
                type: 'line',
                source: 'basemap',
                'source-layer': 'railroads',
                paint: {
                    'line-color': colors.rail,
                    'line-width': 0.5,
                    'line-dasharray': [2, 3],
                },
            },

            // Natural Earth ranks roads, and the rank is the only attribute
            // kept. It does two jobs: it widens the ones worth following, and
            // it holds the minor ones back until a reader is close enough for
            // them to mean something rather than fill the frame.
            {
                id: 'roads',
                type: 'line',
                source: 'basemap',
                'source-layer': 'roads',
                filter: ['<=', ['coalesce', ['get', 'scalerank'], 12],
                    ['interpolate', ['linear'], ['zoom'], 5, 6, 8, 12]],
                paint: {
                    'line-color': colors.road,
                    'line-width': [
                        'interpolate', ['linear'], ['zoom'],
                        5, 0.4,
                        8, ['interpolate', ['linear'], ['coalesce', ['get', 'scalerank'], 12], 0, 2.2, 12, 0.6],
                        9, ['interpolate', ['linear'], ['coalesce', ['get', 'scalerank'], 12], 0, 3.2, 12, 0.9],
                    ],
                },
            },

            // Every symbol layer sits BELOW the cell and the pin, and that
            // ordering is the only thing protecting them: MapLibre's collision
            // index sees symbols alone, so no text property can make a label
            // yield to a circle or a polygon. A test asserts the indices.
            //
            // Placement runs from the TOP of the layer list DOWN, so among
            // symbol layers the LAST one wins a collision. The label layers
            // below are therefore ordered least wanted first: countries, then
            // provinces, then airfields, then towns.
            {
                id: 'country-label',
                type: 'symbol',
                source: 'basemap',
                'source-layer': 'country_labels',

                // Countries name the view when it spans one; past that the
                // reader can see which country they are in and wants the towns
                // instead.
                maxzoom: 6,
                layout: {
                    'text-field': ['get', 'name'],
                    'text-font': ['NotoSans-Regular'],
                    'text-size': ['interpolate', ['linear'], ['zoom'], 2, 10, 6, 14],
                    'text-max-width': 8,
                    'text-padding': 2,
                    'text-allow-overlap': false,
                    'text-ignore-placement': false,
                    'text-optional': true,
                    'symbol-sort-key': ['get', 'rank'],
                },
                paint: {
                    'text-color': colors.label,
                    'text-halo-color': colors.labelHalo,
                    'text-halo-width': 1.4,
                    'text-halo-blur': 0,
                },
            },

            {
                id: 'admin-1-label',
                type: 'symbol',
                source: 'basemap',
                'source-layer': 'admin_1_labels',
                minzoom: 5,
                filter: ['<=', ['coalesce', ['get', 'scalerank'], 11],
                    ['interpolate', ['linear'], ['zoom'], 5, 3, 9, 9]],
                layout: {
                    'text-field': ['get', 'name'],
                    'text-font': ['NotoSans-Regular'],
                    'text-size': ['interpolate', ['linear'], ['zoom'], 5, 9, 9, 11],
                    'text-transform': 'uppercase',
                    'text-letter-spacing': 0.08,
                    'text-max-width': 8,
                    'text-padding': 2,
                    'text-allow-overlap': false,
                    'text-ignore-placement': false,
                    'text-optional': true,
                    'symbol-sort-key': ['coalesce', ['get', 'scalerank'], 11],
                },
                paint: {
                    'text-color': colors.label,
                    'text-halo-color': colors.labelHalo,
                    'text-halo-width': 1.4,
                    'text-halo-blur': 0,
                },
            },
            {
                id: 'airport-dot',
                type: 'circle',
                source: 'basemap',
                'source-layer': 'airports',
                minzoom: 6,
                filter: ['<=', ['coalesce', ['get', 'scalerank'], 9],
                    ['interpolate', ['linear'], ['zoom'], 6, 4, 9, 9]],
                paint: {
                    'circle-radius': ['interpolate', ['linear'], ['zoom'], 6, 2, 9, 3.5],
                    'circle-color': colors.airport,
                    'circle-stroke-color': colors.labelHalo,
                    'circle-stroke-width': 1,
                },
            },
            {
                id: 'airport-label',
                type: 'symbol',
                source: 'basemap',
                'source-layer': 'airports',
                minzoom: 7,
                filter: ['<=', ['coalesce', ['get', 'scalerank'], 9],
                    ['interpolate', ['linear'], ['zoom'], 7, 5, 9, 9]],
                layout: {
                    'text-field': ['get', 'name'],
                    'text-font': ['NotoSans-Regular'],
                    'text-size': ['interpolate', ['linear'], ['zoom'], 7, 10, 9, 12],
                    'text-offset': [0, 0.9],
                    'text-anchor': 'top',
                    'text-max-width': 8,
                    'text-padding': 2,
                    'text-allow-overlap': false,
                    'text-ignore-placement': false,
                    'text-optional': true,
                    'symbol-sort-key': ['coalesce', ['get', 'scalerank'], 9],
                },
                paint: {
                    'text-color': colors.airport,
                    'text-halo-color': colors.labelHalo,
                    'text-halo-width': 1.4,
                    'text-halo-blur': 0,
                },
            },

            // Natural Earth ranks places, and the rank gates them in the same
            // way it gates roads: capitals and major cities at the zoom where
            // the view still spans a country, villages only once the reader is
            // close. NAME_EN, not NAME, so the bundled Latin ranges cover it.
            {
                id: 'place-label',
                type: 'symbol',
                source: 'basemap',
                'source-layer': 'populated_places',
                minzoom: 3,
                filter: ['<=', ['coalesce', ['get', 'scalerank'], 10],
                    ['interpolate', ['linear'], ['zoom'], 3, 1, 8, 8]],
                layout: {
                    'text-field': ['get', 'name_en'],
                    'text-font': ['NotoSans-Regular'],
                    'text-size': ['interpolate', ['linear'], ['zoom'], 3, 10, 8, 13],
                    'text-max-width': 8,
                    'text-padding': 2,
                    'text-allow-overlap': false,
                    'text-ignore-placement': false,
                    'text-optional': true,
                    'symbol-sort-key': ['coalesce', ['get', 'scalerank'], 10],
                },
                paint: {
                    'text-color': colors.label,
                    'text-halo-color': colors.labelHalo,
                    'text-halo-width': 1.4,
                    'text-halo-blur': 0,
                },
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
