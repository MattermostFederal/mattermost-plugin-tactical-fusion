import manifest from 'manifest';
import React from 'react';
import type {Store} from 'redux';

import type {PluginRegistry} from 'types/mattermost-webapp';

import PackageUploader from './admin/PackageUploader';
import {RhsTitle, RhsView} from './components/rhs/RhsView';
import CotPostBody from './cot/CotPostBody';
import {registerCotPanel} from './cot/index';
import {COT_POST_TYPE} from './cot/types';
import {installDecoratorClickHandler} from './decorators/click_handler';
import {registerBuiltinDecorators} from './decorators/index';
import {DecoratorPostBody} from './decorators/PostBody';
import {all} from './decorators/registry';
import {clearSelection, initRhs, toggleRhs} from './decorators/selection';
import {installDecoratorStyles} from './decorators/styles';
import {DecoratorTooltip} from './decorators/Tooltip';
import GeoJsonPostBody from './geojson/GeoJsonPostBody';
import {registerGeoJsonPanel} from './geojson/panel';
import {GEOJSON_POST_TYPE} from './geojson/types';
import {HeaderIcon} from './HeaderIcon';
import {staticBaseUrl} from './plugin_url';

/*
 * Where the browser fetches this bundle's lazy chunks from.
 *
 * Assigned here and nowhere else. It has to run before the first dynamic
 * import, and `plugin_url.ts` is imported by `convert.ts`, which the component
 * tests mount through Vite as strict-mode ESM, where a module-scope assignment
 * to this webpack-only free variable is a ReferenceError.
 */
/* eslint-disable no-underscore-dangle, @typescript-eslint/naming-convention, @typescript-eslint/no-unused-vars */
declare let __webpack_public_path__: string;
declare const __webpack_require__: unknown;
if (typeof __webpack_require__ !== 'undefined') {
    __webpack_public_path__ = staticBaseUrl();
}
/* eslint-enable no-underscore-dangle, @typescript-eslint/naming-convention, @typescript-eslint/no-unused-vars */

export default class Plugin {
    private disposers: Array<() => void> = [];

    public async initialize(registry: PluginRegistry, store: Store) {
        registerBuiltinDecorators();
        registerCotPanel();
        registerGeoJsonPanel();

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

        // One per declared post type, because that is what the registry keys
        // on. A type this build does not register falls through to
        // Mattermost's own markdown, which is the fallback for an older bundle
        // against a newer server.
        for (const decorator of all()) {
            if (!decorator.postType || !decorator.Inline) {
                continue;
            }
            const bodyId = registry.registerPostTypeComponent(decorator.postType, DecoratorPostBody);
            this.disposers.push(() => registry.unregisterPostTypeComponent(bodyId));
        }

        // Registered unconditionally, the posture every decorator takes with its
        // switches off: a post already stamped must keep rendering after an
        // admin turns the feature off, or its body falls through to raw XML.
        const cotId = registry.registerPostTypeComponent(COT_POST_TYPE, CotPostBody);
        this.disposers.push(() => registry.unregisterPostTypeComponent(cotId));

        const geoJsonId = registry.registerPostTypeComponent(GEOJSON_POST_TYPE, GeoJsonPostBody);
        this.disposers.push(() => registry.unregisterPostTypeComponent(geoJsonId));

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

        // The System Console control for detail map packages. `custom` is the
        // only setting type that can carry a file; every other type is a
        // string, a number or a switch.
        registry.registerAdminConsoleCustomSetting('LocationMapPackages', PackageUploader, {showTitle: true});
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
