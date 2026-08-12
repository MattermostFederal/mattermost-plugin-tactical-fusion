import React, {useLayoutEffect} from 'react';

import Countdown from './Countdown';
import Customize from './Customize';
import {describeInstant, hasSeconds} from './describe';
import {EDITOR_TITLE} from './DtgTitle';
import {setEditing, useEditing} from './editing';
import {resolvedUrgentWithinMs, resolvedZones} from './preferences';

import LinkButton from '../../components/LinkButton';
import {docsUrl} from '../../plugin_url';
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
    footer: {
        marginTop: '14px',
        fontSize: '12px',
    },
    separator: {
        color: 'var(--center-channel-color)',
        opacity: 0.4,
        margin: '0 8px',
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

/**
 * The calendar date, in a given IANA zone, as a comparable number.
 *
 * Null when the browser cannot format the zone, which a saved blob can outlive
 * its browser to produce: the server validates against Go's embedded tzdata and
 * the two catalogs disagree, so `posixrules` is storable here and throws in
 * Chromium's `Intl`. Which identifiers those are is engine and version
 * specific, so the guard is on the call rather than on a list of names.
 */
function zoneDateKey(instant: Date, iana: string): number | null {
    let parts: Intl.DateTimeFormatPart[];
    try {
        parts = new Intl.DateTimeFormat('en-US', {
            timeZone: iana,
            year: 'numeric',
            month: 'numeric',
            day: 'numeric',
        }).formatToParts(instant);
    } catch {
        return null;
    }

    const value = (type: string) => Number(parts.find((p) => p.type === type)?.value ?? '0');
    return Date.UTC(value('year'), value('month') - 1, value('day'));
}

/**
 * Formats an instant in a zone, or null if this browser cannot.
 *
 * Nothing may fail the panel. `orderedZones` deliberately keeps a zone it could
 * not measure rather than dropping it, so an unformattable one reaches this
 * table and has to render as a visible gap. Throwing here would take the whole
 * sidebar down, including the "Customize your view" link that is the only way
 * for the reader to remove the offending row.
 */
function formatInZone(instant: Date, iana: string, options: Intl.DateTimeFormatOptions): string | null {
    try {
        return new Intl.DateTimeFormat('en-GB', {...options, timeZone: iana}).format(instant);
    } catch {
        return null;
    }
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
    const seconds = hasSeconds(payload.instant);

    const {preferences} = usePreferences();
    const zones = resolvedZones(preferences.dtg, payload.instant);

    // Kept in a module store rather than here, because the sidebar header is a
    // separate component that has to follow the panel into the editor.
    const customizing = useEditing();

    // Clicking a different DTG while the editor is open would otherwise land on
    // the editor rather than on the DTG that was clicked. React keeps this
    // component mounted across a change of selection, so nothing else resets
    // it.
    //
    // Before paint, not after, so the editor is never flashed on screen
    // carrying a DTG the reader did not open it from. This does not save the
    // picker's few hundred offset measurements: children's render bodies run
    // before any effect, layout or passive, so that frame is computed either
    // way. Avoiding the work needs the decision made during render, not here.
    const instantMs = payload.instant.getTime();
    useLayoutEffect(() => {
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
                        // Seconds only when the token carried them, which a
                        // date-time group never does and an RFC 3339 timestamp
                        // may. See hasSeconds in describe.ts.
                        const time = formatInZone(payload.instant, zone.iana, {
                            hour: '2-digit',
                            minute: '2-digit',
                            ...(seconds ? {second: '2-digit'} : {}),
                            hour12: false,
                        });

                        const date = formatInZone(payload.instant, zone.iana, {
                            weekday: 'short',
                            day: 'numeric',
                            month: 'short',
                        });

                        // No badge for a zone this browser cannot place. An
                        // unmeasurable offset is not a claim that it matches.
                        const dateKey = zoneDateKey(payload.instant, zone.iana);
                        const delta = dateKey === null ? 0 : Math.round((dateKey - reference) / 86400000);

                        return (
                            <tr key={zone.key}>
                                <td style={styles.td}>
                                    {zone.name}
                                    <span style={styles.abbr}>{zone.abbr}</span>
                                </td>
                                <td style={{...styles.td, ...styles.time}}>{time ?? 'n/a'}</td>
                                <td style={styles.td}>
                                    {date ?? 'n/a'}
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

            <div style={styles.footer}>
                <LinkButton onClick={() => setEditing(true)}>{EDITOR_TITLE}</LinkButton>
                <span style={styles.separator}>{'·'}</span>
                <LinkButton href={docsUrl()}>{'Documentation'}</LinkButton>
            </div>
        </div>
    );
};

export default DtgPanel;
