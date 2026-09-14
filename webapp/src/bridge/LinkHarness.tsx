import React from 'react';

import {_resetForTesting as resetBridge} from './client';
import {Link} from './Link';

import {registerBuiltinDecorators} from '../decorators/index';
import {_resetForTesting as resetDecorators} from '../decorators/registry';

const DTG_HREF = '/plugins/com.mattermost.plugin-tactical-fusion/decorate/dtg';

const linkedTokens = new Set(['091630ZAUG26']);

window.fetch = (async (_input: RequestInfo | URL, init?: RequestInit) => {
    const request = JSON.parse(String(init?.body ?? '{}')) as {token: string; label?: string};

    if (!linkedTokens.has(request.token)) {
        return {
            ok: false,
            status: 422,
            json: async () => ({message: 'Not recognized. (TF-19006)', code: 19006, reason: 'not_recognized'}),
        } as Response;
    }

    const url = `${DTG_HREF}?a=&dtg=${request.token}&t=${Date.now() + 5_400_000}&z=Z`;
    const label = request.label || request.token;

    return {
        ok: true,
        status: 200,
        json: async () => ({markdown: `[${label}](${url})`, url, type: 'dtg', label}),
    } as Response;
}) as typeof fetch;

const LinkHarness: React.FC<{token: string; label?: string; fallback?: string}> = ({token, label, fallback}) => {
    resetDecorators();
    registerBuiltinDecorators();
    resetBridge();

    return (
        <div style={{width: 320, padding: 16, paddingBottom: 90, fontFamily: 'sans-serif'}}>
            <span>{'Arrival '}</span>
            <Link
                type='dtg'
                token={token}
                label={label}
                fallback={fallback}
            />
        </div>
    );
};

export default LinkHarness;
