import {expect, test} from '@playwright/test';
import manifest from 'manifest';

import {decorate, link} from './client';
import {installBridgeGlobal} from './global';
import type {BridgeHost} from './global';
import {Link} from './Link';
import {BRIDGE_API_VERSION, READY_EVENT} from './types';

import {registerBuiltinDecorators} from '../decorators/index';
import {_resetForTesting as resetDecorators} from '../decorators/registry';

function fakeHost(): BridgeHost & {events: Event[]} {
    const events: Event[] = [];
    return {
        events,
        dispatchEvent(event: Event) {
            events.push(event);
            return true;
        },
    };
}

test.beforeEach(() => {
    resetDecorators();
    registerBuiltinDecorators();
});

test('publishes a frozen API naming every decorator', () => {
    const host = fakeHost();

    installBridgeGlobal(host);

    const api = host.TacticalFusion;
    expect(api).toBeDefined();
    expect(Object.isFrozen(api)).toBe(true);
    expect(api?.apiVersion).toBe(BRIDGE_API_VERSION);
    expect(api?.version).toBe(manifest.version);
    expect(api?.types).toEqual(['dtg', 'location', 'airport']);
    expect(api?.decorate).toBe(decorate);
    expect(api?.link).toBe(link);
    expect(api?.Link).toBe(Link);
});

test('announces itself to a plugin that loaded first', () => {
    const host = fakeHost();

    installBridgeGlobal(host);

    expect(host.events).toHaveLength(1);
    expect(host.events[0].type).toBe(READY_EVENT);
    expect((host.events[0] as CustomEvent).detail).toBe(host.TacticalFusion);
});

test('the disposer withdraws the API', () => {
    const host = fakeHost();

    const dispose = installBridgeGlobal(host);
    dispose();

    expect(host.TacticalFusion).toBeUndefined();
});

test('a stale disposer leaves a newer registration in place', () => {
    const host = fakeHost();

    const disposeFirst = installBridgeGlobal(host);
    installBridgeGlobal(host);
    const current = host.TacticalFusion;
    disposeFirst();

    expect(host.TacticalFusion).toBe(current);
});
