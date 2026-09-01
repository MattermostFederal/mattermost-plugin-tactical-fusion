import React from 'react';

import GeoJsonMap from './GeoJsonMap';
import {showGeoJsonDocument} from './panel';
import type {GeoJsonFeature, GeoJsonPayload} from './types';
import {ringCount, solePosition, vertexCount} from './types';

import ErrorBoundary from '../components/ErrorBoundary';

interface Props {
    payload: GeoJsonPayload;
    compactDisplay?: boolean;
}

export const CARD_KIND = 'GeoJSON';

/**
 * What a boundary says when it catches something.
 *
 * A boundary rendering null leaves a card with a heading and nothing under it,
 * and no reader can tell that from a document that stated nothing.
 */
export const DETAIL_FAILED = 'The detail of this document could not be rendered. Open details to read the document as it was posted.';

/**
 * How tall the feature list may grow before it scrolls.
 *
 * A document may carry 256 features, each with up to 32 properties, which is
 * several screens of card in the middle of a channel. The list scrolls inside
 * itself rather than pushing every other post out of reach.
 */
const LIST_MAX_HEIGHT_PX = 420;

const styles: Record<string, React.CSSProperties> = {
    text: {whiteSpace: 'pre-wrap'},
    card: {
        border: '1px solid rgba(var(--center-channel-color-rgb), 0.16)',
        borderRadius: 4,
        marginTop: 8,
        maxWidth: 640,
        overflow: 'hidden',
    },
    kind: {fontWeight: 700, margin: 0, padding: '8px 12px 0'},
    header: {
        alignItems: 'baseline',
        display: 'flex',
        flexWrap: 'wrap',
        gap: '0.5em',
        padding: '2px 12px 8px',
    },
    heading: {fontWeight: 600},
    summary: {opacity: 0.85},
    note: {opacity: 0.9, padding: '0 12px 8px'},
    list: {
        listStyle: 'none',
        margin: 0,
        maxHeight: LIST_MAX_HEIGHT_PX,
        overflowY: 'auto',
        padding: '0 12px 8px',
    },
    listItem: {
        borderTop: '1px solid rgba(var(--center-channel-color-rgb), 0.08)',
        padding: '6px 0',
    },
    featureHead: {alignItems: 'baseline', display: 'flex', flexWrap: 'wrap', gap: '0.5em'},
    name: {fontWeight: 600},
    kindLabel: {fontFamily: 'monospace', fontSize: '0.85em', opacity: 0.9},
    shape: {opacity: 0.85, fontSize: '0.9em'},
    measure: {fontWeight: 600, fontSize: '0.9em'},
    coord: {fontFamily: 'monospace', fontSize: '0.9em'},
    featureNote: {opacity: 0.9, fontSize: '0.9em', margin: '2px 0 0'},
    properties: {
        display: 'grid',
        gridTemplateColumns: 'max-content 1fr',
        gap: '1px 12px',
        margin: '4px 0 0',
        fontSize: '0.9em',
    },
    term: {opacity: 0.85},
    value: {margin: 0, wordBreak: 'break-word'},
    actions: {display: 'flex', gap: '12px', padding: '0 12px 8px'},
    button: {
        background: 'none',
        border: 'none',
        color: 'var(--link-color)',
        cursor: 'pointer',
        font: 'inherit',
        padding: 0,
    },
};

/** The geometry mix, as a sentence rather than a table of zeroes. */
export function summaryLine(payload: GeoJsonPayload): string {
    const {counts} = payload;

    const parts: string[] = [];
    if (counts.points > 0) {
        parts.push(plural(counts.points, 'point'));
    }
    if (counts.lines > 0) {
        parts.push(plural(counts.lines, 'line'));
    }
    if (counts.polygons > 0) {
        parts.push(plural(counts.polygons, 'polygon'));
    }
    if (counts.collections > 0) {
        parts.push(plural(counts.collections, 'collection'));
    }

    const tail: string[] = [];
    if (counts.unlocated > 0) {
        tail.push(`${counts.unlocated} with no position`);
    }
    if (counts.undrawable > 0) {
        tail.push(`${counts.undrawable} not drawn`);
    }

    const mix = parts.length === 0 ? '' : joinWords(parts);
    if (tail.length === 0) {
        return mix;
    }

    return mix === '' ? joinWords(tail) : `${mix}, ${joinWords(tail)}`;
}

function plural(n: number, word: string): string {
    return `${n} ${word}${n === 1 ? '' : 's'}`;
}

function joinWords(parts: readonly string[]): string {
    if (parts.length < 2) {
        return parts.join('');
    }
    return `${parts.slice(0, -1).join(', ')} and ${parts[parts.length - 1]}`;
}

/**
 * What a feature's geometry is, in words.
 *
 * Counts rather than coordinates for anything but a lone point: a polygon's
 * vertices are not positions somebody reported, and listing them would say they
 * were.
 */
export function shapeLine(feature: GeoJsonFeature): string {
    if (feature.kind === 'none') {
        return '';
    }

    const vertices = vertexCount(feature);
    const rings = ringCount(feature);

    if (feature.kind === 'Point' && vertices === 1) {
        return '';
    }
    if (feature.kind === 'Polygon' || feature.kind === 'MultiPolygon') {
        return `${plural(rings, 'ring')}, ${plural(vertices, 'point')}`;
    }

    return plural(vertices, 'point');
}

/**
 * What the geometry measures, as the server rendered it.
 *
 * Taken rather than computed, so the card and the panel cannot round the same
 * figure into two different answers. Both empty means the geometry has no such
 * measure, or the server would not stand behind the shape.
 */
export function measureLine(feature: GeoJsonFeature): string {
    return [feature.length, feature.area].filter((part) => part !== '').join(', ');
}

const Feature: React.FC<{feature: GeoJsonFeature}> = ({feature}) => {
    const position = solePosition(feature);
    const shape = shapeLine(feature);
    const measure = measureLine(feature);

    return (
        <li style={styles.listItem}>
            <div style={styles.featureHead}>
                <span style={styles.name}>{feature.name}</span>
                <span style={styles.kindLabel}>{feature.kind === 'none' ? 'no geometry' : feature.kind}</span>
                {position !== null && (
                    <span style={styles.coord}>{`${position.lat}, ${position.lon}`}</span>
                )}
                {shape !== '' && <span style={styles.shape}>{shape}</span>}
                {measure !== '' && (
                    <span
                        style={styles.measure}
                        data-testid='geojson-measure'
                    >
                        {measure}
                    </span>
                )}
            </div>
            {feature.note !== '' && <p style={styles.featureNote}>{feature.note}</p>}
            {feature.properties.length > 0 && (
                <dl style={styles.properties}>
                    {feature.properties.map((property) => (
                        <React.Fragment key={property.key}>
                            <dt style={styles.term}>{property.key}</dt>
                            <dd style={styles.value}>{property.value}</dd>
                        </React.Fragment>
                    ))}
                </dl>
            )}
        </li>
    );
};

export const GeoJsonCard: React.FC<Props> = ({payload}) => {
    const summary = summaryLine(payload);

    return (
        <div>
            {payload.lead !== '' && <span style={styles.text}>{payload.lead}</span>}
            <div
                style={styles.card}
                data-testid='geojson-card'
            >
                <p style={styles.kind}>{CARD_KIND}</p>
                <div style={styles.header}>
                    <span
                        style={styles.heading}
                        data-testid='geojson-heading'
                    >
                        {plural(payload.counts.features, 'feature')}
                    </span>
                    {summary !== '' && (
                        <span
                            style={styles.summary}
                            data-testid='geojson-summary'
                        >
                            {summary}
                        </span>
                    )}
                </div>

                {payload.note !== '' && (
                    <p
                        style={styles.note}
                        data-testid='geojson-note'
                    >
                        {payload.note}
                    </p>
                )}

                {payload.propertiesDropped && (
                    <p
                        style={styles.note}
                        data-testid='geojson-degraded'
                    >
                        {'The properties this document carried were omitted to fit the size limit. Everything else is unchanged, and the document is still readable under "Open details".'}
                    </p>
                )}

                <ErrorBoundary fallback={<p style={styles.note}>{DETAIL_FAILED}</p>}>
                    <GeoJsonMap
                        payload={payload}
                        surface='card'
                    />
                </ErrorBoundary>

                <ErrorBoundary fallback={<p style={styles.note}>{DETAIL_FAILED}</p>}>
                    {payload.features.length > 0 && (
                        <ul
                            style={styles.list}
                            data-testid='geojson-features'
                        >
                            {payload.features.map((feature, index) => (
                                <Feature
                                    key={`${feature.name}-${index}`}
                                    feature={feature}
                                />
                            ))}
                        </ul>
                    )}
                </ErrorBoundary>

                <div style={styles.actions}>
                    <button
                        type='button'
                        style={styles.button}
                        onClick={() => showGeoJsonDocument(payload)}
                    >
                        {'Open details'}
                    </button>
                </div>
            </div>
            {payload.trail !== '' && <span style={styles.text}>{payload.trail}</span>}
        </div>
    );
};

export default GeoJsonCard;
