import React, {useRef, useState} from 'react';

import CotMap, {focusFor} from './CotMap';
import {ClassSummary, chatReading} from './summary';
import type {CotEvent, CotPayload} from './types';
import {affiliationColor, isLinkable, validFor} from './types';

import ErrorBoundary from '../components/ErrorBoundary';
import HoverLink from '../decorators/HoverLink';
import type {MapFocus} from '../decorators/location/map/focus';
import {pluginBaseUrl} from '../plugin_url';

import {showCotEvent} from './index';

interface Props {
    payload: CotPayload;
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
    actions: {display: 'flex', gap: '12px', padding: '0 12px 8px'},
    list: {listStyle: 'none', margin: 0, padding: '0 12px 8px'},
    listItem: {alignItems: 'baseline', display: 'flex', flexWrap: 'wrap', gap: '0.5em', padding: '3px 0'},
    showButton: {alignItems: 'baseline', background: 'none', border: 'none', color: 'inherit', cursor: 'pointer', display: 'inline-flex', flexWrap: 'wrap', font: 'inherit', gap: '0.5em', padding: 0, textAlign: 'left'},
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

/**
 * What a boundary says when it catches something.
 *
 * A boundary rendering null leaves a card with a heading and no rows, and no
 * reader can tell that from an event that stated nothing. The fallback may
 * never be blank, for the reason the post body's own fallback may not be.
 */
export const DETAIL_FAILED = 'The detail of this event could not be rendered. Open details to read the event as it was posted.';

function Row({label, children}: {label: string; children: React.ReactNode}) {
    return (
        <>
            <dt style={styles.term}>{label}</dt>
            <dd style={styles.value}>{children}</dd>
        </>
    );
}

function PositionValue({event}: {event: CotEvent}) {
    return <span>{`${event.lat}, ${event.lon}`}</span>;
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
    const color = affiliationColor(event);
    if (color === undefined) {
        return null;
    }

    return (
        <span
            aria-hidden={true}
            style={{
                background: color,
                borderRadius: '50%',
                display: 'inline-block',
                height: 10,
                width: 10,
            }}
        />
    );
}

/**
 * That the extension rows did not fit, said where the reader is.
 *
 * The panel carries the same notice, but a reader who never opens it would
 * otherwise meet a card whose class summary silently vanished.
 */
function Degraded({event}: {event: CotEvent}) {
    if (event.detailDropped === '') {
        return null;
    }

    return <p style={styles.note}>{'Some detail was too large to store. Open details to read the event as posted.'}</p>;
}

function ShowOnMap({event, onShow, children}: {event: CotEvent; onShow?: (event: CotEvent) => void; children: React.ReactNode}) {
    if (onShow === undefined || focusFor(event, 0) === null) {
        return <>{children}</>;
    }

    const name = event.callsign === '' ? event.uid : event.callsign;

    return (
        <button
            type='button'
            style={styles.showButton}
            onClick={() => onShow(event)}
            aria-label={`Show ${name} on the map`}
            data-testid='cot-event-show'
        >
            {children}
        </button>
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
function EventDetail({event}: {event: CotEvent}) {
    const window = validFor(event);

    // The chat class carries the message above these rows, so drawing Remarks
    // here as well would put the same string on the card twice.
    const showRemarks = event.remarks !== '' && chatReading(event) === null;

    return (
        <>
            {event.positionNote !== '' && <p style={styles.note}>{event.positionNote}</p>}

            <dl
                style={styles.rows}
                role='group'
                aria-label={`Details of the Cursor on Target event ${event.callsign === '' ? event.uid : event.callsign}`}
            >
                {event.lat !== '' && <Row label='Position'><PositionValue event={event}/></Row>}
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
                {event.stale !== '' && (
                    <Row label='Stale'>
                        <TimeValue
                            reading={event.stale}
                            query={event.staleQuery}
                        />
                        {window !== '' && ` (valid for ${window})`}
                    </Row>
                )}
                {event.group !== '' && (
                    <Row label='Team'>
                        {event.group}
                        {event.role !== '' && `, ${event.role}`}
                    </Row>
                )}
                {showRemarks && <Row label='Remarks'>{event.remarks}</Row>}
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
function EventList({events, onShow}: {events: readonly CotEvent[]; onShow: (event: CotEvent) => void}) {
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
                    <ShowOnMap
                        event={event}
                        onShow={onShow}
                    >
                        <Naming event={event}/>
                    </ShowOnMap>
                </li>
            ))}
        </ul>
    );
}

export const CotCard: React.FC<Props> = ({payload, compactDisplay}) => {
    const {events} = payload;
    const only = events.length === 1 ? events[0] : undefined;
    const [focus, setFocus] = useState<MapFocus | undefined>(undefined);
    const clicks = useRef(0);

    const show = (event: CotEvent) => {
        clicks.current += 1;
        const next = focusFor(event, clicks.current);
        if (next !== null) {
            setFocus(next);
        }
    };

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

                <ErrorBoundary fallback={<p style={styles.note}>{DETAIL_FAILED}</p>}>
                    {only && (
                        <div style={styles.header}>
                            <ShowOnMap
                                event={only}
                                onShow={compactDisplay ? undefined : show}
                            >
                                <Naming event={only}/>
                            </ShowOnMap>
                        </div>
                    )}
                    {only && <ClassSummary event={only}/>}
                    {only && <Degraded event={only}/>}
                </ErrorBoundary>

                {!compactDisplay && events.some(isLinkable) && (
                    <ErrorBoundary>
                        <CotMap
                            events={events}
                            surface='card'
                            postId={payload.postId}
                            focus={focus}
                        />
                    </ErrorBoundary>
                )}

                <ErrorBoundary fallback={<p style={styles.note}>{DETAIL_FAILED}</p>}>
                    {only ? (
                        <EventDetail event={only}/>
                    ) : (
                        <EventList
                            events={events}
                            onShow={show}
                        />
                    )}
                </ErrorBoundary>

                <div style={styles.actions}>
                    <button
                        type='button'
                        style={styles.button}
                        onClick={() => showCotEvent(payload)}
                    >
                        {'Open details'}
                    </button>
                </div>
            </div>

            {payload.trail !== '' && <div style={styles.text}>{payload.trail}</div>}
        </div>
    );
};

export default CotCard;
