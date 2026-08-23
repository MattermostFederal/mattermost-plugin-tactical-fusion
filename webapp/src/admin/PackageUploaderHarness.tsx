import React from 'react';

import {AFTER_REMOVAL, AFTER_UPLOAD, INSTALLED, REFUSAL, REMOVABLE, SHADOWING} from './package_fixtures';
import PackageUploader from './PackageUploader';

/**
 * Harness for the System Console package control.
 *
 * Everything the control shows comes from one route answered three ways, so the
 * whole harness is a stub of it plus the two reply modes. The list and the
 * writes are separate props because they fail differently: a list that cannot
 * be read leaves an admin with a stale screen, and a write that cannot be made
 * has to say why, so a test needs to pin one while it varies the other.
 */

export type ListReply =
    | 'ok'
    | 'empty'
    | 'refused'
    | 'offline'
    | 'not-an-array'
    | 'mixed-types'
    | 'shadowing';

export type WriteReply =
    | 'ok'
    | 'refused'
    | 'refused-silently'
    | 'offline'
    | 'hold';

/** Which list a write answers with once it has succeeded. */
export type WriteResult = 'uploaded' | 'removed';

interface Props {
    list?: ListReply;
    write?: WriteReply;
    result?: WriteResult;
}

function listBody(list: ListReply): unknown {
    if (list === 'empty') {
        return {packages: [], removable: []};
    }
    if (list === 'not-an-array') {
        return {packages: 'indopacom-hawaii'};
    }
    if (list === 'shadowing') {
        return {packages: SHADOWING, removable: SHADOWING};
    }
    if (list === 'mixed-types') {
        // No `removable` at all, so the non-array arm of names() runs too.
        return {packages: [7, 'indopacom-hawaii', null, {name: 'indopacom-guam'}]};
    }

    return {packages: INSTALLED, removable: REMOVABLE};
}

const PackageUploaderHarness: React.FC<Props> = ({list = 'ok', write = 'ok', result = 'uploaded'}) => {
    const [requests, setRequests] = React.useState<string[]>([]);

    // Before the first render that mounts the control, so its opening request
    // is answered by the stub rather than escaping to the network.
    const [ready] = React.useState(() => {
        const real = globalThis.fetch;

        globalThis.fetch = (async (input: RequestInfo | URL, init?: RequestInit) => {
            const url = String(input);
            if (!url.includes('/api/v1/packages')) {
                return real(input, init);
            }

            const method = init?.method ?? 'GET';
            setRequests((seen) => [...seen, `${method} ${url.slice(url.indexOf('/plugins/'))}`]);

            if (method === 'GET') {
                if (list === 'offline') {
                    throw new Error('offline');
                }
                if (list === 'refused') {
                    return {ok: false, status: 403} as Response;
                }

                return {
                    ok: true,
                    status: 200,
                    json: async () => listBody(list),
                } as unknown as Response;
            }

            if (write === 'offline') {
                throw new Error('offline');
            }
            if (write === 'hold') {
                // Never settles, which is what makes the in-flight state
                // observable rather than raced against a reply.
                return new Promise<Response>(() => {});
            }
            if (write === 'refused') {
                return {
                    ok: false,
                    status: 400,
                    json: async () => ({message: REFUSAL}),
                } as unknown as Response;
            }
            if (write === 'refused-silently') {
                return {
                    ok: false,
                    status: 400,
                    json: async () => ({}),
                } as unknown as Response;
            }

            return {
                ok: true,
                status: 200,
                json: async () => (result === 'removed' ? AFTER_REMOVAL : AFTER_UPLOAD),
            } as unknown as Response;
        }) as typeof globalThis.fetch;

        return true;
    });

    if (!ready) {
        return null;
    }

    return (
        <div>
            <PackageUploader/>
            <p data-testid='requests'>{requests.join(' | ')}</p>
        </div>
    );
};

export default PackageUploaderHarness;
