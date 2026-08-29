import React from 'react';

import GeoJsonPanelHarness from './GeoJsonPanelHarness';

import {expect, test} from '../../playwright/ct-coverage';
import {savedGeoJsonSections, stubPreferencesRoute} from '../preferences/stub_route';

test('the card opens the sidebar on the document', async ({mount, page}) => {
    await stubPreferencesRoute(page);

    const component = await mount(
        <GeoJsonPanelHarness features={[{name: 'Depot'}, {name: 'Route'}]}/>,
    );

    await expect(component.getByTestId('geojson-panel-features')).toHaveCount(0);

    await component.getByRole('button', {name: 'Open details'}).click();

    await expect(component.getByTestId('geojson-panel-features')).toBeVisible();
    await expect(component.getByTestId('rhs')).toContainText('Depot');
    await expect(component.getByTestId('rhs')).toContainText('Route');
});

test('the sidebar title names the document by its feature count', async ({mount, page}) => {
    await stubPreferencesRoute(page);

    const component = await mount(
        <GeoJsonPanelHarness features={[{name: 'A'}, {name: 'B'}]}/>,
    );
    await component.getByRole('button', {name: 'Open details'}).click();

    await expect(component.getByTestId('rhs-title')).toContainText('GeoJSON: 2 features');
});

test('one feature is singular in the title', async ({mount, page}) => {
    await stubPreferencesRoute(page);

    const component = await mount(<GeoJsonPanelHarness features={[{name: 'A'}]}/>);
    await component.getByRole('button', {name: 'Open details'}).click();

    await expect(component.getByTestId('rhs-title')).toContainText('GeoJSON: 1 feature');
});

/*
 * A lone point whose position the location grammar will stand behind is handed
 * to the coordinate tools. Everything else is text: a polygon has no one
 * position, and a coarse coordinate is one the grammar refuses.
 */
test.describe('a point row', () => {
    test('links into the coordinate tools when the server gave it an identity', async ({mount, page}) => {
        await stubPreferencesRoute(page);

        const component = await mount(
            <GeoJsonPanelHarness features={[{name: 'Depot', format: 'dd', value: '34.056100,-118.250000'}]}/>,
        );
        await component.getByRole('button', {name: 'Open details'}).click();

        const link = component.getByTestId('rhs').getByRole('link', {name: '34.056100, -118.250000'});
        await expect(link).toHaveAttribute('href', /\/decorate\/location\?/);
        await expect(link).toHaveAttribute('href', /f=dd/);
    });

    // A post stamped before the pair existed carries neither key. It must read
    // as text rather than as a link that would land nowhere.
    test('is plain text when the server gave it none', async ({mount, page}) => {
        await stubPreferencesRoute(page);

        const component = await mount(
            <GeoJsonPanelHarness features={[{name: 'Depot'}]}/>,
        );
        await component.getByRole('button', {name: 'Open details'}).click();

        await expect(component.getByTestId('rhs')).toContainText('34.056100, -118.250000');
        await expect(component.getByTestId('rhs').getByRole('link', {name: /34\.056100/})).toHaveCount(0);
    });

    test('a polygon is never linked, because it has no one position', async ({mount, page}) => {
        await stubPreferencesRoute(page);

        const component = await mount(
            <GeoJsonPanelHarness
                features={[{
                    name: 'Area',
                    kind: 'Polygon',
                    parts: [{
                        kind: 'Polygon',
                        rings: [[
                            {lon: '0', lat: '0', alt: ''},
                            {lon: '1', lat: '0', alt: ''},
                            {lon: '1', lat: '1', alt: ''},
                            {lon: '0', lat: '0', alt: ''},
                        ]],
                        ringCounts: [],
                    }],
                }]}
            />,
        );
        await component.getByRole('button', {name: 'Open details'}).click();

        await expect(component.getByTestId('rhs')).toContainText('1 ring, 4 points');

        // Scoped to the feature list: the panel footer carries a Documentation
        // link, which is not a position and must not be counted as one.
        await expect(component.getByTestId('geojson-panel-features').getByRole('link')).toHaveCount(0);
    });
});

test('the panel shows each feature its properties', async ({mount, page}) => {
    await stubPreferencesRoute(page);

    const component = await mount(
        <GeoJsonPanelHarness
            features={[{name: 'Depot', properties: [{key: 'status', value: 'active'}]}]}
        />,
    );
    await component.getByRole('button', {name: 'Open details'}).click();

    await expect(component.getByTestId('rhs')).toContainText('status');
    await expect(component.getByTestId('rhs')).toContainText('active');
});

/*
 * The stored list is HIDDEN sections, so a reader who never chose sees
 * everything and a section added later appears for everybody.
 */
test.describe('hidden sections', () => {
    test('a hidden section is left out and the rest stay', async ({mount, page}) => {
        await stubPreferencesRoute(page, {storedGeoJsonSections: ['properties']});

        const component = await mount(
            <GeoJsonPanelHarness
                features={[{name: 'Depot', properties: [{key: 'status', value: 'active'}]}]}
            />,
        );
        await component.getByRole('button', {name: 'Open details'}).click();

        await expect(component.getByTestId('geojson-panel-features')).toBeVisible();
        await expect(component.getByTestId('rhs')).toContainText('Depot');
        await expect(component.getByTestId('rhs')).not.toContainText('status');
    });

    test('hiding the features section leaves the summary', async ({mount, page}) => {
        await stubPreferencesRoute(page, {storedGeoJsonSections: ['features']});

        const component = await mount(<GeoJsonPanelHarness features={[{name: 'Depot'}]}/>);
        await component.getByRole('button', {name: 'Open details'}).click();

        await expect(component.getByTestId('geojson-panel-summary')).toBeVisible();
        await expect(component.getByTestId('geojson-panel-features')).toHaveCount(0);
    });

    // The GeoJSON list is its own, so hiding a section here must not read the
    // Cursor on Target one. A blob storing only CoT sections hides nothing here.
    test('the Cursor on Target list is not read for this panel', async ({mount, page}) => {
        await stubPreferencesRoute(page, {storedHiddenSections: ['payload', 'source']});

        const component = await mount(<GeoJsonPanelHarness features={[{name: 'Depot'}]}/>);
        await component.getByRole('button', {name: 'Open details'}).click();

        await expect(component.getByTestId('geojson-panel-features')).toBeVisible();
        await expect(component.getByTestId('geojson-panel-source')).toBeAttached();
    });
});

test.describe('the editor', () => {
    test('opens over the panel and renames the header', async ({mount, page}) => {
        await stubPreferencesRoute(page);

        const component = await mount(<GeoJsonPanelHarness features={[{name: 'Depot'}]}/>);
        await component.getByRole('button', {name: 'Open details'}).click();

        await component.getByRole('button', {name: 'Customize your view'}).click();

        await expect(component.getByTestId('rhs-title')).toContainText('Customize your view');
        await expect(component.getByTestId('geojson-panel-features')).toHaveCount(0);
    });

    test('saves what the reader hid, into its own key', async ({mount, page}) => {
        const calls = await stubPreferencesRoute(page);

        const component = await mount(<GeoJsonPanelHarness features={[{name: 'Depot'}]}/>);
        await component.getByRole('button', {name: 'Open details'}).click();
        await component.getByRole('button', {name: 'Customize your view'}).click();

        await component.getByRole('checkbox', {name: 'Feature properties'}).uncheck();
        await component.getByRole('button', {name: 'Save'}).click();

        await expect.poll(() => savedGeoJsonSections(calls)).toEqual(['properties']);
    });

    // Clicking a different document while the editor is open must put the
    // reader back on the panel, not leave them editing over something else.
    test('closes when the reader opens a different document', async ({mount, page}) => {
        await stubPreferencesRoute(page);

        const component = await mount(
            <GeoJsonPanelHarness
                features={[{name: 'Depot'}]}
                second={[{name: 'Other'}, {name: 'Second'}]}
            />,
        );
        await component.getByRole('button', {name: 'Open details'}).first().click();
        await component.getByRole('button', {name: 'Customize your view'}).click();
        await expect(component.getByTestId('rhs-title')).toContainText('Customize your view');

        await component.getByTestId('second-card').getByRole('button', {name: 'Open details'}).click();

        await expect(component.getByTestId('rhs-title')).toContainText('GeoJSON: 2 features');
        await expect(component.getByTestId('rhs')).toContainText('Other');
    });
});

/*
 * The two panels' "As posted" rows have to be the same row.
 *
 * This one shipped wrapped in the group-heading style, so it rendered as
 * uppercase micro-text beside a Cursor on Target panel whose identical section
 * rendered as a bordered control. That is the defect cot.md already records
 * under "A disclosure has to look like a control, not a heading", reintroduced
 * one directory over.
 */
test.describe('the document as posted', () => {
    test('is a control rather than a heading, like the Cursor on Target one', async ({mount, page}) => {
        await stubPreferencesRoute(page);

        const component = await mount(<GeoJsonPanelHarness/>);
        await component.getByRole('button', {name: 'Open details'}).click();

        const summary = component.getByTestId('rhs').locator('summary').filter({hasText: 'As posted'});

        await expect(summary).toHaveCSS('text-transform', 'none');
        await expect(summary).toHaveCSS('list-style-type', 'none');
        await expect(summary.getByTestId('disclosure-chevron')).toHaveAttribute('data-state', 'closed');
    });

    test('can be copied without collapsing the disclosure', async ({mount, page}) => {
        await stubPreferencesRoute(page);

        const component = await mount(<GeoJsonPanelHarness src='{"type":"Point","coordinates":[1,2]}'/>);
        await component.getByRole('button', {name: 'Open details'}).click();

        const rhs = component.getByTestId('rhs');
        await rhs.getByText('As posted').click();

        const pane = rhs.getByRole('region', {name: 'The document as it was posted'});
        await expect(pane).toBeVisible();

        await rhs.getByRole('button', {name: 'Copy the document as posted'}).click();

        await expect(pane).toBeVisible();
        await expect(rhs).toContainText('Copy the document as posted: copied');
    });
});
