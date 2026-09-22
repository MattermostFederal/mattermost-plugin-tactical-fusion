import React from 'react';

import {useAirport} from './airport';
import type {AirportState} from './airport';
import {positionPayload, runwayLabel, runwayShapes} from './map';
import type {AirportCoordinate, AirportDetails, AirportFrequency, AirportRunway} from './types';

import LinkButton from '../../components/LinkButton';
import {useFeatures} from '../../features/store';
import type {Features} from '../../features/types';
import {docsUrl} from '../../plugin_url';
import location from '../location';
import type {LocationPayload} from '../location';
import CopyButton from '../location/CopyButton';
import LocationMap from '../location/map/LocationMap';
import type {MapShape} from '../location/map/paint';
import {airportMapPageHref, viewFor} from '../location/map/view';
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
    section: {
        fontSize: '11px',
        textTransform: 'uppercase',
        letterSpacing: '0.04em',
        fontWeight: 600,
        opacity: 0.7,
        color: 'var(--center-channel-color)',
        margin: '18px 0 4px',
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

function readPosition(coordinate: AirportCoordinate): LocationPayload | null {
    const payload = positionPayload(coordinate);
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

interface Position {
    ident: string;
    payload: LocationPayload;
    region: string;
    shapes: MapShape[];
}

function positionOf(state: AirportState): Position | null {
    if (state.status !== 'ready' || !state.data || !state.data.found || !state.data.airport) {
        return null;
    }

    const {coordinate, airport, ident} = state.data;
    if (!coordinate) {
        return null;
    }

    const payload = readPosition(coordinate);
    return payload ? {ident, payload, region: coordinate.region, shapes: runwayShapes(airport.runways)} : null;
}

const Position: React.FC<{
    position: Position | null;
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
            geometries={position.shapes}
            markerLabel={runwayLabel(position.shapes.length)}
            pageHref={features.mapPage ? airportMapPageHref(position.ident) : undefined}
            pending={false}
        />
    );
};

const Runways: React.FC<{runways: AirportRunway[]}> = ({runways}) => {
    if (runways.length === 0) {
        return null;
    }

    return (
        <>
            <h3 style={styles.section}>{'Runways'}</h3>
            <table style={styles.table}>
                <tbody>
                    {runways.map((runway, index) => (
                        // eslint-disable-next-line react/no-array-index-key
                        <tr key={`${runway.designation}-${index}`}>
                            <th
                                scope='row'
                                style={styles.th}
                            >{runway.designation}</th>
                            <td style={{...styles.td, ...styles.plain}}>{runway.summary}</td>
                            <td style={styles.copyCell}/>
                        </tr>
                    ))}
                </tbody>
            </table>
        </>
    );
};

const Frequencies: React.FC<{frequencies: AirportFrequency[]}> = ({frequencies}) => {
    if (frequencies.length === 0) {
        return null;
    }

    return (
        <>
            <h3 style={styles.section}>{'Frequencies'}</h3>
            <table style={styles.table}>
                <tbody>
                    {frequencies.map((frequency, index) => (
                        <tr key={`${frequency.type}-${frequency.mhz}-${index}`}>
                            <th
                                scope='row'
                                style={styles.th}
                            >
                                {frequency.type}
                                {frequency.description !== '' && (
                                    <span style={{fontWeight: 400, textTransform: 'none', letterSpacing: 0}}>
                                        {` ${frequency.description}`}
                                    </span>
                                )}
                            </th>
                            <td style={styles.td}>{frequency.mhz}</td>
                            <td style={styles.copyCell}>
                                <CopyButton
                                    label={`Copy ${frequency.type}${frequency.description === '' ? '' : ` ${frequency.description}`} ${frequency.mhz}`}
                                    value={frequency.mhz}
                                />
                            </td>
                        </tr>
                    ))}
                </tbody>
            </table>
        </>
    );
};

const Footer: React.FC = () => (
    <div style={styles.footer}>
        <LinkButton href={docsUrl()}>{'Documentation'}</LinkButton>
    </div>
);

const AirportPanel: React.FC<{payload: AirportPayload}> = ({payload}) => {
    const state = useAirport(payload);
    const {features} = useFeatures();
    const position = positionOf(state);

    const drawn = position !== null || state.status === 'loading';

    return (
        <>
            {renderBody(payload.code, state, position)}
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

function militaryUse(airport: AirportDetails): string {
    return airport.military === '' ? '' : `Military (${airport.military})`;
}

function renderBody(
    code: string,
    state: AirportState,
    position: Position | null,
): React.ReactNode {
    if (state.status === 'loading') {
        return (
            <>
                <p style={styles.ident}>{code}</p>
                <p style={styles.note}>{'Looking up this airfield…'}</p>
            </>
        );
    }

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

    if (state.status === 'failed' || !state.data) {
        return (
            <>
                <p style={styles.ident}>{code}</p>
                <p style={styles.note}>
                    {'This airfield could not be looked up just now. The link is fine; the server could not be reached.'}
                </p>
            </>
        );
    }

    const answer = state.data;

    if (!answer.found || !answer.airport) {
        return (
            <>
                <p style={styles.name}>{code}</p>
                <p style={styles.note}>
                    {'This airfield code is not in this build\'s airfield database. The database is refreshed with the plugin, so a code that was recognized when the message was written may have been retired since.'}
                </p>
            </>
        );
    }

    const {airport, coordinate, ident} = answer;
    const openPosition = position ? () => setSelection({type: location.type, payload: position.payload}) : undefined;
    const use = militaryUse(airport);

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
                    {use !== '' && (
                        <Row
                            label='Use'
                            value={use}
                            plain={true}
                            copyable={false}
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

            <Runways runways={airport.runways}/>
            <Frequencies frequencies={airport.frequencies}/>

            {position === null && (
                <p style={styles.note}>
                    {coordinate === undefined ? 'This airfield has no position in the database, so there is nothing to draw and no coordinate readings for it.' : 'This airfield\'s position could not be read by this version of the plugin, so there is nothing to draw and no coordinate readings for it.'}
                </p>
            )}
        </>
    );
}

export default AirportPanel;
