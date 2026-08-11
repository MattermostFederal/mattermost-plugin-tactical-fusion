import manifest from 'manifest';
import React, {useEffect, useState} from 'react';

import {get} from '../../decorators/registry';
import type {Selection} from '../../decorators/selection';
import {getSelection, subscribe} from '../../decorators/selection';

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
        <p style={styles.emptyLead}>{'Mission Context'}</p>
        <p style={styles.emptyHint}>
            {'Highlighted values in a message, such as date-time groups, open their details here.'}
        </p>
        <p style={styles.version}>{`Version ${manifest.version}`}</p>
    </div>
);

/**
 * Renders the selected decorator's panel.
 *
 * A registry lookup rather than a branch per decorator, so adding one needs no
 * change here. An unknown type falls back to the empty state, which is what a
 * stale link or an older bundle would produce.
 */
export const RhsView: React.FC = () => {
    const selection = useSelection();
    const decorator = selection ? get(selection.type) : undefined;

    if (!selection || !decorator) {
        return (
            <div style={styles.container}>
                <EmptyState/>
            </div>
        );
    }

    const {Panel} = decorator;
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
    const decorator = selection ? get(selection.type) : undefined;

    if (!selection || !decorator) {
        return <span>{'Mission Context'}</span>;
    }

    const {Title} = decorator;
    if (Title) {
        return <span><Title payload={selection.payload}/></span>;
    }

    return <span>{decorator.summary(selection.payload)}</span>;
};

export default RhsView;
