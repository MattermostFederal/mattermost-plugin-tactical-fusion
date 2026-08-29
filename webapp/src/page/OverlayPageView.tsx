import React from 'react';

import type {OverlayPageData} from './payload';

import {CotMapCanvas} from '../cot/CotMap';
import {COT_POST_TYPE, fromProps as cotFromProps} from '../cot/types';
import {GeoJsonMapCanvas, mapLabel, markersFor, shapesFor} from '../geojson/GeoJsonMap';
import {GEOJSON_POST_TYPE, fromProps as geoJsonFromProps} from '../geojson/types';

const styles: Record<string, React.CSSProperties> = {
    root: {position: 'fixed', inset: 0, display: 'flex', flexDirection: 'column'},
    map: {flex: 1, minHeight: 0},
    bar: {
        display: 'flex',
        flexWrap: 'wrap',
        alignItems: 'baseline',
        gap: '4px 16px',
        padding: '10px 16px',
        fontSize: 13,
        background: 'var(--center-channel-bg)',
        borderTop: '1px solid rgba(var(--center-channel-color-rgb), 0.12)',
    },
    what: {fontWeight: 600},
    empty: {padding: '24px 16px'},
};

export const OVERLAY_UNREADABLE = 'This build cannot read what this post carries.';

/**
 * A stamped post's overlay, given the whole window.
 *
 * The card's own reader and the card's own canvas, both of them. Nothing here
 * decides what a document draws: `fromProps` is the same function the post body
 * in the channel calls, and the canvas is the same component, so this page
 * cannot come to disagree with the card it was opened from. That is the whole
 * reason the shell carries the props blob rather than a list of markers and
 * shapes worked out in Go.
 */
export const OverlayPageView: React.FC<{data: OverlayPageData}> = ({data}) => {
    const drawn = drawingFor(data);

    return (
        <div style={styles.root}>
            {drawn === null ? (
                <p style={styles.empty}>{OVERLAY_UNREADABLE}</p>
            ) : (
                <div style={styles.map}>{drawn.canvas}</div>
            )}
            <div style={styles.bar}>
                <span style={styles.what}>{drawn === null ? '' : drawn.label}</span>
            </div>
        </div>
    );
};

/**
 * What to draw and what to call it, or null when nothing can be.
 *
 * Keyed on the post type, which the server took from the post itself, so an
 * overlay is never read with the other format's reader. A blob the reader
 * refuses lands here as null and says so, rather than rendering a window of
 * empty basemap that looks like a document with nothing in it.
 */
function drawingFor(data: OverlayPageData): {canvas: React.ReactNode; label: string} | null {
    if (data.kind === COT_POST_TYPE) {
        const payload = cotFromProps(data.props);
        if (payload === null) {
            return null;
        }

        return {
            canvas: (
                <CotMapCanvas
                    events={payload.events}
                    pageEnabled={false}
                    fill={true}
                />
            ),
            label: `${payload.events.length} event${payload.events.length === 1 ? '' : 's'}`,
        };
    }

    if (data.kind === GEOJSON_POST_TYPE) {
        const payload = geoJsonFromProps(data.props);
        if (payload === null) {
            return null;
        }

        return {
            canvas: (
                <GeoJsonMapCanvas
                    payload={payload}
                    fill={true}
                />
            ),
            label: mapLabel(markersFor(payload.features).length, shapesFor(payload.features).length),
        };
    }

    return null;
}
