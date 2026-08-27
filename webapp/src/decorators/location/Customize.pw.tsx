import React from 'react';

import CustomizeHarness from './CustomizeHarness';

import {expect, test} from '../../../playwright/ct-coverage';
import {stubFeaturesRoute} from '../../features/stub_route';
import {savedHiddenRows, stubPreferencesRoute} from '../../preferences/stub_route';

/*
 * The location editor had no component test at all, which is how a
 * draft-clobbering effect and a "Restore defaults" that deleted another
 * decorator's settings both shipped. These are the two behaviors that were
 * wrong, plus the ordinary ones.
 */

test('shows every row ticked when the reader has hidden nothing', async ({mount, page}) => {
    await stubFeaturesRoute(page);
    await stubPreferencesRoute(page);
    const component = await mount(<CustomizeHarness/>);

    await expect(component.getByRole('checkbox', {name: /MGRS/})).toBeChecked();
    await expect(component.getByRole('checkbox', {name: /Datum/})).toBeChecked();
});

test('unticks the rows the reader had hidden', async ({mount, page}) => {
    await stubFeaturesRoute(page);
    await stubPreferencesRoute(page, {storedHiddenRows: ['ddm', 'datum']});
    const component = await mount(<CustomizeHarness/>);

    await expect(component.getByRole('checkbox', {name: /DDM/})).not.toBeChecked();
    await expect(component.getByRole('checkbox', {name: /Datum/})).not.toBeChecked();
    await expect(component.getByRole('checkbox', {name: /MGRS/})).toBeChecked();
});

// Unticking is what gets SAVED, even though the reader is choosing what to show.
test('saves the rows that were unticked', async ({mount, page}) => {
    await stubFeaturesRoute(page);
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
    await stubFeaturesRoute(page);
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
    await stubFeaturesRoute(page);
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
    await stubFeaturesRoute(page);
    const calls = await stubPreferencesRoute(page, {storedHiddenRows: ['ddm']});
    const component = await mount(<CustomizeHarness/>);

    await component.getByRole('button', {name: 'Restore defaults'}).click();
    await expect(component.getByTestId('closed')).toBeVisible();

    expect(calls.some((call) => call.method === 'DELETE')).toBe(true);
});

// A save that failed keeps the reader where they are, with the reason on
// screen: closing would throw away both the message and their edits.
test('a rejected save stays put and says why', async ({mount, page}) => {
    await stubFeaturesRoute(page);
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
    await stubFeaturesRoute(page);
    await stubPreferencesRoute(page);
    const component = await mount(<CustomizeHarness/>);

    // By position rather than by name: several hints mention another row's
    // label, so a name regex matches more than one box.
    // The two map boxes are offered only once the features route has answered,
    // so a snapshot taken on mount misses them and "uncheck everything" would
    // quietly leave two ticked and never fire the warning.
    await expect(component.getByRole('checkbox', {name: 'Map under the post'})).toBeVisible();

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
    await stubFeaturesRoute(page);
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
    await stubFeaturesRoute(page);
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

/*
 * The map under a post is hideable and is not a row, so it is carried by its own
 * id exactly as the panel's map is. Two maps under one legend, and the editor
 * has to keep them apart in both directions.
 */
test.describe('the map under a post', () => {
    test('is offered beside the panel map', async ({mount, page}) => {
        await stubFeaturesRoute(page);
        await stubPreferencesRoute(page);
        const component = await mount(<CustomizeHarness/>);

        await expect(component.getByRole('checkbox', {name: 'Map', exact: true})).toBeChecked();
        await expect(component.getByRole('checkbox', {name: 'Map under the post'})).toBeChecked();
    });

    test('unticks on its own without taking the panel map with it', async ({mount, page}) => {
        await stubFeaturesRoute(page);
        await stubPreferencesRoute(page, {storedHiddenRows: ['inline']});
        const component = await mount(<CustomizeHarness/>);

        await expect(component.getByRole('checkbox', {name: 'Map under the post'})).not.toBeChecked();
        await expect(component.getByRole('checkbox', {name: 'Map', exact: true})).toBeChecked();
    });

    /*
     * `offered` counts what this editor offers rather than ROWS alone, and the
     * count only goes wrong in one direction: leaving the new id out of it made
     * everything-but-the-inline-map read as nothing shown, so the warning fired
     * over a reader who still had a map in the channel. Unticking every box
     * cannot catch that, because then the id is hidden either way.
     */
    test('does not claim everything is hidden while it is still shown', async ({mount, page}) => {
        await stubFeaturesRoute(page);
        await stubPreferencesRoute(page);
        const component = await mount(<CustomizeHarness/>);

        // The two map boxes are offered only once the features route has answered,
        // so a snapshot taken on mount misses them and "uncheck everything" would
        // quietly leave two ticked and never fire the warning.
        await expect(component.getByRole('checkbox', {name: 'Map under the post'})).toBeVisible();

        const boxes = await component.getByRole('checkbox').all();
        await boxes.reduce(
            (chain, box) => chain.then(() => box.uncheck()),
            Promise.resolve(),
        );
        await expect(component.getByText(/Every row is hidden/)).toBeVisible();

        await component.getByRole('checkbox', {name: 'Map under the post'}).check();

        await expect(component.getByText(/Every row is hidden/)).toHaveCount(0);
    });

    test('saves under its own id', async ({mount, page}) => {
        await stubFeaturesRoute(page);
        const calls = await stubPreferencesRoute(page);
        const component = await mount(<CustomizeHarness/>);

        await component.getByRole('checkbox', {name: 'Map under the post'}).uncheck();
        await component.getByRole('button', {name: 'Save'}).click();

        await expect(component.getByTestId('closed')).toBeVisible();
        expect(savedHiddenRows(calls)).toEqual(['inline']);
    });
});

/*
 * A surface the admin has switched off is not offered, because a tick box that
 * changes nothing the reader can see is worse than an absent one: they untick
 * it, save, and nothing happens.
 *
 * What is STORED is untouched, which is the half worth pinning. A reader's
 * hidden list keeps ids this editor is not showing, so turning a switch off and
 * back on returns them to the choice they had made rather than to the default.
 */
test.describe('when the admin has turned a map surface off', () => {
    test('offers no tick box for the panel map', async ({mount, page}) => {
        await stubFeaturesRoute(page, {mapPanel: false});
        await stubPreferencesRoute(page);
        const component = await mount(<CustomizeHarness/>);

        await expect(component.getByRole('checkbox', {name: 'Map under the post'})).toBeVisible();
        await expect(component.getByRole('checkbox', {name: 'Map', exact: true})).toHaveCount(0);
    });

    test('offers no tick box for the map under a post', async ({mount, page}) => {
        await stubFeaturesRoute(page, {mapInline: false});
        await stubPreferencesRoute(page);
        const component = await mount(<CustomizeHarness/>);

        await expect(component.getByRole('checkbox', {name: 'Map', exact: true})).toBeVisible();
        await expect(component.getByRole('checkbox', {name: 'Map under the post'})).toHaveCount(0);
    });

    test('offers neither when maps are off entirely', async ({mount, page}) => {
        await stubFeaturesRoute(page, {mapPanel: false, mapInline: false, mapPage: false});
        await stubPreferencesRoute(page);
        const component = await mount(<CustomizeHarness/>);

        await expect(component.getByRole('checkbox', {name: /MGRS/})).toBeVisible();
        await expect(component.getByRole('checkbox', {name: 'Map', exact: true})).toHaveCount(0);
        await expect(component.getByRole('checkbox', {name: 'Map under the post'})).toHaveCount(0);
    });

    // The reader's own choice survives, so the switch coming back does not
    // silently unhide something they had hidden.
    test('keeps a hidden id the editor is not showing', async ({mount, page}) => {
        await stubFeaturesRoute(page, {mapInline: false});
        const calls = await stubPreferencesRoute(page, {storedHiddenRows: ['inline', 'ddm']});
        const component = await mount(<CustomizeHarness/>);

        await expect(component.getByRole('checkbox', {name: /DDM/})).not.toBeChecked();

        await component.getByRole('checkbox', {name: /DDM/}).check();
        await component.getByRole('button', {name: 'Save'}).click();

        await expect(component.getByTestId('closed')).toBeVisible();
        expect(savedHiddenRows(calls)).toEqual(['inline']);
    });
});
