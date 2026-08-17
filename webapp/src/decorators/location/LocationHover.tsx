import React from 'react';

import {useConversion} from './convert';
import LocationMap from './map/LocationMap';
import {viewFor} from './map/view';

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
const styles: Record<string, React.CSSProperties> = {
    refused: {fontSize: 12, color: 'var(--center-channel-color)'},
};

const LocationHover: React.FC<{payload: LocationPayload}> = ({payload}) => {
    const {format, canonical, raw} = payload;
    const conversion = useConversion(format, canonical, raw);

    // A link the server refused says so, rather than drawing a pin for it. That
    // is the verdict LocationReadings already honours by refusing the table; the
    // card used to branch on `ready` alone, so a hand-edited link whose author
    // text failed validation showed a confident pin here and "Not a coordinate"
    // on the panel one click later.
    //
    // A line rather than `null`, because the framework's tooltip builds its
    // chrome around whatever a Hover returns: `null` there is not "no card", it
    // is an empty bordered box floating beside the link. Inside Tooltip.tsx null
    // means no card at all, since it returns before the chrome; that distinction
    // is invisible from in here, and an empty box agrees with nothing.
    if (conversion.status === 'rejected') {
        return <span style={styles.refused}>{'Not a coordinate'}</span>;
    }

    // Derived through the same viewFor the panel and the map page use, so a
    // hover and the panel behind it cannot come to disagree about where a
    // coordinate is.
    const view = viewFor(payload, conversion);

    return (
        <LocationMap
            {...view}
            pending={conversion.status === 'loading'}
            preview={true}
        />
    );
};

export default LocationHover;
