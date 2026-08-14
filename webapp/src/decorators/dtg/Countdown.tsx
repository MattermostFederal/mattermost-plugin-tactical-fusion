import React, {useEffect, useState} from 'react';

import {formatRelative, isUrgent} from './relative';

interface Props {

    /** The instant being counted towards, or away from. */
    target: Date;

    /** Smaller type, for the hover card. */
    compact?: boolean;

    /**
     * How close counts as urgent. Omitted means the built-in threshold, which
     * is what the reader gets until their preferences have loaded.
     */
    urgentWithinMs?: number;
}

/**
 * The live countdown, shared by the sidebar panel and the hover card.
 *
 * One implementation on purpose: two copies would drift, and the whole point of
 * the hover is that it agrees with the panel behind it.
 */
const Countdown: React.FC<Props> = ({target, compact, urgentWithinMs}) => {
    const [now, setNow] = useState(() => new Date());
    const [pulseOn, setPulseOn] = useState(true);

    useEffect(() => {
        const interval = setInterval(() => setNow(new Date()), 1000);
        return () => clearInterval(interval);
    }, []);

    const urgent = isUrgent(now, target, urgentWithinMs);

    // Pulsed from a timer rather than a CSS animation, because this styles
    // itself inline and has nowhere to declare keyframes.
    //
    // This deliberately ignores prefers-reduced-motion: an imminent DTG is
    // operational information, and it was decided that it should draw the eye
    // for every reader. Do not "fix" this back without asking. It is kept to
    // roughly one pulse per second, well under the three per second that can
    // trigger photosensitivity, and the bar and color below still carry the
    // signal on their own for a reader who does not perceive the movement.
    useEffect(() => {
        if (!urgent) {
            setPulseOn(true);
            return undefined;
        }

        const interval = setInterval(() => setPulseOn((on) => !on), 600);
        return () => clearInterval(interval);
    }, [urgent]);

    const base: React.CSSProperties = {
        fontFamily: 'ui-monospace, SFMono-Regular, Menlo, monospace',
        fontSize: compact ? '16px' : '24px',
        fontWeight: 600,
        color: 'var(--center-channel-color)',
        margin: compact ? 0 : '0 0 6px',
    };

    const style: React.CSSProperties = urgent ? {
        ...base,
        color: 'var(--error-text, #d24b4e)',
        borderLeft: '4px solid var(--error-text, #d24b4e)',
        paddingLeft: '10px',
        opacity: pulseOn ? 1 : 0.35,
        transition: 'opacity 300ms ease-in-out',
    } : base;

    return (
        <p
            style={style}
            data-urgent={urgent ? 'true' : 'false'}
        >{formatRelative(now, target)}</p>
    );
};

export default Countdown;
