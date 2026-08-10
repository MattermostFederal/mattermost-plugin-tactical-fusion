import manifest from 'manifest';
import React from 'react';

import type {PluginRegistry} from 'types/mattermost-webapp';

import {HeaderIcon} from './HeaderIcon';

export default class Plugin {
    public async initialize(registry: PluginRegistry) {
        registry.registerChannelHeaderButtonAction(
            <HeaderIcon/>,
            () => {
                // eslint-disable-next-line no-alert
                window.alert('Hello from Mission Context!');
            },
            'Mission Context',
            'Mission Context',
        );
    }
}

declare global {
    interface Window {
        registerPlugin(pluginId: string, plugin: Plugin): void;
    }
}

window.registerPlugin(manifest.id, new Plugin());
