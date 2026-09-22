import type {ReportLinkPayload} from '../../avreport/client';
import ReportHover from '../../avreport/ReportHover';
import {REPORT_COLOR} from '../../avreport/ReportMap';
import {ReportLinkPanel, ReportLinkTitle} from '../../avreport/ReportPanel';
import {headingFromSource} from '../../avreport/title';
import type {Decorator} from '../types';

export const MAX_SOURCE_RUNES = 2048;

export const MIN_INSTANT_MS = 0;
export const MAX_INSTANT_MS = 7258118400000;

const INSTANT = /^-?\d{1,17}$/;

export function fromParams(params: URLSearchParams): ReportLinkPayload | null {
    const v = params.get('v');
    const t = params.get('t');
    if (v === null || t === null || v === '' || !INSTANT.test(t)) {
        return null;
    }
    const millis = Number(t);
    if (!Number.isSafeInteger(millis) || millis < MIN_INSTANT_MS || millis > MAX_INSTANT_MS) {
        return null;
    }
    if ([...v].length > MAX_SOURCE_RUNES) {
        return null;
    }

    return {v, t};
}

const decorator: Decorator<ReportLinkPayload> = {
    type: 'avreport',
    fromParams,

    summary: (payload) => headingFromSource(payload.v),

    style: {color: REPORT_COLOR, background: 'rgba(46, 125, 154, 0.12)'},

    Panel: ReportLinkPanel,

    Title: ReportLinkTitle,

    Hover: ReportHover,
};

export default decorator;
