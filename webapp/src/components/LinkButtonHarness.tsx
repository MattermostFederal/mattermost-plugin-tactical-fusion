import React, {useEffect, useState} from 'react';

import LinkButton from './LinkButton';

/**
 * Harness for LinkButton.
 *
 * Exists to count clicks and to stop the anchor variant from actually opening a
 * tab: it carries `target='_blank'`, which a test cannot follow. Blocking
 * navigation from inside the harness keeps that out of every test.
 */
const LinkButtonHarness: React.FC<{
    href?: string;
    disabled?: boolean;
    withOnClick?: boolean;
    style?: React.CSSProperties;
}> = ({href, disabled, withOnClick = true, style}) => {
    const [clicks, setClicks] = useState(0);

    useEffect(() => {
        const blockNavigation = (event: MouseEvent) => event.preventDefault();
        document.addEventListener('click', blockNavigation);
        return () => document.removeEventListener('click', blockNavigation);
    }, []);

    return (
        <div>
            <LinkButton
                href={href}
                disabled={disabled}
                style={style}
                onClick={withOnClick ? () => setClicks((count) => count + 1) : undefined}
            >{'Customize your view'}</LinkButton>

            <p data-testid='clicks'>{String(clicks)}</p>

            {/* Somewhere to move focus and the pointer to. */}
            <button data-testid='elsewhere'>{'elsewhere'}</button>
        </div>
    );
};

export default LinkButtonHarness;
