import {useSyncExternalStore} from 'react';

/**
 * A store for whether a decorator's panel is showing its editor.
 *
 * Module state rather than component state because Mattermost renders the
 * sidebar header and the sidebar body as two separate components. The panel
 * cannot pass this to the header, and `summary` is a pure function of the
 * payload and so cannot see it either. One store both of them read is the whole
 * of the mechanism.
 *
 * A factory rather than one shared store, because each decorator needs its own:
 * opening the DTG editor must not put the location panel into its editor too.
 * Each call closes over its own state, which gives that independence exactly as
 * a second copied file would, without the second copy. There was one, for a
 * while — 56 lines duplicated with one word of the doc comment changed — and
 * its cost was that the DTG copy had a spec and the location copy did not.
 */
export interface EditingStore {

    /** Subscribes to the editor opening and closing. Returns the unsubscribe. */
    subscribe: (listener: () => void) => () => void;

    /** Whether the editor is open. */
    isEditing: () => boolean;

    /** Opens or closes the editor. */
    setEditing: (next: boolean) => void;

    /** Subscribes a component to the editor being open. */
    useEditing: () => boolean;

    /** @internal exported for tests */
    _resetForTesting: () => void;
}

export function createEditingStore(): EditingStore {
    let editing = false;
    const listeners = new Set<() => void>();

    const subscribe = (listener: () => void): (() => void) => {
        listeners.add(listener);
        return () => {
            listeners.delete(listener);
        };
    };

    const getSnapshot = (): boolean => editing;

    /*
     * Silent when nothing changes, so a panel resetting this on every selection
     * cannot spuriously re-render the header.
     */
    const setEditing = (next: boolean): void => {
        if (editing === next) {
            return;
        }

        editing = next;
        listeners.forEach((listener) => listener());
    };

    return {
        subscribe,
        isEditing: getSnapshot,
        setEditing,
        useEditing: () => useSyncExternalStore(subscribe, getSnapshot, getSnapshot),
        _resetForTesting: () => {
            editing = false;
            listeners.clear();
        },
    };
}
