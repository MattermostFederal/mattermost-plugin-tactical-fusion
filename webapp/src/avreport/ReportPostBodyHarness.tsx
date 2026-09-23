import React from 'react';

import {HONOLULU_METAR, propsFor} from './report_fixtures';
import ReportPostBody from './ReportPostBody';
import type {ReportPayload} from './types';

import type {Selection} from '../decorators/selection';
import {_resetForTesting as resetSelection, subscribe} from '../decorators/selection';
import {_resetForTesting as resetFeatures} from '../features/store';
import {_resetForTesting as resetPreferences} from '../preferences/store';

interface Props {
    payload?: ReportPayload | null;
    message?: string;
    version?: number;
    editAt?: number;
    compactDisplay?: boolean;
}

const ReportPostBodyHarness: React.FC<Props> = ({
    payload = HONOLULU_METAR,
    message,
    version = 1,
    editAt = 0,
    compactDisplay,
}) => {
    const [ready] = React.useState(() => {
        resetSelection();
        resetFeatures();
        resetPreferences();
        return true;
    });

    const [selection, setSelection] = React.useState<Selection | null>(null);
    React.useEffect(() => subscribe(setSelection), []);

    if (!ready) {
        return null;
    }

    const props = payload === null ? undefined : propsFor(payload, version);
    const text = message ?? '```metar\n' + (payload?.src ?? '') + '\n```';

    return (
        <div>
            <ReportPostBody
                post={{id: 'post0000000000000000000000', message: text, props, edit_at: editAt}}
                compactDisplay={compactDisplay}
            />
            <p data-testid='selection'>{selection ? `${selection.type} ${JSON.stringify(selection.payload).slice(0, 40)}` : ''}</p>
        </div>
    );
};

export default ReportPostBodyHarness;
