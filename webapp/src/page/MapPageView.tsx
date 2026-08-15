import React from 'react';

import type {PageData} from './payload';

import {decimalText, cellDegrees} from '../decorators/location/format';
import LocationMap from '../decorators/location/map/LocationMap';
import {pluginBaseUrl} from '../plugin_url';

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
    token: {fontFamily: 'ui-monospace, SFMono-Regular, Menlo, monospace', fontWeight: 600},
    muted: {color: 'rgba(var(--center-channel-color-rgb), 0.64)'},
    spacer: {marginLeft: 'auto'},
    link: {color: 'var(--link-color)'},
};

/**
 * The map page: the whole window given to one picture.
 *
 * A route of its own rather than a mode of the readings page, which opens onto
 * a table. What it adds beneath the map is the least that keeps the picture
 * honest: which token this is, where it is in words, and the way back to
 * everything else.
 */
const MapPageView: React.FC<{data: PageData}> = ({data}) => {
    const {payload, conversion} = data;
    const {coord, format, canonical, raw} = payload;

    const position = conversion.status === 'ready' ? conversion.data : null;
    const lat = coord ? coord.lat.decimal : (position?.lat ?? null);
    const lon = coord ? coord.lon.decimal : (position?.lon ?? null);
    const [cellLat, cellLon] = cellDegrees(coord, format, canonical, lat);

    const where = coord ? decimalText(coord) : (position?.decimal ?? '');
    const region = position?.region ?? '';

    const params = new URLSearchParams({f: format, v: canonical});
    if (raw !== '' && raw !== canonical) {
        params.set('r', raw);
    }

    return (
        <div style={styles.root}>
            <div style={styles.map}>
                <LocationMap
                    lat={lat}
                    lon={lon}
                    cellDegLat={cellLat}
                    cellDegLon={cellLon}
                    region={region}
                    pending={false}
                    fill={true}
                />
            </div>
            <div style={styles.bar}>
                <span style={styles.token}>{canonical}</span>
                {where !== '' && <span style={styles.muted}>{where}</span>}
                {region !== '' && <span style={styles.muted}>{region}</span>}
                <span style={styles.spacer}/>
                <span style={styles.muted}>{'Natural Earth 110m'}</span>
                <a
                    style={styles.link}
                    href={`${pluginBaseUrl()}/decorate/location?${params.toString()}`}
                >{'All readings'}</a>
            </div>
        </div>
    );
};

export default MapPageView;
