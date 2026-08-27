import React from 'react';

import HoverLinkHarness from './HoverLinkHarness';

import {expect, test} from '../../playwright/ct-coverage';

const HREF = '/plugins/com.mattermost.plugin-tactical-fusion/decorate/dtg';

function dtgHref(offsetMs: number): string {
    return `${HREF}?a=&dtg=091630ZAUG26&t=${Date.now() + offsetMs}&z=Z`;
}

test('a decorator link this bundle drew carries its own hover card', async ({mount}) => {
    const component = await mount(
        <HoverLinkHarness
            href={dtgHref(5_400_000)}
            label='091630ZAUG26'
        />,
    );

    const card = component.locator('.tactical-fusion-hover-card');
    await expect(card).toBeHidden();

    await component.getByRole('link').hover();

    await expect(card).toBeVisible();
    await expect(card).toContainText('in 1h 29m');
});

/*
 * The card is sized by its own content, not by the link it hangs off.
 *
 * An absolutely positioned box with `left` and no width shrinks to fit its
 * CONTAINING BLOCK, which here is the anchor. Without a width the card came out
 * as narrow as the token and broke "in 1h 29m 59s" across two lines, which is
 * what a reader saw hovering a time in the Cursor on Target panel.
 */
test('the card is not squeezed into the width of its link', async ({mount}) => {
    const component = await mount(
        <HoverLinkHarness
            href={dtgHref(5_400_000)}
            label='091630ZAUG26'
        />,
    );

    await component.getByRole('link').hover();

    const card = component.locator('.tactical-fusion-hover-card');
    await expect(card).toBeVisible();

    const reading = card.locator('p');
    const lineHeight = await reading.evaluate((node) => {
        const box = node.getBoundingClientRect();
        const size = Number.parseFloat(getComputedStyle(node).fontSize);
        return box.height / size;
    });

    // One line of a 16px readout is about 1.2 of its own font size. Two would be
    // past 2, whatever the exact line-height resolves to.
    expect(lineHeight).toBeLessThan(1.8);
});

test('the card fits beside its link in a sidebar-width column', async ({mount}) => {
    const width = 320;
    const component = await mount(
        <HoverLinkHarness
            href={dtgHref(5_400_000)}
            label='091630ZAUG26'
            width={width}
        />,
    );

    await component.getByRole('link').hover();

    const card = component.locator('.tactical-fusion-hover-card');
    await expect(card).toBeVisible();

    const box = await card.boundingBox();
    expect(box!.width).toBeLessThan(width);
});

test('nothing is drawn for a link this bundle does not own', async ({mount}) => {
    const component = await mount(
        <HoverLinkHarness
            href='https://example.com/somewhere'
            label='somewhere'
        />,
    );

    await component.getByRole('link').hover();

    await expect(component.locator('.tactical-fusion-hover-card')).toHaveCount(0);
});
