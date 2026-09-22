import type {ReportLinkPayload} from '../../avreport/client';
import ReportHover from '../../avreport/ReportHover';
import {REPORT_COLOR} from '../../avreport/ReportMap';
import {PANEL_TITLE, ReportLinkPanel} from '../../avreport/ReportPanel';
import type {Decorator} from '../types';

export const MAX_SOURCE_RUNES = 2048;

const INSTANT = /^-?\d{1,17}$/;

export function fromParams(params: URLSearchParams): ReportLinkPayload | null {
    const v = params.get('v');
    const t = params.get('t');
    if (v === null || t === null || v === '' || !INSTANT.test(t)) {
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

    summary: () => PANEL_TITLE,

    style: {color: REPORT_COLOR, background: 'rgba(46, 125, 154, 0.12)'},

    Panel: ReportLinkPanel,

    Hover: ReportHover,
};

export default decorator;
