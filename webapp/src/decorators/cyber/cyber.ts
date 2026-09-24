import {useEffect, useState} from 'react';

import type {
    CyberDataset,
    CyberLink,
    CyberReference,
    CyberResponse,
    CyberItem,
    CyberRow,
    CyberSection,
    CyberVectorMetric,
    CyberWatchEntry,
} from './types';

import {pluginBaseUrl} from '../../plugin_url';
import {CACHE_TTL_MS} from '../../preferences/store';
import {fromParams as dtgFromParams} from '../dtg';

export const KINDS = ['cve', 'cwe', 'attack', 'ip', 'hash'] as const;

export type CyberKind = typeof KINDS[number];

export function isKind(value: string): value is CyberKind {
    return (KINDS as readonly string[]).includes(value);
}

export const SHAPES: Record<CyberKind, RegExp> = {
    cve: /^(?:CVE-\d{4}-\d{4,7})$/,
    cwe: /^(?:CWE-[1-9]\d{0,4})$/,
    attack: /^(?:TA\d{4}|T\d{4}(?:\.\d{3})?)$/,
    ip: /^(?:[0-9a-f.:]{2,45})$/,
    hash: /^(?:[0-9a-f]{32}|[0-9a-f]{40}|[0-9a-f]{64})$/,
};

export const KIND_LABELS: Record<CyberKind, string> = {
    cve: 'Vulnerability',
    cwe: 'Weakness',
    attack: 'ATT&CK',
    ip: 'IP address',
    hash: 'File hash',
};

export const SEVERITIES = ['critical', 'high', 'medium', 'low', 'none'] as const;

export type CyberSeverity = typeof SEVERITIES[number];

export function isSeverity(value: string): value is CyberSeverity {
    return (SEVERITIES as readonly string[]).includes(value);
}

function asSeverity(value: string): string {
    return isSeverity(value) ? value : '';
}

export function matchesShape(kind: string, value: string): boolean {
    return isKind(kind) && SHAPES[kind].test(value);
}

export type CyberStatus = 'loading' | 'ready' | 'failed' | 'rejected';

export interface CyberState {
    status: CyberStatus;
    data: CyberResponse | null;
}

const LOADING: CyberState = {status: 'loading', data: null};

class RejectedError extends Error {}

function endpoint(kind: string, value: string): string {
    return `${pluginBaseUrl()}/api/v1/cyber?${new URLSearchParams({k: kind, v: value}).toString()}`;
}

function asObject(body: unknown, what: string): Record<string, unknown> {
    if (body === null || typeof body !== 'object' || Array.isArray(body)) {
        throw new Error(`The server did not return ${what}.`);
    }
    return body as Record<string, unknown>;
}

function asString(wire: Record<string, unknown>, field: string): string {
    const value = wire[field];
    if (typeof value !== 'string') {
        throw new Error(`The server did not return ${field}.`);
    }
    return value;
}

function asArray(wire: Record<string, unknown>, field: string): unknown[] {
    const value = wire[field];
    if (!Array.isArray(value)) {
        throw new Error(`The server did not return ${field}.`);
    }
    return value;
}

function asDtgQuery(query: string): string {
    return query !== '' && dtgFromParams(new URLSearchParams(query)) ? query : '';
}

function asRows(value: unknown[]): CyberRow[] {
    return value.map((entry) => {
        const row = asObject(entry, 'a row');
        return {label: asString(row, 'label'), value: asString(row, 'value'), query: asDtgQuery(asString(row, 'query'))};
    });
}

function asLinks(value: unknown[]): CyberLink[] {
    return value.map((entry) => {
        const link = asObject(entry, 'a related link');
        const kind = asString(link, 'kind');
        if (!isKind(kind)) {
            throw new Error('The server returned a related link of an unknown kind.');
        }

        return {
            kind,
            value: asString(link, 'value'),
            label: asString(link, 'label'),
        };
    });
}

function asWatchlist(value: unknown[]): CyberWatchEntry[] {
    return value.map((entry) => {
        const watch = asObject(entry, 'a watchlist entry');
        return {
            verdict: asString(watch, 'verdict'),
            source: asString(watch, 'source'),
            note: asString(watch, 'note'),
            updated: asString(watch, 'updated'),
            known: watch.known === true,
        };
    });
}

function asStrings(value: unknown[], what: string): string[] {
    return value.map((entry) => {
        if (typeof entry !== 'string') {
            throw new Error(`The server did not return ${what}.`);
        }
        return entry;
    });
}

export function isWebLink(url: string): boolean {
    try {
        const parsed = new URL(url);
        return (parsed.protocol === 'http:' || parsed.protocol === 'https:') && parsed.host !== '';
    } catch {
        return false;
    }
}

function asReferences(value: unknown[]): CyberReference[] {
    return value.
        map((entry) => {
            const ref = asObject(entry, 'a reference');
            return {url: asString(ref, 'url'), tags: asString(ref, 'tags')};
        }).
        filter((ref) => isWebLink(ref.url));
}

function asItem(entry: unknown): CyberItem {
    const item = asObject(entry, 'a section item');
    const kind = asString(item, 'kind');
    const value = asString(item, 'value');
    const linked = matchesShape(kind, value);

    const url = asString(item, 'url');

    return {
        head: asString(item, 'head'),
        text: asString(item, 'text'),
        kind: linked ? kind : '',
        value: linked ? value : '',
        url: isWebLink(url) ? url : '',
    };
}

function asSections(value: unknown[]): CyberSection[] {
    return value.map((entry) => {
        const section = asObject(entry, 'a section');
        return {title: asString(section, 'title'), items: asArray(section, 'items').map(asItem)};
    });
}

function asVector(value: unknown[]): CyberVectorMetric[] {
    return value.map((entry) => {
        const metric = asObject(entry, 'a vector metric');
        return {metric: asString(metric, 'metric'), value: asString(metric, 'value'), severe: metric.severe === true};
    });
}

function asDatasets(value: unknown[]): CyberDataset[] {
    return value.map((entry) => {
        const dataset = asObject(entry, 'a dataset');
        return {
            name: asString(dataset, 'name'),
            label: asString(dataset, 'label'),
            present: dataset.present === true,
            generated: asString(dataset, 'generated'),
        };
    });
}

export function asCyber(body: unknown): CyberResponse {
    const wire = asObject(body, 'an indicator');

    return {
        kind: asString(wire, 'kind'),
        value: asString(wire, 'value'),
        title: asString(wire, 'title'),
        headline: asString(wire, 'headline'),
        summary: asString(wire, 'summary'),
        status: asString(wire, 'status'),
        rows: asRows(asArray(wire, 'rows')),
        related: asLinks(asArray(wire, 'related')),
        watchlist: asWatchlist(asArray(wire, 'watchlist')),
        datasets: asDatasets(asArray(wire, 'datasets')),
        score: asString(wire, 'score'),
        severity: asSeverity(asString(wire, 'severity')),
        exploited: wire.exploited === true,
        vector: asVector(asArray(wire, 'vector')),
        affected: asStrings(asArray(wire, 'affected'), 'affected products'),
        configurations: asStrings(asArray(wire, 'configurations'), 'affected configurations'),
        references: asReferences(asArray(wire, 'references')),
        sections: asSections(asArray(wire, 'sections')),
    };
}

let fetchTimeoutMs = 10000;

async function fetchCyber(kind: string, value: string): Promise<CyberResponse> {
    const controller = new AbortController();
    const timer = setTimeout(() => controller.abort(), fetchTimeoutMs);

    try {
        const response = await fetch(endpoint(kind, value), {
            credentials: 'same-origin',
            signal: controller.signal,
            headers: {'X-Requested-With': 'XMLHttpRequest'},
        });

        if (response.status === 400) {
            throw new RejectedError('not an indicator this plugin issued');
        }
        if (!response.ok) {
            throw new Error(`The server returned ${response.status}.`);
        }

        const answer = asCyber(await response.json());

        if (answer.kind !== kind || answer.value !== value) {
            throw new Error('The server answered about a different indicator.');
        }

        return answer;
    } finally {
        clearTimeout(timer);
    }
}

interface CachedAnswer {
    state: CyberState;
    at: number;
}

const answers = new Map<string, CachedAnswer>();
const inflight = new Map<string, Promise<CyberState>>();

function keyFor(kind: string, value: string): string {
    return `${kind}:${value}`;
}

function now(): number {
    return Date.now();
}

function fresh(key: string): CyberState | null {
    const cached = answers.get(key);
    if (!cached) {
        return null;
    }
    if (now() - cached.at >= CACHE_TTL_MS) {
        answers.delete(key);
        return null;
    }

    return cached.state;
}

function remembered(state: CyberState): boolean {
    return state.status === 'ready' || state.status === 'rejected';
}

async function load(kind: string, value: string): Promise<CyberState> {
    try {
        return {status: 'ready', data: await fetchCyber(kind, value)};
    } catch (error: unknown) {
        return {
            status: error instanceof RejectedError ? 'rejected' : 'failed',
            data: null,
        };
    }
}

export function request(kind: string, value: string): Promise<CyberState> {
    const key = keyFor(kind, value);

    const answer = fresh(key);
    if (answer) {
        return Promise.resolve(answer);
    }

    const pending = inflight.get(key);
    if (pending) {
        return pending;
    }

    const started: Promise<CyberState> = load(kind, value).then((state) => {
        if (inflight.get(key) !== started) {
            return state;
        }
        if (remembered(state)) {
            answers.set(key, {state, at: now()});
        }
        inflight.delete(key);

        return state;
    });

    inflight.set(key, started);

    return started;
}

export function useCyber(kind: string, value: string): CyberState {
    const key = keyFor(kind, value);

    const [state, setState] = useState<CyberState>(() => fresh(key) ?? LOADING);
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

        request(kind, value).then((answer) => {
            if (live) {
                setState(answer);
            }
        });

        return () => {
            live = false;
        };
    }, [kind, value, key]);

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
