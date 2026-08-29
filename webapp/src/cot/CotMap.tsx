import React, {useState} from 'react';

import {isSectionVisible} from './sections';
import type {CotEvent} from './types';
import {accuracyMeters, affiliationColor, affiliationWord, isLinkable, statedColor} from './types';

import type {MapGeometry} from '../decorators/location/map/LocationMap';
import LocationMap, {MAP_HEIGHT} from '../decorators/location/map/LocationMap';
import {useNearViewport} from '../decorators/location/map/near_viewport';
import {isRenderable} from '../decorators/location/map/span';
import {overlayPageHref} from '../decorators/location/map/view';
import {INLINE_ID, isRowVisible} from '../decorators/location/rows';
import {withTheme} from '../decorators/theme';
import {useFeatures} from '../features/store';
import {pluginBaseUrl} from '../plugin_url';
import {usePreferences} from '../preferences/store';

export const COT_MAP_MAX_WIDTH_PX = 640;

const styles: Record<string, React.CSSProperties> = {
    frame: {maxWidth: COT_MAP_MAX_WIDTH_PX, padding: '0 12px 8px'},
    reserved: {height: MAP_HEIGHT},
};

function mapPageHref(event: CotEvent): string {
    const params = new URLSearchParams({f: event.format, v: event.value});
    return withTheme(`${pluginBaseUrl()}/map?${params.toString()}`);
}

/**
 * Where "Open larger" goes, which depends on what is being drawn.
 *
 * ONE drawable event addresses the map page by its own coordinate, as it always
 * has: that page carries the token and a way through to every reading of it,
 * which a post id cannot offer.
 *
 * A BLOCK has no coordinate at all. Linking the first event's page would frame
 * one position and say nothing about the rest, which is the same argument that
 * keeps the accuracy ring off a block map, so a block addresses the page by the
 * post instead and the page redraws the whole block. Before that existed a
 * block simply had no link, which read as the control being broken.
 *
 * No post id means no link. A card built from a payload directly, as a harness
 * does, has nothing to address.
 */
function largerHref(pageEnabled: boolean, only: CotEvent | undefined, postId: string | undefined): string | undefined {
    if (!pageEnabled) {
        return undefined;
    }
    if (only) {
        return mapPageHref(only);
    }

    return postId ? overlayPageHref(postId) : undefined;
}

/** @internal exported for tests */
export function _drawableEventsForTesting( // eslint-disable-line no-underscore-dangle, @typescript-eslint/naming-convention
    events: readonly CotEvent[],
): CotEvent[] {
    return drawableEvents(events);
}

/**
 * Every event this build can put a marker on.
 *
 * `isLinkable` rather than a finiteness test on the numbers. The server writes
 * `format` and `value` only when it parsed a position it is prepared to stand
 * behind, so that pair IS the question being asked. A finiteness test looks
 * equivalent and is not: an event the server gave no position has `lat` of
 * `''`, `Number('')` is `0`, and `0` is finite, so the test admitted a
 * positionless event and pinned it in the Gulf of Guinea. That is the guessed
 * position LocationMap says it never draws.
 *
 * `isRenderable` on top, because the server accepts any latitude to 90 and Web
 * Mercator stops at about 85.05. An event past that cannot be placed, and the
 * first marker decides where the whole map opens.
 */
function drawableEvents(events: readonly CotEvent[]): CotEvent[] {
    return events.filter((event) => isLinkable(event) && isRenderable(Number(event.lat)));
}

/** Those events as markers, with the color to draw each one in. */
function markersFor(events: readonly CotEvent[]) {
    return events.map((event) => ({
        lat: Number(event.lat),
        lon: Number(event.lon),
        color: affiliationColor(event) ?? UNCOLORED,
    }));
}

/**
 * What a block of markers is, for a reader who gets no color.
 *
 * Color is the whole of what tells one marker from another on this map, so a
 * label that says only how many there are leaves a screen reader with a count
 * and nothing else. The affiliations are the second channel, and this is the
 * one surface that has to carry them: a single-event map already states its
 * type, and there the color distinguishes nothing anyway.
 *
 * Counted in EVENTS, not markers. An event with no usable position draws
 * nothing, and the card's own heading counts every event, so a map announcing
 * its marker count contradicted the heading two lines above it.
 */
/** @internal exported for tests */
export function _blockLabelForTesting( // eslint-disable-line no-underscore-dangle, @typescript-eslint/naming-convention
    drawn: readonly CotEvent[], total: number,
): string {
    return blockLabel(drawn, total);
}

function blockLabel(drawn: readonly CotEvent[], total: number): string {
    // The ratio only when it is news. The opening clause already says how many
    // positions are marked, so repeating that count here says nothing; saying
    // "4 of 5" does, because it is the only place an event that could not be
    // drawn is accounted for at all.
    const counted = drawn.length === total ? '' : `${drawn.length} of ${total} events: `;
    const tally = new Map<string, number>();

    for (const event of drawn) {
        const word = affiliationWord(event);
        tally.set(word, (tally.get(word) ?? 0) + 1);
    }

    const parts = [...tally.entries()].map(([word, n]) => `${n} ${word}`);

    return counted + joinWords(parts);
}

function joinWords(parts: readonly string[]): string {
    if (parts.length < 2) {
        return parts.join('');
    }

    return `${parts.slice(0, -1).join(', ')} and ${parts[parts.length - 1]}`;
}

/**
 * What an event whose affiliation this build does not color is drawn in.
 *
 * A marker still has to be drawn: leaving it off the map would be the map
 * quietly disagreeing with the list beside it about how many events there are.
 */
const UNCOLORED = '#8a8f98';

/**
 * The shape one event describes, ready for the map.
 *
 * Only for a single event. A block of shapes on one map is a pile of outlines
 * with nothing saying which belongs to which track, which is the argument the
 * accuracy ring is already drawn under.
 */
function geometryFor(event: CotEvent | undefined): MapGeometry | undefined {
    if (!event?.geometry) {
        return undefined;
    }

    const {geometry} = event;

    // The server's verdict wins. It refuses a shape it will not stand behind,
    // and drawing one anyway made the card say "not drawn" over a drawn shape.
    if (geometry.note !== '') {
        return undefined;
    }

    if (geometry.kind === 'ellipse') {
        const {majorMeters, minorMeters, angleDegrees} = geometry;
        if (!Number.isFinite(majorMeters) || !Number.isFinite(minorMeters) ||
            majorMeters <= 0 || minorMeters <= 0) {
            return undefined;
        }

        return {
            kind: 'ellipse',
            major: majorMeters,
            minor: minorMeters,
            angle: Number.isFinite(angleDegrees) ? angleDegrees : 0,
        };
    }

    if (geometry.points.length < 2) {
        return undefined;
    }

    return {kind: 'outline', points: geometry.points, closed: geometry.closed};
}

/**
 * The one event in a block that draws an outline, or undefined.
 *
 * A block of shapes on one map is a pile of outlines with nothing saying which
 * belongs to which track, which is why more than one draws none. Exactly one is
 * unambiguous, and a suspected area beside the tracks inside it is the case this
 * exists for.
 *
 * Outlines only, and that is not a simplification. An outline carries absolute
 * vertices, so it lands where the event put it whatever else is on the map. An
 * ellipse is drawn around the map's PRIMARY position, which in a block is the
 * first event's, so a circle belonging to the third would be drawn around the
 * first one's marker.
 */
function soleOutline(drawn: readonly CotEvent[]): CotEvent | undefined {
    const outlined = drawn.filter((event) => geometryFor(event)?.kind === 'outline');
    return outlined.length === 1 ? outlined[0] : undefined;
}

/** @internal exported for tests */
export function _soleOutlineForTesting( // eslint-disable-line no-underscore-dangle, @typescript-eslint/naming-convention
    drawn: readonly CotEvent[],
): CotEvent | undefined {
    return soleOutline(drawn);
}

/** @internal exported for tests */
export function _geometryForTesting( // eslint-disable-line no-underscore-dangle, @typescript-eslint/naming-convention
    event: CotEvent | undefined,
): MapGeometry | undefined {
    return geometryFor(event);
}

/**
 * The drawing, with nothing consulted.
 *
 * Exported because the map page draws exactly this and must not consult the
 * reader's sections or the admin's inline switch to do it: the page IS the map,
 * so reaching it is the decision. The wrapper below is where those two live.
 */
export const CotMapCanvas: React.FC<{
    events: readonly CotEvent[];
    pageEnabled: boolean;
    postId?: string;
    fill?: boolean;
}> = ({
    events, pageEnabled, postId, fill,
}) => {
    const drawn = drawableEvents(events);
    const markers = markersFor(drawn);
    if (markers.length === 0) {
        return null;
    }

    // The accuracy circle is the one event's own, so it is drawn only when
    // there is one event. A ring per track in a block reads as a map of
    // overlapping blobs rather than as a set of positions.
    //
    // Both counts, not just one. A post carrying three events of which one can
    // be drawn is not a single-event card: treating it as one would put that
    // event's accuracy ring and its Open larger link on a map the other two
    // are missing from, and say nothing about the two.
    const only = events.length === 1 && drawn.length === 1 ? drawn[0] : undefined;
    const shaped = only ?? soleOutline(drawn);

    return (
        <LocationMap
            lat={markers[0].lat}
            lon={markers[0].lon}
            cellDegLat={0}
            cellDegLon={0}
            region=''
            pending={false}
            accuracyMeters={only && accuracyMeters(only)}
            accuracyLabel={only?.ce}
            markers={markers}
            geometry={geometryFor(shaped)}
            geometryColor={shaped && statedColor(shaped)}
            markerLabel={only ? only.typeLabel : blockLabel(drawn, events.length)}
            pageHref={largerHref(pageEnabled, only, postId)}
            fill={fill}
        />
    );
};

/**
 * The map of a Cursor on Target event, in the channel or in the sidebar.
 *
 * The ADMIN switch is the location decorator's on both surfaces, not a second
 * pair: this is the same map over the same basemap, and drawing it would pull
 * the same archive on exactly the installs that switch exists for.
 *
 * The READER switch differs by surface, which is what `surface` decides. In the
 * channel this is the map under a post and follows the same INLINE_ID a
 * coordinate-only post does. In the sidebar it is not a map under a post at
 * all, so it follows the Cursor on Target panel's own `map` section instead;
 * hiding the coordinate map under posts used to blank this one too, which read
 * as a bug rather than as a setting.
 *
 * Both are read in the OUTER component so the inner one never mounts, and the
 * viewport gate is here for the reason it is on the coordinate map: browsers cap
 * live WebGL contexts at roughly sixteen and a channel of position reports is
 * exactly the shape that reaches it.
 */
const CotMap: React.FC<{
    events: readonly CotEvent[];
    surface: 'card' | 'panel';
    postId?: string;
}> = ({events, surface, postId}) => {
    const {preferences} = usePreferences();
    const {features} = useFeatures();
    const [box, setBox] = useState<HTMLDivElement | null>(null);
    const near = useNearViewport(box);

    const wanted = surface === 'card' ?
        isRowVisible(preferences.location.hiddenRows, INLINE_ID) :
        isSectionVisible(preferences.cot.hiddenSections, 'map');

    // Both surfaces, one switch, deliberately: docs/design/cot.md "Switches"
    // argues it and TestCotHasNoMapSettingOfItsOwn is where it gets revisited.
    // GeoJSON reads mapPanel for its panel instead, which is a different
    // decision rather than a drift; see that file and geojson.md.
    if (!features.mapInline || !wanted) {
        return null;
    }

    return (
        <div
            ref={setBox}
            style={styles.frame}
            data-testid='cot-map'
        >
            {near ? (
                <CotMapCanvas
                    events={events}
                    pageEnabled={features.mapPage}
                    postId={postId}
                />
            ) : <div style={styles.reserved}/>}
        </div>
    );
};

export default CotMap;
