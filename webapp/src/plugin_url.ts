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
/**
 * Where Mattermost serves this plugin's webapp bundle and its lazy chunks.
 *
 * Validated before use, because assigning it to `__webpack_public_path__`
 * promotes `window.basename` from a value that builds fetch URLs into one that
 * decides where the browser loads executable JavaScript from. The server side
 * of this plugin applies the same rule to SiteURL: a path that is not rooted is
 * ignored rather than emitted, since it would resolve against whatever page the
 * reader happens to be on.
 */
export function staticBaseUrl(): string {
    const globalWindow = typeof window === 'undefined' ? undefined : (window as {basename?: string});
    const basename = globalWindow?.basename ?? '';
    const base = `${basename}/static/plugins/${manifest.id}/`;

    if (!base.startsWith('/') || base.startsWith('//') || base.includes('://')) {
        return `/static/plugins/${manifest.id}/`;
    }

    return base;
}

export function docsUrl(): string {
    return `${pluginBaseUrl()}/public/help/help.html`;
}
