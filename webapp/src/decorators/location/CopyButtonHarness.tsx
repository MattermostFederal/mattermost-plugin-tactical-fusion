import React, {useState} from 'react';

import CopyButton from './CopyButton';

/**
 * Harness for the copy button.
 *
 * The clipboard is stubbed at module scope, before any component renders,
 * because `clipboardAvailable()` is read during render and Playwright's
 * `addInitScript` lands too late for a component test.
 *
 * The write is deliberately *deferred*: the test decides when it resolves,
 * which is the only way to make the ordering visible. The bug this exists for
 * is a write that settles after the reader has already moved to a different
 * coordinate.
 */
let pending: (() => void) | null = null;
let refuse: (() => void) | null = null;
let available = true;

Object.defineProperty(navigator, 'clipboard', {
    configurable: true,
    get: () => (available ? {
        writeText: () => new Promise<void>((resolve, reject) => {
            pending = resolve;
            refuse = () => reject(new Error('NotAllowedError'));
        }),
    } : undefined),
});

export const FIRST = '34.0561° N, 118.2500° W';
export const SECOND = '51.5074° N, 0.1278° W';

const CopyButtonHarness: React.FC<{clipboard?: 'yes' | 'no'}> = ({clipboard = 'yes'}) => {
    available = clipboard === 'yes';

    const [value, setValue] = useState(FIRST);

    return (
        <div>
            <CopyButton
                label='Copy lat/lon'
                value={value}
            />
            <button
                type='button'
                onClick={() => setValue(SECOND)}
            >
                {'switch coordinate'}
            </button>
            <button
                type='button'
                onClick={() => pending?.()}
            >
                {'resolve write'}
            </button>
            <button
                type='button'
                onClick={() => refuse?.()}
            >
                {'refuse write'}
            </button>
        </div>
    );
};

export default CopyButtonHarness;
