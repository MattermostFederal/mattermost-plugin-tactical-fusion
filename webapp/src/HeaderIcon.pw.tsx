import React from 'react';

import {HeaderIcon} from './HeaderIcon';

import {expect, test} from '../playwright/ct-coverage';

test('renders the plugin mark', async ({mount, page}) => {
    await mount(<HeaderIcon/>);

    await expect(page.locator('svg')).toBeVisible();
    await expect(page.locator('svg')).toHaveAttribute('viewBox', '0 0 24 24');
});

// A header button sits in whatever theme the reader is using. The compass has
// to take the header's color, or it disappears against a light one; the pin
// keeps the mark's own, or the button stops being recognizable as this plugin.
test('adapts the compass to the theme and keeps the pin', async ({mount, page}) => {
    await mount(<HeaderIcon/>);

    await page.evaluate(() => {
        document.body.style.color = 'rgb(1, 2, 3)';
    });

    const painted = await page.locator('svg').evaluate((svg) => {
        const shapes = Array.from(svg.querySelectorAll('path, circle'));
        return shapes.map((shape) => {
            const style = getComputedStyle(shape);
            return {fill: style.fill, stroke: style.stroke};
        });
    });

    const themed = painted.filter((s) => s.fill === 'rgb(1, 2, 3)' || s.stroke === 'rgb(1, 2, 3)');
    const branded = painted.filter((s) => s.fill === 'rgb(255, 106, 19)');

    // The ring and the four compass points.
    expect(themed).toHaveLength(5);

    // The pin.
    expect(branded).toHaveLength(1);
});

// The button around it already carries the name, so announcing the mark as well
// would say it twice.
test('is not announced separately from its button', async ({mount, page}) => {
    await mount(<HeaderIcon/>);

    await expect(page.locator('svg')).toHaveAttribute('aria-hidden', 'true');
});
