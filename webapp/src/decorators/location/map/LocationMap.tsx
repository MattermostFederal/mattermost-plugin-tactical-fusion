import type {FeatureCollection} from 'geojson';
import type {GeoJSONSource, Map as MapLibreMap} from 'maplibre-gl';
import React, {useCallback, useEffect, useRef, useState} from 'react';

import {loadBasemap} from './basemap';
import {
    buildStyle, cellFeature, emptyCollection, hasWebGL2, loadMapLibre, mapColors, pointFeature,
} from './maplibre';
import {MAX_ZOOM, cellBounds, isRenderable, zoomForSpan} from './span';

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
export const MAP_MIN_HEIGHT_PX = 200;
export const MAP_MAX_HEIGHT_PX = 360;
const MAP_HEIGHT = `clamp(${MAP_MIN_HEIGHT_PX}px, 30vh, ${MAP_MAX_HEIGHT_PX}px)`;

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
const MIN_CELL_PX = 6;

interface View {
    lat: number | null;
    lon: number | null;
    cellDegLat: number;
    cellDegLon: number;
}

interface Props extends View {
    region: string;
    pending: boolean;

    /**
     * Where "Open larger" goes, or absent on the page that IS the larger view.
     */
    pageHref?: string;

    /**
     * Overrides the panel's height. The map page gives the whole window to one
     * picture, which is the thing a reader following "Open larger" asked for.
     */
    height?: string;

    /** Fills its parent rather than sitting in the flow of a panel. */
    fill?: boolean;
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
        justifyContent: 'space-between',
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
    lat, lon, cellDegLat, cellDegLon, region, pageHref, pending, height, fill,
}) => {
    const container = useRef<HTMLDivElement | null>(null);
    const map = useRef<MapLibreMap | null>(null);
    const ready = useRef(false);
    const [note, setNote] = useState<string | null>(LOADING);

    const beyond = lat === null ? null : outsideMercator(lat);
    const known = lat !== null && lon !== null && beyond === null;

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
        cell?.setData(drawableCell(current, instance));
    }, []);

    // Creation. Runs once the reader has something to look at, and then not
    // again: the deps are deliberately not the position.
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
                setNote(hasWebGL2() ? NO_BASEMAP : NO_WEBGL);
                return;
            }
            if (!basemap) {
                setNote(NO_BASEMAP);
                return;
            }
            if (!container.current) {
                return;
            }

            const start = view.current;
            const width = container.current.clientWidth || DEFAULT_WIDTH_PX;

            let instance;
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

                    scrollZoom: true,

                    // The basemap is Natural Earth 110m, which is honest to about
                    // z6: past that a reader is looking at a polygonal coastline
                    // at street scale and has no way to tell. zoomForSpan already
                    // clamps the OPENING zoom, but the controls and the wheel let
                    // a reader leave that range, so the ceiling belongs on the map
                    // and not only on the arithmetic that picks the first view.
                    maxZoom: MAX_ZOOM,
                    minZoom: 0,

                    attributionControl: false,
                    fadeDuration: 0,
                });

                instance.addControl(new maplibre.NavigationControl({showCompass: false}), 'top-right');
                instance.addControl(new maplibre.ScaleControl({maxWidth: 90, unit: 'metric'}), 'bottom-right');
            } catch (e) {
                // The constructor allocates its canvas and GL context before it
                // validates the style, so a throw here leaks a context unless it
                // is removed on the way out.
                instance?.remove();
                throw e;
            }

            // Without this a style or context failure never fires 'load' and the
            // panel sits on "Loading map…" with no way out.
            instance.on('error', () => {
                if (live) {
                    setNote(NO_BASEMAP);
                }
            });

            instance.on('load', () => {
                if (!live) {
                    return;
                }
                ready.current = true;
                applyView();
                setNote(null);
            });

            if (!live) {
                instance.remove();
                return;
            }
            map.current = instance;
        })().catch(() => {
            if (live) {
                setNote(NO_BASEMAP);
            }
        });

        return () => {
            live = false;
        };
    }, [known, applyView]);

    // Movement. The map itself survives; only its camera and its two overlay
    // sources change.
    useEffect(() => {
        applyView();
    }, [applyView, lat, lon, cellDegLat, cellDegLon]);

    // What the reader is told, which is not the same question as whether a map
    // exists: one can be drawn and still have nothing to point at.
    useEffect(() => {
        if (beyond) {
            setNote(beyond);
            return;
        }
        if (!known) {
            setNote(pending ? LOADING : NO_POSITION);
            return;
        }
        if (ready.current) {
            setNote(null);
        }
    }, [beyond, known, pending]);

    // Torn down on unmount only. Browsers cap live WebGL contexts at about
    // sixteen, and the panel outlives any one coordinate.
    useEffect(() => () => {
        map.current?.remove();
        map.current = null;
        ready.current = false;
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

    const root = fill ? {...styles.root, ...styles.fillRoot} : styles.root;
    const frame = {
        ...styles.frame,
        ...(fill ? styles.fillFrame : null),
        ...(height === undefined ? null : {height}),
    };

    return (
        <div style={root}>
            <div style={frame}>
                <div
                    ref={container}
                    style={styles.canvas}
                />
                {note === null && (
                    <button
                        type='button'
                        style={styles.recenter}
                        onClick={applyView}
                    >{RESET_LABEL}</button>
                )}
                {note !== null && <p style={styles.placeholder}>{note}</p>}
                <span style={styles.srOnly}>{label(region, note)}</span>
            </div>
            {!fill && (
                <div style={styles.caption}>
                    <span>{note === null ? scaleNote() : null}</span>
                    {pageHref !== undefined && (
                        <a
                            style={styles.link}
                            href={pageHref}
                            target='_blank'
                            rel='noreferrer'
                        >{'Open larger'}</a>
                    )}
                </div>
            )}
        </div>
    );
};

function outsideMercator(lat: number): string | null {
    if (isRenderable(lat)) {
        return null;
    }

    return lat > 0 ? TOO_FAR_NORTH : TOO_FAR_SOUTH;
}

/**
 * The cell, or nothing.
 *
 * Below a few pixels a rectangle is not information, so a metre-resolution token
 * draws the dot alone and the caption carries the figure instead.
 */
function drawableCell(current: View, instance: MapLibreMap): FeatureCollection {
    const {lat, lon, cellDegLat, cellDegLon} = current;
    if (lat === null || lon === null || !(cellDegLat > 0) || !(cellDegLon > 0)) {
        return emptyCollection();
    }

    const bounds = cellBounds(lat, lon, cellDegLat, cellDegLon);
    try {
        const a = instance.project(bounds[0]);
        const b = instance.project(bounds[1]);
        if (Math.abs(b.x - a.x) < MIN_CELL_PX || Math.abs(b.y - a.y) < MIN_CELL_PX) {
            return emptyCollection();
        }
    } catch {
        return emptyCollection();
    }

    return cellFeature(bounds);
}

/**
 * The caption names the basemap, and only the basemap.
 *
 * It used to append the region as well, which read
 * "Natural Earth 110m. United States of America (Natural Earth 110m)": the
 * Region row's value already carries its own citation, so the source was
 * printed twice in one line. The row names the place, this names what drew it.
 */
function scaleNote(): string {
    return 'Natural Earth 110m';
}

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
