import {useEffect, useState} from 'react';

import type {Report} from './types';
import {fromWire} from './types';

import {pluginBaseUrl} from '../plugin_url';
import {CACHE_TTL_MS} from '../preferences/store';

export interface ReportLinkPayload {
    v: string;
    t: string;
}

export type ReportStatus = 'loading' | 'ready' | 'failed' | 'rejected';

export interface ReportState {
    status: ReportStatus;
    data: Report | null;
}

const LOADING: ReportState = {status: 'loading', data: null};

class RejectedError extends Error {}

function endpoint(payload: ReportLinkPayload): string {
    return `${pluginBaseUrl()}/api/v1/avreport?${new URLSearchParams({v: payload.v, t: payload.t}).toString()}`;
}

function cacheKey(payload: ReportLinkPayload): string {
    return `${payload.t}:${payload.v}`;
}

let fetchTimeoutMs = 10000;

async function fetchReport(payload: ReportLinkPayload): Promise<Report> {
    const controller = new AbortController();
    const timer = setTimeout(() => controller.abort(), fetchTimeoutMs);

    try {
        const response = await fetch(endpoint(payload), {
            credentials: 'same-origin',
            signal: controller.signal,
            headers: {'X-Requested-With': 'XMLHttpRequest'},
        });

        if (response.status === 400) {
            throw new RejectedError('not a report this plugin issued');
        }
        if (!response.ok) {
            throw new Error(`The server returned ${response.status}.`);
        }

        const report = fromWire(await response.json());
        if (report === null) {
            throw new Error('The server did not return a report.');
        }
        if (report.src !== payload.v || report.issuedAt !== payload.t) {
            throw new Error('The server answered about a different report.');
        }

        return report;
    } finally {
        clearTimeout(timer);
    }
}

interface CachedAnswer {
    state: ReportState;
    at: number;
}

const answers = new Map<string, CachedAnswer>();
const inflight = new Map<string, Promise<ReportState>>();

function fresh(key: string): ReportState | null {
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

function remembered(state: ReportState): boolean {
    return state.status === 'ready' || state.status === 'rejected';
}

async function load(payload: ReportLinkPayload): Promise<ReportState> {
    try {
        return {status: 'ready', data: await fetchReport(payload)};
    } catch (error: unknown) {
        return {status: error instanceof RejectedError ? 'rejected' : 'failed', data: null};
    }
}

export function request(payload: ReportLinkPayload): Promise<ReportState> {
    const key = cacheKey(payload);

    const answer = fresh(key);
    if (answer) {
        return Promise.resolve(answer);
    }

    const pending = inflight.get(key);
    if (pending) {
        return pending;
    }

    const started: Promise<ReportState> = load(payload).then((state) => {
        if (inflight.get(key) !== started) {
            return state;
        }
        if (remembered(state)) {
            answers.set(key, {state, at: Date.now()});
        }
        inflight.delete(key);
        return state;
    });

    inflight.set(key, started);

    return started;
}

export function useReport(payload: ReportLinkPayload): ReportState {
    const key = cacheKey(payload);
    const [state, setState] = useState<ReportState>(() => fresh(key) ?? LOADING);
    const [current, setCurrent] = useState(key);

    if (current !== key) {
        setCurrent(key);
        setState(fresh(key) ?? LOADING);
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

/** @internal exported for tests, which must not inherit another test's cache. */
export function _resetForTesting(): void { // eslint-disable-line no-underscore-dangle, @typescript-eslint/naming-convention
    answers.clear();
    inflight.clear();
    fetchTimeoutMs = 10000;
}

/** @internal shortens the hang timeout, so the test for it does not wait ten seconds. */
export function _setFetchTimeoutForTesting(ms: number): void { // eslint-disable-line no-underscore-dangle, @typescript-eslint/naming-convention
    fetchTimeoutMs = ms;
}
