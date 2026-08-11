import type {Page} from '@playwright/test';
import React from 'react';

import Customize from './Customize';
import {DEFAULT_ZONE_IDS} from './zones';

import {expect, test} from '../../../playwright/ct-coverage';
import {savedEntries, savedMinutes, savedZones, stubPreferencesRoute} from '../../preferences/stub_route';

const instant = new Date(Date.UTC(2026, 7, 9, 16, 30, 0));

/**
 * For the tests that are not about closing.
 *
 * Leaving the editor mounted after a save is what lets them read the status
 * line, which the real panel replaces the moment it closes.
 */
function noop() {}

// A reader who has chosen nothing is editing the defaults, so the editor shows
// the rows they can actually see rather than an empty list.
test('starts from the default timezones', async ({mount, page}) => {
    await stubPreferencesRoute(page);
    await mount(<Customize
        instant={instant}
        onClose={noop}
                />);

    await expect(page.getByRole('button', {name: /^Remove /})).toHaveCount(DEFAULT_ZONE_IDS.length);
    await expect(page.getByRole('button', {name: 'Remove Yokota'})).toBeVisible();
});

test('starts from a saved selection when there is one', async ({mount, page}) => {
    await stubPreferencesRoute(page, {
        stored: {zones: [{iana: 'UTC', name: 'Zulu (UTC)'}, {iana: 'Europe/Paris'}], urgentWithinMinutes: 5},
    });
    await mount(<Customize
        instant={instant}
        onClose={noop}
                />);

    await expect(page.getByRole('button', {name: /^Remove /})).toHaveCount(2);
    await expect(page.getByRole('button', {name: 'Remove Zulu (UTC)'})).toBeVisible();
    await expect(page.getByRole('button', {name: 'Remove Paris'})).toBeVisible();
    await expect(page.getByRole('spinbutton')).toHaveValue('5');
});

test('removes a timezone and saves the shorter list', async ({mount, page}) => {
    const calls = await stubPreferencesRoute(page);
    await mount(<Customize
        instant={instant}
        onClose={noop}
                />);

    await page.getByRole('button', {name: 'Remove Yokota'}).click();
    await page.getByRole('button', {name: 'Save'}).click();

    await expect(page.getByRole('status')).toHaveText('Saved.');
    expect(savedZones(calls)).toEqual(DEFAULT_ZONE_IDS.filter((id) => id !== 'Asia/Tokyo'));
});

// The whole point of "select your own timezones": the picker is not limited to
// the nine locations this plugin ships with.
test('adds a timezone the plugin does not ship with', async ({mount, page}) => {
    const calls = await stubPreferencesRoute(page);
    await mount(<Customize
        instant={instant}
        onClose={noop}
                />);

    await openPicker(page);
    await choose(page, '(UTC+02:00) Europe/Paris');
    await expect(page.getByRole('button', {name: 'Remove Paris'})).toBeVisible();

    await page.getByRole('button', {name: 'Save'}).click();

    await expect(page.getByRole('status')).toHaveText('Saved.');
    expect(savedZones(calls)).toEqual([...DEFAULT_ZONE_IDS, 'Europe/Paris']);
});

/** Opens the picker's dropdown by focusing it. */
async function openPicker(page: Page): Promise<void> {
    await page.getByRole('combobox', {name: 'Search timezones'}).click();
}

/**
 * Every option the picker is offering, in order.
 *
 * React appends several hundred of them one at a time, so a naive read catches
 * the list half built and misses everything east of wherever it got to.
 * UTC+14:00 is the largest offset there is, so its arrival at the end means the
 * list is whole.
 */
async function pickerLabels(page: Page): Promise<string[]> {
    const options = page.getByRole('option');
    await expect(options.last()).toHaveText(/^\(UTC\+14:00\) /);

    return options.allTextContents();
}

/** Types a query into the picker and waits for the list to settle on it. */
async function search(page: Page, query: string): Promise<void> {
    await page.getByRole('combobox', {name: 'Search timezones'}).fill(query);
}

/**
 * The labels in the first group, which is the named catalogue.
 *
 * The listbox is flat, with headers as siblings of the options, so the group is
 * whatever lies between one header and the next.
 */
async function basesGroup(page: Page): Promise<string[]> {
    return page.getByRole('listbox').evaluate((list) => {
        const labels: string[] = [];
        let inside = false;

        for (const child of Array.from(list.children)) {
            if (child.hasAttribute('data-group')) {
                if (inside) {
                    break;
                }
                inside = child.getAttribute('data-group') === 'Bases and common zones';
                continue;
            }
            if (inside) {
                labels.push(child.textContent ?? '');
            }
        }

        return labels;
    });
}

/** Chooses an option by its exact label. */
async function choose(page: Page, label: string): Promise<void> {
    await page.getByRole('option', {name: label, exact: true}).click();
}

/**
 * Waits for the saved selection to arrive.
 *
 * The editor renders the defaults until the first read lands, so a test with a
 * stored selection that reads straight away is racing it, and under load loses.
 */
async function loaded(page: Page, zones: number): Promise<void> {
    await expect(page.getByRole('button', {name: /^Remove /})).toHaveCount(zones);
}

/** The whole editor row a Remove button sits in. */
async function rowFor(page: Page, name: string): Promise<string> {
    return page.getByRole('button', {name: `Remove ${name}`}).
        evaluate((button) => button.parentElement?.textContent ?? '');
}

/** `(UTC+05:30) Asia/Kolkata` back to 330. */
function labelledOffset(label: string): number {
    const match = (/^\(UTC([+-])(\d{2}):(\d{2})\)/).exec(label);
    if (!match) {
        throw new Error(`option is not labelled with an offset: ${label}`);
    }

    const minutes = (Number(match[2]) * 60) + Number(match[3]);
    return match[1] === '-' ? -minutes : minutes;
}

// Picking a zone by where it sits relative to UTC is the point of the ordering,
// and it needs the offset visible to be usable.
test('the picker names each zone with its offset', async ({mount, page}) => {
    await stubPreferencesRoute(page);
    await mount(<Customize
        instant={instant}
        onClose={noop}
                />);

    await openPicker(page);
    const labels = await pickerLabels(page);

    // Paris rather than one of the built-in zones: those are already selected,
    // so the picker rightly does not offer them.
    expect(labels).toContain('(UTC+02:00) Europe/Paris');

    // Half-hour and three-quarter-hour zones exist, so the minutes are not a
    // formality. Matched by shape, since which identifier a given engine
    // considers canonical is its own business.
    expect(labels.some((label) => label.startsWith('(UTC+05:30) '))).toBe(true);
    expect(labels.some((label) => label.startsWith('(UTC+05:45) '))).toBe(true);
});

// Several hundred identifiers with no grouping would bury the bases this
// plugin exists to serve.
test('the picker leads with bases before the full list', async ({mount, page}) => {
    await stubPreferencesRoute(page);
    await mount(<Customize
        instant={instant}
        onClose={noop}
                />);

    await openPicker(page);

    const groups = await page.locator('[data-group]').
        evaluateAll((nodes) => nodes.map((node) => node.getAttribute('data-group')));

    expect(groups).toEqual(['Bases and common zones', 'All timezones']);
});

test('the bases group names the installation, not just the city', async ({mount, page}) => {
    await stubPreferencesRoute(page);
    await mount(<Customize
        instant={instant}
        onClose={noop}
                />);

    await openPicker(page);
    await expect(page.getByRole('option').first()).toBeVisible();

    const labels = await basesGroup(page);
    expect(labels).toContain('(UTC+02:00) Aviano AB (Europe/Rome)');
    expect(labels).toContain('(UTC+03:00) NSA Bahrain (Asia/Bahrain)');
});

// Zulu is only offerable when it is not already selected, which the defaults
// make it.
test('does not say the same thing twice in a label', async ({mount, page}) => {
    await stubPreferencesRoute(page, {stored: {zones: [{iana: 'Asia/Tokyo'}], urgentWithinMinutes: 0}});
    await mount(<Customize
        instant={instant}
        onClose={noop}
                />);

    await loaded(page, 1);

    await openPicker(page);

    const labels = await basesGroup(page);
    expect(labels).toContain('(UTC+00:00) Zulu (UTC)');
    expect(labels.some((label) => label.includes('(UTC) (UTC)'))).toBe(false);

    // ...and an unnamed zone is not padded out with a repeat of its own city.
    expect(await pickerLabels(page)).toContain('(UTC+02:00) Europe/Paris');
});

test('a base can be added and reads by its own name', async ({mount, page}) => {
    const calls = await stubPreferencesRoute(page, {
        stored: {zones: [{iana: 'UTC'}], urgentWithinMinutes: 0},
    });
    await mount(<Customize
        instant={instant}
        onClose={noop}
                />);

    await loaded(page, 1);
    await openPicker(page);
    await choose(page, '(UTC+06:00) Diego Garcia (Indian/Chagos)');

    await expect(page.getByRole('button', {name: 'Remove Diego Garcia'})).toBeVisible();

    await page.getByRole('button', {name: 'Save'}).click();
    await expect(page.getByRole('status')).toHaveText('Saved.');

    // The name is stored with the zone, which is what lets the row read
    // "Diego Garcia" rather than "Chagos" next time it loads.
    expect(savedEntries(calls)).toEqual([
        {iana: 'UTC'},
        {iana: 'Indian/Chagos', name: 'Diego Garcia'},
    ]);
});

test('the picker runs west to east', async ({mount, page}) => {
    await stubPreferencesRoute(page);
    await mount(<Customize
        instant={instant}
        onClose={noop}
                />);

    await openPicker(page);

    const labels = await pickerLabels(page);
    expect(labels.length).toBeGreaterThan(100);

    // Each group runs west to east on its own, so the sequence restarts between
    // them. Compared as numbers, since "UTC+01:00" sorts before "UTC-11:00" as
    // text and a string comparison would pass on a list in no order at all.
    const bases = await basesGroup(page);
    const rest = labels.slice(bases.length);

    for (const group of [bases, rest]) {
        const offsets = group.map(labelledOffset);
        expect(offsets.length).toBeGreaterThan(1);
        for (let i = 1; i < offsets.length; i++) {
            expect(offsets[i]).toBeGreaterThanOrEqual(offsets[i - 1]);
        }
    }
});

// The editor lists the same zones the table above does, so it has to read in
// the same order or the two look like different lists.
test('the chosen zones run west to east, with their offsets', async ({mount, page}) => {
    await stubPreferencesRoute(page, {
        stored: {
            zones: [
                {iana: 'Asia/Tokyo', name: 'Yokota'},
                {iana: 'Pacific/Honolulu', name: 'Honolulu'},
                {iana: 'UTC', name: 'Zulu (UTC)'},
            ],
            urgentWithinMinutes: 0,
        },
    });
    await mount(<Customize
        instant={instant}
        onClose={noop}
                />);

    const rows = page.getByRole('button', {name: /^Remove /});
    await expect(rows).toHaveCount(3);

    expect(await rows.evaluateAll((buttons) => buttons.map((button) => button.getAttribute('aria-label')))).
        toEqual(['Remove Honolulu', 'Remove Zulu (UTC)', 'Remove Yokota']);

    expect(await rowFor(page, 'Honolulu')).toContain('UTC-10:00');
    expect(await rowFor(page, 'Yokota')).toContain('UTC+09:00');
});

// Removing has to key off the identifier: the rows are ordered by offset, which
// is not the order the selection is held in.
test('removes the zone that was asked for, not the one in that position', async ({mount, page}) => {
    const calls = await stubPreferencesRoute(page, {
        stored: {zones: [{iana: 'Asia/Tokyo'}, {iana: 'Pacific/Honolulu'}, {iana: 'UTC'}], urgentWithinMinutes: 0},
    });
    await mount(<Customize
        instant={instant}
        onClose={noop}
                />);

    await loaded(page, 3);

    // First row on screen, last in the stored order.
    await page.getByRole('button', {name: 'Remove Honolulu'}).click();
    await page.getByRole('button', {name: 'Save'}).click();

    await expect(page.getByRole('status')).toHaveText('Saved.');
    expect(savedZones(calls)).toEqual(['Asia/Tokyo', 'UTC']);
});

// Otherwise opening the editor and pressing Save would freeze the reader's
// table at today's defaults for good.
test('saving an unchanged list stores nothing', async ({mount, page}) => {
    const calls = await stubPreferencesRoute(page);
    await mount(<Customize
        instant={instant}
        onClose={noop}
                />);

    await page.getByRole('button', {name: 'Save'}).click();

    await expect(page.getByRole('status')).toHaveText('Saved.');
    expect(savedZones(calls)).toEqual([]);
});

test('saves a flash threshold in minutes', async ({mount, page}) => {
    const calls = await stubPreferencesRoute(page);
    await mount(<Customize
        instant={instant}
        onClose={noop}
                />);

    await page.getByRole('spinbutton').fill('10');
    await page.getByRole('button', {name: 'Save'}).click();

    await expect(page.getByRole('status')).toHaveText('Saved.');
    expect(savedMinutes(calls)).toBe(10);
});

// Blank is not an error, it is how a reader goes back to the default.
test('clearing the threshold means the default', async ({mount, page}) => {
    const calls = await stubPreferencesRoute(page, {stored: {zones: [], urgentWithinMinutes: 45}});
    await mount(<Customize
        instant={instant}
        onClose={noop}
                />);

    await expect(page.getByRole('spinbutton')).toHaveValue('45');
    await page.getByRole('spinbutton').fill('');
    await page.getByRole('button', {name: 'Save'}).click();

    await expect(page.getByRole('status')).toHaveText('Saved.');
    expect(savedMinutes(calls)).toBe(0);
});

// The server rejects it too, but bouncing off the network to say so would be a
// worse way to tell the reader they typed the wrong thing.
test('rejects an impossible threshold without asking the server', async ({mount, page}) => {
    const calls = await stubPreferencesRoute(page);
    await mount(<Customize
        instant={instant}
        onClose={noop}
                />);

    await page.getByRole('spinbutton').fill('5000');
    await page.getByRole('button', {name: 'Save'}).click();

    await expect(page.getByRole('status')).toContainText('between 1 and 1440');
    expect(calls.filter((call) => call.method === 'PUT')).toHaveLength(0);
});

// "Restore defaults" deletes the blob rather than writing today's defaults into
// it, so the reader goes back to tracking whatever the defaults become.
test('restoring defaults deletes the saved settings', async ({mount, page}) => {
    const calls = await stubPreferencesRoute(page, {
        stored: {zones: [{iana: 'UTC'}, {iana: 'Europe/Paris'}], urgentWithinMinutes: 5},
    });
    await mount(<Customize
        instant={instant}
        onClose={noop}
                />);

    await expect(page.getByRole('button', {name: /^Remove /})).toHaveCount(2);

    await page.getByRole('button', {name: 'Restore defaults'}).click();

    await expect(page.getByRole('status')).toHaveText('Defaults restored.');
    expect(calls.some((call) => call.method === 'DELETE')).toBe(true);

    // The editor goes back to showing the defaults, which is what the panel is
    // now rendering.
    await expect(page.getByRole('button', {name: /^Remove /})).toHaveCount(DEFAULT_ZONE_IDS.length);
    await expect(page.getByRole('spinbutton')).toHaveValue('');
});

// A save that quietly did nothing would leave the reader believing their
// settings had been kept.
test('shows the reason the server gave for a rejected save', async ({mount, page}) => {
    await stubPreferencesRoute(page, {
        saveStatus: 400,
        saveMessage: 'unknown timezone "Mars/Olympus_Mons"',
    });
    await mount(<Customize
        instant={instant}
        onClose={noop}
                />);

    await page.getByRole('button', {name: 'Save'}).click();

    await expect(page.getByRole('status')).toHaveText('unknown timezone "Mars/Olympus_Mons"');
});

// A save closes the editor: the changed table behind it is the receipt, so the
// reader does not have to find their own way back.
test('closes after a save', async ({mount, page}) => {
    await stubPreferencesRoute(page);

    let closed = 0;
    await mount(<Customize
        instant={instant}
        onClose={() => {
            closed++;
        }}
                />);

    await page.getByRole('button', {name: 'Save'}).click();

    await expect.poll(() => closed).toBe(1);
});

test('closes after restoring the defaults', async ({mount, page}) => {
    await stubPreferencesRoute(page);

    let closed = 0;
    await mount(<Customize
        instant={instant}
        onClose={() => {
            closed++;
        }}
                />);

    await page.getByRole('button', {name: 'Restore defaults'}).click();

    await expect.poll(() => closed).toBe(1);
});

// Closing on a failure would throw away both the reason and the reader's edits.
test('stays put when the save is rejected', async ({mount, page}) => {
    await stubPreferencesRoute(page, {saveStatus: 400, saveMessage: 'Rejected.'});

    let closed = 0;
    await mount(<Customize
        instant={instant}
        onClose={() => {
            closed++;
        }}
                />);

    await page.getByRole('button', {name: 'Save'}).click();

    await expect(page.getByRole('status')).toHaveText('Rejected.');
    expect(closed).toBe(0);
});

// Saving is not the only way out. Without this a reader who opened the editor
// by accident would be stuck in it.
test('the back link leaves without saving', async ({mount, page}) => {
    const calls = await stubPreferencesRoute(page);

    let closed = 0;
    await mount(<Customize
        instant={instant}
        onClose={() => {
            closed++;
        }}
                />);

    await page.getByRole('button', {name: 'Back'}).click();

    await expect.poll(() => closed).toBe(1);
    expect(calls.filter((call) => call.method !== 'GET')).toHaveLength(0);
});

test('says so when the settings could not be loaded', async ({mount, page}) => {
    await stubPreferencesRoute(page, {loadStatus: 500});
    await mount(<Customize
        instant={instant}
        onClose={noop}
                />);

    await expect(page.getByRole('status')).toHaveText('Could not read your settings.');

    // ...and still offers the defaults to edit, rather than an empty editor.
    await expect(page.getByRole('button', {name: /^Remove /})).toHaveCount(DEFAULT_ZONE_IDS.length);
});

// Two installations sharing a zone are two rows, each under its own name. The
// clocks agree, which is the accepted cost of naming both.
test('two bases in one zone can both be chosen', async ({mount, page}) => {
    const calls = await stubPreferencesRoute(page, {
        stored: {zones: [{iana: 'Europe/Berlin', name: 'Ramstein'}], urgentWithinMinutes: 0},
    });
    await mount(<Customize
        instant={instant}
        onClose={noop}
                />);

    await loaded(page, 1);
    await openPicker(page);
    await choose(page, '(UTC+02:00) USAG Stuttgart (Europe/Berlin)');

    await expect(page.getByRole('button', {name: 'Remove Ramstein'})).toBeVisible();
    await expect(page.getByRole('button', {name: 'Remove USAG Stuttgart'})).toBeVisible();

    await page.getByRole('button', {name: 'Save'}).click();
    await expect(page.getByRole('status')).toHaveText('Saved.');

    expect(savedEntries(calls)).toEqual([
        {iana: 'Europe/Berlin', name: 'Ramstein'},
        {iana: 'Europe/Berlin', name: 'USAG Stuttgart'},
    ]);
});

// Choosing Ramstein must not take Stuttgart out of the picker with it.
test('choosing one base leaves the others in its zone offerable', async ({mount, page}) => {
    await stubPreferencesRoute(page, {
        stored: {zones: [{iana: 'Europe/Berlin', name: 'Ramstein'}], urgentWithinMinutes: 0},
    });
    await mount(<Customize
        instant={instant}
        onClose={noop}
                />);

    await loaded(page, 1);
    await openPicker(page);
    const labels = await pickerLabels(page);

    expect(labels).toContain('(UTC+02:00) USAG Stuttgart (Europe/Berlin)');
    expect(labels).not.toContain('(UTC+02:00) Ramstein (Europe/Berlin)');
});

// Removing has to match the whole identity, or removing Ramstein would take
// Stuttgart with it.
test('removing one base leaves the other in its zone', async ({mount, page}) => {
    const calls = await stubPreferencesRoute(page, {
        stored: {
            zones: [
                {iana: 'Europe/Berlin', name: 'Ramstein'},
                {iana: 'Europe/Berlin', name: 'USAG Stuttgart'},
            ],
            urgentWithinMinutes: 0,
        },
    });
    await mount(<Customize
        instant={instant}
        onClose={noop}
                />);

    await loaded(page, 2);
    await page.getByRole('button', {name: 'Remove Ramstein'}).click();

    await expect(page.getByRole('button', {name: 'Remove USAG Stuttgart'})).toBeVisible();

    await page.getByRole('button', {name: 'Save'}).click();
    await expect(page.getByRole('status')).toHaveText('Saved.');
    expect(savedEntries(calls)).toEqual([{iana: 'Europe/Berlin', name: 'USAG Stuttgart'}]);
});

/** Mounts the editor with nothing saved, ready to search. */
async function mountForSearch(mount: (c: React.JSX.Element) => Promise<unknown>): Promise<void> {
    await mount(<Customize
        instant={instant}
        onClose={noop}
                />);
}

/** The options currently offered, however many groups they span. */
function offered(page: Page) {
    return page.getByRole('option');
}

// Several hundred options behind a select that offsets have robbed of its
// typeahead. Searching is the only way to find anything.
test('filters the picker by base name', async ({mount, page}) => {
    await stubPreferencesRoute(page);
    await mountForSearch(mount);

    await search(page, 'sigonella');

    const labels = await offered(page).allTextContents();
    expect(labels).toEqual(['(UTC+02:00) NAS Sigonella (Europe/Rome)']);
});

test('filters by identifier, spelled either way', async ({mount, page}) => {
    await stubPreferencesRoute(page);
    await mountForSearch(mount);

    await search(page, 'america/argentina');
    expect((await offered(page).allTextContents()).join(' ')).toContain('Argentina');

    // The separators opened out, which is how people actually type a city.
    await search(page, 'buenos aires');
    expect((await offered(page).allTextContents()).join(' ')).toContain('Buenos_Aires');
});

test('filters by offset', async ({mount, page}) => {
    await stubPreferencesRoute(page);
    await mountForSearch(mount);

    await search(page, '+05:45');

    const labels = await offered(page).allTextContents();
    expect(labels.length).toBeGreaterThan(0);
    for (const label of labels) {
        expect(label).toContain('UTC+05:45');
    }
});

// "raf uk" and "berlin ram" are the queries people actually type, and a single
// substring would find neither.
test('every term has to match, in any order', async ({mount, page}) => {
    await stubPreferencesRoute(page);
    await mountForSearch(mount);

    await search(page, 'berlin spang');

    const labels = await offered(page).allTextContents();
    expect(labels).toEqual(['(UTC+02:00) Spangdahlem AB (Europe/Berlin)']);
});

test('ignores case', async ({mount, page}) => {
    await stubPreferencesRoute(page);
    await mountForSearch(mount);

    // Not one of the defaults, which are already chosen and so not offered.
    await search(page, 'sPaNgDaHlEm');

    expect(await offered(page).allTextContents()).toContain('(UTC+02:00) Spangdahlem AB (Europe/Berlin)');
});

test('says so when nothing matches', async ({mount, page}) => {
    await stubPreferencesRoute(page);
    await mountForSearch(mount);

    await search(page, 'mars');

    await expect(page.getByText('Nothing matches that.')).toBeVisible();
    await expect(offered(page)).toHaveCount(0);
});

test('counts what it found', async ({mount, page}) => {
    await stubPreferencesRoute(page);
    await mountForSearch(mount);

    await search(page, 'sigonella');

    await expect(page.getByText('1 matching.')).toBeVisible();
});

// Adding two bases from one search is the obvious thing to want, so the query
// survives the pick even though the list closes over it.
test('keeps the query after adding, and drops what was added', async ({mount, page}) => {
    await stubPreferencesRoute(page);
    await mountForSearch(mount);

    await search(page, 'europe/berlin');
    await choose(page, '(UTC+02:00) USAG Stuttgart (Europe/Berlin)');

    await expect(page.getByRole('combobox', {name: 'Search timezones'})).toHaveValue('europe/berlin');
    await expect(page.getByRole('button', {name: 'Remove USAG Stuttgart'})).toBeVisible();

    // Closed, so it cannot sit over the buttons below.
    await expect(offered(page)).toHaveCount(0);

    // One arrow key brings the rest of that same search back.
    await combobox(page).press('ArrowDown');
    await expect(offered(page).first()).toBeVisible();

    const labels = await offered(page).allTextContents();
    expect(labels).not.toContain('(UTC+02:00) USAG Stuttgart (Europe/Berlin)');
    expect(labels).toContain('(UTC+02:00) Spangdahlem AB (Europe/Berlin)');
});

test('clearing the query brings the whole list back', async ({mount, page}) => {
    await stubPreferencesRoute(page);
    await mountForSearch(mount);

    await search(page, 'sigonella');
    await expect(offered(page)).toHaveCount(1);

    await search(page, '');

    await expect(page.getByText('matching.')).toHaveCount(0);
    expect((await pickerLabels(page)).length).toBeGreaterThan(100);
});

// The groups are what make the bases findable at all, so filtering must not
// flatten them.
test('keeps the grouping while filtered', async ({mount, page}) => {
    await stubPreferencesRoute(page);
    await mountForSearch(mount);

    await search(page, 'tokyo');

    const groups = await page.locator('[data-group]').
        evaluateAll((nodes) => nodes.map((node) => node.getAttribute('data-group')));

    expect(groups).toEqual(['Bases and common zones', 'All timezones']);
});

/** The picker's text box. */
function combobox(page: Page) {
    return page.getByRole('combobox', {name: 'Search timezones'});
}

// Filtered to Europe/Berlin, the list is Spangdahlem, then Stuttgart, then the
// bare zone: two named bases sharing a zone, ordered by the name tiebreak.
test('arrow keys move through the list and Enter picks', async ({mount, page}) => {
    await stubPreferencesRoute(page);
    await mountForSearch(mount);

    await search(page, 'europe/berlin');
    await expect(offered(page).first()).toBeVisible();

    await combobox(page).press('ArrowDown');
    await combobox(page).press('Enter');

    await expect(page.getByRole('button', {name: 'Remove USAG Stuttgart'})).toBeVisible();
    await expect(page.getByRole('button', {name: 'Remove Spangdahlem AB'})).toHaveCount(0);
});

test('ArrowUp goes back the way it came', async ({mount, page}) => {
    await stubPreferencesRoute(page);
    await mountForSearch(mount);

    await search(page, 'europe/berlin');
    await expect(offered(page).first()).toBeVisible();

    await combobox(page).press('ArrowDown');
    await combobox(page).press('ArrowDown');
    await combobox(page).press('ArrowUp');
    await combobox(page).press('Enter');

    await expect(page.getByRole('button', {name: 'Remove USAG Stuttgart'})).toBeVisible();
});

// Whatever is active has to be announced, or a screen reader follows nothing as
// the arrow keys move.
test('marks the active option for assistive technology', async ({mount, page}) => {
    await stubPreferencesRoute(page);
    await mountForSearch(mount);

    await search(page, 'europe/berlin');
    await combobox(page).press('ArrowDown');

    const activeId = await combobox(page).getAttribute('aria-activedescendant');
    expect(activeId).not.toBeNull();

    // The one the input points at is the one marked selected, and it is the one
    // the arrow key moved to.
    const active = page.locator('[aria-selected="true"]');
    await expect(active).toHaveAttribute('id', activeId!);
    await expect(active).toHaveText(/USAG Stuttgart/);
});

test('Escape closes the list without choosing anything', async ({mount, page}) => {
    await stubPreferencesRoute(page);
    await mountForSearch(mount);

    await search(page, 'sigonella');
    await expect(offered(page)).toHaveCount(1);
    await expect(combobox(page)).toHaveAttribute('aria-expanded', 'true');

    await combobox(page).press('Escape');

    await expect(offered(page)).toHaveCount(0);
    await expect(combobox(page)).toHaveAttribute('aria-expanded', 'false');
    await expect(page.getByRole('button', {name: 'Remove NAS Sigonella'})).toHaveCount(0);
});

test('closes when the reader moves on', async ({mount, page}) => {
    await stubPreferencesRoute(page);
    await mountForSearch(mount);

    await search(page, 'sigonella');
    await expect(offered(page)).toHaveCount(1);

    await combobox(page).press('Tab');

    await expect(offered(page)).toHaveCount(0);
    await expect(page.getByRole('button', {name: 'Remove NAS Sigonella'})).toHaveCount(0);
});

// The list is taller than its box, so an active option the reader cannot see
// makes the arrow keys look broken.
test('scrolls the active option into view', async ({mount, page}) => {
    await stubPreferencesRoute(page);
    await mountForSearch(mount);

    await combobox(page).click();
    await expect(offered(page).first()).toBeVisible();

    for (let i = 0; i < 25; i++) {
        // eslint-disable-next-line no-await-in-loop
        await combobox(page).press('ArrowDown');
    }

    await expect(page.locator('[data-active="true"]')).toBeInViewport();
});

// The sidebar header already says where this is, so repeating it at the top of
// the panel just spends a line saying the same thing twice.
test('does not repeat the header inside the panel', async ({mount, page}) => {
    await stubPreferencesRoute(page);
    await mountForSearch(mount);

    await expect(page.getByText('Customize your view')).toHaveCount(0);

    // ...and the way out is still there.
    await expect(page.getByRole('button', {name: 'Back'})).toBeVisible();
});
