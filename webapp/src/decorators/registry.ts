import type {Decorator} from './types';

import {pluginBaseUrl} from '../plugin_url';

/*
 * Decorators are heterogeneous in their payload type, and Decorator<T> is
 * invariant because T appears in parameter position in both summary() and
 * Panel. A Decorator<Dtg> is therefore not assignable to Decorator<unknown>,
 * and <d.Panel payload={selection.payload}/> would not typecheck.
 *
 * The erasure is confined to this file. Authoring a decorator stays fully
 * typed; only the registry's storage is loose.
 */
/* eslint-disable @typescript-eslint/no-explicit-any */
type AnyDecorator = Decorator<any>;

const ordered: AnyDecorator[] = [];
const byType = new Map<string, AnyDecorator>();

/** Registers a decorator. Throws on a duplicate type, which is a coding error. */
export function register(decorator: AnyDecorator): void {
    if (!decorator.type) {
        throw new Error('decorator type must not be empty');
    }
    if (byType.has(decorator.type)) {
        throw new Error(`decorator type "${decorator.type}" is already registered`);
    }
    byType.set(decorator.type, decorator);
    ordered.push(decorator);
}

/** All registered decorators, in registration order. */
export function all(): AnyDecorator[] {
    return ordered;
}

/** The decorator for a type, or undefined when none is registered. */
export function get(type: string): AnyDecorator | undefined {
    return byType.get(type);
}

/**
 * Makes this file leave a decorator link alone, so the browser navigates to the
 * server-rendered page instead of opening the right-hand sidebar.
 *
 * It exists for testing: the page is normally only reachable from a client that
 * does not run this bundle, such as the mobile app, which makes it awkward to
 * check from a desktop browser.
 *
 * The leading underscore marks it as belonging to the framework. Decorators own
 * every other parameter name, so reserving a prefix keeps a future decorator
 * from colliding with this. Must match ForcePageParam in the Go package.
 */
export const FORCE_PAGE_PARAM = '_page';

/**
 * The window every standalone decorator page opens into.
 *
 * One name for the whole plugin, not one per decorator, so a reader following a
 * second link lands in the tab they already have open rather than collecting
 * one per DTG they were curious about.
 *
 * The name is namespaced because a browsing context name is shared with every
 * other page on the origin.
 */
export const PAGE_TARGET = 'tactical-fusion-decorator-page';

/**
 * Carries the reader's Mattermost theme to the standalone page.
 *
 * That page is a separate document and cannot read the webapp's CSS variables,
 * so without a hint it can only follow the operating system's light or dark
 * preference. Those are different settings: a light Mattermost on a dark laptop
 * would open a dark page next to a light sidebar.
 *
 * Must match ThemeParam in the Go package.
 */
export const THEME_PARAM = '_theme';

/**
 * The path every decorator link starts with, e.g.
 * `/plugins/com.mattermost.plugin-tactical-fusion/decorate/`.
 *
 * The server builds the stored URLs from SiteURL's path component, which is the
 * same value `pluginBaseUrl` reads from `window.basename`.
 *
 * Both the click handler and the stylesheet build from this single helper, so
 * the path they look for cannot drift apart. What they accept around it does
 * differ: the stylesheet matches from the start of the href, while
 * parseDecoratorHref accepts any same-origin form of the same path. A reader
 * who pastes the standalone page's absolute address into a channel therefore
 * gets the sidebar and the hover but no chip. The server only ever writes the
 * root-relative form, so this only shows up on a hand-written link.
 */
export function decoratePathPrefix(): string {
    return `${pluginBaseUrl()}/decorate/`;
}

/**
 * Pulls the decorator type and params out of a link's href.
 *
 * Shared so everything that reacts to a decorator link, the click handler and
 * the hover card, agrees on what counts as one. Returns null for anything else.
 */
export function parseDecoratorHref(href: string): {type: string; params: URLSearchParams} | null {
    let url: URL;
    try {
        url = new URL(href, window.location.origin);
    } catch {
        return null;
    }

    // Same origin only. The path is resolved against this origin, so an
    // absolute cross-origin URL carrying the right path used to pass: the
    // decorated links this plugin writes are always root-relative, and anything
    // else is somebody else's destination wearing our path. It would otherwise
    // collect a genuine hover card from us, and, on the _page branch, have its
    // rel stripped and be opened in a named window with a live opener.
    if (url.origin !== window.location.origin) {
        return null;
    }

    const prefix = decoratePathPrefix();
    if (!url.pathname.startsWith(prefix)) {
        return null;
    }

    const type = url.pathname.slice(prefix.length);
    if (!type || type.includes('/')) {
        return null;
    }

    return {type, params: url.searchParams};
}

/** @internal exported for tests */
export function _resetForTesting(): void { // eslint-disable-line no-underscore-dangle, @typescript-eslint/naming-convention
    ordered.length = 0;
    byType.clear();
}
