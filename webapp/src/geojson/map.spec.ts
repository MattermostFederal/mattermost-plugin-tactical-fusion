import {expect, test} from '@playwright/test';

import {mapLabel, markersFor, shapesFor} from './GeoJsonMap';
import type {GeoJsonFeature, GeoJsonPart} from './types';

function feature(over: Partial<GeoJsonFeature>): GeoJsonFeature {
    return {name: 'F', kind: 'Point', note: '', format: '', value: '', length: '', area: '', color: '', width: '', lineOpacity: '', fillOpacity: '', markerSize: '', parts: [], properties: [], ...over};
}

function part(kind: GeoJsonPart['kind'], rings: Array<Array<[string, string]>>): GeoJsonPart {
    return {
        kind,
        rings: rings.map((ring) => ring.map(([lon, lat]) => ({lon, lat, alt: ''}))),
        ringCounts: [],
    };
}

test('a point becomes a marker and never a shape', () => {
    const features = [feature({kind: 'Point', parts: [part('Point', [[['-118.25', '34.05']]])]})];

    expect(markersFor(features)).toHaveLength(1);
    expect(shapesFor(features)).toHaveLength(0);
});

// The geometry source is drawn by a fill layer and a line layer, neither of
// which renders a point, so a point routed through shapes draws nothing at all.
test('a line and a polygon become shapes and never markers', () => {
    const features = [
        feature({kind: 'LineString', parts: [part('LineString', [[['0', '0'], ['1', '1']]])]}),
        feature({kind: 'Polygon', parts: [part('Polygon', [[['0', '0'], ['1', '0'], ['1', '1'], ['0', '0']]])]}),
    ];

    expect(markersFor(features)).toHaveLength(0);
    expect(shapesFor(features)).toHaveLength(2);
});

test('a polygon is closed and a line is not', () => {
    const [line, area] = shapesFor([
        feature({kind: 'LineString', parts: [part('LineString', [[['0', '0'], ['1', '1']]])]}),
        feature({kind: 'Polygon', parts: [part('Polygon', [[['0', '0'], ['1', '0'], ['1', '1'], ['0', '0']]])]}),
    ]);

    expect(line.closed).toBe(false);
    expect(area.closed).toBe(true);
});

test('a polygon keeps every ring, so a hole stays a hole', () => {
    const rings = [
        [['0', '0'], ['4', '0'], ['4', '4'], ['0', '0']] as Array<[string, string]>,
        [['1', '1'], ['2', '1'], ['2', '2'], ['1', '1']] as Array<[string, string]>,
    ];

    const [shape] = shapesFor([feature({kind: 'Polygon', parts: [part('Polygon', rings)]})]);

    expect(shape.rings).toHaveLength(2);
});

// One feature reaching both channels is the case a collection exists for, and
// it is the only one where a single feature is drawn twice.
test('a geometry collection splits across both channels by each part\'s own kind', () => {
    const features = [feature({
        kind: 'GeometryCollection',
        parts: [
            part('Point', [[['1', '2']]]),
            part('Polygon', [[['0', '0'], ['1', '0'], ['1', '1'], ['0', '0']]]),
        ],
    })];

    expect(markersFor(features)).toHaveLength(1);
    expect(shapesFor(features)).toHaveLength(1);
});

// The server notes a feature it will not stand behind. Drawing one anyway makes
// the card say "not drawn" over something drawn.
test('a feature the server noted is drawn in neither channel', () => {
    const features = [feature({
        kind: 'Point',
        note: 'A coordinate in this feature is not one this build will stand behind.',
        parts: [part('Point', [[['1', '2']]])],
    })];

    expect(markersFor(features)).toHaveLength(0);
    expect(shapesFor(features)).toHaveLength(0);
});

/*
 * Number('') is 0, and 0 is finite, so a finiteness test alone admits a
 * positionless coordinate and pins the Gulf of Guinea. That is the guessed
 * position this map says it never draws.
 */
test('an empty lexeme is not a position', () => {
    const features = [feature({kind: 'Point', parts: [part('Point', [[['', '']]])]})];

    expect(markersFor(features)).toHaveLength(0);
});

// The server accepts any latitude to 90 and Web Mercator stops near 85.05.
test('a latitude past the Mercator limit is not drawn', () => {
    const features = [feature({kind: 'Point', parts: [part('Point', [[['0', '89.9']]])]})];

    expect(markersFor(features)).toHaveLength(0);
});

test('a ring left with fewer than two usable points is dropped', () => {
    const features = [feature({
        kind: 'LineString',
        parts: [part('LineString', [[['0', '0'], ['', '']]])],
    })];

    expect(shapesFor(features)).toHaveLength(0);
});

test('the label says what is on the map, in both channels', () => {
    expect(mapLabel(3, 2)).toBe('3 marked positions and 2 drawn shapes');
    expect(mapLabel(1, 0)).toBe('1 marked position');
    expect(mapLabel(0, 1)).toBe('1 drawn shape');
    expect(mapLabel(0, 0)).toBe('');
});

/*
 * A feature's stated color reaches the map; anything else falls back.
 *
 * The server has already refused everything that is not a hex triple, and
 * `styleOf` refuses it again before it becomes paint. This checks the handoff
 * between the two, which is the part neither of those tests covers.
 */
test('a stated color is carried onto the shape', () => {
    const [shape] = shapesFor([feature({
        kind: 'Polygon',
        color: '#ff8800',
        parts: [part('Polygon', [[['0', '0'], ['1', '0'], ['1', '1'], ['0', '0']]])],
    })]);

    expect(shape.color).toBe('#ff8800');
});

test('a feature that states no color carries none, so the theme decides', () => {
    const [shape] = shapesFor([feature({
        kind: 'Polygon',
        parts: [part('Polygon', [[['0', '0'], ['1', '0'], ['1', '1'], ['0', '0']]])],
    })]);

    expect(shape.color).toBeUndefined();
});

test('a stated color reaches a point marker too', () => {
    const [marker] = markersFor([feature({
        kind: 'Point',
        color: '#00ff00',
        parts: [part('Point', [[['1', '2']]])],
    })]);

    expect(marker.color).toBe('#00ff00');
});

/*
 * A MultiPolygon is several polygons, not one polygon with holes.
 *
 * The server carries the member boundaries in `ringCounts`; flattening them
 * into one shape makes ring 0 the exterior and every later ring a hole, so a
 * two-island MultiPolygon was drawn as island one punched through.
 */
test('a multi polygon becomes one shape per member, not one shape with holes', () => {
    const square = (offset: number): Array<[string, string]> => [
        [String(offset), '0'], [String(offset + 1), '0'],
        [String(offset + 1), '1'], [String(offset), '0'],
    ];

    const shapes = shapesFor([feature({
        kind: 'MultiPolygon',
        parts: [{
            kind: 'MultiPolygon',
            rings: [square(0), square(10)].map((ring) => ring.map(([lon, lat]) => ({lon, lat, alt: ''}))),
            ringCounts: [1, 1],
        }],
    })]);

    expect(shapes).toHaveLength(2);
    expect(shapes[0].rings).toHaveLength(1);
    expect(shapes[1].rings).toHaveLength(1);
});

test('a polygon with a hole stays one shape of two rings', () => {
    const outer = [['0', '0'], ['4', '0'], ['4', '4'], ['0', '0']] as Array<[string, string]>;
    const hole = [['1', '1'], ['2', '1'], ['2', '2'], ['1', '1']] as Array<[string, string]>;

    const shapes = shapesFor([feature({
        kind: 'Polygon',
        parts: [part('Polygon', [outer, hole])],
    })]);

    expect(shapes).toHaveLength(1);
    expect(shapes[0].rings).toHaveLength(2);
});

// If ring 0 is the one that loses its positions, a hole would be promoted to
// the outer boundary and drawn as the shape.
test('a member whose exterior is unusable is dropped whole', () => {
    const unusable = [['0', '95'], ['1', '95'], ['1', '96'], ['0', '95']] as Array<[string, string]>;
    const hole = [['1', '1'], ['2', '1'], ['2', '2'], ['1', '1']] as Array<[string, string]>;

    expect(shapesFor([feature({
        kind: 'Polygon',
        parts: [part('Polygon', [unusable, hole])],
    })])).toHaveLength(0);
});

// The gate the comments claim: a color that is not a hex triple never reaches
// the map, on the marker path as well as the shape path.
test('a color that is not a hex triple falls back on both channels', () => {
    const hostile = 'url(https://attacker.example/px)';

    const [marker] = markersFor([feature({
        kind: 'Point', color: hostile, parts: [part('Point', [[['1', '2']]])],
    })]);
    expect(marker.color).not.toContain('attacker.example');

    const [shape] = shapesFor([feature({
        kind: 'Polygon',
        color: hostile,
        parts: [part('Polygon', [[['0', '0'], ['1', '0'], ['1', '1'], ['0', '0']]])],
    })]);
    expect(shape.color).toBeUndefined();
});
