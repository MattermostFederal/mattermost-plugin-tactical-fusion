import React, {useRef, useState} from 'react';

import GeoJsonMap, {focusFor} from './GeoJsonMap';
import {showGeoJsonDocument} from './panel';
import type {GeoJsonFeature, GeoJsonPayload, GeoJsonProperty} from './types';
import {ringCount, vertexCount} from './types';

import ErrorBoundary from '../components/ErrorBoundary';
import type {MapFocus} from '../decorators/location/map/focus';

interface Props {
    payload: GeoJsonPayload;
    compactDisplay?: boolean;
}

export const CARD_KIND = 'GeoJSON';

export const STYLE_KEYS = new Set([
    'marker-color', 'marker-size', 'marker-symbol',
    'stroke', 'stroke-width', 'stroke-opacity',
    'fill', 'fill-opacity',
]);

const NAME_KEYS = new Set(['name', 'title', 'label']);

const DESCRIPTION_KEY = 'description';

export function descriptionOf(feature: GeoJsonFeature): string {
    return feature.properties.find((property) => property.key === DESCRIPTION_KEY)?.value ?? '';
}

export function shownProperties(feature: GeoJsonFeature): GeoJsonProperty[] {
    return feature.properties.filter((property) => {
        if (STYLE_KEYS.has(property.key) || property.key === DESCRIPTION_KEY) {
            return false;
        }
        return !(NAME_KEYS.has(property.key) && property.value === feature.name);
    });
}

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
    description: {opacity: 0.9, margin: 0, padding: '0 12px 8px', whiteSpace: 'pre-wrap'},
    note: {opacity: 0.9, padding: '0 12px 8px'},
    dot: {borderRadius: '50%', display: 'inline-block', height: 10, width: 10, flex: 'none', alignSelf: 'center'},
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
    featureButton: {background: 'none', border: 'none', color: 'inherit', cursor: 'pointer', font: 'inherit', padding: 0, textAlign: 'left'},
    name: {fontWeight: 600},
    featureNote: {opacity: 0.9, fontSize: '0.9em', margin: '2px 0 0'},
    featureDescription: {opacity: 0.9, fontSize: '0.9em', margin: '2px 0 0', whiteSpace: 'pre-wrap'},
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

function plural(n: number, word: string): string {
    return `${n} ${word}${n === 1 ? '' : 's'}`;
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

const Dot: React.FC<{color: string}> = ({color}) => {
    if (color === '') {
        return null;
    }

    return (
        <span
            aria-hidden={true}
            style={{...styles.dot, background: color}}
            data-testid='geojson-dot'
        />
    );
};

const Feature: React.FC<{feature: GeoJsonFeature; onShow?: () => void}> = ({feature, onShow}) => {
    const description = descriptionOf(feature);
    const head = (
        <>
            <Dot color={feature.color}/>
            <span style={styles.name}>{feature.name}</span>
        </>
    );

    return (
        <li style={styles.listItem}>
            {onShow === undefined ? (
                <div style={styles.featureHead}>{head}</div>
            ) : (
                <button
                    type='button'
                    style={{...styles.featureHead, ...styles.featureButton}}
                    onClick={onShow}
                    aria-label={`Show ${feature.name} on the map`}
                    data-testid='geojson-feature-show'
                >
                    {head}
                </button>
            )}
            {description !== '' && (
                <p
                    style={styles.featureDescription}
                    data-testid='geojson-feature-description'
                >
                    {description}
                </p>
            )}
            {feature.note !== '' && <p style={styles.featureNote}>{feature.note}</p>}
        </li>
    );
};

export const GeoJsonCard: React.FC<Props> = ({payload}) => {
    const [focus, setFocus] = useState<MapFocus | undefined>(undefined);
    const clicks = useRef(0);

    const show = (feature: GeoJsonFeature) => {
        clicks.current += 1;
        const next = focusFor(feature, clicks.current);
        if (next !== null) {
            setFocus(next);
        }
    };

    return (
        <div>
            {payload.lead !== '' && <span style={styles.text}>{payload.lead}</span>}
            <div
                style={styles.card}
                data-testid='geojson-card'
            >
                <p
                    style={styles.kind}
                    data-testid='geojson-kind'
                >
                    {payload.name === '' ? CARD_KIND : `${CARD_KIND}: ${payload.name}`}
                </p>
                {payload.name === '' && payload.fileName !== '' && (
                    <div style={styles.header}>
                        <span
                            style={styles.heading}
                            data-testid='geojson-heading'
                        >
                            {payload.fileName}
                        </span>
                    </div>
                )}
                {payload.description !== '' && (
                    <p
                        style={styles.description}
                        data-testid='geojson-description'
                    >
                        {payload.description}
                    </p>
                )}

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
                        focus={focus}
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
                                    onShow={focusFor(feature, 0) === null ? undefined : () => show(feature)}
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
