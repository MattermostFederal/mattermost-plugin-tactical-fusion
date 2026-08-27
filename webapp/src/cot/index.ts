import CotPanel, {CotTitle, PANEL_TITLE} from './CotPanel';
import type {CotPayload} from './types';
import {COT_PANEL_TYPE} from './types';

import {openRhs, setSelection} from '../decorators/selection';
import {getPanel, registerPanel} from '../panels';

/**
 * Registers the Cursor on Target sidebar panel.
 *
 * Idempotent, for the reason `registerBuiltinDecorators` is: the table lives in
 * module state that survives a plugin re-registration while `initialize()` runs
 * again, and throwing on the second pass would leave the sidebar dead until a
 * page reload.
 */
export function registerCotPanel(): void {
    if (getPanel(COT_PANEL_TYPE)) {
        return;
    }

    registerPanel(COT_PANEL_TYPE, {
        Panel: CotPanel,
        Title: CotTitle,
        summary: () => PANEL_TITLE,
    });
}

/** Opens the sidebar on one event. */
export function showCotEvent(payload: CotPayload): void {
    setSelection({type: COT_PANEL_TYPE, payload});
    openRhs();
}
