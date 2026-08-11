import type {Store} from 'redux';

/** What the RHS is currently showing. */
export interface Selection {
    type: string;
    payload: unknown;
}

type Listener = (selection: Selection | null) => void;

let current: Selection | null = null;
const listeners = new Set<Listener>();

/**
 * One observable holds the whole RHS state, so the view is a registry lookup
 * rather than an if-chain that grows with every decorator.
 */
export function getSelection(): Selection | null {
    return current;
}

export function setSelection(selection: Selection | null): void {
    current = selection;
    listeners.forEach((listener) => listener(current));
}

export function clearSelection(): void {
    setSelection(null);
}

export function subscribe(listener: Listener): () => void {
    listeners.add(listener);
    return () => {
        listeners.delete(listener);
    };
}

// Redux is only needed to dispatch the RHS show/toggle actions the registry
// hands back. Nothing else here needs a store, so there is no reducer.
let store: Store | null = null;
let showAction: unknown = null;
let toggleAction: unknown = null;

export function initRhs(reduxStore: Store, show: unknown, toggle: unknown): void {
    store = reduxStore;
    showAction = show;
    toggleAction = toggle;
}

export function openRhs(): void {
    if (store && showAction) {
        store.dispatch(showAction as never);
    }
}

export function toggleRhs(): void {
    if (store && toggleAction) {
        store.dispatch(toggleAction as never);
    }
}

/** @internal exported for tests */
export function _resetForTesting(): void { // eslint-disable-line no-underscore-dangle, @typescript-eslint/naming-convention
    current = null;
    listeners.clear();
    store = null;
    showAction = null;
    toggleAction = null;
}
