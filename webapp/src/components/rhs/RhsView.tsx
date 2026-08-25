import manifest from 'manifest';
import React, {useEffect, useState} from 'react';

import type {Selection} from '../../decorators/selection';
import {getSelection, subscribe} from '../../decorators/selection';
import {getPanel} from '../../panels';

const styles: Record<string, React.CSSProperties> = {
    container: {padding: '16px'},
    empty: {color: 'var(--center-channel-color)'},
    emptyLead: {fontSize: '14px', margin: '0 0 8px'},
    emptyHint: {fontSize: '13px', opacity: 0.65, margin: 0},
    version: {fontSize: '12px', opacity: 0.5, marginTop: '24px'},
};

/** Subscribes a component to the current selection. */
function useSelection(): Selection | null {
    const [selection, setSelection] = useState<Selection | null>(getSelection());
    useEffect(() => subscribe(setSelection), []);
    return selection;
}

export const EmptyState: React.FC = () => (
    <div style={styles.empty}>
        <p style={styles.emptyLead}>{'Tactical Fusion'}</p>
        <p style={styles.emptyHint}>
            {'Highlighted values in a message, such as date-time groups and coordinates, open their details here.'}
        </p>
        <p style={styles.version}>{`Version ${manifest.version}`}</p>
    </div>
);

/**
 * Renders the selected panel.
 *
 * A registry lookup rather than a branch per surface, so adding one needs no
 * change here. An unknown type falls back to the empty state, which is what a
 * stale link or an older bundle would produce.
 *
 * The lookup is the panel table rather than the decorator registry, because not
 * everything with a panel is a decorator: Cursor on Target has no token and no
 * link and could never be in that one.
 */
export const RhsView: React.FC = () => {
    const selection = useSelection();
    const entry = selection ? getPanel(selection.type) : undefined;

    if (!selection || !entry) {
        return (
            <div style={styles.container}>
                <EmptyState/>
            </div>
        );
    }

    const {Panel} = entry;
    return (
        <div style={styles.container}>
            <Panel payload={selection.payload}/>
        </div>
    );
};

/**
 * The RHS header. Null-safe: the header button opens with no selection.
 *
 * A decorator whose panel has more than one view can declare a `Title`
 * component to follow it, since this is rendered separately from the body and
 * `summary` cannot see which view the panel is on. Everything else gets
 * `summary`.
 */
export const RhsTitle: React.FC = () => {
    const selection = useSelection();
    const entry = selection ? getPanel(selection.type) : undefined;

    if (!selection || !entry) {
        return <span>{'Tactical Fusion'}</span>;
    }

    const {Title} = entry;
    if (Title) {
        return <span><Title payload={selection.payload}/></span>;
    }

    return <span>{entry.summary(selection.payload)}</span>;
};

export default RhsView;
