import type {Report} from './types';
import {fromWire} from './types';

import type {Answer, AnswerStatus} from '../decorators/cached_client';
import {createCachedClient} from '../decorators/cached_client';
import {pluginBaseUrl} from '../plugin_url';

export interface ReportLinkPayload {
    v: string;
    t: string;
}

export type ReportStatus = AnswerStatus;

export type ReportState = Answer<Report>;

export function readReport(body: unknown, payload: ReportLinkPayload): Report {
    const report = fromWire(body);
    if (report === null) {
        throw new Error('The server did not return a report.');
    }
    if (report.src !== payload.v || report.issuedAt !== payload.t) {
        throw new Error('The server answered about a different report.');
    }
    return report;
}

const client = createCachedClient<ReportLinkPayload, Report>({
    endpoint: (payload) => `${pluginBaseUrl()}/api/v1/avreport?${new URLSearchParams({v: payload.v, t: payload.t}).toString()}`,
    cacheKey: (payload) => `${payload.t}:${payload.v}`,
    read: readReport,
});

export const {request, useAnswer: useReport} = client;

export const {_resetForTesting, _setFetchTimeoutForTesting} = client; // eslint-disable-line @typescript-eslint/naming-convention
