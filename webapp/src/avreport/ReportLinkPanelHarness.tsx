import React from 'react';

import {_resetForTesting as resetReports} from './client';
import {HONOLULU_METAR, propsFor} from './report_fixtures';
import ReportHover from './ReportHover';
import {ReportLinkPanel} from './ReportPanel';
import {AVREPORT_PROPS_KEY} from './types';

import {_resetForTesting as resetSelection} from '../decorators/selection';
import {_resetForTesting as resetFeatures} from '../features/store';
import {featuresReply, isFeaturesRequest, setStubbedFeatures} from '../features/stub_fetch';

export type Reply = 'found' | 'unplaced' | 'rejected' | 'failed' | 'hold';

interface Props {
    surface: 'panel' | 'hover';
    reply: Reply;
}

const WIRE = propsFor(HONOLULU_METAR)[AVREPORT_PROPS_KEY] as Record<string, unknown>;

const ReportLinkPanelHarness: React.FC<Props> = ({surface, reply}) => {
    const [ready] = React.useState(() => {
        resetReports();
        resetSelection();
        resetFeatures();

        const real = globalThis.fetch;
        globalThis.fetch = (async (input: RequestInfo | URL, init?: RequestInit) => {
            const url = String(input);
            if (isFeaturesRequest(url)) {
                return featuresReply();
            }
            if (!url.includes('/api/v1/avreport')) {
                return real(input, init);
            }
            if (reply === 'hold') {
                return new Promise<Response>(() => {});
            }
            if (reply === 'failed') {
                throw new Error('offline');
            }
            if (reply === 'rejected') {
                return {status: 400, ok: false} as Response;
            }
            const body = reply === 'unplaced' ? {...WIRE, station_name: '', format: '', value: '', region: ''} : WIRE;
            return {status: 200, ok: true, json: async () => body} as unknown as Response;
        }) as typeof globalThis.fetch;

        return true;
    });

    setStubbedFeatures({mapPanel: false, mapInline: false, mapPage: false});

    if (!ready) {
        return null;
    }

    const payload = {v: HONOLULU_METAR.src, t: HONOLULU_METAR.issuedAt};

    return (
        <div data-testid='harness'>
            {surface === 'panel' ? <ReportLinkPanel payload={payload}/> : <ReportHover payload={payload}/>}
        </div>
    );
};

export default ReportLinkPanelHarness;
