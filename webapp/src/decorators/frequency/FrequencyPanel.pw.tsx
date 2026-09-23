import React from 'react';

import FrequencyPanel from './FrequencyPanel';

import {expect, test} from '../../../playwright/ct-coverage';

test('shows the band, the readings and the allocation', async ({mount}) => {
    const panel = await mount(<FrequencyPanel payload={{token: '121.5', khz: 121500}}/>);

    await expect(panel.getByTestId('frequency-token')).toHaveText('121.5');
    await expect(panel.getByTestId('frequency-band')).toHaveText('VHF air band');
    await expect(panel.getByRole('row', {name: /^MHz /})).toContainText('121.500');
    await expect(panel.getByRole('row', {name: /^kHz /})).toContainText('121500');
    await expect(panel.getByRole('row', {name: /Channel/})).toContainText('25 kHz channel');
    await expect(panel.getByRole('row', {name: /Use/})).toContainText('Aeronautical emergency');
    await expect(panel.getByRole('button', {name: 'Copy MHz'})).toBeVisible();
});

test('says nothing about a channel or a use it does not know', async ({mount}) => {
    const panel = await mount(<FrequencyPanel payload={{token: '1090.0', khz: 1090000}}/>);

    await expect(panel.getByTestId('frequency-band')).toHaveText('Outside the aviation bands this plugin names');
    await expect(panel.getByRole('row', {name: /Channel/})).toBeHidden();
    await expect(panel.getByRole('row', {name: /Use/})).toBeHidden();
});
