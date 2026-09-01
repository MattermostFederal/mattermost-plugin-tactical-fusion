import React from 'react';

import {useAirport} from './airport';
import type {AirportState} from './airport';
import type {AirportCoordinate} from './types';

import LinkButton from '../../components/LinkButton';
import {useFeatures} from '../../features/store';
import type {Features} from '../../features/types';
import {docsUrl} from '../../plugin_url';
import location from '../location';
import type {LocationPayload} from '../location';
import CopyButton from '../location/CopyButton';
import LocationMap from '../location/map/LocationMap';
import {mapPageHref, viewFor} from '../location/map/view';
import {setSelection} from '../selection';

import type {AirportPayload} from './index';

const styles: Record<string, React.CSSProperties> = {
    name: {
        fontSize: '18px',
        fontWeight: 600,
        color: 'var(--center-channel-color)',
        margin: '0 0 2px',
    },
    ident: {
        fontFamily: 'ui-monospace, SFMono-Regular, Menlo, monospace',
        fontSize: '12px',
        letterSpacing: '0.08em',
        opacity: 0.6,
        color: 'var(--center-channel-color)',
        margin: '0 0 16px',
    },
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
    onward: {marginTop: '14px', fontSize: '13px'},
    mapWrap: {marginTop: '18px'},
};

/**
 * One line of the airfield.
 *
 * `onOpen` turns the VALUE into the link rather than adding a second control
 * beside it. The place is where the airfield is, so the place is the thing to
 * follow to see where that is; a separate line saying "Location" underneath was
 * a second way to say the same thing.
 *
 * The copy button stays either way: copying "Indianapolis, IN, US" is copying a
 * value, and whether it also opens something is beside the point.
 *
 * A row that carries a classification rather than a value gets none, which is
 * the rule the coordinate table follows for Resolution, Confidence and Datum:
 * copying "Large Airport" gets you a category, not something to paste into
 * anything.
 */
const Row: React.FC<{
    label: string;
    value: string;
    plain?: boolean;
    copyable?: boolean;
    onOpen?: () => void;
}> = ({label, value, plain, copyable = true, onOpen}) => (
    <tr>
        <th
            scope='row'
            style={styles.th}
        >{label}</th>
        <td style={plain ? {...styles.td, ...styles.plain} : styles.td}>
            {onOpen ? <LinkButton onClick={onOpen}>{value}</LinkButton> : value}
        </td>
        <td style={styles.copyCell}>
            {copyable && (
                <CopyButton
                    label={`Copy ${label}`}
                    value={value}
                />
            )}
        </td>
    </tr>
);

/**
 * The payload the map draws and the place links to, or null.
 *
 * Built through the location decorator's own `fromParams` from the (format,
 * token) pair the server sent, so both of them use exactly what a click on a
 * coordinate link would have produced. Null can only mean the two sides
 * disagree about the grammar, and then neither the map nor the link is offered,
 * the same way the click handler stands aside rather than guessing.
 */
function positionPayload(coordinate: AirportCoordinate): LocationPayload | null {
    const payload = location.fromParams(new URLSearchParams({f: coordinate.format, v: coordinate.value}));
    if (payload) {
        return payload;
    }

    // eslint-disable-next-line no-console
    console.warn(
        '[tactical-fusion] this build does not recognize the coordinate the server issued for an airfield; ' +
        'drawing no map and offering no link',
        {format: coordinate.format, value: coordinate.value},
    );

    return null;
}

/**
 * Where the map should point, or null when there is nothing to draw yet.
 *
 * Read from the state rather than from inside the rendered table, because the
 * map has to be mounted from a place that survives a change of selection. See
 * `AirportPanel`.
 */
function positionOf(state: AirportState): {payload: LocationPayload; region: string} | null {
    if (state.status !== 'ready' || !state.data || !state.data.found) {
        return null;
    }

    const {coordinate} = state.data;
    if (!coordinate) {
        return null;
    }

    const payload = positionPayload(coordinate);
    return payload ? {payload, region: coordinate.region} : null;
}

/**
 * The airfield's position, drawn.
 *
 * The map is the location decorator's own component, not a second one: two
 * implementations of a projection and a palette are two things that can
 * disagree, and this is the place a reader looks first.
 *
 * No conversion is needed once the lookup lands: `viewFor` reads latitude and
 * longitude out of the parsed token and only the region comes from the server.
 * While the lookup is in flight there is no token yet, so the position is null
 * and `pending` carries the wait, which is the same pair `LocationReadings`
 * passes for a grid token whose conversion has not arrived.
 */
const Position: React.FC<{
    position: {payload: LocationPayload; region: string} | null;
    pending: boolean;
    features: Features;
}> = ({position, pending, features}) => {
    if (!features.mapPanel) {
        return null;
    }

    if (!position) {
        return (
            <LocationMap
                lat={null}
                lon={null}
                cellDegLat={0}
                cellDegLon={0}
                region=''
                pending={pending}
            />
        );
    }

    return (
        <LocationMap
            {...viewFor(position.payload, {status: 'loading', data: null})}
            region={position.region}
            pageHref={features.mapPage ? mapPageHref(position.payload) : undefined}
            pending={false}
        />
    );
};

const Footer: React.FC = () => (
    <div style={styles.footer}>
        <LinkButton href={docsUrl()}>{'Documentation'}</LinkButton>
    </div>
);

/**
 * The airfield panel.
 *
 * Everything here comes from the server, which is what makes this decorator
 * different from the other two: the airfield database is compiled into the Go
 * binary and the link carries only the code, so there is nothing to compute
 * locally and nothing to show before the answer arrives.
 */
const AirportPanel: React.FC<{payload: AirportPayload}> = ({payload}) => {
    const state = useAirport(payload.ident);
    const {features} = useFeatures();
    const position = positionOf(state);

    // The map is mounted from HERE rather than from inside renderBody, and that
    // placement is the whole of it. The sidebar keeps this panel mounted across
    // a change of selection, so a second code drives the state back to loading;
    // with the mount inside the "found" branch that unmounted LocationMap,
    // which released its WebGL context and re-tessellated the whole basemap on
    // the next answer, against the browser's cap of roughly sixteen live
    // contexts shared with the hover and every inline map on screen.
    const drawn = position !== null || state.status === 'loading';

    return (
        <>
            {renderBody(payload.ident, state, position)}
            {drawn && (
                <Position
                    position={position}
                    pending={state.status === 'loading'}
                    features={features}
                />
            )}
            <Footer/>
        </>
    );
};

function renderBody(
    ident: string,
    state: AirportState,
    position: {payload: LocationPayload; region: string} | null,
): React.ReactNode {
    if (state.status === 'loading') {
        return (
            <>
                <p style={styles.ident}>{ident}</p>
                <p style={styles.note}>{'Looking up this airfield…'}</p>
            </>
        );
    }

    // A hand-edited link. The same verdict the location panel renders, and the
    // same reason: the server looked at it and said it is not one this plugin
    // issued.
    if (state.status === 'rejected') {
        return (
            <>
                <p style={styles.verdict}>{'Not an airfield code'}</p>
                <p style={styles.note}>
                    {'This link does not carry an airfield code this plugin issued, so there is nothing to look up. It was most likely edited by hand.'}
                </p>
            </>
        );
    }

    // Nothing may fail the panel silently, and there is nothing computed
    // locally to fall back to, so this says so plainly rather than rendering
    // empty rows.
    if (state.status === 'failed' || !state.data) {
        return (
            <>
                <p style={styles.ident}>{ident}</p>
                <p style={styles.note}>
                    {'This airfield could not be looked up just now. The link is fine; the server could not be reached.'}
                </p>
            </>
        );
    }

    const answer = state.data;

    // An answer, not a failure: the database is refreshed with the plugin, so a
    // code that was recognized when the message was written can be retired
    // later. Saying so beats a blank panel.
    if (!answer.found || !answer.airport) {
        return (
            <>
                <p style={styles.name}>{ident}</p>
                <p style={styles.note}>
                    {'This airfield code is not in this build\'s airfield database. The database is refreshed with the plugin, so a code that was recognized when the message was written may have been retired since.'}
                </p>
            </>
        );
    }

    const {airport, coordinate} = answer;
    const openPosition = position ? () => setSelection({type: location.type, payload: position.payload}) : undefined;

    return (
        <>
            <p style={styles.name}>{airport.name || ident}</p>
            <p style={styles.ident}>{ident}</p>

            <table style={styles.table}>
                <tbody>
                    <Row
                        label='Code'
                        value={ident}
                    />
                    {airport.place !== '' && (
                        <Row
                            label='Place'
                            value={airport.place}
                            plain={true}
                            onOpen={openPosition}
                        />
                    )}
                    {airport.type !== '' && (
                        <Row
                            label='Type'
                            value={airport.type}
                            plain={true}
                            copyable={false}
                        />
                    )}
                    {airport.elevation !== '' && (
                        <Row
                            label='Elevation'
                            value={airport.elevation}
                            plain={true}
                        />
                    )}
                    {airport.iata !== '' && (
                        <Row
                            label='IATA'
                            value={airport.iata}
                        />
                    )}

                </tbody>
            </table>

            {position === null && (
                <p style={styles.note}>
                    {coordinate === undefined ? 'This airfield has no position in the database, so there is nothing to draw and no coordinate readings for it.' : 'This airfield\'s position could not be read by this version of the plugin, so there is nothing to draw and no coordinate readings for it.'}
                </p>
            )}
        </>
    );
}

export default AirportPanel;
