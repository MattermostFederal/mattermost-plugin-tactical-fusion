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

        const {showRHSPlugin, toggleRHSPlugin} = registry.registerRightHandSidebarComponent(
            RhsView,
            <RhsTitle/>,
        );
        initRhs(store, showRHSPlugin, toggleRHSPlugin);

        // No registerMessageWillFormatHook: the server already put the link in
        // the message, which is what makes it work on clients that never run
        // this bundle.
        this.disposers.push(installDecoratorClickHandler());
        this.disposers.push(installDecoratorStyles());

        // One registration for the whole plugin. A decorator gets a hover card
        // by declaring one, not by touching the bootstrap.
        registry.registerLinkTooltipComponent(DecoratorTooltip);

        registry.registerChannelHeaderButtonAction(
            <HeaderIcon/>,
            () => {
                // Always land on the empty state, which is also the only way
                // back from a decorator panel.
                clearSelection();
                toggleRhs();
            },
            'Mission Context',
            'Mission Context',
        );
    }

    // Without this, a re-registration leaves the old capture listener attached
    // and every click gets dispatched twice.
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
