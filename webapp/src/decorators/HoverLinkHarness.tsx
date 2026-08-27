import React from 'react';

import HoverLink from './HoverLink';
import {_resetForTesting as resetDecorators} from './registry';

import {registerBuiltinDecorators} from './index';

/**
 * A decorator link inside a narrow column, which is the shape the sidebar is.
 *
 * The width is the point: an absolutely positioned card with `left: 0` and no
 * width of its own shrinks to fit its CONTAINING BLOCK, which is the anchor, so
 * the defect this exists for only appears beside a short link.
 */
const HoverLinkHarness: React.FC<{href: string; label: string; width?: number}> = ({
    href, label, width = 320,
}) => {
    resetDecorators();
    registerBuiltinDecorators();

    return (
        <div
            data-testid='column'
            style={{width, padding: 16, paddingBottom: 90, fontFamily: 'sans-serif'}}
        >
            <div>{'Time'}</div>
            <HoverLink href={href}>{label}</HoverLink>
        </div>
    );
};

export default HoverLinkHarness;
