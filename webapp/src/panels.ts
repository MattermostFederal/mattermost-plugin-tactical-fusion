import type React from 'react';

/*
 * What the right-hand sidebar can show.
 *
 * The RHS used to dispatch through the decorator registry directly, which was
 * right while every panel belonged to a decorator. Cursor on Target is not a
 * decorator: it has no token, no link and no page, so it could never be in that
 * registry, and `docs/design/cot.md` costed this out before the card was built.
 *
 * The alternative was registering a sham decorator for it, which would have
 * emitted a stylesheet chip rule and a click route for a `/decorate/cot` the
 * server answers with a 404. This table is the smaller of the two: it says what
 * the sidebar actually holds, which is panels, and a decorator gets one by being
 * registered rather than by anything here knowing about decorators.
 */

/*
 * Panels are heterogeneous in their payload type and the components are
 * invariant in it, exactly as `decorators/registry.ts` describes. The erasure is
 * confined to this file.
 */
/* eslint-disable @typescript-eslint/no-explicit-any */
export interface PanelEntry {
    Panel: React.ComponentType<{payload: any}>;
    Title?: React.ComponentType<{payload: any}>;
    summary: (payload: any) => string;
}

const byType = new Map<string, PanelEntry>();

/** Registers a panel. Throws on a duplicate type, which is a coding error. */
export function registerPanel(type: string, entry: PanelEntry): void {
    if (!type) {
        throw new Error('panel type must not be empty');
    }
    if (byType.has(type)) {
        throw new Error(`panel type "${type}" is already registered`);
    }
    byType.set(type, entry);
}

/** The panel for a type, or undefined when none is registered. */
export function getPanel(type: string): PanelEntry | undefined {
    return byType.get(type);
}

/** @internal exported for tests */
export function _resetPanelsForTesting(): void { // eslint-disable-line no-underscore-dangle, @typescript-eslint/naming-convention
    byType.clear();
}
