import {expect, test} from '@playwright/experimental-ct-react';
import React from 'react';

import TooltipHarness from './TooltipHarness';

const PREFIX = '/plugins/com.mattermost.plugin-tactical-fusion/decorate/';

test('shows the decorator hover card', async ({mount, page}) => {
    await mount(<TooltipHarness href={`${PREFIX}fix?v=hello`}/>);

    await expect(page.getByTestId('fixture-hover')).toHaveText('hello');
});

// Mattermost provides no backdrop, so a card without its own background renders
// transparent over the post behind it.
test('renders as an opaque card', async ({mount, page}) => {
    await mount(<TooltipHarness href={`${PREFIX}fix?v=hello`}/>);

    const card = page.getByTestId('fixture-hover').locator('..');
    const style = await card.evaluate((el) => {
        const computed = getComputedStyle(el);
        return {
            background: computed.backgroundColor,
            radius: computed.borderTopLeftRadius,
            borderWidth: computed.borderTopWidth,
            shadow: computed.boxShadow,
        };
    });

    expect(style.background).not.toBe('rgba(0, 0, 0, 0)');
    expect(style.background).not.toBe('transparent');
    expect(style.radius).toBe('8px');
    expect(style.borderWidth).toBe('1px');
    expect(style.shadow).not.toBe('none');
});

// Mattermost offers every link in a post, not just ours.
test('ignores a link this plugin does not own', async ({mount, page}) => {
    await mount(<TooltipHarness href='/some/other/place'/>);

    await expect(page.getByTestId('fixture-hover')).toHaveCount(0);
});

test('ignores an unknown decorator type', async ({mount, page}) => {
    await mount(<TooltipHarness href={`${PREFIX}nope?v=hello`}/>);

    await expect(page.getByTestId('fixture-hover')).toHaveCount(0);
});

test('ignores params the decorator cannot use', async ({mount, page}) => {
    await mount(<TooltipHarness href={`${PREFIX}fix?other=1`}/>);

    await expect(page.getByTestId('fixture-hover')).toHaveCount(0);
});

// A decorator gets a hover by declaring one, so a decorator without it has none.
test('renders nothing for a decorator with no hover card', async ({mount, page}) => {
    await mount(
        <TooltipHarness
            href={`${PREFIX}fix?v=hello`}
            withHover={false}
        />,
    );

    await expect(page.getByTestId('fixture-hover')).toHaveCount(0);
});

test('renders nothing until the reader is actually pointing at the link', async ({mount, page}) => {
    await mount(
        <TooltipHarness
            href={`${PREFIX}fix?v=hello`}
            show={false}
        />,
    );

    await expect(page.getByTestId('fixture-hover')).toHaveCount(0);
});
