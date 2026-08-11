import manifest from 'manifest';
import React from 'react';
import type {Store} from 'redux';

import type {PluginRegistry} from 'types/mattermost-webapp';

import {RhsTitle, RhsView} from './components/rhs/RhsView';
import {installDecoratorClickHandler} from './decorators/click_handler';
import {registerBuiltinDecorators} from './decorators/index';
import {clearSelection, initRhs, toggleRhs} from './decorators/selection';
import {installDecoratorStyles} from './decorators/styles';
import {DecoratorTooltip} from './decorators/Tooltip';
import {HeaderIcon} from './HeaderIcon';

export default class Plugin {
    private disposers: Array<() => void> = [];

    public async initialize(registry: PluginRegistry, store: Store) {
        registerBuiltinDecorators();

        const {id: rhsId, showRHSPlugin, toggleRHSPlugin} = registry.registerRightHandSidebarComponent(
            RhsView,
            <RhsTitle/>,
        );
        this.disposers.push(() => registry.unregisterComponent(rhsId));
        initRhs(store, showRHSPlugin, toggleRHSPlugin);

        // No registerMessageWillFormatHook: the server already put the link in
        // the message, which is what makes it work on clients that never run
        // this bundle.
        this.disposers.push(installDecoratorClickHandler());
        this.disposers.push(installDecoratorStyles());

        // One registration for the whole plugin. A decorator gets a hover card
        // by declaring one, not by touching the bootstrap.
        const tooltipId = registry.registerLinkTooltipComponent(DecoratorTooltip);
        this.disposers.push(() => registry.unregisterComponent(tooltipId));

        const headerId = registry.registerChannelHeaderButtonAction(
            <HeaderIcon/>,
            () => {
                // Always land on the empty state, which is also the only way
                // back from a decorator panel.
                clearSelection();
                toggleRhs();
            },
            'Tactical Fusion',
            'Tactical Fusion',
        );
        this.disposers.push(() => registry.unregisterComponent(headerId));
    }

    /**
     * Releases everything initialize() claimed.
     *
     * Two separate obligations, both ours. The click handler and the injected
     * stylesheet are ours outright: without releasing them a re-registration
     * leaves the old capture listener attached and every click gets dispatched
     * twice.
     *
     * The registry components are ours because Mattermost says so. Its
     * registerPlugin calls uninitialize() on the previous instance and then
     * replaces it, but it does not touch what that instance registered, and
     * removePlugin carries the comment "The plugin is responsible for removing
     * any of its registered components." So an id that is not unregistered here
     * is a duplicate sidebar, tooltip or header button on the next
     * registration, which is what a plugin upgrade in a live tab does.
     */
    public uninitialize() {
        this.disposers.forEach((dispose) => dispose());
        this.disposers = [];
    }
}

declare global {
    interface Window {
        registerPlugin(pluginId: string, plugin: Plugin): void;
    }
}

window.registerPlugin(manifest.id, new Plugin());
