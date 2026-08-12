import React, {useEffect, useRef, useState} from 'react';

import {installDecoratorClickHandler} from './click_handler';
import {decoratePathPrefix, register, _resetForTesting as resetRegistry} from './registry';
import {
    getSelection,
    subscribe,
    _resetForTesting as resetSelection,
} from './selection';
import type {Decorator} from './types';

/**
 * Harness for the click handler's DOM wiring.
 *
 * The listener attaches to document, so it needs a real page. Everything it
 * decides is surfaced as text nodes the test can assert on, which keeps the
 * test free of DOM stubbing.
 */
const fixture: Decorator<{value: string}> = {
    type: 'fix',
    fromParams: (params) => {
        const value = params.get('v');
        return value === null ? null : {value};
    },
    summary: (payload) => payload.value,
    style: {color: '#000', background: '#fff'},
    Panel: (() => null) as unknown as Decorator<{value: string}>['Panel'],
};

const ClickHarness: React.FC<{installTwice?: boolean; centerChannelBg?: string}> = ({
    installTwice,
    centerChannelBg,
}) => {
    const [selection, setSelectionText] = useState('none');
    const [prevented, setPrevented] = useState(false);
    const [dispatchCount, setDispatchCount] = useState(0);
    const disposers = useRef<Array<() => void>>([]);

    useEffect(() => {
        // Stand in for the theme variable Mattermost sets on the root element.
        if (centerChannelBg) {
            document.documentElement.style.setProperty('--center-channel-bg', centerChannelBg);
        }

        resetRegistry();
        resetSelection();
        register(fixture);

        const unsubscribe = subscribe((next) => {
            setSelectionText(next ? `${next.type}:${(next.payload as {value: string}).value}` : 'none');
            setDispatchCount((count) => count + 1);
        });

        disposers.current.push(installDecoratorClickHandler());
        if (installTwice) {
            disposers.current.push(installDecoratorClickHandler());
        }

        // Both of these run on the capture phase and are registered after the
        // handler, so they fire after it on the same node. The bubble phase is
        // no good here because the handler calls stopPropagation when it takes
        // a click, so a bubble listener would never see the handled case.
        const observer = (event: MouseEvent) => setPrevented(event.defaultPrevented);
        document.addEventListener('click', observer, true);

        // Stop the browser from actually following the links this harness
        // deliberately leaves un-intercepted, which would navigate the test
        // page away from the mounted component. Registered last so it cannot
        // affect what the observer recorded.
        const blockNavigation = (event: MouseEvent) => event.preventDefault();
        document.addEventListener('click', blockNavigation, true);

        return () => {
            unsubscribe();
            document.removeEventListener('click', observer, true);
            document.removeEventListener('click', blockNavigation, true);
            disposers.current.forEach((dispose) => dispose());
            disposers.current = [];
            document.documentElement.style.removeProperty('--center-channel-bg');
        };
    }, [installTwice, centerChannelBg]);

    const prefix = decoratePathPrefix();

    return (
        <div>
            <a
                data-testid='decorator-link'
                href={`${prefix}fix?v=hello`}
            >{'decorator'}</a>
            <a
                data-testid='unknown-type-link'
                href={`${prefix}nope?v=hello`}
            >{'unknown'}</a>
            <a
                data-testid='bad-params-link'
                href={`${prefix}fix?other=1`}
            >{'bad params'}</a>
            <a
                data-testid='force-page-link'
                href={`${prefix}fix?v=hello&_page=1`}

                // What Mattermost puts on a rendered link, and what has to come
                // off for the shared target to be honored.
                rel='noopener noreferrer'
            >{'force page'}</a>
            <a
                data-testid='second-force-page-link'
                href={`${prefix}fix?v=second&_page=1`}
            >{'another page'}</a>
            <a
                data-testid='rel-keeps-others-link'
                href={`${prefix}fix?v=third&_page=1`}
                rel='noopener nofollow noreferrer'
            >{'page with other rel'}</a>
            <a
                data-testid='other-link'
                href='/some/other/place'
            >{'other'}</a>

            {/*
              * Somebody else's host wearing our path. Anything the handler
              * accepts is trusted afterwards, and on the _page branch that
              * means a named window with noopener and noreferrer stripped.
              */}
            <a
                data-testid='cross-origin-link'
                href={`https://evil.example${prefix}fix?v=hello`}
            >{'cross origin'}</a>
            <a
                data-testid='cross-origin-page-link'
                href={`https://evil.example${prefix}fix?v=hello&_page=1`}
                rel='noopener noreferrer'
            >{'cross origin page'}</a>

            <button
                data-testid='dispose'
                onClick={() => {
                    disposers.current.forEach((dispose) => dispose());
                    disposers.current = [];
                }}
            >{'dispose'}</button>

            <p data-testid='selection'>{selection}</p>
            <p data-testid='default-prevented'>{String(prevented)}</p>
            <p data-testid='dispatch-count'>{String(dispatchCount)}</p>
            <p data-testid='raw-selection'>{JSON.stringify(getSelection())}</p>
        </div>
    );
};

export default ClickHarness;
