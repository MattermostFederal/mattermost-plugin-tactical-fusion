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

test('the map page carries the token, the position and the way back', async ({mount}) => {
    const component = await mount(<MapPageView data={{...GRID, mode: 'map'}}/>);

    await expect(component.getByText('18SUJ2347806483')).toBeVisible();
    await expect(component.getByText('38.8895° N, 77.0353° W')).toBeVisible();

    const back = component.getByRole('link', {name: 'All readings'});
    await expect(back).toBeVisible();
    await expect(back).toHaveAttribute('href', /\/decorate\/location\?f=mgrs&v=18SUJ2347806483/);
});

/*
 * The page that IS the larger view does not offer to open a larger view, and
 * does not repeat the attribution the bar beneath it already carries.
 */
test('the map page does not link to itself', async ({mount}) => {
    const component = await mount(<MapPageView data={{...GRID, mode: 'map'}}/>);

    await expect(component.getByText('Open larger')).toHaveCount(0);

    // Exact, because the Region reading ends in the same citation and the point
    // here is the standalone attribution, which the map's own caption used to
    // repeat directly above the bar carrying it.
    await expect(component.getByText('Natural Earth 110m', {exact: true})).toHaveCount(1);
});
