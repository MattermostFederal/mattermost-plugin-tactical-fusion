import manifest from 'manifest';

/**
 * Teaches `pluginBaseUrl` where this page is.
 *
 * Mattermost sets `window.basename` for the webapp, and every URL this bundle
 * builds goes through it: the basemap fetch, the documentation link and the
 * link between the two pages. A standalone page has no Mattermost around it, so
 * it derives the same value from its own address, which is the only thing here
 * that knows about a subpath install.
 *
 * Validated before it is set, for the reason `staticBaseUrl` is: this decides
 * where the browser fetches from, so a value that is not a rooted path is
 * dropped rather than used.
 */
export function applyBasename(): void {
    if (typeof window === 'undefined') {
        return;
    }

    const marker = `/plugins/${manifest.id}/`;
    const path = window.location.pathname;
    const at = path.indexOf(marker);
    if (at <= 0) {
        return;
    }

    const basename = path.slice(0, at);
    if (!basename.startsWith('/') || basename.startsWith('//') || basename.includes('://')) {
        return;
    }

    (window as {basename?: string}).basename = basename;
}
