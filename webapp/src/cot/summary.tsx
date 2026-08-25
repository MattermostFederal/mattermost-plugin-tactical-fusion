import React from 'react';

import type {CotEvent} from './types';

/**
 * How much of a line a class may claim on the card.
 *
 * The card's whole discipline is that the rows are the answer and the panel is
 * where you go to check it, so a class adds one line and nothing moves. A cap
 * in runes is what makes "one line" reviewable rather than a hope: past this the
 * trailing readings are dropped, and the panel still carries all of them.
 */
export const SUMMARY_MAX_RUNES = 90;

const styles: Record<string, React.CSSProperties> = {
    summary: {margin: 0, padding: '0 12px 8px', opacity: 0.95},
    label: {opacity: 0.8},
    message: {margin: 0, padding: '0 12px 8px', whiteSpace: 'pre-wrap'},
};

/**
 * Joins readings into one line, dropping from the end until it fits.
 *
 * Dropping whole readings rather than clipping the string keeps every figure on
 * the card a complete one. A clipped "2 urgen" is worse than an absent reading.
 */
export const TOO_LONG = 'stated, but too long to show here. Open details.';

export function oneLine(parts: string[]): string {
    const kept = parts.filter((part) => part !== '');
    if (kept.length === 0) {
        return '';
    }

    while (kept.length > 1 && tooWide(kept.join(' · '))) {
        kept.pop();
    }

    // Never blank when there were readings, and never a clipped figure. Popping
    // to empty made the whole line vanish; clipping put "2 urgen" under a
    // casualty label, which is the worse of the two and is the rule this file
    // states. A reading that cannot be shown honestly points at the panel.
    const line = kept.join(' · ');
    return tooWide(line) ? TOO_LONG : line;
}

function tooWide(line: string): boolean {
    return [...line].length > SUMMARY_MAX_RUNES;
}

/**
 * The sender and room a chat event STATES, or null when it stated neither.
 *
 * Null is what makes the class degrade: a `b-t-f` carrying no `__chat` has no
 * sender and no room, so the chat layout would be empty chrome and the card
 * falls back to the ordinary one.
 */
export function chatReading(event: CotEvent): string | null {
    if (event.cotClass !== 'chat') {
        return null;
    }

    const {chatSender, chatRoom} = event.detail;
    if (chatSender === '' && chatRoom === '') {
        return null;
    }

    const sender = chatSender === '' ? 'a sender it did not name' : chatSender;
    const reading = chatRoom === '' ? sender : `${sender} to ${chatRoom}`;
    return tooWide(reading) ? TOO_LONG : reading;
}

/**
 * The chat line, which is a reading of an event and not a message.
 *
 * senderCallsign is author-chosen text with no relationship to the Mattermost
 * identity that posted, so anything shaped like a quoted message from a named
 * person would borrow Mattermost's own attribution and hand a reader a message
 * from somebody who never sent one. The label is what says whose word this is,
 * and there is no blockquote, no avatar and no username styling.
 */
function ChatSummary({event}: {event: CotEvent}) {
    const reading = chatReading(event);
    if (reading === null) {
        return null;
    }

    return (
        <>
            <p style={styles.summary}>
                <span style={styles.label}>{'Event states sender: '}</span>
                {reading}
            </p>
            {event.remarks !== '' && <p style={styles.message}>{event.remarks}</p>}
        </>
    );
}

function MedevacSummary({event}: {event: CotEvent}) {
    const {medevacUrgent, medevacPriority, medevacRoutine, medevacLitter, medevacAmbulatory} = event.detail;

    const counts = [
        medevacUrgent === '' ? '' : `${medevacUrgent} urgent`,
        medevacPriority === '' ? '' : `${medevacPriority} priority`,
        medevacRoutine === '' ? '' : `${medevacRoutine} routine`,
        medevacLitter === '' ? '' : `${medevacLitter} litter`,
        medevacAmbulatory === '' ? '' : `${medevacAmbulatory} ambulatory`,
    ];

    const line = oneLine(counts);
    if (line === '') {
        return null;
    }

    return (
        <p style={styles.summary}>
            <span style={styles.label}>{'Patients stated: '}</span>
            {line}
        </p>
    );
}

function SensorSummary({event}: {event: CotEvent}) {
    const {sensorFov, sensorAzimuth, sensorRange, sensorModel} = event.detail;

    const line = oneLine([
        sensorFov === '' ? '' : `field of view ${sensorFov}`,
        sensorAzimuth === '' ? '' : `azimuth ${sensorAzimuth}`,
        sensorRange === '' ? '' : `range ${sensorRange}`,
        sensorModel,
    ]);

    if (line === '') {
        return null;
    }

    return (
        <p style={styles.summary}>
            <span style={styles.label}>{'Sensor: '}</span>
            {line}
        </p>
    );
}

/**
 * That a stream exists, and never where it is.
 *
 * The address is author-controlled and stays off the card entirely. It is text
 * in the panel, under its own heading, and it is never an anchor.
 */
function VideoSummary({event}: {event: CotEvent}) {
    const {videoUrl, videoConnAddress, videoUid} = event.detail;
    if (videoUrl === '' && videoConnAddress === '' && videoUid === '') {
        return null;
    }

    return (
        <p style={styles.summary}>
            <span style={styles.label}>{'Video: '}</span>
            {'a stream is associated with this event. The address is in the sidebar.'}
        </p>
    );
}

/**
 * One line, chosen by the class, degrading to nothing when the block the class
 * names is absent.
 */
export const ClassSummary: React.FC<{event: CotEvent}> = ({event}) => {
    switch (event.cotClass) {
    case 'chat':
        return <ChatSummary event={event}/>;
    case 'medevac':
        return <MedevacSummary event={event}/>;
    case 'sensor':
        return <SensorSummary event={event}/>;
    case 'video':
        return <VideoSummary event={event}/>;
    default:
        return null;
    }
};
