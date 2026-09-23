import ReportPanel from './ReportPanel';
import type {ReportPayload} from './types';
import {AVREPORT_PANEL_TYPE, headingOf} from './types';

import {openRhs, setSelection} from '../decorators/selection';
import {getPanel, registerPanel} from '../panels';

export function registerReportPanel(): void {
    if (getPanel(AVREPORT_PANEL_TYPE)) {
        return;
    }

    registerPanel(AVREPORT_PANEL_TYPE, {
        Panel: ReportPanel,
        summary: headingOf,
    });
}

export function showReport(payload: ReportPayload): void {
    setSelection({type: AVREPORT_PANEL_TYPE, payload});
    openRhs();
}
