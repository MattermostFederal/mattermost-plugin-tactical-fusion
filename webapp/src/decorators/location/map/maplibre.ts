import type {FeatureCollection} from 'geojson';
import type {StyleSpecification} from 'maplibre-gl';

import type {Archive, Bounds, DetailArchive} from './basemap';
import {DEGREE_METERS, MAX_ZOOM, SEAM_ZOOM} from './span';

import {pluginBaseUrl} from '../../../plugin_url';
import {detectTheme} from '../../theme';

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

/** The same bound basemap.ts puts on its own fetch, for the same reason. */
const LOAD_TIMEOUT_MS = 10000;

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

/**
 * Loads MapLibre, or resolves null. Never throws.
 *
 * A failed load is NOT remembered, which is the same split basemap.ts makes
 * about the archive and for the same reason: the thing that fails here is a
 * ~950 KB chunk fetch, and on the constrained links this plugin is built for a
 * dropped fetch is weather rather than a broken deploy. Latching it meant one
 * stalled request told a reader the map was unavailable for the rest of the
 * session, with a reload the only way back. Clearing `loading` is what lets a
 * later `import()` retry at all: webpack evicts a failed chunk so the next call
 * refetches it, and holding the rejected promise here defeated that.
 *
 * The capability probe is the opposite case and stays memoised: WebGL2 support
 * cannot change mid-session, and re-probing walks the browser's context cap.
 */
export async function loadMapLibre(): Promise<MapLibre | null> {
    if (!hasWebGL2()) {
        return null;
    }
    if (!loading) {
        // Bounded, because a REJECTION is not the only way this fails and it is
        // not the likeliest. A chunk fetch that hangs never rejects, so clearing
        // `loading` in the catch alone left every retry joining the same pending
        // promise and the note reading "Loading map…" forever, with a reload the
        // only way back. That is the failure this policy exists to prevent, one
        // step removed from the one it was written against. basemap.ts bounds
        // its own fetch the same way and for the same reason.
        loading = Promise.race([
            loadAndConfigure(),
            new Promise<never>((_, reject) => {
                setTimeout(() => reject(new Error('maplibre load timed out')), LOAD_TIMEOUT_MS);
            }),
        ]).catch((error: unknown) => {
            // Logged, not swallowed. A failed chunk, a CSP refusal and a throw
            // inside addProtocol all reach the reader as one string, so without
            // this a report of "the map never loads" leaves nothing behind that
            // says which of them it was.
            // eslint-disable-next-line no-console
            console.warn('[tactical-fusion] MapLibre failed to load', error);
            loading = null;

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
    return palette(ALWAYS_DARK || detectTheme() === 'dark');
}

/** @internal exported so the unused half of the palette stays exercised. */
export function palette(dark: boolean): MapColors {
    // Every pair here is measured, not chosen by eye. The land/water edge IS the
    // content of this map, so it carries WCAG 1.4.11's 3:1 for non-text: the
    // first palette sat at 1.46:1 in dark and 1.28:1 in light, which read as a
    // near-uniform slab once the map filled a window. The pin is distinguished
    // by its outline against land rather than by its fill, which is what frees
    // the fill to be chosen for what it MEANS.
    return {
        water: dark ? '#12161d' : '#eef2f7',
        land: dark ? '#5a6472' : '#76889f',
        line: dark ? 'rgba(255,255,255,0.45)' : 'rgba(20,24,32,0.55)',
        cell: dark ? '#b3d1ff' : '#0b2f7a',
        cellFill: dark ? 'rgba(179,209,255,0.18)' : 'rgba(11,47,122,0.16)',

        // Orchid, and deliberately not red. This plugin renders Cursor on
        // Target affiliations on the same maps, where colour is a claim about
        // what a thing IS: red is hostile and suspect, blue is friend, green is
        // neutral, amber is unknown and pending. A location is a place somebody
        // typed and has no affiliation at all, so its pin has to sit outside
        // that vocabulary rather than borrow the most loaded colour in it.
        //
        // Purple through magenta is the ONLY window left. Holding 45 degrees
        // from all four affiliations leaves 252 to 321 and nothing else: the
        // gaps between red and amber, amber and green, and green and blue are
        // all too narrow to stand in. Within that window this is the hue that
        // separates best from everything else drawn on the same map.
        pin: dark ? '#e070e0' : '#8d0da0',
        pinEdge: dark ? '#0b0e13' : '#ffffff',
        label: dark ? '#e8edf5' : '#101418',
        labelHalo: dark ? '#12161d' : '#eef2f7',

        // Roads carry meaning once a reader is close, so they are held to the
        // same measured 3:1 the land/water edge is: 3.43:1 light, 3.69:1 dark.
        road: dark ? '#c2ccd9' : '#2b3542',

        // An airfield is a landmark rather than context, so it is held to the
        // same floor as roads: 3.73:1 light, 3.71:1 dark. It is the one hue on
        // the map that is neither the greys of the basemap nor the orchid of
        // the pin, which is what stops an aerodrome reading as a town.
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
export const MARKER_IMAGE_ID = 'tf-marker';

/** The image name for one colour, so a map registers each colour once. */
export function markerImageID(color: string): string {
    return `${MARKER_IMAGE_ID}-${color.replace('#', '')}`;
}

export function buildStyle(
    archive: Archive, details: readonly DetailArchive[], colors: MapColors,
    overzoomGlobal = false, withAccuracy = false, withMarker = false,
): StyleSpecification {
    const globalCap = overzoomGlobal ? MAX_ZOOM : SEAM_ZOOM;
    const sources: StyleSpecification['sources'] = {
        basemap: {type: 'vector', url: `pmtiles://${archive.url}`},
        cell: {type: 'geojson', data: emptyCollection()},
        pin: {type: 'geojson', data: emptyCollection()},
    };

    // Only where something will draw one. Every location surface states no
    // accuracy, so building the source and its two layers on all of them was
    // work for a shape that could never appear.
    if (withAccuracy) {
        sources.accuracy = {type: 'geojson', data: emptyCollection()};
    }

    for (const detail of details) {
        sources[detailSourceID(detail.name)] = {
            type: 'vector',
            url: `pmtiles://${detail.archive.url}`,
            attribution: OSM_CREDIT.map((credit) => credit.label).join(' '),
        };
    }

    return {
        version: 8,
        glyphs: `${pluginBaseUrl()}/public/map/fonts/{fontstack}/{range}.pbf`,
        sources,
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
                maxzoom: globalCap,
                paint: {'fill-color': colors.urban},
            },
            {
                id: 'lakes',
                type: 'fill',
                source: 'basemap',
                'source-layer': 'lakes',
                maxzoom: globalCap,
                paint: {'fill-color': colors.water},
            },
            {
                id: 'rivers',
                type: 'line',
                source: 'basemap',
                'source-layer': 'rivers',
                maxzoom: globalCap,
                paint: {
                    'line-color': colors.water,
                    'line-width': ['interpolate', ['linear'], ['zoom'], 4, 0.4, 8, 1.4, 9, 2, MAX_ZOOM, 4],
                },
            },
            {
                id: 'admin-1',
                type: 'line',
                source: 'basemap',
                'source-layer': 'admin_1_lines',
                maxzoom: globalCap,
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
                maxzoom: globalCap,
                paint: {'line-color': colors.line, 'line-width': 0.6},
            },
            {
                id: 'railroads',
                type: 'line',
                source: 'basemap',
                'source-layer': 'railroads',
                maxzoom: globalCap,
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
                maxzoom: globalCap,
                filter: ['<=', ['coalesce', ['get', 'scalerank'], 12],
                    ['interpolate', ['linear'], ['zoom'], 5, 6, 8, 12]],
                paint: {
                    'line-color': colors.road,
                    'line-width': [
                        'interpolate', ['linear'], ['zoom'],
                        5, 0.4,
                        8, ['interpolate', ['linear'], ['coalesce', ['get', 'scalerank'], 12], 0, 2.2, 12, 0.6],
                        9, ['interpolate', ['linear'], ['coalesce', ['get', 'scalerank'], 12], 0, 3.2, 12, 0.9],
                        MAX_ZOOM, ['interpolate', ['linear'], ['coalesce', ['get', 'scalerank'], 12], 0, 6.4, 12, 1.8],
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
                maxzoom: globalCap,
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
                maxzoom: globalCap,
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
                maxzoom: globalCap,
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
                maxzoom: globalCap,
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
            ...details.flatMap((detail) => detailLayers(colors, detail.name)),
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
            ...(withAccuracy ? accuracyLayers(colors) : []),
            ...(withMarker ? [{
                id: 'pin',
                type: 'symbol' as const,
                source: 'pin',
                layout: {

                    // Read from the feature, so a block of events draws each
                    // one in its own affiliation's colour.
                    'icon-image': ['get', 'icon'] as unknown as string,
                    'icon-size': 1,

                    // The reticle marks a point, so it stays put and stays put
                    // on top: dropping it because a label got there first would
                    // hide the one thing the map is drawn to show.
                    'icon-allow-overlap': true,
                    'icon-ignore-placement': true,
                },
            }] : [{
                id: 'pin',
                type: 'circle' as const,
                source: 'pin',
                paint: {
                    'circle-radius': 4,
                    'circle-color': colors.pin,
                    'circle-stroke-color': colors.pinEdge,
                    'circle-stroke-width': 1.5,
                },
            }]),
        ],
    };
}

/**
 * The OpenStreetMap tier, in the OpenMapTiles schema, from the seam upward.
 *
 * Every layer here starts at SEAM_ZOOM and every Natural Earth layer that draws
 * the same concern stops there. MapLibre's `minzoom` is inclusive and its
 * `maxzoom` is exclusive, so the pair partitions at exactly the seam with no
 * gap and no overlap, and nothing is ever drawn twice from two sources.
 *
 * `land` is the deliberate exception on the other side: OpenMapTiles has no
 * land polygon, so the Natural Earth fill keeps overzooming underneath and the
 * accurate water above it is what draws the coastline.
 *
 * Every width interpolation carries a stop at MAX_ZOOM. Past the data the
 * camera overzooms, which magnifies geometry while a line width stays in screen
 * pixels, so a network without that stop holds its z14 width while the map
 * doubles around it and reads as thinning out exactly where a reader zoomed in.
 */
export const DETAIL_SOURCE_LAYERS = [
    'aerodrome_label',
    'aeroway',
    'boundary',
    'place',
    'transportation',
    'transportation_name',
    'water',
    'water_name',
    'waterway',
];

/**
 * Whether any installed package reaches anywhere in this view.
 *
 * INTERSECTION rather than a test of the centre, and the asymmetry is
 * deliberate. A centre test lifts the cap whenever the middle of the frame
 * falls outside a package, so a reader who pans until Oahu sits at the edge
 * gets the generalised tier overzoomed across the whole view while the accurate
 * one still draws Oahu: the same road twice, kilometres apart, which is the one
 * failure the seam exists to prevent. Capping whenever any covered ground is on
 * screen cannot do that. It costs a blank margin at the edge of coverage, which
 * is the cheaper of the two.
 *
 * A package's PMTiles header carries one rectangular bounds, so this is the
 * only coverage signal the client has, and it is coarser than the data: a box
 * can contain ground its extracts never held. `indopacom-japan` spans the
 * Ryukyus to Hokkaido and therefore contains Korea, so a Seoul view reads as
 * covered on an install holding Japan and not Korea. One rectangle cannot
 * exclude Korea while keeping Yonaguni, so this is a limit of the shape rather
 * than something to fix here.
 */
export function coveredBy(details: readonly DetailArchive[], view: Bounds): boolean {
    const [west, south, east, north] = view;

    return details.some(({archive}) => {
        const [pWest, pSouth, pEast, pNorth] = archive.bounds;

        return west <= pEast && east >= pWest && south <= pNorth && north >= pSouth;
    });
}

/**
 * The Natural Earth layers whose reach the seam decides.
 *
 * Listed rather than derived from "draws from the basemap source and carries a
 * maxzoom", which was the first shape of this and was wrong: `country-label`
 * carries `maxzoom: 6` to hand over to the town labels, so deriving swept it up
 * and the sync overwrote a handover threshold with a seam cap. `land` carries
 * no maxzoom at all and must stay out either way, since it is what stops an
 * uncovered frame going white.
 *
 * TestSeamCappedLayersAreExactlyTheCappedOnes holds this to the style in both
 * directions, so a layer added on either side fails rather than drifts.
 */
export const SEAM_CAPPED_LAYERS = [
    'urban',
    'lakes',
    'rivers',
    'admin-1',
    'borders',
    'railroads',
    'roads',
    'admin-1-label',
    'airport-dot',
    'airport-label',
    'place-label',
];

/**
 * Re-decides the global tier's reach for where the map now sits.
 *
 * The style is built once and the map is moved thereafter, so without this a
 * reader who pans out of a covered area keeps the cap and sees the empty frame
 * this exists to remove, and one who pans into a covered area sees both tiers
 * draw the same road kilometres apart.
 */
export function syncGlobalReach(
    map: {
        getBounds(): {getWest(): number; getSouth(): number; getEast(): number; getNorth(): number};
        getLayer(id: string): {minzoom?: number; maxzoom?: number} | undefined;
        setLayerZoomRange(id: string, min: number, max: number): void;
    },
    details: readonly DetailArchive[],
    layers: readonly string[],
): void {
    const view = map.getBounds();
    const cap = coveredBy(details, [
        view.getWest(), view.getSouth(), view.getEast(), view.getNorth(),
    ]) ? SEAM_ZOOM : MAX_ZOOM;

    for (const id of layers) {
        const layer = map.getLayer(id);
        if (layer && layer.maxzoom !== cap) {
            map.setLayerZoomRange(id, layer.minzoom ?? 0, cap);
        }
    }
}

/** The style source and layer ids one package's archive is drawn under. */
function detailSourceID(name: string): string {
    return `detail:${name}`;
}

function detailLayers(colors: MapColors, name: string): StyleSpecification['layers'] {
    const source = detailSourceID(name);
    const id = (base: string) => `${base}:${name}`;
    const roadClasses = ['motorway', 'trunk', 'primary', 'secondary', 'tertiary'];
    const label = {
        'text-font': ['NotoSans-Regular'],
        'text-max-width': 8,
        'text-padding': 2,
        'text-allow-overlap': false,
        'text-ignore-placement': false,
        'text-optional': true,
    } as const;
    const labelPaint = {
        'text-color': colors.label,
        'text-halo-color': colors.labelHalo,
        'text-halo-width': 1.4,
        'text-halo-blur': 0,
    } as const;

    // OpenMapTiles carries the local name in `name` and a transliterated one in
    // `name:latin`. The bundled glyph ranges are Latin only, so a local name in
    // Hangul, CJK, Arabic or Ge'ez has no typeface to be drawn in and the label
    // is simply lost. Preferring the transliteration is what keeps a town named
    // at all outside Latin script.
    const named = ['coalesce', ['get', 'name:latin'], ['get', 'name']];

    return [
        {
            id: id('osm-water'),
            type: 'fill',
            source,
            'source-layer': 'water',
            minzoom: SEAM_ZOOM,
            paint: {'fill-color': colors.water},
        },
        {
            id: id('osm-waterway'),
            type: 'line',
            source,
            'source-layer': 'waterway',
            minzoom: SEAM_ZOOM,
            paint: {
                'line-color': colors.water,
                'line-width': ['interpolate', ['linear'], ['zoom'], SEAM_ZOOM, 0.6, 14, 2, MAX_ZOOM, 4],
            },
        },
        {
            id: id('osm-boundary'),
            type: 'line',
            source,
            'source-layer': 'boundary',
            minzoom: SEAM_ZOOM,
            filter: ['all',
                ['<=', ['coalesce', ['get', 'admin_level'], 99], 4],
                ['!=', ['coalesce', ['get', 'disputed'], 0], 1]],
            paint: {
                'line-color': ['case', ['<=', ['coalesce', ['get', 'admin_level'], 99], 2],
                    colors.line, colors.adminLine],
                'line-width': ['case', ['<=', ['coalesce', ['get', 'admin_level'], 99], 2], 0.8, 0.5],
            },
        },

        // A contested line is dashed rather than drawn like a settled one. Not
        // shipping the classification at all was the Natural Earth posture, and
        // silence there is an assertion that a ceasefire line and the
        // France-Germany border are the same kind of thing. OpenStreetMap
        // carries the flag, so this depicts the dispute rather than resolving
        // it.
        {
            id: id('osm-boundary-disputed'),
            type: 'line',
            source,
            'source-layer': 'boundary',
            minzoom: SEAM_ZOOM,
            filter: ['all',
                ['<=', ['coalesce', ['get', 'admin_level'], 99], 4],
                ['==', ['coalesce', ['get', 'disputed'], 0], 1]],
            paint: {
                'line-color': colors.line,
                'line-width': 0.8,
                'line-dasharray': [2, 2],
            },
        },
        {
            id: id('osm-rail'),
            type: 'line',
            source,
            'source-layer': 'transportation',
            minzoom: SEAM_ZOOM,
            filter: ['==', ['get', 'class'], 'rail'],
            paint: {
                'line-color': colors.rail,
                'line-width': 0.5,
                'line-dasharray': [2, 3],
            },
        },
        {
            id: id('osm-runway'),
            type: 'line',
            source,
            'source-layer': 'aeroway',
            minzoom: SEAM_ZOOM,
            filter: ['match', ['get', 'class'], ['runway', 'taxiway'], true, false],
            paint: {
                'line-color': colors.airport,
                'line-width': ['interpolate', ['linear'], ['zoom'],
                    SEAM_ZOOM, ['match', ['get', 'class'], 'runway', 1, 0.4],
                    14, ['match', ['get', 'class'], 'runway', 4, 1.2],
                    MAX_ZOOM, ['match', ['get', 'class'], 'runway', 10, 3]],
            },
        },
        {
            id: id('osm-roads'),
            type: 'line',
            source,
            'source-layer': 'transportation',
            minzoom: SEAM_ZOOM,
            filter: ['match', ['get', 'class'], roadClasses, true, false],
            paint: {
                'line-color': colors.road,
                'line-width': ['interpolate', ['linear'], ['zoom'],
                    SEAM_ZOOM, ['match', ['get', 'class'],
                        'motorway', 1.2, 'trunk', 1, 'primary', 0.8, 'secondary', 0.6, 0.4],
                    14, ['match', ['get', 'class'],
                        'motorway', 4, 'trunk', 3.2, 'primary', 2.6, 'secondary', 2, 1.4],
                    MAX_ZOOM, ['match', ['get', 'class'],
                        'motorway', 10, 'trunk', 8, 'primary', 6.5, 'secondary', 5, 3.5]],
            },
        },

        // Symbol placement runs from the TOP of the layer list DOWN, so among
        // symbol layers the LAST one wins a collision. These are therefore
        // ordered least wanted first: water, then roads, then aerodromes, then
        // towns, which is the landmark somebody placing a coordinate wants most.
        {
            id: id('osm-water-label'),
            type: 'symbol',
            source,
            'source-layer': 'water_name',
            minzoom: SEAM_ZOOM,
            layout: {...label, 'text-field': named, 'text-size': 11, 'text-transform': 'none'},
            paint: {...labelPaint, 'text-color': colors.label},
        },
        {
            id: id('osm-road-label'),
            type: 'symbol',
            source,
            'source-layer': 'transportation_name',
            minzoom: 12,
            layout: {
                ...label,
                'text-field': ['coalesce', ['get', 'ref'], named],
                'text-size': 10,
                'symbol-placement': 'line',
            },
            paint: {...labelPaint, 'text-color': colors.road},
        },
        {
            id: id('osm-aerodrome-dot'),
            type: 'circle',
            source,
            'source-layer': 'aerodrome_label',
            minzoom: SEAM_ZOOM,
            paint: {
                'circle-radius': ['interpolate', ['linear'], ['zoom'], SEAM_ZOOM, 2.5, 14, 4],
                'circle-color': colors.airport,
                'circle-stroke-color': colors.labelHalo,
                'circle-stroke-width': 1,
            },
        },
        {
            id: id('osm-aerodrome-label'),
            type: 'symbol',
            source,
            'source-layer': 'aerodrome_label',
            minzoom: SEAM_ZOOM,
            layout: {
                ...label,
                'text-field': named,
                'text-size': 11,
                'text-offset': [0, 0.9],
                'text-anchor': 'top',
            },
            paint: {...labelPaint, 'text-color': colors.airport},
        },
        {
            id: id('osm-place-label'),
            type: 'symbol',
            source,
            'source-layer': 'place',
            minzoom: SEAM_ZOOM,
            filter: ['match', ['get', 'class'],
                ['city', 'town', 'village', 'hamlet', 'suburb'], true, false],
            layout: {
                ...label,
                'text-field': named,
                'text-size': ['interpolate', ['linear'], ['zoom'], SEAM_ZOOM, 10, 14, 13],
                'symbol-sort-key': ['coalesce', ['get', 'rank'], 20],
            },
            paint: labelPaint,
        },
    ] as StyleSpecification['layers'];
}

/**
 * The credit the detail tier may not be drawn without.
 *
 * OpenStreetMap is ODbL and the OpenMapTiles schema is CC-BY, so unlike Natural
 * Earth, whose credit this plugin deliberately dropped, both of these are
 * licence conditions. Written once here and read by both the style's own
 * `attribution` field and the line the component renders, so the two cannot
 * disagree about what was credited.
 */
export const OSM_CREDIT: ReadonlyArray<{label: string; href: string}> = [
    {label: '© OpenMapTiles', href: 'https://openmaptiles.org/'},
    {label: '© OpenStreetMap contributors', href: 'https://www.openstreetmap.org/copyright'},
];

export function emptyCollection(): FeatureCollection {
    return {type: 'FeatureCollection', features: []};
}

/**
 * One marked point per event, each naming the image it is drawn with.
 *
 * The image is named per feature rather than per layer because a block of
 * events is a block of different affiliations, and one layer drawing one icon
 * would paint a hostile track in a friendly colour.
 */
export function markedPoints(
    points: ReadonlyArray<{lat: number; lon: number; icon: string}>,
): FeatureCollection {
    return {
        type: 'FeatureCollection',
        features: points.map((point) => ({
            type: 'Feature',
            properties: {icon: point.icon},
            geometry: {type: 'Point', coordinates: [point.lon, point.lat]},
        })),
    };
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

/**
 * A stated circular error, as a polygon on the ground rather than on the screen.
 *
 * MapLibre's own circle layer takes a radius in PIXELS, which would mean the
 * drawn accuracy changed every time the reader zoomed. A polygon is the only
 * shape that keeps its metres.
 *
 * The vertices are geodesic. A metre is a different number of longitude degrees
 * at every latitude, so an equal-degree ring is right at the equator and wrong
 * everywhere else, which for a layer whose whole purpose is to stop the map
 * overstating a fix is the wrong way to be wrong.
 */
export const ACCURACY_VERTICES = 64;

/**
 * The circle's two layers, drawn UNDER the pin.
 *
 * Order is the contract: a fill drawn over the marker would hide the position
 * the circle exists to qualify.
 */
export function accuracyLayers(colors: MapColors): StyleSpecification['layers'] {
    return [
        {
            id: 'accuracy-fill',
            type: 'fill',
            source: 'accuracy',
            paint: {'fill-color': colors.cellFill},
        },
        {
            id: 'accuracy-outline',
            type: 'line',
            source: 'accuracy',
            paint: {'line-color': colors.cell, 'line-width': 1.5, 'line-dasharray': [2, 2]},
        },
    ];
}

export function accuracyFeature(lat: number, lon: number, meters: number): FeatureCollection {
    const cosLat = Math.cos((lat * Math.PI) / 180);
    if (!Number.isFinite(lat) || !Number.isFinite(lon) ||
        !Number.isFinite(meters) || meters <= 0 || Math.abs(cosLat) < 1e-9) {
        return emptyCollection();
    }

    const dLat = meters / DEGREE_METERS;
    const dLon = meters / (DEGREE_METERS * cosLat);

    const ring: Array<[number, number]> = [];
    for (let i = 0; i < ACCURACY_VERTICES; i++) {
        const angle = (2 * Math.PI * i) / ACCURACY_VERTICES;
        ring.push([lon + (dLon * Math.cos(angle)), lat + (dLat * Math.sin(angle))]);
    }

    // Closed by copying the first position rather than by computing the last
    // one. sin(2 pi) is 1e-16 rather than 0, so the computed form leaves a ring
    // GeoJSON considers unclosed.
    if (ring.length === 0) {
        return emptyCollection();
    }
    ring.push(ring[0]);

    return {
        type: 'FeatureCollection',
        features: [{
            type: 'Feature',
            properties: {},
            geometry: {type: 'Polygon', coordinates: [ring]},
        }],
    };
}

/**
 * The marker: a circle with a horizontal and a vertical line through it.
 *
 * Built as a raw RGBA buffer rather than through a canvas or an SVG, for two
 * reasons. It is a pure function of its arguments, so it is unit tested against
 * the pixels rather than against a screenshot. And it needs no glyph: a Unicode
 * crosshair would depend on the font ranges this plugin trims for an air-gapped
 * bundle, so the character could simply be absent.
 *
 * Drawn from signed distances rather than by testing whether a pixel is inside
 * the shape. A yes-or-no test gives every curve a stepped edge, which at this
 * size reads as grain rather than as a line; a distance gives each pixel the
 * fraction of it the shape covers, which is what antialiasing is.
 *
 * The shape is stroked twice, the edge colour first and wider. That outline is
 * what the palette's note relies on ("the pin is distinguished by its outline
 * against land rather than by its fill"), and it is what lets the fill carry
 * the affiliation.
 */
export const MARKER_SIZE = 20;

/**
 * Four device pixels per CSS pixel, which is oversampling even a retina screen.
 * The marker is 20px on screen and every edge in it is a curve or a thin line,
 * so the cost of the extra samples is a few kilobytes and the benefit is that
 * nothing steps.
 */
export const MARKER_PIXEL_RATIO = 4;

const RING = 0.56;

// Chosen against the rendered shape rather than by taste: past about 0.125 the
// ring and the lines merge and the circle stops reading as one.
const THICKNESS = 0.105;
const EDGE_THICKNESS = 0.045;
const LINE_REACH = 0.90;

export interface MarkerImage {
    width: number;
    height: number;
    data: Uint8Array;
}

/** Distance to a rectangle centred on the origin, negative inside it. */
function toBox(px: number, py: number, halfWidth: number, halfHeight: number): number {
    const dx = Math.abs(px) - halfWidth;
    const dy = Math.abs(py) - halfHeight;

    return Math.min(Math.max(dx, dy), 0) + Math.hypot(Math.max(dx, 0), Math.max(dy, 0));
}

/** Distance to the whole marker, negative inside it. */
function toMarker(nx: number, ny: number, half: number): number {
    const ring = Math.abs(Math.hypot(nx, ny) - RING) - half;

    // The line grows along its length as well as across it, so the wider edge
    // pass caps its ends. Growing only the thickness left the last pixel of
    // each line as bare fill meeting the map, which is the one place the
    // outline has to be.
    const reach = LINE_REACH + (half - THICKNESS);
    const across = toBox(nx, ny, reach, half);
    const down = toBox(nx, ny, half, reach);

    return Math.min(ring, across, down);
}

function channels(hex: string): [number, number, number] {
    const value = hex.replace('#', '');
    const full = value.length === 3 ? value.split('').map((c) => c + c).join('') : value;
    return [
        parseInt(full.slice(0, 2), 16) || 0,
        parseInt(full.slice(2, 4), 16) || 0,
        parseInt(full.slice(4, 6), 16) || 0,
    ];
}

export function crosshairImage(color: string, edge: string): MarkerImage {
    const side = MARKER_SIZE * MARKER_PIXEL_RATIO;
    const data = new Uint8Array(side * side * 4);

    const fillRGB = channels(color);
    const edgeRGB = channels(edge);
    const centre = (side - 1) / 2;

    // One pixel, in the units the distances are measured in. Coverage ramps
    // across exactly this, which is what makes an edge look soft rather than
    // stepped without making it look blurred.
    const pixel = 1 / centre;

    const coverage = (distance: number) =>
        Math.max(0, Math.min(1, 0.5 - (distance / pixel)));

    for (let y = 0; y < side; y++) {
        for (let x = 0; x < side; x++) {
            const nx = (x - centre) / centre;
            const ny = (y - centre) / centre;

            const outer = coverage(toMarker(nx, ny, THICKNESS + EDGE_THICKNESS));
            if (outer <= 0) {
                continue;
            }
            const inner = coverage(toMarker(nx, ny, THICKNESS));

            // The fill over the edge, both already covering their own share of
            // the pixel. Straight rather than premultiplied, which is what
            // addImage takes.
            const at = ((y * side) + x) * 4;
            for (let c = 0; c < 3; c++) {
                data[at + c] = Math.round(
                    ((fillRGB[c] * inner) + (edgeRGB[c] * (outer - inner))) / outer,
                );
            }
            data[at + 3] = Math.round(outer * 255);
        }
    }

    return {width: side, height: side, data};
}

/** @internal exported for tests */
export function _assetUrlForTesting(imported: unknown): string | null { // eslint-disable-line no-underscore-dangle, @typescript-eslint/naming-convention
    return assetUrl(imported);
}

/** Test hook, for the same reason basemap.ts has one. */
export function _resetForTesting(): void { // eslint-disable-line no-underscore-dangle, @typescript-eslint/naming-convention
    loading = null;
    webgl2 = null;
}
