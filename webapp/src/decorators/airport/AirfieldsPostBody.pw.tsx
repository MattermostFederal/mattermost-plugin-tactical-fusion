import React from 'react';

import AirfieldsPostBodyHarness from './AirfieldsPostBodyHarness';
import {HICKAM, HONOLULU, messageFor} from './route_fixtures';

import {expect, test} from '../../../playwright/ct-coverage';
import {stubFeaturesRoute} from '../../features/stub_route';
import {serveMapAssets} from '../location/map/asset_fixtures';

test.beforeEach(async ({page}) => {
    await stubFeaturesRoute(page, {mapPanel: true, mapInline: false, mapPage: true});
});

test('renders the links as links and the airfields as a numbered legend', async ({mount}) => {
    const body = await mount(<AirfieldsPostBodyHarness/>);

    await expect(body.getByTestId('airfields-post')).toBeVisible();
    await expect(body.getByRole('link', {name: 'PHIK'})).toHaveAttribute('href', /airport\?v=PHIK/);
    await expect(body.getByRole('link', {name: 'HNL'})).toHaveAttribute('href', /airport\?i=HNL/);
    await expect(body.getByText('1.', {exact: true})).toBeVisible();
    await expect(body.getByRole('button', {name: 'Hickam Air Force Base'})).toBeVisible();
    await expect(body.getByRole('button', {name: 'Daniel K. Inouye International Airport'})).toBeVisible();
    await expect(body).toContainText('//');
});

test('a legend entry opens that airfield in the sidebar by the code the author wrote', async ({mount}) => {
    const body = await mount(<AirfieldsPostBodyHarness/>);

    await body.getByRole('button', {name: 'Daniel K. Inouye International Airport'}).click();

    await expect(body.getByTestId('selection')).toHaveText('airport {"key":"iata","code":"HNL"}');
});

test('falls back to the text when the message and the props disagree', async ({mount}) => {
    const body = await mount(
        <AirfieldsPostBodyHarness
            airfields={[HICKAM, HONOLULU]}
            message={messageFor([HONOLULU, HICKAM])}
        />,
    );

    await expect(body.getByTestId('airfields-post')).toBeHidden();
    await expect(body.getByRole('button', {name: 'Hickam Air Force Base'})).toBeHidden();
    await expect(body).toContainText('[HNL](');
});

test('falls back to the text when the message has prose the props do not know', async ({mount}) => {
    const body = await mount(
        <AirfieldsPostBodyHarness message={`${messageFor([HICKAM])} and more`}/>,
    );

    await expect(body.getByTestId('airfields-post')).toBeHidden();
});

test('stands down after an edit', async ({mount}) => {
    const body = await mount(<AirfieldsPostBodyHarness editAt={5}/>);

    await expect(body.getByTestId('airfields-post')).toBeHidden();
    await expect(body).toContainText('[PHIK](');
});

test('falls back when the props are missing or unreadable', async ({mount}) => {
    const missing = await mount(<AirfieldsPostBodyHarness airfields={null}/>);
    await expect(missing.getByTestId('airfields-post')).toBeHidden();
    await missing.unmount();

    const unreadable = await mount(<AirfieldsPostBodyHarness version={99}/>);
    await expect(unreadable.getByTestId('airfields-post')).toBeHidden();
});

test('draws no legend or map in the compact display', async ({mount}) => {
    const body = await mount(<AirfieldsPostBodyHarness compactDisplay={true}/>);

    await expect(body.getByRole('link', {name: 'PHIK'})).toBeVisible();
    await expect(body.getByRole('button', {name: 'Hickam Air Force Base'})).toBeHidden();
    await expect(body.getByTestId('airfields-map')).toBeHidden();
});

test('draws no map when the admin has turned the inline map off', async ({mount}) => {
    const body = await mount(<AirfieldsPostBodyHarness/>);

    await expect(body.getByRole('button', {name: 'Hickam Air Force Base'})).toBeVisible();
    await expect(body.getByTestId('airfields-map')).toBeHidden();
});

test('draws the route under the post when the inline map is on', async ({mount, page}) => {
    await stubFeaturesRoute(page, {mapPanel: true, mapInline: true, mapPage: true});
    await serveMapAssets(page);

    const body = await mount(<AirfieldsPostBodyHarness/>);

    await expect(body.getByTestId('airfields-map')).toBeVisible();
    await expect(body.getByRole('button', {name: 'Reset view'})).toBeVisible();
    await expect(body.getByRole('link', {name: 'Open larger'})).toHaveAttribute('href', /map\?post=post0000000000000000000000/);
});
