import React from 'react';

/**
 * The pin color from the plugin mark. Must match assets/icon.svg, which a spec
 * enforces: the two are the same mark and a reader sees both.
 */
const PIN_COLOR = '#FF6A13';

/**
 * The channel header button that opens the sidebar.
 *
 * The plugin mark from assets/icon.svg without its plate: a dropped pin inside
 * a compass, at the same proportions. The pin's hole is punched with a fill
 * rule rather than drawn in the plate color, because here there is no plate
 * and whatever is behind the button has to show through.
 *
 * The pin keeps its color and the compass takes the theme's, which is the
 * only combination that survives every header. A compass in the mark's own
 * bone gray disappears against a light header, and a wholly monochrome mark
 * gives up the one thing that makes it recognizable at this size.
 */
export const HeaderIcon = () => (
    <svg
        width='24'
        height='24'
        viewBox='0 0 24 24'
        fill='none'
        aria-hidden='true'
    >
        <circle
            cx='12'
            cy='12'
            r='7.9'
            stroke='currentColor'
            strokeWidth='1.5'
        />
        <g fill='currentColor'>
            <path d='M12 1.5 13.5 4.1h-3Z'/>
            <path d='M12 22.5 10.5 19.9h3Z'/>
            <path d='M1.5 12 4.1 10.5v3Z'/>
            <path d='M22.5 12 19.9 13.5v-3Z'/>
        </g>
        <path
            fillRule='evenodd'
            clipRule='evenodd'
            fill={PIN_COLOR}
            d='M8.44 9.94a3.56 3.56 0 1 1 7.12 0c0 2.62-2.43 5.44-3.56 8.06-1.13-2.62-3.56-5.44-3.56-8.06Zm3.56-1.43a1.43 1.43 0 1 0 0 2.86 1.43 1.43 0 0 0 0-2.86Z'
        />
    </svg>
);
