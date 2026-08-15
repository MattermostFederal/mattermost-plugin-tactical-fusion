import React, {useEffect, useState} from 'react';

import {MAP_ID, ROWS} from './rows';
import type {HideableID} from './rows';

import LinkButton from '../../components/LinkButton';
import {resetPreferencesSection, savePreferencesSection, usePreferences} from '../../preferences/store';

const styles: Record<string, React.CSSProperties> = {
    root: {color: 'var(--center-channel-color)'},

    // No heading below it: the sidebar header already says where this is.
    back: {fontSize: '13px'},
    section: {marginTop: '16px'},
    legend: {
        fontSize: '11px',
        textTransform: 'uppercase',
        letterSpacing: '0.04em',
        fontWeight: 600,
        opacity: 0.7,
        margin: '0 0 8px',
    },
    row: {
        display: 'flex',
        alignItems: 'baseline',
        gap: '8px',
        padding: '5px 0',
        fontSize: '13px',
        cursor: 'pointer',
    },
    label: {flex: 1, minWidth: 0},
    hint: {fontSize: '12px', opacity: 0.6, margin: '6px 0 0'},
    rowHint: {display: 'block', fontSize: '11px', opacity: 0.6, marginTop: '1px'},
    actions: {display: 'flex', gap: '8px', marginTop: '16px'},
    save: {
        border: 'none',
        background: 'var(--button-bg, #1c58d9)',
        color: 'var(--button-color, #ffffff)',
        borderRadius: '4px',
        cursor: 'pointer',
        fontSize: '13px',
        fontWeight: 600,
        padding: '8px 14px',
    },
    reset: {
        border: '1px solid rgba(var(--center-channel-color-rgb), 0.24)',
        background: 'transparent',
        color: 'var(--center-channel-color)',
        borderRadius: '4px',
        cursor: 'pointer',
        fontSize: '13px',
        fontWeight: 600,
        padding: '8px 14px',
    },
    status: {fontSize: '12px', margin: '10px 0 0', opacity: 0.75},
    error: {fontSize: '12px', margin: '10px 0 0', color: 'var(--error-text, #d24b4e)'},
};

interface Props {
    onClose: () => void;
}

/**
 * The location panel's editor: which rows a reader wants to see.
 *
 * Presented as what to SHOW while stored as what to hide, and the two
 * directions are not interchangeable. Storing the hidden ones means an empty
 * list is "everything", so a reader who never chose is stored as nothing at
 * all, which is what lets "Restore defaults" be a delete; and a row added in a
 * later version appears for everybody rather than being invisible to exactly
 * the readers who cared enough to choose. A checkbox reading "hide this" would
 * be the honest mirror of the storage and the wrong thing to ask a person.
 *
 * It takes the panel over rather than expanding below the table, matching the
 * DTG editor, so the coordinate the reader opened the sidebar for is not buried
 * under a list of settings.
 */
const Customize: React.FC<Props> = ({onClose}) => {
    const {preferences, loading, error: loadError} = usePreferences();

    const [hidden, setHidden] = useState<HideableID[]>(() => [...preferences.location.hiddenRows]);
    const [busy, setBusy] = useState(false);
    const [status, setStatus] = useState<string | null>(null);
    const [error, setError] = useState<string | null>(null);

    // Whether the reader has touched the form, which is the only thing that can
    // tell their empty list from the store's.
    const [touched, setTouched] = useState(false);

    // The stored settings arrive after the first render, so the form has to
    // pick them up when they land, but never over the top of an edit.
    //
    // The `touched` flag is doing that work, and the version without it was
    // wrong in a way that read as correct: it keyed the effect on
    // `preferences.location.hiddenRows` and treated that as a value. It is not.
    // `asRowIDs` builds a fresh array on every read, so the identity changes on
    // every completed request and the effect fired whether or not anything had.
    //
    // The window is wider than the first load. A failed load leaves the store
    // unloaded on purpose, so opening this editor afterwards starts a fresh GET
    // while the reader is already looking at the form, and every box ticked
    // during that round trip was silently reverted when it landed.
    useEffect(() => {
        if (touched) {
            return;
        }
        setHidden([...preferences.location.hiddenRows]);
    }, [preferences.location.hiddenRows, touched]);

    const toggle = (id: HideableID) => {
        setStatus(null);
        setError(null);
        setTouched(true);
        setHidden((current) => (current.includes(id) ? current.filter((other) => other !== id) : [...current, id]));
    };

    const onSave = async () => {
        setBusy(true);
        setStatus(null);
        setError(null);
        try {
            // Section-scoped, so this cannot touch the reader's timezone table.
            // The store re-reads before writing rather than spreading the
            // cached blob, which is as stale as the tab is old.
            await savePreferencesSection('location', {hiddenRows: hidden});
            setStatus('Saved.');

            // Straight back to the coordinate, where the changed table is the
            // receipt. A save that failed stays put instead, with the reason on
            // screen, because closing would throw away both the message and the
            // reader's edits.
            onClose();
        } catch (err) {
            setError(err instanceof Error ? err.message : 'Could not save your settings.');
        } finally {
            setBusy(false);
        }
    };

    const onReset = async () => {
        setBusy(true);
        setStatus(null);
        setError(null);
        try {
            await resetPreferencesSection('location');
            setStatus('Defaults restored.');
            onClose();
        } catch (err) {
            setError(err instanceof Error ? err.message : 'Could not restore the defaults.');
        } finally {
            setBusy(false);
        }
    };

    // Counted over what this editor actually offers, not over ROWS alone. The
    // map is hideable and is not a row, so subtracting the hidden list from
    // ROWS.length went negative once everything was hidden and silenced the
    // warning in exactly the case it exists for.
    const offered: HideableID[] = [MAP_ID, ...ROWS.map((row) => row.id)];
    const shown = offered.filter((id) => !hidden.includes(id)).length;

    return (
        <div style={styles.root}>
            <LinkButton
                style={styles.back}
                disabled={busy}
                onClick={onClose}
            >{'← Back'}</LinkButton>

            <div style={styles.section}>
                {/*
                  * A group with a name, not eleven loose checkboxes. Without
                  * it a screen reader announces each one with no idea what the
                  * set is for. A fieldset would be the semantic choice, but
                  * everything here is styled inline and a fieldset carries
                  * browser chrome this panel does not want.
                  */}
                <p
                    id='tf-location-rows-legend'
                    style={styles.legend}
                >{'Rows to show'}</p>

                <div
                    role='group'
                    aria-labelledby='tf-location-rows-legend'
                >
                    <label style={styles.row}>
                        <input
                            type='checkbox'
                            checked={!hidden.includes(MAP_ID)}
                            disabled={busy || loading}
                            onChange={() => toggle(MAP_ID)}
                        />
                        <span style={styles.label}>
                            {'Map'}
                            <span style={styles.rowHint}>{'A small world map showing where the coordinate is'}</span>
                        </span>
                    </label>
                    {ROWS.map((row) => (
                        <label
                            key={row.id}
                            style={styles.row}
                        >
                            <input
                                type='checkbox'
                                checked={!hidden.includes(row.id)}
                                disabled={busy || loading}
                                onChange={() => toggle(row.id)}
                            />
                            <span style={styles.label}>
                                {row.label}
                                {row.hint && <span style={styles.rowHint}>{row.hint}</span>}
                            </span>
                        </label>
                    ))}
                </div>

                {/*
                  * Announced, because the reader who just unchecked the last
                  * row is the one who most needs to hear it and is the least
                  * likely to see it.
                  */}
                <p
                    style={styles.hint}
                    role='status'
                    aria-live='polite'
                >
                    {shown === 0 ? 'Every row is hidden, so the panel will show nothing but the note at the ' +
                        'bottom. That is allowed, and this link is how you get back.' : ''}
                </p>
                <p style={styles.hint}>
                    {'This applies to the sidebar only. The page a link opens outside Mattermost ' +
                        'has no reader to ask, so it always shows every row.'}
                </p>
            </div>

            <div style={styles.actions}>
                <button
                    type='button'
                    style={styles.save}
                    disabled={busy || loading}
                    onClick={onSave}
                >{'Save'}</button>
                <button
                    type='button'
                    style={styles.reset}
                    disabled={busy || loading}
                    onClick={onReset}
                >{'Restore defaults'}</button>
            </div>

            <p
                style={error || loadError ? styles.error : styles.status}
                role='status'
                aria-live='polite'
            >{error ?? loadError ?? status ?? ''}</p>
        </div>
    );
};

export default Customize;
