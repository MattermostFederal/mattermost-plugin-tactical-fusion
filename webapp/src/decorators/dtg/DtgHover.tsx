import React from 'react';

import Countdown from './Countdown';
import {resolvedUrgentWithinMs} from './preferences';

import {usePreferences} from '../../preferences/store';

import type {Dtg} from './index';

/**
 * The hover card for a DTG: the countdown, and nothing else.
 *
 * A hover answers "how far away is this" at a glance. The plain-language
 * reading, the token and the timezone table are all a click away in the panel,
 * and repeating them here only makes the glance slower.
 *
 * It honors the reader's flash threshold so that pointing at a link and
 * opening it cannot disagree about whether the same DTG is imminent. The
 * preferences are cached, so hovering does not mean a request.
 */
const DtgHover: React.FC<{payload: Dtg}> = ({payload}) => {
    const {preferences} = usePreferences();

    return (
        <Countdown
            target={payload.instant}
            compact={true}
            urgentWithinMs={resolvedUrgentWithinMs(preferences.dtg)}
        />
    );
};

export default DtgHover;
