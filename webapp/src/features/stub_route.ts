import type {Page} from '@playwright/test';

import type {Features} from './types';
import {ALL_FEATURES} from './types';

/**
 * Stands in for the plugin's features route in a component test.
 *
 * Every surface on unless a test says otherwise, so a test about something else
 * gets today's behaviour and does not have to know this route exists.
 *
 * It has to be stubbed rather than left to fail, even for those tests. An
 * unrouted request would reach the network, fail, and land on the store's
 * degrade path, which answers ALL_FEATURES and would give the same result by
 * accident: the test would pass while exercising the error handler, and would
 * keep passing if the route were deleted.
 *
 * Test-only, and imported only from `.pw.tsx` files, so it never reaches the
 * plugin bundle.
 */
export async function stubFeaturesRoute(page: Page, features: Partial<Features> = {}): Promise<void> {
    const answer = {...ALL_FEATURES, ...features};

    await page.route('**/api/v1/features', async (route) => {
        await route.fulfill({
            status: 200,
            contentType: 'application/json',
            body: JSON.stringify({
                map_panel: answer.mapPanel,
                map_inline: answer.mapInline,
                map_page: answer.mapPage,
            }),
        });
    });
}
