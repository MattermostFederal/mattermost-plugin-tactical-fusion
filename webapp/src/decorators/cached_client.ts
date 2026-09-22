import {useEffect, useState} from 'react';

import {CACHE_TTL_MS} from '../preferences/store';

export type AnswerStatus = 'loading' | 'ready' | 'failed' | 'rejected';

export interface Answer<T> {
    status: AnswerStatus;
    data: T | null;
}

export const MAX_CACHED_ANSWERS = 256;

const DEFAULT_TIMEOUT_MS = 10000;

export class RejectedError extends Error {}

interface Spec<P, T> {
    endpoint: (payload: P) => string;
    cacheKey: (payload: P) => string;
    read: (body: unknown, payload: P) => T;
}

interface Cached<T> {
    state: Answer<T>;
    at: number;
}

export interface CachedClient<P, T> {
    request: (payload: P) => Promise<Answer<T>>;
    useAnswer: (payload: P) => Answer<T>;
    _resetForTesting: () => void;
    _setFetchTimeoutForTesting: (ms: number) => void;
}

export function createCachedClient<P, T>(spec: Spec<P, T>): CachedClient<P, T> {
    const loading: Answer<T> = {status: 'loading', data: null};
    const answers = new Map<string, Cached<T>>();
    const inflight = new Map<string, Promise<Answer<T>>>();
    let fetchTimeoutMs = DEFAULT_TIMEOUT_MS;

    async function fetchAnswer(payload: P): Promise<T> {
        const controller = new AbortController();
        const timer = setTimeout(() => controller.abort(), fetchTimeoutMs);

        try {
            const response = await fetch(spec.endpoint(payload), {
                credentials: 'same-origin',
                signal: controller.signal,
                headers: {'X-Requested-With': 'XMLHttpRequest'},
            });

            if (response.status === 400) {
                throw new RejectedError('not a token this plugin issued');
            }
            if (!response.ok) {
                throw new Error(`The server returned ${response.status}.`);
            }

            return spec.read(await response.json(), payload);
        } finally {
            clearTimeout(timer);
        }
    }

    function fresh(key: string): Answer<T> | null {
        const cached = answers.get(key);
        if (!cached || Date.now() - cached.at >= CACHE_TTL_MS) {
            return null;
        }
        return cached.state;
    }

    function remember(key: string, state: Answer<T>): void {
        if (state.status !== 'ready' && state.status !== 'rejected') {
            return;
        }
        answers.delete(key);
        answers.set(key, {state, at: Date.now()});
        while (answers.size > MAX_CACHED_ANSWERS) {
            const oldest = answers.keys().next().value;
            if (oldest === undefined) {
                break;
            }
            answers.delete(oldest);
        }
    }

    async function load(payload: P): Promise<Answer<T>> {
        try {
            return {status: 'ready', data: await fetchAnswer(payload)};
        } catch (error: unknown) {
            return {status: error instanceof RejectedError ? 'rejected' : 'failed', data: null};
        }
    }

    function request(payload: P): Promise<Answer<T>> {
        const key = spec.cacheKey(payload);

        const answer = fresh(key);
        if (answer) {
            return Promise.resolve(answer);
        }
        answers.delete(key);

        const pending = inflight.get(key);
        if (pending) {
            return pending;
        }

        const started: Promise<Answer<T>> = load(payload).then((state) => {
            if (inflight.get(key) !== started) {
                return state;
            }
            remember(key, state);
            inflight.delete(key);
            return state;
        });

        inflight.set(key, started);

        return started;
    }

    function useAnswer(payload: P): Answer<T> {
        const key = spec.cacheKey(payload);
        const [state, setState] = useState<Answer<T>>(() => fresh(key) ?? loading);
        const [current, setCurrent] = useState(key);

        if (current !== key) {
            setCurrent(key);
            setState(fresh(key) ?? loading);
        }

        useEffect(() => {
            const answered = fresh(key);
            if (answered) {
                setState(answered);
                return undefined;
            }

            let live = true;

            request(payload).then((answer) => {
                if (live) {
                    setState(answer);
                }
            });

            return () => {
                live = false;
            };
        }, [key]); // eslint-disable-line react-hooks/exhaustive-deps

        return state;
    }

    return {
        request,
        useAnswer,
        _resetForTesting: () => {
            answers.clear();
            inflight.clear();
            fetchTimeoutMs = DEFAULT_TIMEOUT_MS;
        },
        _setFetchTimeoutForTesting: (ms: number) => {
            fetchTimeoutMs = ms;
        },
    };
}
