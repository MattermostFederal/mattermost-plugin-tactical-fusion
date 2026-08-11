import type {Locator} from '@playwright/test';
import React from 'react';

import DtgPanel from './DtgPanel';
import {DISPLAY_ZONES} from './zones';

import {expect, test} from '../../../playwright/ct-coverage';
import {stubPreferencesRoute} from '../../preferences/stub_route';

import type {Dtg} from './index';

// 091630ZAUG26 is 2026-08-09T16:30:00Z.
const zulu: Dtg = {
    instant: new Date(Date.UTC(2026, 7, 9, 16, 30, 0)),
    canonical: '091630ZAUG26',
    offsetMinutes: 0,
    zoneLabel: 'Z',
    assumedMonth: false,
    assumedYear: false,
};

test('renders the canonical DTG and the zone table', async ({mount, page}) => {
    await mount(<DtgPanel payload={zulu}/>);

    await expect(page.getByText('091630ZAUG26')).toBeVisible();

    // Scoped to the table: the editor at the bottom names the same zones.
    await expect(page.locator('tbody').getByText('Zulu (UTC)')).toBeVisible();

    await expect(page.locator('tbody tr')).toHaveCount(DISPLAY_ZONES.length);
});

// The standalone page leads with the same three lines in the same order, so the
// two renderings cannot tell different stories about one message.
test('leads with the countdown, then the plain reading, then the token', async ({mount, page}) => {
    await mount(<DtgPanel payload={zulu}/>);

    const lines = page.locator('p');

    await expect(lines.nth(0)).toHaveText(/ago$|^in |^now$/);
    await expect(lines.nth(1)).toHaveText('09 Aug 2026 16:30 Z');
    await expect(lines.nth(2)).toHaveText('091630ZAUG26');
});

test('describes a non-Zulu DTG in its own zone', async ({mount, page}) => {
    await mount(
        <DtgPanel
            payload={{

                // 16:30R is 21:30Z.
                instant: new Date(Date.UTC(2026, 7, 9, 21, 30, 0)),
                canonical: '091630RAUG26',
                offsetMinutes: -5 * 60,
                zoneLabel: 'R',
                assumedMonth: false,
                assumedYear: false,
            }}
        />,
    );

    await expect(page.getByText('09 Aug 2026 16:30 R')).toBeVisible();
});

test('converts into each display zone', async ({mount, page}) => {
    await mount(<DtgPanel payload={zulu}/>);

    // 16:30Z is 01:30 the next day in Tokyo.
    const tokyoRow = page.locator('tbody tr', {hasText: 'Yokota'});
    await expect(tokyoRow.locator('td').nth(1)).toHaveText('01:30');
});

test('converts into Pacific time', async ({mount, page}) => {
    await mount(<DtgPanel payload={zulu}/>);

    // 9 August is daylight saving, so 16:30Z is 09:30 in Los Angeles.
    const pacificRow = page.locator('tbody tr', {hasText: 'San Diego'});
    await expect(pacificRow.locator('td').nth(1)).toHaveText('09:30');
});

// The abbreviation is a fixed label, but the time comes from the IANA zone, so
// the row has to shift with the season.
test('Pacific follows daylight saving', async ({mount, page}) => {
    await mount(<DtgPanel payload={{...zulu, instant: new Date(Date.UTC(2026, 0, 9, 16, 30, 0))}}/>);

    // Mid-winter the same zone is an hour further back.
    const pacificRow = page.locator('tbody tr', {hasText: 'San Diego'});
    await expect(pacificRow.locator('td').nth(1)).toHaveText('08:30');
});

test('badges a row that falls on a different day', async ({mount, page}) => {
    await mount(<DtgPanel payload={zulu}/>);

    // Tokyo is a day ahead of the Zulu reference date for this instant.
    const tokyoRow = page.locator('tbody tr', {hasText: 'Yokota'});
    await expect(tokyoRow.getByText('+1')).toBeVisible();

    // Honolulu is on the same date, so it carries no badge.
    const honoluluRow = page.locator('tbody tr', {hasText: 'Honolulu'});
    await expect(honoluluRow.getByText('+1')).toHaveCount(0);
    await expect(honoluluRow.getByText('-1')).toHaveCount(0);
});

// The offset is the only time-dependent value in the panel now.
test('counts in real time and shows no wall clock', async ({mount, page}) => {
    // Far enough out that the reading is stable across the test.
    const target = new Date(Date.now() + (3 * 3600 * 1000));
    await mount(<DtgPanel payload={{...zulu, instant: target}}/>);

    const offset = page.getByText(/^in \d+h \d+m \d+s$/);
    await expect(offset).toBeVisible();

    const first = await offset.textContent();
    await expect(offset).not.toHaveText(first!, {timeout: 3000});

    await expect(page.getByText(/Current time/)).toHaveCount(0);
});

// Within half an hour either way, the countdown calls for attention.
test('flags an imminent countdown as urgent', async ({mount, page}) => {
    const target = new Date(Date.now() + (10 * 60 * 1000));
    await mount(<DtgPanel payload={{...zulu, instant: target}}/>);

    await expect(page.locator('[data-urgent="true"]')).toBeVisible();
});

test('flags a recently passed instant as urgent', async ({mount, page}) => {
    const target = new Date(Date.now() - (10 * 60 * 1000));
    await mount(<DtgPanel payload={{...zulu, instant: target}}/>);

    await expect(page.locator('[data-urgent="true"]')).toBeVisible();
});

test('leaves a distant countdown alone', async ({mount, page}) => {
    const target = new Date(Date.now() + (48 * 3600 * 1000));
    await mount(<DtgPanel payload={{...zulu, instant: target}}/>);

    await expect(page.locator('[data-urgent="false"]')).toBeVisible();
    await expect(page.locator('[data-urgent="true"]')).toHaveCount(0);
});

// A reader with reduced motion enabled sees no pulse at all, so urgency has to
// survive without it, and without resting on colour alone.
test('marks urgency without relying on motion', async ({mount, page}) => {
    const target = new Date(Date.now() + (10 * 60 * 1000));
    await mount(<DtgPanel payload={{...zulu, instant: target}}/>);

    const countdown = page.locator('[data-urgent="true"]');
    const width = await countdown.evaluate((el) => getComputedStyle(el).borderLeftWidth);

    expect(width).toBe('4px');
});

test('leaves a distant countdown unmarked', async ({mount, page}) => {
    const target = new Date(Date.now() + (48 * 3600 * 1000));
    await mount(<DtgPanel payload={{...zulu, instant: target}}/>);

    const countdown = page.locator('[data-urgent="false"]');
    const width = await countdown.evaluate((el) => getComputedStyle(el).borderLeftWidth);

    expect(width).toBe('0px');
});

// The pulse is an opacity change on a timer, so the text stays legible at both
// ends of it rather than disappearing.
test('pulses an urgent countdown without hiding it', async ({mount, page}) => {
    const target = new Date(Date.now() + (10 * 60 * 1000));
    await mount(<DtgPanel payload={{...zulu, instant: target}}/>);

    const countdown = page.locator('[data-urgent="true"]');
    const opacity = () => countdown.evaluate((el) => getComputedStyle(el).opacity);

    const first = await opacity();
    expect(Number(first)).toBeGreaterThan(0.2);

    // It has to actually move, or it is not pulsing.
    await expect.poll(opacity, {timeout: 3000}).not.toBe(first);

    // ...and stay readable at the other end of the pulse.
    expect(Number(await opacity())).toBeGreaterThan(0.2);
});

test('counts up for an instant in the past', async ({mount, page}) => {
    const target = new Date(Date.now() - (90 * 60 * 1000));
    await mount(<DtgPanel payload={{...zulu, instant: target}}/>);

    await expect(page.getByText(/^1h 30m \d+s ago$/)).toBeVisible();
});

test('shows the assumed-fields note for a short-form DTG', async ({mount, page}) => {
    await mount(
        <DtgPanel
            payload={{...zulu, canonical: '091630Z', assumedMonth: true, assumedYear: true}}
        />,
    );

    await expect(page.getByText(/taken from the date the message was posted/)).toBeVisible();
});

test('omits the note when nothing was assumed', async ({mount, page}) => {
    await mount(<DtgPanel payload={zulu}/>);

    await expect(page.getByText(/taken from the date the message was posted/)).toHaveCount(0);
});

// The day badge is measured against the DTG's own zone, so a non-Zulu DTG is
// not badged against UTC.
test('measures day offsets against the DTG zone', async ({mount, page}) => {
    // 01:00M on the 9th is 13:00Z on the 8th. The author wrote "the 9th", so
    // the UTC row is a day behind that.
    await mount(
        <DtgPanel
            payload={{
                instant: new Date(Date.UTC(2026, 7, 8, 13, 0, 0)),
                canonical: '090100MAUG26',
                offsetMinutes: 12 * 60,
                zoneLabel: 'M',
                assumedMonth: false,
                assumedYear: false,
            }}
        />,
    );

    const utcRow = page.locator('tbody tr', {hasText: 'Zulu (UTC)'});
    await expect(utcRow.getByText('-1')).toBeVisible();
});

// The reader's own settings, which is the whole point of the editor at the
// bottom of the panel.
test('renders the reader own timezones instead of the built-in table', async ({mount, page}) => {
    await stubPreferencesRoute(page, {
        stored: {zones: [{iana: 'Europe/Paris'}, {iana: 'UTC'}], urgentWithinMinutes: 0},
    });
    await mount(<DtgPanel payload={zulu}/>);

    // West to east, whatever order they were saved in. Both were picked bare
    // out of "All timezones", so both read as their own city.
    await expect(page.locator('tbody tr')).toHaveCount(2);
    await expect(page.locator('tbody tr').nth(0)).toContainText('UTC');
    await expect(page.locator('tbody tr').nth(1)).toContainText('Paris');

    // 16:30Z is 18:30 in Paris in August.
    await expect(page.locator('tbody tr').nth(1).locator('td').nth(1)).toHaveText('18:30');
});

// The table reads like a world clock rather than in whatever order the zones
// were added.
test('orders the table west to east', async ({mount, page}) => {
    await stubPreferencesRoute(page);
    await mount(<DtgPanel payload={zulu}/>);

    const names = await page.locator('tbody tr td:first-child').allTextContents();

    expect(names[0]).toContain('Honolulu');
    expect(names[names.length - 1]).toContain('Andersen, Guam');
});

test('falls back to the built-in table when nothing is saved', async ({mount, page}) => {
    await stubPreferencesRoute(page);
    await mount(<DtgPanel payload={zulu}/>);

    await expect(page.locator('tbody tr')).toHaveCount(DISPLAY_ZONES.length);
});

// A shorter threshold has to actually narrow the window, or the setting is
// decorative.
test('honours a shorter flash threshold', async ({mount, page}) => {
    await stubPreferencesRoute(page, {stored: {zones: [], urgentWithinMinutes: 5}});

    // Ten minutes out is urgent by default, but not within five.
    const target = new Date(Date.now() + (10 * 60 * 1000));
    await mount(<DtgPanel payload={{...zulu, instant: target}}/>);

    await expect(page.locator('[data-urgent="false"]')).toBeVisible();
});

test('honours a longer flash threshold', async ({mount, page}) => {
    await stubPreferencesRoute(page, {stored: {zones: [], urgentWithinMinutes: 180}});

    // Two hours out is well outside the default half hour.
    const target = new Date(Date.now() + (2 * 3600 * 1000));
    await mount(<DtgPanel payload={{...zulu, instant: target}}/>);

    await expect(page.locator('[data-urgent="true"]')).toBeVisible();
});

// Settings that could not be loaded must leave a working panel behind, not an
// empty one.
test('shows the defaults when the settings cannot be loaded', async ({mount, page}) => {
    await stubPreferencesRoute(page, {loadStatus: 500});
    await mount(<DtgPanel payload={zulu}/>);

    await expect(page.locator('tbody tr')).toHaveCount(DISPLAY_ZONES.length);
    await expect(page.getByText('091630ZAUG26')).toBeVisible();
});

// Below the table, not above it: the panel answers the question first.
test('offers the editor as a link below the table', async ({mount, page}) => {
    await stubPreferencesRoute(page);
    await mount(<DtgPanel payload={zulu}/>);

    const link = page.getByRole('button', {name: 'Customize your view'});
    await expect(link).toBeVisible();

    const order = await page.evaluate(() => {
        const table = document.querySelector('table');
        const button = document.querySelector('button');
        if (!table || !button) {
            return 'missing';
        }
        return table.compareDocumentPosition(button) & Node.DOCUMENT_POSITION_FOLLOWING ? 'after' : 'before';
    });
    expect(order).toBe('after');

    // A link, not a button: it goes somewhere rather than doing something.
    const look = await link.evaluate((el) => {
        const computed = getComputedStyle(el);
        return {
            border: computed.borderTopWidth,
            background: computed.backgroundColor,
        };
    });
    expect(look.border).toBe('0px');
    expect(look.background).toBe('rgba(0, 0, 0, 0)');
});

const decorationOf = (locator: Locator) =>
    locator.evaluate((el) => getComputedStyle(el).textDecorationLine);

// Like every other link in Mattermost: quiet until pointed at.
test('underlines the link only on hover', async ({mount, page}) => {
    await stubPreferencesRoute(page);
    await mount(<DtgPanel payload={zulu}/>);

    const link = page.getByRole('button', {name: 'Customize your view'});
    expect(await decorationOf(link)).toBe('none');

    await link.hover();
    await expect.poll(() => decorationOf(link)).toBe('underline');

    // ...and back to quiet when the pointer moves away.
    await page.locator('table').hover();
    await expect.poll(() => decorationOf(link)).toBe('none');
});

// Hover alone would leave the underline invisible to anybody moving through
// the panel by keyboard.
test('underlines the link on keyboard focus', async ({mount, page}) => {
    await stubPreferencesRoute(page);
    await mount(<DtgPanel payload={zulu}/>);

    const link = page.getByRole('button', {name: 'Customize your view'});
    await link.focus();

    await expect.poll(() => decorationOf(link)).toBe('underline');
});

// A way out of the panel, not a call to action, so it sits below the body type
// rather than at it.
test('keeps the link understated', async ({mount, page}) => {
    await stubPreferencesRoute(page);
    await mount(<DtgPanel payload={zulu}/>);

    const link = page.getByRole('button', {name: 'Customize your view'});
    const size = await link.evaluate((el) => getComputedStyle(el).fontSize);

    expect(Number.parseFloat(size)).toBeLessThan(13);
});

// The table's own last rule already separates the link from the rows, so a
// second line right above it would just be a double underline.
test('draws no rule above the link', async ({mount, page}) => {
    await stubPreferencesRoute(page);
    await mount(<DtgPanel payload={zulu}/>);

    const width = await page.getByRole('button', {name: 'Customize your view'}).
        evaluate((el) => getComputedStyle(el).borderTopWidth);

    expect(width).toBe('0px');
});

// The picker is several hundred rows. Under the table it would have buried the
// DTG the reader opened the sidebar to see.
test('the editor takes over the panel', async ({mount, page}) => {
    await stubPreferencesRoute(page);
    await mount(<DtgPanel payload={zulu}/>);

    await page.getByRole('button', {name: 'Customize your view'}).click();

    await expect(page.getByRole('button', {name: 'Save'})).toBeVisible();

    // The DTG itself is gone while editing, not merely pushed down.
    await expect(page.locator('table')).toHaveCount(0);
    await expect(page.getByText('091630ZAUG26')).toHaveCount(0);
});

test('saving returns to the DTG, with the new table', async ({mount, page}) => {
    await stubPreferencesRoute(page);
    await mount(<DtgPanel payload={zulu}/>);

    await page.getByRole('button', {name: 'Customize your view'}).click();
    await page.getByRole('button', {name: 'Remove Yokota'}).click();
    await page.getByRole('button', {name: 'Save'}).click();

    await expect(page.getByText('091630ZAUG26')).toBeVisible();
    await expect(page.getByRole('button', {name: 'Save'})).toHaveCount(0);
    await expect(page.locator('tbody tr')).toHaveCount(DISPLAY_ZONES.length - 1);
});

test('restoring defaults returns to the DTG', async ({mount, page}) => {
    await stubPreferencesRoute(page, {stored: {zones: [{iana: 'UTC'}], urgentWithinMinutes: 5}});
    await mount(<DtgPanel payload={zulu}/>);

    await expect(page.locator('tbody tr')).toHaveCount(1);

    await page.getByRole('button', {name: 'Customize your view'}).click();
    await page.getByRole('button', {name: 'Restore defaults'}).click();

    await expect(page.getByText('091630ZAUG26')).toBeVisible();
    await expect(page.locator('tbody tr')).toHaveCount(DISPLAY_ZONES.length);
});

test('the back link returns to the DTG unchanged', async ({mount, page}) => {
    await stubPreferencesRoute(page);
    await mount(<DtgPanel payload={zulu}/>);

    await page.getByRole('button', {name: 'Customize your view'}).click();
    await page.getByRole('button', {name: 'Remove Yokota'}).click();
    await page.getByRole('button', {name: 'Back'}).click();

    await expect(page.getByText('091630ZAUG26')).toBeVisible();
    await expect(page.locator('tbody tr')).toHaveCount(DISPLAY_ZONES.length);
});

// The panel is one of the three places the built-in documentation is
// advertised, and the only one a reader who never opens the System Console will
// ever see.
test('offers the documentation beside the editor link', async ({mount, page}) => {
    await stubPreferencesRoute(page);
    await mount(<DtgPanel payload={zulu}/>);

    const docs = page.getByRole('link', {name: 'Documentation'});
    await expect(docs).toBeVisible();

    // Served by Mattermost straight out of the bundle, so the path has to be the
    // plugin's own public directory and not a route in the server code.
    const href = await docs.getAttribute('href');
    expect(href).toContain('/public/help/help.html');
    expect(href).toContain('/plugins/');

    // A new tab: the sidebar lives inside the app, and navigating it away would
    // lose the reader's place in the channel.
    await expect(docs).toHaveAttribute('target', '_blank');
    await expect(docs).toHaveAttribute('rel', 'noopener noreferrer');
});

// It is a real destination, so it has to survive being opened deliberately
// rather than only being clicked.
test('the documentation link is an anchor, not a button', async ({mount, page}) => {
    await stubPreferencesRoute(page);
    await mount(<DtgPanel payload={zulu}/>);

    const tag = await page.getByRole('link', {name: 'Documentation'}).evaluate((el) => el.tagName);
    expect(tag).toBe('A');
});

// The same quiet-until-pointed-at treatment as every other link here. Inline
// styles cannot express :hover, so this is driven from React state and would be
// easy to lose on the anchor branch without noticing.
test('underlines the documentation link on hover and on focus', async ({mount, page}) => {
    await stubPreferencesRoute(page);
    await mount(<DtgPanel payload={zulu}/>);

    const docs = page.getByRole('link', {name: 'Documentation'});
    expect(await decorationOf(docs)).toBe('none');

    await docs.hover();
    await expect.poll(() => decorationOf(docs)).toBe('underline');

    await page.mouse.move(0, 0);
    await expect.poll(() => decorationOf(docs)).toBe('none');

    await docs.focus();
    await expect.poll(() => decorationOf(docs)).toBe('underline');
});

// Opening the editor replaces the whole panel, so the footer goes with it. A
// documentation link left floating over the editor would be a second way out of
// a view that already has Back, Save and Restore defaults.
test('the footer does not follow the reader into the editor', async ({mount, page}) => {
    await stubPreferencesRoute(page);
    await mount(<DtgPanel payload={zulu}/>);

    await expect(page.getByRole('link', {name: 'Documentation'})).toBeVisible();

    await page.getByRole('button', {name: 'Customize your view'}).click();

    await expect(page.getByRole('link', {name: 'Documentation'})).toHaveCount(0);
});
