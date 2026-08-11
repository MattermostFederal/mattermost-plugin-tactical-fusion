import React, {useRef, useState} from 'react';

import {register, _resetForTesting as resetRegistry} from './registry';
import {installDecoratorStyles} from './styles';
import type {Decorator} from './types';

/**
 * Harness for the stylesheet's document wiring.
 *
 * `buildDecoratorStyles` is pure and already covered without a browser in
 * styles.spec.ts. What needs a real page is the half that touches the document:
 * whether the element lands in the head, whether the disposer takes it away
 * again, and whether a second install leaves the first one's element alone.
 *
 * Install and dispose are driven from buttons rather than an effect, so a test
 * can order them itself and observe the document between each step.
 */
const fixture: Decorator<unknown> = {
    type: 'fix',
    fromParams: () => ({}),
    summary: () => 'fix',
    style: {color: '#ff0000', background: '#00ff00'},
    Panel: (() => null) as unknown as Decorator<unknown>['Panel'],
};

const StylesHarness: React.FC = () => {
    const disposers = useRef<Array<() => void>>([]);
    const [installs, setInstalls] = useState(0);

    return (
        <div>
            <button
                data-testid='reset'
                onClick={() => {
                    resetRegistry();
                    register(fixture);
                    setInstalls(0);
                }}
            >{'reset'}</button>

            <button
                data-testid='install'
                onClick={() => {
                    disposers.current.push(installDecoratorStyles());
                    setInstalls((count) => count + 1);
                }}
            >{'install'}</button>

            {/* Runs only the most recent disposer, so a test can prove the
                second install's disposer is the inert one. */}
            <button
                data-testid='dispose-last'
                onClick={() => {
                    disposers.current.pop()?.();
                }}
            >{'dispose last'}</button>

            <button
                data-testid='dispose-all'
                onClick={() => {
                    disposers.current.forEach((dispose) => dispose());
                    disposers.current = [];
                }}
            >{'dispose all'}</button>

            <p data-testid='installs'>{String(installs)}</p>
        </div>
    );
};

export default StylesHarness;
