import React from 'react';

import FrequencyHover from './FrequencyHover';

import {expect, test} from '../../../playwright/ct-coverage';

test('names the frequency, its band and its use', async ({mount, page}) => {
    await mount(<FrequencyHover payload={{token: '121.5', khz: 121500}}/>);

    await expect(page.getByText('121.500 MHz, VHF air band')).toBeVisible();
    await expect(page.getByText('Aeronautical emergency')).toBeVisible();
});

test('leaves out the use line when no allocation is known', async ({mount, page}) => {
    await mount(<FrequencyHover payload={{token: '1090.0', khz: 1090000}}/>);

    await expect(page.getByText(/MHz, Outside the aviation bands this plugin names$/)).toBeVisible();
    await expect(page.locator('p')).toHaveCount(1);
});
