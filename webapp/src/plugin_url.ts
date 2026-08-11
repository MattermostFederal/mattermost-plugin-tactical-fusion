import manifest from 'manifest';

/**
 * The plugin's own base path, e.g. `/plugins/com.mattermost.plugin-tactical-fusion`.
 *
 * `window.basename` carries the subpath when Mattermost is served from one, so
 * a hardcoded root-relative path would break those installs.
 *
 * Guarded so the pure logic that builds on this stays testable outside a
 * browser.
 */
export function pluginBaseUrl(): string {
    const globalWindow = typeof window === 'undefined' ? undefined : (window as {basename?: string});
    const basename = globalWindow?.basename ?? '';
    return `${basename}/plugins/${manifest.id}`;
}
