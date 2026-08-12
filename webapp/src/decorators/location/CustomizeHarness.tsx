import React, {useState} from 'react';

import Customize from './Customize';
import {_resetForTesting as resetEditing} from './editing';

import {_resetForTesting as resetPreferencesStore} from '../../preferences/store';

/**
 * Harness for the location editor.
 *
 * The module-level preferences store is reset on mount, because it caches the
 * blob for the life of the page and every test needs its own.
 */
const CustomizeHarness: React.FC = () => {
    useState(() => {
        resetEditing();
        resetPreferencesStore();
        return null;
    });

    const [closed, setClosed] = useState(false);

    return (
        <div>
            <Customize onClose={() => setClosed(true)}/>
            {closed && <p data-testid='closed'>{'closed'}</p>}
        </div>
    );
};

export default CustomizeHarness;
