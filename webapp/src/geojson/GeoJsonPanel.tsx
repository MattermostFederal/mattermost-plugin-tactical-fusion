import React, {useEffect, useLayoutEffect, useRef} from 'react';

import Customize from './Customize';
import {setEditing, useEditing} from './editing';
import {measureLine, shapeLine, summaryLine} from './GeoJsonCard';
import GeoJsonMap from './GeoJsonMap';
import {isSectionVisible, sectionLabel} from './sections';
import type {GeoJsonFeature, GeoJsonPayload} from './types';
import {isLinkable, solePosition} from './types';

import ErrorBoundary from '../components/ErrorBoundary';
import LinkButton from '../components/LinkButton';
import Disclosure from '../cot/Disclosure';
import HoverLink from '../decorators/HoverLink';
import CopyButton from '../decorators/location/CopyButton';
import {docsUrl, pluginBaseUrl} from '../plugin_url';
import {usePreferences} from '../preferences/store';

export const PANEL_TITLE = 'GeoJSON';

/**
 * The sidebar header while the editor has the panel.
 *
 * Matches the link that opens it, so the reader is never told two different
 * names for where they are.
 */
export const EDITOR_TITLE = 'Customize your view';

export const SECTION_FAILED = 'This document could not be rendered.';

const styles: Record<string, React.CSSProperties> = {
    heading: {margin: '0 0 4px', fontSize: '16px', fontWeight: 600},
    subhead: {margin: '0 0 12px', opacity: 0.85, fontSize: '13px'},
    section: {margin: '16px 0 4px', fontSize: '12px', textTransform: 'uppercase', letterSpacing: '0.04em', opacity: 0.85},
    note: {margin: '0 0 8px', opacity: 0.9, fontSize: '13px'},
    list: {listStyle: 'none', margin: 0, padding: 0},
    item: {
        borderTop: '1px solid rgba(var(--center-channel-color-rgb), 0.08)',
        padding: '8px 0',
    },
    featureHead: {alignItems: 'baseline', display: 'flex', flexWrap: 'wrap', gap: '0.5em'},
    name: {fontWeight: 600},
    kind: {fontFamily: 'monospace', fontSize: '0.85em', opacity: 0.9},
    shape: {opacity: 0.85, fontSize: '0.9em'},
    measure: {fontWeight: 600, fontSize: '0.9em'},
    featureNote: {margin: '2px 0 0', opacity: 0.9, fontSize: '0.9em'},
    rows: {display: 'grid', gridTemplateColumns: 'max-content 1fr', gap: '2px 12px', margin: '6px 0 0', fontSize: '0.9em'},
    term: {opacity: 0.85},
    value: {margin: 0, wordBreak: 'break-word'},
    source: {
        fontFamily: 'monospace',
        fontSize: '0.85em',
        margin: 0,
        maxHeight: 280,
        overflow: 'auto',
        whiteSpace: 'pre-wrap',
        wordBreak: 'break-word',
    },
    footer: {
        borderTop: '1px solid rgba(var(--center-channel-color-rgb), 0.16)',
        fontSize: '13px',
        marginTop: '20px',
        paddingTop: '12px',
    },
};

/**
 * A lone point's position, linked into the coordinate tools where the server
 * gave it an identity to link with.
 *
 * `isLinkable` rather than a test on the coordinates: the server writes the
 * pair only when the location grammar will stand behind the position, so a
 * coarse coordinate and a post stamped before the pair existed both fall to
 * plain text rather than to a link that would land nowhere.
 */
function Position({feature}: {feature: GeoJsonFeature}) {
    const position = solePosition(feature);
    if (position === null) {
        return null;
    }

    const reading = `${position.lat}, ${position.lon}`;
    if (!isLinkable(feature)) {
        return <span style={styles.shape}>{reading}</span>;
    }

    const params = new URLSearchParams({f: feature.format, v: feature.value});
    return (
        <HoverLink href={`${pluginBaseUrl()}/decorate/location?${params.toString()}`}>
            {reading}
        </HoverLink>
    );
}

const Feature: React.FC<{feature: GeoJsonFeature; showProperties: boolean}> = ({
    feature, showProperties,
}) => {
    const shape = shapeLine(feature);
    const measure = measureLine(feature);

    return (
        <li style={styles.item}>
            <div style={styles.featureHead}>
                <span style={styles.name}>{feature.name}</span>
                <span style={styles.kind}>{feature.kind === 'none' ? 'no geometry' : feature.kind}</span>
                <Position feature={feature}/>
                {shape !== '' && <span style={styles.shape}>{shape}</span>}
                {measure !== '' && <span style={styles.measure}>{measure}</span>}
            </div>
            {feature.note !== '' && <p style={styles.featureNote}>{feature.note}</p>}
            {showProperties && feature.properties.length > 0 && (
                <dl style={styles.rows}>
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

export const GeoJsonPanel: React.FC<{payload: GeoJsonPayload}> = ({payload}) => {
    const {preferences} = usePreferences();
    const customizing = useEditing();

    // Returning from the editor unmounts it, so focus would fall to the body.
    // Put the reader back on the control they opened it from.
    const customizeRef = useRef<HTMLButtonElement>(null);
    const wasCustomizing = useRef(customizing);
    useEffect(() => {
        if (wasCustomizing.current && !customizing) {
            customizeRef.current?.focus();
        }
        wasCustomizing.current = customizing;
    }, [customizing]);

    // Keyed on the payload itself rather than on anything derived from it, so
    // it changes exactly once per selection and never on a re-render. Before
    // paint, so the editor is never flashed on screen carrying a document the
    // reader did not open it from.
    useLayoutEffect(() => {
        setEditing(false);
    }, [payload]);

    if (customizing) {
        return <Customize onClose={() => setEditing(false)}/>;
    }

    const hidden = preferences.geojson.hiddenSections;
    const summary = summaryLine(payload);

    return (
        <div>
            <ErrorBoundary fallback={<p style={styles.subhead}>{SECTION_FAILED}</p>}>
                {isSectionVisible(hidden, 'summary') && (
                    <div data-testid='geojson-panel-summary'>
                        <p style={styles.heading}>
                            {`${payload.counts.features} feature${payload.counts.features === 1 ? '' : 's'}`}
                        </p>
                        {summary !== '' && <p style={styles.subhead}>{summary}</p>}
                        {payload.note !== '' && <p style={styles.note}>{payload.note}</p>}
                        {payload.propertiesDropped && (
                            <p style={styles.note}>
                                {'The properties this document carried were omitted to fit the size limit.'}
                            </p>
                        )}
                    </div>
                )}

                {/*
                  * The map gets its OWN boundary, with no fallback, so a throw
                  * inside it takes the map and nothing else. Sharing the
                  * panel's boundary meant a map failure replaced the summary,
                  * the feature list and the source dump with one sentence,
                  * against the contract the map itself states: every failure
                  * hides the map and leaves every reading on screen. CotPanel
                  * and GeoJsonCard already do it this way.
                  */}
                {isSectionVisible(hidden, 'map') && (
                    <ErrorBoundary>
                        <GeoJsonMap
                            payload={payload}
                            surface='panel'
                        />
                    </ErrorBoundary>
                )}

                {isSectionVisible(hidden, 'features') && payload.features.length > 0 && (
                    <>
                        <p style={styles.section}>{sectionLabel('features')}</p>
                        <ul
                            style={styles.list}
                            data-testid='geojson-panel-features'
                        >
                            {payload.features.map((feature, index) => (
                                <Feature
                                    key={`${feature.name}-${index}`}
                                    feature={feature}
                                    showProperties={isSectionVisible(hidden, 'properties')}
                                />
                            ))}
                        </ul>
                    </>
                )}

                {isSectionVisible(hidden, 'source') && (
                    <Disclosure
                        label={sectionLabel('source')}
                        trailing={
                            <CopyButton
                                label='Copy the document as posted'
                                value={payload.src}
                            />
                        }
                    >
                        <pre
                            style={styles.source}
                            tabIndex={0}
                            role='region'
                            aria-label='The document as it was posted'
                            data-testid='geojson-panel-source'
                        >{payload.src}</pre>
                    </Disclosure>
                )}
            </ErrorBoundary>

            <p style={styles.footer}>
                <LinkButton
                    ref={customizeRef}
                    onClick={() => setEditing(true)}
                >{'Customize your view'}</LinkButton>
                <span aria-hidden={true}>{' · '}</span>
                <LinkButton href={docsUrl()}>{'Documentation'}</LinkButton>
            </p>
        </div>
    );
};

export const GeoJsonTitle: React.FC<{payload: GeoJsonPayload}> = ({payload}) => {
    const customizing = useEditing();

    // The editor takes the panel over, so a header still naming the document
    // would be describing something no longer on screen.
    if (customizing) {
        return <span>{EDITOR_TITLE}</span>;
    }

    const {features} = payload.counts;
    return <span>{`${PANEL_TITLE}: ${features} feature${features === 1 ? '' : 's'}`}</span>;
};

export default GeoJsonPanel;
