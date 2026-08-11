import {expect, test} from '@playwright/experimental-ct-react';
import React from 'react';

import TitleHarness from './TitleHarness';

import {stubPreferencesRoute} from '../../preferences/stub_route';

const INSTANT_MS = Date.UTC(2026, 7, 9, 16, 30, 0);

test('names the DTG while the panel is showing it', async ({mount, page}) => {
    await stubPreferencesRoute(page);
    await mount(<TitleHarness instantMs={INSTANT_MS}/>);

    await expect(page.getByTestId('rhs-title')).toHaveText('Date/Time');
});

// The editor takes the panel over, so a header still reading "Date/Time" would
// be describing something no longer on screen.
test('follows the panel into the editor', async ({mount, page}) => {
    await stubPreferencesRoute(page);
    await mount(<TitleHarness instantMs={INSTANT_MS}/>);

    await page.getByRole('button', {name: 'Customize your view'}).click();

    await expect(page.getByTestId('rhs-title')).toHaveText('Customize your view');
});

test('follows it back out again', async ({mount, page}) => {
    await stubPreferencesRoute(page);
    await mount(<TitleHarness instantMs={INSTANT_MS}/>);

    await page.getByRole('button', {name: 'Customize your view'}).click();
    await expect(page.getByTestId('rhs-title')).toHaveText('Customize your view');

    await page.getByRole('button', {name: 'Back'}).click();

    await expect(page.getByTestId('rhs-title')).toHaveText('Date/Time');
});

test('follows it back out after a save', async ({mount, page}) => {
    await stubPreferencesRoute(page);
    await mount(<TitleHarness instantMs={INSTANT_MS}/>);

    await page.getByRole('button', {name: 'Customize your view'}).click();
    await page.getByRole('button', {name: 'Save'}).click();

    await expect(page.getByTestId('rhs-title')).toHaveText('Date/Time');
    await expect(page.getByText('091630ZAUG26')).toBeVisible();
});

// The editor state outlives a change of selection, since React keeps the panel
// mounted across one. Without a reset, clicking a second DTG while editing
// would land on the editor rather than on the DTG that was clicked.
test('a different DTG closes the editor', async ({mount, page}) => {
    await stubPreferencesRoute(page);
    const component = await mount(<TitleHarness instantMs={INSTANT_MS}/>);

    await page.getByRole('button', {name: 'Customize your view'}).click();
    await expect(page.getByTestId('rhs-title')).toHaveText('Customize your view');

    await component.update(<TitleHarness instantMs={INSTANT_MS + (3600 * 1000)}/>);

    await expect(page.getByTestId('rhs-title')).toHaveText('Date/Time');
    await expect(page.locator('table')).toHaveCount(1);
});

// The header and the link that opens the editor say the same thing, so a
// reader is never told two names for where they are.
test('matches the link that opens the editor', async ({mount, page}) => {
    await stubPreferencesRoute(page);
    await mount(<TitleHarness instantMs={INSTANT_MS}/>);

    const link = page.getByRole('button', {name: 'Customize your view'});
    const label = await link.textContent();
    await link.click();

    await expect(page.getByTestId('rhs-title')).toHaveText(label!);
});
