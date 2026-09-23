import React from 'react';

import {EXAMPLES_COMMAND} from './examples';
import {MAX_HISTORY} from './history';
import {EXAMPLES_WARNING, HISTORY_EMPTY, HISTORY_HEADING, SETTINGS_HEADING, SETTINGS_SECTIONS} from './Home';
import RhsHarness from './RhsHarness';

import {expect, test} from '../../../playwright/ct-coverage';
import {stubPreferencesRoute} from '../../preferences/stub_route';

test('shows the home view with no selection', async ({mount, page}) => {
    await mount(<RhsHarness/>);

    await expect(page.getByTestId('tf-home')).toBeVisible();
    await expect(page.getByRole('heading', {name: HISTORY_HEADING})).toBeVisible();
    await expect(page.getByText(HISTORY_EMPTY)).toBeVisible();
    await expect(page.getByRole('heading', {name: SETTINGS_HEADING})).toBeVisible();
    await expect(page.getByRole('link', {name: 'Documentation'})).toBeVisible();
    await expect(page.getByTestId('fixture-panel')).toHaveCount(0);
});

test('lists what was opened, newest first, once each, and reopens it', async ({mount, page}) => {
    await mount(<RhsHarness opened={['alpha', 'bravo', 'alpha']}/>);

    await expect(page.getByTestId('tf-home-history').getByRole('listitem')).toHaveText(['Fixture alpha', 'Fixture bravo']);

    await page.getByRole('button', {name: 'Fixture bravo'}).click();
    await expect(page.getByTestId('fixture-panel')).toHaveText('bravo');
});

test('keeps no more than the history cap', async ({mount, page}) => {
    const opened = Array.from({length: MAX_HISTORY + 3}, (_, i) => `item ${i}`);
    await mount(<RhsHarness opened={opened}/>);

    await expect(page.getByTestId('tf-home-history').getByRole('listitem')).toHaveCount(MAX_HISTORY);
    await expect(page.getByTestId('tf-home-history').getByRole('listitem').first()).toHaveText(`Fixture item ${MAX_HISTORY + 2}`);
});

for (const section of SETTINGS_SECTIONS) {
    test(`opens the ${section.label} settings without a link and comes back`, async ({mount, page}) => {
        await stubPreferencesRoute(page);
        await mount(<RhsHarness/>);

        await page.getByRole('button', {name: section.label, exact: true}).click();
        await expect(page.getByRole('heading', {name: `${SETTINGS_HEADING}: ${section.label}`})).toBeVisible();
        await page.getByRole('button', {name: /Back/}).click();
        await expect(page.getByTestId('tf-home')).toBeVisible();
    });
}

test('posts the examples only after it is confirmed', async ({mount, page}) => {
    const posted: unknown[] = [];
    await page.route('**/api/v4/commands/execute', async (route) => {
        posted.push(route.request().postDataJSON());
        await route.fulfill({status: 200, contentType: 'application/json', body: '{}'});
    });
    await mount(<RhsHarness channelId='channel1'/>);

    await page.getByRole('button', {name: /Post the examples/}).click();
    await expect(page.getByText(EXAMPLES_WARNING)).toBeVisible();
    await page.getByRole('button', {name: 'Cancel'}).click();
    expect(posted).toHaveLength(0);

    await page.getByRole('button', {name: /Post the examples/}).click();
    await page.getByRole('button', {name: 'Post them'}).click();
    await expect(page.getByText('Posted to this channel.')).toBeVisible();
    expect(posted).toEqual([{channel_id: 'channel1', team_id: 'team1', command: EXAMPLES_COMMAND}]);
});

test('says why the examples were not posted', async ({mount, page}) => {
    await page.route('**/api/v4/commands/execute', (route) => route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({text: 'The examples do not fit in a post on this server. (TF-16004)'}),
    }));
    await mount(<RhsHarness channelId='channel1'/>);

    await page.getByRole('button', {name: /Post the examples/}).click();
    await page.getByRole('button', {name: 'Post them'}).click();
    await expect(page.getByRole('alert')).toHaveText('The examples do not fit in a post on this server. (TF-16004)');
});

test('renders the selected decorator panel', async ({mount, page}) => {
    await mount(
        <RhsHarness
            selectionType='fix'
            value='hello'
        />,
    );

    await expect(page.getByTestId('fixture-panel')).toHaveText('hello');
});

test('falls back to the home view for an unknown type', async ({mount, page}) => {
    await mount(<RhsHarness selectionType='not-registered'/>);

    await expect(page.getByTestId('tf-home')).toBeVisible();
    await expect(page.getByTestId('fixture-panel')).toHaveCount(0);
});

test('title is null-safe with no selection', async ({mount, page}) => {
    await mount(<RhsHarness title={true}/>);

    await expect(page.getByText('Tactical Fusion')).toBeVisible();
});

test('title uses the decorator summary', async ({mount, page}) => {
    await mount(
        <RhsHarness
            selectionType='fix'
            value='hello'
            title={true}
        />,
    );

    await expect(page.getByText('Fixture hello')).toBeVisible();
});

// A panel with more than one view cannot drive its header through summary,
// which is a pure function of the payload and cannot see which view is up.
test('title prefers a header component when the decorator declares one', async ({mount, page}) => {
    await mount(
        <RhsHarness
            selectionType='fix'
            value='hello'
            title={true}
            withTitle={true}
        />,
    );

    await expect(page.getByTestId('fixture-title')).toHaveText('Titled hello');
    await expect(page.getByText('Fixture hello')).toHaveCount(0);
});

// The header component is optional, exactly like the hover card.
test('title falls back to the summary without one', async ({mount, page}) => {
    await mount(
        <RhsHarness
            selectionType='fix'
            value='hello'
            title={true}
        />,
    );

    await expect(page.getByTestId('fixture-title')).toHaveCount(0);
    await expect(page.getByText('Fixture hello')).toBeVisible();
});

test('title falls back for an unknown type', async ({mount, page}) => {
    await mount(
        <RhsHarness
            selectionType='not-registered'
            title={true}
        />,
    );

    await expect(page.getByText('Tactical Fusion')).toBeVisible();
});
