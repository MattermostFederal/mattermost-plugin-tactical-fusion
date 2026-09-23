import React from 'react';

import type {AirportMapPayload} from './map';
import {positionPayload, runwayLabel, runwayShapes} from './map';

import type {Camera} from '../location/map/camera';
import LocationMap from '../location/map/LocationMap';
import {viewFor} from '../location/map/view';

export const AirportMapCanvas: React.FC<{
    payload: AirportMapPayload;
    fill?: boolean;
    openAt?: Camera;
}> = ({payload, fill, openAt}) => {
    const position = positionPayload(payload.coordinate);
    if (position === null) {
        return null;
    }

    const shapes = runwayShapes(payload.runways);

    return (
        <LocationMap
            {...viewFor(position, {status: 'loading', data: null})}
            region={payload.coordinate.region}
            pending={false}
            geometries={shapes}
            markerLabel={runwayLabel(shapes.length)}
            fill={fill}
            openAt={openAt}
        />
    );
};
