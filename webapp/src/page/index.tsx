import React from 'react';
import {createRoot} from 'react-dom/client';

import {applyBasename} from './basename';
import MapPageView from './MapPageView';
import {OverlayPageView} from './OverlayPageView';
import {readPageData} from './payload';
import type {PageData} from './payload';

import LinkButton from '../components/LinkButton';
import LocationReadings from '../decorators/location/LocationReadings';
import {seedPackages} from '../packages/store';
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

    // Before the first render, so the map's creation effect sees the list
    // rather than fetching a route this document has no session for.
    seedPackages(data.packages);

    createRoot(root).render(
        <React.StrictMode>{view(data)}</React.StrictMode>,
    );
}

function view(data: PageData): React.ReactNode {
    if (data.mode === 'overlay') {
        return <OverlayPageView data={data}/>;
    }

    if (data.mode === 'map') {
        return <MapPageView data={data}/>;
    }

    return (
        <LocationReadings
            payload={data.payload}
            conversion={data.conversion}
            hidden={[]}
            maps={data.maps}
            footer={<LinkButton href={docsUrl()}>{'Documentation'}</LinkButton>}
        />
    );
}
