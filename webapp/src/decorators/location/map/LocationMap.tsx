import type {FeatureCollection} from 'geojson';
import type {GeoJSONSource, Map as MapLibreMap} from 'maplibre-gl';
import React, {useCallback, useEffect, useRef, useState} from 'react';

import {loadBasemap} from './basemap';
import {
    buildStyle, cellFeature, emptyCollection, hasWebGL2, loadMapLibre, mapColors, pointFeature,
} from './maplibre';
import {MAX_ZOOM, cellBounds, isRenderable, zoomForSpan} from './span';
import type {View} from './view';

/**
 * The map above the readings table.
 *
 * Nothing here may fail the panel. Every failure hides the map and leaves every
 * reading on screen, and no pin is ever drawn at a guessed position: a marker
 * at 0, 0 because a conversion failed is a position, and a wrong one.
 *
 * The map is created ONCE and moved thereafter. Rebuilding it per coordinate
 * meant a fresh WebGL context and a re-tessellation of the whole basemap on
 * every click, against the browser's cap of roughly sixteen live contexts, and
 * the panel stays mounted across a change of selection so those clicks arrive
 * in a run.
 */

/**
 * How tall the map is.
 *
 * A range rather than a number, because the sidebar's height is the reader's
 * screen and not ours: a 1366x768 VDI, which this plugin's audience runs, has
 * roughly 557px of sidebar to spend, where a 1080p desktop has about 847px.
 * Sizing for the small one wastes the large one and sizing for the large one
 * pushes the readings off the bottom of the small one.
 *
 * CSS does this without measuring anything, and MapLibre's ResizeObserver
 * already picks up the change. Height is free context: the map holds its ground
 * span across its WIDTH, so a taller box shows more latitude at the same scale
 * rather than zooming.
 */
const MAP_MIN_HEIGHT_PX = 200;
const MAP_MAX_HEIGHT_PX = 360;
export const MAP_HEIGHT = `clamp(${MAP_MIN_HEIGHT_PX}px, 30vh, ${MAP_MAX_HEIGHT_PX}px)`;

/*
 * The hover card's map.
 *
 * Sized HERE rather than left to fill the card, because the frame has no
 * intrinsic width: inside a tooltip that sizes itself to its content, a child
 * with only a height is at the mercy of whatever the overlay happens to give it,
 * and what it gave it was a strip. An explicit pair also keeps the aspect fixed
 * at 16:9, so the opening view frames the same ground on every card rather than
 * whatever the chrome left over.
 *
 * The framework's card caps at 360px, which is this plus its padding.
 */
const PREVIEW_WIDTH_PX = 320;
const PREVIEW_HEIGHT_PX = 180;

const LOADING = 'Loading map…';
const NO_WEBGL = 'This browser cannot draw the map.';
const NO_BASEMAP = 'The map could not be loaded.';
const NO_POSITION = 'The position for this coordinate is unavailable.';

// One label across all three maps. The pages call it the same thing, and a
// reader who moves between them should not have to learn a second word for the
// same button. It resets the zoom as well as the centre, which is why it is not
// "Recenter".
const RESET_LABEL = 'Reset view';
const TOO_FAR_NORTH = 'This position is too far north for the map.';
const TOO_FAR_SOUTH = 'This position is too far south for the map.';

const DEFAULT_WIDTH_PX = 320;

/**
 * Where a test can get at the map.
 *
 * The pin and the cell are MapLibre sources drawn through WebGL, so nothing
 * about them reaches the DOM: a test watching the note and the Reset button
 * sees exactly the same thing whether or not `applyView` wrote to either
 * source. Both overlay writes, the camera move and `remove()` on unmount were
 * executed by the suite and asserted by none of it, which is how the stale pin
 * this component is arranged around could be reintroduced with every test
 * green.
 *
 * A module-level observer rather than a prop: the panel and both pages
 * construct this component, and none of them should carry a field that exists
 * for the tests.
 */
let mapObserver: ((map: MapLibreMap | null) => void) | null = null;

interface Props extends View {
    region: string;
    pending: boolean;

    /**
     * Where "Open larger" goes, or absent on the page that IS the larger view.
     */
    pageHref?: string;

    /** Fills its parent rather than sitting in the flow of a panel. */
    fill?: boolean;

    /**
     * A picture and nothing else: no controls, no gestures, no readout.
     *
     * For the hover card, where the reader is pointing rather than working. A
     * card is dismissed by moving the pointer, so a map inside one that swallowed
     * the wheel would trap a scroll over a channel, and controls small enough to
     * fit would be too small to hit before the card vanished. Everything that
     * makes the panel's map operable is therefore off, and what is left is the
     * one thing a glance is asking: where.
     */
    preview?: boolean;
}

const styles: Record<string, React.CSSProperties> = {
    root: {marginBottom: 16},
    frame: {
        position: 'relative',
        height: MAP_HEIGHT,
        borderRadius: 6,
        overflow: 'hidden',
        border: '1px solid rgba(var(--center-channel-color-rgb), 0.08)',
        background: 'rgba(var(--center-channel-color-rgb), 0.04)',
    },
    canvas: {position: 'absolute', inset: 0},
    fillRoot: {marginBottom: 0, display: 'flex', flexDirection: 'column', height: '100%'},
    fillFrame: {flex: 1, borderRadius: 0, border: 'none'},
    previewRoot: {marginBottom: 0},
    previewFrame: {width: PREVIEW_WIDTH_PX, height: PREVIEW_HEIGHT_PX},
    placeholder: {
        position: 'absolute',
        inset: 0,
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
        margin: 0,
        padding: '0 12px',
        textAlign: 'center',
        fontSize: 12,
        color: 'rgba(var(--center-channel-color-rgb), 0.72)',
        background: 'rgba(var(--center-channel-color-rgb), 0.04)',
    },
    caption: {
        display: 'flex',

        // Was space-between, which held the basemap credit at the left and this
        // at the right. With the credit gone a single child under space-between
        // falls back to the left edge, which is not where the link has been.
        justifyContent: 'flex-end',
        gap: 8,
        marginTop: 4,
        fontSize: 11,
        color: 'rgba(var(--center-channel-color-rgb), 0.64)',
    },
    link: {color: 'var(--link-color)'},
    recenter: {
        position: 'absolute',
        left: 8,
        top: 8,
        padding: '3px 8px',
        fontSize: 11,
        lineHeight: '16px',
        borderRadius: 4,
        cursor: 'pointer',
        border: '1px solid rgba(var(--center-channel-color-rgb), 0.16)',
        background: 'var(--center-channel-bg)',
        color: 'var(--center-channel-color)',
    },
    zoomLevel: {
        position: 'absolute',
        left: 8,
        bottom: 8,
        width: 'fit-content',
        margin: 0,
        padding: '2px 6px',
        borderRadius: 4,
        fontSize: 11,
        lineHeight: '16px',
        fontVariantNumeric: 'tabular-nums',
        pointerEvents: 'none',
        border: '1px solid rgba(var(--center-channel-color-rgb), 0.16)',
        background: 'var(--center-channel-bg)',
        color: 'rgba(var(--center-channel-color-rgb), 0.72)',
    },
    srOnly: {
        position: 'absolute',
        width: 1,
        height: 1,
        overflow: 'hidden',
        clip: 'rect(0 0 0 0)',
        whiteSpace: 'nowrap',
    },
};

const LocationMap: React.FC<Props> = ({
    lat, lon, cellDegLat, cellDegLon, region, pageHref, pending, fill, preview,
}) => {
    const container = useRef<HTMLDivElement | null>(null);
    const map = useRef<MapLibreMap | null>(null);
    const ready = useRef(false);

    // `loaded` mirrors the `ready` ref because applyView is a stable callback
    // that reads refs, while the note below has to re-render when readiness
    // changes. `failure` is separate so the two cannot overwrite each other.
    const [loaded, setLoaded] = useState(false);
    const [failure, setFailure] = useState<string | null>(null);

    // The live camera zoom, and the only thing on this map read from a `zoom`
    // event. Null until the map exists.
    const [zoomLevel, setZoomLevel] = useState<number | null>(null);

    const beyond = lat === null ? null : outsideMercator(lat);
    const known = lat !== null && lon !== null && beyond === null;

    // Derived, with one expression deciding it, rather than assigned from three
    // effects and two event handlers. The load handler used to set it to null
    // unconditionally while applyView was clearing the pin, so the two
    // disagreed; and it read a ref it could not re-render on, so a map that
    // finished loading after the reader moved on stayed on "Loading map…".
    const note = failure ?? positionNote(beyond, known, pending, loaded);

    // Read through a ref for the same reason `view` is: the creation effect runs
    // once and must not gain a dependency that would rebuild the map.
    const previewRef = useRef(preview);
    previewRef.current = preview;

    // Read at apply time rather than closed over, so the creation effect's
    // 'load' handler positions the map at wherever the reader is NOW, not at
    // whatever was selected when the load started.
    const view = useRef<View>({lat, lon, cellDegLat, cellDegLon});
    view.current = {lat, lon, cellDegLat, cellDegLon};

    const applyView = useCallback(() => {
        const instance = map.current;
        if (!instance || !ready.current) {
            return;
        }

        const pin = instance.getSource<GeoJSONSource>('pin');
        const cell = instance.getSource<GeoJSONSource>('cell');
        const current = view.current;

        // A stale pin is worse than no pin. Clicking a grid coordinate while an
        // earlier one is drawn would otherwise leave the previous position on
        // screen, beside the new one's readings, until the conversion lands, and
        // permanently if it never does.
        if (current.lat === null || current.lon === null || outsideMercator(current.lat)) {
            pin?.setData(emptyCollection());
            cell?.setData(emptyCollection());
            return;
        }

        const width = container.current?.clientWidth || DEFAULT_WIDTH_PX;

        // jumpTo, never flyTo: a world-crossing animation on every click is a
        // vestibular risk and says nothing in a box this size.
        instance.jumpTo({
            center: [current.lon, current.lat],
            zoom: zoomForSpan(current.lat, width),
        });

        pin?.setData(pointFeature(current.lat, current.lon));
        cell?.setData(drawableCell(current));
    }, []);

    // A verdict belongs to the coordinate that produced it, so a change of
    // position retires it.
    //
    // This is its own effect, ahead of creation, and that placement is the whole
    // point. Clearing inside the creation effect put the clear BEHIND
    // `if (map.current || !known)`, so it never ran in the two cases that matter
    // most: a map that was built and then errored before `load` (the likeliest
    // failure, since loadBasemap reads only 127 bytes and every tile fetch comes
    // after construction) latched `failure` with `ready` false, and every later
    // coordinate returned at that guard with nothing left to clear it. The note
    // read "The map could not be loaded" for the life of the panel while
    // applyView no-opped on !ready.current, so the map was frozen too.
    //
    // NO_WEBGL is kept rather than cleared. hasWebGL2 is memoised precisely
    // because the answer cannot change mid-session, so clearing it only made
    // every coordinate flash "Loading map…" before restating it.
    useEffect(() => {
        setFailure((prev) => (prev === NO_WEBGL ? prev : null));
    }, [lat, lon]);

    // Creation. Runs once the reader has something to look at, and then not
    // again while a usable map exists: the guard below, not the deps, is what
    // stops a rebuild.
    //
    // The position IS a dependency, and only because creation can fail. A
    // transient basemap or chunk failure leaves `map.current` null, and with the
    // position out of the deps this effect never ran again, so `loadBasemap()`
    // was never called again either and every later coordinate read "The map
    // could not be loaded" until the panel unmounted. That is precisely the
    // retry basemap.ts declines to latch away, made unreachable one layer up.
    //
    // The old shape also failed asymmetrically, which is the tell: `known`
    // flips false->true per selection for a GRID token (it has no position
    // until the conversion lands) and stays true across every textual one, so
    // grid links retried and lat/lon links did not.
    useEffect(() => {
        if (map.current || !known) {
            return undefined;
        }

        let live = true;

        (async () => {
            const [maplibre, basemap] = await Promise.all([loadMapLibre(), loadBasemap()]);
            if (!live) {
                return;
            }
            if (!maplibre) {
                // Two different failures. Reporting a load failure as a missing
                // capability sends the reader, and whoever they report it to,
                // looking at the wrong thing.
                setFailure(hasWebGL2() ? NO_BASEMAP : NO_WEBGL);
                return;
            }
            if (!basemap) {
                setFailure(NO_BASEMAP);
                return;
            }
            if (!container.current) {
                return;
            }

            const start = view.current;
            const width = container.current.clientWidth || DEFAULT_WIDTH_PX;

            let instance: MapLibreMap | undefined;
            try {
                instance = new maplibre.Map({
                    container: container.current,
                    style: buildStyle(basemap, mapColors()),
                    center: [start.lon ?? 0, start.lat ?? 0],
                    zoom: zoomForSpan(start.lat ?? 0, width),

                    // A rotatable map with no compass means a reader misreads
                    // every bearing taken off it.
                    dragRotate: false,
                    pitchWithRotate: false,
                    touchZoomRotate: false,

                    scrollZoom: !previewRef.current,
                    dragPan: !previewRef.current,
                    doubleClickZoom: !previewRef.current,
                    keyboard: !previewRef.current,
                    interactive: !previewRef.current,

                    // zoomForSpan clamps only the OPENING zoom, but the wheel
                    // and the controls let a reader leave that range, so the
                    // ceiling belongs on the map too. Why it is where it is:
                    // see MAX_ZOOM in span.ts.
                    maxZoom: MAX_ZOOM,
                    minZoom: 0,

                    attributionControl: false,
                    fadeDuration: 0,
                });

                if (!previewRef.current) {
                    instance.addControl(new maplibre.NavigationControl({showCompass: false}), 'top-right');
                    instance.addControl(new maplibre.ScaleControl({maxWidth: 90, unit: 'metric'}), 'bottom-right');
                }

                // Seeded as well as listened for, because constructing a map at
                // a zoom fires no event and the readout would otherwise be blank
                // until the reader's first gesture.
                instance.on('zoom', (event) => setZoomLevel(event.target.getZoom()));
                setZoomLevel(instance.getZoom());
            } catch (e) {
                // The constructor allocates its canvas and GL context before it
                // validates the style, so a throw here leaks a context unless it
                // is removed on the way out.
                instance?.remove();
                throw e;
            }

            // Without this a style or context failure never fires 'load' and the
            // panel sits on "Loading map…" with no way out. Only before the map
            // is usable: an error afterwards would replace a working map with a
            // notice saying it could not be loaded.
            //
            // TILE-scoped errors are ignored, and the distinction from
            // source-scoped ones is the whole of it. MapLibre attaches a
            // sourceId to EVERY error bubbling through a source, the archive
            // failing to load at all included, so filtering on that threw away
            // the one failure worth reporting: a truncated archive, or a
            // backend without byte serving, drew water with no land and said
            // nothing. Only a per-tile error carries `tile`.
            //
            // Worth knowing before narrowing this again: neither case it was
            // originally written for raises an error at all. A tile the archive
            // omits comes back from the pmtiles protocol as an empty buffer,
            // and a glyph range that 404s falls back to a locally drawn glyph
            // with a console warning.
            instance.on('error', (event) => {
                if (ready.current || (event as {tile?: unknown}).tile) {
                    return;
                }

                // Identity-guarded, like the load handler beside it. An instance
                // abandoned by a superseded run still carries these handlers, and
                // without this its error would write a verdict over the map the
                // reader is actually looking at.
                if (map.current !== instance) {
                    return;
                }

                setFailure(NO_BASEMAP);

                // Torn down, not just reported. This map never became usable, so
                // leaving it in the ref made `map.current` truthy forever and
                // every later creation attempt returned at the guard: the reader
                // was left with a frozen map, a stale note, and no way back short
                // of unmounting the panel. Releasing it also returns the WebGL
                // context rather than holding one for a map that draws nothing.
                instance.remove();
                map.current = null;
                mapObserver?.(null);
            });

            // Guarded on the instance, NOT on this effect run's `live` flag. The
            // map is stored on a ref and removed only on unmount, so it outlives
            // the run that made it. Tying readiness to `live` meant that a reader
            // who clicked a second coordinate while the first was still loading
            // left `ready` false forever: every later applyView no-opped and the
            // map sat on "Loading map…" until the page was reloaded.
            instance.on('load', () => {
                if (map.current !== instance) {
                    return;
                }
                ready.current = true;
                setLoaded(true);

                // An error before load must not outlive a map that then loads.
                // Nothing else clears `failure`, so it was a one-way latch: a
                // single timeout left every later coordinate reading "could not
                // be loaded" over a working map, which is exactly the transient
                // failure basemap.ts deliberately declines to remember.
                setFailure(null);
                applyView();
            });

            if (!live) {
                instance.remove();
                return;
            }
            map.current = instance;
            mapObserver?.(instance);
        })().catch(() => {
            if (live) {
                setFailure(NO_BASEMAP);
            }
        });

        return () => {
            live = false;
        };
    }, [known, lat, lon, applyView]);

    // Movement. The map itself survives; only its camera and its two overlay
    // sources change.
    useEffect(() => {
        applyView();
    }, [applyView, lat, lon, cellDegLat, cellDegLon]);

    // Torn down on unmount only. Browsers cap live WebGL contexts at about
    // sixteen, and the panel outlives any one coordinate.
    useEffect(() => () => {
        map.current?.remove();
        map.current = null;
        ready.current = false;
        mapObserver?.(null);
    }, []);

    // The sidebar is resizable and MapLibre does not notice on its own.
    useEffect(() => {
        if (typeof ResizeObserver === 'undefined' || !container.current) {
            return undefined;
        }

        // Deps are empty because the container div is rendered unconditionally
        // and so never changes identity. Make it conditional and this observer
        // silently stops tracking.
        const observer = new ResizeObserver(() => map.current?.resize());
        observer.observe(container.current);

        return () => observer.disconnect();
    }, []);

    let root = fill ? {...styles.root, ...styles.fillRoot} : styles.root;
    let frame = fill ? {...styles.frame, ...styles.fillFrame} : styles.frame;
    if (preview) {
        root = {...styles.root, ...styles.previewRoot};
        frame = {...styles.frame, ...styles.previewFrame};
    }

    return (
        <div style={root}>
            <div style={frame}>
                <div
                    ref={container}
                    style={styles.canvas}
                />
                {note === null && !preview && (
                    <button
                        type='button'
                        style={styles.recenter}
                        onClick={applyView}
                    >{RESET_LABEL}</button>
                )}
                {note !== null && (
                    <p
                        data-testid='map-note'
                        style={styles.placeholder}
                    >{note}</p>
                )}
                {note === null && !preview && zoomLevel !== null && (
                    <p style={styles.zoomLevel}>{`z${zoomLevel.toFixed(1)}`}</p>
                )}
                <span style={styles.srOnly}>{label(region, note)}</span>
            </div>
            {!fill && !preview && pageHref !== undefined && (
                <div style={styles.caption}>
                    <a
                        style={styles.link}
                        href={pageHref}
                        target='_blank'
                        rel='noreferrer'
                    >{'Open larger'}</a>
                </div>
            )}
        </div>
    );
};

/**
 * What the reader is told, which is not the same question as whether a map
 * exists: one can be drawn and still have nothing to point at.
 */
function positionNote(
    beyond: string | null, known: boolean, pending: boolean, loaded: boolean,
): string | null {
    if (beyond) {
        return beyond;
    }
    if (!known) {
        return pending ? LOADING : NO_POSITION;
    }
    return loaded ? null : LOADING;
}

function outsideMercator(lat: number): string | null {
    if (isRenderable(lat)) {
        return null;
    }

    return lat > 0 ? TOO_FAR_NORTH : TOO_FAR_SOUTH;
}

/**
 * The cell, or nothing, where nothing means only an unknown position or a token
 * that carries no resolution at all.
 */
function drawableCell(current: View): FeatureCollection {
    const {lat, lon, cellDegLat, cellDegLon} = current;
    if (lat === null || lon === null || !(cellDegLat > 0) || !(cellDegLon > 0)) {
        return emptyCollection();
    }

    // No minimum size, and no threshold below which the cell is dropped. These
    // surfaces zoom, so there is no one scale to test against: a metre-wide cell
    // is invisible until the reader zooms far enough to see it, which is more
    // honest than a number guessing on their behalf.
    //
    // There was a 6px floor here, measured once at the OPENING camera and never
    // recomputed, because nothing listens for zoom. It dropped every square
    // finer than about 45km permanently, including a 10km grid reference that
    // would have been 11px across at maximum zoom.
    return cellFeature(cellBounds(lat, lon, cellDegLat, cellDegLon));
}

/**
 * The accessible label, which is the only place the country reaches a reader.
 *
 * The Region row was retired, so this string is it: with the map hidden, or
 * read by eye rather than by a screen reader, there is no country anywhere.
 * Removing the region from here removes it from every surface at once.
 *
 * The basemap is not named. The region's own value carries its citation, and
 * naming the source again here printed it twice in one line.
 */
function label(region: string, note: string | null): string {
    if (note !== null) {
        return note;
    }

    if (region === '') {
        return 'World map with the position marked.';
    }

    return `World map. The marked position is in ${region}.`;
}

export default LocationMap;

/** @internal exported for tests */
export function _setMapObserverForTesting( // eslint-disable-line no-underscore-dangle, @typescript-eslint/naming-convention
    observer: ((map: MapLibreMap | null) => void) | null,
): void {
    mapObserver = observer;
}

/** @internal exported for tests */
export function _drawableCellForTesting(current: View): FeatureCollection { // eslint-disable-line no-underscore-dangle, @typescript-eslint/naming-convention
    return drawableCell(current);
}

/** @internal exported for tests */
export function _positionNoteForTesting( // eslint-disable-line no-underscore-dangle, @typescript-eslint/naming-convention
    beyond: string | null, known: boolean, pending: boolean, loaded: boolean,
): string | null {
    return positionNote(beyond, known, pending, loaded);
}
