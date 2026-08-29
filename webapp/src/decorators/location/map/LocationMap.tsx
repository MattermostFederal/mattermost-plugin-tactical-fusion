import React from 'react';

import {label} from './label';
import {OSM_CREDIT} from './maplibre';
import type {MapProps} from './use_map_instance';
import {MAP_HEIGHT, useMapInstance} from './use_map_instance';

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

// One label across all three maps. The pages call it the same thing, and a
// reader who moves between them should not have to learn a second word for the
// same button. It resets the zoom as well as the center, which is why it is not
// "Recenter".
const RESET_LABEL = 'Reset view';

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
        justifyContent: 'flex-end',
        gap: 8,
        flexWrap: 'wrap',
        marginTop: 4,
        fontSize: 11,
        color: 'rgba(var(--center-channel-color-rgb), 0.64)',
    },
    credit: {display: 'inline-flex', gap: 6, marginRight: 'auto'},
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

/**
 * The map above the readings table.
 *
 * Presentation only. `useMapInstance` owns the MapLibre instance and everything
 * it reports; this decides how that looks and what it says.
 *
 * Nothing here may fail the panel. Every failure hides the map and leaves every
 * reading on screen, and no pin is ever drawn at a guessed position: a marker
 * at 0, 0 because a conversion failed is a position, and a wrong one.
 */
const LocationMap: React.FC<MapProps> = (props) => {
    const {region, pageHref, fill, preview, accuracyLabel, markers, markerLabel,
        extentOnly, extentLabel} = props;
    const {container, applyView, note, credited, zoomLevel} = useMapInstance(props);

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
                <span style={styles.srOnly}>{label(region, note, accuracyLabel, markerLabel, markers?.length ?? 1, extentOnly === true ? extentLabel : undefined)}</span>
            </div>
            {!preview && (credited || (!fill && pageHref !== undefined)) && (
                <div style={styles.caption}>
                    {credited && (
                        <span style={styles.credit}>
                            {OSM_CREDIT.map((credit) => (
                                <a
                                    key={credit.href}
                                    style={styles.link}
                                    href={credit.href}
                                    target='_blank'
                                    rel='noreferrer'
                                >{credit.label}</a>
                            ))}
                        </span>
                    )}
                    {!fill && pageHref !== undefined && (
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

export {MAP_HEIGHT};

export default LocationMap;
