import React, {useState} from 'react';

import {_resetForTesting as resetEditing} from './editing';
import {parseCanonical} from './format';
import LocationPanel from './LocationPanel';
import LocationTitle from './LocationTitle';

import type {LocationPayload} from './index';

/**
 * Harness for the sidebar header following the panel.
 *
 * Mattermost renders the header and the body as two separate components, which
 * is the whole reason the editor state lives in a module store. Mounting both
 * together is the only way to test that they actually agree.
 */
const TitleHarness: React.FC = () => {
    useState(() => {
        resetEditing();
        return null;
    });

    const canonical = '34.0561,-118.2500';
    const payload: LocationPayload = {
        coord: parseCanonical('dd', canonical),
        format: 'dd',
        canonical,
        raw: canonical,
    };

    return (
        <div>
            <h1 data-testid='rhs-title'><LocationTitle/></h1>
            <LocationPanel payload={payload}/>
        </div>
    );
};

export default TitleHarness;
