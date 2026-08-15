import React, {useLayoutEffect} from 'react';

import {useConversion} from './convert';
import Customize from './Customize';
import {setEditing, useEditing} from './editing';
import LocationReadings from './LocationReadings';

import LinkButton from '../../components/LinkButton';
import {docsUrl} from '../../plugin_url';
import {usePreferences} from '../../preferences/store';

import type {LocationPayload} from './index';

/**
 * The location panel: the sidebar's environment around the shared readings.
 *
 * What lives here is everything the standalone pages cannot have. Both the
 * conversion and the reader's hidden rows come from the authenticated API, and
 * the editor writes to it, so a public page is handed its data instead and
 * shows no Customize link. The table itself is LocationReadings, once.
 */
const LocationPanel: React.FC<{payload: LocationPayload}> = ({payload}) => {
    const {format, canonical, raw} = payload;

    const conversion = useConversion(format, canonical, raw);
    const {preferences} = usePreferences();
    const customizing = useEditing();

    // Clicking a different coordinate while the editor is open would otherwise
    // land on the editor rather than on the coordinate that was clicked. React
    // keeps this component mounted across a change of selection, so nothing
    // else resets it.
    //
    // Before paint, not after, so the editor is never flashed on screen
    // carrying a coordinate the reader did not open it from.
    useLayoutEffect(() => {
        setEditing(false);
    }, [format, canonical]);

    // The editor takes the panel over rather than sitting below the table,
    // matching the DTG panel: a list of settings under the coordinate would
    // bury the thing the reader opened the sidebar for.
    //
    // Ahead of the rejected check on purpose. A reader who reached the editor
    // and then clicked a hand-edited link would otherwise be dropped on "Not a
    // coordinate" with no way back to their settings.
    if (customizing) {
        return <Customize onClose={() => setEditing(false)}/>;
    }

    return (
        <LocationReadings
            payload={payload}
            conversion={conversion}
            hidden={preferences.location.hiddenRows}
            footer={
                <>
                    <LinkButton onClick={() => setEditing(true)}>{'Customize your view'}</LinkButton>
                    {' · '}
                    <LinkButton href={docsUrl()}>{'Documentation'}</LinkButton>
                </>
            }
        />
    );
};

export default LocationPanel;
