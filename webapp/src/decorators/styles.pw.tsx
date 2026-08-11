import React from 'react';

import StylesHarness from './StylesHarness';

import {expect, test} from '../../playwright/ct-coverage';

// The stylesheet's document wiring. What it builds is covered without a browser
// in styles.spec.ts; this file only needs to prove where it puts it, and that it
// can be taken away again.

const STYLE_ID = 'tactical-fusion-decorator-styles';

test('appends the stylesheet to the document head', async ({mount, page}) => {
    await mount(<StylesHarness/>);
    await page.getByTestId('reset').click();
    await page.getByTestId('install').click();

    const style = page.locator(`head style#${STYLE_ID}`);
    await expect(style).toHaveCount(1);

    // A style element in the head renders nothing, so its text has to be read
    // rather than asserted on as visible content.
    const css = await style.evaluate((element) => element.textContent);
    expect(css).toContain('#ff0000');
    expect(css).toContain('#00ff00');
    expect(css).toContain('/decorate/fix?"]');
});

test('the disposer takes the stylesheet away again', async ({mount, page}) => {
    await mount(<StylesHarness/>);
    await page.getByTestId('reset').click();
    await page.getByTestId('install').click();
    await expect(page.locator(`#${STYLE_ID}`)).toHaveCount(1);

    await page.getByTestId('dispose-all').click();

    await expect(page.locator(`#${STYLE_ID}`)).toHaveCount(0);
});

// Idempotent by id: a second install must not add a second element, or the
// rules would be duplicated in the page for as long as both stayed.
test('installing twice leaves one stylesheet', async ({mount, page}) => {
    await mount(<StylesHarness/>);
    await page.getByTestId('reset').click();
    await page.getByTestId('install').click();
    await page.getByTestId('install').click();

    await expect(page.getByTestId('installs')).toHaveText('2');
    await expect(page.locator(`#${STYLE_ID}`)).toHaveCount(1);
});

// The element belongs to whoever installed it. A later caller gets a disposer
// that does nothing, so releasing it cannot pull the stylesheet out from under
// the install that is still using it.
test('the second install returns a disposer that owns nothing', async ({mount, page}) => {
    await mount(<StylesHarness/>);
    await page.getByTestId('reset').click();
    await page.getByTestId('install').click();
    await page.getByTestId('install').click();

    await page.getByTestId('dispose-last').click();

    await expect(page.locator(`#${STYLE_ID}`)).toHaveCount(1);

    // The first install still owns it, so its disposer still works.
    await page.getByTestId('dispose-last').click();

    await expect(page.locator(`#${STYLE_ID}`)).toHaveCount(0);
});

// Reinstalling after a dispose has to work, or a re-registration would leave
// the page permanently unstyled.
test('installs again after being disposed', async ({mount, page}) => {
    await mount(<StylesHarness/>);
    await page.getByTestId('reset').click();
    await page.getByTestId('install').click();
    await page.getByTestId('dispose-all').click();
    await expect(page.locator(`#${STYLE_ID}`)).toHaveCount(0);

    await page.getByTestId('install').click();

    await expect(page.locator(`#${STYLE_ID}`)).toHaveCount(1);
});
