import {useSyncExternalStore} from 'react';

import type {Selection} from '../../decorators/selection';
import {subscribe as subscribeSelection} from '../../decorators/selection';
import {getPanel} from '../../panels';

export const MAX_HISTORY = 10;

export interface HistoryEntry {
    label: string;
    selection: Selection;
}

type Listener = () => void;

let entries: readonly HistoryEntry[] = [];
const listeners = new Set<Listener>();

function publish(next: readonly HistoryEntry[]): void {
    entries = next;
    listeners.forEach((listener) => listener());
}

function labelOf(selection: Selection): string | null {
    const entry = getPanel(selection.type);
    if (!entry) {
        return null;
    }
    try {
        const label = entry.summary(selection.payload).trim();
        return label === '' ? null : label;
    } catch {
        return null;
    }
}

export function recordSelection(selection: Selection | null): void {
    if (!selection) {
        return;
    }
    const label = labelOf(selection);
    if (label === null) {
        return;
    }
    const rest = entries.filter((entry) => entry.selection.type !== selection.type || entry.label !== label);
    publish([{label, selection}, ...rest].slice(0, MAX_HISTORY));
}

export function startHistory(): () => void {
    return subscribeSelection(recordSelection);
}

export function getHistory(): readonly HistoryEntry[] {
    return entries;
}

export function clearHistory(): void {
    publish([]);
}

function subscribe(listener: Listener): () => void {
    listeners.add(listener);
    return () => {
        listeners.delete(listener);
    };
}

export function useHistory(): readonly HistoryEntry[] {
    return useSyncExternalStore(subscribe, getHistory, getHistory);
}
