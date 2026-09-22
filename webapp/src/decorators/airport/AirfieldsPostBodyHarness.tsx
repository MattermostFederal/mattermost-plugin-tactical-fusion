import React from 'react';

import AirfieldsPostBody from './AirfieldsPostBody';
import {AIRFIELDS_PROPS_KEY, AIRFIELDS_PROPS_VERSION} from './route';
import type {RouteAirfield} from './route';
import {HICKAM, HONOLULU, messageFor} from './route_fixtures';

import {_resetForTesting as resetFeatures} from '../../features/store';
import {_resetForTesting as resetPreferences} from '../../preferences/store';
import type {Selection} from '../selection';
import {_resetForTesting as resetSelection, subscribe} from '../selection';

interface Props {
    airfields?: RouteAirfield[] | null;
    message?: string;
    version?: number;
    editAt?: number;
    compactDisplay?: boolean;
}

const AirfieldsPostBodyHarness: React.FC<Props> = ({
    airfields = [HICKAM, HONOLULU],
    message,
    version = AIRFIELDS_PROPS_VERSION,
    editAt = 0,
    compactDisplay,
}) => {
    const [ready] = React.useState(() => {
        resetSelection();
        resetFeatures();
        resetPreferences();
        return true;
    });

    const [selection, setSelection] = React.useState<Selection | null>(null);
    React.useEffect(() => subscribe(setSelection), []);

    if (!ready) {
        return null;
    }

    const props = airfields === null ? undefined : {[AIRFIELDS_PROPS_KEY]: {version, airfields}};
    const text = message ?? messageFor(airfields ?? []);

    return (
        <div>
            <AirfieldsPostBody
                post={{id: 'post0000000000000000000000', message: text, props, edit_at: editAt}}
                compactDisplay={compactDisplay}
            />
            <p data-testid='selection'>{selection ? `${selection.type} ${JSON.stringify(selection.payload)}` : ''}</p>
        </div>
    );
};

export default AirfieldsPostBodyHarness;
