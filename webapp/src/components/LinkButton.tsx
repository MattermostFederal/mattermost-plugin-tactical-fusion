import React, {useState} from 'react';

/**
 * The resting look: Mattermost's link colour, no underline until pointed at.
 *
 * Everything in this plugin styles itself inline, which has nowhere to declare
 * a `:hover` rule, so the underline is driven from React state instead. Focus
 * counts as well as hover, or the underline would be invisible to somebody
 * moving through the panel by keyboard.
 */
const base: React.CSSProperties = {
    background: 'none',
    border: 'none',
    padding: 0,
    font: 'inherit',
    color: 'var(--link-color, #386fe5)',
    cursor: 'pointer',
};

interface Props {

    /** What the link does. */
    onClick: () => void;

    /** Greyed out and inert while something is in flight. */
    disabled?: boolean;

    /** Placement and type size. The link colouring is not overridable. */
    style?: React.CSSProperties;

    children: React.ReactNode;
}

/**
 * A button that looks and behaves like a Mattermost link.
 *
 * A button rather than an anchor because it goes nowhere the browser knows
 * about: it swaps what the sidebar is showing. That keeps it keyboard
 * operable and announced correctly without pretending to be a URL.
 */
const LinkButton: React.FC<Props> = ({onClick, disabled, style, children}) => {
    const [pointed, setPointed] = useState(false);

    return (
        <button
            type='button'
            disabled={disabled}
            onClick={onClick}
            onMouseEnter={() => setPointed(true)}
            onMouseLeave={() => setPointed(false)}
            onFocus={() => setPointed(true)}
            onBlur={() => setPointed(false)}
            style={{...base, ...style, textDecoration: pointed ? 'underline' : 'none'}}
        >{children}</button>
    );
};

export default LinkButton;
