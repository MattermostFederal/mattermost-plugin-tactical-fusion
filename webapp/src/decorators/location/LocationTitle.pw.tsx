import React from 'react';

import TitleHarness from './TitleHarness';

import {expect, test} from '../../../playwright/ct-coverage';
import {stubPreferencesRoute} from '../../preferences/stub_route';

test('names the coordinate while the panel is showing it', async ({mount, page}) => {
    await stubPreferencesRoute(page);
    await mount(<TitleHarness/>);

    await expect(page.getByTestId('rhs-title')).toHaveText('Location');
});

// The editor takes the panel over, so a header still reading "Location" would
// be describing something no longer on screen.
test('follows the panel into the editor', async ({mount, page}) => {
    await stubPreferencesRoute(page);
    await mount(<TitleHarness/>);

    await page.getByRole('button', {name: 'Customize your view'}).click();

    await expect(page.getByTestId('rhs-title')).toHaveText('Customize your view');
});

test('follows it back out again', async ({mount, page}) => {
    await stubPreferencesRoute(page);
    await mount(<TitleHarness/>);

    await page.getByRole('button', {name: 'Customize your view'}).click();
    await expect(page.getByTestId('rhs-title')).toHaveText('Customize your view');

    await page.getByRole('button', {name: 'Back'}).click();

    await expect(page.getByTestId('rhs-title')).toHaveText('Location');
});
