import {KINDS, KIND_LABELS, matchesShape} from './cyber';
import CyberHover from './CyberHover';
import CyberPanel from './CyberPanel';

import type {Decorator} from '../types';

export const PANEL_TITLE = 'Cyber context';

export interface CyberPayload {
    kind: string;
    value: string;
}

export function fromParams(params: URLSearchParams): CyberPayload | null {
    const kind = params.get('k') ?? '';
    const value = params.get('v') ?? '';

    if (!(KINDS as readonly string[]).includes(kind)) {
        return null;
    }
    if (!matchesShape(kind, value)) {
        return null;
    }

    return {kind, value};
}

const decorator: Decorator<CyberPayload> = {
    type: 'cyber',
    fromParams,

    summary: (payload) => KIND_LABELS[payload.kind] ?? PANEL_TITLE,

    style: {color: '#1d7a7a', background: 'rgba(29, 122, 122, 0.12)'},

    Panel: CyberPanel,

    Hover: CyberHover,
};

export default decorator;
