import {useEffect, useState} from 'react';

import type {CyberMention, CyberMentionsResponse} from './types';

import {pluginBaseUrl} from '../../plugin_url';
import {CACHE_TTL_MS} from '../../preferences/store';

export type MentionsStatus = 'idle' | 'loading' | 'ready' | 'failed';

export interface MentionsState {
    status: MentionsStatus;
    data: CyberMentionsResponse | null;
}

const IDLE: MentionsState = {status: 'idle', data: null};
const LOADING: MentionsState = {status: 'loading', data: null};

function endpoint(team: string, kind: string, value: string): string {
    const query = new URLSearchParams({k: kind, v: value, team});
    return `${pluginBaseUrl()}/api/v1/cyber/mentions?${query.toString()}`;
}

function asMention(entry: unknown): CyberMention {
    if (entry === null || typeof entry !== 'object' || Array.isArray(entry)) {
        throw new Error('The server did not return a mention.');
    }

    const wire = entry as Record<string, unknown>;
    const text = (field: string): string => {
        const value = wire[field];
        if (typeof value !== 'string') {
            throw new Error(`The server did not return ${field}.`);
        }
        return value;
    };

    return {
        postId: text('post_id'),
        channelId: text('channel_id'),
        channel: text('channel'),
        createAt: typeof wire.create_at === 'number' ? wire.create_at : 0,
        snippet: text('snippet'),
        permalink: text('permalink'),
    };
}

export function asMentions(body: unknown): CyberMentionsResponse {
    if (body === null || typeof body !== 'object' || Array.isArray(body)) {
        throw new Error('The server did not return mentions.');
    }

    const wire = body as Record<string, unknown>;
    if (typeof wire.value !== 'string' || !Array.isArray(wire.mentions)) {
        throw new Error('The server did not return mentions.');
    }

    return {
        value: wire.value,
        mentions: wire.mentions.map(asMention),
        truncated: wire.truncated === true,
    };
}

let fetchTimeoutMs = 10000;

async function fetchMentions(team: string, kind: string, value: string): Promise<CyberMentionsResponse> {
    const controller = new AbortController();
    const timer = setTimeout(() => controller.abort(), fetchTimeoutMs);

    try {
        const response = await fetch(endpoint(team, kind, value), {
            credentials: 'same-origin',
            signal: controller.signal,
            headers: {'X-Requested-With': 'XMLHttpRequest'},
        });

        if (!response.ok) {
            throw new Error(`The server returned ${response.status}.`);
        }

        return asMentions(await response.json());
    } finally {
        clearTimeout(timer);
    }
}

interface CachedAnswer {
    state: MentionsState;
    at: number;
}

const answers = new Map<string, CachedAnswer>();
const inflight = new Map<string, Promise<MentionsState>>();

function keyFor(team: string, kind: string, value: string): string {
    return `${team}:${kind}:${value}`;
}

function fresh(key: string): MentionsState | null {
    const cached = answers.get(key);
    if (!cached) {
        return null;
    }
    if (Date.now() - cached.at >= CACHE_TTL_MS) {
        answers.delete(key);
        return null;
    }

    return cached.state;
}

async function load(team: string, kind: string, value: string): Promise<MentionsState> {
    try {
        return {status: 'ready', data: await fetchMentions(team, kind, value)};
    } catch {
        return {status: 'failed', data: null};
    }
}

export function request(team: string, kind: string, value: string): Promise<MentionsState> {
    const key = keyFor(team, kind, value);

    const answer = fresh(key);
    if (answer) {
        return Promise.resolve(answer);
    }

    const pending = inflight.get(key);
    if (pending) {
        return pending;
    }

    const started: Promise<MentionsState> = load(team, kind, value).then((state) => {
        if (inflight.get(key) !== started) {
            return state;
        }
        if (state.status === 'ready') {
            answers.set(key, {state, at: Date.now()});
        }
        inflight.delete(key);

        return state;
    });

    inflight.set(key, started);

    return started;
}

export function useMentions(team: string, kind: string, value: string): MentionsState {
    const key = keyFor(team, kind, value);

    const [state, setState] = useState<MentionsState>(() => (team ? fresh(key) ?? LOADING : IDLE));
    const [current, setCurrent] = useState(key);

    if (current !== key) {
        setCurrent(key);
        setState(team ? fresh(key) ?? LOADING : IDLE);
    }

    useEffect(() => {
        if (!team) {
            setState(IDLE);
            return undefined;
        }

        const answered = fresh(key);
        if (answered) {
            setState(answered);
            return undefined;
        }

        let live = true;

        request(team, kind, value).then((answer) => {
            if (live) {
                setState(answer);
            }
        });

        return () => {
            live = false;
        };
    }, [team, kind, value, key]);

    return state;
}

export function _resetForTesting(): void { // eslint-disable-line no-underscore-dangle, @typescript-eslint/naming-convention
    answers.clear();
    inflight.clear();
    fetchTimeoutMs = 10000;
}

export function _setFetchTimeoutForTesting(ms: number): void { // eslint-disable-line no-underscore-dangle, @typescript-eslint/naming-convention
    fetchTimeoutMs = ms;
}
