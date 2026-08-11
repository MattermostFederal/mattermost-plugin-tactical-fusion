import React, {useEffect} from 'react';

import Countdown from './Countdown';
import Customize from './Customize';
import {describeInstant} from './describe';
import {setEditing, useEditing} from './editing';
import {resolvedUrgentWithinMs, resolvedZones} from './preferences';
import {EDITOR_TITLE} from './titles';

import LinkButton from '../../components/LinkButton';
import {usePreferences} from '../../preferences/store';

import type {Dtg} from './index';

const styles: Record<string, React.CSSProperties> = {
    described: {
        fontSize: '14px',
        color: 'var(--center-channel-color)',
        margin: '0 0 2px',
    },
    canonical: {
        fontFamily: 'ui-monospace, SFMono-Regular, Menlo, monospace',
        fontSize: '12px',
        color: 'var(--center-channel-color)',
        opacity: 0.6,
        margin: 0,
        wordBreak: 'break-all',
    },
    note: {
        fontSize: '12px',
        color: 'var(--center-channel-color)',
        opacity: 0.6,
        fontStyle: 'italic',
        margin: '12px 0 0',
    },
    table: {width: '100%', borderCollapse: 'collapse', fontSize: '13px', marginTop: '20px'},
    th: {
        textAlign: 'left',
        fontSize: '11px',
        textTransform: 'uppercase',
        letterSpacing: '0.04em',
        fontWeight: 600,
        opacity: 0.7,
        color: 'var(--center-channel-color)',
        padding: '8px 10px',
        borderBottom: '1px solid rgba(var(--center-channel-color-rgb), 0.12)',
    },
    td: {
        padding: '10px',
        color: 'var(--center-channel-color)',
        borderBottom: '1px solid rgba(var(--center-channel-color-rgb), 0.08)',
    },
    time: {
        fontFamily: 'ui-monospace, SFMono-Regular, Menlo, monospace',
        textAlign: 'right',
        whiteSpace: 'nowrap',
    },
    abbr: {fontSize: '11px', opacity: 0.5, marginLeft: '6px'},

    // Deliberately quiet: a way out of the panel, not a call to action. The
    // table's own last rule already separates it from the rows above, so it
    // needs no line of its own either.
    customize: {
        display: 'inline-block',
        marginTop: '14px',
        fontSize: '12px',
    },
    badge: {
        fontSize: '10px',
        fontWeight: 600,
        padding: '1px 5px',
        borderRadius: '3px',
        marginLeft: '6px',
        opacity: 0.7,
        border: '1px solid rgba(var(--center-channel-color-rgb), 0.16)',
    },
};

/** The calendar date, in a given IANA zone, as a comparable number. */
function zoneDateKey(instant: Date, iana: string): number {
    const parts = new Intl.DateTimeFormat('en-US', {
        timeZone: iana,
        year: 'numeric',
        month: 'numeric',
        day: 'numeric',
    }).formatToParts(instant);

    const value = (type: string) => Number(parts.find((p) => p.type === type)?.value ?? '0');
    return Date.UTC(value('year'), value('month') - 1, value('day'));
}

/**
 * The token's own zone date, which every row's day offset is measured against.
 *
 * Measuring against UTC would badge rows as a day out for any token that was
 * never written in UTC to begin with.
 */
function referenceDateKey(payload: Dtg): number {
    const shifted = new Date(payload.instant.getTime() + (payload.offsetMinutes * 60000));
    return Date.UTC(shifted.getUTCFullYear(), shifted.getUTCMonth(), shifted.getUTCDate());
}

function assumedNote(payload: Dtg): string {
    if (payload.assumedMonth && payload.assumedYear) {
        return 'Month and year were not in the original text; both were taken from the date the message was posted.';
    }
    if (payload.assumedYear) {
        return 'The year was not in the original text; it was taken from the date the message was posted.';
    }
    return '';
}

const DtgPanel: React.FC<{payload: Dtg}> = ({payload}) => {
    const reference = referenceDateKey(payload);
    const note = assumedNote(payload);

    const {preferences} = usePreferences();
    const zones = resolvedZones(preferences.dtg, payload.instant);

    // Kept in a module store rather than here, because the sidebar header is a
    // separate component that has to follow the panel into the editor.
    const customizing = useEditing();

    // Clicking a different DTG while the editor is open would otherwise land on
    // the editor rather than on the DTG that was clicked. React keeps this
    // component mounted across a change of selection, so nothing else resets
    // it.
    const instantMs = payload.instant.getTime();
    useEffect(() => {
        setEditing(false);
    }, [instantMs, payload.canonical]);

    // The editor takes the panel over rather than sitting below the table: its
    // timezone picker is several hundred rows, which under the table would
    // bury the DTG the reader opened the sidebar for.
    if (customizing) {
        return (
            <Customize
                instant={payload.instant}
                onClose={() => setEditing(false)}
            />
        );
    }

    return (
        <div>
            <Countdown
                target={payload.instant}
                urgentWithinMs={resolvedUrgentWithinMs(preferences.dtg)}
            />
            <p style={styles.described}>{describeInstant(payload.instant, payload.offsetMinutes, payload.zoneLabel)}</p>
            <p style={styles.canonical}>{payload.canonical}</p>
            {note && <p style={styles.note}>{note}</p>}

            <table style={styles.table}>
                <thead>
                    <tr>
                        <th style={styles.th}>{'Location'}</th>
                        <th style={{...styles.th, textAlign: 'right'}}>{'Time'}</th>
                        <th style={styles.th}>{'Date'}</th>
                    </tr>
                </thead>
                <tbody>
                    {zones.map((zone) => {
                        const time = new Intl.DateTimeFormat('en-GB', {
                            timeZone: zone.iana,
                            hour: '2-digit',
                            minute: '2-digit',
                            hour12: false,
                        }).format(payload.instant);

                        const date = new Intl.DateTimeFormat('en-GB', {
                            timeZone: zone.iana,
                            weekday: 'short',
                            day: 'numeric',
                            month: 'short',
                        }).format(payload.instant);

                        const delta = Math.round(
                            (zoneDateKey(payload.instant, zone.iana) - reference) / 86400000,
                        );

                        return (
                            <tr key={zone.iana}>
                                <td style={styles.td}>
                                    {zone.name}
                                    <span style={styles.abbr}>{zone.abbr}</span>
                                </td>
                                <td style={{...styles.td, ...styles.time}}>{time}</td>
                                <td style={styles.td}>
                                    {date}
                                    {delta !== 0 && (
                                        <span style={styles.badge}>
                                            {delta > 0 ? `+${delta}` : String(delta)}
                                        </span>
                                    )}
                                </td>
                            </tr>
                        );
                    })}
                </tbody>
            </table>

            <LinkButton
                style={styles.customize}
                onClick={() => setEditing(true)}
            >{EDITOR_TITLE}</LinkButton>
        </div>
    );
};

export default DtgPanel;
