import {expect, test} from '@playwright/test';

import {GEOJSON_PROPS_KEY, GEOJSON_PROPS_VERSION, fromProps, isLinkable, ringCount, solePosition, vertexCount} from './types';

function props(blob: unknown): unknown {
    return {[GEOJSON_PROPS_KEY]: blob};
}

function wellFormed(over: Record<string, unknown> = {}): unknown {
    return props({
        version: GEOJSON_PROPS_VERSION,
        source: 'fence',
        features: [],
        ...over,
    });
}

/*
 * The reader meets a props blob it did not write. A post can be edited, a
 * bundle can be older than the server, and a props key can be forged, so every
 * branch here is a real arrival rather than a hypothetical.
 */
test.describe('refusing a blob it cannot honor', () => {
    for (const [name, value] of [
        ['undefined', undefined],
        ['null', null],
        ['an array', []],
        ['a string', 'nope'],
        ['a number', 7],
    ] as Array<[string, unknown]>) {
        test(`${name} is refused`, () => {
            expect(fromProps(value)).toBeNull();
        });
    }

    test('a blob with no version is refused', () => {
        expect(fromProps(props({source: 'fence', features: []}))).toBeNull();
    });

    // The case an older bundle meets against a newer server. Guessing at a
    // shape it has never seen is worse than falling back to the post's text.
    test('a version this build does not know is refused', () => {
        expect(fromProps(wellFormed({version: 99}))).toBeNull();
    });

    // The failure the sync guard names: a drift in the source kinds falls every
    // card back to the post's own text.
    test('a source naming neither fence nor file is refused', () => {
        expect(fromProps(wellFormed({source: 'telepathy'}))).toBeNull();
        expect(fromProps(wellFormed({source: ''}))).toBeNull();
    });
});

test.describe('degrading rather than throwing', () => {
    test('a features member that is not an array reads as no features', () => {
        const payload = fromProps(wellFormed({features: 'nope'}));
        expect(payload).not.toBeNull();
        expect(payload?.features).toEqual([]);
    });

    test('non-object entries inside features are skipped', () => {
        const payload = fromProps(wellFormed({features: [7, null, 'x', {name: 'kept'}]}));
        expect(payload?.features).toHaveLength(1);
        expect(payload?.features[0].name).toBe('kept');
    });

    test('a kind this build does not know reads as no geometry', () => {
        const payload = fromProps(wellFormed({features: [{kind: 'Hypercube'}]}));
        expect(payload?.features[0].kind).toBe('none');
    });

    test('parts, rings and properties that are not arrays read as empty', () => {
        const payload = fromProps(wellFormed({
            features: [{name: 'a', parts: 'no', properties: 7}],
        }));
        expect(payload?.features[0].parts).toEqual([]);
        expect(payload?.features[0].properties).toEqual([]);
    });

    test('non-finite ring counts are dropped', () => {
        const payload = fromProps(wellFormed({
            features: [{kind: 'MultiPolygon', parts: [{kind: 'MultiPolygon', rings: [], ring_counts: [1, 'x', null, 2]}]}],
        }));
        expect(payload?.features[0].parts[0].ringCounts).toEqual([1, 2]);
    });

    test('missing counts read as zero rather than NaN', () => {
        const payload = fromProps(wellFormed({counts: 'nope'}));
        expect(payload?.counts.features).toBe(0);
        expect(Number.isNaN(payload?.counts.points)).toBe(false);
    });

    test('a missing note and color read as empty, not undefined', () => {
        const payload = fromProps(wellFormed({features: [{name: 'a'}]}));
        expect(payload?.note).toBe('');
        expect(payload?.features[0].color).toBe('');
        expect(payload?.unplaceable).toBe(false);
    });
});

test.describe('the helpers a surface branches on', () => {
    test('isLinkable needs both halves of the pair', () => {
        const one = fromProps(wellFormed({features: [{format: 'dd', value: ''}]}));
        const both = fromProps(wellFormed({features: [{format: 'dd', value: '1,2'}]}));

        expect(isLinkable(one!.features[0])).toBe(false);
        expect(isLinkable(both!.features[0])).toBe(true);
    });

    test('solePosition is only a lone point', () => {
        const point = fromProps(wellFormed({
            features: [{kind: 'Point', parts: [{kind: 'Point', rings: [[{lon: '1', lat: '2'}]]}]}],
        }));
        expect(solePosition(point!.features[0])?.lon).toBe('1');

        const line = fromProps(wellFormed({
            features: [{kind: 'LineString', parts: [{kind: 'LineString', rings: [[{lon: '1', lat: '2'}, {lon: '3', lat: '4'}]]}]}],
        }));
        expect(solePosition(line!.features[0])).toBeNull();
    });

    test('vertexCount and ringCount total every ring of every part', () => {
        const payload = fromProps(wellFormed({
            features: [{
                kind: 'Polygon',
                parts: [{kind: 'Polygon',
rings: [
                    [{lon: '0', lat: '0'}, {lon: '1', lat: '1'}],
                    [{lon: '2', lat: '2'}],
                ]}],
            }],
        }));

        expect(vertexCount(payload!.features[0])).toBe(3);
        expect(ringCount(payload!.features[0])).toBe(2);
    });
});
