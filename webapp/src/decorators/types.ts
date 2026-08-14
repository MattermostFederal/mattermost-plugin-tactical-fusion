import type React from 'react';

/**
 * A decorator renders the detail panel behind a link the server produced.
 *
 * The server has already matched, validated and parsed the token, so this side
 * has no patterns and no grammar. It reads query params and renders. Keeping
 * parsing in Go only means the two sides cannot drift apart.
 *
 * ## Adding a decorator
 *
 * 1. Create `webapp/src/decorators/<type>/` exporting a `Decorator<T>`.
 * 2. Register it in `registerBuiltinDecorators()` in `decorators/index.ts`.
 * 3. Add the matching Go decorator under `server/decorators/<type>/` and
 *    register it in `OnActivate`.
 *
 * Nothing else in `decorators/` needs to change. `type` must match the Go
 * decorator's `Type()` exactly, since that string is the URL path segment.
 */
export interface Decorator<T> {

    /** URL path segment. Must equal the Go decorator's `Type()`. */
    type: string;

    /**
     * Builds the payload from the clicked URL's query params.
     *
     * Return `null` if the params are unusable. The click handler then leaves
     * the click alone and lets the browser navigate to the server-rendered
     * page, which is a better outcome than a dead click.
     */
    fromParams: (params: URLSearchParams) => T | null;

    /**
     * One-line description, used as the RHS header.
     *
     * Still required when `Title` is supplied: it is what the header falls back
     * to, and a decorator should always be able to name itself in one string.
     */
    summary: (payload: T) => string;

    /** Link colors. The framework generates the CSS rule from these. */
    style: {color: string; background: string};

    /** The RHS body. */
    Panel: React.ComponentType<{payload: T}>;

    /**
     * Optional RHS header, for a panel whose header has to change with what the
     * panel is showing.
     *
     * The header is rendered by Mattermost as a separate component from the
     * body, so a panel with more than one view cannot drive its own header
     * through `summary`, which is a pure function of the payload. A decorator
     * that needs this keeps the state in a module store both components read.
     * A decorator that omits it gets `summary`.
     */
    Title?: React.ComponentType<{payload: T}>;

    /**
     * Optional hover card, shown when a reader points at the link.
     *
     * Keep it to the one thing worth knowing without opening the sidebar. A
     * decorator that omits this simply has no hover.
     */
    Hover?: React.ComponentType<{payload: T}>;
}
