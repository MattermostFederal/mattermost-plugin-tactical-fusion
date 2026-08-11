import {expect, test} from '@playwright/test';

import {
    clearSelection,
    getSelection,
    setSelection,
    subscribe,
    _resetForTesting,
} from './selection';

test.beforeEach(() => {
    _resetForTesting();
});

test('starts empty', () => {
    expect(getSelection()).toBeNull();
});

test('set and get round trip', () => {
    setSelection({type: 'dtg', payload: {canonical: '091630ZAUG26'}});

    expect(getSelection()).toEqual({type: 'dtg', payload: {canonical: '091630ZAUG26'}});
});

test('subscribers are notified on change', () => {
    const seen: Array<string | null> = [];
    subscribe((selection) => seen.push(selection?.type ?? null));

    setSelection({type: 'dtg', payload: {}});
    setSelection({type: 'other', payload: {}});
    clearSelection();

    expect(seen).toEqual(['dtg', 'other', null]);
});

test('unsubscribing stops notifications', () => {
    const seen: string[] = [];
    const unsubscribe = subscribe((selection) => {
        if (selection) {
            seen.push(selection.type);
        }
    });

    setSelection({type: 'first', payload: {}});
    unsubscribe();
    setSelection({type: 'second', payload: {}});

    expect(seen).toEqual(['first']);
});

test('clearSelection resets to null', () => {
    setSelection({type: 'dtg', payload: {}});
    clearSelection();

    expect(getSelection()).toBeNull();
});
