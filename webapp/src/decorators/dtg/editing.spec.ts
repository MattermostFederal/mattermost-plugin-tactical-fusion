import {expect, test} from '@playwright/test';

import {isEditing, setEditing, subscribe, _resetForTesting} from './editing';

test.beforeEach(() => {
    _resetForTesting();
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
