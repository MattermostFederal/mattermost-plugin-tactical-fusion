export const AVREPORT_POST_TYPE = 'custom_tf_avreport';

export const AVREPORT_PROPS_KEY = 'tactical_fusion_avreport';

export const AVREPORT_PROPS_VERSION = 1;

export const AVREPORT_PANEL_TYPE = 'avreport-post';

export const SOURCE_MESSAGE = 'message';

export const SOURCE_FENCE = 'fence';

export const MAX_REPORT_ROWS = 64;

export const MAX_REPORT_PERIODS = 24;

export const MAX_REPORT_UNKNOWN = 64;

export const KINDS = ['METAR', 'SPECI', 'TAF', 'NOTAM'] as const;

export type ReportKind = typeof KINDS[number];

export interface ReportRow {
    label: string;
    value: string;
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
    inferred: boolean;
    summary: string;
    flags: string[];
    rows: ReportRow[];
    periods: ReportPeriod[];
    remarks: ReportRow[];
    unknown: string[];
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

function text(blob: Record<string, unknown>, key: string): string {
    return typeof blob[key] === 'string' ? (blob[key] as string) : '';
}

function flag(blob: Record<string, unknown>, key: string): boolean {
    return blob[key] === true;
}

function strings(blob: Record<string, unknown>, key: string, cap: number): string[] | null {
    const raw = blob[key];
    if (!Array.isArray(raw)) {
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

function readRow(value: unknown): ReportRow | null {
    const raw = record(value);
    if (raw === null) {
        return null;
    }
    const label = text(raw, 'label');
    if (label === '') {
        return null;
    }
    return {label, value: text(raw, 'value')};
}

function rows(blob: Record<string, unknown>, key: string): ReportRow[] | null {
    const raw = blob[key];
    if (!Array.isArray(raw)) {
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
    if (period === '' || periodRows === null) {
        return null;
    }
    return {period, rows: periodRows};
}

function periods(blob: Record<string, unknown>, key: string): ReportPeriod[] | null {
    const raw = blob[key];
    if (!Array.isArray(raw)) {
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
    if (!isKind(kind) || src === '') {
        return null;
    }

    const format = text(blob, 'format');
    const value = text(blob, 'value');
    if ((format === '') !== (value === '')) {
        return null;
    }

    const issuedAt = text(blob, 'issued_at');
    if (!(/^-?\d+$/).test(issuedAt)) {
        return null;
    }

    const flags = strings(blob, 'flags', MAX_REPORT_UNKNOWN);
    const bodyRows = rows(blob, 'rows');
    const bodyPeriods = periods(blob, 'periods');
    const remarks = rows(blob, 'remarks');
    const unknown = strings(blob, 'unknown', MAX_REPORT_UNKNOWN);
    if (flags === null || bodyRows === null || bodyPeriods === null || remarks === null || unknown === null) {
        return null;
    }

    return {
        kind,
        station: text(blob, 'station'),
        stationName: text(blob, 'station_name'),
        issued: text(blob, 'issued'),
        issuedAt,
        inferred: flag(blob, 'inferred'),
        summary: text(blob, 'summary'),
        flags,
        rows: bodyRows,
        periods: bodyPeriods,
        remarks,
        unknown,
        format,
        value,
        region: text(blob, 'region'),
        radiusNm: text(blob, 'radius_nm'),
        src,
    };
}

export function fromProps(props: unknown): ReportPayload | null {
    const outer = record(props);
    if (outer === null) {
        return null;
    }

    const blob = record(outer[AVREPORT_PROPS_KEY]);
    if (blob === null || Number(blob.version) !== AVREPORT_PROPS_VERSION) {
        return null;
    }

    const source = text(blob, 'source');
    if (source !== SOURCE_MESSAGE && source !== SOURCE_FENCE) {
        return null;
    }

    const report = fromWire(blob);
    if (report === null) {
        return null;
    }

    return {
        ...report,
        source,
        lead: text(blob, 'lead'),
        trail: text(blob, 'trail'),
        rowsDropped: text(blob, 'rows_dropped') === '1',
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

export const INFERRED_NOTE = 'month and year taken from the post date';
