import {useSyncExternalStore} from 'react';
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

/**
 * The team the reader is looking at, or "" when there is none to read.
 *
 * Read through a narrow local shape rather than by importing the webapp's own
 * state types, which would be a runtime dependency on mattermost-redux for one
 * string. Empty means a caller must not ask for anything team-scoped, rather
 * than meaning every team.
 */
export function currentTeamId(): string {
    if (!store) {
        return '';
    }

    const state = store.getState() as {
        entities?: {teams?: {currentTeamId?: unknown}};
    } | undefined;

    const id = state?.entities?.teams?.currentTeamId;

    return typeof id === 'string' ? id : '';
}

function subscribeToStore(onStoreChange: () => void): () => void {
    if (!store) {
        return () => undefined;
    }

    return store.subscribe(onStoreChange);
}

/**
 * currentTeamId as a hook, so a panel that reads it re-renders when the reader
 * switches team rather than holding the team it mounted under.
 */
export function useCurrentTeamId(): string {
    return useSyncExternalStore(subscribeToStore, currentTeamId, currentTeamId);
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
