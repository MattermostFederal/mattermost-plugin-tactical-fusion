import React, {useState} from 'react';

import {useConversion} from './convert';
import LocationMap, {MAP_HEIGHT} from './map/LocationMap';
import {useNearViewport} from './map/near_viewport';
import {mapPageHref, viewFor} from './map/view';
import {INLINE_ID, isRowVisible} from './rows';

import {useFeatures} from '../../features/store';
import {usePreferences} from '../../preferences/store';

import type {LocationPayload} from './index';

export const INLINE_MAX_WIDTH_PX = 640;

const styles: Record<string, React.CSSProperties> = {
    frame: {maxWidth: INLINE_MAX_WIDTH_PX, marginTop: 8},
    reserved: {height: MAP_HEIGHT},
};

const LocationInlineMap: React.FC<{payload: LocationPayload; pageEnabled: boolean}> = ({payload, pageEnabled}) => {
    const {format, canonical, raw} = payload;
    const conversion = useConversion(format, canonical, raw);

    if (conversion.status === 'rejected') {
        return null;
    }

    return (
        <LocationMap
            {...viewFor(payload, conversion)}
            pageHref={pageEnabled ? mapPageHref(payload) : undefined}
            pending={conversion.status === 'loading'}
        />
    );
};

const LocationInline: React.FC<{payload: LocationPayload}> = ({payload}) => {
    const {preferences} = usePreferences();
    const {features} = useFeatures();
    const [box, setBox] = useState<HTMLDivElement | null>(null);
    const near = useNearViewport(box);

    // The admin's switch and the reader's own are checked in the same place and
    // mean the same thing here: draw nothing at all, not a reserved gap. The
    // post's own link is still on screen either way, which is why neither needs
    // to say anything in the channel.
    //
    // Read in the OUTER component, so the inner one never mounts and
    // useConversion is never reached. Mattermost renders on the order of thirty
    // posts at a time, so a gate below that would fetch a conversion for every
    // qualifying post in the window whether or not a map was ever drawn.
    if (!features.mapInline || !isRowVisible(preferences.location.hiddenRows, INLINE_ID)) {
        return null;
    }

    return (
        <div
            ref={setBox}
            style={styles.frame}
            data-testid='location-inline'
        >
            {near ? (
                <LocationInlineMap
                    payload={payload}
                    pageEnabled={features.mapPage}
                />
            ) : <div style={styles.reserved}/>}
        </div>
    );
};

export default LocationInline;
