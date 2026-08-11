import React from 'react';

import {ALL, TOTAL_ZONES} from './zone_fixtures';
import ZonePickerHarness from './ZonePickerHarness';

import {expect, test} from '../../../playwright/ct-coverage';

// Customize.pw.tsx already drives the picker in place: filtering by name,
// identifier and offset, multi-term search, the count, arrow keys, Enter,
// Escape, blur, and the query surviving a pick. None of that is repeated here.
//
// What is left is what the editor never asks for: the disabled state, an empty
// or emptied list, and the keyboard edges either side of the list.

const TOTAL = TOTAL_ZONES;

async function openList(page: import('@playwright/test').Page) {
    await page.getByRole('combobox', {name: 'Search timezones'}).focus();
    await expect(page.getByRole('listbox')).toBeVisible();
}

// The presses have to land one after another, so they are chained rather than
// awaited in a loop.
function pressRepeatedly(
    input: import('@playwright/test').Locator,
    key: string,
    times: number,
): Promise<void> {
    return Array.from({length: times}).reduce<Promise<void>>(
        (chain) => chain.then(() => input.press(key)),
        Promise.resolve(),
    );
}

test.describe('disabled', () => {
    // This is how the editor enforces its row limit and how it goes inert while
    // a save is in flight, so it has to be genuinely unusable rather than just
    // greyed out.
    test('cannot be focused, so the list never opens', async ({mount, page}) => {
        await mount(<ZonePickerHarness disabled={true}/>);

        const input = page.getByRole('combobox', {name: 'Search timezones'});
        await expect(input).toBeDisabled();

        await input.focus({timeout: 1000}).catch(() => undefined);

        await expect(page.getByRole('listbox')).toHaveCount(0);
        await expect(page.getByTestId('pick-count')).toHaveText('0');
    });

    test('cannot be typed into', async ({mount, page}) => {
        await mount(<ZonePickerHarness disabled={true}/>);

        await page.getByRole('combobox', {name: 'Search timezones'}).
            fill('ramstein', {force: true, timeout: 1000}).catch(() => undefined);

        await expect(page.getByRole('listbox')).toHaveCount(0);
        await expect(page.getByTestId('pick-count')).toHaveText('0');
    });
});

test.describe('an empty list', () => {
    test('renders nothing to choose from', async ({mount, page}) => {
        await mount(<ZonePickerHarness groups='none'/>);

        await page.getByRole('combobox', {name: 'Search timezones'}).focus();

        await expect(page.getByRole('listbox')).toHaveCount(0);
    });

    // ArrowDown clamps against `flat.length - 1`, which is -1 with nothing to
    // move through. Enter must stay inert rather than picking `undefined`.
    test('arrow keys and Enter do nothing', async ({mount, page}) => {
        await mount(<ZonePickerHarness groups='none'/>);

        const input = page.getByRole('combobox', {name: 'Search timezones'});
        await input.focus();
        await input.press('ArrowDown');
        await input.press('Enter');

        await expect(page.getByTestId('pick-count')).toHaveText('0');
        await expect(input).not.toHaveAttribute('aria-activedescendant', /.*/);
    });

    // A group with no zones left is still handed to the picker by this harness,
    // which is what the identity path does when there is no query: the header
    // renders with nothing under it. Pinned because the search path prunes such
    // a group and the no-query path does not.
    test('an empty group still shows its header', async ({mount, page}) => {
        await mount(<ZonePickerHarness groups='empty-first-group'/>);

        await openList(page);

        await expect(page.locator('[data-group="Bases and common zones"]')).toBeVisible();
        await expect(page.getByRole('option')).toHaveCount(ALL.length);
    });
});

test.describe('the keyboard edges', () => {
    // ArrowDown reopens a closed list; ArrowUp deliberately does not, so a
    // reader stepping back up out of the list does not reopen it.
    test('ArrowUp does not reopen a closed list', async ({mount, page}) => {
        await mount(<ZonePickerHarness/>);

        const input = page.getByRole('combobox', {name: 'Search timezones'});
        await openList(page);
        await input.press('Escape');
        await expect(page.getByRole('listbox')).toHaveCount(0);

        await input.press('ArrowUp');
        await expect(page.getByRole('listbox')).toHaveCount(0);

        await input.press('ArrowDown');
        await expect(page.getByRole('listbox')).toBeVisible();
    });

    // Enter is only swallowed when there is something to pick with it.
    // Otherwise it has to pass through, or the panel around this could never be
    // submitted from the picker.
    test('Enter on a closed list picks nothing', async ({mount, page}) => {
        await mount(<ZonePickerHarness/>);

        const input = page.getByRole('combobox', {name: 'Search timezones'});
        await openList(page);
        await input.press('Escape');

        await input.press('Enter');

        await expect(page.getByTestId('pick-count')).toHaveText('0');
    });

    test('ArrowDown stops at the last option', async ({mount, page}) => {
        await mount(<ZonePickerHarness/>);

        const input = page.getByRole('combobox', {name: 'Search timezones'});
        await openList(page);

        await pressRepeatedly(input, 'ArrowDown', TOTAL + 3);

        await expect(page.getByRole('option').last()).toHaveAttribute('data-active', 'true');
    });

    test('ArrowUp stops at the first option', async ({mount, page}) => {
        await mount(<ZonePickerHarness/>);

        const input = page.getByRole('combobox', {name: 'Search timezones'});
        await openList(page);

        await input.press('ArrowDown');
        await pressRepeatedly(input, 'ArrowUp', TOTAL + 3);

        await expect(page.getByRole('option').first()).toHaveAttribute('data-active', 'true');
    });
});

test.describe('the pointer', () => {
    // Hovering moves the active option, so the keyboard carries on from wherever
    // the pointer left off rather than from where it had got to before.
    test('hovering an option makes it the one Enter picks', async ({mount, page}) => {
        await mount(<ZonePickerHarness/>);
        await openList(page);

        const yokota = page.getByRole('option', {name: /Yokota/});
        await yokota.hover();
        await expect(yokota).toHaveAttribute('aria-selected', 'true');

        await page.getByRole('combobox', {name: 'Search timezones'}).press('Enter');

        await expect(page.getByTestId('picked')).toHaveText('Asia/Tokyo|Yokota');
    });

    test('clicking an option picks it', async ({mount, page}) => {
        await mount(<ZonePickerHarness/>);
        await openList(page);

        await page.getByRole('option', {name: /Ramstein/}).click();

        await expect(page.getByTestId('picked')).toHaveText('Europe/Berlin|Ramstein');
        await expect(page.getByRole('listbox')).toHaveCount(0);
    });

    // Two bases sharing a zone are separate rows and both can be taken, which
    // is why identity is the pair rather than the identifier alone.
    test('both bases in one zone can be picked', async ({mount, page}) => {
        await mount(<ZonePickerHarness/>);
        await openList(page);
        await page.getByRole('option', {name: /Ramstein/}).click();

        await page.getByRole('combobox', {name: 'Search timezones'}).press('ArrowDown');
        await page.getByRole('option', {name: /Stuttgart/}).click();

        await expect(page.getByTestId('picked')).
            toHaveText('Europe/Berlin|Ramstein,Europe/Berlin|Stuttgart');
    });

    // The list shrinks by one under the cursor, so the active index is held
    // rather than allowed to drift down it.
    test('picking the last option leaves the highlight in range', async ({mount, page}) => {
        await mount(<ZonePickerHarness/>);
        await openList(page);

        await page.getByRole('option').last().click();
        await page.getByRole('combobox', {name: 'Search timezones'}).press('ArrowDown');

        await expect(page.getByRole('option')).toHaveCount(TOTAL - 1);
        await expect(page.locator('[data-active="true"]')).toHaveCount(1);
    });
});

test.describe('the query', () => {
    // filter(Boolean) drops the empty terms a whitespace-only query splits into,
    // so nothing is filtered, but the status line still appears because the
    // query is not empty.
    test('a whitespace-only query filters nothing', async ({mount, page}) => {
        await mount(<ZonePickerHarness/>);

        await page.getByRole('combobox', {name: 'Search timezones'}).fill('   ');

        await expect(page.getByRole('option')).toHaveCount(TOTAL);
        await expect(page.getByText(`${TOTAL} matching.`)).toBeVisible();
    });

    test('says nothing at all until something is typed', async ({mount, page}) => {
        await mount(<ZonePickerHarness/>);
        await openList(page);

        await expect(page.getByText('matching.')).toHaveCount(0);
        await expect(page.getByText('Nothing matches that.')).toHaveCount(0);
    });

    // Filtering the list empty leaves nothing to render, so aria-expanded is
    // true with no listbox to point at. Pinned as it behaves.
    test('a filter that empties the list closes it', async ({mount, page}) => {
        await mount(<ZonePickerHarness/>);

        const input = page.getByRole('combobox', {name: 'Search timezones'});
        await input.fill('nowhere at all');

        await expect(page.getByRole('listbox')).toHaveCount(0);
        await expect(page.getByText('Nothing matches that.')).toBeVisible();
        await expect(input).toHaveAttribute('aria-expanded', 'true');
    });
});

// useId gives each instance its own ids, or two pickers on one page would point
// their aria-controls and aria-activedescendant at each other's options.
test('two pickers do not share their option ids', async ({mount, page}) => {
    await mount(<ZonePickerHarness twice={true}/>);

    const boxes = page.getByRole('combobox', {name: 'Search timezones'});
    await expect(boxes).toHaveCount(2);

    const first = await boxes.nth(0).getAttribute('aria-controls');
    const second = await boxes.nth(1).getAttribute('aria-controls');

    expect(first).not.toBe(second);
});
