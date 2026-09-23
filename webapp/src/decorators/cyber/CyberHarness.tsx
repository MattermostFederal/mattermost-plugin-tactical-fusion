import React from 'react';

import {_resetForTesting as resetCyber} from './cyber';
import CyberHover from './CyberHover';
import CyberPanel from './CyberPanel';
import {_resetForTesting as resetMentions} from './mentions';

import {
    _resetForTesting as resetSelection,
    initRhs,
    subscribe,
} from '../selection';
import type {Selection} from '../selection';

import type {CyberPayload} from './index';

type Reply = 'found' | 'bare' | 'status' | 'rejected' | 'failed' | 'hold';
type MentionsReply = 'some' | 'none' | 'failed' | 'hold';

const HEADLINE = '10.0 Critical, in KEV';
const SUMMARY = 'Remote code execution in a logging library.';

const FOUND = {
    kind: 'cve',
    value: 'CVE-2021-44228',
    title: 'CVE-2021-44228',
    headline: HEADLINE,
    summary: SUMMARY,
    status: '',
    rows: [
        {label: 'CVSS', value: '10.0 Critical'},
        {label: 'Published', value: '2021-12-10'},
    ],
    related: [{kind: 'cwe', value: 'CWE-502', label: 'CWE-502 Deserialization of Untrusted Data'}],
    watchlist: [{
        verdict: 'malicious',
        source: 'internal',
        note: 'seen beaconing',
        updated: '2026-08-01',
        known: true,
    }],
    datasets: [
        {name: 'cve', label: 'vulnerability', present: true, generated: '2026-09-01T00:00:00Z'},
        {name: 'ip', label: 'IP address', present: false, generated: ''},
    ],
};

const BARE = {
    ...FOUND,
    summary: '',
    rows: [],
    related: [],
    watchlist: [],
};

const NO_DATASET = {
    ...BARE,
    headline: 'No vulnerability dataset is installed.',
    status: 'No vulnerability dataset is installed.',
    datasets: [{name: 'cve', label: 'vulnerability', present: false, generated: ''}],
};

const MENTIONS = {
    value: 'CVE-2021-44228',
    mentions: [{
        post_id: 'post1',
        channel_id: 'channel1',
        channel: 'Incident 4821',
        create_at: 1767225600000,
        snippet: 'We are patching CVE-2021-44228 on the edge tonight.',
        permalink: '/ops/pl/post1',
    }],
    truncated: false,
};

function baseFor(reply: Reply): typeof FOUND {
    if (reply === 'status') {
        return NO_DATASET;
    }
    if (reply === 'bare') {
        return BARE;
    }

    return FOUND;
}

function bodyFor(reply: Reply, kind: string, value: string): unknown {
    const base = baseFor(reply);

    return {...base, kind, value, title: base.title === base.value ? value : base.title};
}

function paramOf(url: string, name: string): string {
    return new URLSearchParams(url.slice(url.indexOf('?'))).get(name) ?? '';
}

let onRequest: (() => void) | null = null;
let onMentionsRequest: (() => void) | null = null;
let setups = 0;

const teamListeners = new Set<() => void>();
let currentTeam = '';

const testStore = {
    getState: () => ({entities: {teams: {currentTeamId: currentTeam}}}),
    dispatch: () => undefined,
    subscribe: (listener: () => void) => {
        teamListeners.add(listener);
        return () => {
            teamListeners.delete(listener);
        };
    },
    replaceReducer: () => undefined,
};

function switchTeam(id: string): void {
    currentTeam = id;
    teamListeners.forEach((listener) => listener());
}

const CyberHarness: React.FC<{
    surface: 'panel' | 'hover';
    payload: CyberPayload;
    second?: CyberPayload;
    reply?: Reply;
    mentions?: MentionsReply;
    team?: string;
    nextTeam?: string;
}> = ({surface, payload, second, reply = 'found', mentions = 'none', team = '', nextTeam}) => {
    const [requests, setRequests] = React.useState(0);
    const [mentionRequests, setMentionRequests] = React.useState(0);
    const [shown, setShown] = React.useState(payload);

    React.useState(() => {
        setups += 1;
        resetCyber();
        resetMentions();
        resetSelection();

        teamListeners.clear();
        currentTeam = team;
        initRhs(testStore as never, null);

        onRequest = () => setRequests((n) => n + 1);
        onMentionsRequest = () => setMentionRequests((n) => n + 1);

        const real = globalThis.fetch;
        globalThis.fetch = (async (input: RequestInfo | URL, init?: RequestInit) => {
            const url = String(input);

            if (url.includes('/api/v1/cyber/mentions')) {
                onMentionsRequest?.();
                if (mentions === 'hold') {
                    return new Promise<Response>(() => undefined);
                }
                if (mentions === 'failed') {
                    throw new Error('offline');
                }

                return {
                    status: 200,
                    ok: true,
                    json: async () => (mentions === 'some' ? MENTIONS : {...MENTIONS, mentions: []}),
                } as unknown as Response;
            }

            if (!url.includes('/api/v1/cyber')) {
                return real(input, init);
            }

            onRequest?.();
            if (reply === 'hold') {
                return new Promise<Response>(() => undefined);
            }
            if (reply === 'failed') {
                throw new Error('offline');
            }
            if (reply === 'rejected') {
                return {status: 400, ok: false} as Response;
            }

            return {
                status: 200,
                ok: true,
                json: async () => bodyFor(reply, paramOf(url, 'k'), paramOf(url, 'v')),
            } as unknown as Response;
        }) as typeof globalThis.fetch;

        return setups;
    });

    const [selection, setSelected] = React.useState<Selection | null>(null);
    React.useEffect(() => subscribe(setSelected), []);

    const Surface = surface === 'panel' ? CyberPanel : CyberHover;

    return (
        <div>
            <Surface payload={shown}/>
            <p data-testid='requests'>{requests}</p>
            <p data-testid='mention-requests'>{mentionRequests}</p>
            <p data-testid='selection'>
                {selection ? `${selection.type}:${JSON.stringify(selection.payload)}` : 'none'}
            </p>
            {second && (
                <button
                    type='button'
                    onClick={() => setShown(second)}
                >{'Show the second'}</button>
            )}
            {nextTeam !== undefined && (
                <button
                    type='button'
                    onClick={() => switchTeam(nextTeam)}
                >{'Switch team'}</button>
            )}
        </div>
    );
};

export default CyberHarness;
