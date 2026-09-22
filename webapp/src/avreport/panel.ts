import ReportPanel, {PANEL_TITLE} from './ReportPanel';
import type {ReportPayload} from './types';
import {AVREPORT_PANEL_TYPE} from './types';

import {openRhs, setSelection} from '../decorators/selection';
import {getPanel, registerPanel} from '../panels';

export function registerReportPanel(): void {
    if (getPanel(AVREPORT_PANEL_TYPE)) {
        return;
    }

    registerPanel(AVREPORT_PANEL_TYPE, {
        Panel: ReportPanel,
        summary: () => PANEL_TITLE,
    });
}

export function showReport(payload: ReportPayload): void {
    setSelection({type: AVREPORT_PANEL_TYPE, payload});
    openRhs();
}
