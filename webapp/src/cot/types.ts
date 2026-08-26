export const COT_POST_TYPE = 'custom_tf_cot';

export const COT_PROPS_KEY = 'tactical_fusion_cot';

/**
 * What the sidebar keys this panel on.
 *
 * Deliberately not a decorator type: there is no `/decorate/cot` route and no
 * link carrying this, so nothing parses it out of an href. It only ever reaches
 * the selection store from the card's own button.
 */
export const COT_PANEL_TYPE = 'cot';

/**
 * The shape the server writes today: a blob holding an `events` array.
 *
 * Version 1 held a single `event`, and posts stamped then are still out there
 * and still render, which is why `readEvents` reads both. A version NEITHER of
 * them knows is refused, and the card falls back to the post's own text rather
 * than guessing at a shape it has never seen.
 */
export const COT_PROPS_VERSION = 2;

const READABLE_VERSIONS = [1, 2];

/**
 * Every registered <detail> extension, flat and registry-derived.
 *
 * The key space is closed on the Go side and every name here comes from the
 * registry, which is what keeps an author-chosen string from ever landing
 * beside `format`, `value` or `affiliation` in the same map.
 */
export interface CotDetail {
    archive: string;
    attitudePitch: string;
    attitudeRoll: string;
    attitudeYaw: string;
    chatGroupOwner: string;
    chatId: string;
    chatParent: string;
    chatRoom: string;
    chatSender: string;
    chatgrpId: string;
    chatgrpUid0: string;
    chatgrpUid1: string;
    colorArgb: string;
    contactEndpoint: string;
    medevacAmbulatory: string;
    medevacCasevac: string;
    medevacEquipmentDetail: string;
    medevacEquipmentNone: string;
    medevacFreq: string;
    medevacHlzMarking: string;
    medevacLitter: string;
    medevacMedlineRemarks: string;
    medevacNationality: string;
    medevacNbc: string;
    medevacPriority: string;
    medevacRoutine: string;
    medevacSecurity: string;
    medevacTerrainNone: string;
    medevacTitle: string;
    medevacUrgent: string;
    medevacZoneProtSelection: string;
    chatReceiptAck: string;
    chatReceiptId: string;
    chatReceiptRoom: string;
    chatReceiptSender: string;
    destinationServers: string;
    radioRssi: string;
    radioGps: string;
    geofenceElevation: string;
    geofenceMax: string;
    geofenceMin: string;
    geofenceMonitor: string;
    geofenceSphere: string;
    geofenceTracking: string;
    geofenceTrigger: string;
    attachmentsCount: string;
    takcontrol: string;
    takcontrolRequestVersion: string;
    takcontrolResponseStatus: string;
    takcontrolSupportVersion: string;
    routeType: string;
    routePlanning: string;
    routeMethod: string;
    routeDirection: string;
    routeOrder: string;
    precisionAltsrc: string;
    precisionGeopointsrc: string;
    precisionHdop: string;
    precisionPdop: string;
    precisionVdop: string;
    sensorAzimuth: string;
    sensorElevation: string;
    sensorFov: string;
    sensorModel: string;
    sensorRange: string;
    sensorRoll: string;
    sensorVfov: string;
    statusBattery: string;
    statusReadiness: string;
    takvDevice: string;
    takvOs: string;
    takvPlatform: string;
    takvVersion: string;
    trackSlope: string;
    uidExtraDroid: string;
    usericonIconsetpath: string;
    videoConnAddress: string;
    videoConnPath: string;
    videoConnPort: string;
    videoConnProtocol: string;
    videoUid: string;
    videoUrl: string;
}

export interface CotFlowHop {
    system: string;
    time: string;
}

export interface CotVertex {
    lat: number;
    lon: number;
}

export interface CotChecklistKind {
    name: string;
    count: string;
}

export interface CotChecklist {
    count: string;
    kinds: CotChecklistKind[];
}

/**
 * The shape an event describes, when it describes one.
 *
 * `points` is empty for an ellipse, which is drawn from its axes and the
 * event's own position, and for a shape the server would not stand behind: it
 * says so in `note` rather than handing over a partial outline.
 */
export interface CotGeometry {
    kind: string;
    closed: boolean;
    points: CotVertex[];
    count: string;
    major: string;
    minor: string;
    angle: string;
    majorMeters: number;
    minorMeters: number;
    angleDegrees: number;
    note: string;
}

/** The classes the server writes. Anything else falls to the default layout. */
export const COT_CLASSES = ['chat', 'medevac', 'sensor', 'video'] as const;

export type CotClass = (typeof COT_CLASSES)[number] | '';

export interface CotEvent {
    uid: string;
    cotClass: CotClass;
    detailUnknown: string;
    detailDropped: string;
    detail: CotDetail;
    flow: CotFlowHop[];
    geometry: CotGeometry | null;
    checklist: CotChecklist | null;
    callsign: string;
    cotType: string;
    typeLabel: string;
    affiliation: string;
    how: string;
    howLabel: string;
    time: string;
    timeQuery: string;
    start: string;
    startQuery: string;
    stale: string;
    staleQuery: string;
    staleAt: string;
    timeAt: string;
    format: string;
    value: string;
    lat: string;
    lon: string;
    positionNote: string;
    hae: string;
    ce: string;
    ceMeters: string;
    le: string;
    speed: string;
    course: string;
    group: string;
    role: string;
    remarks: string;
    parent: string;
    related: string;
}

export interface CotPayload {
    source: string;
    lead: string;
    trail: string;
    src: string;
    fileId: string;
    fileName: string;
    events: CotEvent[];
}

export const SOURCE_FENCE = 'fence';
export const SOURCE_FILE = 'file';

function text(blob: Record<string, unknown>, key: string): string {
    const value = Object.hasOwn(blob, key) ? blob[key] : undefined;
    return typeof value === 'string' ? value : '';
}

function record(value: unknown): Record<string, unknown> | null {
    if (typeof value !== 'object' || value === null || Array.isArray(value)) {
        return null;
    }
    return value as Record<string, unknown>;
}

export function fromProps(props: unknown): CotPayload | null {
    const outer = record(props);
    if (outer === null) {
        return null;
    }

    const blob = record(outer[COT_PROPS_KEY]);
    if (blob === null || !READABLE_VERSIONS.includes(Number(blob.version))) {
        return null;
    }

    const source = text(blob, 'source');
    if (source !== SOURCE_FENCE && source !== SOURCE_FILE) {
        return null;
    }

    const events = readEvents(blob);
    if (events === null) {
        return null;
    }

    return {
        source,
        lead: text(blob, 'lead'),
        trail: text(blob, 'trail'),
        src: text(blob, 'src'),
        fileId: text(blob, 'file_id'),
        fileName: text(blob, 'file_name'),
        events,
    };
}

/**
 * The events, from either shape the server has written.
 *
 * Null rather than an empty array for a blob carrying none, since a stamped
 * post with nothing in it is one the card cannot honour and should hand back to
 * the post's own text.
 */
function readEvents(blob: Record<string, unknown>): CotEvent[] | null {
    const raw = Array.isArray(blob.events) ? blob.events : [blob.event];

    const events: CotEvent[] = [];
    for (const entry of raw) {
        const event = record(entry);
        if (event === null) {
            return null;
        }

        const read = readEvent(event);
        if (read === null) {
            return null;
        }
        events.push(read);
    }

    return events.length === 0 ? null : events;
}

function readEvent(event: Record<string, unknown>): CotEvent | null {
    const uid = text(event, 'uid');
    if (uid === '') {
        return null;
    }

    return {
        uid,
            cotClass: readClass(event),
            detailUnknown: text(event, 'detail_unknown'),
            detailDropped: text(event, 'detail_dropped'),
            detail: readDetail(event),
            flow: readFlow(event),
            geometry: readGeometry(event),
            checklist: readChecklist(event),
            callsign: text(event, 'callsign'),
            cotType: text(event, 'cot_type'),
            typeLabel: text(event, 'type_label'),
            affiliation: text(event, 'affiliation'),
            how: text(event, 'how'),
            howLabel: text(event, 'how_label'),
            time: text(event, 'time'),
            timeQuery: text(event, 'time_q'),
            start: text(event, 'start'),
            startQuery: text(event, 'start_q'),
            stale: text(event, 'stale'),
            staleQuery: text(event, 'stale_q'),
            staleAt: text(event, 'stale_at'),
            timeAt: text(event, 'time_at'),
            format: text(event, 'format'),
            value: text(event, 'value'),
            lat: text(event, 'lat'),
            lon: text(event, 'lon'),
            positionNote: text(event, 'position_note'),
            hae: text(event, 'hae'),
            ce: text(event, 'ce'),
            ceMeters: text(event, 'ce_meters'),
            le: text(event, 'le'),
            speed: text(event, 'speed'),
            course: text(event, 'course'),
            group: text(event, 'group'),
            role: text(event, 'role'),
        remarks: text(event, 'remarks'),
        parent: text(event, 'parent'),
        related: text(event, 'related'),
    };
}

/**
 * A detail block with nothing in it.
 *
 * Exported for the harnesses and the tests, which otherwise have to spell every
 * registry key to build one event, and would then quietly stop compiling every
 * time the registry grows.
 */
export function emptyDetail(): CotDetail {
    return {
    archive: '',
    attitudePitch: '',
    attitudeRoll: '',
    attitudeYaw: '',
    chatGroupOwner: '',
    chatId: '',
    chatParent: '',
    chatRoom: '',
    chatSender: '',
    chatgrpId: '',
    chatgrpUid0: '',
    chatgrpUid1: '',
    colorArgb: '',
    contactEndpoint: '',
    medevacAmbulatory: '',
    medevacCasevac: '',
    medevacEquipmentDetail: '',
    medevacEquipmentNone: '',
    medevacFreq: '',
    medevacHlzMarking: '',
    medevacLitter: '',
    medevacMedlineRemarks: '',
    medevacNationality: '',
    medevacNbc: '',
    medevacPriority: '',
    medevacRoutine: '',
    medevacSecurity: '',
    medevacTerrainNone: '',
    medevacTitle: '',
    medevacUrgent: '',
    medevacZoneProtSelection: '',
    chatReceiptAck: '',
    chatReceiptId: '',
    chatReceiptRoom: '',
    chatReceiptSender: '',
    destinationServers: '',
    radioRssi: '',
    radioGps: '',
    geofenceElevation: '',
    geofenceMax: '',
    geofenceMin: '',
    geofenceMonitor: '',
    geofenceSphere: '',
    geofenceTracking: '',
    geofenceTrigger: '',
    attachmentsCount: '',
    takcontrol: '',
    takcontrolRequestVersion: '',
    takcontrolResponseStatus: '',
    takcontrolSupportVersion: '',
    routeType: '',
    routePlanning: '',
    routeMethod: '',
    routeDirection: '',
    routeOrder: '',
    precisionAltsrc: '',
    precisionGeopointsrc: '',
    precisionHdop: '',
    precisionPdop: '',
    precisionVdop: '',
    sensorAzimuth: '',
    sensorElevation: '',
    sensorFov: '',
    sensorModel: '',
    sensorRange: '',
    sensorRoll: '',
    sensorVfov: '',
    statusBattery: '',
    statusReadiness: '',
    takvDevice: '',
    takvOs: '',
    takvPlatform: '',
    takvVersion: '',
    trackSlope: '',
    uidExtraDroid: '',
    usericonIconsetpath: '',
    videoConnAddress: '',
    videoConnPath: '',
    videoConnPort: '',
    videoConnProtocol: '',
    videoUid: '',
    videoUrl: '',
    };
}

function readClass(event: Record<string, unknown>): CotClass {
    const value = text(event, 'class');
    return (COT_CLASSES as readonly string[]).includes(value) ? (value as CotClass) : '';
}

function readDetail(event: Record<string, unknown>): CotDetail {
    return {
        archive: text(event, 'archive'),
        attitudePitch: text(event, 'attitude_pitch'),
        attitudeRoll: text(event, 'attitude_roll'),
        attitudeYaw: text(event, 'attitude_yaw'),
        chatGroupOwner: text(event, 'chat_group_owner'),
        chatId: text(event, 'chat_id'),
        chatParent: text(event, 'chat_parent'),
        chatRoom: text(event, 'chat_room'),
        chatSender: text(event, 'chat_sender'),
        chatgrpId: text(event, 'chatgrp_id'),
        chatgrpUid0: text(event, 'chatgrp_uid0'),
        chatgrpUid1: text(event, 'chatgrp_uid1'),
        colorArgb: text(event, 'color_argb'),
        contactEndpoint: text(event, 'contact_endpoint'),
        medevacAmbulatory: text(event, 'medevac_ambulatory'),
        medevacCasevac: text(event, 'medevac_casevac'),
        medevacEquipmentDetail: text(event, 'medevac_equipment_detail'),
        medevacEquipmentNone: text(event, 'medevac_equipment_none'),
        medevacFreq: text(event, 'medevac_freq'),
        medevacHlzMarking: text(event, 'medevac_hlz_marking'),
        medevacLitter: text(event, 'medevac_litter'),
        medevacMedlineRemarks: text(event, 'medevac_medline_remarks'),
        medevacNationality: text(event, 'medevac_nationality'),
        medevacNbc: text(event, 'medevac_nbc'),
        medevacPriority: text(event, 'medevac_priority'),
        medevacRoutine: text(event, 'medevac_routine'),
        medevacSecurity: text(event, 'medevac_security'),
        medevacTerrainNone: text(event, 'medevac_terrain_none'),
        medevacTitle: text(event, 'medevac_title'),
        medevacUrgent: text(event, 'medevac_urgent'),
        medevacZoneProtSelection: text(event, 'medevac_zone_prot_selection'),
        chatReceiptAck: text(event, 'chat_receipt_ack'),
        chatReceiptId: text(event, 'chat_receipt_id'),
        chatReceiptRoom: text(event, 'chat_receipt_room'),
        chatReceiptSender: text(event, 'chat_receipt_sender'),
        destinationServers: text(event, 'destination_servers'),
        radioRssi: text(event, 'radio_rssi'),
        radioGps: text(event, 'radio_gps'),
        geofenceElevation: text(event, 'geofence_elevation'),
        geofenceMax: text(event, 'geofence_max'),
        geofenceMin: text(event, 'geofence_min'),
        geofenceMonitor: text(event, 'geofence_monitor'),
        geofenceSphere: text(event, 'geofence_sphere'),
        geofenceTracking: text(event, 'geofence_tracking'),
        geofenceTrigger: text(event, 'geofence_trigger'),
        attachmentsCount: text(event, 'attachments_count'),
        takcontrol: text(event, 'takcontrol'),
        takcontrolRequestVersion: text(event, 'takcontrol_request_version'),
        takcontrolResponseStatus: text(event, 'takcontrol_response_status'),
        takcontrolSupportVersion: text(event, 'takcontrol_support_version'),
        routeType: text(event, 'route_type'),
        routePlanning: text(event, 'route_planning'),
        routeMethod: text(event, 'route_method'),
        routeDirection: text(event, 'route_direction'),
        routeOrder: text(event, 'route_order'),
        precisionAltsrc: text(event, 'precision_altsrc'),
        precisionGeopointsrc: text(event, 'precision_geopointsrc'),
        precisionHdop: text(event, 'precision_hdop'),
        precisionPdop: text(event, 'precision_pdop'),
        precisionVdop: text(event, 'precision_vdop'),
        sensorAzimuth: text(event, 'sensor_azimuth'),
        sensorElevation: text(event, 'sensor_elevation'),
        sensorFov: text(event, 'sensor_fov'),
        sensorModel: text(event, 'sensor_model'),
        sensorRange: text(event, 'sensor_range'),
        sensorRoll: text(event, 'sensor_roll'),
        sensorVfov: text(event, 'sensor_vfov'),
        statusBattery: text(event, 'status_battery'),
        statusReadiness: text(event, 'status_readiness'),
        takvDevice: text(event, 'takv_device'),
        takvOs: text(event, 'takv_os'),
        takvPlatform: text(event, 'takv_platform'),
        takvVersion: text(event, 'takv_version'),
        trackSlope: text(event, 'track_slope'),
        uidExtraDroid: text(event, 'uid_extra_droid'),
        usericonIconsetpath: text(event, 'usericon_iconsetpath'),
        videoConnAddress: text(event, 'video_conn_address'),
        videoConnPath: text(event, 'video_conn_path'),
        videoConnPort: text(event, 'video_conn_port'),
        videoConnProtocol: text(event, 'video_conn_protocol'),
        videoUid: text(event, 'video_uid'),
        videoUrl: text(event, 'video_url'),
    };
}

/**
 * The processing path, in the order the event wrote it.
 *
 * An ordered array rather than a map on both sides, because the ordering IS the
 * path and a map would have been re-sorted by its keys on the way through JSON.
 */
function readFlow(event: Record<string, unknown>): CotFlowHop[] {
    const raw = event.flow;
    if (!Array.isArray(raw)) {
        return [];
    }

    const hops: CotFlowHop[] = [];
    for (const entry of raw) {
        const hop = record(entry);
        if (hop === null) {
            continue;
        }

        const system = typeof hop.system === 'string' ? hop.system : '';
        if (system === '') {
            continue;
        }

        hops.push({system, time: typeof hop.time === 'string' ? hop.time : ''});
    }

    return hops;
}

/**
 * The shape the event describes, or null when it describes none.
 *
 * A vertex is dropped rather than coerced when either half is not a finite
 * number, and a shape left with fewer than two is not a shape: the server
 * already refused those, and this is the same refusal on the side that draws.
 */
function readGeometry(event: Record<string, unknown>): CotGeometry | null {
    const raw = record(event.geometry);
    if (raw === null) {
        return null;
    }

    const kind = text(raw, 'kind');
    if (kind === '') {
        return null;
    }

    // One bad vertex refuses the whole shape, which is the rule the server
    // states and applies: a polygon missing a corner is a different polygon.
    // Dropping the corner here drew that different polygon as fact.
    const points: CotVertex[] = [];
    let usable = true;
    if (Array.isArray(raw.points)) {
        for (const entry of raw.points.slice(0, MAX_VERTICES)) {
            const vertex = record(entry);
            const lat = vertex === null ? NaN : Number(text(vertex, 'lat'));
            const lon = vertex === null ? NaN : Number(text(vertex, 'lon'));

            if (!Number.isFinite(lat) || !Number.isFinite(lon) ||
                Math.abs(lat) > 90 || Math.abs(lon) > 180) {
                usable = false;
                break;
            }
            points.push({lat, lon});
        }
    }

    return {
        kind,
        closed: text(raw, 'closed') !== '',
        points: usable && points.length >= 2 ? points : [],
        count: text(raw, 'count'),
        major: text(raw, 'major'),
        minor: text(raw, 'minor'),
        angle: text(raw, 'angle'),
        majorMeters: Number(text(raw, 'major_m')),
        minorMeters: Number(text(raw, 'minor_m')),
        angleDegrees: Number(text(raw, 'angle_deg')),
        note: text(raw, 'note'),
    };
}

function readChecklist(event: Record<string, unknown>): CotChecklist | null {
    const raw = record(event.checklist);
    if (raw === null) {
        return null;
    }

    const kinds: CotChecklistKind[] = [];
    if (Array.isArray(raw.kinds)) {
        for (const entry of raw.kinds.slice(0, MAX_CHECKLIST_KINDS)) {
            const kind = record(entry);
            if (kind === null) {
                continue;
            }

            const name = text(kind, 'name');
            if (name === '') {
                continue;
            }

            kinds.push({name, count: text(kind, 'count')});
        }
    }

    return {count: text(raw, 'count'), kinds};
}

/** Matches cot.maxChecklistKinds. A forged blob is not a trusted input either. */
const MAX_CHECKLIST_KINDS = 8;

/** Matches cot.MaxVertices. A forged blob is not a trusted input either. */
const MAX_VERTICES = 512;

/**
 * The colour the EVENT stated, and never this plugin's own.
 *
 * Re-validated here even though Go already validated it, because a props blob
 * is not a trusted input either: the post type is forgeable and props under a
 * plugin's key are not protected. This is the only author-derived value in the
 * bundle that reaches a style property, and React sets style values through
 * setProperty without sanitising them.
 */
const HEX_COLOR = /^#[0-9a-f]{6}$/i;

export function statedColor(event: CotEvent): string | undefined {
    return HEX_COLOR.test(event.detail.colorArgb) ? event.detail.colorArgb : undefined;
}

/**
 * What colour an affiliation is drawn in, or undefined for one this build does
 * not colour.
 *
 * Read by the dot beside the callsign AND by the map marker, so the two cannot
 * disagree about what a track is. Colour is never the only channel: the type
 * label always begins with the affiliation word wherever this returns a colour,
 * and the map states it in its accessible label.
 */
export const AFFILIATION_COLORS: Record<string, string> = {
    friend: '#3d85c6',
    'assumed-friend': '#3d85c6',
    hostile: '#c0392b',
    suspect: '#c0392b',
    neutral: '#3c8f3c',
    unknown: '#8a6d00',
    pending: '#8a6d00',
};

/**
 * What to CALL an affiliation, for the surfaces that cannot use its colour.
 *
 * Every affiliation the SERVER can decode, which is a wider set than
 * AFFILIATION_COLORS: this table names all eleven, and only some of them earn a
 * colour. `TestWebappAffiliationWordsMatch` holds it to the Go table.
 *
 * The wider set is the point. The four the colours leave out (joker, faker,
 * none, other) were falling through to `unstated`, so an event whose
 * affiliation this build was holding in a string was described as though
 * nothing were known about it, on the one surface where the word is the whole
 * channel. Saying "unstated" about a value we have is the ignorance the card
 * refuses to claim everywhere else.
 */
const AFFILIATION_WORDS: Record<string, string> = {
    friend: 'friendly',
    'assumed-friend': 'assumed friendly',
    hostile: 'hostile',
    suspect: 'suspect',
    neutral: 'neutral',
    unknown: 'unknown',
    pending: 'pending',
    joker: 'joker',
    faker: 'faker',
    none: 'unaffiliated',
    other: 'other',
};

/** The affiliation as a word, or 'unstated' when this build does not name it. */
export function affiliationWord(event: CotEvent): string {
    return Object.hasOwn(AFFILIATION_WORDS, event.affiliation) ? AFFILIATION_WORDS[event.affiliation] : 'unstated';
}

export function affiliationColor(event: CotEvent): string | undefined {
    return Object.hasOwn(AFFILIATION_COLORS, event.affiliation) ? AFFILIATION_COLORS[event.affiliation] : undefined;
}

export function isLinkable(event: CotEvent): boolean {
    return event.format !== '' && event.value !== '';
}

export function accuracyMeters(event: CotEvent): number | undefined {
    if (event.ceMeters === '') {
        return undefined;
    }

    const meters = Number(event.ceMeters);
    if (!Number.isFinite(meters) || meters <= 0) {
        return undefined;
    }

    return meters;
}

/**
 * How long the event says it is good for, from its own two timestamps.
 *
 * Both come from the event, so this reads the same on every machine.
 */
export function validFor(event: CotEvent): string {
    return spanBetween(event.timeAt, event.staleAt);
}

/**
 * How long after the post was written the event went stale.
 *
 * Both halves are server-side values, so the answer is the same on every
 * machine. Nothing here reads the reader's clock: a workstation twenty minutes
 * out would otherwise report a live track as expired.
 */
export function staleAfterPosting(event: CotEvent, createAt: number): string {
    const staleAt = Number(event.staleAt);
    if (!Number.isFinite(staleAt) || staleAt <= 0 || !Number.isFinite(createAt) || createAt <= 0) {
        return '';
    }

    const seconds = Math.round((staleAt - createAt) / 1000);
    if (seconds <= 0) {
        return 'already stale when it was posted';
    }

    return `stale ${compactDuration(seconds)} after posting`;
}

function spanBetween(fromMillis: string, toMillis: string): string {
    const from = Number(fromMillis);
    const to = Number(toMillis);
    if (!Number.isFinite(from) || !Number.isFinite(to) || from <= 0 || to <= from) {
        return '';
    }

    return compactDuration(Math.round((to - from) / 1000));
}

function compactDuration(seconds: number): string {
    const days = Math.floor(seconds / 86400);
    const hours = Math.floor((seconds % 86400) / 3600);
    const minutes = Math.floor((seconds % 3600) / 60);
    const remainder = seconds % 60;

    const parts: string[] = [];
    if (days > 0) {
        parts.push(`${days}d`);
    }
    if (hours > 0) {
        parts.push(`${hours}h`);
    }
    if (minutes > 0) {
        parts.push(`${minutes}m`);
    }
    if (remainder > 0 && days === 0 && hours === 0) {
        parts.push(`${remainder}s`);
    }

    return parts.join(' ');
}
