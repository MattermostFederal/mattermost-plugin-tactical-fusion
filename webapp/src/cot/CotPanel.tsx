import React, {useEffect, useLayoutEffect, useRef, useState} from 'react';

import CotMap from './CotMap';
import Customize from './Customize';
import DetailGroups from './DetailGroups';
import Disclosure from './Disclosure';
import {setEditing, useEditing} from './editing';
import {isSectionVisible, sectionLabel} from './sections';
import {staleWait} from './stale';
import type {CotEvent, CotPayload} from './types';
import {SOURCE_FILE, isLinkable, validFor} from './types';

import ErrorBoundary from '../components/ErrorBoundary';
import LinkButton from '../components/LinkButton';
import Countdown from '../decorators/dtg/Countdown';
import HoverLink from '../decorators/HoverLink';
import CopyButton from '../decorators/location/CopyButton';
import {docsUrl, pluginBaseUrl} from '../plugin_url';
import {usePreferences} from '../preferences/store';

export const PANEL_TITLE = 'Cursor on Target';

/**
 * The sidebar header while the editor has the panel.
 *
 * Matches the link that opens it, so the reader is never told two different
 * names for where they are.
 */
export const EDITOR_TITLE = 'Customize your view';

export const SECTION_FAILED = 'This event could not be rendered.';

const styles: Record<string, React.CSSProperties> = {
    heading: {margin: '0 0 4px', fontSize: '16px', fontWeight: 600},
    groupHeading: {margin: '16px 0 4px', fontSize: '12px', textTransform: 'uppercase', letterSpacing: '0.04em', opacity: 0.85, fontWeight: 600},
    subhead: {margin: '0 0 12px', opacity: 0.85, fontSize: '13px'},
    rawType: {fontFamily: 'monospace', fontSize: '0.85em', opacity: 0.9},
    section: {margin: '16px 0 4px', fontSize: '12px', textTransform: 'uppercase', letterSpacing: '0.04em', opacity: 0.85},
    rows: {display: 'grid', gridTemplateColumns: 'max-content 1fr', gap: '4px 12px', margin: 0},
    term: {opacity: 0.85},
    value: {margin: 0, wordBreak: 'break-word'},
    remarks: {margin: 0, whiteSpace: 'pre-wrap'},
    source: {
        fontFamily: 'monospace',
        fontSize: '0.85em',
        margin: 0,
        maxHeight: 280,
        overflow: 'auto',
        whiteSpace: 'pre',
    },
    stale: {
        fontFamily: 'ui-monospace, SFMono-Regular, Menlo, monospace',
        fontSize: '24px',
        fontWeight: 600,
        color: 'var(--error-text, #d24b4e)',
        borderLeft: '4px solid var(--error-text, #d24b4e)',
        paddingLeft: '10px',
        margin: '16px 0 6px',
    },
    later: {
        borderTop: '1px solid rgba(var(--center-channel-color-rgb), 0.16)',
        marginTop: '20px',
        paddingTop: '16px',
    },
    footer: {
        borderTop: '1px solid rgba(var(--center-channel-color-rgb), 0.16)',
        fontSize: '13px',
        marginTop: '20px',
        paddingTop: '12px',
    },
};

function Row({label, children}: {label: string; children: React.ReactNode}) {
    return (
        <>
            <dt style={styles.term}>{label}</dt>
            <dd style={styles.value}>{children}</dd>
        </>
    );
}

function TimeValue({reading, query}: {reading: string; query: string}) {
    if (query === '') {
        return <span>{reading}</span>;
    }

    return <HoverLink href={`${pluginBaseUrl()}/decorate/dtg?${query}`}>{reading}</HoverLink>;
}

function Position({event}: {event: CotEvent}) {
    const reading = `${event.lat}, ${event.lon}`;

    if (!isLinkable(event)) {
        return <span>{reading}</span>;
    }

    const params = new URLSearchParams({f: event.format, v: event.value});
    return (
        <HoverLink href={`${pluginBaseUrl()}/decorate/location?${params.toString()}`}>
            {reading}
        </HoverLink>
    );
}

function StaleCountdown({event}: {event: CotEvent}) {
    const staleAt = Number(event.staleAt);
    const usable = Number.isFinite(staleAt) && staleAt > 0;
    const [passed, setPassed] = useState(() => usable && staleAt <= Date.now());
    const [rearm, setRearm] = useState(0);

    useEffect(() => {
        if (!usable) {
            return undefined;
        }

        const remaining = staleAt - Date.now();
        if (remaining <= 0) {
            setPassed(true);
            return undefined;
        }

        setPassed(false);
        const wait = staleWait(remaining);
        const timer = setTimeout(() => {
            if (wait.settles) {
                setPassed(true);
                return;
            }
            setRearm((armed) => armed + 1);
        }, wait.ms);
        return () => clearTimeout(timer);
    }, [staleAt, usable, rearm]);

    if (!usable) {
        return null;
    }

    // No heading once it has passed. "Goes stale" is what the countdown NEEDS,
    // because a number on its own says nothing; the word "Stale" says the whole
    // thing by itself, and a future-tense heading over it read as a label with
    // its own value repeated underneath.
    if (passed) {
        return <p style={styles.stale}>{'Stale'}</p>;
    }

    return (
        <div>
            <h3 style={styles.groupHeading}>{sectionLabel('stale')}</h3>
            <Countdown target={new Date(staleAt)}/>
        </div>
    );
}

const EventSection: React.FC<{event: CotEvent; payload: CotPayload; hidden: readonly string[]}> = ({event, payload, hidden}) => {
    const window = validFor(event);

    return (
        <div>
            <h2 style={styles.heading}>{event.callsign === '' ? event.uid : event.callsign}</h2>
            <p style={styles.subhead}>
                {event.typeLabel === '' ? 'Unrecognized event type' : event.typeLabel}
                {' '}
                <span style={styles.rawType}>{`(${event.cotType})`}</span>
            </p>

            {isLinkable(event) && (
                <ErrorBoundary>
                    <CotMap
                        events={[event]}
                        surface='panel'
                    />
                </ErrorBoundary>
            )}

            {event.positionNote !== '' && <p style={styles.subhead}>{event.positionNote}</p>}

            {isSectionVisible(hidden, 'stale') && <StaleCountdown event={event}/>}

            {isSectionVisible(hidden, 'event') && (
                <>
                    <h3 style={styles.groupHeading}>{sectionLabel('event')}</h3>
                    <dl
                        style={styles.rows}
                        role='group'
                        aria-label={`Readings for the Cursor on Target event ${event.callsign === '' ? event.uid : event.callsign}`}
                    >
                        {event.lat !== '' && <Row label='Position'><Position event={event}/></Row>}
                        {event.hae !== '' && <Row label='Altitude (HAE)'>{event.hae}</Row>}
                        <Row label='Accuracy'>
                            {event.ce === '' ? 'Not stated' : `${event.ce} circular`}
                            {event.le !== '' && `, ${event.le} vertical`}
                        </Row>
                        {event.speed !== '' && <Row label='Speed'>{event.speed}</Row>}
                        {event.course !== '' && <Row label='Course'>{event.course}</Row>}
                        {event.time !== '' && (
                            <Row label='Time'>
                                <TimeValue
                                    reading={event.time}
                                    query={event.timeQuery}
                                />
                            </Row>
                        )}
                        {event.start !== '' && (
                            <Row label='Valid from'>
                                <TimeValue
                                    reading={event.start}
                                    query={event.startQuery}
                                />
                            </Row>
                        )}
                        {event.stale !== '' && (
                            <Row label='Stale'>
                                <TimeValue
                                    reading={event.stale}
                                    query={event.staleQuery}
                                />
                                {window !== '' && ` (valid for ${window})`}
                            </Row>
                        )}
                        {event.group !== '' && <Row label='Team'>{event.group}</Row>}
                        {event.role !== '' && <Row label='Role'>{event.role}</Row>}
                        {event.parent !== '' && <Row label='Sent by'>{event.parent}</Row>}
                        {event.related !== '' && <Row label='Relates to'>{event.related}</Row>}
                        {event.howLabel !== '' && <Row label='Source'>{event.howLabel}</Row>}
                        <Row label='UID'>{event.uid}</Row>
                    </dl>
                </>
            )}

            {isSectionVisible(hidden, 'remarks') && event.remarks !== '' && (
                <div>
                    <h3 style={styles.groupHeading}>{sectionLabel('remarks')}</h3>
                    <p style={styles.remarks}>{event.remarks}</p>
                </div>
            )}

            <DetailGroups
                event={event}
                hidden={hidden}
            />

            {payload.source === SOURCE_FILE && payload.fileName !== '' && (
                <div>
                    <h3 style={styles.groupHeading}>{'Source file'}</h3>
                    <p style={styles.value}>{payload.fileName}</p>
                </div>
            )}

            {isSectionVisible(hidden, 'source') && payload.src !== '' && (
                <Disclosure
                    label={sectionLabel('source')}
                    trailing={
                        <CopyButton
                            label='Copy the event as posted'
                            value={payload.src}
                        />
                    }
                >
                    <pre
                        style={styles.source}
                        tabIndex={0}
                        role='region'
                        aria-label='The event as it was posted'
                    >{payload.src}</pre>
                </Disclosure>
            )}
        </div>
    );
};

export const CotPanel: React.FC<{payload: CotPayload}> = ({payload}) => {
    const {preferences} = usePreferences();
    const customizing = useEditing();

    // Returning from the editor unmounts it, so focus would fall to the body.
    // Put the reader back on the control they opened it from.
    const customizeRef = useRef<HTMLButtonElement>(null);
    const wasCustomizing = useRef(customizing);
    useEffect(() => {
        if (wasCustomizing.current && !customizing) {
            customizeRef.current?.focus();
        }
        wasCustomizing.current = customizing;
    }, [customizing]);

    // Keyed on the payload itself rather than on anything derived from it.
    // `setSelection` stores one object per click, so this changes exactly once
    // per selection and never on a re-render. A derived key was wrong: it was
    // `count:firstUid`, and two position reports from one device are two posts
    // with one event each and the same uid, so clicking the second left the
    // reader in the editor. That is the case this effect exists for.
    //
    // Before paint, not after, so the editor is never flashed on screen
    // carrying an event the reader did not open it from.
    useLayoutEffect(() => {
        setEditing(false);
    }, [payload]);

    if (customizing) {
        return <Customize onClose={() => setEditing(false)}/>;
    }

    const hidden = preferences.cot.hiddenSections;

    return (
        <div>
            {payload.events.length > 1 && (
                <p style={styles.subhead}>{`${payload.events.length} events in this post`}</p>
            )}

            {payload.events.map((event, index) => (
                <div

                    // Nothing in a CoT event is unique inside one block: two
                    // position reports from one device share a uid.
                    // eslint-disable-next-line react/no-array-index-key
                    key={`${event.uid}-${index}`}
                    style={index > 0 ? styles.later : undefined}
                >
                    <ErrorBoundary fallback={<p style={styles.subhead}>{SECTION_FAILED}</p>}>
                        <EventSection
                            event={event}
                            payload={payload}
                            hidden={hidden}
                        />
                    </ErrorBoundary>
                </div>
            ))}

            <p style={styles.footer}>
                <LinkButton
                    ref={customizeRef}
                    onClick={() => setEditing(true)}
                >{'Customize your view'}</LinkButton>
                <span aria-hidden={true}>{' · '}</span>
                <LinkButton href={docsUrl()}>{'Documentation'}</LinkButton>
            </p>
        </div>
    );
};

export const CotTitle: React.FC<{payload: CotPayload}> = ({payload}) => {
    const {events} = payload;
    const customizing = useEditing();

    // The editor takes the panel over, so a header still naming the event would
    // be describing something no longer on screen.
    if (customizing) {
        return <span>{EDITOR_TITLE}</span>;
    }

    // A block is named by its count rather than by its first event, which would
    // read as a panel about that one track.
    if (events.length !== 1) {
        return <span>{`${PANEL_TITLE}: ${events.length} events`}</span>;
    }

    const [event] = events;
    return <span>{`${PANEL_TITLE}: ${event.callsign === '' ? event.uid : event.callsign}`}</span>;
};

export default CotPanel;
