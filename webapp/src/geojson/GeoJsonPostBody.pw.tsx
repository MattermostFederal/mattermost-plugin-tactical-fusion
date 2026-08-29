import React from 'react';

import GeoJsonPostBodyHarness from './GeoJsonPostBodyHarness';

import {expect, test} from '../../playwright/ct-coverage';
import {stubFeaturesRoute} from '../features/stub_route';

test('renders the card for a well formed post', async ({mount}) => {
    const component = await mount(
        <GeoJsonPostBodyHarness features={[{name: 'Depot', kind: 'Point'}]}/>,
    );

    await expect(component.getByTestId('geojson-card')).toBeVisible();
    await expect(component.getByTestId('geojson-heading')).toContainText('1 feature');
    await expect(component).toContainText('Depot');
    await expect(component).toContainText('34.0561, -118.25');
});

test('counts and the geometry mix read as a sentence', async ({mount}) => {
    const component = await mount(
        <GeoJsonPostBodyHarness
            features={[{name: 'A'}, {name: 'B'}, {name: 'C'}]}
            counts={{features: 3, points: 1, lines: 1, polygons: 1}}
        />,
    );

    await expect(component.getByTestId('geojson-heading')).toContainText('3 features');
    await expect(component.getByTestId('geojson-summary')).
        toContainText('1 point, 1 line and 1 polygon');
});

test('keeps the text the author wrote around the document, in reading order', async ({mount}) => {
    const component = await mount(
        <GeoJsonPostBodyHarness
            lead='overlay for tonight'
            trail='from ALPHA'
        />,
    );

    await expect(component).toContainText('overlay for tonight');
    await expect(component).toContainText('from ALPHA');
    await expect(component.getByTestId('geojson-card')).toBeVisible();
});

test('shows each feature the properties it carries', async ({mount}) => {
    const component = await mount(
        <GeoJsonPostBodyHarness
            features={[{
                name: 'Depot',
                properties: [
                    {key: 'status', value: 'active'},
                    {key: 'capacity', value: '240'},
                ],
            }]}
        />,
    );

    await expect(component).toContainText('status');
    await expect(component).toContainText('active');
    await expect(component).toContainText('capacity');
    await expect(component).toContainText('240');
});

/*
 * Property keys and values are author text. React escapes them as text nodes,
 * and this is the test that says so: nothing on this path may reach the DOM as
 * markup.
 */
test('renders author markup in a property as text, never as markup', async ({mount}) => {
    const component = await mount(
        <GeoJsonPostBodyHarness
            features={[{
                name: '<img src=x onerror=alert(1)>',
                properties: [{key: '<b>k</b>', value: '<script>alert(1)</script>'}],
            }]}
        />,
    );

    await expect(component).toContainText('<script>alert(1)</script>');
    await expect(component).toContainText('<img src=x onerror=alert(1)>');
    await expect(component.locator('script')).toHaveCount(0);
    await expect(component.locator('img')).toHaveCount(0);
    await expect(component.locator('b')).toHaveCount(0);
});

test('states the geometry of a feature that is not a lone point', async ({mount}) => {
    const component = await mount(
        <GeoJsonPostBodyHarness
            features={[{
                name: 'Area',
                kind: 'Polygon',
                parts: [{
                    kind: 'Polygon',
                    rings: [
                        [{lon: '0', lat: '0', alt: ''}, {lon: '1', lat: '0', alt: ''}, {lon: '1', lat: '1', alt: ''}, {lon: '0', lat: '0', alt: ''}],
                        [{lon: '0.2', lat: '0.2', alt: ''}, {lon: '0.3', lat: '0.2', alt: ''}, {lon: '0.3', lat: '0.3', alt: ''}, {lon: '0.2', lat: '0.2', alt: ''}],
                    ],
                    ringCounts: [],
                }],
            }]}
        />,
    );

    await expect(component).toContainText('2 rings, 8 points');
});

test('names a feature the document gave no geometry', async ({mount}) => {
    const component = await mount(
        <GeoJsonPostBodyHarness
            features={[{
                name: 'Unlocated',
                kind: 'none',
                note: 'The document states no position for this feature.',
                parts: [],
            }]}
        />,
    );

    await expect(component).toContainText('no geometry');
    await expect(component).toContainText('The document states no position for this feature.');
});

test('says why nothing is drawn when the document says so', async ({mount}) => {
    const component = await mount(
        <GeoJsonPostBodyHarness note='The document states a coordinate reference system whose axis order this build cannot confirm, so nothing is drawn.'/>,
    );

    await expect(component.getByTestId('geojson-note')).
        toContainText('axis order this build cannot confirm');
});

/*
 * The lower rung of the props ladder. The card has to SAY it is missing the
 * property bags rather than quietly showing less, since an absent bag is
 * otherwise indistinguishable from a feature that carries none.
 */
test('says so when the server dropped the properties to fit', async ({mount}) => {
    const component = await mount(
        <GeoJsonPostBodyHarness
            propertiesDropped={true}
            features={[{name: 'Depot'}]}
        />,
    );

    await expect(component.getByTestId('geojson-degraded')).toContainText('omitted to fit');

    // The features are still listed by name, which is what hoisting the name
    // out of the properties bag buys.
    await expect(component).toContainText('Depot');
});

/*
 * The card does NOT carry the document as posted; the panel does.
 *
 * A card sits in the channel under everything else somebody wrote, and the raw
 * document is the longest thing the payload holds. "Open details" is one click
 * away and is where a reader who wants the source is going, so the card keeps
 * the channel readable and the panel keeps the source.
 */
test('leaves the document as posted to the panel', async ({mount}) => {
    const component = await mount(
        <GeoJsonPostBodyHarness src='{"type":"Point","coordinates":[1,2]}'/>,
    );

    await expect(component.getByTestId('geojson-card')).toBeVisible();
    await expect(component.getByText('As posted')).toHaveCount(0);
    await expect(component).not.toContainText('"coordinates":[1,2]');
});

/*
 * Post.Type survives an edit and Props may not, so a stamped post can arrive
 * describing something that is no longer there. Every one of these stands the
 * card down rather than describing a document it cannot vouch for.
 */
test.describe('a post the card cannot vouch for', () => {
    test('an edited post falls back to its own text', async ({mount}) => {
        const component = await mount(
            <GeoJsonPostBodyHarness
                message='I fixed the coordinate'
                editAt={1700000000000}
            />,
        );

        await expect(component.getByTestId('geojson-card')).toHaveCount(0);
        await expect(component).toContainText('I fixed the coordinate');
    });

    test('a post whose props were lost falls back to its own text', async ({mount}) => {
        const component = await mount(
            <GeoJsonPostBodyHarness
                features={null}
                message='```geojson\n{"type":"Point"}\n```'
            />,
        );

        await expect(component.getByTestId('geojson-card')).toHaveCount(0);
        await expect(component).toContainText('"type":"Point"');
    });

    test('a version this bundle cannot read falls back to its own text', async ({mount}) => {
        const component = await mount(
            <GeoJsonPostBodyHarness
                version={99}
                message='a document from a newer server'
            />,
        );

        await expect(component.getByTestId('geojson-card')).toHaveCount(0);
        await expect(component).toContainText('a document from a newer server');
    });

    test('a file case whose attachment is gone falls back', async ({mount}) => {
        const component = await mount(
            <GeoJsonPostBodyHarness
                source='file'
                fileId='geofileaaaaaaaaaaaaaaaaaaa'
                fileIds={[]}
                message='the file went away'
            />,
        );

        await expect(component.getByTestId('geojson-card')).toHaveCount(0);
        await expect(component).toContainText('the file went away');
    });

    /*
     * A file-case post has an empty message by construction and has already lost
     * its embeds, so the fallback may never be blank.
     */
    test('a blank file case still names its attachments', async ({mount}) => {
        const component = await mount(
            <GeoJsonPostBodyHarness
                features={null}
                message=''
                fileIds={['geofileaaaaaaaaaaaaaaaaaaa']}
            />,
        );

        await expect(component).toContainText('Attached:');
    });

    test('a post with nothing left says so rather than rendering blank', async ({mount}) => {
        const component = await mount(
            <GeoJsonPostBodyHarness
                features={null}
                message=''
                fileIds={[]}
            />,
        );

        await expect(component).toContainText('no readable content');
    });
});

/*
 * The map under the card.
 *
 * The document-level note is the server saying no feature can be placed at all,
 * which is what a foreign coordinate reference system means. Drawing a map
 * anyway would put features in the wrong country.
 */
test('draws no map when the server says nothing can be placed', async ({mount}) => {
    const component = await mount(
        <GeoJsonPostBodyHarness
            note='The document states a coordinate reference system whose axis order this build cannot confirm, so nothing is drawn.'
            unplaceable={true}
        />,
    );

    await expect(component.getByTestId('geojson-map')).toHaveCount(0);
    await expect(component.getByTestId('geojson-note')).toBeVisible();
});

/*
 * The measurement is the server's, verbatim.
 *
 * Rendered there rather than here so the card and the panel cannot round the
 * same figure into two different answers.
 */
test.describe('measurements', () => {
    test('shows a line its length and a polygon its area', async ({mount}) => {
        const component = await mount(
            <GeoJsonPostBodyHarness
                features={[
                    {name: 'Route', kind: 'LineString', length: '12.3 km'},
                    {name: 'Area', kind: 'Polygon', area: '4.5 km²'},
                ]}
            />,
        );

        await expect(component).toContainText('12.3 km');
        await expect(component).toContainText('4.5 km²');
    });

    test('shows both when a collection carries both', async ({mount}) => {
        const component = await mount(
            <GeoJsonPostBodyHarness
                features={[{name: 'Mixed', kind: 'GeometryCollection', length: '800 m', area: '250 m²'}]}
            />,
        );

        await expect(component.getByTestId('geojson-measure')).toContainText('800 m, 250 m²');
    });

    // A geometry with no such measure, and a feature the server would not stand
    // behind, both carry nothing rather than a zero.
    test('shows nothing for a geometry that has no measure', async ({mount}) => {
        const component = await mount(
            <GeoJsonPostBodyHarness features={[{name: 'Depot', kind: 'Point'}]}/>,
        );

        await expect(component.getByTestId('geojson-measure')).toHaveCount(0);
    });
});

/*
 * "Open larger" on a document, which had no link at all.
 *
 * A GeoJSON overlay is extent-only and has no primary position, so there was no
 * coordinate to address the map page with and the link was never offered. The
 * post id is the address, and the page redraws the whole document.
 */
test.describe('Open larger', () => {
    test('addresses the map page by the post', async ({mount, page}) => {
        await stubFeaturesRoute(page);

        const component = await mount(
            <GeoJsonPostBodyHarness postId='post0000000000000000000000'/>,
        );

        const larger = component.getByRole('link', {name: 'Open larger'});
        await expect(larger).toHaveAttribute('href', /post=post0000000000000000000000/);
        await expect(larger).not.toHaveAttribute('href', /[?&]f=/);
    });

    test('is absent when nothing knows which post this is', async ({mount, page}) => {
        await stubFeaturesRoute(page);

        const component = await mount(<GeoJsonPostBodyHarness postId=''/>);

        await expect(component.getByTestId('geojson-map')).toBeVisible();
        await expect(component.getByText('Open larger')).toHaveCount(0);
    });
});
