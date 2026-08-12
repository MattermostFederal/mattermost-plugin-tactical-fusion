import {get, parseDecoratorHref, FORCE_PAGE_PARAM, PAGE_TARGET, THEME_PARAM} from './registry';
import {openRhs, setSelection} from './selection';
import {detectTheme} from './theme';

/**
 * Routes a decorator link click to the RHS.
 *
 * Returns false when this click should be left to the browser, which happens
 * for a type this bundle does not know about, params it cannot use, or an
 * explicit request for the standalone page. An older webapp bundle against a
 * newer server then degrades to the server-rendered page rather than dying
 * silently.
 *
 * Kept free of DOM access so it can be tested without a browser.
 */
export function dispatchDecoratorClick(type: string, params: URLSearchParams): boolean {
    // A deliberate opt-out, so the standalone page can be reached from a
    // desktop browser without pretending to be a phone.
    if (params.has(FORCE_PAGE_PARAM)) {
        return false;
    }

    const decorator = get(type);
    if (!decorator) {
        return false;
    }

    const payload = decorator.fromParams(params);
    if (payload === null || payload === undefined) {
        return false;
    }

    setSelection({type, payload});
    openRhs();
    return true;
}

/**
 * Adds the reader's current theme to a decorator URL.
 *
 * The standalone page cannot read the webapp's CSS variables, so it is told
 * which way to paint itself. The result stays root-relative: an absolute URL
 * here would defeat the point of never storing a host.
 *
 * Returns the href unchanged when the theme cannot be determined, leaving the
 * page to fall back to the operating system preference.
 */
export function withTheme(href: string): string {
    const theme = detectTheme();
    if (!theme) {
        return href;
    }

    try {
        const url = new URL(href, window.location.origin);
        url.searchParams.set(THEME_PARAM, theme);
        return url.pathname + url.search;
    } catch {
        return href;
    }
}

/**
 * The `rel` a standalone page link should carry, or null to drop it entirely.
 *
 * A named target is only honored if the link does not also demand a fresh
 * browsing context, and `noopener` and `noreferrer` both demand exactly that:
 * with either present the browser ignores the name and opens another tab every
 * time. Mattermost puts them on rendered links as a matter of course, so they
 * have to come off for the target to mean anything.
 *
 * Safe here and only here. These links point at this plugin's own page on this
 * same origin, so there is no cross-origin opener to withhold; the usual reason
 * for the tokens does not apply. Anything else in the attribute is left alone.
 */
export function pageLinkRel(rel: string | null): string | null {
    if (!rel) {
        return null;
    }

    const kept = rel.split(/\s+/).
        filter((token) => token !== '' && token !== 'noopener' && token !== 'noreferrer');

    return kept.length > 0 ? kept.join(' ') : null;
}

let installed = false;

/**
 * Installs the single delegated click listener. Idempotent, and returns a
 * disposer.
 *
 * Without the disposer a plugin re-registration would leave a second listener
 * behind and dispatch every click twice.
 */
export function installDecoratorClickHandler(): () => void {
    if (installed) {
        return () => {
            // Already installed by an earlier call, which owns the listener.
        };
    }

    const onClick = (event: MouseEvent) => {
        // Leave modified clicks alone so "open in new tab" reaches the
        // server-rendered page. The URL is a real destination, not a marker.
        if (event.defaultPrevented || event.button !== 0 || event.metaKey ||
            event.ctrlKey || event.shiftKey || event.altKey) {
            return;
        }

        const target = event.target as HTMLElement | null;
        const link = target?.closest?.('a');
        const href = link?.getAttribute('href');
        if (!href) {
            return;
        }

        const parsed = parseDecoratorHref(href);
        if (!parsed) {
            return;
        }

        // Heading for the standalone page, so tell it which theme to paint
        // itself with, and point it at the one window these pages share, before
        // the browser follows the link. The click is not swallowed: rewriting
        // the attributes is enough.
        //
        // Nothing here is decorator-specific. Every type reaches the page
        // through the same reserved parameter, so every type gets this.
        if (parsed.params.has(FORCE_PAGE_PARAM) && link) {
            link.setAttribute('href', withTheme(href));
            link.setAttribute('target', PAGE_TARGET);

            const rel = pageLinkRel(link.getAttribute('rel'));
            if (rel === null) {
                link.removeAttribute('rel');
            } else {
                link.setAttribute('rel', rel);
            }
            return;
        }

        // Only swallow the click once we know we can handle it.
        if (dispatchDecoratorClick(parsed.type, parsed.params)) {
            event.preventDefault();
            event.stopPropagation();
        }
    };

    // Capture phase, or Mattermost's own internal-link handling routes the
    // click through the router before this sees it.
    document.addEventListener('click', onClick, true);
    installed = true;

    return () => {
        document.removeEventListener('click', onClick, true);
        installed = false;
    };
}
