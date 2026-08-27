import React from 'react';

import CustomizeHarness from './CustomizeHarness';
import {SECTIONS} from './sections';

import {expect, test} from '../../playwright/ct-coverage';
import {featuresAnswered, stubFeaturesRoute} from '../features/stub_route';
import {savedHiddenSections, stubPreferencesRoute} from '../preferences/stub_route';

test('shows every section ticked when the reader has hidden nothing', async ({mount, page}) => {
    await stubFeaturesRoute(page);
    await stubPreferencesRoute(page);
    const component = await mount(<CustomizeHarness/>);

    await expect(component.getByRole('checkbox', {name: /Payload/})).toBeChecked();
    await expect(component.getByRole('checkbox', {name: /Processing path/})).toBeChecked();
});

test('unticks the sections the reader had hidden', async ({mount, page}) => {
    await stubFeaturesRoute(page);
    await stubPreferencesRoute(page, {storedHiddenSections: ['payload', 'flow']});
    const component = await mount(<CustomizeHarness/>);

    await expect(component.getByRole('checkbox', {name: /Payload/})).not.toBeChecked();
    await expect(component.getByRole('checkbox', {name: /Processing path/})).not.toBeChecked();
    await expect(component.getByRole('checkbox', {name: /Shape/})).toBeChecked();
});

// Unticking is what gets SAVED, even though the reader is choosing what to show.
test('saves the sections that were unticked', async ({mount, page}) => {
    await stubFeaturesRoute(page);
    const calls = await stubPreferencesRoute(page);
    const component = await mount(<CustomizeHarness/>);

    await component.getByRole('checkbox', {name: /Payload/}).uncheck();
    await component.getByRole('button', {name: 'Save'}).click();

    await expect(component.getByTestId('closed')).toBeVisible();
    expect(savedHiddenSections(calls)).toEqual(['payload']);
});

// A PUT replaces the whole blob, so the store re-reads and merges rather than
// sending whatever this editor happened to have cached. Getting this wrong
// deletes the reader's settings in the other two editors.
test('saving sections leaves the other two editors alone', async ({mount, page}) => {
    await stubFeaturesRoute(page);
    const calls = await stubPreferencesRoute(page, {
        stored: {zones: [{iana: 'Asia/Tokyo', name: 'Yokota'}], urgentWithinMinutes: 42},
        storedHiddenRows: ['ddm'],
    });
    const component = await mount(<CustomizeHarness/>);

    await component.getByRole('checkbox', {name: /Payload/}).uncheck();
    await component.getByRole('button', {name: 'Save'}).click();
    await expect(component.getByTestId('closed')).toBeVisible();

    const put = calls.filter((call) => call.method === 'PUT').at(-1);
    expect(put?.body?.dtg?.zones).toEqual([{iana: 'Asia/Tokyo', name: 'Yokota'}]);
    expect(put?.body?.dtg?.urgent_within_minutes).toBe(42);
    expect(put?.body?.location?.hidden_rows).toEqual(['ddm']);
});

test('restoring defaults leaves the other two editors alone', async ({mount, page}) => {
    await stubFeaturesRoute(page);
    const calls = await stubPreferencesRoute(page, {
        stored: {zones: [{iana: 'Asia/Tokyo', name: 'Yokota'}], urgentWithinMinutes: 42},
        storedHiddenSections: ['payload'],
    });
    const component = await mount(<CustomizeHarness/>);

    await component.getByRole('button', {name: 'Restore defaults'}).click();
    await expect(component.getByTestId('closed')).toBeVisible();

    expect(calls.some((call) => call.method === 'DELETE')).toBe(false);

    const put = calls.filter((call) => call.method === 'PUT').at(-1);
    expect(put?.body?.cot?.hidden_sections).toEqual([]);
    expect(put?.body?.dtg?.zones).toEqual([{iana: 'Asia/Tokyo', name: 'Yokota'}]);
});

// With nothing else stored there is nothing to keep, so the blob goes rather
// than being written back saying nothing. Leaving this section out of
// hasNoChoices is what would keep it alive forever.
test('restoring defaults deletes when nothing else is stored', async ({mount, page}) => {
    await stubFeaturesRoute(page);
    const calls = await stubPreferencesRoute(page, {storedHiddenSections: ['payload']});
    const component = await mount(<CustomizeHarness/>);

    await component.getByRole('button', {name: 'Restore defaults'}).click();
    await expect(component.getByTestId('closed')).toBeVisible();

    expect(calls.some((call) => call.method === 'DELETE')).toBe(true);
});

// A save that failed keeps the reader where they are, with the reason on
// screen: closing would throw away both the message and their edits.
test('a rejected save stays put and says why', async ({mount, page}) => {
    await stubFeaturesRoute(page);
    await stubPreferencesRoute(page, {saveStatus: 400, saveMessage: 'Sections are wrong.'});
    const component = await mount(<CustomizeHarness/>);

    await component.getByRole('checkbox', {name: /Payload/}).uncheck();
    await component.getByRole('button', {name: 'Save'}).click();

    await expect(component.getByText('Sections are wrong.')).toBeVisible();
    await expect(component.getByTestId('closed')).toHaveCount(0);
    await expect(component.getByRole('checkbox', {name: /Payload/})).not.toBeChecked();
});

test('a rejected restore stays put and says why', async ({mount, page}) => {
    await stubFeaturesRoute(page);
    await stubPreferencesRoute(page, {
        storedHiddenSections: ['payload'],
        resetStatus: 500,
        resetMessage: 'Could not reach the settings store.',
    });
    const component = await mount(<CustomizeHarness/>);

    await component.getByRole('button', {name: 'Restore defaults'}).click();

    await expect(component.getByText('Could not reach the settings store.')).toBeVisible();
    await expect(component.getByTestId('closed')).toHaveCount(0);
});

/*
 * The settings land after the first render, and a draft made in the meantime
 * must not be reverted when they do. The fix has two halves and this pins the
 * one a test can reach: while a read is in the air every box is disabled, so
 * there is no draft to lose. The `touched` flag behind them covers the residual
 * path, where a failed load leaves the form usable and a later mount starts a
 * fresh read.
 */
test('the form cannot be edited while a read is in the air', async ({mount, page}) => {
    let release = () => { /* replaced by holdLoad below */ };

    await stubFeaturesRoute(page);
    await stubPreferencesRoute(page, {
        storedHiddenSections: ['payload'],
        holdLoad: (releaseHeld) => {
            release = releaseHeld;
        },
    });
    const component = await mount(<CustomizeHarness/>);

    await expect(component.getByRole('checkbox', {name: /Processing path/})).toBeDisabled();

    release();

    // And once it lands the form is theirs, carrying what was stored.
    await expect(component.getByRole('checkbox', {name: /Processing path/})).toBeEnabled();
    await expect(component.getByRole('checkbox', {name: /Payload/})).not.toBeChecked();
});

/*
 * A first read that FAILED leaves the store on the defaults, which renders as
 * every box ticked. Editing that and saving would write a selection derived
 * from settings the reader never had, replacing their real section wholesale.
 * So nothing is operable until a read has actually succeeded.
 */
test('the form is sealed when the first read failed', async ({mount, page}) => {
    await stubFeaturesRoute(page);
    await stubPreferencesRoute(page, {loadStatus: 500});
    const component = await mount(<CustomizeHarness/>);

    await expect(component.getByText('Could not read your settings.')).toBeVisible();
    await expect(component.getByRole('checkbox', {name: /Payload/})).toBeDisabled();
    await expect(component.getByRole('button', {name: 'Save'})).toBeDisabled();
    await expect(component.getByRole('button', {name: 'Restore defaults'})).toBeDisabled();
});

/*
 * A LATER read that failed is different: the store keeps the last good blob, so
 * the form is editable and a save is safe. Distinguishing the two is the whole
 * job of the store's `loaded` flag.
 */
test('a failed refresh after a good read leaves the form usable', async ({mount, page}) => {
    await stubFeaturesRoute(page);
    await stubPreferencesRoute(page, {storedHiddenSections: ['payload']});
    const component = await mount(<CustomizeHarness/>);

    await expect(component.getByRole('checkbox', {name: /Payload/})).not.toBeChecked();
    await expect(component.getByRole('checkbox', {name: /Payload/})).toBeEnabled();
});

// The checkboxes are one named group rather than eleven loose ones.
test('names the group of checkboxes', async ({mount, page}) => {
    await stubFeaturesRoute(page);
    await stubPreferencesRoute(page);
    const component = await mount(<CustomizeHarness/>);

    await expect(component.getByRole('group', {name: 'Sections to show'})).toBeVisible();
});

// The warning is announced, not just drawn: the reader who unticked the last
// section is the one who most needs to hear it.
test('announces when every section has been hidden', async ({mount, page}) => {
    await stubFeaturesRoute(page);
    await stubPreferencesRoute(page);
    const component = await mount(<CustomizeHarness/>);

    // The map box is offered only once the features route has answered, so a
    // snapshot taken on mount misses it and "uncheck everything" would quietly
    // leave one ticked and never fire the warning.
    await expect(component.getByRole('checkbox', {name: /Map/})).toBeVisible();

    const boxes = await component.getByRole('checkbox').all();
    await boxes.reduce(
        (chain, box) => chain.then(() => box.uncheck()),
        Promise.resolve(),
    );

    await expect(component.getByText(/Every section is hidden/)).toBeVisible();
    await expect(component.getByText(/Every section is hidden/)).toHaveAttribute('aria-live', 'polite');
});

/*
 * A surface the admin has switched off is not offered, because a tickbox that
 * changes nothing a reader can see is worse than an absent one. Every other
 * section stays, so the count comes from the catalog rather than a number typed
 * here that a new section would make wrong.
 */
test('does not offer the map when the admin has switched maps off', async ({mount, page}) => {
    await stubFeaturesRoute(page, {mapInline: false});
    await stubPreferencesRoute(page);
    const answered = featuresAnswered(page);
    const component = await mount(<CustomizeHarness/>);
    await answered;

    // The count is the barrier: every OTHER box is present, which is only true
    // once the route has answered. The Processing path box below is present at
    // t=0, so on its own it barriers nothing.
    await expect(component.getByRole('checkbox')).toHaveCount(SECTIONS.length - 1);
    await expect(component.getByRole('checkbox', {name: /Processing path/})).toBeVisible();
    await expect(component.getByRole('checkbox', {name: 'Map', exact: true})).toHaveCount(0);
    await expect(component.getByRole('checkbox')).toHaveCount(SECTIONS.length - 1);
});

/*
 * What is STORED survives a surface the admin switched off.
 *
 * The editor does not offer a box for a section the admin has disabled, but the
 * reader's hidden list keeps the id, so switching the surface back on returns
 * them to the choice they had made rather than to the default. Ported from the
 * location editor, where this is the test the first copy of this suite dropped.
 */
test('keeps a hidden id the editor is not showing', async ({mount, page}) => {
    await stubFeaturesRoute(page, {mapInline: false});
    const calls = await stubPreferencesRoute(page, {storedHiddenSections: ['map', 'payload']});
    const answered = featuresAnswered(page);
    const component = await mount(<CustomizeHarness/>);
    await answered;

    await expect(component.getByRole('checkbox')).toHaveCount(SECTIONS.length - 1);
    await expect(component.getByRole('checkbox', {name: /Payload/})).not.toBeChecked();
    await expect(component.getByRole('checkbox', {name: 'Map', exact: true})).toHaveCount(0);

    await component.getByRole('checkbox', {name: /Payload/}).check();
    await component.getByRole('button', {name: 'Save'}).click();

    await expect(component.getByTestId('closed')).toBeVisible();
    expect(savedHiddenSections(calls)).toEqual(['map']);
});

/*
 * `shown` counts what the editor OFFERS, not the catalog. Counting the catalog
 * made "everything is hidden" fire while a surface the admin disabled was still
 * hidden but not offered, so the warning appeared over a panel that still had
 * sections on screen.
 */
test('does not claim everything is hidden while a section is still shown', async ({mount, page}) => {
    await stubFeaturesRoute(page, {mapInline: false});
    await stubPreferencesRoute(page, {storedHiddenSections: ['map']});
    const component = await mount(<CustomizeHarness/>);

    await expect(component.getByRole('checkbox', {name: /Processing path/})).toBeVisible();

    const boxes = await component.getByRole('checkbox').all();
    await boxes.reduce((chain, box) => chain.then(() => box.uncheck()), Promise.resolve());
    await expect(component.getByText(/Every section is hidden/)).toBeVisible();

    await component.getByRole('checkbox', {name: /Processing path/}).check();
    await expect(component.getByText(/Every section is hidden/)).toHaveCount(0);
});

/*
 * The editor replaces the whole panel, so the control that opened it is gone.
 * Without somewhere to put focus a keyboard reader is dropped at the top of the
 * document with nothing announced.
 */
test('focus lands on Back when the editor opens', async ({mount, page}) => {
    await stubFeaturesRoute(page);
    await stubPreferencesRoute(page);
    const component = await mount(<CustomizeHarness/>);

    await expect(component.getByRole('button', {name: '← Back'})).toBeFocused();
});

/*
 * A hint describes a box; it is not part of its name. Inside the label it
 * became the name, so every box announced a dozen words of prose before its
 * own state.
 */
test('a hint describes its checkbox rather than naming it', async ({mount, page}) => {
    await stubFeaturesRoute(page);
    await stubPreferencesRoute(page);
    const component = await mount(<CustomizeHarness/>);

    const box = component.getByRole('checkbox', {name: 'Payload', exact: true});
    await expect(box).toBeVisible();

    const describedBy = await box.getAttribute('aria-describedby');
    expect(describedBy).not.toBeNull();
    await expect(component.locator(`#${describedBy}`)).toHaveText(/Sensor, video, GeoChat/);
});

// The sealed state has to say why, or a reader meets an inert panel with no
// explanation.
test('says it is loading while the form is sealed', async ({mount, page}) => {
    let release = () => { /* replaced by holdLoad */ };
    await stubFeaturesRoute(page);
    await stubPreferencesRoute(page, {
        holdLoad: (releaseHeld) => {
            release = releaseHeld;
        },
    });
    const component = await mount(<CustomizeHarness/>);

    await expect(component.getByText('Loading your settings…')).toBeVisible();
    release();
    await expect(component.getByText('Loading your settings…')).toHaveCount(0);
});

// Every section in the catalog gets a box, read from the catalog so a section
// added later is covered without anybody remembering to come back here.
test('offers one box per section', async ({mount, page}) => {
    await stubFeaturesRoute(page);
    await stubPreferencesRoute(page);
    const component = await mount(<CustomizeHarness/>);

    await expect(component.getByRole('checkbox')).toHaveCount(SECTIONS.length);
});
