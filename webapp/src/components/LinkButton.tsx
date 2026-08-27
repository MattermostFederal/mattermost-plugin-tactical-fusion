import React, {forwardRef, useState} from 'react';

/**
 * The resting look: Mattermost's link color, no underline until pointed at.
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

    /** What the link does. Ignored when `href` is set. */
    onClick?: () => void;

    /**
     * Where the link goes, for the one case that is a real destination. Renders
     * an anchor instead of a button.
     */
    href?: string;

    /** Grayed out and inert while something is in flight. Buttons only. */
    disabled?: boolean;

    /** Placement and type size. The link coloring is not overridable. */
    style?: React.CSSProperties;

    children: React.ReactNode;
}

/**
 * Something that looks and behaves like a Mattermost link.
 *
 * Renders a button by default, because most of these go nowhere the browser
 * knows about: they swap what the sidebar is showing. A button keeps that
 * keyboard operable and announced correctly without pretending to be a URL.
 *
 * Pass `href` for the exception, a link to somewhere that really is a page. It
 * opens in a new tab, since the sidebar sits inside the app and navigating it
 * away would lose the reader's place in the channel.
 *
 * One component rather than two so the hover and focus underline, which only
 * exists because inline styles cannot express `:hover`, is defined once.
 *
 * Forwards a ref to the button, because a panel that swaps one view for another
 * has to put focus somewhere: without it the activated control unmounts and
 * focus falls to the body.
 */
const LinkButton = forwardRef<HTMLButtonElement, Props>(({onClick, href, disabled, style, children}, ref) => {
    const [pointed, setPointed] = useState(false);

    const shared = {
        onMouseEnter: () => setPointed(true),
        onMouseLeave: () => setPointed(false),
        onFocus: () => setPointed(true),
        onBlur: () => setPointed(false),
        style: {...base, ...style, textDecoration: pointed ? 'underline' : 'none'},
    };

    if (href !== undefined) {
        return (
            <a
                href={href}
                target='_blank'
                rel='noopener noreferrer'
                {...shared}
            >{children}</a>
        );
    }

    return (
        <button
            ref={ref}
            type='button'
            disabled={disabled}
            onClick={onClick}
            {...shared}
        >{children}</button>
    );
});

LinkButton.displayName = 'LinkButton';

export default LinkButton;
