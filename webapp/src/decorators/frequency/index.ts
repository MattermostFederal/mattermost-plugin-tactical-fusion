import {bandOf, channelOf, mhzText, allocationOf} from './bands';
import FrequencyHover from './FrequencyHover';
import FrequencyPanel from './FrequencyPanel';

import type {Decorator} from '../types';

export const PANEL_TITLE = 'Frequency';

export const TOKEN = /^(\d{1,4}\.\d{1,3}|\d{4,5})(?:[ \t]*(MHZ|KHZ))?$/;

export const MIN_KHZ = 2000;

export const MAX_KHZ = 1300000;

export interface FrequencyPayload {
    token: string;
    khz: number;
}

export interface FrequencyDetails {
    token: string;
    mhz: string;
    khz: string;
    band: string;
    channel: string;
    use: string;
}

type Unit = 'MHZ' | 'KHZ';

function toKHz(number: string, unit: Unit | ''): number | null {
    const [whole, fraction] = number.split('.');
    const decimal = fraction !== undefined;
    const assumed: Unit = decimal ? 'MHZ' : 'KHZ';
    const resolved: Unit = unit === '' ? assumed : unit;

    const wholeN = Number(whole);
    const fractionN = decimal ? Number(`${fraction}000`.slice(0, 3)) : 0;
    if (!Number.isInteger(wholeN) || !Number.isInteger(fractionN)) {
        return null;
    }

    if (resolved === 'MHZ') {
        return (wholeN * 1000) + fractionN;
    }
    return fractionN === 0 ? wholeN : null;
}

export function parseToken(token: string): FrequencyPayload | null {
    const m = TOKEN.exec(token);
    if (m === null) {
        return null;
    }
    const unit = m[2] ?? '';
    if (unit !== '' && unit !== 'MHZ' && unit !== 'KHZ') {
        return null;
    }
    const khz = toKHz(m[1], unit);
    if (khz === null || khz < MIN_KHZ || khz > MAX_KHZ) {
        return null;
    }
    return {token, khz};
}

export function fromParams(params: URLSearchParams): FrequencyPayload | null {
    const token = params.get('v');
    return token === null ? null : parseToken(token);
}

export function describe(payload: FrequencyPayload): FrequencyDetails {
    return {
        token: payload.token,
        mhz: mhzText(payload.khz),
        khz: String(payload.khz),
        band: bandOf(payload.khz),
        channel: channelOf(payload.khz),
        use: allocationOf(payload.khz),
    };
}

export const FREQUENCY_COLOR = '#6b4fbb';

const decorator: Decorator<FrequencyPayload> = {
    type: 'frequency',
    fromParams,

    summary: (payload) => `${PANEL_TITLE}: ${payload.token}`,

    style: {color: FREQUENCY_COLOR, background: 'rgba(107, 79, 187, 0.12)'},

    Panel: FrequencyPanel,

    Hover: FrequencyHover,
};

export default decorator;
