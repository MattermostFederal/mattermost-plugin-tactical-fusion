import React, {useState} from 'react';

import CotPostBody from './CotPostBody';
import {COT_PROPS_KEY, COT_PROPS_VERSION} from './types';

import {registerBuiltinDecorators} from '../decorators/index';
import {_resetForTesting as resetDecorators} from '../decorators/registry';

interface Props {

    /** Null for a post carrying no blob at all, which is the props-loss case. */
    uid?: string | null;

    version?: number;
    source?: string;
    lead?: string;
    trail?: string;
    src?: string;
    fileId?: string;
    fileName?: string;

    message?: string;
    editAt?: number;
    createAt?: number;
    fileIds?: string[];

    event?: Record<string, string>;

    /** Several events, for the block case. Overrides `event` when given. */
    events?: Array<Record<string, string>>;
    compactDisplay?: boolean;
}

/**
 * Harness for the Cursor on Target post body.
 *
 * The map is deliberately never reachable here: every fixture leaves the
 * position unlinkable, so `CotCard` renders no `CotMap` and the component tests
 * stay free of the feature store, the preference store and WebGL. The map has
 * its own tests on `LocationMap`.
 */
const CotPostBodyHarness: React.FC<Props> = ({
    uid = 'ANDROID-1',
    version = COT_PROPS_VERSION,
    source = 'fence',
    lead = '',
    trail = '',
    src = '<event uid="ANDROID-1"/>',
    fileId = '',
    fileName = '',
    message = '',
    editAt = 0,
    createAt = 0,
    fileIds = [],
    event = {},
    events,
    compactDisplay,
}) => {
    // The hover routes through the decorator registry, which `initialize()`
    // populates in the running app and nothing populates here.
    useState(() => {
        resetDecorators();
        registerBuiltinDecorators();
        return true;
    });

    let postProps: Record<string, unknown> | undefined;
    if (uid !== null) {
        postProps = {
            [COT_PROPS_KEY]: {
                version,
                source,
                lead,
                trail,
                src,
                file_id: fileId,
                file_name: fileName,
                events: events ?
                    events.map((e, i) => ({uid: `${uid}-${i}`, cot_type: 'a-f-G-U-C', type_label: 'Friendly Ground', ...e})) :
                    [{uid, cot_type: 'a-f-G-U-C', type_label: 'Friendly Ground', ...event}],
            },
        };
    }

    return (
        <div data-testid='cot-body'>
            <CotPostBody
                post={{
                    message,
                    props: postProps,
                    edit_at: editAt,
                    create_at: createAt,
                    file_ids: fileIds,
                }}
                compactDisplay={compactDisplay}
            />
        </div>
    );
};

export default CotPostBodyHarness;
