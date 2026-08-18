import manifest from 'manifest';
import React from 'react';

import IndexHarness, {type Recorded} from './IndexHarness';

import {expect, test} from '../playwright/ct-coverage';

// The entry point registers itself at import time and wires everything else in
// initialize(), so it can only be exercised in a browser. IndexHarness drives it
// against a recording registry and reports what it saw.

async function recorded(page: import('@playwright/test').Page): Promise<Recorded> {
    const node = page.getByTestId('index-harness-result');
    await expect(node).not.toBeEmpty();
    return JSON.parse((await node.textContent()) ?? '{}');
}

test('registers the plugin under the manifest id', async ({mount, page}) => {
    await mount(<IndexHarness/>);

    const result = await recorded(page);

    expect(result.error).toBeUndefined();
    expect(result.registeredId).toBe(manifest.id);
    expect(result.hasInitialize).toBe(true);
    expect(result.hasUninitialize).toBe(true);
});

test('gives the sidebar its view and its title', async ({mount, page}) => {
    await mount(<IndexHarness/>);

    const result = await recorded(page);

    expect(result.rhsComponentName).toBe('RhsView');
    expect(result.rhsTitleName).toBe('RhsTitle');
});

// The header action closes over showRHSPlugin/toggleRHSPlugin, which only exist
// once the sidebar registration has returned them.
test('registers the sidebar before the header button', async ({mount, page}) => {
    await mount(<IndexHarness/>);

    const result = await recorded(page);

    expect(result.order).toEqual(['rhs', 'tooltip', 'post-type', 'header']);
});

// One registration for the whole plugin: a decorator gets a hover by declaring
// a Hover component, not by touching the bootstrap.
test('registers the tooltip exactly once', async ({mount, page}) => {
    await mount(<IndexHarness/>);

    const result = await recorded(page);

    expect(result.tooltipComponentName).toBe('DecoratorTooltip');
    expect(result.called?.filter((name) => name === 'registerLinkTooltipComponent')).toHaveLength(1);
});

// One registration per declared post type, and only for a decorator that has an
// inline view to put in it. DTG declares neither, so a lone date-time group
// stays an ordinary post.
test('registers a post body for every decorator that declares one', async ({mount, page}) => {
    await mount(<IndexHarness/>);

    const result = await recorded(page);

    expect(result.postTypes).toEqual(['custom_tf_location']);
    expect(result.postBodyComponentNames).toEqual(['DecoratorPostBody']);
});

test('wires the channel header button', async ({mount, page}) => {
    await mount(<IndexHarness/>);

    const result = await recorded(page);

    expect(result.headerIconName).toBe('HeaderIcon');
    expect(result.headerDropdownText).toBe('Tactical Fusion');
    expect(result.headerTooltip).toBe('Tactical Fusion');
});

// Decoration happens on the server, which is what makes the link work on
// clients that never run this bundle. A format hook here would be a second,
// divergent implementation of the same thing.
test('never registers a message formatting hook', async ({mount, page}) => {
    await mount(<IndexHarness/>);

    const result = await recorded(page);

    expect(result.called).not.toContain('registerMessageWillFormatHook');
    expect(result.called).not.toContain('registerMessageWillBePostedHook');
});

// Always land on the empty state, which is also the only way back from a
// decorator panel. The selection recorded at dispatch time is what proves the
// clear happened first rather than after.
test('the header button clears the selection before toggling', async ({mount, page}) => {
    await mount(<IndexHarness/>);

    const result = await recorded(page);

    expect(result.dispatched).toEqual([{type: 'TOGGLE_RHS'}]);
    expect(result.selectionAtDispatch).toEqual([null]);
});

test('intercepts a decorator link once initialized', async ({mount, page}) => {
    await mount(<IndexHarness/>);
    await recorded(page);
    await page.getByTestId('reset-count').click();

    await page.getByTestId('dtg-link').click();

    await expect(page.getByTestId('selection')).toHaveText('dtg');
    await expect(page.getByTestId('dispatch-count')).toHaveText('1');
});

// Without uninitialize clearing the disposers, a re-registration would leave the
// old capture listener attached and every click would be handled twice.
test('re-initializing does not attach a second click listener', async ({mount, page}) => {
    await mount(<IndexHarness/>);
    await recorded(page);

    await page.getByTestId('uninitialize').click();
    await page.getByTestId('initialize').click();
    await page.getByTestId('reset-count').click();

    await page.getByTestId('dtg-link').click();

    // Twice would mean the old listener was still attached alongside the new.
    await expect(page.getByTestId('selection')).toHaveText('dtg');
    await expect(page.getByTestId('dispatch-count')).toHaveText('1');
});

test('uninitialize releases the click handler and the stylesheet', async ({mount, page}) => {
    await mount(<IndexHarness/>);
    await recorded(page);
    await expect(page.locator('#tactical-fusion-decorator-styles')).toHaveCount(1);

    await page.getByTestId('uninitialize').click();
    await page.getByTestId('reset-count').click();

    await expect(page.locator('#tactical-fusion-decorator-styles')).toHaveCount(0);

    // The listener is gone, so the click no longer reaches the selection.
    await page.getByTestId('dtg-link').click();

    await expect(page.getByTestId('selection')).toHaveText('none');
    await expect(page.getByTestId('dispatch-count')).toHaveText('0');
});

// Mattermost calls uninitialize() on the old instance and then replaces it, but
// it does not remove what that instance registered: removePlugin in the host
// says "The plugin is responsible for removing any of its registered
// components." So every id has to come back here, or a plugin upgrade in a live
// tab leaves a second sidebar, tooltip and header button behind.
test('uninitialize gives back every registry component', async ({mount, page}) => {
    await mount(<IndexHarness/>);
    await recorded(page);

    await expect(page.getByTestId('unregistered')).toHaveText('none');

    await page.getByTestId('uninitialize').click();

    await expect(page.getByTestId('unregistered')).toHaveText(
        'rhs-id,tooltip-id,post-type-id-custom_tf_location,header-id');
});

// Calling it twice must be a no-op rather than running every disposer again.
test('uninitialize is safe to call twice', async ({mount, page}) => {
    await mount(<IndexHarness/>);
    await recorded(page);

    await page.getByTestId('uninitialize').click();
    await page.getByTestId('uninitialize').click();

    const result = await recorded(page);
    expect(result.error).toBeUndefined();
    await expect(page.locator('#tactical-fusion-decorator-styles')).toHaveCount(0);
});
