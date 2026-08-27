import React, {useEffect, useRef, useState} from 'react';

import {resetPreferencesSection, savePreferencesSection, usePreferences} from './store';
import type {Preferences} from './types';

import LinkButton from '../components/LinkButton';

const styles: Record<string, React.CSSProperties> = {
    root: {color: 'var(--center-channel-color)'},
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
    row: {padding: '5px 0', fontSize: '13px'},
    control: {
        display: 'flex',
        alignItems: 'baseline',
        gap: '8px',
        cursor: 'pointer',
    },
    label: {flex: 1, minWidth: 0},
    hint: {fontSize: '12px', opacity: 0.75, margin: '6px 0 0'},
    rowHint: {display: 'block', fontSize: '11px', opacity: 0.75, marginTop: '1px', marginLeft: '22px'},
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
    error: {fontSize: '12px', margin: '10px 0 0', fontWeight: 600, color: 'var(--error-text, #c9393c)'},
};

export interface Hideable<T extends string> {
    id: T;
    label: string;
    hint?: string;
}

interface Props<T extends string, K extends keyof Preferences> {

    /** Which decorator's settings this writes. Never a whole blob. */
    section: K;

    /** Everything this editor offers, already filtered by the admin switches. */
    items: ReadonlyArray<Hideable<T>>;

    /** What the reader has hidden, from the store. */
    stored: readonly T[];

    /** Wraps the hidden list in the shape the section stores. */
    build: (hidden: T[]) => Preferences[K];

    legend: string;
    legendId: string;
    allHiddenWarning: string;
    footerHint: string;
    onClose: () => void;
}

/**
 * The editor behind every "Customize your view" link that hides things.
 *
 * One component rather than one per decorator, because what differs between
 * them is a catalog and four strings while what is the same is every property
 * that took a defect to get right: the `touched` guard, the seal on an
 * unread blob, the section-scoped save, and the announced warning. Those had
 * been copied twice and fixed at different times in each copy.
 *
 * Presented as what to SHOW while stored as what to hide. Storing the hidden
 * ones means an empty list is "everything", so a reader who never chose is
 * stored as nothing at all, which is what lets "Restore defaults" be a delete;
 * and an item added in a later version appears for everybody rather than being
 * invisible to exactly the readers who cared enough to choose.
 */
export function HideableEditor<T extends string, K extends keyof Preferences>({
    section, items, stored, build, legend, legendId, allHiddenWarning, footerHint, onClose,
}: Props<T, K>) {
    const {loading, error: loadError, loaded} = usePreferences();

    const [hidden, setHidden] = useState<T[]>(() => [...stored]);
    const [busy, setBusy] = useState(false);
    const [status, setStatus] = useState<string | null>(null);
    const [error, setError] = useState<string | null>(null);

    const [touched, setTouched] = useState(false);

    const backRef = useRef<HTMLButtonElement>(null);
    const saveRef = useRef<HTMLButtonElement>(null);

    // The editor replaces the whole panel, so the control that opened it is
    // gone and focus would fall to the body: a keyboard reader is dropped at
    // the top of the document with nothing announced. Land them on Back, which
    // is both the first control and the way out.
    useEffect(() => {
        backRef.current?.focus();
    }, []);

    // The stored settings arrive after the first render, so the form has to
    // pick them up when they land, but never over the top of an edit.
    //
    // Keyed on `stored`, whose identity changes on every completed read because
    // the store rebuilds the array. Without `touched` the effect fires whether
    // or not anything changed, and every box ticked during a round trip was
    // silently reverted when it landed.
    useEffect(() => {
        if (touched) {
            return;
        }
        setHidden([...stored]);
    }, [stored, touched]);

    // Nothing is editable until a read has succeeded. A failed FIRST read
    // degrades to the defaults, which renders as every box ticked: editing that
    // and saving writes a selection derived from settings the reader never had,
    // and a save replaces the whole section.
    const sealed = busy || loading || !loaded;

    const toggle = (id: T) => {
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
            await savePreferencesSection(section, build(hidden));
            setStatus('Saved.');
            onClose();
        } catch (err) {
            setError(err instanceof Error ? err.message : 'Could not save your settings.');

            // Disabling the button while it held focus blurred it to the body,
            // so the reader has nowhere to retry from. The message is announced
            // either way; this is what lets them act on it.
            window.setTimeout(() => saveRef.current?.focus(), 0);
        } finally {
            setBusy(false);
        }
    };

    const onReset = async () => {
        setBusy(true);
        setStatus(null);
        setError(null);
        try {
            await resetPreferencesSection(section);
            setStatus('Defaults restored.');
            onClose();
        } catch (err) {
            setError(err instanceof Error ? err.message : 'Could not restore the defaults.');
            window.setTimeout(() => saveRef.current?.focus(), 0);
        } finally {
            setBusy(false);
        }
    };

    // Counted over what this editor OFFERS rather than over the whole catalog.
    // Counting the catalog made the warning fire while an item the admin had
    // switched off was hidden but not offered, so it appeared over a panel that
    // still had things on screen.
    const shown = items.filter((item) => !hidden.includes(item.id)).length;

    return (
        <div style={styles.root}>
            <LinkButton
                ref={backRef}
                style={styles.back}
                disabled={busy}
                onClick={onClose}
            >{'← Back'}</LinkButton>

            <div style={styles.section}>
                {/*
                  * A group with a name, not a dozen loose checkboxes. Without
                  * it a screen reader announces each one with no idea what the
                  * set is for. A fieldset would be the semantic choice, but
                  * everything here is styled inline and a fieldset carries
                  * browser chrome this panel does not want.
                  */}
                <p
                    id={legendId}
                    style={styles.legend}
                >{legend}</p>

                <div
                    role='group'
                    aria-labelledby={legendId}
                >
                    {/*
                      * The hint sits OUTSIDE the label, which is what makes it
                      * a description rather than part of the name. Inside, it
                      * became the name: every box announced a dozen words of
                      * prose before its own state.
                      */}
                    {items.map((item) => (
                        <div
                            key={item.id}
                            style={styles.row}
                        >
                            <label style={styles.control}>
                                <input
                                    type='checkbox'
                                    checked={!hidden.includes(item.id)}
                                    disabled={sealed}
                                    aria-describedby={item.hint === undefined ? undefined : `${legendId}-${item.id}`}
                                    onChange={() => toggle(item.id)}
                                />
                                <span style={styles.label}>{item.label}</span>
                            </label>
                            {item.hint !== undefined && (
                                <span
                                    id={`${legendId}-${item.id}`}
                                    style={styles.rowHint}
                                >{item.hint}</span>
                            )}
                        </div>
                    ))}
                </div>

                {/*
                  * Announced, because the reader who just unchecked the last
                  * one is who most needs to hear it and is least likely to see
                  * it.
                  */}
                <p
                    style={styles.hint}
                    role='status'
                    aria-live='polite'
                >
                    {shown === 0 ? allHiddenWarning : ''}
                </p>
                <p style={styles.hint}>{footerHint}</p>
            </div>

            <div style={styles.actions}>
                <button
                    ref={saveRef}
                    type='button'
                    style={styles.save}
                    disabled={sealed}
                    onClick={onSave}
                >{'Save'}</button>
                <button
                    type='button'
                    style={styles.reset}
                    disabled={sealed}
                    onClick={onReset}
                >{'Restore defaults'}</button>
            </div>

            <p
                style={error || loadError ? styles.error : styles.status}
                role='status'
                aria-live='polite'
            >{error ?? loadError ?? status ?? (sealed ? 'Loading your settings…' : '')}</p>
        </div>
    );
}

export default HideableEditor;
