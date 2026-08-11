import {useSyncExternalStore} from 'react';

/*
 * Whether the DTG panel is showing its editor rather than the DTG.
 *
 * Module state rather than component state because Mattermost renders the
 * sidebar header and the sidebar body as two separate components. The panel
 * cannot pass this to the header, and `summary` is a pure function of the
 * payload and so cannot see it either. One store both of them read is the whole
 * of the mechanism.
 */
let editing = false;
const listeners = new Set<() => void>();

/** Subscribes to the editor opening and closing. Returns the unsubscribe. */
export function subscribe(listener: () => void): () => void {
    listeners.add(listener);
    return () => {
        listeners.delete(listener);
    };
}

function getSnapshot(): boolean {
    return editing;
}

/** Whether the editor is open. */
export function isEditing(): boolean {
    return editing;
}

/**
 * Opens or closes the editor.
 *
 * Silent when nothing changes, so the panel resetting this on every selection
 * cannot spuriously re-render the header.
 */
export function setEditing(next: boolean): void {
    if (editing === next) {
        return;
    }

    editing = next;
    listeners.forEach((listener) => listener());
}

/** Subscribes a component to the editor being open. */
export function useEditing(): boolean {
    return useSyncExternalStore(subscribe, getSnapshot, getSnapshot);
}

/** @internal exported for tests */
export function _resetForTesting(): void { // eslint-disable-line no-underscore-dangle, @typescript-eslint/naming-convention
    editing = false;
    listeners.clear();
}
