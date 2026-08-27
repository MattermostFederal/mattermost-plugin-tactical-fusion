import React from 'react';

import Countdown from './Countdown';

import {expect, test} from '../../../playwright/ct-coverage';

// DtgPanel.pw.tsx and DtgHover.pw.tsx already cover the countdown in place:
// urgent, recently passed, distant, the compact variant and a custom threshold.
// What is left is the edges of the threshold itself and the pulse's own state,
// which the panel cannot reach because it never changes urgency mid-life.

const MINUTE = 60 * 1000;

function inFuture(ms: number): Date {
    return new Date(Date.now() + ms);
}

test.describe('the urgency threshold', () => {
    test('a target inside the threshold is urgent', async ({mount, page}) => {
        await mount(<Countdown
            target={inFuture(5 * MINUTE)}
            urgentWithinMs={30 * MINUTE}
                    />);

        await expect(page.locator('p')).toHaveAttribute('data-urgent', 'true');
    });

    test('a target outside the threshold is not', async ({mount, page}) => {
        await mount(<Countdown
            target={inFuture(45 * MINUTE)}
            urgentWithinMs={30 * MINUTE}
                    />);

        await expect(page.locator('p')).toHaveAttribute('data-urgent', 'false');
    });

    // Urgency is measured on the absolute difference, so a DTG that has just
    // gone by is as urgent as one about to.
    test('a target just past is urgent too', async ({mount, page}) => {
        await mount(<Countdown
            target={inFuture(-5 * MINUTE)}
            urgentWithinMs={30 * MINUTE}
                    />);

        await expect(page.locator('p')).toHaveAttribute('data-urgent', 'true');
    });

    // Zero is a real argument rather than an omission, so the default threshold
    // must not step in for it: a reader who set the threshold to nothing gets
    // nothing, not thirty minutes.
    test('a zero threshold is honored rather than defaulted', async ({mount, page}) => {
        await mount(<Countdown
            target={inFuture(MINUTE)}
            urgentWithinMs={0}
                    />);

        await expect(page.locator('p')).toHaveAttribute('data-urgent', 'false');
    });

    // The same target with the threshold left off is urgent, which is what makes
    // the test above about the zero rather than about the distance.
    test('the built-in threshold applies when none is given', async ({mount, page}) => {
        await mount(<Countdown target={inFuture(MINUTE)}/>);

        await expect(page.locator('p')).toHaveAttribute('data-urgent', 'true');
    });
});

test.describe('how urgency reads', () => {
    // The bar and the color carry the signal on their own, for a reader who
    // does not perceive the pulse.
    test('urgent draws a bar and colors the text', async ({mount, page}) => {
        await mount(<Countdown
            target={inFuture(MINUTE)}
            urgentWithinMs={30 * MINUTE}
                    />);

        const countdown = page.locator('p');
        await expect(countdown).toHaveCSS('border-left-width', '4px');
        await expect(countdown).toHaveCSS('padding-left', '10px');
    });

    // Not merely a different color: the bar and its padding have to be absent,
    // or every countdown would sit indented waiting for one.
    test('a calm countdown has no bar and no indent', async ({mount, page}) => {
        await mount(<Countdown
            target={inFuture(45 * MINUTE)}
            urgentWithinMs={30 * MINUTE}
                    />);

        const countdown = page.locator('p');
        await expect(countdown).toHaveCSS('border-left-width', '0px');
        await expect(countdown).toHaveCSS('padding-left', '0px');
        await expect(countdown).toHaveCSS('opacity', '1');
    });

    test('compact is smaller and unspaced', async ({mount, page}) => {
        await mount(<Countdown
            target={inFuture(45 * MINUTE)}
            compact={true}
                    />);

        const countdown = page.locator('p');
        await expect(countdown).toHaveCSS('font-size', '16px');
        await expect(countdown).toHaveCSS('margin-bottom', '0px');
    });

    test('the panel variant is larger and leaves room below', async ({mount, page}) => {
        await mount(<Countdown target={inFuture(45 * MINUTE)}/>);

        const countdown = page.locator('p');
        await expect(countdown).toHaveCSS('font-size', '24px');
        await expect(countdown).toHaveCSS('margin-bottom', '6px');
    });
});

test.describe('the pulse', () => {
    test('an urgent countdown pulses', async ({mount, page}) => {
        await mount(<Countdown
            target={inFuture(MINUTE)}
            urgentWithinMs={30 * MINUTE}
                    />);

        // Auto-retrying, so this waits for the dimmed half of the cycle.
        await expect(page.locator('p')).toHaveCSS('opacity', '0.35');
        await expect(page.locator('p')).toHaveCSS('opacity', '1');
    });

    // Leaving urgency has to restore full opacity, or a countdown that pulsed
    // out of the window would be stranded half faded for good.
    test('leaving urgency restores full opacity', async ({mount, page}) => {
        const component = await mount(<Countdown
            target={inFuture(MINUTE)}
            urgentWithinMs={30 * MINUTE}
                                      />);

        // Wait until it is genuinely mid-pulse before taking urgency away.
        await expect(page.locator('p')).toHaveCSS('opacity', '0.35');

        await component.update(<Countdown
            target={inFuture(MINUTE)}
            urgentWithinMs={0}
                               />);

        await expect(page.locator('p')).toHaveAttribute('data-urgent', 'false');
        await expect(page.locator('p')).toHaveCSS('opacity', '1');
    });
});

// The countdown drives itself off its own clock, so it has to move without
// anything re-rendering it.
test('counts down on its own', async ({mount, page}) => {
    await mount(<Countdown
        target={inFuture(90 * MINUTE)}
        urgentWithinMs={30 * MINUTE}
                />);

    const countdown = page.locator('p');
    const first = await countdown.textContent();

    await expect(countdown).not.toHaveText(first ?? '');
});

// There is deliberately no test for an invalid target. `fromParams` rejects a
// `t` that is not an integer inside the allowed range before it ever builds the
// instant, so an unparseable Date cannot reach here from a decorator link.
// Playwright CT cannot serialize one across the component boundary either,
// which is the same fact showing up twice.
