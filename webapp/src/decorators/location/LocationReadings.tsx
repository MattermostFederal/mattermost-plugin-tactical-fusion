import React from 'react';

import {vouchedText} from './convert';
import type {ConversionState} from './convert';
import CopyButton from './CopyButton';
import {
    confidenceText,
    ddmText,
    decimalText,
    dmsText,
    gridText,
    remoteResolutionText,
    resolutionText,
    usmtfText,
} from './format';
import type {LocationFormat} from './format';
import LocationMap from './map/LocationMap';
import {mapPageHref, viewFor} from './map/view';
import {MAP_ID, isRowVisible, ROWS} from './rows';
import type {HideableID, RowID} from './rows';

import LinkButton from '../../components/LinkButton';
import type {Features} from '../../features/types';
import {docsUrl} from '../../plugin_url';

import type {LocationPayload} from './index';

const styles: Record<string, React.CSSProperties> = {

    // The refusal below, which is the only thing left above the table. There
    // used to be a lead line here repeating the grid reference, three lines
    // above the row that already carried it with a label and a copy button.
    verdict: {
        fontSize: '18px',
        color: 'var(--center-channel-color)',
        margin: '0 0 16px',
    },
    table: {width: '100%', borderCollapse: 'collapse', fontSize: '13px'},
    th: {
        textAlign: 'left',
        fontSize: '11px',
        textTransform: 'uppercase',
        letterSpacing: '0.04em',
        fontWeight: 600,
        opacity: 0.7,
        color: 'var(--center-channel-color)',
        padding: '8px 10px 8px 0',
        verticalAlign: 'top',
        whiteSpace: 'nowrap',
        width: '38%',
        borderBottom: '1px solid rgba(var(--center-channel-color-rgb), 0.08)',
    },
    td: {
        padding: '8px 0',
        color: 'var(--center-channel-color)',
        fontFamily: 'ui-monospace, SFMono-Regular, Menlo, monospace',
        wordBreak: 'break-word',
        borderBottom: '1px solid rgba(var(--center-channel-color-rgb), 0.08)',
    },
    plain: {fontFamily: 'inherit'},
    pending: {fontFamily: 'inherit', opacity: 0.6},
    copyCell: {
        width: '24px',
        padding: '6px 0 6px 8px',
        verticalAlign: 'top',
        textAlign: 'right',
        borderBottom: '1px solid rgba(var(--center-channel-color-rgb), 0.08)',
    },
    note: {
        fontSize: '12px',
        color: 'var(--center-channel-color)',
        opacity: 0.6,
        margin: '14px 0 0',
    },
    footer: {marginTop: '14px', fontSize: '12px'},
};

/**
 * What a row shows while its value is still in flight, and what it shows when
 * the request failed.
 *
 * Never blank and never a zero. A coordinate row reading "0.0000 N, 0.0000 E"
 * because a conversion failed would be a position, and a wrong one; saying so
 * in words cannot be mistaken for data.
 */
export const PENDING = 'converting…';
export const UNAVAILABLE = 'unavailable';

/**
 * Whether a rendered value is a coordinate rather than a placeholder.
 *
 * Both placeholders are prose and neither is a legal value in any of these
 * notations, so this cannot mistake one for the other. It gates the copy
 * buttons, which otherwise cheerfully put "converting…" on the clipboard.
 */
function isValue(value: string): boolean {
    return value !== '' && value !== PENDING && value !== UNAVAILABLE;
}

const Row: React.FC<{
    label: string;
    value: string;
    plain?: boolean;

    /**
     * Whether this row's value is worth copying.
     *
     * False for the rows that are prose rather than a value: a reader pasting
     * "about 11 m" or "WGS 84" into another system has copied a sentence, not a
     * position. The icon is also absent while the value is a placeholder, since
     * copying "converting…" is worse than having nothing to click.
     */
    copyable?: boolean;
}> = ({label, value, plain, copyable}) => {
    let style = styles.td;
    if (!isValue(value)) {
        style = {...styles.td, ...styles.pending};
    } else if (plain) {
        style = {...styles.td, ...styles.plain};
    }

    return (
        <tr>
            <th
                scope='row'
                style={styles.th}
            >{label}</th>
            <td style={style}>{value}</td>
            <td style={styles.copyCell}>
                {copyable && isValue(value) && (
                    <CopyButton
                        label={`Copy ${label}`}
                        value={value}
                    />
                )}
            </td>
        </tr>
    );
};

/**
 * The map and the readings table, with no opinion about where its data came
 * from.
 *
 * Split out of LocationPanel so the sidebar and the standalone pages render one
 * table rather than two that agree by inspection. Everything environmental
 * stays with the caller: the panel fetches its conversion and reads the
 * reader's hidden rows, the pages are handed both, and neither can reach the
 * other's assumptions from in here.
 *
 * Values are split by where they can be worked out rather than by format.
 * Anything the canonical token yields by slicing and dividing is computed here
 * and is on screen immediately; anything needing the ellipsoid comes from the
 * conversion, because the projection lives in Go and writing a second one in
 * TypeScript would be two implementations of the same geodesy.
 *
 * `raw` is the one value here that originated as message text. It is rendered
 * as a React text node and nothing here uses `dangerouslySetInnerHTML`.
 */
const LocationReadings: React.FC<{
    payload: LocationPayload;
    conversion: ConversionState;
    hidden: HideableID[];

    /**
     * Which map surfaces the admin has left on.
     *
     * Handed in rather than read here, for the same reason `hidden` is: the
     * panel learns it from /api/v1/features and a page reads it out of the
     * shell the server wrote, and neither route is available to the other.
     */
    maps: Features;
    footer?: React.ReactNode;
}> = ({payload, conversion, hidden, maps, footer}) => {
    const {coord, format, canonical, raw} = payload;

    // The server looked at this link and said it is not one this plugin wrote.
    //
    // Refusing to render it is the point rather than a failure state. Two of
    // the four checks on the author's text need the token grammar, which lives
    // in Go, so before the conversion carried "r" a hand-written link could put
    // any short run of letters and digits in the "As written" row next to a
    // position derived from a different token, with a copy button beside it,
    // while the server-rendered page refused the identical link.
    //
    // Distinguished from a request that simply did not arrive, which degrades
    // instead: an offline reader keeps every row the token yields locally.
    if (conversion.status === 'rejected') {
        return (
            <div>
                <p style={styles.verdict}>{'Not a coordinate'}</p>
                <p style={styles.note}>
                    {'This link does not carry a coordinate this plugin issued, so nothing here ' +
                        'can be trusted to name a place. It was most likely edited by hand.'}
                </p>
                <div style={styles.footer}>
                    <LinkButton href={docsUrl()}>{'Documentation'}</LinkButton>
                </div>
            </div>
        );
    }

    // A value the server owns: what it sent, or a placeholder saying which of
    // the two reasons it is not here yet.
    //
    // An empty string counts as absent rather than as an answer. The server
    // sends one for a position outside the grid, which is the polar regions,
    // and "unavailable" is the honest reading of that too: past 84 north the
    // notation is a different system this plugin does not implement.
    const remote = (value: string | undefined): string => {
        if (value) {
            return value;
        }
        return conversion.status === 'loading' ? PENDING : UNAVAILABLE;
    };

    // Local where possible. A grid token already is its own MGRS or UTM row, so
    // that row never waits on anything and never degrades.
    //
    // The `||` matters on the pages. gridText returns "" when the token does not
    // match this side's copy of the canonical shapes, which is the same
    // condition that makes fromParams fall back, so without it the one row the
    // reader opened the link for would disappear under grammar drift while the
    // server's own answer sat unused in the conversion.
    const mgrs = format === 'mgrs' ? (gridText(format, canonical) || remote(conversion.data?.mgrs)) : remote(conversion.data?.mgrs);
    const utm = format === 'utm' ? (gridText(format, canonical) || remote(conversion.data?.utm)) : remote(conversion.data?.utm);

    // A token that is itself an area reference already carries its own code, so
    // that row never waits on the request and never degrades, exactly as the
    // grid rows above do not.
    const own = (id: LocationFormat, value: string | undefined): string =>
        (format === id ? canonical : remote(value));

    const decimal = coord ? decimalText(coord) : remote(conversion.data?.decimal);
    const dms = coord ? dmsText(coord) : remote(conversion.data?.dms);
    const ddm = coord ? ddmText(coord) : remote(conversion.data?.ddm);
    const usmtf = coord ? usmtfText(coord) : remote(conversion.data?.usmtf);

    const resolution = coord ? resolutionText(coord) : remoteResolutionText(format, canonical);
    const confidence = coord ? confidenceText(coord) : '';

    const vouched = vouchedText(conversion, raw, canonical);

    // Every row's value, by id, so the table can be rendered from the shared
    // ROWS catalog rather than from eleven hand-written entries. That catalog
    // is what the reader's hidden-row setting names, so a row that existed here
    // and nowhere else would be a row nobody could hide.
    //
    // An empty value is a row that does not apply and is left out, which is how
    // the conditional rows express themselves: Confidence unless the token
    // stated one, and Normalized unless it says something the row above did
    // not.
    const values: Record<RowID, string> = {
        mgrs,
        decimal,
        dms,
        ddm,
        usmtf,
        utm,
        georef: own('georef', conversion.data?.georef),
        gars: own('gars', conversion.data?.gars),
        pluscode: own('pluscode', conversion.data?.pluscode),
        resolution,
        confidence,
        datum: 'WGS 84',
        raw: vouched,
        canonical: canonical === vouched ? '' : canonical,
    };

    // "Open larger" points at the map's own page, which gives the whole window
    // to one picture. Not the readings page, which opens onto a table, and not a
    // decorator link either: this route is outside /decorate, so the framework's
    // click handler does not recognise it and the browser simply follows it.
    //
    // The theme travels with it, for the reason mapPageHref records.
    //
    // Undefined when the admin has turned that page off, since /map answers 404
    // then and a link to it is a link to nothing. LocationMap renders no caption
    // at all without one, so the strip under the frame goes with it.
    const pageHref = maps.mapPage ? mapPageHref(payload) : undefined;

    // Through the same viewFor the hover card and the map page use, so the
    // three surfaces cannot come to disagree about where a coordinate is.
    const view = viewFor(payload, conversion);

    // No lead line. The table already opens with the grid reference, labeled
    // and with a copy button beside it, so a bare repeat of the same string
    // three lines above it was saying nothing twice.
    return (
        <div>
            {maps.mapPanel && isRowVisible(hidden, MAP_ID) && (
                <LocationMap
                    {...view}
                    pageHref={pageHref}
                    pending={conversion.status === 'loading'}
                />
            )}
            <table
                style={styles.table}
                aria-label='Readings for this coordinate'
            >
                <tbody>
                    {ROWS.filter((row) => isRowVisible(hidden, row.id) && values[row.id] !== '').map((row) => (
                        <Row
                            key={row.id}
                            label={row.label}
                            value={values[row.id]}
                            plain={!row.copyable}
                            copyable={row.copyable}
                        />
                    ))}
                </tbody>
            </table>

            <p style={styles.note}>
                {'Positions are WGS 84. Values are shown at the resolution the original text carried ' +
                    'and no finer. A grid reference names a square, and the position shown for one is ' +
                    'its center.'}
            </p>

            {footer !== undefined && <div style={styles.footer}>{footer}</div>}
        </div>
    );
};

export default LocationReadings;
