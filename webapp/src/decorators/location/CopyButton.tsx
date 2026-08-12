import React, {useCallback, useEffect, useRef, useState} from 'react';

/**
 * Whether this browser can write to the clipboard at all.
 *
 * `navigator.clipboard` is undefined on a non-secure origin, which for an
 * on-prem or air-gapped Mattermost served over plain HTTP is the deployment
 * norm rather than an edge case. The button is hidden rather than offered and
 * then failing silently; every value it would have copied stays selectable.
 */
export function clipboardAvailable(): boolean {
    return typeof navigator !== 'undefined' && Boolean(navigator.clipboard?.writeText);
}

const styles: Record<string, React.CSSProperties> = {
    button: {
        display: 'inline-flex',
        alignItems: 'center',
        justifyContent: 'center',
        width: '24px',
        height: '24px',
        padding: 0,
        border: 'none',
        borderRadius: '4px',
        background: 'transparent',
        color: 'var(--center-channel-color)',
        cursor: 'pointer',

        // Dim until the row is hovered or the button is focused, so a column of
        // these does not compete with the values it sits beside.
        opacity: 0.5,
    },
    active: {opacity: 1, background: 'rgba(var(--center-channel-color-rgb), 0.08)'},
    status: {
        position: 'absolute',
        width: '1px',
        height: '1px',
        overflow: 'hidden',
        clip: 'rect(0 0 0 0)',
        whiteSpace: 'nowrap',
    },
};

/** How long the copied acknowledgement stays on the button. */
const COPIED_MS = 1500;

const CopyGlyph: React.FC = () => (
    <svg
        width='14'
        height='14'
        viewBox='0 0 24 24'
        fill='none'
        stroke='currentColor'
        strokeWidth='2'
        strokeLinecap='round'
        strokeLinejoin='round'
        aria-hidden='true'
        focusable='false'
    >
        <rect
            x='9'
            y='9'
            width='11'
            height='11'
            rx='2'
        />
        <path d='M5 15V5a2 2 0 0 1 2-2h10'/>
    </svg>
);

const CheckGlyph: React.FC = () => (
    <svg
        width='14'
        height='14'
        viewBox='0 0 24 24'
        fill='none'
        stroke='currentColor'
        strokeWidth='2.5'
        strokeLinecap='round'
        strokeLinejoin='round'
        aria-hidden='true'
        focusable='false'
    >
        <path d='M20 6 9 17l-5-5'/>
    </svg>
);

interface Props {

    /**
     * Names what is being copied, for assistive technology.
     *
     * The button shows an icon and no text, so this is its entire accessible
     * name. It stays constant across the copied state: a name that changed to
     * "Copied" would move the button out from under a screen reader mid-read,
     * which is what the status region below is for instead.
     */
    label: string;
    value: string;
}

/**
 * A copy control that sits beside the value it copies.
 *
 * Icon rather than a labelled button because there is one per row: a column of
 * "Copy MGRS", "Copy lat/lon", "Copy DMS" would be wider than the coordinates
 * and would read as the point of the panel rather than as an affordance on it.
 */
const CopyButton: React.FC<Props> = ({label, value}) => {
    const [copied, setCopied] = useState(false);
    const [active, setActive] = useState(false);
    const timer = useRef<ReturnType<typeof setTimeout> | null>(null);

    // Bumped whenever the value changes or the component goes away, so a
    // clipboard write still in flight can tell that it is stale.
    const generation = useRef(0);

    const clear = useCallback(() => {
        if (timer.current !== null) {
            clearTimeout(timer.current);
            timer.current = null;
        }
    }, []);

    // The timer must not outlive the component, and it must not survive a
    // change of coordinate either: the sidebar keeps this mounted when the
    // reader clicks a second link, so a stale "Copied" would sit under a value
    // that was never copied.
    useEffect(() => {
        setCopied(false);
        clear();

        return () => {
            generation.current += 1;
            clear();
        };
    }, [value, clear]);

    const onClick = useCallback(() => {
        // The write is a promise, and the effect above cannot cancel one that
        // is already in flight. Without this check, clicking Copy and then
        // clicking a second coordinate leaves "Copied" showing under a value
        // that was never copied, and schedules a timer on a component the
        // cleanup has already run for.
        const mine = generation.current;

        navigator.clipboard.writeText(value).then(() => {
            if (generation.current !== mine) {
                return;
            }

            setCopied(true);
            clear();
            timer.current = setTimeout(() => setCopied(false), COPIED_MS);
        }, () => {
            // A rejected write is the browser refusing, not something the
            // reader can act on. Say nothing and leave the value selectable.
        });
    }, [value, clear]);

    if (!clipboardAvailable()) {
        return null;
    }

    return (
        <>
            <button
                type='button'
                aria-label={label}
                title={label}
                style={active || copied ? {...styles.button, ...styles.active} : styles.button}
                onClick={onClick}
                onMouseEnter={() => setActive(true)}
                onMouseLeave={() => setActive(false)}
                onFocus={() => setActive(true)}
                onBlur={() => setActive(false)}
            >
                {copied ? <CheckGlyph/> : <CopyGlyph/>}
            </button>
            <span
                role='status'
                aria-live='polite'
                style={styles.status}
            >
                {copied ? `${label}: copied` : ''}
            </span>
        </>
    );
};

export default CopyButton;
