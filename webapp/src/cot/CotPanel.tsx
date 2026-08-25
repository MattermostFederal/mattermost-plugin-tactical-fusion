import React from 'react';

import CotMap from './CotMap';
import DetailGroups from './DetailGroups';
import type {CotEvent, CotPayload} from './types';
import {SOURCE_FILE, isLinkable, validFor} from './types';

import ErrorBoundary from '../components/ErrorBoundary';
import Countdown from '../decorators/dtg/Countdown';
import HoverLink from '../decorators/HoverLink';
import {pluginBaseUrl} from '../plugin_url';

export const PANEL_TITLE = 'Cursor on Target';

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
    countdownNote: {fontSize: '12px', opacity: 0.85, margin: '4px 0 0'},
    later: {
        borderTop: '1px solid rgba(var(--center-channel-color-rgb), 0.16)',
        marginTop: '20px',
        paddingTop: '16px',
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

/**
 * The countdown, which lives here and nowhere else.
 *
 * A ticking clock in a channel reads as a live feed and puts a timer behind
 * every post in the window, which is why the card carries a fixed reading
 * instead. A reader who opened this panel asked for one event, so the argument
 * `dtg/Countdown` was built under holds again: there is one of them, and the
 * reader chose it.
 *
 * The caveat is stated rather than hidden. This is the only reading in the
 * feature that depends on the workstation's clock, and clock drift on field kit
 * is not rare.
 */
function StaleCountdown({event}: {event: CotEvent}) {
    const staleAt = Number(event.staleAt);
    if (!Number.isFinite(staleAt) || staleAt <= 0) {
        return null;
    }

    return (
        <div>
            <h3 style={styles.groupHeading}>{'Goes stale'}</h3>
            <Countdown target={new Date(staleAt)}/>
            <p style={styles.countdownNote}>
                {'Counted against this device’s clock, unlike every other reading here.'}
            </p>
        </div>
    );
}

const EventSection: React.FC<{event: CotEvent; payload: CotPayload}> = ({event, payload}) => {
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
                    <CotMap events={[event]}/>
                </ErrorBoundary>
            )}

            {event.positionNote !== '' && <p style={styles.subhead}>{event.positionNote}</p>}

            <h3 style={styles.groupHeading}>{'Event'}</h3>
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

            <StaleCountdown event={event}/>

            {event.remarks !== '' && (
                <div>
                    <h3 style={styles.groupHeading}>{'Remarks'}</h3>
                    <p style={styles.remarks}>{event.remarks}</p>
                </div>
            )}

            <DetailGroups event={event}/>

            {payload.source === SOURCE_FILE && payload.fileName !== '' && (
                <div>
                    <h3 style={styles.groupHeading}>{'Source file'}</h3>
                    <p style={styles.value}>{payload.fileName}</p>
                </div>
            )}

            {payload.src !== '' && (
                <div>
                    <h3 style={styles.groupHeading}>{'As posted'}</h3>
                    <pre
                        style={styles.source}
                        tabIndex={0}
                        role='region'
                        aria-label='The event as it was posted'
                    >{payload.src}</pre>
                </div>
            )}
        </div>
    );
};

export const CotPanel: React.FC<{payload: CotPayload}> = ({payload}) => (
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
                    />
                </ErrorBoundary>
            </div>
        ))}
    </div>
);

export const CotTitle: React.FC<{payload: CotPayload}> = ({payload}) => {
    const {events} = payload;

    // A block is named by its count rather than by its first event, which would
    // read as a panel about that one track.
    if (events.length !== 1) {
        return <span>{`${PANEL_TITLE}: ${events.length} events`}</span>;
    }

    const [event] = events;
    return <span>{`${PANEL_TITLE}: ${event.callsign === '' ? event.uid : event.callsign}`}</span>;
};

export default CotPanel;
