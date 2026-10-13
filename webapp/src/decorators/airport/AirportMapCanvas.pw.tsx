import React from 'react';

import {AirportMapCanvas} from './AirportMapCanvas';
import type {AirportMapPayload} from './map';

import {expect, test} from '../../../playwright/ct-coverage';

const HONOLULU: AirportMapPayload = {
    ident: 'PHNL',
    name: 'Daniel K. Inouye International Airport',
    coordinate: {format: 'dd', value: '21.3184,-157.9257', region: 'United States of America'},
    runways: [{
        designation: '08L/26R',
        ends: [{format: 'dd', value: '21.3252,-157.9430'}, {format: 'dd', value: '21.3252,-157.9070'}],
    }],
};

test('draws the airfield with its runways', async ({mount, page}) => {
    await mount(
        <div data-testid='frame'>
            <AirportMapCanvas payload={HONOLULU}/>
        </div>,
    );

    await expect(page.getByTestId('frame')).not.toBeEmpty();
});

test('renders nothing for an airfield it cannot place', async ({mount, page}) => {
    await mount(
        <div data-testid='frame'>
            <AirportMapCanvas payload={{...HONOLULU, coordinate: {...HONOLULU.coordinate, value: 'nowhere'}}}/>
        </div>,
    );

    await expect(page.getByTestId('frame')).toBeEmpty();
});
