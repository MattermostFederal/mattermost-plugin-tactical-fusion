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

const ALL_HIDDEN = 'Every section is hidden, so the panel will show nothing but the callsign and ' +
    'these links. That is allowed, and this link is how you get back.';

const FOOTER = 'These apply to the sidebar. The card in the channel is unchanged, and so is what ' +
    'is stored in the message.';

/**
 * The Cursor on Target panel's editor: which groups a reader wants to see.
 *
 * Groups rather than rows, which is the one place this differs from the
 * location editor. This panel is the longest in the plugin and its volume comes
 * from whole headings, so a tickbox per reading would have traded one long
 * panel for a longer editor.
 */
const Customize: React.FC<Props> = ({onClose}) => {
    const {preferences} = usePreferences();
    const {features} = useFeatures();

    // The map is not offered while the admin has maps off, because a tickbox
    // that changes nothing a reader can see is worse than an absent one. What
    // is STORED is left alone either way, so switching it back on returns them
    // to the choice they had made.
    const items: Array<Hideable<SectionID>> = SECTIONS.
        filter((section) => section.id !== 'map' || features.mapInline).
        map((section) => ({id: section.id, label: section.label, hint: section.hint}));

    return (
        <HideableEditor
            section='cot'
            items={items}
            stored={preferences.cot.hiddenSections}
            build={(hiddenSections) => ({hiddenSections})}
            legend='Sections to show'
            legendId='tf-cot-sections-legend'
            allHiddenWarning={ALL_HIDDEN}
            footerHint={FOOTER}
            onClose={onClose}
        />
    );
};

export default Customize;
