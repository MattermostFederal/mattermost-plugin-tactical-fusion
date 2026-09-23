import React from 'react';

import RhsHarness from './RhsHarness';

import {expect, test} from '../../../playwright/ct-coverage';

test('shows the empty state with no selection', async ({mount, page}) => {
    await mount(<RhsHarness/>);

    await expect(page.getByText('Tactical Fusion')).toBeVisible();
    await expect(page.getByTestId('fixture-panel')).toHaveCount(0);
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

// A stale link, or a selection from a decorator this bundle does not ship, must
// not blow up the sidebar.
test('falls back to the empty state for an unknown type', async ({mount, page}) => {
    await mount(<RhsHarness selectionType='not-registered'/>);

    await expect(page.getByText('Tactical Fusion')).toBeVisible();
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
