import React from 'react';

import CustomizeHarness from './CustomizeHarness';

import {expect, test} from '../../../playwright/ct-coverage';
import {savedHiddenRows, stubPreferencesRoute} from '../../preferences/stub_route';

/*
 * The location editor had no component test at all, which is how a
 * draft-clobbering effect and a "Restore defaults" that deleted another
 * decorator's settings both shipped. These are the two behaviours that were
 * wrong, plus the ordinary ones.
 */

test('shows every row ticked when the reader has hidden nothing', async ({mount, page}) => {
    await stubPreferencesRoute(page);
    const component = await mount(<CustomizeHarness/>);

    await expect(component.getByRole('checkbox', {name: /MGRS/})).toBeChecked();
    await expect(component.getByRole('checkbox', {name: /Datum/})).toBeChecked();
});

test('unticks the rows the reader had hidden', async ({mount, page}) => {
    await stubPreferencesRoute(page, {storedHiddenRows: ['ddm', 'datum']});
    const component = await mount(<CustomizeHarness/>);

    await expect(component.getByRole('checkbox', {name: /DDM/})).not.toBeChecked();
    await expect(component.getByRole('checkbox', {name: /Datum/})).not.toBeChecked();
    await expect(component.getByRole('checkbox', {name: /MGRS/})).toBeChecked();
});

// Unticking is what gets SAVED, even though the reader is choosing what to show.
test('saves the rows that were unticked', async ({mount, page}) => {
    const calls = await stubPreferencesRoute(page);
    const component = await mount(<CustomizeHarness/>);

    await component.getByRole('checkbox', {name: /DDM/}).uncheck();
    await component.getByRole('button', {name: 'Save'}).click();

    await expect(component.getByTestId('closed')).toBeVisible();
    expect(savedHiddenRows(calls)).toEqual(['ddm']);
});

// The save must not carry the reader's timezone settings away with it. A PUT
// replaces the whole blob, so the store re-reads and merges rather than sending
// whatever this editor happened to have cached.
test('saving rows leaves the timezone settings alone', async ({mount, page}) => {
    const calls = await stubPreferencesRoute(page, {
        stored: {zones: [{iana: 'Asia/Tokyo', name: 'Yokota'}], urgentWithinMinutes: 42},
    });
    const component = await mount(<CustomizeHarness/>);

    await component.getByRole('checkbox', {name: /DDM/}).uncheck();
    await component.getByRole('button', {name: 'Save'}).click();
    await expect(component.getByTestId('closed')).toBeVisible();

    const put = calls.filter((call) => call.method === 'PUT').at(-1);
    expect(put?.body?.dtg?.zones).toEqual([{iana: 'Asia/Tokyo', name: 'Yokota'}]);
    expect(put?.body?.dtg?.urgent_within_minutes).toBe(42);
});

// And neither must "Restore defaults", which used to DELETE the whole blob from
// under a legend reading "Rows to show".
test('restoring defaults leaves the timezone settings alone', async ({mount, page}) => {
    const calls = await stubPreferencesRoute(page, {
        stored: {zones: [{iana: 'Asia/Tokyo', name: 'Yokota'}], urgentWithinMinutes: 42},
        storedHiddenRows: ['ddm'],
    });
    const component = await mount(<CustomizeHarness/>);

    await component.getByRole('button', {name: 'Restore defaults'}).click();
    await expect(component.getByTestId('closed')).toBeVisible();

    expect(calls.some((call) => call.method === 'DELETE')).toBe(false);

    const put = calls.filter((call) => call.method === 'PUT').at(-1);
    expect(put?.body?.location?.hidden_rows).toEqual([]);
    expect(put?.body?.dtg?.zones).toEqual([{iana: 'Asia/Tokyo', name: 'Yokota'}]);
});

// With nothing else stored there is nothing to keep, so the blob goes rather
// than being written back saying nothing.
test('restoring defaults deletes when nothing else is stored', async ({mount, page}) => {
    const calls = await stubPreferencesRoute(page, {storedHiddenRows: ['ddm']});
    const component = await mount(<CustomizeHarness/>);

    await component.getByRole('button', {name: 'Restore defaults'}).click();
    await expect(component.getByTestId('closed')).toBeVisible();

    expect(calls.some((call) => call.method === 'DELETE')).toBe(true);
});

// A save that failed keeps the reader where they are, with the reason on
// screen: closing would throw away both the message and their edits.
test('a rejected save stays put and says why', async ({mount, page}) => {
    await stubPreferencesRoute(page, {saveStatus: 400, saveMessage: 'Rows are wrong.'});
    const component = await mount(<CustomizeHarness/>);

    await component.getByRole('checkbox', {name: /DDM/}).uncheck();
    await component.getByRole('button', {name: 'Save'}).click();

    await expect(component.getByText('Rows are wrong.')).toBeVisible();
    await expect(component.getByTestId('closed')).toHaveCount(0);
    await expect(component.getByRole('checkbox', {name: /DDM/})).not.toBeChecked();
});

// The warning is announced, not just drawn: the reader who unticked the last
// row is the one who most needs to hear it.
test('announces when every row has been hidden', async ({mount, page}) => {
    await stubPreferencesRoute(page);
    const component = await mount(<CustomizeHarness/>);

    // By position rather than by name: several hints mention another row's
    // label, so a name regex matches more than one box.
    const boxes = await component.getByRole('checkbox').all();
    await boxes.reduce(
        (chain, box) => chain.then(() => box.uncheck()),
        Promise.resolve(),
    );

    await expect(component.getByText(/Every row is hidden/)).toBeVisible();
    await expect(component.getByText(/Every row is hidden/)).toHaveAttribute('aria-live', 'polite');
});

// The checkboxes are one named group rather than eleven loose ones.
test('names the group of checkboxes', async ({mount, page}) => {
    await stubPreferencesRoute(page);
    const component = await mount(<CustomizeHarness/>);

    await expect(component.getByRole('group', {name: 'Rows to show'})).toBeVisible();
});

/*
 * A failed "Restore defaults" behaves like a failed save and for the same
 * reason: closing on a write that did not happen would tell the reader their
 * rows are back when they are not, and the receipt for the write landing is the
 * changed table behind.
 */
test('a rejected restore stays put and says why', async ({mount, page}) => {
    await stubPreferencesRoute(page, {
        storedHiddenRows: ['ddm'],
        resetStatus: 500,
        resetMessage: 'Could not reach the settings store.',
    });
    const component = await mount(<CustomizeHarness/>);

    await component.getByRole('button', {name: 'Restore defaults'}).click();

    await expect(component.getByText('Could not reach the settings store.')).toBeVisible();
    await expect(component.getByTestId('closed')).toHaveCount(0);
});
