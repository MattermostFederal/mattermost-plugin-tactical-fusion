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

/**
 * The built-in documentation, which Mattermost serves out of the bundle's
 * `public/` directory. There is no route for it in the server code.
 *
 * Derived from the plugin id rather than written out, so the link cannot drift
 * from whatever `plugin.json` says. The one place the id is spelled in full is
 * `plugin.json` itself, which is where it is defined.
 */
export function docsUrl(): string {
    return `${pluginBaseUrl()}/public/help/help.html`;
}
