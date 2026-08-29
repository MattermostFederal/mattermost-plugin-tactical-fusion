import React from 'react';

import type {LocationPageData} from './payload';

import {vouchedText} from '../decorators/location/convert';
import LocationMap from '../decorators/location/map/LocationMap';
import {viewFor} from '../decorators/location/map/view';
import {withTheme} from '../decorators/theme';
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
    spacer: {marginLeft: 'auto'},
    link: {color: 'var(--link-color)'},
};

/**
 * The map page: the whole window given to one picture.
 *
 * A route of its own rather than a mode of the readings page, which opens onto
 * a table. Beneath the map is the least that keeps the picture honest: the
 * author's own text, and the way back to everything else.
 *
 * The bar carried the canonical token, the position in decimal degrees, the
 * region and the basemap credit as well. All four are readings, and the page
 * one link away is nothing but readings, rendered with their labels beside
 * them; four of them crammed unlabeled under a picture is a worse table, not a
 * summary. What is left is the one line that says which text this map was drawn
 * for. The credit goes with them: LocationMap suppresses its own caption when
 * filling, so this page now carries none, and Natural Earth is public domain.
 */
const MapPageView: React.FC<{data: LocationPageData}> = ({data}) => {
    const {payload, conversion} = data;
    const {format, canonical, raw} = payload;

    // Through the same viewFor the panel and the hover use. `region` is not
    // shown, but the map puts it in its accessible label, which is the only
    // place a reader without the picture learns the country.
    const view = viewFor(payload, conversion);

    // The same value the readings table leads with, through the same helper, so
    // the two surfaces cannot come to disagree about which text is the author's.
    const written = vouchedText(conversion, raw, canonical);

    const params = new URLSearchParams({f: format, v: canonical});
    if (raw !== '' && raw !== canonical) {
        params.set('r', raw);
    }

    // The theme this page was painted with, carried back the way it came. The
    // click handler that themes every /decorate link runs in the webapp bundle,
    // and a page has no webapp around it, so the link states it itself.
    const back = withTheme(`${pluginBaseUrl()}/decorate/location?${params.toString()}`);

    return (
        <div style={styles.root}>
            <div style={styles.map}>
                <LocationMap
                    {...view}
                    pending={false}
                    fill={true}
                />
            </div>
            <div style={styles.bar}>
                <span style={styles.token}>{written}</span>
                <span style={styles.spacer}/>
                <a
                    style={styles.link}
                    href={back}
                >{'All readings'}</a>
            </div>
        </div>
    );
};

export default MapPageView;
