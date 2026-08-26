import React, {useState} from 'react';

import DtgPanel from './DtgPanel';
import DtgTitle from './DtgTitle';
import {_resetForTesting as resetEditing} from './editing';

import type {Dtg} from './index';

/**
 * Harness for the sidebar header following the panel.
 *
 * Mattermost renders the header and the body as two separate components, which
 * is the whole reason the editor state lives in a module store. Mounting both
 * together is the only way to test that they actually agree.
 *
 * The instant arrives as milliseconds because Playwright drives this from Node
 * over serializable props.
 */
const TitleHarness: React.FC<{instantMs: number}> = ({instantMs}) => {
    useState(() => {
        resetEditing();
        return null;
    });

    const payload: Dtg = {
        instant: new Date(instantMs),
        canonical: '091630ZAUG26',
        offsetMinutes: 0,
        zoneLabel: 'Z',
        assumedMonth: false,
        assumedYear: false,
    };

    return (
        <div>
            <h1 data-testid='rhs-title'><DtgTitle/></h1>
            <DtgPanel payload={payload}/>
        </div>
    );
};

export default TitleHarness;
