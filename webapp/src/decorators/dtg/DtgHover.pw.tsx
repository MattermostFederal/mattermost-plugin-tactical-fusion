import React from 'react';

import DtgHover from './DtgHover';

import {expect, test} from '../../../playwright/ct-coverage';
import {stubPreferencesRoute} from '../../preferences/stub_route';

import type {Dtg} from './index';

function payload(overrides: Partial<Dtg> = {}): Dtg {
    return {
        instant: new Date(Date.UTC(2026, 7, 9, 16, 30, 0)),
        canonical: '091630ZAUG26',
        offsetMinutes: 0,
        zoneLabel: 'Z',
        assumedMonth: false,
        assumedYear: false,
        ...overrides,
    };
}

test('shows the countdown', async ({mount, page}) => {
    await mount(<DtgHover payload={payload()}/>);

    await expect(page.getByText(/ago$|^in |^now$/)).toBeVisible();
});

// A hover is a glance. Everything else about the DTG is a click away in the
// panel, and repeating it here only makes the glance slower.
test('shows nothing but the countdown', async ({mount, page}) => {
    await mount(<DtgHover payload={payload()}/>);

    await expect(page.locator('p')).toHaveCount(1);
    await expect(page.getByText('09 Aug 2026 16:30 Z')).toHaveCount(0);
    await expect(page.getByText('091630ZAUG26')).toHaveCount(0);
    await expect(page.locator('table')).toHaveCount(0);
});

test('stays a single line for an inferred short-form DTG', async ({mount, page}) => {
    await mount(
        <DtgHover payload={payload({canonical: '091630Z', assumedMonth: true, assumedYear: true})}/>,
    );

    await expect(page.locator('p')).toHaveCount(1);
    await expect(page.getByText(/post date/)).toHaveCount(0);
});

test('counts in real time', async ({mount, page}) => {
    const target = new Date(Date.now() + (3 * 3600 * 1000));
    await mount(<DtgHover payload={payload({instant: target})}/>);

    const countdown = page.getByText(/^in \d+h \d+m \d+s$/);
    await expect(countdown).toBeVisible();

    const first = await countdown.textContent();
    await expect(countdown).not.toHaveText(first!, {timeout: 3000});
});

// The hover uses the same countdown component as the panel, so urgency has to
// read the same way in both.
test('flags an imminent countdown as urgent', async ({mount, page}) => {
    const target = new Date(Date.now() + (10 * 60 * 1000));
    await mount(<DtgHover payload={payload({instant: target})}/>);

    await expect(page.locator('[data-urgent="true"]')).toBeVisible();
});

test('leaves a distant countdown alone', async ({mount, page}) => {
    const target = new Date(Date.now() + (48 * 3600 * 1000));
    await mount(<DtgHover payload={payload({instant: target})}/>);

    await expect(page.locator('[data-urgent="false"]')).toBeVisible();
});

// Pointing at a link and opening it must not disagree about whether the same
// DTG is imminent, so the hover reads the reader's threshold too.
test('honors the reader own flash threshold', async ({mount, page}) => {
    await stubPreferencesRoute(page, {stored: {zones: [], urgentWithinMinutes: 5}});

    // Ten minutes out is urgent by default, but not within five.
    const target = new Date(Date.now() + (10 * 60 * 1000));
    await mount(<DtgHover payload={payload({instant: target})}/>);

    await expect(page.locator('[data-urgent="false"]')).toBeVisible();
});

test('still shows only the countdown once the settings have loaded', async ({mount, page}) => {
    await stubPreferencesRoute(page, {stored: {zones: [{iana: 'UTC'}], urgentWithinMinutes: 5}});
    await mount(<DtgHover payload={payload()}/>);

    await expect(page.locator('p')).toHaveCount(1);
    await expect(page.locator('table')).toHaveCount(0);
});
