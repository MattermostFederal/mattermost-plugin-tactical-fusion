import React, {useState} from 'react';

import {useConversion} from './convert';
import LocationMap, {MAP_HEIGHT} from './map/LocationMap';
import {useNearViewport} from './map/near_viewport';
import {mapPageHref, viewFor} from './map/view';
import {INLINE_ID, isRowVisible} from './rows';

import {usePreferences} from '../../preferences/store';

import type {LocationPayload} from './index';

export const INLINE_MAX_WIDTH_PX = 640;

const styles: Record<string, React.CSSProperties> = {
    frame: {maxWidth: INLINE_MAX_WIDTH_PX, marginTop: 8},
    reserved: {height: MAP_HEIGHT},
};

const LocationInlineMap: React.FC<{payload: LocationPayload}> = ({payload}) => {
    const {format, canonical, raw} = payload;
    const conversion = useConversion(format, canonical, raw);

    if (conversion.status === 'rejected') {
        return null;
    }

    return (
        <LocationMap
            {...viewFor(payload, conversion)}
            pageHref={mapPageHref(payload)}
            pending={conversion.status === 'loading'}
        />
    );
};

const LocationInline: React.FC<{payload: LocationPayload}> = ({payload}) => {
    const {preferences} = usePreferences();
    const [box, setBox] = useState<HTMLDivElement | null>(null);
    const near = useNearViewport(box);

    if (!isRowVisible(preferences.location.hiddenRows, INLINE_ID)) {
        return null;
    }

    return (
        <div
            ref={setBox}
            style={styles.frame}
            data-testid='location-inline'
        >
            {near ? <LocationInlineMap payload={payload}/> : <div style={styles.reserved}/>}
        </div>
    );
};

export default LocationInline;
