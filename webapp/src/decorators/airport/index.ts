import {IDENT} from './airport';
import AirportHover from './AirportHover';
import AirportPanel from './AirportPanel';

import type {Decorator} from '../types';

export const PANEL_TITLE = 'Airfield';

/**
 * An airfield code, already matched, validated and looked up by the server.
 *
 * The identity of an airfield is its code, and everything else is derived from
 * it. Nothing derived travels in the link, so a link cannot disagree with
 * itself, and there is nothing here for the panel to compute: the database
 * lives in Go.
 */
export interface AirportPayload {

    /** The ICAO ident, four upper-case letters. */
    ident: string;
}

/**
 * Builds the payload from the link's query params.
 *
 * Params come from a URL a user could have hand-edited, so the shape is
 * re-checked. Whether the code names an airfield this build holds is the
 * server's answer, not this one's: the database is Go-only, and a code that is
 * well formed but absent is a thing the panel renders rather than a link it
 * refuses.
 */
export function fromParams(params: URLSearchParams): AirportPayload | null {
    const ident = params.get('v') ?? '';
    if (!IDENT.test(ident)) {
        return null;
    }

    return {ident};
}

const decorator: Decorator<AirportPayload> = {
    type: 'airport',
    fromParams,

    summary: () => PANEL_TITLE,

    // Amber, distinct from the DTG and the coordinate colors: an airfield is a
    // place you go rather than a place on the ground or a time, and a reader
    // scanning a USMTF line should be able to tell the three apart at a glance.
    style: {color: '#b8770f', background: 'rgba(184, 119, 15, 0.12)'},

    Panel: AirportPanel,

    // A four-letter code says nothing to a reader, so naming the airfield is
    // the one thing worth knowing without opening the sidebar. Affordable for
    // the same reason the location hover is: the module cache in airport.ts
    // means a cursor crossing a line of codes costs one request per code
    // whatever it does.
    Hover: AirportHover,
};

export default decorator;
