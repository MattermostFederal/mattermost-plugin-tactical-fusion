import {expect, test} from '@playwright/test';

import {createEditingStore} from './editing';

/*
 * The factory, tested once, rather than a copy of this file per decorator.
 *
 * There used to be two byte-identical stores with one word of the doc comment
 * different, and only one of them had a spec: the location copy's silence guard
 * was untested while the panel's payload-change reset depended on it.
 */
let store = createEditingStore();
const isEditing = (): boolean => store.isEditing();
const setEditing = (next: boolean): void => store.setEditing(next);
const subscribe = (listener: () => void): (() => void) => store.subscribe(listener);

test.beforeEach(() => {
    store = createEditingStore();
});

// Independence is the whole reason this is a factory: opening one decorator's
// editor must not put another decorator's panel into its editor too.
test('each store is its own', () => {
    const dtg = createEditingStore();
    const location = createEditingStore();

    dtg.setEditing(true);

    expect(dtg.isEditing()).toBe(true);
    expect(location.isEditing()).toBe(false);
});

test('starts closed', () => {
    expect(isEditing()).toBe(false);
});

test('opens and closes', () => {
    setEditing(true);
    expect(isEditing()).toBe(true);

    setEditing(false);
    expect(isEditing()).toBe(false);
});

test('tells subscribers about a change', () => {
    let notifications = 0;
    const unsubscribe = subscribe(() => {
        notifications++;
    });

    setEditing(true);
    expect(notifications).toBe(1);

    setEditing(false);
    expect(notifications).toBe(2);

    unsubscribe();
    setEditing(true);
    expect(notifications).toBe(2);
});

// The panel resets this on every change of selection, so a set that changes
// nothing must not re-render the header along with it.
test('says nothing when nothing changed', () => {
    let notifications = 0;
    subscribe(() => {
        notifications++;
    });

    setEditing(false);
    setEditing(false);

    expect(isEditing()).toBe(false);
    expect(notifications).toBe(0);
});
