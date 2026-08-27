import type {Page} from '@playwright/test';
import React from 'react';

import PageEntryHarness from './PageEntryHarness';
import type {Plant} from './PageEntryHarness';

import {expect, test} from '../../playwright/ct-coverage';

/*
 * The page bundle's entry point, which is what a client with no Mattermost
 * webapp runs: in practice the mobile app opening a decorated link.
 *
 * It reads the shell the server wrote and mounts one of two views into it. Both
 * of its early returns are silent by design, so the test for each is that
 * nothing renders and nothing throws.
 */

const CONVERSION = JSON.stringify({
    mgrs: '18S UJ 23478 06483',
    utm: '18S 323478E 4306483N',
    decimal: '38.8895° N, 77.0353° W',
    dms: '38°53\'22"N 77°02\'07"W',
    ddm: '38°53.37\'N 77°02.12\'W',
    usmtf: '385322N0770207W',
    georef: 'GJNJ57885337',
    gars: '206LT26',
    pluscode: '87C4VXQ7+RV44',
    region: 'United States of America (Natural Earth 110m)',
    lat: 38.8895,
    lon: -77.0353,
});

const SHELL: Plant = {
    f: 'mgrs',
    v: '18SUJ2347806483',
    r: '18S UJ 23478 06483',
    conversion: CONVERSION,
};

async function load(
    mount: (component: React.JSX.Element) => Promise<unknown>,
    page: Page,
    plant: Plant | null,
): Promise<void> {
    await mount(<PageEntryHarness plant={plant}/>);

    // Every assertion below depends on the entry point having run. `mount`
    // resolves as soon as the harness renders, which is before the dynamic
    // import settles and before `#page-root` exists, so a negative assertion
    // made here would pass against a document nothing had written to yet.
    await expect(page.getByTestId('entry-state')).toHaveText('imported');
}

/*
 * A document with no shell in it. The entry point is loaded by the shell the
 * server renders, so this can only happen to a page served without one, and the
 * honest answer is to do nothing rather than to throw in a client whose console
 * nobody is reading.
 */
test('a document with no root renders nothing and does not throw', async ({mount, page}) => {
    const thrown: string[] = [];
    page.on('pageerror', (error) => thrown.push(error.message));

    await load(mount, page, null);

    // Not `#page-root`: with no shell planted, nothing could ever carry that id,
    // so counting it passes whatever the entry point does. The claim worth
    // making is that it rendered into nothing at all, which is what catches a
    // guard replaced by a fallback container.
    await expect(page.locator('table')).toHaveCount(0);
    await expect(page.getByText('Documentation')).toHaveCount(0);
    await expect.poll(() => thrown).toEqual([]);
});

/*
 * A shell whose format this build does not have still renders every reading.
 *
 * This asserted the opposite, and the opposite was the defect: the entry point
 * left the document alone, so the reader got a wholly blank page on the route
 * the mobile app opens, while the shell's own `data-conversion` carried MGRS,
 * UTM, all three angular readings and the region. Nothing was logged, and no
 * test failed, because this one said blank was correct.
 *
 * It degrades the way a canonical-shape disagreement already did: no local
 * coordinate, every row from the conversion the server worked out. Which is the
 * point of the shell carrying one at all.
 */
test('a shell naming a format this build does not have renders the server readings',
    async ({mount, page}) => {
        const thrown: string[] = [];
        page.on('pageerror', (error) => thrown.push(error.message));

        await load(mount, page, {...SHELL, f: 'notaformat'});

        await expect(page.locator('table')).toHaveCount(1);
        await expect(page.getByText('18S UJ 23478 06483').first()).toBeVisible();
        await expect(page.getByText('38.8895° N, 77.0353° W')).toBeVisible();
        await expect.poll(() => thrown).toEqual([]);
    });

test.describe('the readings page', () => {
    test('renders the table from the shell alone', async ({mount, page}) => {
        await load(mount, page, SHELL);

        const root = page.locator('#page-root');

        // Every row the token cannot work out for itself is already answered,
        // because the server put its conversion in the shell rather than
        // leaving a page with no session to ask a route it cannot reach.
        await expect(root.getByText('18S 323478E 4306483N')).toBeVisible();
        await expect(root.getByText('38.8895° N, 77.0353° W')).toBeVisible();
        await expect(root.getByText('converting…')).toHaveCount(0);
        await expect(root.getByText('unavailable')).toHaveCount(0);
    });

    // The author's own spelling, which is the whole reason `r` travels in the
    // link: this page shows only the link, never the message it came from.
    test('leads with the author\'s own text', async ({mount, page}) => {
        await load(mount, page, SHELL);

        await expect(page.locator('#page-root').getByText('18S UJ 23478 06483').first()).toBeVisible();
    });

    test('offers the documentation as its footer', async ({mount, page}) => {
        await load(mount, page, SHELL);

        await expect(page.locator('#page-root').getByText('Documentation')).toBeVisible();
    });

    // Reader preferences need a session and the page renderer is handed no
    // reader, so every row is shown and there is nothing to customize.
    test('offers no Customize link, which would need a session', async ({mount, page}) => {
        await load(mount, page, SHELL);

        const root = page.locator('#page-root');

        await expect(root.getByText('Documentation')).toBeVisible();
        await expect(root.getByText('Customize your view')).toHaveCount(0);
    });
});

test.describe('the map page', () => {
    test('gives the window to the picture instead of the table', async ({mount, page}) => {
        await load(mount, page, {...SHELL, mode: 'map'});

        const root = page.locator('#page-root');

        await expect(root.getByText('All readings')).toBeVisible();
        await expect(root.getByText('18S UJ 23478 06483')).toBeVisible();

        // The readings live one link away, which is the point of the two pages
        // being separate routes rather than two modes of one.
        await expect(root.getByText('18S 323478E 4306483N')).toHaveCount(0);
    });

    // Anything other than the literal keyword is the readings page. The shell
    // writes the attribute for both, so a typo must not open a map.
    test('an unrecognized mode is the readings page', async ({mount, page}) => {
        await load(mount, page, {...SHELL, mode: 'chart'});

        await expect(page.locator('#page-root').getByText('18S 323478E 4306483N')).toBeVisible();
        await expect(page.locator('#page-root').getByText('All readings')).toHaveCount(0);
    });
});
