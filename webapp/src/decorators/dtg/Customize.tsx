import React, {useEffect, useMemo, useState} from 'react';

import {editableZoneIds, normalizeZoneSelection} from './preferences';
import {DEFAULT_URGENT_WITHIN_MS} from './relative';
import ZonePicker from './ZonePicker';
import type {ZoneSelection} from './zones';
import {availableZoneGroups, orderedZones, zoneKey} from './zones';

import LinkButton from '../../components/LinkButton';
import {resetPreferencesSection, savePreferencesSection, usePreferences} from '../../preferences/store';

/** Mirrors maxZones in server/preferences.go, which rejects anything above it. */
const MAX_ZONES = 25;

/** Mirrors the threshold bounds in server/preferences.go. */
const MIN_MINUTES = 1;
const MAX_MINUTES = 24 * 60;

const DEFAULT_MINUTES = DEFAULT_URGENT_WITHIN_MS / 60000;

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
    zoneRow: {
        display: 'flex',
        alignItems: 'baseline',
        gap: '8px',
        padding: '4px 0',
        fontSize: '13px',
    },
    zoneName: {flex: 1, minWidth: 0, overflowWrap: 'anywhere'},
    offset: {
        fontSize: '11px',
        opacity: 0.7,
        fontFamily: 'ui-monospace, SFMono-Regular, Menlo, monospace',
        whiteSpace: 'nowrap',
    },
    zoneId: {
        fontSize: '11px',
        opacity: 0.5,
        fontFamily: 'ui-monospace, SFMono-Regular, Menlo, monospace',
    },
    remove: {
        border: '1px solid rgba(var(--center-channel-color-rgb), 0.16)',
        background: 'transparent',
        color: 'var(--center-channel-color)',
        borderRadius: '4px',
        cursor: 'pointer',
        fontSize: '12px',
        lineHeight: 1,
        padding: '4px 7px',
    },
    control: {
        marginTop: '8px',
        width: '100%',
        maxWidth: '100%',
        boxSizing: 'border-box',
        padding: '6px 8px',
        fontSize: '13px',
        color: 'var(--center-channel-color)',
        background: 'var(--center-channel-bg)',
        border: '1px solid rgba(var(--center-channel-color-rgb), 0.16)',
        borderRadius: '4px',
    },
    minutes: {width: '90px', marginTop: 0, marginRight: '8px'},
    hint: {fontSize: '12px', opacity: 0.6, margin: '6px 0 0'},
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

/**
 * Reads a minutes field.
 *
 * Blank is not an error, it is how a reader goes back to the default, so it
 * maps to zero rather than to a complaint.
 */
function parseMinutes(raw: string): {minutes: number} | {error: string} {
    const trimmed = raw.trim();
    if (trimmed === '') {
        return {minutes: 0};
    }

    const value = Number(trimmed);
    if (!Number.isInteger(value) || value < MIN_MINUTES || value > MAX_MINUTES) {
        return {error: `Enter a whole number of minutes between ${MIN_MINUTES} and ${MAX_MINUTES}, or leave it blank for the default.`};
    }

    return {minutes: value};
}

interface Props {

    /** The DTG being viewed, which the offsets are measured at. */
    instant: Date;

    /** Returns the panel to the DTG itself. */
    onClose: () => void;
}

/**
 * The reader's own view settings, taking over the whole panel.
 *
 * It replaces the DTG rather than sitting under it because the picker alone is
 * several hundred rows: below the table it would have buried the thing the
 * reader opened the sidebar to see.
 *
 * Everything here is per-reader and stored server side, so it follows them to
 * another browser. It does not follow the link into the standalone page, which
 * is served without a session and therefore always shows the defaults.
 *
 * Mounted only while the reader is actually editing, which is what keeps the
 * few hundred offset measurements below off the path of every panel that is
 * only ever read.
 */
const Customize: React.FC<Props> = ({instant, onClose}) => {
    const {preferences, loading, error: loadError, loaded} = usePreferences();

    const [zones, setZones] = useState<ZoneSelection[]>(() => editableZoneIds(preferences.dtg));
    const [minutes, setMinutes] = useState<string>(
        () => (preferences.dtg.urgentWithinMinutes === 0 ? '' : String(preferences.dtg.urgentWithinMinutes)),
    );
    const [busy, setBusy] = useState(false);

    // Nothing is editable until a read has succeeded. See the cot editor for
    // the defect: a failed FIRST read degrades to the defaults, and saving an
    // edit made on top of those replaces the reader's real section.
    const sealed = busy || loading || !loaded;
    const [status, setStatus] = useState<string | null>(null);
    const [error, setError] = useState<string | null>(null);

    // Whether the reader has touched the form, which is the only thing that can
    // tell their draft from what the store happens to hold.
    const [touched, setTouched] = useState(false);

    // The stored settings arrive after the first render, so the form has to
    // pick them up when they land, but never over the top of an edit.
    //
    // This carried a comment arguing the only window was "the reader typing
    // inside the first round trip", and therefore not worth a flag. That was
    // true when the store loaded once per page and never again. It is not true
    // now: the cache has a lifetime, and usePreferences starts a fresh read on
    // the first mount after it lapses. Every hover card is such a mount, so
    // moving the pointer across a decorated timestamp while this editor is open
    // was enough to reset the picker and the threshold, with nothing on screen
    // saying why. The location editor has carried this flag from the start; the
    // two are now the same.
    //
    // The controls are disabled for the whole of a read, which is the half a
    // test can reach and does. This flag is the other half, for the path where
    // `loading` is false and the form is usable anyway: a failed load leaves
    // the store unloaded on purpose, so a later mount starts a fresh read with
    // the reader already typing. Removing it breaks no test today, which is
    // recorded here rather than taken as permission to remove it.
    useEffect(() => {
        if (touched) {
            return;
        }

        setZones(editableZoneIds(preferences.dtg));
        setMinutes(preferences.dtg.urgentWithinMinutes === 0 ? '' : String(preferences.dtg.urgentWithinMinutes));
    }, [preferences, touched]);

    // Ordered west to east, the same way the table is, so the editor and the
    // panel do not read as two different lists.
    const chosen = useMemo(() => orderedZones(zones, instant), [zones, instant]);

    const selectable = useMemo(() => {
        // By identity, not by zone: Ramstein being chosen must not take
        // Stuttgart out of the picker with it.
        const taken = new Set(zones.map(zoneKey));
        return availableZoneGroups(instant).
            map((group) => ({...group, zones: group.zones.filter((zone) => !taken.has(zone.key))})).
            filter((group) => group.zones.length > 0);
    }, [instant, zones]);

    const edited = (next: ZoneSelection[]) => {
        setTouched(true);
        setZones(next);
        setStatus(null);
        setError(null);
    };

    const onSave = async () => {
        const parsed = parseMinutes(minutes);
        if ('error' in parsed) {
            setStatus(null);
            setError(parsed.error);
            return;
        }

        setBusy(true);
        setStatus(null);
        setError(null);
        try {
            // Section-scoped, so this cannot touch what the reader chose in
            // another decorator's editor. The store re-reads before writing;
            // spreading the cached blob here would carry a snapshot as stale as
            // the tab is old straight back over the top of a newer one.
            await savePreferencesSection('dtg', {
                zones: normalizeZoneSelection(zones),
                urgentWithinMinutes: parsed.minutes,
            });
            setStatus('Saved.');

            // Straight back to the DTG, where the new table is the receipt. A
            // save that failed stays put instead, with the reason on screen.
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
            await resetPreferencesSection('dtg');
            setStatus('Defaults restored.');
            onClose();
        } catch (err) {
            setError(err instanceof Error ? err.message : 'Could not restore the defaults.');
        } finally {
            setBusy(false);
        }
    };

    return (
        <div style={styles.root}>
            <LinkButton
                style={styles.back}
                disabled={sealed}
                onClick={onClose}
            >{'← Back'}</LinkButton>

            <div style={styles.section}>
                <p style={styles.legend}>{'Timezones'}</p>

                {chosen.map((zone) => (
                    <div
                        key={zone.key}
                        style={styles.zoneRow}
                    >
                        <span style={styles.zoneName}>
                            {zone.name}
                            {zone.offsetLabel === '' ? null : <span style={styles.offset}>{` ${zone.offsetLabel}`}</span>}
                            {zone.name === zone.iana ? null : <span style={styles.zoneId}>{` ${zone.iana}`}</span>}
                        </span>
                        <button
                            type='button'
                            style={styles.remove}
                            aria-label={`Remove ${zone.name}`}
                            disabled={sealed}

                            // By identity, not by position: the rows are
                            // ordered by offset, which is not the order the
                            // selection is held in, and two bases can share a
                            // zone.
                            onClick={() => edited(zones.filter((entry) => zoneKey(entry) !== zone.key))}
                        >{'Remove'}</button>
                    </div>
                ))}

                {zones.length === 0 && <p style={styles.hint}>{'No timezones selected, so the defaults are shown.'}</p>}

                <ZonePicker
                    groups={selectable}
                    disabled={busy || loading || zones.length >= MAX_ZONES}
                    onPick={(zone) => edited([...zones, {iana: zone.iana, name: zone.name}])}
                />

                {zones.length >= MAX_ZONES && <p style={styles.hint}>{`That is the most this panel will show (${MAX_ZONES}).`}</p>}
            </div>

            <div style={styles.section}>
                <p style={styles.legend}>{'Flash warning'}</p>
                <label>
                    <input
                        type='number'
                        min={MIN_MINUTES}
                        max={MAX_MINUTES}
                        step={1}
                        value={minutes}
                        disabled={sealed}
                        placeholder={String(DEFAULT_MINUTES)}
                        style={{...styles.control, ...styles.minutes}}
                        onChange={(event) => {
                            setTouched(true);
                            setMinutes(event.target.value);
                            setStatus(null);
                            setError(null);
                        }}
                    />
                    {'minutes before or after the time'}
                </label>
                <p style={styles.hint}>{`Leave blank for the default of ${DEFAULT_MINUTES} minutes.`}</p>
            </div>

            <div style={styles.actions}>
                <button
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
            >{error ?? loadError ?? status ?? ''}</p>
        </div>
    );
};

export default Customize;
