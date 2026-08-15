import {expect, test} from '@playwright/test';

import type {Basemap} from './basemap';
import {
    _assetUrlForTesting as assetUrl, buildStyle, cellFeature, emptyCollection, mapColors, pointFeature,
} from './maplibre';
import {cellBounds} from './span';

/*
 * The style object is where the air-gap promise lives, so it is asserted rather
 * than described. This is the webapp counterpart of the Go page's
 * TestRenderPageCSPGainsNoImgSrcAndNoFontSrc.
 */

const BASEMAP: Basemap = {type: 'FeatureCollection', features: []};

test('the style fetches nothing from the network', () => {
    const style = JSON.stringify(buildStyle(BASEMAP, mapColors()));

    // glyphs and sprite are the only two style fields MapLibre resolves as URLs,
    // and an air-gapped install has nowhere for them to point.
    expect(style).not.toContain('glyphs');
    expect(style).not.toContain('sprite');
    expect(style).not.toMatch(/https?:/);
});

test('the style has no symbol layer, which is what would need glyphs', () => {
    const style = buildStyle(BASEMAP, mapColors());

    expect(style.layers.some((layer) => layer.type === 'symbol')).toBe(false);
});

test('the basemap is the only geometry source, and it is inlined', () => {
    const style = buildStyle(BASEMAP, mapColors());

    for (const source of Object.values(style.sources)) {
        expect(source.type).toBe('geojson');
        expect(typeof (source as {data?: unknown}).data).toBe('object');
    }
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
