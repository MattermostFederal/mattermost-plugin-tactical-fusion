import React from 'react';

import LocationInlineHarness from './LocationInlineHarness';
import {serveMapAssets} from './map/asset_fixtures';

import {expect, test} from '../../../playwright/ct-coverage';
import {stubPreferencesRoute} from '../../preferences/stub_route';

const RESET = 'Reset view';
const ANSWER = 'answer the conversion';

/*
 * Browsers cap live WebGL contexts at roughly sixteen, shared with the panel and
 * any hover card, and a channel of coordinate-only posts is exactly the shape
 * that stresses that. So the map is mounted only while its post is near the
 * screen, and the reserved box is what keeps the channel from jumping as the
 * reader scrolls past it.
 */
test.describe('the viewport gate', () => {
    test('builds no map until the post is near the screen', async ({mount, page}) => {
        await serveMapAssets(page);
        await stubPreferencesRoute(page);

        const component = await mount(<LocationInlineHarness/>);

        await expect(component.getByTestId('maps-built')).toHaveText('0');
    });

    // The conversion is inside the gate as well, so a channel of thirty
    // rendered posts does not issue thirty requests for maps nobody sees.
    test('asks for no conversion until the post is near the screen', async ({mount, page}) => {
        await serveMapAssets(page);
        await stubPreferencesRoute(page);

        const component = await mount(<LocationInlineHarness/>);
        await expect(component.getByTestId('conversions')).toHaveText('0');

        await page.mouse.wheel(0, 4000);

        await expect(component.getByTestId('conversions')).toHaveText('1');
    });

    test('reserves the map height before the map exists', async ({mount, page}) => {
        await serveMapAssets(page);
        await stubPreferencesRoute(page);

        const component = await mount(<LocationInlineHarness/>);

        const empty = await component.getByTestId('location-inline').boundingBox();
        expect(empty?.height ?? 0).toBeGreaterThanOrEqual(200);
    });

    test('builds the map once the post scrolls into range', async ({mount, page}) => {
        await serveMapAssets(page);
        await stubPreferencesRoute(page);

        const component = await mount(<LocationInlineHarness/>);
        await page.mouse.wheel(0, 4000);

        await expect(component.getByTestId('maps-built')).not.toHaveText('0');
    });
});

/*
 * The map a reader can work with, not the hover card's picture. The controls and
 * "Open larger" are what separate the two, and these are what stop the inline
 * map being "simplified" into preview mode later.
 */
test.describe('the map itself', () => {
    test('is interactive rather than a preview', async ({mount, page}) => {
        await serveMapAssets(page);
        await stubPreferencesRoute(page);

        const component = await mount(<LocationInlineHarness inView={true}/>);
        await component.getByRole('button', {name: ANSWER}).click();

        await expect(component.getByRole('button', {name: RESET})).toBeVisible();
    });

    test('offers Open larger, pointing at the map page', async ({mount, page}) => {
        await serveMapAssets(page);
        await stubPreferencesRoute(page);

        const component = await mount(<LocationInlineHarness inView={true}/>);
        await component.getByRole('button', {name: ANSWER}).click();

        await expect(component.getByRole('link', {name: 'Open larger'})).
            toHaveAttribute('href', /\/map\?f=dd&v=/);
    });

    /*
     * zoomForSpan holds the target ground span across the WIDTH, so an uncapped
     * map in a wide centre channel would open several zoom levels deeper than
     * the panel does for the same coordinate.
     */
    test('caps its width rather than taking the whole channel', async ({mount, page}) => {
        await serveMapAssets(page);
        await stubPreferencesRoute(page);

        const component = await mount(<LocationInlineHarness inView={true}/>);

        const box = await component.getByTestId('location-inline').boundingBox();
        expect(box?.width ?? 0).toBeLessThanOrEqual(640);
    });
});

/*
 * A link the server refuses draws nothing rather than a pin. The post's own link
 * is still on screen and the panel is one click away, so a refusal banner under
 * somebody's message would be loud out of proportion to a hand-edited link.
 */
test('a link the server refuses draws nothing', async ({mount, page}) => {
    await serveMapAssets(page);
    await stubPreferencesRoute(page);

    const component = await mount(
        <LocationInlineHarness
            inView={true}
            outcome='reject'
        />,
    );
    await component.getByRole('button', {name: ANSWER}).click();

    await expect(component.getByRole('button', {name: RESET})).toHaveCount(0);
});

/*
 * The preference is read outside the map, so a reader who hid it pays no
 * conversion request per coordinate in the channel to be shown nothing.
 */
test.describe('when the reader has hidden it', () => {
    test('draws no map', async ({mount, page}) => {
        await serveMapAssets(page);
        await stubPreferencesRoute(page, {storedHiddenRows: ['inline']});

        const component = await mount(<LocationInlineHarness inView={true}/>);

        await expect(component.getByTestId('location-inline')).toHaveCount(0);
    });

    test('asks for no conversion', async ({mount, page}) => {
        await serveMapAssets(page);
        await stubPreferencesRoute(page, {storedHiddenRows: ['inline']});

        const component = await mount(<LocationInlineHarness inView={true}/>);

        await expect(component.getByTestId('conversions')).toHaveText('0');
    });

    // Two ids, two maps. Hiding the panel's must not take this one with it.
    test('hiding the panel map leaves this one alone', async ({mount, page}) => {
        await serveMapAssets(page);
        await stubPreferencesRoute(page, {storedHiddenRows: ['map']});

        const component = await mount(<LocationInlineHarness inView={true}/>);

        await expect(component.getByTestId('location-inline')).toBeVisible();
    });
});

/*
 * The admin's switch and the reader's own do the same thing here and are checked
 * in the same place, so the map costs no conversion request per coordinate in
 * the rendered window either way. Mattermost renders on the order of thirty
 * posts at a time, which is what makes that worth being exact about.
 */
test.describe('when the admin has turned the inline map off', () => {
    test('draws no map and asks for no conversion', async ({mount, page}) => {
        await serveMapAssets(page);
        await stubPreferencesRoute(page);

        const component = await mount(
            <LocationInlineHarness
                inView={true}
                features={{mapInline: false}}
            />);

        await expect(component.getByTestId('location-inline')).toHaveCount(0);
        await expect(component.getByTestId('conversions')).toHaveText('0');
    });

    // The three surfaces are independent, so the channel map survives the panel
    // and the page being off.
    test('is unaffected by the other surfaces being off', async ({mount, page}) => {
        await serveMapAssets(page);
        await stubPreferencesRoute(page);

        const component = await mount(
            <LocationInlineHarness
                inView={true}
                features={{mapPanel: false, mapPage: false}}
            />);

        await expect(component.getByTestId('location-inline')).toBeVisible();
    });

    // "Open larger" points at /map, which answers 404 with its own switch off.
    test('offers no Open larger when the map page is off', async ({mount, page}) => {
        await serveMapAssets(page);
        await stubPreferencesRoute(page);

        const component = await mount(
            <LocationInlineHarness
                inView={true}
                features={{mapPage: false}}
            />);

        await expect(component.getByTestId('location-inline')).toBeVisible();
        await expect(component.getByText('Open larger')).toHaveCount(0);
    });
});
