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

let store: Store | null = null;
let showAction: unknown = null;

export function initRhs(reduxStore: Store, show: unknown): void {
    store = reduxStore;
    showAction = show;
}

export function openRhs(): void {
    if (store && showAction) {
        store.dispatch(showAction as never);
    }
}

interface ConfigState {
    entities?: {general?: {config?: {HasImageProxy?: string}}};
}

export function hasImageProxy(): boolean {
    return (store?.getState() as ConfigState | undefined)?.entities?.general?.config?.HasImageProxy === 'true';
}

/** @internal exported for tests */
export function _resetForTesting(): void { // eslint-disable-line no-underscore-dangle, @typescript-eslint/naming-convention
    current = null;
    listeners.clear();
    store = null;
    showAction = null;
}
