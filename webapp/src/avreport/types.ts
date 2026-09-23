export const AVREPORT_POST_TYPE = 'custom_tf_avreport';

export const AVREPORT_PROPS_KEY = 'tactical_fusion_avreport';

export const AVREPORT_PROPS_VERSION = 1;

export const AVREPORT_PANEL_TYPE = 'avreport-post';

export const SOURCE_MESSAGE = 'message';

export const SOURCE_FENCE = 'fence';

export const MAX_REPORT_ROWS = 64;

export const MAX_REPORT_PERIODS = 24;

export const MAX_REPORT_UNKNOWN = 64;

export const MAX_REPORT_FLAGS = 8;

export const MAX_AREA_POINTS = 64;

export const MIN_AREA_POINTS = 3;

export const RESTRICTION_LABEL = 'Restriction';

export function isRestriction(report: Report): boolean {
    return report.rows.some((row) => row.label === RESTRICTION_LABEL);
}

export const KINDS = ['METAR', 'SPECI', 'TAF', 'NOTAM'] as const;

export type ReportKind = typeof KINDS[number];

export interface ReportRow {
    label: string;
    value: string;
    query?: string;
}

export interface ReportPeriod {
    period: string;
    rows: ReportRow[];
}

export interface Report {
    kind: ReportKind;
    station: string;
    stationName: string;
    issued: string;
    issuedAt: string;
    issuedQuery: string;
    inferred: boolean;
    summary: string;
    flags: string[];
    rows: ReportRow[];
    periods: ReportPeriod[];
    remarks: ReportRow[];
    unknown: string[];
    area: string[];
    format: string;
    value: string;
    region: string;
    radiusNm: string;
    src: string;
}

export interface ReportPayload extends Report {
    source: string;
    lead: string;
    trail: string;
    rowsDropped: boolean;
    postId: string;
}

function record(value: unknown): Record<string, unknown> | null {
    if (value === null || typeof value !== 'object' || Array.isArray(value)) {
        return null;
    }
    return value as Record<string, unknown>;
}

function text(blob: Record<string, unknown>, key: string): string | null {
    const value = blob[key];
    if (value === undefined) {
        return '';
    }
    return typeof value === 'string' ? value : null;
}

function flag(blob: Record<string, unknown>, key: string): boolean {
    return blob[key] === true;
}

function listOf(blob: Record<string, unknown>, key: string): unknown[] | null {
    const raw = Object.hasOwn(blob, key) ? blob[key] : undefined;
    if (raw === undefined || raw === null) {
        return [];
    }
    return Array.isArray(raw) ? raw : null;
}

function strings(blob: Record<string, unknown>, key: string, cap: number): string[] | null {
    const raw = listOf(blob, key);
    if (raw === null) {
        return null;
    }
    const out: string[] = [];
    for (const item of raw.slice(0, cap)) {
        if (typeof item !== 'string') {
            return null;
        }
        out.push(item);
    }
    return out;
}

function readRow(item: unknown): ReportRow | null {
    const raw = record(item);
    if (raw === null) {
        return null;
    }
    const label = text(raw, 'label');
    const value = text(raw, 'value');
    const query = text(raw, 'query');
    if (label === '' || label === null || value === null || query === null) {
        return null;
    }
    return query === '' ? {label, value} : {label, value, query};
}

function rows(blob: Record<string, unknown>, key: string): ReportRow[] | null {
    const raw = listOf(blob, key);
    if (raw === null) {
        return null;
    }
    const out: ReportRow[] = [];
    for (const item of raw.slice(0, MAX_REPORT_ROWS)) {
        const row = readRow(item);
        if (row === null) {
            return null;
        }
        out.push(row);
    }
    return out;
}

function readPeriod(value: unknown): ReportPeriod | null {
    const raw = record(value);
    if (raw === null) {
        return null;
    }
    const period = text(raw, 'period');
    const periodRows = rows(raw, 'rows');
    if (period === '' || period === null || periodRows === null) {
        return null;
    }
    return {period, rows: periodRows};
}

function periods(blob: Record<string, unknown>, key: string): ReportPeriod[] | null {
    const raw = listOf(blob, key);
    if (raw === null) {
        return null;
    }
    const out: ReportPeriod[] = [];
    for (const item of raw.slice(0, MAX_REPORT_PERIODS)) {
        const period = readPeriod(item);
        if (period === null) {
            return null;
        }
        out.push(period);
    }
    return out;
}

function isKind(value: string): value is ReportKind {
    return (KINDS as readonly string[]).includes(value);
}

export function fromWire(body: unknown): Report | null {
    const blob = record(body);
    if (blob === null) {
        return null;
    }

    const kind = text(blob, 'kind');
    const src = text(blob, 'src');
    if (kind === null || !isKind(kind) || src === '' || src === null) {
        return null;
    }

    const format = text(blob, 'format');
    const value = text(blob, 'value');
    if (format === null || value === null || (format === '') !== (value === '')) {
        return null;
    }

    const issuedAt = text(blob, 'issued_at');
    if (issuedAt === null || !(/^-?\d+$/).test(issuedAt)) {
        return null;
    }
    const issuedQuery = text(blob, 'issued_query');
    if (issuedQuery === null) {
        return null;
    }

    const station = text(blob, 'station');
    const stationName = text(blob, 'station_name');
    const issued = text(blob, 'issued');
    const summary = text(blob, 'summary');
    const region = text(blob, 'region');
    const radiusNm = text(blob, 'radius_nm');
    if (station === null || stationName === null || issued === null || summary === null || region === null || radiusNm === null) {
        return null;
    }

    const flags = strings(blob, 'flags', MAX_REPORT_FLAGS);
    const bodyRows = rows(blob, 'rows');
    const bodyPeriods = periods(blob, 'periods');
    const remarks = rows(blob, 'remarks');
    const unknown = strings(blob, 'unknown', MAX_REPORT_UNKNOWN);
    const area = (listOf(blob, 'area')?.length ?? 0) > MAX_AREA_POINTS ? [] : strings(blob, 'area', MAX_AREA_POINTS);
    if (flags === null || bodyRows === null || bodyPeriods === null || remarks === null || unknown === null || area === null) {
        return null;
    }

    return {
        kind,
        station,
        stationName,
        issued,
        issuedAt,
        issuedQuery,
        inferred: flag(blob, 'inferred'),
        summary,
        flags,
        rows: bodyRows,
        periods: bodyPeriods,
        remarks,
        unknown,
        area,
        format,
        value,
        region,
        radiusNm,
        src,
    };
}

export function fromProps(props: unknown): ReportPayload | null {
    const outer = record(props);
    if (outer === null) {
        return null;
    }

    const blob = record(outer[AVREPORT_PROPS_KEY]);
    if (blob === null || typeof blob.version !== 'number' || blob.version !== AVREPORT_PROPS_VERSION) {
        return null;
    }

    const source = text(blob, 'source');
    if (source !== SOURCE_MESSAGE && source !== SOURCE_FENCE) {
        return null;
    }

    const report = fromWire(blob);
    const lead = text(blob, 'lead');
    const trail = text(blob, 'trail');
    const rowsDropped = text(blob, 'rows_dropped');
    if (report === null || lead === null || trail === null || rowsDropped === null) {
        return null;
    }

    return {
        ...report,
        source,
        lead,
        trail,
        rowsDropped: rowsDropped === '1',
        postId: '',
    };
}

export function isPlaced(report: Report): boolean {
    return report.format !== '' && report.value !== '';
}

export function headingOf(report: Report): string {
    return report.station === '' ? report.kind : `${report.kind} ${report.station}`;
}

export function issuedLabel(report: Report): string {
    return report.kind === 'NOTAM' ? 'Effective' : 'Issued';
}

