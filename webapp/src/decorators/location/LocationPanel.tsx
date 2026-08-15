import React, {useLayoutEffect} from 'react';

import {useConversion} from './convert';
import CopyButton from './CopyButton';
import Customize from './Customize';
import {setEditing, useEditing} from './editing';
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
import {isRowVisible, ROWS} from './rows';
import type {RowID} from './rows';

import LinkButton from '../../components/LinkButton';
import {docsUrl} from '../../plugin_url';
import {usePreferences} from '../../preferences/store';

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
const PENDING = 'converting…';
const UNAVAILABLE = 'unavailable';

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
 * The location panel.
 *
 * Split by where a value can be worked out rather than by format. Anything the
 * canonical token yields by slicing and dividing is computed here and is on
 * screen the instant the panel opens; anything needing the ellipsoid comes from
 * the conversion endpoint, because the projection lives in Go and writing a
 * second one in TypeScript would be two implementations of the same geodesy.
 *
 * In practice that means a lat/lon link renders complete and fills in its grid
 * rows a moment later, while a grid link shows its own reference, its
 * resolution and the author's text immediately and fills in the position. Both
 * survive the request failing outright.
 *
 * Each value row carries its own copy icon rather than the panel carrying a row
 * of labelled buttons underneath: there are eleven values worth copying and a
 * button per value would be wider than the coordinates themselves.
 *
 * `raw` is the one value here that originated as message text. It is rendered
 * as a React text node and nothing on this panel uses `dangerouslySetInnerHTML`.
 */
const LocationPanel: React.FC<{payload: LocationPayload}> = ({payload}) => {
    const {coord, format, canonical, raw} = payload;

    const conversion = useConversion(format, canonical, raw);
    const {preferences} = usePreferences();
    const customizing = useEditing();

    // Clicking a different coordinate while the editor is open would otherwise
    // land on the editor rather than on the coordinate that was clicked. React
    // keeps this component mounted across a change of selection, so nothing
    // else resets it.
    //
    // Before paint, not after, so the editor is never flashed on screen
    // carrying a coordinate the reader did not open it from.
    useLayoutEffect(() => {
        setEditing(false);
    }, [format, canonical]);

    // The server looked at this link and said it is not one this plugin wrote.
    //
    // Refusing to render it is the point rather than a failure state. Two of
    // the four checks on the author's text need the token grammar, which lives
    // in Go, so before the conversion carried "r" a hand-written link could put
    // any short run of letters and digits in the "Original text" row next to a
    // position derived from a different token, with a copy button beside it,
    // while the server-rendered page refused the identical link. The panel now
    // asks the same question the page asks, and this is it agreeing.
    //
    // Distinguished from a request that simply did not arrive, which degrades
    // instead: an offline reader keeps every row the token yields locally.
    // The editor takes the panel over rather than sitting below the table,
    // matching the DTG panel: a list of settings under the coordinate would
    // bury the thing the reader opened the sidebar for.
    //
    // Ahead of the rejected check on purpose. A reader who reached the editor
    // and then clicked a hand-edited link would otherwise be dropped on "Not a
    // coordinate" with no way back to their settings.
    if (customizing) {
        return <Customize onClose={() => setEditing(false)}/>;
    }

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
    const mgrs = format === 'mgrs' ? gridText(format, canonical) : remote(conversion.data?.mgrs);
    const utm = format === 'utm' ? gridText(format, canonical) : remote(conversion.data?.utm);
    const own = (id: LocationFormat, value: string | undefined): string =>
        (format === id ? canonical : remote(value));

    const decimal = coord ? decimalText(coord) : remote(conversion.data?.decimal);
    const dms = coord ? dmsText(coord) : remote(conversion.data?.dms);
    const ddm = coord ? ddmText(coord) : remote(conversion.data?.ddm);
    const usmtf = coord ? usmtfText(coord) : remote(conversion.data?.usmtf);

    const resolution = coord ? resolutionText(coord) : remoteResolutionText(format, canonical);
    const confidence = coord ? confidenceText(coord) : '';

    // The author's own text, but only once the server has vouched for it.
    //
    // Two of the four gates on `r` need the token grammar, which lives in Go,
    // so `fromParams` can check its length and its alphabet and nothing more,
    // and that alphabet had to widen to the whole Latin alphabet for grid
    // letters. A hand-written link can therefore put a short run of words in
    // `r` that this side cannot tell from a coordinate: `r=DISREGARD USE
    // 18SUJ11111111` passes both local gates.
    //
    // Rendering it before the verdict arrives put that string in a row labeled
    // as the author's words, with a copy button beside it, on every open of
    // every link during loading, and permanently whenever the request failed.
    // Falling back to the canonical token means the row is always true: either
    // the author's text as the server confirmed it, or the token the link is
    // built from.
    const vouched = conversion.status === 'ready' ? raw : canonical;

    // Every row's value, by id, so the table can be rendered from the shared
    // ROWS catalog rather than from eleven hand-written entries. That catalog
    // is what the reader's hidden-row setting names and what the Go page
    // renders from, so a row that existed here and nowhere else would be a row
    // nobody could hide.
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

    const hidden = preferences.location.hiddenRows;

    // No lead line. The table already opens with the grid reference, labeled
    // and with a copy button beside it, so a bare repeat of the same string
    // three lines above it was saying nothing twice.
    return (
        <div>
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

            <div style={styles.footer}>
                <LinkButton onClick={() => setEditing(true)}>{'Customize your view'}</LinkButton>
                {' · '}
                <LinkButton href={docsUrl()}>{'Documentation'}</LinkButton>
            </div>
        </div>
    );
};

export default LocationPanel;
