import React from 'react';
import {createRoot} from 'react-dom/client';

import {applyBasename} from './basename';
import MapPageView from './MapPageView';
import {readPageData} from './payload';

import LinkButton from '../components/LinkButton';
import LocationReadings from '../decorators/location/LocationReadings';
import {docsUrl} from '../plugin_url';

/**
 * The standalone pages, built from the same source as the sidebar panel.
 *
 * These are what a client with no Mattermost webapp opens, which in practice
 * means the mobile app. The server validates the link, works out everything the
 * projection is needed for, and puts both in the shell; this renders the same
 * components the panel renders, so the two surfaces cannot come to disagree
 * about what a coordinate says.
 *
 * No reader preferences and no Customize link: both need a session, and this
 * route is public. The page therefore always shows every row, which is what it
 * showed when the server rendered it.
 */
applyBasename();

start();

function start(): void {
    const root = document.getElementById('root');
    if (!root) {
        return;
    }

    const data = readPageData(root);
    if (!data) {
        return;
    }

    createRoot(root).render(
        <React.StrictMode>
            {data.mode === 'map' ? (
                <MapPageView data={data}/>
            ) : (
                <LocationReadings
                    payload={data.payload}
                    conversion={data.conversion}
                    hidden={[]}
                    footer={<LinkButton href={docsUrl()}>{'Documentation'}</LinkButton>}
                />
            )}
        </React.StrictMode>,
    );
}
