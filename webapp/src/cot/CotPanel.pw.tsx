import React from 'react';

import CotPanelHarness from './CotPanelHarness';

import {expect, test} from '../../playwright/ct-coverage';

test('the sidebar is empty until the card asks for it', async ({mount}) => {
    const component = await mount(<CotPanelHarness/>);

    await expect(component.getByTestId('rhs')).toContainText('Tactical Fusion');
    await expect(component.getByTestId('rhs')).not.toContainText('Readings for this');
});

test('the card opens the sidebar on its own event', async ({mount}) => {
    const component = await mount(
        <CotPanelHarness event={{callsign: 'DELTA1', uid: 'ANDROID-1'}}/>,
    );

    await component.getByRole('button', {name: 'Open details'}).click();

    await expect(
        component.getByTestId('rhs').getByRole('group', {name: 'Readings for this Cursor on Target event'}),
    ).toBeVisible();
    await expect(component.getByTestId('rhs')).toContainText('ANDROID-1');
});

test('the sidebar header names the event rather than the feature', async ({mount}) => {
    const component = await mount(<CotPanelHarness event={{callsign: 'DELTA1'}}/>);

    await component.getByRole('button', {name: 'Open details'}).click();

    await expect(component.getByTestId('rhs-title')).toContainText('Cursor on Target: DELTA1');
});

test('an event with no callsign is named by its uid', async ({mount}) => {
    const component = await mount(<CotPanelHarness event={{callsign: ''}}/>);

    await component.getByRole('button', {name: 'Open details'}).click();

    await expect(component.getByTestId('rhs-title')).toContainText('Cursor on Target: ANDROID-1');
});

/*
 * The countdown is the one reading in the feature that depends on the reader's
 * clock, which is why it is here and not on the card: a ticking clock in a
 * channel reads as a live feed and puts a timer behind every post in the window.
 */
test.describe('the countdown', () => {
    test('runs in the panel, and says what it is counted against', async ({mount}) => {
        const staleAt = String(Date.now() + 3_600_000);
        const component = await mount(
            <CotPanelHarness event={{stale: '231245ZAUG26', staleAt}}/>,
        );

        await component.getByRole('button', {name: 'Open details'}).click();

        await expect(component.getByTestId('rhs')).toContainText('Goes stale');
        await expect(component.getByTestId('rhs')).toContainText('clock');
    });

    test('is absent when the event states no stale time', async ({mount}) => {
        const component = await mount(<CotPanelHarness event={{staleAt: ''}}/>);

        await component.getByRole('button', {name: 'Open details'}).click();

        await expect(component.getByTestId('rhs')).not.toContainText('Goes stale');
    });

    test('is absent when the stale time is not a number', async ({mount}) => {
        const component = await mount(<CotPanelHarness event={{staleAt: 'soon'}}/>);

        await component.getByRole('button', {name: 'Open details'}).click();

        await expect(component.getByTestId('rhs')).not.toContainText('Goes stale');
    });
});

test('an accuracy the event never stated is not invented in the panel either', async ({mount}) => {
    const component = await mount(<CotPanelHarness event={{ce: ''}}/>);

    await component.getByRole('button', {name: 'Open details'}).click();

    await expect(component.getByTestId('rhs')).toContainText('Not stated');
});

test('the panel links a position the server could spell', async ({mount}) => {
    const component = await mount(
        <CotPanelHarness
            event={{lat: '34.0561', lon: '-118.2500', format: 'dd', value: '34.0561,-118.2500'}}
        />,
    );

    await component.getByRole('button', {name: 'Open details'}).click();

    await expect(
        component.getByTestId('rhs').getByRole('link', {name: '34.0561, -118.2500'}),
    ).toHaveAttribute('href', /\/decorate\/location\?f=dd&v=/);
});

test('the panel shows the source as posted, reachable without a pointer', async ({mount}) => {
    const component = await mount(<CotPanelHarness src='<event uid="X"/>'/>);

    await component.getByRole('button', {name: 'Open details'}).click();

    const pane = component.getByTestId('rhs').getByRole('region', {name: 'The event as it was posted'});
    await expect(pane).toContainText('<event uid="X"/>');
    await expect(pane).toHaveAttribute('tabindex', '0');
});

test('the panel names the source file for a file post', async ({mount}) => {
    const component = await mount(
        <CotPanelHarness
            source='file'
            fileName='event.cot'
        />,
    );

    await component.getByRole('button', {name: 'Open details'}).click();

    await expect(component.getByTestId('rhs')).toContainText('event.cot');
});
