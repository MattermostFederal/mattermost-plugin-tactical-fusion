import {pluginBaseUrl} from '../../../plugin_url';
import {withTheme} from '../../theme';
import type {ConversionState} from '../convert';
import {cellDegrees} from '../format';
import type {LocationPayload} from '../index';

/**
 * Where a coordinate is drawn, and how big its cell is.
 *
 * One function rather than the same four lines copied into the panel, the hover
 * card and the map page. Those three copies agreed, and the hover's carried a
 * comment saying it had to: "a hover and the panel behind it disagreeing about
 * where a coordinate is would be the worst possible place for the two to
 * drift". An invariant a comment asks three files to keep is one this codebase
 * turns into code everywhere else, and this is the value where drifting costs
 * most.
 *
 * It lives in its own module rather than in `LocationMap.tsx` beside its one
 * consumer, because putting it there made the component import `../index` for
 * `LocationPayload`, closing a cycle: index -> LocationHover -> LocationMap ->
 * index. Type-only imports are erased, so nothing broke, but there is no
 * `import/no-cycle` rule configured to catch the day somebody needs a value
 * from `../index` here. The failure mode of that cycle is a module-scope
 * binding reading `undefined` in some evaluation orders, which is silent.
 *
 * `data` is read only once the status is `ready`, which is what keeps a failed
 * or refused conversion from putting a pin at a position nothing vouched for.
 */
export interface View {
    lat: number | null;
    lon: number | null;
    cellDegLat: number;
    cellDegLon: number;
}

export function viewFor(
    payload: LocationPayload, conversion: ConversionState): View & {region: string} {
    const {coord, format, canonical} = payload;
    const position = conversion.status === 'ready' ? conversion.data : null;

    const lat = coord ? coord.lat.decimal : (position?.lat ?? null);
    const lon = coord ? coord.lon.decimal : (position?.lon ?? null);
    const [cellDegLat, cellDegLon] = cellDegrees(coord, format, canonical, lat);

    return {lat, lon, cellDegLat, cellDegLon, region: position?.region ?? ''};
}

/**
 * Where "Open larger" goes, for every surface that offers it.
 *
 * `/map` is outside `/decorate`, so the framework's click handler does not
 * recognise it and the browser simply follows it. Which is exactly why the
 * theme is written here rather than at click: the handler is what puts `_theme`
 * on every other page link and it stands aside for this one, so a map page
 * opened from a light sidebar on a dark laptop came up dark, palette and all.
 * Read at render, one step less live than the handler, and in exchange it
 * survives a middle-click.
 */
export function mapPageHref(payload: LocationPayload): string {
    const {format, canonical, raw} = payload;

    const params = new URLSearchParams({f: format, v: canonical});
    if (raw !== '' && raw !== canonical) {
        params.set('r', raw);
    }

    return withTheme(`${pluginBaseUrl()}/map?${params.toString()}`);
}
