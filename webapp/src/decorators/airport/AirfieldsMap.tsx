import React, {useState} from 'react';

import type {RoutePayload} from './route';
import {drawsNothing, routeLabel, routeLegs, routeMarkers} from './route';

import {useFeatures} from '../../features/store';
import {usePreferences} from '../../preferences/store';
import type {Camera} from '../location/map/camera';
import LocationMap, {INLINE_MAP_HEIGHT} from '../location/map/LocationMap';
import {useNearViewport} from '../location/map/near_viewport';
import {overlayPageHref} from '../location/map/view';
import {INLINE_ID, isRowVisible} from '../location/rows';

export const INLINE_MAX_WIDTH_PX = 640;

const styles: Record<string, React.CSSProperties> = {
    frame: {maxWidth: INLINE_MAX_WIDTH_PX, marginTop: 8},
    reserved: {height: INLINE_MAP_HEIGHT},
};

export const AirfieldsMapCanvas: React.FC<{
    payload: RoutePayload;
    pageEnabled?: boolean;
    fill?: boolean;
    inline?: boolean;
    openAt?: Camera;
}> = ({payload, pageEnabled, fill, inline, openAt}) => {
    if (drawsNothing(payload)) {
        return null;
    }

    return (
        <LocationMap
            lat={null}
            lon={null}
            cellDegLat={0}
            cellDegLon={0}
            region=''
            pending={false}
            extentLabel={routeLabel(payload)}
            markers={routeMarkers(payload)}
            geometries={routeLegs(payload)}
            pageHref={pageEnabled && payload.postId ? overlayPageHref(payload.postId) : undefined}
            fill={fill}
            inline={inline}
            openAt={openAt}
        />
    );
};

const AirfieldsMap: React.FC<{payload: RoutePayload}> = ({payload}) => {
    const {preferences} = usePreferences();
    const {features} = useFeatures();
    const [box, setBox] = useState<HTMLDivElement | null>(null);
    const near = useNearViewport(box);

    if (!features.mapInline || !isRowVisible(preferences.location.hiddenRows, INLINE_ID) || drawsNothing(payload)) {
        return null;
    }

    return (
        <div
            ref={setBox}
            style={styles.frame}
            data-testid='airfields-map'
        >
            {near ? (
                <AirfieldsMapCanvas
                    payload={payload}
                    pageEnabled={features.mapPage}
                    inline={true}
                />
            ) : <div style={styles.reserved}/>}
        </div>
    );
};

export default AirfieldsMap;
