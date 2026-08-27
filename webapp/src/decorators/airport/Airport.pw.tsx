import React from 'react';

import AirportHarness from './AirportHarness';

import {expect, test} from '../../../playwright/ct-coverage';
import {serveMapAssets} from '../location/map/asset_fixtures';

/*
 * The airfield surfaces.
 *
 * Both of them are a pure function of one route's answer, which is what makes
 * this decorator different from the other two: there is nothing computed
 * locally, so every state the reader can meet is a state of that request. All
 * five are asserted on both surfaces, because a panel and a hover disagreeing
 * about the same link is a defect this repository has already had once.
 */

const NAME = 'Indianapolis International Airport';
const NOT_A_CODE = 'Not an airfield code';
const PLACE = 'Indianapolis, IN, US';
const RESET = 'Reset view';
const SECOND_NAME = 'John F Kennedy International Airport';
const SECOND_PLACE = 'New York, NY, US';

test.describe('the panel', () => {
    test('shows the airfield and where it is', async ({mount}) => {
        const panel = await mount(
            <AirportHarness
                surface='panel'
                ident='KIND'
                reply='found'
            />);

        await expect(panel.getByText(NAME)).toBeVisible();

        // Exact, because "IND" is a substring of "Indianapolis" and a loose
        // match would pass on the wrong cell.
        await Promise.all([
            'Indianapolis, IN, US',
            'Large Airport',
            '797 ft',
            'IND',
        ].map((value) => expect(panel.getByText(value, {exact: true})).toBeVisible()));
    });

    /*
     * The panel names the field and links to the position. It does not reprint
     * the readings: the location panel renders them with a map, copy buttons
     * and the reader's own row choices, and a second table here would say the
     * same thing worse and then drift from it.
     */
    test('shows no coordinate readings of its own', async ({mount}) => {
        const panel = await mount(
            <AirportHarness
                surface='panel'
                ident='KIND'
                reply='found'
            />);

        await Promise.all([
            '16S EJ 6047 9661',
            '39.7173° N, 86.2944° W',
            '394302.3N0861739.8W',
            'United States of America (Natural Earth 110m)',
        ].map((reading) => expect(panel.getByText(reading, {exact: true})).toBeHidden()));
    });

    /*
     * The place IS the link, rather than a separate line under the table. The
     * place is where the airfield is, so it is the thing to follow to see where
     * that is, and a line saying "Location" underneath was a second way to say
     * the same thing.
     */
    test('the place swaps the sidebar to the coordinate panel', async ({mount}) => {
        const panel = await mount(
            <AirportHarness
                surface='panel'
                ident='KIND'
                reply='found'
            />);

        await panel.getByRole('button', {name: PLACE}).click();
        await expect(panel.getByTestId('selection')).toHaveText('location 39.7173,-86.2944');
    });

    test('offers no copy button over a classification', async ({mount}) => {
        const panel = await mount(
            <AirportHarness
                surface='panel'
                ident='KIND'
                reply='found'
                features={{mapPanel: false, mapInline: false, mapPage: false}}
            />);

        await expect(panel.getByText('Large Airport', {exact: true})).toBeVisible();
        await expect(panel.getByRole('button', {name: 'Copy Type'})).toHaveCount(0);

        // The rows that carry a value keep theirs.
        await expect(panel.getByRole('button', {name: 'Copy Code'})).toBeVisible();
        await expect(panel.getByRole('button', {name: 'Copy Place'})).toBeVisible();
        await expect(panel.getByRole('button', {name: 'Copy Elevation'})).toBeVisible();
        await expect(panel.getByRole('button', {name: 'Copy IATA'})).toBeVisible();
    });

    // Copying "Indianapolis, IN, US" is copying a value, and whether it also
    // opens something is beside the point.
    test('the place keeps its copy button', async ({mount}) => {
        const panel = await mount(
            <AirportHarness
                surface='panel'
                ident='KIND'
                reply='found'
            />);

        await expect(panel.getByRole('button', {name: 'Copy Place'})).toBeVisible();
    });

    // Offering a map or a link to a view that would refuse it is worse than
    // offering none, so an airfield with no usable position gets neither.
    test('draws nothing and offers no link when there is no coordinate', async ({mount, page}) => {
        await serveMapAssets(page);

        const panel = await mount(
            <AirportHarness
                surface='panel'
                ident='KIND'
                reply='no-position'
            />);

        await expect(panel.getByText('nothing to draw', {exact: false})).toBeVisible();
        await expect(panel.getByRole('button', {name: RESET})).toBeHidden();

        // The place is still shown, just not as a link to a view that would
        // refuse it.
        await expect(panel.getByText(PLACE, {exact: true})).toBeVisible();
        await expect(panel.getByRole('button', {name: PLACE})).toBeHidden();
    });

    /*
     * The map is the location decorator's own component, so what is under test
     * here is the wiring rather than the map: LocationMap.pw.tsx covers the
     * camera, both overlay sources and every failure. Reaching "Reset view" is
     * what proves it mounted and loaded, since a map that failed renders a note
     * and no controls at all.
     */
    test('draws the position under the airfield details', async ({mount, page}) => {
        await serveMapAssets(page);

        const panel = await mount(
            <AirportHarness
                surface='panel'
                ident='KIND'
                reply='found'
            />);

        await expect(panel.getByRole('button', {name: RESET})).toBeVisible();
    });

    /*
     * A fifth mount site for the map, so it has to honor the same switch the
     * other four do. Off means nothing is FETCHED rather than nothing drawn,
     * which gating the mount site is what achieves: the archive, the glyph
     * ranges and the library are all reached from this component and nowhere
     * else.
     */
    test('draws no map when the admin has turned panel maps off', async ({mount, page}) => {
        await serveMapAssets(page);

        const panel = await mount(
            <AirportHarness
                surface='panel'
                ident='KIND'
                reply='found'
                features={{mapPanel: false, mapInline: false, mapPage: false}}
            />);

        // The airfield is still fully rendered, and the readings are still one
        // click away on the place. Only the picture is gone.
        await expect(panel.getByText(NAME)).toBeVisible();
        await expect(panel.getByRole('button', {name: PLACE})).toBeVisible();
        await expect(panel.getByRole('button', {name: RESET})).toBeHidden();
    });

    // A refreshed database can retire a code. That is an answer, not a failure,
    // and it must not read as a broken link.
    test('says so for a code this build does not hold', async ({mount}) => {
        const panel = await mount(
            <AirportHarness
                surface='panel'
                ident='QZQZ'
                reply='unknown'
            />);

        await expect(panel.getByText('QZQZ', {exact: true})).toBeVisible();
        await expect(panel.getByText('not in this build', {exact: false})).toBeVisible();
        await expect(panel.getByText(NOT_A_CODE)).toBeHidden();
    });

    test('renders the airfield without readings when there is no position', async ({mount}) => {
        const panel = await mount(
            <AirportHarness
                surface='panel'
                ident='KIND'
                reply='no-position'
            />);

        await expect(panel.getByText(NAME)).toBeVisible();
        await expect(panel.getByText('no position in the database', {exact: false})).toBeVisible();
    });

    // A hand-edited link. Distinct from an outage, and from a code the database
    // does not hold, because all three want different things from the reader.
    test('reports a refused link as a verdict', async ({mount}) => {
        const panel = await mount(
            <AirportHarness
                surface='panel'
                ident='KIND'
                reply='rejected'
            />);

        await expect(panel.getByText(NOT_A_CODE)).toBeVisible();
    });

    // Nothing may fail the panel silently, and there is nothing local to fall
    // back to, so it says the link is fine and the server is not.
    test('reports an outage as an outage', async ({mount}) => {
        const panel = await mount(
            <AirportHarness
                surface='panel'
                ident='KIND'
                reply='failed'
            />);

        await expect(panel.getByText('could not be looked up', {exact: false})).toBeVisible();
        await expect(panel.getByText(NOT_A_CODE)).toBeHidden();
    });

    test('shows the code while the lookup is in flight', async ({mount}) => {
        const panel = await mount(
            <AirportHarness
                surface='panel'
                ident='KIND'
                reply='hold'
            />);

        await expect(panel.getByText('KIND', {exact: true})).toBeVisible();
        await expect(panel.getByText(NAME)).toBeHidden();
    });

    test('never shows the previous airfield once the code has changed', async ({mount}) => {
        const panel = await mount(
            <AirportHarness
                surface='panel'
                ident='KIND'
                reply='found'
                second='KJFK'
                features={{mapPanel: false, mapInline: false, mapPage: false}}
            />);

        await expect(panel.getByText(NAME)).toBeVisible();

        await panel.getByRole('button', {name: 'select the second airfield'}).click();

        await expect(panel.getByText(SECOND_NAME)).toBeVisible();
        await expect(panel.getByText(SECOND_PLACE, {exact: true})).toBeVisible();
        await expect(panel.getByText(NAME)).toBeHidden();
        await expect(panel.getByTestId('stale-frames')).toHaveText('0');
    });

    /*
     * The panel stays mounted across a change of selection, so a second code
     * drives the state back to loading. With the map mounted inside the "found"
     * branch that unmounted LocationMap and released its WebGL context, against
     * a browser cap of roughly sixteen shared with the hover and every inline
     * map on screen.
     *
     * The canvas node is the observable: MapLibre builds one per map, so if the
     * component is torn down and rebuilt the handle taken before the swap is
     * detached afterwards.
     */
    test('keeps one map across a change of selection', async ({mount, page}) => {
        await serveMapAssets(page);

        const panel = await mount(
            <AirportHarness
                surface='panel'
                ident='KIND'
                reply='found'
                second='KJFK'
            />);

        await expect(panel.getByRole('button', {name: RESET})).toBeVisible({timeout: 15_000});
        const canvas = await page.locator('canvas').first().elementHandle();

        await panel.getByRole('button', {name: 'select the second airfield'}).click();
        await expect(panel.getByText(SECOND_PLACE, {exact: true})).toBeVisible();
        await expect(panel.getByRole('button', {name: RESET})).toBeVisible({timeout: 15_000});

        expect(
            await canvas?.evaluate((el) => el.isConnected),
            'the map was torn down and rebuilt',
        ).toBe(true);
    });

    test('takes an already answered code from the cache rather than asking again', async ({mount}) => {
        const component = await mount(
            <AirportHarness
                surface='hover'
                ident='KIND'
                reply='found'
            />);

        await expect(component.getByText(NAME)).toBeVisible();
        await expect(component.getByTestId('requests')).toHaveText('1');

        await component.update(
            <AirportHarness
                surface='panel'
                ident='KIND'
                reply='found'
                features={{mapPanel: false, mapInline: false, mapPage: false}}
            />);

        await expect(component.getByText(PLACE, {exact: true})).toBeVisible();

        await expect(component.getByTestId('setup')).toHaveText('1');
        await expect(component.getByTestId('requests')).toHaveText('1');
    });

    test('leaves nothing behind when unmounted mid-lookup', async ({mount, page}) => {
        const errors: string[] = [];
        page.on('pageerror', (error) => errors.push(String(error)));
        page.on('console', (message) => {
            if (message.type() === 'error') {
                errors.push(message.text());
            }
        });

        const panel = await mount(
            <AirportHarness
                surface='panel'
                ident='KIND'
                reply='hold'
            />);

        await expect(panel.getByText('KIND', {exact: true})).toBeVisible();
        await panel.unmount();

        await expect(page.getByText('KIND', {exact: true})).toHaveCount(0);
        expect(errors, 'unmounting mid-request must not throw').toEqual([]);
    });

    test('says the position could not be read rather than blaming the database', async ({mount}) => {
        const panel = await mount(
            <AirportHarness
                surface='panel'
                ident='KIND'
                reply='unreadable-position'
                features={{mapPanel: false, mapInline: false, mapPage: false}}
            />);

        await expect(panel.getByText('could not be read by this version', {exact: false})).toBeVisible();
        await expect(panel.getByText('no position in the database', {exact: false})).toBeHidden();
        await expect(panel.getByRole('button', {name: PLACE})).toBeHidden();
    });

    test('omits the elevation and IATA rows when the database states neither', async ({mount}) => {
        const panel = await mount(
            <AirportHarness
                surface='panel'
                ident='KIND'
                reply='unnamed'
            />);

        await expect(panel.getByText(PLACE, {exact: true})).toBeVisible();
        await expect(panel.getByText('Elevation', {exact: true})).toBeHidden();
        await expect(panel.getByText('IATA', {exact: true})).toBeHidden();
    });

    test('shows sea level as a value rather than as an absence', async ({mount}) => {
        const panel = await mount(
            <AirportHarness
                surface='panel'
                ident='KIND'
                reply='sea-level'
            />);

        await expect(panel.getByText('Elevation', {exact: true})).toBeVisible();
        await expect(panel.getByText('0 ft', {exact: true})).toBeVisible();
    });

    test('heads an unnamed airfield with its code', async ({mount}) => {
        const named = await mount(
            <AirportHarness
                surface='panel'
                ident='KIND'
                reply='found'
                features={{mapPanel: false, mapInline: false, mapPage: false}}
            />);

        await expect(named.getByText(NAME)).toBeVisible();
        const baseline = await named.getByText('KIND', {exact: true}).count();
        await named.unmount();

        const panel = await mount(
            <AirportHarness
                surface='panel'
                ident='KIND'
                reply='unnamed'
                features={{mapPanel: false, mapInline: false, mapPage: false}}
            />);

        await expect(panel.getByText(PLACE, {exact: true})).toBeVisible();
        await expect(panel.getByText(NAME)).toBeHidden();
        await expect(panel.getByText('KIND', {exact: true})).toHaveCount(baseline + 1);
    });

    test('draws the map and offers no larger view when the map page is off', async ({mount, page}) => {
        await serveMapAssets(page);

        const panel = await mount(
            <AirportHarness
                surface='panel'
                ident='KIND'
                reply='found'
                features={{mapPanel: true, mapInline: true, mapPage: false}}
            />);

        await expect(panel.getByRole('button', {name: RESET})).toBeVisible();
        await expect(panel.getByRole('link', {name: 'Open larger'})).toBeHidden();
    });
});

test.describe('the hover card', () => {
    test('names the airfield and where it is', async ({mount}) => {
        const card = await mount(
            <AirportHarness
                surface='hover'
                ident='KIND'
                reply='found'
            />);

        await expect(card.getByText(NAME)).toBeVisible();
        await expect(card.getByText('Indianapolis, IN, US')).toBeVisible();
    });

    /*
     * A glance asks what the code is, and nothing more. Every reading is one
     * click away, and repeating them here only makes the glance slower. This is
     * the same bar the DTG hover meets with the countdown alone.
     */
    test('is the name and nothing else', async ({mount}) => {
        const card = await mount(
            <AirportHarness
                surface='hover'
                ident='KIND'
                reply='found'
            />);

        await Promise.all(
            ['797 ft', 'IND'].map(
                (reading) => expect(card.getByText(reading, {exact: true})).toBeHidden()));
    });

    test('names an unnamed airfield by its code', async ({mount}) => {
        const card = await mount(
            <AirportHarness
                surface='hover'
                ident='KIND'
                reply='unnamed'
            />);

        await expect(card.getByText('KIND', {exact: true})).toBeVisible();
        await expect(card.getByText(PLACE, {exact: true})).toBeVisible();
    });

    /*
     * The framework hides an empty card's chrome through a :empty rule, so a
     * hover with nothing to say is no card rather than an empty bordered box.
     * Every state but a found airfield renders nothing, because a card that
     * said "looking up" would flicker on every code a cursor crossed.
     */
    for (const reply of ['hold', 'unknown', 'rejected', 'failed'] as const) {
        test(`renders nothing at all when the lookup is ${reply}`, async ({mount, page}) => {
            await mount(
                <AirportHarness
                    surface='hover'
                    ident='KIND'
                    reply={reply}
                />);

            await expect(page.getByTestId('card')).toBeEmpty();
        });
    }
});
