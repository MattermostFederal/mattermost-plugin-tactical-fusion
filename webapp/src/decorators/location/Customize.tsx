import React from 'react';

import {INLINE_ID, MAP_ID, ROWS} from './rows';
import type {HideableID} from './rows';

import {useFeatures} from '../../features/store';
import HideableEditor from '../../preferences/HideableEditor';
import type {Hideable} from '../../preferences/HideableEditor';
import {usePreferences} from '../../preferences/store';

interface Props {
    onClose: () => void;
}

const ALL_HIDDEN = 'Every row is hidden, so the panel will show nothing but the note at the ' +
    'bottom. That is allowed, and this link is how you get back.';

const FOOTER = 'These apply inside Mattermost. The page a link opens outside it has no ' +
    'reader to ask, so it always shows every row.';

/**
 * The location panel's editor: which rows a reader wants to see.
 *
 * The two maps are hideable and are not rows, so they carry their own ids and
 * are offered only while the admin has each surface on.
 */
const Customize: React.FC<Props> = ({onClose}) => {
    const {preferences} = usePreferences();
    const {features} = useFeatures();

    const items: Array<Hideable<HideableID>> = [
        ...(features.mapPanel ? [{
            id: MAP_ID as HideableID,
            label: 'Map',
            hint: 'A small world map showing where the coordinate is',
        }] : []),
        ...(features.mapInline ? [{
            id: INLINE_ID as HideableID,
            label: 'Map under the post',
            hint: 'Drawn in the channel when a message is only a coordinate',
        }] : []),
        ...ROWS.map((row) => ({id: row.id as HideableID, label: row.label, hint: row.hint})),
    ];

    return (
        <HideableEditor
            section='location'
            items={items}
            stored={preferences.location.hiddenRows}
            build={(hiddenRows) => ({hiddenRows})}
            legend='Rows to show'
            legendId='tf-location-rows-legend'
            allHiddenWarning={ALL_HIDDEN}
            footerHint={FOOTER}
            onClose={onClose}
        />
    );
};

export default Customize;
