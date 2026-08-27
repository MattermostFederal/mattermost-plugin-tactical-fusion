import React, {useState} from 'react';

import {DecoratorHoverCard} from './Tooltip';

/**
 * A decorator link that carries its own hover card.
 *
 * `registerLinkTooltipComponent` is wired into Mattermost's own markdown link
 * rendering, so it only ever offers a link that renderer drew. A plugin that
 * owns a post body draws its own anchors, and nothing offers those, so a reader
 * pointing at one got no card at all. `docs/design/mapping.md` recorded that as
 * expected but unverified when the inline map shipped; it is verified now, and
 * this is the answer for a surface that needs the hover rather than merely
 * liking it.
 *
 * The chrome and the routing are the framework's: this decides only WHEN to show
 * a card, never what goes in one. A decorator still declares its hover once.
 */

const styles: Record<string, React.CSSProperties> = {
    anchor: {position: 'relative', display: 'inline-block'},
    card: {
        left: 0,
        position: 'absolute',
        top: '100%',

        // Sized by its own content, not by the link it hangs off.
        //
        // An absolutely positioned box with `left` and no width shrinks to fit
        // its CONTAINING BLOCK, which here is the anchor, so the card was as
        // wide as the token and wrapped "in 1h 29m 59s" onto two lines. The
        // card's own maxWidth still caps it.
        width: 'max-content',

        // Above the post it is drawn over. Mattermost's own post chrome sits
        // well below this, and the card is dismissed the moment the pointer
        // leaves, so it cannot strand anything underneath it.
        zIndex: 1000,

        // Display only, which is what keeps the card from stealing the pointer
        // and flickering itself in and out as the reader moves onto it. Every
        // decorator hover is a readout: the DTG countdown, and a location map
        // built with `preview`, which takes no gestures by design.
        pointerEvents: 'none',
    },
};

interface Props {
    href: string;
    children: React.ReactNode;
}

export const HoverLink: React.FC<Props> = ({href, children}) => {
    const [shown, setShown] = useState(false);

    const show = () => setShown(true);
    const hide = () => setShown(false);

    return (
        <span
            style={styles.anchor}
            onMouseEnter={show}
            onMouseLeave={hide}
        >
            <a
                href={href}

                // Focus and blur as well as the pointer, so the card is
                // reachable by keyboard rather than being a mouse-only affordance.
                onFocus={show}
                onBlur={hide}
            >{children}</a>
            {shown && (
                <span style={styles.card}>
                    <DecoratorHoverCard href={href}/>
                </span>
            )}
        </span>
    );
};

export default HoverLink;
