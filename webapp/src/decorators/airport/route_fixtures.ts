import manifest from 'manifest';

import type {RouteAirfield} from './route';

const PREFIX = `/plugins/${manifest.id}/decorate`;

export const HICKAM: RouteAirfield = {ident: 'PHIK', code: 'PHIK', name: 'Hickam Air Force Base', format: 'dd', value: '21.3353,-157.9483'};

export const HONOLULU: RouteAirfield = {ident: 'PHNL', code: 'HNL', name: 'Daniel K. Inouye International Airport', format: 'dd', value: '21.3184,-157.9257'};

function linkFor(airfield: RouteAirfield): string {
    const param = airfield.code === airfield.ident ? 'v' : 'i';
    return `[${airfield.code}](${PREFIX}/airport?${param}=${airfield.code})`;
}

export function messageFor(airfields: RouteAirfield[], trail = '//'): string {
    return airfields.map(linkFor).join(' ') + trail;
}
