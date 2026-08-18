import type {Features} from './types';
import {ALL_FEATURES} from './types';

/**
 * The features route, for the harnesses that replace `window.fetch` wholesale
 * rather than routing through Playwright.
 *
 * Those harnesses install their stub at module scope, before any component
 * renders, so a `page.route` interception is not what they use. Without a branch
 * for this route the request would either hang on the deferred conversion path,
 * leaving every surface reading "not answered yet" and no map ever drawn, or
 * fall through to the network and land on the store's degrade path, which
 * answers every surface on and would make the test pass for the wrong reason.
 *
 * Test-only, imported only from harnesses, so it never reaches the plugin
 * bundle.
 */
let answer: Features = ALL_FEATURES;

/** Sets what the stubbed route reports. Call before mounting. */
export function setStubbedFeatures(features: Partial<Features> = {}): void {
    answer = {...ALL_FEATURES, ...features};
}

export function isFeaturesRequest(url: string): boolean {
    return url.includes('/api/v1/features');
}

export function featuresReply(): Response {
    return {
        ok: true,
        status: 200,
        json: () => Promise.resolve({
            map_panel: answer.mapPanel,
            map_inline: answer.mapInline,
            map_page: answer.mapPage,
        }),
    } as Response;
}
