import type {GeoJSONSource, Map as MapLibreMap} from 'maplibre-gl';
import {useCallback, useEffect, useRef, useState} from 'react';

import type {Bounds} from './basemap';
import {loadBasemap, loadPackages} from './basemap';
import {centerOf, degenerate, frameBounds, openingAnchor} from './bounds';
import {outsideMercator, positionNote} from './label';
import {
    SEAM_CAPPED_LAYERS,
    buildStyle, coveredBy, emptyCollection, hasWebGL2,
    loadMapLibre, mapColors, syncGlobalReach,
} from './maplibre';
import type {MapEllipse, MapMarker} from './overlay';
import {
    addMarkerImages, drawableAccuracy, drawableCell, drawableMarkers, drawableOverlay,
    markerFeatures, overlayDigest,
} from './overlay';
import type {MapShape} from './paint';
import {paintGeometry} from './paint';
import {MAX_ZOOM, fitPadding, zoomForSpan} from './span';
import type {View} from './view';

import {loadPackageNames} from '../../../packages/store';

const MAP_MIN_HEIGHT_PX = 200;
const MAP_MAX_HEIGHT_PX = 360;

/**
 * How tall the map is.
 *
 * A range rather than a number, because the sidebar's height is the reader's
 * screen and not ours. CSS does this without measuring anything, and MapLibre's
 * ResizeObserver already picks up the change.
 */
export const MAP_HEIGHT = `clamp(${MAP_MIN_HEIGHT_PX}px, 30vh, ${MAP_MAX_HEIGHT_PX}px)`;

/** What to assume the box is before it has been laid out. */
const DEFAULT_WIDTH_PX = 320;

/**
 * How long a constructed map has to say it is ready.
 *
 * MapLibre does its tiling in a worker. If that worker never starts, no source
 * ever finishes and `load` never fires, and because that is not an ERROR the
 * error handler below never runs either: the map sits on "Loading map…"
 * forever, with a reload the only way out. A worker URL that 404s is the way
 * this happens in the field, and asset_fixtures.ts records the same failure
 * from the other side.
 *
 * So readiness is bounded, exactly as loadMapLibre and basemap.ts bound their
 * own fetches and for the same reason: a hang that reports nothing is worse
 * than a failure that does. Generous rather than tight, because `load` waits
 * for the first tiles and this plugin's readers are on constrained links; the
 * point is to end an infinite wait, not to police a slow one.
 */
const READY_DEADLINE_MS = 20000;

let readyDeadlineMs = READY_DEADLINE_MS;
const NO_WEBGL = 'This browser cannot draw the map.';
const NO_BASEMAP = 'The map could not be loaded.';

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

/**
 * Everything a surface asks this map to draw. The map's public contract, which
 * is why it lives beside the hook that consumes it rather than beside the
 * component that forwards it.
 */
export interface MapProps extends View {
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

    /**
     * A stated circular error, in meters, drawn on the ground around the pin.
     *
     * Absent means the source stated no accuracy, and nothing is drawn: a
     * default radius would be one this component invented. Every location
     * surface omits it, so a coordinate's own precision keeps being carried by
     * the cell rather than by a circle.
     */
    accuracyMeters?: number;

    /**
     * How the accuracy reads, already rendered by whoever stated it.
     *
     * Taken rather than formatted here so the label and the row a reader sees
     * beside it cannot say different things: rounding meters a second time
     * turned a stated 0.4 into "within 0 meters", which is a claim of a perfect
     * fix made only to a screen reader.
     */
    accuracyLabel?: string;

    /**
     * Every point to mark, each in its own color, which also asks for the
     * reticle shape rather than the plain dot.
     *
     * Absent on every location surface, which keeps the single pin they have
     * always drawn from `lat` and `lon`. A Cursor on Target card passes one per
     * event, so a block of them draws each track in its own affiliation's
     * color, and `lat` and `lon` stay the primary position that decides
     * whether there is anything to draw at all.
     */
    markers?: readonly MapMarker[];

    /**
     * What the marker IS, in words.
     *
     * Required alongside a marker color in practice, because the color is
     * carrying a meaning and a color cannot be the only way to read one: the
     * affiliation hues are near enough in luminance that red and green are the
     * same mark to a good share of readers. This is the other channel.
     */
    markerLabel?: string;

    /**
     * An ellipse around the primary position, in meters on the ground.
     *
     * The one shape that cannot be a `MapShape`, because it is placed by this
     * map's anchor rather than by its own vertices. Everything else, including
     * what used to be the `outline` variant beside it, goes in `geometries`.
     */
    ellipse?: MapEllipse;

    /**
     * Every shape to draw, each carrying its own rings.
     *
     * Rings rather than a flat point list, because ring 0 is a polygon's
     * exterior and the rest are its holes: passing a holed polygon as two
     * separate outlines paints the hole as a solid island on the fill layer.
     *
     * Vertices are deliberately not markers: routing them through `markers`
     * would be less code and would put a reticle on every corner of a polygon,
     * which says each corner is a position somebody reported.
     *
     * Each shape may state its own color, validated by `styleOf` before it
     * reaches the collection. A shape that states none is drawn in the theme's,
     * which is what the `coalesce` in `paintGeometry` falls back to.
     */
    geometries?: readonly MapShape[];

    /**
     * What the overlay IS, in words, for a map that draws no pin.
     *
     * The accessible label otherwise says "the position marked", which an
     * extent-only map has not drawn. Words are the only channel a map has for a
     * reader who gets no picture.
     */
    extentLabel?: string;
}

/**
 * The MapLibre instance, and everything it reports about itself.
 *
 * Owns the map's whole lifetime: creation once the reader has something to look
 * at, the readiness deadline, the camera and overlay writes, and teardown. The
 * component above it decides how to present what this returns and nothing else.
 *
 * The map is created ONCE and moved thereafter. Rebuilding it per coordinate
 * meant a fresh WebGL context and a re-tessellation of the whole basemap on
 * every click, against the browser's cap of roughly sixteen live contexts, and
 * the panel stays mounted across a change of selection so those clicks arrive
 * in a run.
 */
export function useMapInstance({
    lat, lon, cellDegLat, cellDegLon, pending, preview, accuracyMeters,
    markers, ellipse, geometries,
}: MapProps): {
    container: React.RefObject<HTMLDivElement | null>;
    applyView: () => void;
    note: string | null;
    credited: boolean;
    zoomLevel: number | null;
    extentOnly: boolean;
} {
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

    // Whether the OpenStreetMap tier made it into the style, which is the only
    // thing that decides whether its credit is drawn. Set from the archive that
    // was actually handed to buildStyle rather than from a probe, so a credit
    // can never appear over a map that has no OSM in it, nor be missing from
    // one that does.
    const [credited, setCredited] = useState(false);

    const beyond = lat === null ? null : outsideMercator(lat);

    // Positioned is the old `known`: this map has a primary position to pin.
    // Known is now the wider question the creation guard and the note ask,
    // which an extent-only map answers yes to with no position at all.
    /*
     * Derived, not passed.
     *
     * "No primary position, but something to frame" is exactly what a caller
     * with markers or shapes and a null lat/lon is saying. It was an explicit
     * prop to keep null from meaning two things; the reason that mattered was
     * that a null WITHOUT anything to draw must still read as unavailable, and
     * the clause below is what keeps that true.
     */
    const extentOnly = lat === null &&
        ((markers ?? []).length > 0 || (geometries ?? []).length > 0);

    const positioned = lat !== null && lon !== null && beyond === null;
    const known = positioned || extentOnly;

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

    const radius = useRef<number | undefined>(accuracyMeters);
    radius.current = accuracyMeters;

    const drawn = useRef<readonly MapMarker[] | undefined>(markers);
    drawn.current = markers;

    const shape = useRef<MapEllipse | undefined>(ellipse);
    shape.current = ellipse;

    const shapes = useRef<readonly MapShape[] | undefined>(geometries);
    shapes.current = geometries;

    const extentOnlyRef = useRef(extentOnly);
    extentOnlyRef.current = extentOnly;

    // Pending readiness deadlines, so unmounting cannot leave one to fire
    // against a component that is gone.
    const deadlines = useRef<Set<ReturnType<typeof setTimeout>>>(new Set());

    const applyView = useCallback(() => {
        const instance = map.current;
        if (!instance || !ready.current) {
            return;
        }

        // Here rather than once at load. The image a marker names is keyed by
        // its color, and a panel reuses one map across selections, so a color
        // the first event did not carry had no image and the symbol layer drew
        // nothing at all for it.
        addMarkerImages(instance, drawn.current);

        const pin = instance.getSource<GeoJSONSource>('pin');
        const cell = instance.getSource<GeoJSONSource>('cell');
        const accuracy = instance.getSource<GeoJSONSource>('accuracy');
        const outline = instance.getSource<GeoJSONSource>('geometry');
        const current = view.current;

        const width = container.current?.clientWidth || DEFAULT_WIDTH_PX;
        const height = container.current?.clientHeight || MAP_MIN_HEIGHT_PX;

        // The camera, once, for both paths. jumpTo, never flyTo: a
        // world-crossing animation on every click is a vestibular risk and says
        // nothing in a box this size.
        //
        // Several markers, or any shape, frame ALL of them, because a block of
        // events opening on the first one would put the rest off screen with
        // nothing to say they were there. The union is taken across markers and
        // shapes together: a shape larger than its own <point> would otherwise
        // open half off screen, or at a zoom chosen for a point inside it.
        const frame = (fallback: {lat: number; lon: number} | null) => {
            const box = frameBounds(
                drawn.current, shape.current, shapes.current, current.lat, current.lon);

            if (box !== null && !degenerate(box)) {
                instance.fitBounds(box, {
                    padding: fitPadding(width, height, !previewRef.current),
                    animate: false,
                    maxZoom: MAX_ZOOM,
                });
                return;
            }

            // A degenerate box is a single point, a due-north line or a
            // zero-area polygon. fitBounds takes those to maxZoom, which is a
            // street-level view of something that may be a country wide.
            const center = box === null ? fallback : centerOf(box);
            if (center === null) {
                return;
            }

            instance.jumpTo({
                center: [center.lon, center.lat],
                zoom: zoomForSpan(center.lat, width),
            });
        };

        // No primary position.
        //
        // An extent-only map draws its overlay and frames it, and draws no pin,
        // no cell and no accuracy ring: those three are all about a position
        // this surface does not have. Every other caller clears instead, because
        // a stale pin is worse than no pin: clicking a grid coordinate while an
        // earlier one is drawn would otherwise leave the previous position on
        // screen, beside the new one's readings, until the conversion lands, and
        // permanently if it never does.
        if (current.lat === null || current.lon === null || outsideMercator(current.lat)) {
            cell?.setData(emptyCollection());
            accuracy?.setData(emptyCollection());

            if (!extentOnlyRef.current) {
                pin?.setData(emptyCollection());
                outline?.setData(emptyCollection());
                return;
            }

            pin?.setData(markerFeatures(drawn.current));
            outline?.setData(drawableOverlay(
                shape.current, shapes.current, current.lat, current.lon,
            ));
            paintGeometry(instance);
            frame(null);
            return;
        }

        frame({lat: current.lat, lon: current.lon});

        pin?.setData(drawableMarkers(drawn.current, current.lat, current.lon));
        cell?.setData(drawableCell(current));
        accuracy?.setData(drawableAccuracy(current.lat, current.lon, radius.current));
        outline?.setData(drawableOverlay(shape.current, shapes.current, current.lat, current.lon));
        paintGeometry(instance);
    }, []);

    // What the overlays ARE, rather than the objects carrying them.
    //
    // A caller that builds its geometry inline hands over a new object every
    // render, which re-framed the camera under a reader who had panned; and
    // markers were read through a ref and named nowhere, so a changed marker
    // set over an unchanged position never redrew at all.
    //
    // Declared ahead of both effects that read it, because the creation effect
    // depends on it as well as the movement one.
    const overlayKey = overlayDigest(markers, ellipse, geometries);

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

        // overlayKey, not just the position. An extent-only surface passes a
        // literal null lat and lon forever, so keyed on those alone this ran
        // once at mount and never again: a verdict latched by one transient
        // basemap failure outlived every later document, while the creation
        // effect below retried on exactly the key this was missing.
    }, [lat, lon, overlayKey]);

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
            // A hover card is built with interactive: false at a zoom
            // zoomForSpan has already clamped below the seam, so it can never
            // request an OpenStreetMap tile. Carrying the source anyway would
            // put an OSM credit on a card with no OSM on it.
            // The package list is AWAITED here rather than read from a hook,
            // and that is not a style choice. The map is created once and then
            // moved, so a list that arrives after creation would never reach a
            // style again: the panel would draw the global tier for the whole
            // of its life on an install that has detail areas, which looks
            // exactly like an install that has none. Both this and the archives
            // it names are bounded and module-cached, so the cost is one
            // request per session rather than per map.
            const names = previewRef.current ? [] : await loadPackageNames();
            const [maplibre, basemap, details] = await Promise.all([
                loadMapLibre(),
                loadBasemap(),
                loadPackages(names),
            ]);
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

            setCredited(details.length > 0);

            const start = view.current;
            const width = container.current.clientWidth || DEFAULT_WIDTH_PX;

            // Where a map with no position of its own opens. Without this the
            // ?? 0 fallbacks below put it at 0,0 for the frame or two before
            // applyView runs, which is a visible jump from the Gulf of Guinea
            // to wherever the document actually is.
            const anchor = openingAnchor(start, drawn.current, shapes.current);

            // A point rather than the opening viewport, and it does not matter
            // which: zoomForSpan clamps the opening zoom to DATA_MAX_ZOOM, which
            // is below SEAM_ZOOM, so nothing the cap governs is drawn until the
            // reader zooms in and moveend has re-decided from real bounds.
            const opening: Bounds = [anchor.lon, anchor.lat, anchor.lon, anchor.lat];
            const style = buildStyle(
                basemap, details, mapColors(), !coveredBy(details, opening),
            );

            let instance: MapLibreMap | undefined;
            try {
                instance = new maplibre.Map({
                    container: container.current,
                    style,
                    center: [anchor.lon, anchor.lat],
                    zoom: zoomForSpan(anchor.lat, width),

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
                // Rounded to the readout's own precision (z0.0). The zoom event fires
                // per animation frame through a wheel gesture, and useState's Object.is
                // bailout then drops most of those renders, taking the per-frame
                // overlayDigest walk and ref churn with them.
                instance.on('zoom', (event) => setZoomLevel(Math.round(event.target.getZoom() * 10) / 10));
                setZoomLevel(Math.round(instance.getZoom() * 10) / 10);

                instance.on('moveend', (event) => syncGlobalReach(event.target, details, SEAM_CAPPED_LAYERS));
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
            // Armed once the map exists, because everything before this point
            // is already bounded by its own timeout.
            const deadline = setTimeout(() => {
                // Retired here as well as on load. A deadline that FIRES, or one
                // whose creation attempt was superseded, stayed in the Set until
                // unmount; clearTimeout on a fired timer is a no-op, so this is
                // about the Set not growing per attempt rather than about the
                // timer.
                deadlines.current.delete(deadline);

                if (map.current !== instance || ready.current) {
                    return;
                }

                setFailure(NO_BASEMAP);

                // Torn down for the reason the error handler tears down: this
                // map never became usable, so leaving it in the ref makes
                // map.current truthy forever and every later attempt returns at
                // the creation guard. Releasing it also hands back the WebGL
                // context rather than holding one for a map that draws nothing.
                instance.remove();
                map.current = null;
                mapObserver?.(null);
            }, readyDeadlineMs);
            deadlines.current.add(deadline);

            instance.on('load', () => {
                if (map.current !== instance) {
                    return;
                }

                clearTimeout(deadline);
                deadlines.current.delete(deadline);

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
    }, [known, lat, lon, applyView, overlayKey]);

    // Movement. The map itself survives; only its camera and its four sources
    // change.

    useEffect(() => {
        applyView();
    }, [applyView, lat, lon, cellDegLat, cellDegLon, accuracyMeters, overlayKey]);

    // Torn down on unmount only. Browsers cap live WebGL contexts at about
    // sixteen, and the panel outlives any one coordinate.
    useEffect(() => () => {
        for (const deadline of deadlines.current) {
            clearTimeout(deadline);
        }
        deadlines.current.clear();

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

    return {container, applyView, note, credited, zoomLevel, extentOnly};
}

/** Shortens the readiness deadline so a test can prove it fires. */
export function _setReadyDeadlineForTesting(ms: number | null): void { // eslint-disable-line no-underscore-dangle, @typescript-eslint/naming-convention
    readyDeadlineMs = ms ?? READY_DEADLINE_MS;
}

/** Hands each created map to a test, and null on teardown. */
export function _setMapObserverForTesting( // eslint-disable-line no-underscore-dangle, @typescript-eslint/naming-convention
    observer: ((map: MapLibreMap | null) => void) | null,
): void {
    mapObserver = observer;
}
