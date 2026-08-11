import React, {useEffect, useRef, useState} from 'react';

import {decoratePathPrefix} from './decorators/registry';
import {getSelection, setSelection, subscribe} from './decorators/selection';

/**
 * Harness for the plugin entry point.
 *
 * The entry point calls `window.registerPlugin(...)` at import time, so the stub
 * has to be in place before the module is pulled in: hence a dynamic import
 * rather than a top-level one. Everything `initialize` does to the registry is
 * recorded and serialised into a text node, which keeps the assertions in the
 * test file rather than in here.
 *
 * The registry is a Proxy so that *any* method the entry point reaches for is
 * recorded, including ones this harness does not implement. That is what makes
 * "never registers a message hook" assertable: a plain object would throw on the
 * unexpected call instead of reporting it.
 */

export type Recorded = {
    registeredId?: string;
    hasInitialize?: boolean;
    hasUninitialize?: boolean;
    called?: string[];
    order?: string[];
    rhsComponentName?: string;
    rhsTitleName?: string;
    tooltipComponentName?: string;
    headerIconName?: string;
    headerDropdownText?: string;
    headerTooltip?: string;
    dispatched?: unknown[];
    selectionAtDispatch?: unknown[];
    error?: string;
};

type CapturedPlugin = {
    initialize: (registry: unknown, store: unknown) => Promise<void>;
    uninitialize: () => void;
};

// A real DTG link, so a click exercises the handler the entry point installed.
const INSTANT_MS = Date.UTC(2026, 7, 9, 16, 30, 0);

function componentName(component: unknown): string {
    const asElement = component as {type?: {name?: string}};
    if (asElement?.type?.name) {
        return asElement.type.name;
    }
    return (component as {name?: string})?.name ?? String(component);
}

const IndexHarness: React.FC = () => {
    const [json, setJson] = useState('');
    const [dispatchCount, setDispatchCount] = useState(0);
    const [selected, setSelected] = useState('none');
    const [unregistered, setUnregistered] = useState<string[]>([]);
    const plugin = useRef<CapturedPlugin | undefined>(undefined);
    const registry = useRef<unknown>(undefined);
    const store = useRef<unknown>(undefined);

    useEffect(() => {
        const result: Recorded = {};

        // Setting up notifies more than once, so the count is only meaningful
        // against a baseline the test resets for itself.
        const unsubscribe = subscribe((next) => {
            setDispatchCount((count) => count + 1);
            setSelected(next ? next.type : 'none');
        });

        // Stops the browser following a link this harness left un-intercepted,
        // which would navigate the test page away from the mounted component.
        //
        // Bubble phase, deliberately. The entry point's handler runs on capture
        // and bails on an already-prevented event, so a capture listener here
        // would disable it: whichever was registered first would win, and
        // re-initializing would flip that. On the bubble phase the ordering
        // cannot matter, because a click the handler takes is stopped there and
        // never reaches this at all.
        const blockNavigation = (event: MouseEvent) => event.preventDefault();
        document.addEventListener('click', blockNavigation);

        (async () => {
            try {
                let captured: CapturedPlugin | undefined;
                (window as unknown as {registerPlugin: (id: string, instance: unknown) => void}).
                    registerPlugin = (id, instance) => {
                        result.registeredId = id;
                        captured = instance as CapturedPlugin;
                    };

                const mod = await import('./index');

                // Prefer the instance the module registered; fall back to the
                // exported class so the harness survives module caching.
                const instance = captured ?? new (mod.default as new () => CapturedPlugin)();
                plugin.current = instance;
                result.hasInitialize = typeof instance.initialize === 'function';
                result.hasUninitialize = typeof instance.uninitialize === 'function';

                const called: string[] = [];
                const order: string[] = [];
                const dispatched: unknown[] = [];
                const selectionAtDispatch: unknown[] = [];
                let headerAction: (() => void) | undefined;

                // Every registration hands back an id, as the real registry
                // does, so uninitialize has something to give back. The ids are
                // fixed rather than generated: a test asserts exactly which ones
                // were released.
                const handlers: Record<string, (...args: never[]) => unknown> = {
                    registerRightHandSidebarComponent: ((component: unknown, title: unknown) => {
                        order.push('rhs');
                        result.rhsComponentName = componentName(component);
                        result.rhsTitleName = componentName(title);
                        return {
                            id: 'rhs-id',
                            showRHSPlugin: {type: 'SHOW_RHS'},
                            toggleRHSPlugin: {type: 'TOGGLE_RHS'},
                        };
                    }) as never,
                    registerLinkTooltipComponent: ((component: unknown) => {
                        order.push('tooltip');
                        result.tooltipComponentName = componentName(component);
                        return 'tooltip-id';
                    }) as never,
                    registerChannelHeaderButtonAction: ((
                        icon: unknown,
                        action: () => void,
                        dropdownText: string,
                        tooltipText: string,
                    ) => {
                        order.push('header');
                        headerAction = action;
                        result.headerIconName = componentName(icon);
                        result.headerDropdownText = dropdownText;
                        result.headerTooltip = tooltipText;
                        return 'header-id';
                    }) as never,

                    // Mattermost does not remove a plugin's components when it
                    // re-registers, so what lands here is the whole of the
                    // cleanup. Recorded in React state rather than in `result`,
                    // because that is serialised as soon as initialize returns
                    // and these arrive when the test presses uninitialize.
                    unregisterComponent: ((componentId: string) => {
                        setUnregistered((ids) => [...ids, componentId]);
                    }) as never,
                };

                const recordingRegistry = new Proxy({}, {
                    get: (_target, property: string) => (...args: never[]) => {
                        called.push(property);
                        return handlers[property]?.(...args);
                    },
                });

                const recordingStore = {
                    dispatch: (action: unknown) => {
                        dispatched.push(action);

                        // What the selection was at the moment of dispatch. The
                        // header action clears before it toggles, so a null here
                        // is what proves the order.
                        selectionAtDispatch.push(getSelection());
                        return action;
                    },
                };

                registry.current = recordingRegistry;
                store.current = recordingStore;

                await instance.initialize(recordingRegistry, recordingStore);

                // Something to clear, so "cleared before toggling" is visible.
                setSelection({type: 'dtg', payload: {}});
                headerAction?.();

                result.called = called;
                result.order = order;
                result.dispatched = dispatched;
                result.selectionAtDispatch = selectionAtDispatch;
            } catch (err) {
                result.error = (err as Error)?.message ?? String(err);
            }
            setJson(JSON.stringify(result));
        })();

        return () => {
            unsubscribe();
            document.removeEventListener('click', blockNavigation);
        };
    }, []);

    const prefix = decoratePathPrefix();

    return (
        <div>
            <a
                data-testid='dtg-link'
                href={`${prefix}dtg?t=${INSTANT_MS}&dtg=091630ZAUG26&z=Z`}
            >{'091630ZAUG26'}</a>

            <button
                data-testid='uninitialize'
                onClick={() => plugin.current?.uninitialize()}
            >{'uninitialize'}</button>

            <button
                data-testid='initialize'
                onClick={() => {
                    plugin.current?.initialize(registry.current, store.current);
                }}
            >{'initialize'}</button>

            <button
                data-testid='reset-count'
                onClick={() => setDispatchCount(0)}
            >{'reset count'}</button>

            <p data-testid='index-harness-result'>{json}</p>
            <p data-testid='dispatch-count'>{String(dispatchCount)}</p>
            <p data-testid='selection'>{selected}</p>
            <p data-testid='unregistered'>{unregistered.join(',') || 'none'}</p>
        </div>
    );
};

export default IndexHarness;
