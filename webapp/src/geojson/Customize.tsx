import React from 'react';

import {SECTIONS} from './sections';
import type {SectionID} from './sections';

import {useFeatures} from '../features/store';
import HideableEditor from '../preferences/HideableEditor';
import type {Hideable} from '../preferences/HideableEditor';
import {usePreferences} from '../preferences/store';

interface Props {
    onClose: () => void;
}

const ALL_HIDDEN = 'Every section is hidden, so the panel will show nothing but the heading and ' +
    'these links. That is allowed, and this link is how you get back.';

const FOOTER = 'These apply to the sidebar. The card in the channel is unchanged, and so is what ' +
    'is stored in the message.';

/**
 * The GeoJSON panel's editor: which groups a reader wants to see.
 *
 * Its own stored list rather than the Cursor on Target one, because the two
 * panels have different sections: hiding "Map" here must not hide it there.
 */
const Customize: React.FC<Props> = ({onClose}) => {
    const {preferences} = usePreferences();
    const {features} = useFeatures();

    // The map is not offered while the admin has maps off, because a tickbox
    // that changes nothing a reader can see is worse than an absent one. What
    // is STORED is left alone either way, so switching it back on returns them
    // to the choice they had made.
    const items: Array<Hideable<SectionID>> = SECTIONS.
        filter((section) => section.id !== 'map' || features.mapPanel).
        map((section) => ({id: section.id, label: section.label, hint: section.hint}));

    return (
        <HideableEditor
            section='geojson'
            items={items}
            stored={preferences.geojson.hiddenSections}
            build={(hiddenSections) => ({hiddenSections})}
            legend='Sections to show'
            legendId='tf-geojson-sections-legend'
            allHiddenWarning={ALL_HIDDEN}
            footerHint={FOOTER}
            onClose={onClose}
        />
    );
};

export default Customize;
