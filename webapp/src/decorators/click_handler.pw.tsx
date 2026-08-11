import React from 'react';

import ClickHarness from './ClickHarness';

import {expect, test} from '../../playwright/ct-coverage';

// The DOM wiring is exercised through a harness because the routing half is
// already covered without a browser in click_handler.spec.ts. This file only
// needs to prove the listener behaves: what it swallows, and what it lets go.

test('intercepts a decorator link and reports the selection', async ({mount, page}) => {
    await mount(<ClickHarness/>);

    await page.getByTestId('decorator-link').click();

    await expect(page.getByTestId('selection')).toHaveText('fix:hello');
    await expect(page.getByTestId('default-prevented')).toHaveText('true');
});

test('leaves an unrelated link alone', async ({mount, page}) => {
    await mount(<ClickHarness/>);

    await page.getByTestId('other-link').click();

    await expect(page.getByTestId('selection')).toHaveText('none');
    await expect(page.getByTestId('default-prevented')).toHaveText('false');
});

// An unknown type means an older bundle against a newer server. Letting the
// browser navigate reaches the server-rendered page, which beats a dead click.
test('does not intercept an unknown decorator type', async ({mount, page}) => {
    await mount(<ClickHarness/>);

    await page.getByTestId('unknown-type-link').click();

    await expect(page.getByTestId('selection')).toHaveText('none');
    await expect(page.getByTestId('default-prevented')).toHaveText('false');
});

test('does not intercept params the decorator cannot use', async ({mount, page}) => {
    await mount(<ClickHarness/>);

    await page.getByTestId('bad-params-link').click();

    await expect(page.getByTestId('selection')).toHaveText('none');
    await expect(page.getByTestId('default-prevented')).toHaveText('false');
});

// The escape hatch for checking the server-rendered page from a desktop
// browser. The handler must stand aside so the browser follows the link.
test('does not intercept a link carrying the force-page flag', async ({mount, page}) => {
    await mount(<ClickHarness/>);

    await page.getByTestId('force-page-link').click();

    await expect(page.getByTestId('selection')).toHaveText('none');
    await expect(page.getByTestId('default-prevented')).toHaveText('false');
});

// The page is a separate document and cannot read the webapp's CSS variables,
// so it is told which theme to paint itself with on the way out.
test('passes the current theme to the standalone page', async ({mount, page}) => {
    await mount(<ClickHarness centerChannelBg='#090a0b'/>);

    const link = page.getByTestId('force-page-link');
    await link.click();

    await expect(link).toHaveAttribute('href', /_theme=dark/);

    // Still root-relative: storing a host is what the whole URL design avoids.
    const href = await link.getAttribute('href');
    expect(href!.startsWith('/')).toBe(true);
});

test('passes a light theme when the sidebar is light', async ({mount, page}) => {
    await mount(<ClickHarness centerChannelBg='#ffffff'/>);

    const link = page.getByTestId('force-page-link');
    await link.click();

    await expect(link).toHaveAttribute('href', /_theme=light/);
});

// With no theme variable to read, the page falls back to the operating system
// preference rather than being told something invented.
test('leaves the theme unstated when it cannot be read', async ({mount, page}) => {
    await mount(<ClickHarness/>);

    const link = page.getByTestId('force-page-link');
    await link.click();

    await expect(link).not.toHaveAttribute('href', /_theme/);
});

// The URL is a real destination, so "open in new tab" must still work.
test('leaves modified clicks to the browser', async ({mount, page}) => {
    await mount(<ClickHarness/>);

    await page.getByTestId('decorator-link').click({modifiers: ['Control']});

    await expect(page.getByTestId('selection')).toHaveText('none');
    await expect(page.getByTestId('default-prevented')).toHaveText('false');
});

test('the disposer removes the listener', async ({mount, page}) => {
    await mount(<ClickHarness/>);

    await page.getByTestId('dispose').click();
    await page.getByTestId('decorator-link').click();

    await expect(page.getByTestId('selection')).toHaveText('none');
});

// A second install must not attach a second listener, or a plugin reload would
// dispatch every click twice.
test('installing twice is a no-op', async ({mount, page}) => {
    await mount(<ClickHarness installTwice={true}/>);

    await page.getByTestId('decorator-link').click();

    await expect(page.getByTestId('dispatch-count')).toHaveText('1');
});

// One window for the whole plugin, so following a second link lands in the tab
// the reader already has open rather than collecting one per link.
test('points every standalone page at the same window', async ({mount, page}) => {
    await mount(<ClickHarness/>);

    await page.getByTestId('force-page-link').click();
    await page.getByTestId('second-force-page-link').click();

    const first = await page.getByTestId('force-page-link').getAttribute('target');
    const second = await page.getByTestId('second-force-page-link').getAttribute('target');

    expect(first).toBeTruthy();
    expect(second).toBe(first);
});

// A named target is ignored outright if the link also demands a fresh browsing
// context, which is exactly what Mattermost's own rel asks for.
test('drops the rel that would defeat the shared window', async ({mount, page}) => {
    await mount(<ClickHarness/>);

    const link = page.getByTestId('force-page-link');
    await expect(link).toHaveAttribute('rel', 'noopener noreferrer');

    await link.click();

    await expect(link).not.toHaveAttribute('rel', /.*/);
});

test('keeps any other rel the link was carrying', async ({mount, page}) => {
    await mount(<ClickHarness/>);

    const link = page.getByTestId('rel-keeps-others-link');
    await link.click();

    await expect(link).toHaveAttribute('rel', 'nofollow');
});

// The sidebar is opened in place, so those links must not be sent anywhere.
test('leaves a sidebar link with no target', async ({mount, page}) => {
    await mount(<ClickHarness/>);

    await page.getByTestId('decorator-link').click();

    await expect(page.getByTestId('decorator-link')).not.toHaveAttribute('target', /.*/);
});

/*
 * A decorator link is always root-relative, so anything on another host is not
 * one however closely its path matches.
 *
 * The href is resolved against this origin so a relative one is parseable, and
 * an absolute cross-origin URL used to survive that. Everything the handler
 * accepts is trusted from then on: it opens the sidebar, it collects a hover
 * card, and on the _page branch it is opened in a named window with noopener
 * and noreferrer taken off the rel.
 */
test('does not intercept a cross-origin link wearing the decorator path', async ({mount, page}) => {
    await mount(<ClickHarness/>);

    const link = page.getByTestId('cross-origin-link');
    await link.click();

    await expect(page.getByTestId('selection')).toHaveText('none');
    await expect(page.getByTestId('default-prevented')).toHaveText('false');
});

// The rel matters most here: stripping it on a link this plugin does not own
// hands the destination a live window.opener onto the reader's Mattermost tab.
test('leaves the rel alone on a cross-origin page link', async ({mount, page}) => {
    await mount(<ClickHarness/>);

    const link = page.getByTestId('cross-origin-page-link');
    await link.click();

    await expect(page.getByTestId('selection')).toHaveText('none');
    await expect(link).toHaveAttribute('rel', 'noopener noreferrer');
    await expect(link).not.toHaveAttribute('target', /.+/);
});
