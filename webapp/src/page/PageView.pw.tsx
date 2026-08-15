import React from 'react';

import MapPageView from './MapPageView';
import type {PageData} from './payload';

import {expect, test} from '../../playwright/ct-coverage';
import LocationReadings from '../decorators/location/LocationReadings';

/*
 * The standalone pages, which are what a client with no Mattermost webapp
 * opens. They render the same components the sidebar renders, so what is worth
 * testing here is the environment the pages supply rather than the table: the
 * conversion arrives already answered, there is no Customize link, and the map
 * page gives the window to one picture.
 */

const GRID: PageData = {
    payload: {coord: null, format: 'mgrs', canonical: '18SUJ2347806483', raw: '18S UJ 23478 06483'},
    conversion: {
        status: 'ready',
        data: {
            mgrs: '18S UJ 23478 06483',
            utm: '18S 323478E 4306483N',
            decimal: '38.8895° N, 77.0353° W',
            dms: '38°53\'22"N 77°02\'07"W',
            ddm: '38°53.37\'N 77°02.12\'W',
            usmtf: '385322N0770207W',
            region: 'United States of America (Natural Earth 110m)',
            lat: 38.8895,
            lon: -77.0353,
        },
    },
    mode: 'location',
};

test('a page renders every row without waiting on anything', async ({mount}) => {
    const component = await mount(
        <LocationReadings
            payload={GRID.payload}
            conversion={GRID.conversion}
            hidden={[]}
        />,
    );

    // The rows a grid token cannot work out for itself are already answered,
    // because the server put them in the shell rather than leaving the page to
    // ask a route it has no session for.
    await expect(component.getByText('18S 323478E 4306483N')).toBeVisible();
    await expect(component.getByText('38.8895° N, 77.0353° W')).toBeVisible();
    await expect(component.getByText('converting…')).toHaveCount(0);
    await expect(component.getByText('unavailable')).toHaveCount(0);
});

test('a page offers no Customize link, which would need a session', async ({mount}) => {
    const component = await mount(
        <LocationReadings
            payload={GRID.payload}
            conversion={GRID.conversion}
            hidden={[]}
        />,
    );

    await expect(component.getByText('Customize your view')).toHaveCount(0);
});

/*
 * A reader's hidden rows live behind the authenticated API, so a public page
 * has nobody to ask and shows the lot. That is what the server-rendered page
 * showed, and it is the honest answer rather than a guess at somebody's
 * settings.
 */
test('a page shows every row, because it has no reader to ask', async ({mount}) => {
    const component = await mount(
        <LocationReadings
            payload={GRID.payload}
            conversion={GRID.conversion}
            hidden={[]}
        />,
    );

    // Every row the token and the conversion between them can fill: no
    // Confidence, because this token states none.
    await expect(component.getByRole('row')).toHaveCount(10);
});

test('a failed conversion degrades rather than blanking the page', async ({mount}) => {
    const component = await mount(
        <LocationReadings
            payload={GRID.payload}
            conversion={{status: 'failed', data: null}}
            hidden={[]}
        />,
    );

    // The token still names its own grid square, so that row is never in doubt.
    await expect(component.getByText('18S UJ 23478 06483').first()).toBeVisible();
    await expect(component.getByText('unavailable').first()).toBeVisible();
});

/*
 * "Open larger" is outside /decorate, so the framework's click handler stands
 * aside and nothing else would tell the map page which way to paint itself. It
 * opened on the operating system preference instead, which meant a light
 * Mattermost on a dark laptop gave a dark page and a dark map palette with it.
 *
 * The same three cases the click handler is held to, on the one link it does
 * not cover.
 */
test('Open larger tells the map page which theme to paint itself with', async ({mount, page}) => {
    await page.evaluate(() => document.documentElement.style.setProperty('--center-channel-bg', '#090a0b'));

    const component = await mount(
        <LocationReadings
            payload={GRID.payload}
            conversion={GRID.conversion}
            hidden={[]}
        />,
    );

    const larger = component.getByRole('link', {name: 'Open larger'});
    await expect(larger).toHaveAttribute('href', /_theme=dark/);

    // Still root-relative: storing a host is what the whole URL design avoids.
    const href = await larger.getAttribute('href');
    expect(href!.startsWith('/')).toBe(true);

    await page.evaluate(() => document.documentElement.style.removeProperty('--center-channel-bg'));
});

test('Open larger passes a light theme when the sidebar is light', async ({mount, page}) => {
    await page.evaluate(() => document.documentElement.style.setProperty('--center-channel-bg', '#ffffff'));

    const component = await mount(
        <LocationReadings
            payload={GRID.payload}
            conversion={GRID.conversion}
            hidden={[]}
        />,
    );

    await expect(component.getByRole('link', {name: 'Open larger'})).toHaveAttribute('href', /_theme=light/);

    await page.evaluate(() => document.documentElement.style.removeProperty('--center-channel-bg'));
});

test('Open larger leaves the theme unstated when it cannot be read', async ({mount}) => {
    const component = await mount(
        <LocationReadings
            payload={GRID.payload}
            conversion={GRID.conversion}
            hidden={[]}
        />,
    );

    await expect(component.getByRole('link', {name: 'Open larger'})).not.toHaveAttribute('href', /_theme/);
});

test('the map page carries the author\'s text and the way back', async ({mount}) => {
    const component = await mount(<MapPageView data={{...GRID, mode: 'map'}}/>);

    await expect(component.getByText('18S UJ 23478 06483')).toBeVisible();

    const back = component.getByRole('link', {name: 'All readings'});
    await expect(back).toBeVisible();
    await expect(back).toHaveAttribute('href', /\/decorate\/location\?f=mgrs&v=18SUJ2347806483/);
});

/*
 * The return trip. A page has no webapp around it and therefore no click
 * handler, so this link states the theme itself or the reader lands back on the
 * readings page painted the other way.
 */
test('the way back carries the theme the map page was painted with', async ({mount, page}) => {
    await page.evaluate(() => document.documentElement.style.setProperty('--center-channel-bg', '#090a0b'));

    const component = await mount(<MapPageView data={{...GRID, mode: 'map'}}/>);

    await expect(component.getByRole('link', {name: 'All readings'})).toHaveAttribute('href', /_theme=dark/);

    await page.evaluate(() => document.documentElement.style.removeProperty('--center-channel-bg'));
});

/*
 * Everything else the bar used to carry is a reading, and the page one link
 * away is nothing but readings with their labels beside them. Four of them
 * crammed unlabeled under a picture is a worse table rather than a summary.
 */
test('the map page leaves the readings to the readings page', async ({mount}) => {
    const component = await mount(<MapPageView data={{...GRID, mode: 'map'}}/>);

    await expect(component.getByText('18SUJ2347806483', {exact: true})).toHaveCount(0);
    await expect(component.getByText('38.8895° N, 77.0353° W')).toHaveCount(0);
    await expect(component.getByText('United States of America (Natural Earth 110m)')).toHaveCount(0);
});

/*
 * The page that IS the larger view does not offer to open a larger view.
 */
test('the map page does not link to itself', async ({mount}) => {
    const component = await mount(<MapPageView data={{...GRID, mode: 'map'}}/>);

    await expect(component.getByText('Open larger')).toHaveCount(0);
});

/*
 * No surface credits the basemap in words any more. The region's own value
 * still carries its citation, which is a different thing: it says where the
 * country came from, and it reaches the map's accessible label rather than the
 * screen.
 */
test('the map page does not print the basemap credit', async ({mount}) => {
    const component = await mount(<MapPageView data={{...GRID, mode: 'map'}}/>);

    await expect(component.getByText('Natural Earth 110m', {exact: true})).toHaveCount(0);
});

test('the readings do not print the basemap credit either', async ({mount}) => {
    const component = await mount(
        <LocationReadings
            payload={GRID.payload}
            conversion={GRID.conversion}
            hidden={[]}
        />,
    );

    await expect(component.getByText('Natural Earth 110m', {exact: true})).toHaveCount(0);

    // The map is still there, and still offers its one control.
    await expect(component.getByRole('link', {name: 'Open larger'})).toBeVisible();
});

/*
 * The author's text is what the bar names, so a conversion that did not arrive
 * must fall back to the token rather than showing an `r` nothing vouched for.
 */
test('the map page falls back to the token when nothing vouched for the text', async ({mount}) => {
    const component = await mount(
        <MapPageView data={{...GRID, conversion: {status: 'failed', data: null}, mode: 'map'}}/>,
    );

    await expect(component.getByText('18SUJ2347806483', {exact: true})).toBeVisible();
    await expect(component.getByText('18S UJ 23478 06483')).toHaveCount(0);
});
