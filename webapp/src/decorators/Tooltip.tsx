import React from 'react';

import {get, parseDecoratorHref} from './registry';

/**
 * The card chrome, owned by the framework so every decorator's hover looks the
 * same. Decorators supply content, not a container.
 *
 * Mattermost provides no backdrop of its own, so without a background the card
 * renders transparent over the post behind it. Each variable carries a fallback
 * for the same reason: an unstyled card is worse than a slightly off-theme one.
 */
const style: React.CSSProperties = {
    padding: '10px 12px',

    // Wide enough for the location hover's map, which is the widest thing any
    // decorator puts in here. A max rather than a width, so the DTG countdown
    // still shrinks to the size of its own line.
    maxWidth: '360px',
    background: 'var(--center-channel-bg, #ffffff)',
    color: 'var(--center-channel-color, #3f4350)',
    border: '1px solid rgba(var(--center-channel-color-rgb, 63, 67, 80), 0.16)',
    borderRadius: '8px',
    boxShadow: '0 6px 16px rgba(0, 0, 0, 0.24)',
};

interface Props {

    /** The link being hovered. Mattermost offers every link, not just ours. */
    href?: string;

    /** Whether the reader is actually pointing at it right now. */
    show?: boolean;
}

/**
 * Routes a hover to the decorator that owns the link.
 *
 * Registered once for the whole plugin, so a decorator gets a hover card by
 * declaring one rather than by touching anything here. Renders nothing for a
 * link this bundle does not own, params it cannot use, or a decorator with no
 * hover of its own.
 */
export const DecoratorTooltip: React.FC<Props> = ({href, show}) => {
    if (!show || !href) {
        return null;
    }

    const parsed = parseDecoratorHref(href);
    if (!parsed) {
        return null;
    }

    const decorator = get(parsed.type);
    if (!decorator?.Hover) {
        return null;
    }

    const payload = decorator.fromParams(parsed.params);
    if (payload === null || payload === undefined) {
        return null;
    }

    const {Hover} = decorator;
    return (
        <div style={style}>
            <Hover payload={payload}/>
        </div>
    );
};

export default DecoratorTooltip;
