import React from 'react';

import LinkHarness from './LinkHarness';

import {expect, test} from '../../playwright/ct-coverage';

test('renders the decorator link the server built, with its hover card', async ({mount}) => {
    const component = await mount(<LinkHarness token='091630ZAUG26'/>);

    const anchor = component.getByRole('link', {name: '091630ZAUG26'});
    await expect(anchor).toHaveAttribute('href', /\/decorate\/dtg\?a=&dtg=091630ZAUG26&t=\d+&z=Z$/);

    const card = component.locator('.tactical-fusion-hover-card');
    await expect(card).toBeHidden();
    await anchor.hover();
    await expect(card).toBeVisible();
    await expect(card).toContainText('in 1h 29m');
});

test('uses the label the host asked for', async ({mount}) => {
    const component = await mount(
        <LinkHarness
            token='091630ZAUG26'
            label='Wheels down'
        />,
    );

    await expect(component.getByRole('link', {name: 'Wheels down'})).toBeVisible();
});

test('a declined token renders the fallback as plain text', async ({mount}) => {
    const component = await mount(
        <LinkHarness
            token='091630J'
            fallback='local time'
        />,
    );

    await expect(component).toContainText('local time');
    await expect(component.getByRole('link')).toHaveCount(0);
});

test('a declined token with no fallback renders the token', async ({mount}) => {
    const component = await mount(<LinkHarness token='091630J'/>);

    await expect(component.locator('[data-tactical-fusion-link="declined"]')).toHaveText('091630J');
    await expect(component.getByRole('link')).toHaveCount(0);
});
