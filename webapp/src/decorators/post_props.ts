export const POST_PROPS_KEY = 'tactical_fusion';

export const POST_PROPS_VERSION = 1;

const RESERVED = new Set(['version', 'type']);

export interface StandalonePayload {
    type: string;
    params: URLSearchParams;
}

export function standalonePayload(props: unknown): StandalonePayload | null {
    if (typeof props !== 'object' || props === null) {
        return null;
    }

    const blob = (props as Record<string, unknown>)[POST_PROPS_KEY];
    if (typeof blob !== 'object' || blob === null) {
        return null;
    }

    const fields = blob as Record<string, unknown>;
    if (Number(fields.version) !== POST_PROPS_VERSION) {
        return null;
    }

    const type = fields.type;
    if (typeof type !== 'string' || type === '') {
        return null;
    }

    const params = new URLSearchParams();
    for (const [key, value] of Object.entries(fields)) {
        if (!RESERVED.has(key) && typeof value === 'string') {
            params.set(key, value);
        }
    }

    return {type, params};
}
