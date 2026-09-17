import type {GeoJSONSource, Map as MapLibreMap} from 'maplibre-gl';
import React, {useEffect, useRef, useState} from 'react';

import {OverlayPageView} from './OverlayPageView';
import type {OverlayPageData} from './payload';

import {_setMapObserverForTesting} from '../decorators/location/map/use_map_instance';

function ringsIn(map: MapLibreMap | null, id: string): string {
    const source = map?.getSource<GeoJSONSource>(id);
    if (!source) {
        return 'none';
    }

    const data = (source.serialize() as {
        data?: {features?: Array<{geometry?: {type?: string; coordinates?: unknown[]}}>};
    }).data;

    return (data?.features ?? []).map((feature) => {
        const type = feature.geometry?.type ?? '?';
        const rings = type === 'Polygon' ? (feature.geometry?.coordinates ?? []).length : 1;
        return `${type}:${rings}`;
    }).join('|');
}

const OverlayPageHarness: React.FC<{data: OverlayPageData}> = ({data}) => {
    const last = useRef<MapLibreMap | null>(null);
    const [reading, setReading] = useState({geometry: 'unread', pin: 'unread'});

    useEffect(() => {
        _setMapObserverForTesting((instance) => {
            if (instance) {
                last.current = instance;
            }
        });

        return () => _setMapObserverForTesting(null);
    }, []);

    return (
        <div>
            <OverlayPageView data={data}/>
            <button
                type='button'
                onClick={() => setReading({
                    geometry: ringsIn(last.current, 'geometry'),
                    pin: ringsIn(last.current, 'pin'),
                })}
            >{'read the map'}</button>
            <output data-testid='drawn-geometry'>{reading.geometry}</output>
            <output data-testid='drawn-pins'>{reading.pin}</output>
        </div>
    );
};

export default OverlayPageHarness;
