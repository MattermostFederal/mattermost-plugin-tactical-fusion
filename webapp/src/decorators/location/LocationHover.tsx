import React from 'react';

import {useConversion} from './convert';
import {cellDegrees} from './format';
import LocationMap from './map/LocationMap';

import type {LocationPayload} from './index';

/**
 * The hover card for a coordinate: the map, and nothing else.
 *
 * A hover answers "where is this" at a glance, which for a coordinate is a
 * picture rather than a number. Every reading, every alternative notation and
 * every copy button is a click away in the panel, and repeating any of them here
 * only makes the glance slower. That is the same bar the DTG hover is held to,
 * where the answer is the countdown and nothing else.
 *
 * It is the panel's own map component in `preview` mode, not a second one, for
 * the reason every other surface shares it: two implementations of a projection
 * and a palette are two things that can disagree, and this one would disagree in
 * the place a reader looks first.
 *
 * Two costs worth knowing, because a hover is not a click:
 *
 * Pointing at a coordinate now pulls the MapLibre chunk, about 950 KB, where it
 * used to take a click. It is one cacheable response for the whole session, and
 * the panel would have pulled it the moment the reader opened anything.
 *
 * A grid reference has no position until the server projects it, so hovering one
 * costs a request. `useConversion` is cached by token, so it costs at most one
 * per distinct coordinate however many times a cursor crosses it, and the click
 * that follows a hover joins the same request rather than issuing a second.
 * That cache is what this component was waiting for.
 */
const LocationHover: React.FC<{payload: LocationPayload}> = ({payload}) => {
    const {coord, format, canonical, raw} = payload;
    const conversion = useConversion(format, canonical, raw);

    // Derived exactly as LocationReadings derives it, down to reading `data`
    // only once the status is `ready`. A hover and the panel behind it
    // disagreeing about where a coordinate is would be the worst possible place
    // for the two to drift, and the cell is sized through the same `cellDegrees`
    // so a grid square is not squared off differently in the card than on the
    // panel.
    const position = conversion.status === 'ready' ? conversion.data : null;
    const lat = coord ? coord.lat.decimal : (position?.lat ?? null);
    const lon = coord ? coord.lon.decimal : (position?.lon ?? null);
    const [cellLat, cellLon] = cellDegrees(coord, format, canonical, lat);

    return (
        <LocationMap
            lat={lat}
            lon={lon}
            cellDegLat={cellLat}
            cellDegLon={cellLon}
            region={position?.region ?? ''}
            pending={conversion.status === 'loading'}
            preview={true}
        />
    );
};

export default LocationHover;
