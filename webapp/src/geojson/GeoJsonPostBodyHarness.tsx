import React from 'react';

import GeoJsonPostBody from './GeoJsonPostBody';
import type {GeoJsonCounts, GeoJsonFeature} from './types';
import {GEOJSON_PROPS_KEY, GEOJSON_PROPS_VERSION} from './types';

interface Props {

    /** Null for a post carrying no blob at all, which is the props-loss case. */
    features?: Array<Partial<GeoJsonFeature>> | null;

    version?: number;
    source?: string;
    lead?: string;
    trail?: string;
    src?: string;
    note?: string;
    unplaceable?: boolean;

    /** The post the card was read from, which is what "Open larger" addresses. */
    postId?: string;
    propertiesDropped?: boolean;
    counts?: Partial<GeoJsonCounts>;
    fileId?: string;
    fileName?: string;

    message?: string;
    editAt?: number;
    fileIds?: string[];
    compactDisplay?: boolean;
}

const point = {
    kind: 'Point',
    rings: [[{lon: '-118.25', lat: '34.0561', alt: ''}]],
};

/**
 * Harness for the GeoJSON post body.
 *
 * The map is not reachable here: Phase 1a renders no map at all, so these tests
 * stay free of the feature store, the preference store and WebGL.
 */
const GeoJsonPostBodyHarness: React.FC<Props> = ({
    features = [{name: 'Depot', kind: 'Point'}],
    version = GEOJSON_PROPS_VERSION,
    source = 'fence',
    lead = '',
    trail = '',
    src = '{"type":"Point","coordinates":[-118.25,34.0561]}',
    note = '',
    unplaceable = false,
    postId = 'post0000000000000000000000',
    propertiesDropped = false,
    counts = {},
    fileId = '',
    fileName = '',
    message = '',
    editAt = 0,
    fileIds = [],
    compactDisplay,
}) => {
    let postProps: Record<string, unknown> | undefined;
    if (features !== null) {
        const blob: Record<string, unknown> = {
            version,
            source,
            lead,
            trail,
            src,
            note,
            ...(unplaceable ? {unplaceable: '1'} : {}),
            file_id: fileId,
            file_name: fileName,
            counts: {
                features: features.length,
                points: features.length,
                lines: 0,
                polygons: 0,
                collections: 0,
                unlocated: 0,
                undrawable: 0,
                ...counts,
            },
            features: features.map((feature, index) => ({
                name: feature.name ?? `Feature ${index + 1}`,
                kind: feature.kind ?? 'Point',
                note: feature.note ?? '',
                ...(feature.format === undefined ? {} : {format: feature.format}),
                ...(feature.value === undefined ? {} : {value: feature.value}),
                ...(feature.length === undefined ? {} : {length: feature.length}),
                ...(feature.area === undefined ? {} : {area: feature.area}),
                ...(feature.color === undefined ? {} : {color: feature.color}),
                ...(feature.width === undefined ? {} : {width: feature.width}),
                ...(feature.lineOpacity === undefined ? {} : {line_opacity: feature.lineOpacity}),
                ...(feature.fillOpacity === undefined ? {} : {fill_opacity: feature.fillOpacity}),
                ...(feature.markerSize === undefined ? {} : {marker_size: feature.markerSize}),
                parts: feature.parts ?? [point],
                ...(feature.properties === undefined ? {} : {properties: feature.properties}),
            })),
        };

        if (propertiesDropped) {
            blob.properties_dropped = '1';
        }

        postProps = {[GEOJSON_PROPS_KEY]: blob};
    }

    return (
        <div data-testid='geojson-body'>
            <GeoJsonPostBody
                post={{
                    id: postId,
                    message,
                    props: postProps,
                    edit_at: editAt,
                    file_ids: fileIds,
                }}
                compactDisplay={compactDisplay}
            />
        </div>
    );
};

export default GeoJsonPostBodyHarness;
