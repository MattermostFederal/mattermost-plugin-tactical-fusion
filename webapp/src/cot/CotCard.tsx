import React from 'react';

import CotMap from './CotMap';
import type {CotEvent, CotPayload} from './types';
import {SOURCE_FILE, affiliationColor, isLinkable, staleAfterPosting, validFor} from './types';

import ErrorBoundary from '../components/ErrorBoundary';
import HoverLink from '../decorators/HoverLink';
import {pluginBaseUrl} from '../plugin_url';

import {showCotEvent} from './index';

interface Props {
    payload: CotPayload;
    createAt: number;
    compactDisplay?: boolean;
}

const styles: Record<string, React.CSSProperties> = {
    text: {whiteSpace: 'pre-wrap'},
    card: {
        border: '1px solid rgba(var(--center-channel-color-rgb), 0.16)',
        borderRadius: 4,
        marginTop: 8,
        maxWidth: 640,
        overflow: 'hidden',
    },
    kind: {
        fontWeight: 700,
        margin: 0,
        padding: '8px 12px 0',
    },
    header: {
        alignItems: 'baseline',
        display: 'flex',
        flexWrap: 'wrap',
        gap: '0.5em',
        padding: '2px 12px 8px',
    },
    callsign: {fontWeight: 600},
    typeLabel: {opacity: 0.85},
    rawType: {fontFamily: 'monospace', fontSize: '0.85em', opacity: 0.9},
    rows: {display: 'grid', gridTemplateColumns: 'max-content 1fr', gap: '2px 12px', margin: 0, padding: '0 12px 8px'},
    term: {opacity: 0.85},
    value: {margin: 0, wordBreak: 'break-word'},
    note: {opacity: 0.9, padding: '0 12px 8px'},
    disclosure: {borderTop: '1px solid rgba(var(--center-channel-color-rgb), 0.16)', padding: '6px 12px'},
    actions: {display: 'flex', gap: '12px', padding: '0 12px 8px'},
    list: {listStyle: 'none', margin: 0, padding: '0 12px 8px'},
    listItem: {alignItems: 'baseline', display: 'flex', flexWrap: 'wrap', gap: '0.5em', padding: '3px 0'},
    button: {
        background: 'none',
        border: 'none',
        color: 'var(--link-color)',
        cursor: 'pointer',
        font: 'inherit',
        padding: 0,
    },
};

/**
 * What the card is, said in full before what the event is.
 *
 * The abbreviation is carried beside the expansion because both are in use: an
 * operator reads CoT, and somebody meeting the card for the first time has no
 * way to expand it. The sidebar's own title says "Cursor on Target" without it,
 * since by then the reader has already clicked through.
 */
export const CARD_KIND = 'Cursor on Target (CoT)';

function Row({label, children}: {label: string; children: React.ReactNode}) {
    return (
        <>
            <dt style={styles.term}>{label}</dt>
            <dd style={styles.value}>{children}</dd>
        </>
    );
}

function PositionValue({event}: {event: CotEvent}) {
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

const FILE_ID = /^[a-z0-9]{26}$/;

/**
 * A time, linked to the date-time group tools when the server could spell it.
 *
 * The anchor is a plain one, exactly as the position row's is: the stylesheet's
 * href rule and the document-level click handler both key off the path, so the
 * chip and the sidebar come back without either of them knowing this card
 * exists.
 */
function TimeValue({reading, query}: {reading: string; query: string}) {
    if (query === '') {
        return <span>{reading}</span>;
    }

    return <HoverLink href={`${pluginBaseUrl()}/decorate/dtg?${query}`}>{reading}</HoverLink>;
}

export function fileHref(fileId: string): string | null {
    if (!FILE_ID.test(fileId)) {
        return null;
    }

    const globalWindow = typeof window === 'undefined' ? undefined : (window as {basename?: string});
    return `${globalWindow?.basename ?? ''}/api/v4/files/${encodeURIComponent(fileId)}`;
}

/** The dot, which is redundant with the affiliation word in the type label. */
function Dot({event}: {event: CotEvent}) {
    const colour = affiliationColor(event);
    if (colour === undefined) {
        return null;
    }

    return (
        <span
            aria-hidden={true}
            style={{
                background: colour,
                borderRadius: '50%',
                display: 'inline-block',
                height: 10,
                width: 10,
            }}
        />
    );
}

function Naming({event}: {event: CotEvent}) {
    return (
        <>
            <Dot event={event}/>
            <span style={styles.callsign}>{event.callsign === '' ? event.uid : event.callsign}</span>
            <span style={styles.typeLabel}>
                {event.typeLabel === '' ? 'Unrecognized event type' : event.typeLabel}
            </span>
            <span style={styles.rawType}>{`(${event.cotType})`}</span>
        </>
    );
}

/** One event, in full. What a post carrying a single event shows. */
function EventDetail({event, createAt}: {event: CotEvent; createAt: number}) {
    const staleReading = staleAfterPosting(event, createAt);
    const window = validFor(event);

    return (
        <>
            {event.positionNote !== '' && <p style={styles.note}>{event.positionNote}</p>}

            <dl
                style={styles.rows}
                role='group'
                aria-label='Details of this Cursor on Target event'
            >
                {event.lat !== '' && <Row label='Position'><PositionValue event={event}/></Row>}
                {event.hae !== '' && <Row label='Altitude (HAE)'>{event.hae}</Row>}
                <Row label='Accuracy'>
                    {event.ce === '' ? 'Not stated' : `${event.ce} circular`}
                    {event.le !== '' && `, ${event.le} vertical`}
                </Row>
                {event.speed !== '' && (
                    <Row label='Track'>
                        {event.speed}
                        {event.course !== '' && ` on ${event.course}`}
                    </Row>
                )}
                {event.time !== '' && (
                    <Row label='Time'>
                        <TimeValue
                            reading={event.time}
                            query={event.timeQuery}
                        />
                    </Row>
                )}
                {event.start !== '' && event.start !== event.time && (
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
                {staleReading !== '' && <Row label='Freshness'>{staleReading}</Row>}
                {event.group !== '' && (
                    <Row label='Team'>
                        {event.group}
                        {event.role !== '' && `, ${event.role}`}
                    </Row>
                )}
                {event.parent !== '' && <Row label='Sent by'>{event.parent}</Row>}
                {event.related !== '' && <Row label='Relates to'>{event.related}</Row>}
                {event.howLabel !== '' && <Row label='Source'>{event.howLabel}</Row>}
                <Row label='UID'>{event.uid}</Row>
                {event.remarks !== '' && <Row label='Remarks'>{event.remarks}</Row>}
            </dl>
        </>
    );
}

/**
 * Several events, one line each.
 *
 * A post is one post. Rendering every event in full would put N maps and N
 * tables in the channel, so the list names each track and links its position,
 * and the panel behind "Open details" carries the rest.
 */
function EventList({events}: {events: readonly CotEvent[]}) {
    return (
        <ul
            style={styles.list}
            aria-label='The events in this post'
        >
            {events.map((event, index) => (
                <li

                    // Nothing in a CoT event is guaranteed unique inside one
                    // block: two position reports from one device share a uid.
                    // eslint-disable-next-line react/no-array-index-key
                    key={`${event.uid}-${index}`}
                    style={styles.listItem}
                >
                    <Naming event={event}/>
                    {event.lat !== '' && <PositionValue event={event}/>}
                </li>
            ))}
        </ul>
    );
}

export const CotCard: React.FC<Props> = ({payload, createAt, compactDisplay}) => {
    const {events} = payload;
    const only = events.length === 1 ? events[0] : undefined;

    return (
        <div>
            {payload.lead !== '' && <div style={styles.text}>{payload.lead}</div>}

            <div
                style={styles.card}
                data-testid='cot-card'
            >
                <p style={styles.kind}>
                    {only ? `${CARD_KIND}:` : `${CARD_KIND}: ${events.length} events`}
                </p>

                {only && <div style={styles.header}><Naming event={only}/></div>}

                {!compactDisplay && events.some(isLinkable) && (
                    <ErrorBoundary>
                        <CotMap events={events}/>
                    </ErrorBoundary>
                )}

                {only ? (
                    <EventDetail
                        event={only}
                        createAt={createAt}
                    />
                ) : <EventList events={events}/>}

                <div style={styles.actions}>
                    <button
                        type='button'
                        style={styles.button}
                        onClick={() => showCotEvent(payload)}
                    >
                        {'Open details'}
                    </button>
                </div>

                {payload.source === SOURCE_FILE && fileHref(payload.fileId) !== null && (
                    <div style={styles.disclosure}>
                        <a href={fileHref(payload.fileId) ?? undefined}>
                            {payload.fileName === '' ? 'Download the source file' : `Download ${payload.fileName}`}
                        </a>
                    </div>
                )}
            </div>

            {payload.trail !== '' && <div style={styles.text}>{payload.trail}</div>}
        </div>
    );
};

export default CotCard;
