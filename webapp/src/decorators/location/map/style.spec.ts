import {expect, test} from '@playwright/test';

import type {Archive, Bounds} from './basemap';
import {
    DETAIL_SOURCE_LAYERS,
    OSM_CREDIT,
    SEAM_CAPPED_LAYERS,
    _assetUrlForTesting as assetUrl, buildStyle, cellFeature, coveredBy, emptyCollection,
    mapColors, palette, pointFeature, syncGlobalReach,
} from './maplibre';
import {cellBounds, DATA_MAX_ZOOM, MAX_ZOOM, SEAM_ZOOM, zoomForSpan} from './span';

/*
 * The style object is where the air-gap promise lives, so it is asserted rather
 * than described. This is the webapp counterpart of the Go page's
 * TestRenderPageCSPGainsNoImgSrcAndNoFontSrc.
 */

const ARCHIVE: Archive = {
    url: '/plugins/x/public/map/world.pmtiles?v=1',
    minZoom: 0,
    maxZoom: DATA_MAX_ZOOM,
    bounds: [-180, -85, 180, 85],
};

const DETAIL: Archive = {
    url: '/plugins/x/packages/indopacom-hawaii.pmtiles?v=1',
    minZoom: SEAM_ZOOM,
    maxZoom: 14,
    bounds: [-160.6, 18.6, -154.6, 22.4],
};

/** A view nowhere near any package, for the sync stubs. */
const far = {getWest: () => -98.2, getSouth: () => 41.4, getEast: () => -97.8, getNorth: () => 41.6};

const PACKAGE_NAME_FIXTURE = 'indopacom-hawaii';
const PACKAGE = {name: PACKAGE_NAME_FIXTURE, archive: DETAIL};

/** A detail layer's id carries the package it is drawn from. */
const osm = (base: string) => `${base}:${PACKAGE_NAME_FIXTURE}`;

/** Every URL a MapLibre style can resolve: the two URL fields, plus tile lists. */
function urlsIn(style: ReturnType<typeof buildStyle>): string[] {
    const urls: string[] = [];
    if (style.glyphs) {
        urls.push(style.glyphs);
    }
    if ((style as {sprite?: string}).sprite) {
        urls.push((style as {sprite: string}).sprite);
    }
    for (const source of Object.values(style.sources)) {
        const spec = source as {url?: string; tiles?: string[]};
        if (spec.url) {
            urls.push(spec.url);
        }
        urls.push(...(spec.tiles ?? []));
    }

    return urls;
}

/*
 * The air-gap promise. This used to read "there are no URLs in the style", which
 * a vector source and a glyphs path both make false. The promise it was standing
 * in for is the one asserted now, and it is the stronger statement: it names
 * where bytes may come from rather than which two fields are forbidden.
 */
test('every URL in the style is served by this plugin', () => {
    const urls = urlsIn(buildStyle(ARCHIVE, [], mapColors()));

    expect(urls.length).toBeGreaterThan(0);
    for (const url of urls) {
        // pmtiles:// is this style's one scheme, and it wraps a same-origin
        // path rather than replacing it.
        const bare = url.replace(/^pmtiles:\/\//, '');

        expect(bare).toMatch(/^\/plugins\//);
        expect(bare).not.toMatch(/^https?:/i);
        expect(bare).not.toMatch(/^\/\//);
    }
});

test('the style still has no sprite, which would widen img-src', () => {
    // Glyphs load over fetch, which connect-src already allows. A sprite's image
    // half loads under img-src, which the page policy does not grant.
    expect(JSON.stringify(buildStyle(ARCHIVE, [], mapColors()))).not.toContain('sprite');
});

test('the overlay sources stay inlined, and only the basemap is tiled', () => {
    const style = buildStyle(ARCHIVE, [], mapColors());

    for (const id of ['cell', 'pin']) {
        const source = style.sources[id] as {type: string; data?: unknown};
        expect(source.type).toBe('geojson');
        expect(typeof source.data).toBe('object');
    }
    expect(style.sources.basemap.type).toBe('vector');
});

// The discriminator that existed only because three layers shared one GeoJSON
// source. A half-migrated filter draws nothing and reads as a data problem.
test('no layer filters on the old inlined-layer discriminator', () => {
    const style = buildStyle(ARCHIVE, [], mapColors());

    for (const layer of style.layers) {
        expect(JSON.stringify((layer as {filter?: unknown}).filter ?? null)).not.toContain('layer');
    }
});

test('every basemap layer names a source-layer', () => {
    const style = buildStyle(ARCHIVE, [], mapColors());

    for (const layer of style.layers) {
        const spec = layer as {source?: string; 'source-layer'?: string};
        if (spec.source === 'basemap') {
            expect(spec['source-layer']).toBeTruthy();
        }
    }
});

/*
 * The pin and the cell are what the reader opened the panel for. MapLibre's
 * collision index sees symbols alone, so no text property can make a label yield
 * to them: draw order is the entire mechanism, which is why it is asserted here
 * rather than left as a convention in the style.
 */
test('every label is drawn beneath the cell and the pin', () => {
    const style = buildStyle(ARCHIVE, [], mapColors());
    const indexOf = (id: string) => style.layers.findIndex((layer) => layer.id === id);
    const overlay = Math.min(indexOf('cell-fill'), indexOf('cell-outline'), indexOf('pin'));

    const symbols = style.layers.
        map((layer, i) => ({type: layer.type, i})).
        filter((entry) => entry.type === 'symbol');

    expect(symbols.length).toBeGreaterThan(0);
    for (const symbol of symbols) {
        expect(symbol.i).toBeLessThan(overlay);
    }
});

/*
 * The layer set, as the style declares it.
 *
 * This does NOT check the archive: it cannot open one, and it used to claim it
 * did. Whether every source-layer named here exists in the archive, and whether
 * every layer the archive carries is drawn, is held by
 * TestArchiveCarriesEveryLayerTheStyleDraws in Go, which is the only place that
 * can read both the binary and this file.
 *
 * What it is still worth pinning here is that the set does not change by
 * accident: adding a layer is a deliberate act, and this is the diff that says
 * so.
 */
test('the style declares exactly the layers it is meant to', () => {
    const style = buildStyle(ARCHIVE, [], mapColors());

    const drawn = new Set(style.layers.
        map((layer) => (layer as {'source-layer'?: string})['source-layer']).
        filter(Boolean));

    expect([...drawn].sort()).toEqual([
        'admin_1_labels',
        'admin_1_lines',
        'airports',
        'boundary_lines',
        'country_labels',
        'lakes',
        'land',
        'populated_places',
        'railroads',
        'rivers',
        'roads',
        'urban_areas',
    ]);
});

/*
 * A town is a better landmark than a province name or an aerodrome when a reader
 * is placing a coordinate, so towns take the collision and the rest fill what is
 * left.
 *
 * Layer order is the only thing that can express this: the collision index sees
 * symbols alone, so no text property reaches it. MapLibre runs placement from
 * the TOP of the layer list DOWN, which means the LAST symbol layer wins, and
 * the order below therefore reads least-wanted first.
 *
 * Getting it backwards is not a subtle regression and is not loud either: three
 * new label layers appended after place-label silently suppressed the town label
 * on Paris, and the only symptom was one assertion counting zero.
 */
test('label layers are ordered least wanted first', () => {
    const style = buildStyle(ARCHIVE, [], mapColors());
    const at = (id: string) => style.layers.findIndex((l) => l.id === id);

    expect(at('country-label')).toBeLessThan(at('admin-1-label'));
    expect(at('admin-1-label')).toBeLessThan(at('airport-label'));
    expect(at('airport-label')).toBeLessThan(at('place-label'));
    expect(at('place-label')).toBeLessThan(at('cell-fill'));
});

// The aerodrome and its name are one feature drawn as two layers, so a filter
// that lets the label through at a zoom the dot is absent from would caption
// nothing.
test('an airport label never outruns its dot', () => {
    const style = buildStyle(ARCHIVE, [], mapColors());
    const byId = (id: string) => style.layers.find((l) => l.id === id) as {minzoom?: number};

    expect(byId('airport-label').minzoom).toBeGreaterThanOrEqual(byId('airport-dot').minzoom!);
});

// Countries name the view while it spans one and hand over to towns past that.
// The maxzoom is what ends the country label; the ordering below is checked by
// 'label layers are ordered least wanted first', which states the real rule.
test('country labels hand over to place labels', () => {
    const style = buildStyle(ARCHIVE, [], mapColors());
    const byId = (id: string) => style.layers.find((layer) => layer.id === id) as {
        minzoom?: number; maxzoom?: number;
    };

    expect(byId('country-label').maxzoom).toBe(6);
    expect(byId('place-label').minzoom).toBeLessThanOrEqual(6);
    expect(style.layers.findIndex((l) => l.id === 'country-label')).
        toBeLessThan(style.layers.findIndex((l) => l.id === 'place-label'));
});

test('a point feature is longitude first, as GeoJSON requires', () => {
    // Every other surface in this plugin is latitude first, and lon/lat
    // inversion is one of the bugs this repo deliberately did not inherit.
    const geometry = pointFeature(34.0561, -118.25).features[0].geometry;
    if (geometry.type !== 'Point') {
        throw new Error(`expected a Point, got ${geometry.type}`);
    }

    expect(geometry.coordinates).toEqual([-118.25, 34.0561]);
});

test('a cell feature closes its ring and is longitude first', () => {
    const geometry = cellFeature(cellBounds(35, -79, 1, 1)).features[0].geometry;
    if (geometry.type !== 'Polygon') {
        throw new Error(`expected a Polygon, got ${geometry.type}`);
    }
    const ring = geometry.coordinates[0];

    expect(ring).toHaveLength(5);
    expect(ring[0]).toEqual(ring[4]);
    expect(ring[0][0]).toBeCloseTo(-79.5, 6);
    expect(ring[0][1]).toBeCloseTo(34.5, 6);
});

test('an empty collection is a valid empty source', () => {
    expect(emptyCollection()).toEqual({type: 'FeatureCollection', features: []});
});

/*
 * The worker URL, which bundlers disagree about.
 *
 * webpack's asset/resource emits CommonJS, so a dynamic import yields
 * {default: url}. A bundler treating the .mjs as real ESM executes the worker
 * instead and yields its exports, where no URL exists. Handing MapLibre the
 * second shape stores a non-string as WORKER_URL, endsWith throws inside the
 * worker setup, the style never finishes, and the panel sits on "Loading map…"
 * forever with nothing logged.
 */
test('an asset URL is recognised whatever shape the bundler gives it', () => {
    expect(assetUrl('/static/plugins/x/worker.mjs')).toBe('/static/plugins/x/worker.mjs');
    expect(assetUrl({default: '/static/plugins/x/worker.mjs'})).toBe('/static/plugins/x/worker.mjs');
});

test('anything that is not a URL degrades to null rather than reaching MapLibre', () => {
    // The shape a real-ESM bundler produces: the worker's own exports.
    expect(assetUrl({default: {}, PerformanceUtils: {}})).toBeNull();
    expect(assetUrl({})).toBeNull();
    expect(assetUrl(undefined)).toBeNull();
    expect(assetUrl(null)).toBeNull();
    expect(assetUrl(42)).toBeNull();
});

/*
 * The map is drawn dark whatever theme the reader is in.
 *
 * This runs where there is no document, so `themeColor` falls back to a white
 * background and the theme sniffing therefore reports LIGHT. That is what makes
 * this assertion worth anything: it is the case that used to return the light
 * palette, and it does not any more.
 */
test('the palette does not follow the theme', () => {
    expect(mapColors()).toEqual(palette(true));
    expect(mapColors().land).not.toBe(palette(false).land);
});

/*
 * The light half is kept rather than deleted, so it has to stay correct.
 *
 * Its measured contrast is held by TestMapPaletteCarriesItsContrast on the Go
 * side, which reads the hex values out of maplibre.ts; what that cannot see is
 * whether anything still RETURNS them. Between the two, a palette nothing draws
 * today cannot quietly rot into one that would be wrong when it is drawn again.
 */
test('the unused light palette is still whole', () => {
    const light = palette(false);
    const dark = palette(true);

    expect(Object.keys(light).sort()).toEqual(Object.keys(dark).sort());
    for (const [name, value] of Object.entries(light)) {
        expect(value, `${name} is not a colour`).toMatch(/^(#[0-9a-f]{6}|rgba\()/i);
        expect(value, `${name} is the same in both themes`).not.toBe(dark[name as keyof typeof dark]);
    }
});

/*
 * The seam between the two tiers.
 *
 * MapLibre's layer `minzoom` is inclusive and its `maxzoom` is exclusive, so a
 * Natural Earth layer capped at SEAM_ZOOM and an OpenStreetMap layer starting
 * at SEAM_ZOOM partition exactly, with no zoom at which neither draws and none
 * at which both do. The failure in one direction is a blank band; in the other
 * it is the same road drawn twice, kilometres apart, which reads as a rendering
 * artefact rather than as two sources disagreeing.
 */
const NE_CAPPED = [
    'urban', 'lakes', 'rivers', 'admin-1', 'borders', 'railroads', 'roads',
    'admin-1-label', 'airport-dot', 'airport-label', 'place-label',
];

test('every Natural Earth detail layer stops at the seam', () => {
    const style = buildStyle(ARCHIVE, [PACKAGE], mapColors());

    for (const id of NE_CAPPED) {
        const layer = style.layers.find((l) => l.id === id) as {maxzoom?: number} | undefined;

        expect(layer, `${id} is gone from the style`).toBeTruthy();
        expect(layer!.maxzoom, `${id} outlives the seam`).toBe(SEAM_ZOOM);
    }
});

test('every OpenStreetMap layer starts at the seam or later', () => {
    const style = buildStyle(ARCHIVE, [PACKAGE], mapColors());
    const detail = style.layers.filter((l) => (l as {source?: string}).source?.startsWith('detail:'));

    expect(detail.length).toBeGreaterThan(0);
    for (const layer of detail) {
        const spec = layer as {id: string; minzoom?: number};

        expect(spec.minzoom, `${spec.id} draws below the seam`).toBeGreaterThanOrEqual(SEAM_ZOOM);
    }
});

/*
 * `land` is the deliberate exception, and capping it is the change that would
 * empty the map outside a covered region rather than merely thin it: OpenMapTiles
 * has no land polygon, so the Natural Earth fill is what the accurate water is
 * drawn on top of. Asserted by name, because the cost of getting it wrong is a
 * white frame everywhere the detail tier does not reach.
 */
test('the land fill is the one Natural Earth layer that outlives the seam', () => {
    const style = buildStyle(ARCHIVE, [PACKAGE], mapColors());
    const land = style.layers.find((l) => l.id === 'land') as {maxzoom?: number};

    expect(land.maxzoom).toBeUndefined();

    const water = style.layers.findIndex((l) => l.id === osm('osm-water'));
    expect(water).toBeGreaterThan(style.layers.findIndex((l) => l.id === 'land'));
});

// No concern may be drawn by both tiers at once. Every pair here is one thing a
// reader sees, named once per source, and the seam is the only thing keeping
// them apart.
test('no concern is drawn from both sources at any zoom', () => {
    const style = buildStyle(ARCHIVE, [PACKAGE], mapColors());
    const byId = (id: string) => style.layers.find((l) => l.id === id) as {
        minzoom?: number; maxzoom?: number;
    };

    const pairs: Array<[string, string]> = [
        ['lakes', osm('osm-water')],
        ['rivers', osm('osm-waterway')],
        ['borders', osm('osm-boundary')],
        ['admin-1', osm('osm-boundary')],
        ['railroads', osm('osm-rail')],
        ['roads', osm('osm-roads')],
        ['place-label', osm('osm-place-label')],
        ['airport-dot', osm('osm-aerodrome-dot')],
        ['airport-label', osm('osm-aerodrome-label')],
    ];

    for (const [ne, osm] of pairs) {
        const stops = byId(ne).maxzoom;
        const starts = byId(osm).minzoom;

        expect(stops, `${ne} never stops`).toBeDefined();
        expect(starts, `${osm} never starts`).toBeDefined();
        expect(stops, `${ne} and ${osm} both draw somewhere`).toBe(starts);
    }
});

test('the detail layers are drawn beneath the cell and the pin', () => {
    const style = buildStyle(ARCHIVE, [PACKAGE], mapColors());
    const overlay = Math.min(
        style.layers.findIndex((l) => l.id === 'cell-fill'),
        style.layers.findIndex((l) => l.id === 'cell-outline'),
        style.layers.findIndex((l) => l.id === 'pin'),
    );

    const detail = style.layers.
        map((layer, i) => ({source: (layer as {source?: string}).source, id: layer.id, i})).
        filter((entry) => entry.source?.startsWith('detail:'));

    expect(detail.length).toBeGreaterThan(0);
    for (const layer of detail) {
        expect(layer.i, `${layer.id} is drawn over the pin`).toBeLessThan(overlay);
    }
});

test('the detail style declares exactly the source-layers it is meant to', () => {
    const style = buildStyle(ARCHIVE, [PACKAGE], mapColors());

    const drawn = new Set(style.layers.
        filter((layer) => (layer as {source?: string}).source?.startsWith('detail:')).
        map((layer) => (layer as {'source-layer'?: string})['source-layer']).
        filter(Boolean));

    // Held to the exported list rather than to a second copy, because that
    // list is what Go reads to check every committed package archive: the
    // style, the constant and the archives are one chain, and a copy here would
    // let the middle link drift while both ends still passed.
    expect([...drawn].sort()).toEqual([...DETAIL_SOURCE_LAYERS].sort());
    expect(DETAIL_SOURCE_LAYERS.length).toBeGreaterThan(0);
});

// A global-only build is a supported shipping profile, so the absent archive
// must not be a degraded map: it is the whole map, unchanged, plus the caps.
test('without a detail archive the style carries no detail source at all', () => {
    const style = buildStyle(ARCHIVE, [], mapColors());

    expect(Object.keys(style.sources).some((id) => id.startsWith('detail:'))).toBe(false);
    expect(style.layers.some((l) => (l as {source?: string}).source?.startsWith('detail:'))).toBe(false);
    expect(JSON.stringify(style)).not.toContain('OpenStreetMap');
});

/*
 * The credit is a licence condition rather than a courtesy, which is what makes
 * it unlike the Natural Earth credit this plugin deliberately dropped:
 * OpenStreetMap is ODbL and the OpenMapTiles schema is CC-BY. Written once and
 * read by both the style and the line the component renders, so the two cannot
 * disagree about what was credited.
 */
test('the detail source carries the credit its licences require', () => {
    const style = buildStyle(ARCHIVE, [PACKAGE], mapColors());
    const attribution = (style.sources[`detail:${PACKAGE_NAME_FIXTURE}`] as {attribution?: string}).attribution ?? '';

    expect(attribution).toContain('OpenStreetMap contributors');
    expect(attribution).toContain('OpenMapTiles');

    const hrefs = OSM_CREDIT.map((credit) => credit.href);
    expect(hrefs).toContain('https://www.openstreetmap.org/copyright');
    expect(hrefs).toContain('https://openmaptiles.org/');
    for (const credit of OSM_CREDIT) {
        expect(attribution).toContain(credit.label);
    }
});

/*
 * The hover card can never reach the seam, which is the whole argument for it
 * carrying no detail source and therefore no credit. It is built with
 * interactive: false at a zoom zoomForSpan produced, so if that function can
 * ever return SEAM_ZOOM the card could draw OpenStreetMap with nothing crediting
 * it. Driven across the widths and latitudes the surfaces actually use.
 */
test('a preview camera cannot reach the seam', () => {
    for (let lat = -85; lat <= 85; lat += 5) {
        for (const width of [200, 320, 360, 640, 900, 1600, 2560]) {
            expect(zoomForSpan(lat, width), `lat ${lat} at ${width}px`).toBeLessThan(SEAM_ZOOM);
        }
    }
});

test('a view meeting a package bbox is covered and one clear of it is not', () => {
    expect(coveredBy([PACKAGE], [-158.0, 21.2, -157.6, 21.4])).toBe(true);
    expect(coveredBy([PACKAGE], [-98.2, 41.4, -97.8, 41.6])).toBe(false);
    expect(coveredBy([], [-158.0, 21.2, -157.6, 21.4])).toBe(false);
});

/*
 * The centre test this replaced said "not covered" here, which uncapped the
 * generalised tier across a view the accurate one was still drawing into.
 */
test('a view whose centre has left coverage is still covered while any of it is on screen', () => {
    // Centre is -153.25, east of the package's -154.6 edge, but its western
    // half still holds covered ground.
    const straddling: Bounds = [-155.5, 19.0, -151.0, 22.0];
    const centre: Bounds = [-153.25, 20.5, -153.25, 20.5];

    expect(coveredBy([PACKAGE], centre)).toBe(false);
    expect(coveredBy([PACKAGE], straddling)).toBe(true);
});

test('the global tier overzooms past the seam where no package covers', () => {
    const capped = buildStyle(ARCHIVE, [PACKAGE], mapColors(), false);
    const reaching = buildStyle(ARCHIVE, [PACKAGE], mapColors(), true);
    const maxzoomOf = (style: ReturnType<typeof buildStyle>, id: string) =>
        (style.layers.find((l) => l.id === id) as {maxzoom?: number}).maxzoom;

    for (const id of ['roads', 'rivers', 'borders', 'place-label']) {
        expect(maxzoomOf(capped, id)).toBe(SEAM_ZOOM);
        expect(maxzoomOf(reaching, id)).toBe(MAX_ZOOM);
    }
});

test('the land fill is never capped, so an uncovered frame is never white', () => {
    for (const overzoom of [false, true]) {
        const style = buildStyle(ARCHIVE, [PACKAGE], mapColors(), overzoom);
        const land = style.layers.find((l) => l.id === 'land') as {maxzoom?: number};

        expect(land.maxzoom).toBeUndefined();
        expect(SEAM_CAPPED_LAYERS).not.toContain('land');
    }
});

/*
 * Both directions. A layer that gains the seam cap without joining the list is
 * never lifted where nothing covers, and one on the list that does not carry it
 * has its own maxzoom overwritten by syncGlobalReach, which is how
 * `country-label` lost its handover to the town labels.
 */
test('SEAM_CAPPED_LAYERS is exactly the layers the seam caps', () => {
    for (const [overzoom, cap] of [[false, SEAM_ZOOM], [true, MAX_ZOOM]] as Array<[boolean, number]>) {
        const style = buildStyle(ARCHIVE, [PACKAGE], mapColors(), overzoom);
        const capped = style.layers.
            filter((l) => (l as {maxzoom?: number}).maxzoom === cap).
            map((l) => l.id);

        expect([...capped].sort()).toEqual([...SEAM_CAPPED_LAYERS].sort());
    }
});

test('country labels keep their handover threshold through a coverage change', () => {
    const style = buildStyle(ARCHIVE, [PACKAGE], mapColors());
    const before = style.layers.find((l) => l.id === 'country-label') as {maxzoom?: number};

    expect(before.maxzoom).toBe(6);
    expect(SEAM_CAPPED_LAYERS).not.toContain('country-label');

    const zooms = new Map<string, {minzoom?: number; maxzoom?: number}>([['country-label', {...before}]]);
    syncGlobalReach({
        getBounds: () => far,
        getLayer: (id: string) => zooms.get(id),
        setLayerZoomRange: (id: string, min: number, max: number) => {
            zooms.set(id, {minzoom: min, maxzoom: max});
        },
    }, [], SEAM_CAPPED_LAYERS);

    expect(zooms.get('country-label')?.maxzoom).toBe(6);
});

test('panning out of coverage lifts the cap and panning back restores it', () => {
    const style = buildStyle(ARCHIVE, [PACKAGE], mapColors());
    const reach = SEAM_CAPPED_LAYERS;
    const zooms = new Map<string, {minzoom?: number; maxzoom?: number}>(
        reach.map((id) => [id, {...(style.layers.find((l) => l.id === id) as {minzoom?: number; maxzoom?: number})}]),
    );

    let view: Bounds = [-98.2, 41.4, -97.8, 41.6];
    const map = {
        getBounds: () => ({
            getWest: () => view[0],
            getSouth: () => view[1],
            getEast: () => view[2],
            getNorth: () => view[3],
        }),
        getLayer: (id: string) => zooms.get(id),
        setLayerZoomRange: (id: string, min: number, max: number) => {
            zooms.set(id, {minzoom: min, maxzoom: max});
        },
    };

    syncGlobalReach(map, [PACKAGE], reach);
    expect(zooms.get('roads')?.maxzoom).toBe(MAX_ZOOM);

    view = [-158.0, 21.2, -157.6, 21.4];
    syncGlobalReach(map, [PACKAGE], reach);
    expect(zooms.get('roads')?.maxzoom).toBe(SEAM_ZOOM);
});

test('a label layer keeps its own minzoom when the cap moves', () => {
    const style = buildStyle(ARCHIVE, [PACKAGE], mapColors());
    const before = style.layers.find((l) => l.id === 'place-label') as {minzoom?: number; maxzoom?: number};
    const zooms = new Map<string, {minzoom?: number; maxzoom?: number}>([['place-label', {...before}]]);

    syncGlobalReach({
        getBounds: () => far,
        getLayer: (id: string) => zooms.get(id),
        setLayerZoomRange: (id: string, min: number, max: number) => {
            zooms.set(id, {minzoom: min, maxzoom: max});
        },
    }, [PACKAGE], ['place-label']);

    expect(zooms.get('place-label')?.minzoom).toBe(before.minzoom);
    expect(zooms.get('place-label')?.maxzoom).toBe(MAX_ZOOM);
});
