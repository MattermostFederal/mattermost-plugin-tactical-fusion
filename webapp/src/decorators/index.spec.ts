import {expect, test} from '@playwright/test';

import {all, get, _resetForTesting} from './registry';

import {registerBuiltinDecorators} from './index';

test.beforeEach(() => {
    _resetForTesting();
});

test('registers the shipped decorators', () => {
    registerBuiltinDecorators();

    expect(get('dtg')).toBeDefined();
});

// The registry lives in module state that survives a plugin re-registration
// while initialize() runs again. Throwing on the second pass would leave the
// sidebar dead until a page reload.
test('registering twice is a no-op', () => {
    registerBuiltinDecorators();

    expect(() => registerBuiltinDecorators()).not.toThrow();
    expect(all()).toHaveLength(1);
});
