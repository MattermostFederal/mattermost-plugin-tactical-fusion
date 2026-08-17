import React from 'react';

import LocationHoverHarness from './LocationHoverHarness';
import {serveMapAssets} from './map/asset_fixtures';

import {expect, test} from '../../../playwright/ct-coverage';

/*
 * The hover card, which is the map and nothing else.
 *
 * What is under test here is not the map: `LocationMap.pw.tsx` covers preview
 * mode, the camera and both overlay sources. It is the wiring the card
 * contributes, which had no test at all and is the one place the panel and the
 * card can drift apart:
 *
 * - the local `coord` is preferred over the server's position, so a lat/lon link
 *   draws before the conversion answers and a grid link waits;
 * - `conversion.data` is read only once the status is `ready`, so a failed
 *   request leaves no pin rather than one at 0,0;
 * - the cell is sized through the shared `cellDegrees`, so a grid square is not
 *   squared off differently in the card than on the panel;
 * - `region` reaches the accessible label, which since the Region row was
 *   retired is the only place the country reaches a reader at all.
 */

const LOADING = 'Loading map…';
const NO_POSITION = 'The position for this coordinate is unavailable.';
const TOO_FAR_NORTH = 'This position is too far north for the map.';
const RESET = 'Reset view';

const REGION = 'United States of America (Natural Earth 110m)';

/** Reads MapLibre's own state into the harness's outputs. */
const READ = 'read the map';
const ANSWER = 'answer the conversion';

/*
 * Map creation is gated on a known position, so a card with nothing to point at
 * never builds a map, never fetches the archive and never reaches the "could not
 * be loaded" note that would otherwise mask the one these assert. That is what
 * lets this group run with no routes at all.
 */
test.describe('the card is a glance', () => {
    /*
     * The hover-level mirror of the DTG hover's "shows nothing but the
     * countdown". Every reading, every alternative notation and every copy
     * button is one click away in the panel, and repeating any of them here only
     * makes the glance slower.
     */
    test('is the map and nothing else', async ({mount, page}) => {
        await serveMapAssets(page);

        const component = await mount(<LocationHoverHarness/>);

        await expect(component.locator('table')).toHaveCount(0);
        await expect(component.getByRole('button', {name: /^Copy /})).toHaveCount(0);
        await expect(component.getByRole('link', {name: 'Open larger'})).toHaveCount(0);
        await expect(component.getByRole('button', {name: RESET})).toHaveCount(0);
        await expect(component.getByText(/^z\d/)).toHaveCount(0);
        await expect(component.getByText(REGION)).toHaveCount(0);
    });

    // A grid reference has no position until the server projects it. Saying so
    // is different from saying the position is missing, and the card has to tell
    // the two apart for the same reason the panel does.
    test('a grid reference waits on the server rather than claiming a position', async ({mount}) => {
        const component = await mount(
            <LocationHoverHarness
                format='mgrs'
                canonical='11SLT84636908'
            />);

        await expect(component.getByTestId('map-note')).toHaveText(LOADING);
    });

    /*
     * Nothing may fail the card, and a failure may never leave a zero behind. A
     * pin dropped at 0,0 because a conversion failed is a position, and a wrong
     * one, off the coast of Ghana.
     */
    test('a failed conversion says the position is unavailable rather than drawing a zero', async ({mount}) => {
        const component = await mount(
            <LocationHoverHarness
                format='mgrs'
                canonical='11SLT84636908'
                outcome='fail'
            />);

        await component.getByRole('button', {name: ANSWER}).click();

        await expect(component.getByTestId('map-note')).toHaveText(NO_POSITION);

        await component.getByRole('button', {name: READ}).click();
        await expect(component.getByTestId('pin-features')).toHaveText('-1');
    });

    /*
     * A link the server refused draws no map, and says why.
     *
     * The card branched on `ready` alone, so a hand-edited link whose author
     * text failed the server's gates showed a confident pin here while the panel
     * one click later said "Not a coordinate": two surfaces disagreeing about a
     * link this plugin says it did not issue, on the surface a reader meets
     * first.
     *
     * The refusal is a line rather than `null`, because the framework's tooltip
     * builds its chrome around whatever a Hover returns, so `null` is an empty
     * bordered box rather than no card.
     */
    test('a link the server refuses says so instead of drawing a pin', async ({mount}) => {
        const component = await mount(
            <LocationHoverHarness
                format='mgrs'
                canonical='11SLT84636908'
                outcome='reject'
            />);

        await component.getByRole('button', {name: ANSWER}).click();

        await expect(component.getByText('Not a coordinate')).toBeVisible();
        await expect(component.getByTestId('map-note')).toHaveCount(0);
    });

    /*
     * Web Mercator caps at 85.05 while the grammars validate latitude to 90, so
     * this is a decoratable token whose position the projection cannot hold.
     * Clamping it would put the pin 550 km from what the author wrote, so the
     * card omits the map and says which end it ran off.
     */
    test('omits the map past the Mercator limit', async ({mount}) => {
        const component = await mount(
            <LocationHoverHarness canonical='89.9000,12.0000'/>);

        await expect(component.getByTestId('map-note')).toHaveText(TOO_FAR_NORTH);
    });
});

test.describe('with a basemap it can verify', () => {
    /*
     * These need a real WebGL2 context, which headless Chromium supplies through
     * SwiftShader. Asserted once, by name, because without it every test below
     * fails as a timeout that says nothing about the cause.
     */
    test.beforeAll(async ({browser}) => {
        const page = await browser.newPage();
        const ok = await page.evaluate(() => Boolean(document.createElement('canvas').getContext('webgl2')));
        await page.close();

        expect(ok, 'these tests need WebGL2 in the component-test browser').toBe(true);
    });

    /*
     * The `coord ? coord.lat.decimal : position?.lat` branch, from the side that
     * needs no server. A lat/lon token carries its own position, so pointing at
     * one shows the place immediately; going to the server for it first would
     * make a glance cost a round trip for a number already in the link.
     *
     * The conversion is deliberately left in the air for the whole test, so a
     * pin drawn here can only have come from the token.
     */
    test('draws a textual coordinate before the conversion answers', async ({mount, page}) => {
        await serveMapAssets(page);

        const component = await mount(<LocationHoverHarness/>);

        await expect.poll(async () => {
            await component.getByRole('button', {name: READ}).click();

            return component.getByTestId('pin-features').textContent();
        }, {message: 'pin drawn from the token alone'}).toBe('1');

        await expect(component.getByTestId('camera')).toHaveText('34.056,-118.250');
    });

    // The mirror image: a grid token yields a resolution and nothing else, so
    // the card has nothing to draw until the projection comes back.
    test('draws a grid reference only once the server has projected it', async ({mount, page}) => {
        await serveMapAssets(page);

        const component = await mount(
            <LocationHoverHarness
                format='mgrs'
                canonical='11SLT84636908'
            />);

        await component.getByRole('button', {name: READ}).click();
        await expect(component.getByTestId('pin-features')).toHaveText('-1');

        await component.getByRole('button', {name: ANSWER}).click();

        await expect.poll(async () => {
            await component.getByRole('button', {name: READ}).click();

            return component.getByTestId('pin-features').textContent();
        }, {message: 'pin drawn once the conversion landed'}).toBe('1');

        await expect(component.getByTestId('cell-features')).toHaveText('1');
        await expect(component.getByTestId('camera')).toHaveText('34.056,-118.250');
    });

    /*
     * The size of the rectangle is the whole of what it states, and it is the
     * one thing a feature count cannot see. Both of these coordinates are the
     * same place written two ways, so a card that sized its cell from anything
     * other than the token would draw them identically.
     *
     * 11SLT84636908 is a 10 m square, which is 10/111320 of a degree; the
     * four-decimal lat/lon carries 0.0001. Told apart to six decimal places,
     * which is far tighter than the difference between the two and far looser
     * than floating point.
     *
     * The second coordinate arrives as a prop change rather than a second mount,
     * because that is what a reader moving a pointer from one link to the next
     * does, and because one map is created and moved rather than rebuilt.
     */
    test('sizes the cell from the token, like the panel does', async ({mount, page}) => {
        await serveMapAssets(page);

        const component = await mount(
            <LocationHoverHarness
                format='mgrs'
                canonical='11SLT84636908'
            />);

        await component.getByRole('button', {name: ANSWER}).click();
        await expect.poll(async () => {
            await component.getByRole('button', {name: READ}).click();

            return Number(await component.getByTestId('cell-span').textContent());
        }, {message: 'a 10 m square'}).toBeCloseTo(10 / 111320, 6);

        await component.update(<LocationHoverHarness/>);

        await expect.poll(async () => {
            await component.getByRole('button', {name: READ}).click();

            return Number(await component.getByTestId('cell-span').textContent());
        }, {message: 'four decimal places of a degree'}).toBeCloseTo(0.0001, 6);
    });

    /*
     * The Region row was retired, so this label is the only place the country
     * reaches a reader on any surface. The card computes its own position for a
     * lat/lon token but can never compute a country, so the label is the one
     * thing here that still has to wait for the server.
     */
    test('names the region in the accessible label once the conversion lands', async ({mount, page}) => {
        await serveMapAssets(page);

        const component = await mount(<LocationHoverHarness/>);

        await expect(component.getByText('World map with the position marked.')).toBeAttached();

        await component.getByRole('button', {name: ANSWER}).click();

        await expect(
            component.getByText(`World map. The marked position is in ${REGION}.`),
        ).toBeAttached();
    });
});
