import {IATA, IDENT} from './airport';
import AirportHover from './AirportHover';
import AirportPanel from './AirportPanel';
import {AIRFIELD_COLOR} from './map';

import type {Decorator} from '../types';

export const PANEL_TITLE = 'Airfield';

export type AirportKey = 'icao' | 'iata';

export interface AirportPayload {
    key: AirportKey;
    code: string;
}

export function fromParams(params: URLSearchParams): AirportPayload | null {
    const ident = params.get('v');
    const iata = params.get('i');

    if ((ident === null) === (iata === null)) {
        return null;
    }

    if (ident !== null) {
        return IDENT.test(ident) ? {key: 'icao', code: ident} : null;
    }

    return iata !== null && IATA.test(iata) ? {key: 'iata', code: iata} : null;
}

const decorator: Decorator<AirportPayload> = {
    type: 'airport',
    fromParams,

    summary: (payload) => `${PANEL_TITLE}: ${payload.code}`,

    style: {color: AIRFIELD_COLOR, background: 'rgba(184, 119, 15, 0.12)'},

    Panel: AirportPanel,

    Hover: AirportHover,
};

export default decorator;
