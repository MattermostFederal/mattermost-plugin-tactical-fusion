import React from 'react';

import Disclosure from './Disclosure';
import {isSectionVisible} from './sections';
import type {CotChecklist, CotEvent} from './types';
import {statedColor} from './types';

const styles: Record<string, React.CSSProperties> = {
    group: {margin: '16px 0 4px', fontSize: '12px', textTransform: 'uppercase', letterSpacing: '0.04em', opacity: 0.85, fontWeight: 600},
    block: {margin: '12px 0 4px', fontSize: '12px', opacity: 0.85, fontWeight: 600},
    rows: {display: 'grid', gridTemplateColumns: 'max-content 1fr', gap: '4px 12px', margin: 0},
    term: {opacity: 0.85},
    value: {margin: 0, wordBreak: 'break-word'},
    swatch: {
        border: '1px solid rgba(var(--center-channel-color-rgb), 0.4)',
        borderRadius: 2,
        display: 'inline-block',
        height: 12,
        marginRight: 6,
        verticalAlign: '-1px',
        width: 12,
    },
    unknown: {margin: '12px 0 0', opacity: 0.85, fontSize: '13px'},
    flowTable: {borderCollapse: 'collapse', margin: 0, width: '100%'},
    flowCell: {padding: '2px 12px 2px 0', textAlign: 'left', verticalAlign: 'top'},
};

type Reading = [label: string, value: string];

function present(readings: Reading[]): Reading[] {
    return readings.filter(([, value]) => value !== '');
}

function checklistReadings(list: CotChecklist | null): Reading[] {
    if (list === null) {
        return [];
    }
    if (list.kinds.length === 0) {
        return [['Contents', 'None this build could count']];
    }
    return list.kinds.map((kind): Reading => [kind.name, kind.count]);
}

function Rows({readings}: {readings: Reading[]}) {
    return (
        <dl style={styles.rows}>
            {readings.map(([label, value]) => (
                <React.Fragment key={label}>
                    <dt style={styles.term}>{label}</dt>
                    <dd style={styles.value}>{value}</dd>
                </React.Fragment>
            ))}
        </dl>
    );
}

function Block({label, readings}: {label: string; readings: Reading[]}) {
    const shown = present(readings);
    if (shown.length === 0) {
        return null;
    }

    return (
        <>
            <h4 style={styles.block}>{label}</h4>
            <Rows readings={shown}/>
        </>
    );
}

function ColorRow({event}: {event: CotEvent}) {
    const stated = event.detail.colorArgb;
    if (stated === '') {
        return null;
    }

    const color = statedColor(event);

    return (
        <>
            <h4 style={styles.block}>{'Stated display color'}</h4>
            <dl style={styles.rows}>
                <dt style={styles.term}>{'Color'}</dt>
                <dd style={styles.value}>
                    {color !== undefined && (
                        <span
                            aria-hidden={true}
                            style={{...styles.swatch, background: color}}
                        />
                    )}
                    {stated}
                </dd>
            </dl>
        </>
    );
}

/**
 * The processing path, collapsed.
 *
 * Provenance rather than situational awareness, so it does not push the
 * countdown and the remarks down the panel. A native <details> is
 * keyboard-reachable and announced without any ARIA of its own.
 */
function ProcessingPath({event}: {event: CotEvent}) {
    const d = event.detail;
    const routing = present([
        ['Destinations', d.destinationServers],
        ['TAK control', d.takcontrol === '' ? '' : 'The event carries a protocol exchange'],
        ['Protocol supported', d.takcontrolSupportVersion],
        ['Protocol requested', d.takcontrolRequestVersion],
        ['Protocol response', d.takcontrolResponseStatus],
    ]);

    if (event.flow.length === 0 && routing.length === 0) {
        return null;
    }

    const heading = event.flow.length === 0 ? 'Routing' : `Processing path (${event.flow.length})`;

    return (
        <Disclosure label={heading}>
            {routing.length > 0 && <Rows readings={routing}/>}
            <table style={styles.flowTable}>
                <tbody>
                    {event.flow.map((hop, index) => (
                        <tr

                            // Two hops can name the same system: an event that
                            // went out and came back through one gateway is a
                            // path with a repeat in it, not a duplicate row.
                            // eslint-disable-next-line react/no-array-index-key
                            key={`${hop.system}-${index}`}
                        >
                            <th
                                scope='row'
                                style={{...styles.flowCell, ...styles.term, fontWeight: 400}}
                            >{hop.system}</th>
                            <td style={styles.flowCell}>{hop.time}</td>
                        </tr>
                    ))}
                </tbody>
            </table>
        </Disclosure>
    );
}

function Shape({event}: {event: CotEvent}) {
    const {geometry} = event;
    const d = event.detail;
    if (geometry === null) {
        return null;
    }

    const readings = present([
        ['Kind', SHAPE_WORDS[geometry.kind] ?? geometry.kind],
        ['Route type', d.routeType],
        ['Planning', d.routePlanning],
        ['Method', d.routeMethod],
        ['Direction', d.routeDirection],
        ['Order', d.routeOrder],
        ['Points', geometry.count],
        ['Closed', geometry.closed ? 'Yes' : ''],
        ['Major axis', geometry.major],
        ['Minor axis', geometry.minor],
        ['Orientation', geometry.angle],
    ]);

    return (
        <>
            <h3 style={styles.group}>{'Shape'}</h3>
            {readings.length > 0 && <Rows readings={readings}/>}
            {geometry.note !== '' && <p style={styles.unknown}>{geometry.note}</p>}
        </>
    );
}

const SHAPE_WORDS: Record<string, string> = {
    polyline: 'Drawn outline',
    ellipse: 'Circle or ellipse',
    route: 'Route',
};

function Dropped({event}: {event: CotEvent}) {
    if (event.detailDropped === '') {
        return null;
    }

    return (
        <p style={styles.unknown}>
            {'This post carried more detail than it had room to store, so the extension rows, the count of unrecognized elements and the event class were left out. '}
            {'The whole event is unchanged under "As posted" below.'}
        </p>
    );
}

function Unrecognized({event}: {event: CotEvent}) {
    const count = Number(event.detailUnknown);
    if (!Number.isFinite(count) || count <= 0) {
        return null;
    }

    const plural = count === 1 ? 'element' : 'elements';
    return (
        <p style={styles.unknown}>
            {`This event carried ${count} other <detail> ${plural} that this build does not recognize. `}
            {'They are unchanged under "As posted" below.'}
        </p>
    );
}

/**
 * Every registered extension the event carried, in a fixed handful of groups.
 *
 * Grouped rather than one section per block so the panel never runs past about
 * seven headings whatever an emitter writes, and drawn AFTER the readings, the
 * countdown and the remarks so none of those moves down the page.
 */
export const DetailGroups: React.FC<{event: CotEvent; hidden: readonly string[]}> = ({event, hidden}) => {
    const d = event.detail;

    const device = present([
        ['Platform', d.takvPlatform],
        ['Device', d.takvDevice],
        ['Operating system', d.takvOs],
        ['TAK version', d.takvVersion],
        ['Device id', d.uidExtraDroid],
        ['Battery', d.statusBattery],
        ['Readiness', d.statusReadiness],
        ['Network endpoint', d.contactEndpoint],
        ['Icon', d.usericonIconsetpath],
        ['Radio signal', d.radioRssi],
        ['Radio GPS', d.radioGps],
        ['Archive', d.archive === '' ? '' : 'The event asks to be kept'],
    ]);

    const precision = present([
        ['Position source', d.precisionGeopointsrc],
        ['Altitude source', d.precisionAltsrc],
        ['PDOP', d.precisionPdop],
        ['HDOP', d.precisionHdop],
        ['VDOP', d.precisionVdop],
    ]);

    const telemetry = present([
        ['Yaw', d.attitudeYaw],
        ['Pitch', d.attitudePitch],
        ['Roll', d.attitudeRoll],
        ['Slope', d.trackSlope],
    ]);

    const sensor: Reading[] = [
        ['Field of view', d.sensorFov],
        ['Vertical field of view', d.sensorVfov],
        ['Azimuth', d.sensorAzimuth],
        ['Elevation', d.sensorElevation],
        ['Range', d.sensorRange],
        ['Roll', d.sensorRoll],
        ['Model', d.sensorModel],
    ];

    const video: Reading[] = [
        ['Stream id', d.videoUid],
        ['Address', d.videoConnAddress],
        ['Port', d.videoConnPort],
        ['Protocol', d.videoConnProtocol],
        ['Path', d.videoConnPath],
        ['URL', d.videoUrl],
    ];

    const chat: Reading[] = [
        ['Stated sender', d.chatSender],
        ['Receipt for', d.chatReceiptId],
        ['Receipt room', d.chatReceiptRoom],
        ['Receipt from', d.chatReceiptSender],
        ['Acknowledged', d.chatReceiptAck],
        ['Room', d.chatRoom],
        ['Thread id', d.chatId],
        ['Parent', d.chatParent],
        ['Group owner', d.chatGroupOwner],
        ['Group id', d.chatgrpId],
        ['Group member 1', d.chatgrpUid0],
        ['Group member 2', d.chatgrpUid1],
    ];

    const medevac: Reading[] = [
        ['Title', d.medevacTitle],
        ['Urgent', d.medevacUrgent],
        ['Priority', d.medevacPriority],
        ['Routine', d.medevacRoutine],
        ['Litter', d.medevacLitter],
        ['Ambulatory', d.medevacAmbulatory],
        ['CASEVAC', d.medevacCasevac],
        ['Frequency', d.medevacFreq],
        ['Security', d.medevacSecurity],
        ['HLZ marking', d.medevacHlzMarking],
        ['Terrain', d.medevacTerrainNone],
        ['Equipment', d.medevacEquipmentDetail],
        ['Equipment not needed', d.medevacEquipmentNone],
        ['Zone protection', d.medevacZoneProtSelection],
        ['Nationality', d.medevacNationality],
        ['NBC', d.medevacNbc],
        ['Medline remarks', d.medevacMedlineRemarks],
    ];

    const geofence: Reading[] = [
        ['Monitoring', d.geofenceMonitor],
        ['Trigger', d.geofenceTrigger],
        ['Tracking', d.geofenceTracking],
        ['Elevation monitored', d.geofenceElevation],
        ['Minimum elevation', d.geofenceMin],
        ['Maximum elevation', d.geofenceMax],
        ['Bounding sphere', d.geofenceSphere],
    ];

    const attachments: Reading[] = [
        ['Files referenced', d.attachmentsCount],
    ];

    const checklist = checklistReadings(event.checklist);

    const payload = present([
        ...sensor, ...video, ...chat, ...medevac, ...geofence, ...attachments,
        ...checklist,
    ]).length > 0;
    const hasColor = d.colorArgb !== '';

    return (
        <>
            {isSectionVisible(hidden, 'device') && (device.length > 0 || hasColor) && (
                <>
                    <h3 style={styles.group}>{'Device'}</h3>
                    {device.length > 0 && <Rows readings={device}/>}
                    <ColorRow event={event}/>
                </>
            )}

            {isSectionVisible(hidden, 'precision') && precision.length > 0 && (
                <>
                    <h3 style={styles.group}>{'Position quality'}</h3>
                    <Rows readings={precision}/>
                </>
            )}

            {isSectionVisible(hidden, 'orientation') && telemetry.length > 0 && (
                <>
                    <h3 style={styles.group}>{'Orientation'}</h3>
                    <Rows readings={telemetry}/>
                </>
            )}

            {isSectionVisible(hidden, 'payload') && payload && (
                <>
                    <h3 style={styles.group}>{'Payload'}</h3>
                    <Block
                        label='Sensor'
                        readings={sensor}
                    />
                    <Block
                        label='Video'
                        readings={video}
                    />
                    <Block
                        label='GeoChat'
                        readings={chat}
                    />
                    <Block
                        label='MEDEVAC'
                        readings={medevac}
                    />
                    <Block
                        label='Geofence'
                        readings={geofence}
                    />
                    <Block
                        label='Attachments'
                        readings={attachments}
                    />
                    <Block
                        label='Checklist'
                        readings={checklist}
                    />
                </>
            )}

            {isSectionVisible(hidden, 'shape') && <Shape event={event}/>}
            {isSectionVisible(hidden, 'flow') && <ProcessingPath event={event}/>}
            <Unrecognized event={event}/>
            <Dropped event={event}/>
        </>
    );
};

export default DetailGroups;
