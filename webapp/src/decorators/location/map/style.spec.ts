import {expect, test} from '@playwright/test';

import type {Archive} from './basemap';
import {
    _assetUrlForTesting as assetUrl, buildStyle, cellFeature, emptyCollection, mapColors, palette, pointFeature,
} from './maplibre';
import {cellBounds, DATA_MAX_ZOOM} from './span';

/*
 * The style object is where the air-gap promise lives, so it is asserted rather
 * than described. This is the webapp counterpart of the Go page's
 * TestRenderPageCSPGainsNoImgSrcAndNoFontSrc.
 */

const ARCHIVE: Archive = {
    url: '/plugins/x/public/map/world.pmtiles?v=1',
    minZoom: 0,
    maxZoom: DATA_MAX_ZOOM,
};

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
    const urls = urlsIn(buildStyle(ARCHIVE, mapColors()));

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
    expect(JSON.stringify(buildStyle(ARCHIVE, mapColors()))).not.toContain('sprite');
});

test('the overlay sources stay inlined, and only the basemap is tiled', () => {
    const style = buildStyle(ARCHIVE, mapColors());

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
    const style = buildStyle(ARCHIVE, mapColors());

    for (const layer of style.layers) {
        expect(JSON.stringify((layer as {filter?: unknown}).filter ?? null)).not.toContain('layer');
    }
});

test('every basemap layer names a source-layer', () => {
    const style = buildStyle(ARCHIVE, mapColors());

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
    const style = buildStyle(ARCHIVE, mapColors());
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
    const style = buildStyle(ARCHIVE, mapColors());

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
    const style = buildStyle(ARCHIVE, mapColors());
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
    const style = buildStyle(ARCHIVE, mapColors());
    const byId = (id: string) => style.layers.find((l) => l.id === id) as {minzoom?: number};

    expect(byId('airport-label').minzoom).toBeGreaterThanOrEqual(byId('airport-dot').minzoom!);
});

// Countries name the view while it spans one and hand over to towns past that.
// The maxzoom is what ends the country label; the ordering below is checked by
// 'label layers are ordered least wanted first', which states the real rule.
test('country labels hand over to place labels', () => {
    const style = buildStyle(ARCHIVE, mapColors());
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
