import React from 'react';

import {useConversion} from './convert';
import LocationMap from './map/LocationMap';
import {viewFor} from './map/view';

import {useFeatures} from '../../features/store';

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

/**
 * The card itself, mounted only once the panel map is known to be on.
 *
 * Split from the gate below rather than guarded inside it, because `useConversion`
 * cannot be called conditionally and a hook that runs is a request that goes out.
 * With the split, a maps-off install issues nothing at all as a pointer crosses a
 * channel full of coordinates. That is the same outer/inner shape LocationInline
 * uses, and for the same reason.
 */
const LocationHoverCard: React.FC<{payload: LocationPayload}> = ({payload}) => {
    const {format, canonical, raw} = payload;
    const conversion = useConversion(format, canonical, raw);

    // A link the server refused says so, rather than drawing a pin for it. That
    // is the verdict LocationReadings already honours by refusing the table; the
    // card used to branch on `ready` alone, so a hand-edited link whose author
    // text failed validation showed a confident pin here and "Not a coordinate"
    // on the panel one click later.
    //
    // A line rather than `null`, and now that the framework hides an empty card
    // it is a choice rather than a workaround: a refused link is something the
    // reader has to be told, where a switched-off map is something they have
    // not asked about.
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

const LocationHover: React.FC<{payload: LocationPayload}> = ({payload}) => {
    const {features} = useFeatures();

    // With the panel map switched off there is no card, because this card IS
    // the map: there is no reduced version of it worth floating beside a link.
    //
    // `null` is safe here now and was not before. The framework's tooltip hides
    // its own chrome when a Hover renders nothing, so this is no card rather
    // than an empty bordered box; see Tooltip.tsx.
    if (!features.mapPanel) {
        return null;
    }

    return <LocationHoverCard payload={payload}/>;
};

export default LocationHover;
