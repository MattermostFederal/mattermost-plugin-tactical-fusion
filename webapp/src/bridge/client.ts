import type {
    DecorateOptions,
    DecorateRequest,
    DeclineReason,
    DecoratedText,
    DecoratorLink,
    LinkOptions,
    LinkRequest,
    ReferenceTime,
} from './types';

import {pluginBaseUrl} from '../plugin_url';

const DECLINE_REASONS: readonly DeclineReason[] = ['unknown_type', 'not_recognized', 'disabled'];

export const LINK_CACHE_TTL_MS = 5 * 60 * 1000;

export const LINK_CACHE_LIMIT = 500;

let fetchTimeoutMs = 10000;

let now: () => number = () => Date.now();

interface CachedLink {
    at: number;
    promise: Promise<DecoratorLink>;
}

const linkCache = new Map<string, CachedLink>();

export class TacticalFusionError extends Error {
    readonly status: number;

    readonly code: number;

    readonly reason: DeclineReason | null;

    constructor(message: string, status: number, code: number, reason: DeclineReason | null) {
        super(message);
        this.name = 'TacticalFusionError';
        this.status = status;
        this.code = code;
        this.reason = reason;
    }
}

export function decorate(message: string, options: DecorateOptions = {}): Promise<DecoratedText> {
    const body: DecorateRequest = {message};
    const reference = toMillis(options.referenceTime);
    if (reference !== undefined) {
        body.reference_time = reference;
    }

    return post('decorate', body, options.signal).then(readDecorated);
}

export function link(type: string, token: string, options: LinkOptions = {}): Promise<DecoratorLink> {
    const body: LinkRequest = {type, token};
    if (options.label) {
        body.label = options.label;
    }
    const reference = toMillis(options.referenceTime);
    if (reference !== undefined) {
        body.reference_time = reference;
    }

    if (options.signal) {
        return post('link', body, options.signal).then(readLink);
    }

    const key = JSON.stringify([type, token, body.label ?? '', reference ?? null]);
    const cached = linkCache.get(key);
    if (cached && now() - cached.at < LINK_CACHE_TTL_MS) {
        return cached.promise;
    }

    const promise = post('link', body).then(readLink);
    linkCache.delete(key);
    linkCache.set(key, {at: now(), promise});
    evictOldest();

    promise.catch((error: unknown) => {
        if (!(error instanceof TacticalFusionError && error.reason) && linkCache.get(key)?.promise === promise) {
            linkCache.delete(key);
        }
    });

    return promise;
}

function evictOldest(): void {
    while (linkCache.size > LINK_CACHE_LIMIT) {
        const oldest = linkCache.keys().next().value;
        if (oldest === undefined) {
            return;
        }
        linkCache.delete(oldest);
    }
}

function toMillis(reference: ReferenceTime | undefined): number | undefined {
    if (reference === undefined) {
        return undefined;
    }

    const value = reference instanceof Date ? reference.getTime() : reference;
    if (!Number.isFinite(value) || value <= 0) {
        return undefined;
    }

    return Math.trunc(value);
}

async function post(route: string, body: unknown, signal?: AbortSignal): Promise<unknown> {
    const controller = new AbortController();
    const timer = setTimeout(() => controller.abort(), fetchTimeoutMs);
    const forwardAbort = () => controller.abort();
    if (signal?.aborted) {
        controller.abort();
    }
    signal?.addEventListener('abort', forwardAbort, {once: true});

    try {
        const response = await fetch(`${pluginBaseUrl()}/api/v1/${route}`, {
            method: 'POST',
            credentials: 'same-origin',
            signal: controller.signal,
            headers: {
                'X-Requested-With': 'XMLHttpRequest',
                'Content-Type': 'application/json',
            },
            body: JSON.stringify(body),
        });

        const payload: unknown = await response.json().catch(() => null);
        if (!response.ok) {
            throw errorFrom(response.status, payload);
        }

        return payload;
    } finally {
        clearTimeout(timer);
        signal?.removeEventListener('abort', forwardAbort);
    }
}

function errorFrom(status: number, payload: unknown): TacticalFusionError {
    const wire = asRecord(payload);
    const message = typeof wire?.message === 'string' && wire.message ? wire.message : `The server returned ${status}.`;
    const code = typeof wire?.code === 'number' ? wire.code : 0;
    const reason = DECLINE_REASONS.find((known) => known === wire?.reason) ?? null;

    return new TacticalFusionError(message, status, code, reason);
}

function readDecorated(payload: unknown): DecoratedText {
    const wire = requireRecord(payload);

    return {
        message: field(wire, 'message', 'string'),
        changed: field(wire, 'changed', 'boolean'),
        fitsPost: field(wire, 'fits_post', 'boolean'),
    };
}

function readLink(payload: unknown): DecoratorLink {
    const wire = requireRecord(payload);

    return {
        markdown: field(wire, 'markdown', 'string'),
        url: field(wire, 'url', 'string'),
        type: field(wire, 'type', 'string'),
        label: field(wire, 'label', 'string'),
    };
}

function asRecord(payload: unknown): Record<string, unknown> | null {
    if (payload === null || typeof payload !== 'object' || Array.isArray(payload)) {
        return null;
    }
    return payload as Record<string, unknown>;
}

function requireRecord(payload: unknown): Record<string, unknown> {
    const wire = asRecord(payload);
    if (!wire) {
        throw new TacticalFusionError('The server sent something that is not a bridge response.', 0, 0, null);
    }
    return wire;
}

function field(wire: Record<string, unknown>, key: string, kind: 'string'): string;
function field(wire: Record<string, unknown>, key: string, kind: 'boolean'): boolean;
function field(wire: Record<string, unknown>, key: string, kind: 'string' | 'boolean'): string | boolean {
    const value = Object.hasOwn(wire, key) ? wire[key] : undefined;
    if (typeof value !== kind) {
        throw new TacticalFusionError(`The server sent no ${key}.`, 0, 0, null);
    }
    return value as string | boolean;
}

export function _resetForTesting(): void { // eslint-disable-line no-underscore-dangle, @typescript-eslint/naming-convention
    linkCache.clear();
    now = () => Date.now();
    fetchTimeoutMs = 10000;
}

export function _setClockForTesting(clock: () => number): void { // eslint-disable-line no-underscore-dangle, @typescript-eslint/naming-convention
    now = clock;
}

export function _setFetchTimeoutForTesting(ms: number): void { // eslint-disable-line no-underscore-dangle, @typescript-eslint/naming-convention
    fetchTimeoutMs = ms;
}
