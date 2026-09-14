import manifest from 'manifest';

import {decorate, link} from './client';
import {Link} from './Link';
import {BRIDGE_API_VERSION, GLOBAL_NAME, READY_EVENT} from './types';
import type {TacticalFusionApi} from './types';

import {all} from '../decorators/registry';

export interface BridgeHost {
    [GLOBAL_NAME]?: TacticalFusionApi;
    dispatchEvent(event: Event): boolean;
}

export function createBridgeApi(): TacticalFusionApi {
    return Object.freeze({
        apiVersion: BRIDGE_API_VERSION,
        version: String(manifest.version ?? ''),
        types: Object.freeze(all().map((decorator) => decorator.type)),
        decorate,
        link,
        Link,
    });
}

export function installBridgeGlobal(host: BridgeHost = window): () => void {
    const api = createBridgeApi();
    host[GLOBAL_NAME] = api;
    host.dispatchEvent(new CustomEvent(READY_EVENT, {detail: api}));

    return () => {
        if (host[GLOBAL_NAME] === api) {
            delete host[GLOBAL_NAME];
        }
    };
}
