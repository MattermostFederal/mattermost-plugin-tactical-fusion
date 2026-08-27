import React, {useState} from 'react';

import {RhsTitle, RhsView} from './RhsView';

import {register, _resetForTesting as resetRegistry} from '../../decorators/registry';
import {setSelection, _resetForTesting as resetSelection} from '../../decorators/selection';
import type {Decorator} from '../../decorators/types';

/**
 * Harness for the RHS components.
 *
 * Playwright runs the test body in Node but mounts the component in the
 * browser, so registry and selection state set from a test file would never
 * reach the component. Everything therefore has to be set up in here, driven by
 * plain serializable props.
 */
const FixturePanel: React.FC<{payload: {value: string}}> = ({payload}) => (
    <div data-testid='fixture-panel'>{payload.value}</div>
);

const FixtureTitle: React.FC<{payload: {value: string}}> = ({payload}) => (
    <span data-testid='fixture-title'>{`Titled ${payload.value}`}</span>
);

function fixture(withTitle: boolean): Decorator<{value: string}> {
    return {
        type: 'fix',
        fromParams: () => ({value: 'x'}),
        summary: (payload) => `Fixture ${payload.value}`,
        style: {color: '#000', background: '#fff'},
        Panel: FixturePanel,
        Title: withTitle ? FixtureTitle : undefined,
    };
}

interface Props {

    /** Selection type to install, or omitted for no selection at all. */
    selectionType?: string;

    /** Payload value for the fixture panel. */
    value?: string;

    /** Render the title instead of the body. */
    title?: boolean;

    /** Whether the registered decorator declares its own header component. */
    withTitle?: boolean;
}

const RhsHarness: React.FC<Props> = ({selectionType, value = 'hello', title, withTitle = false}) => {
    // Set up once, before the first render, so the component never observes an
    // empty registry.
    useState(() => {
        resetRegistry();
        resetSelection();
        register(fixture(withTitle));
        if (selectionType) {
            setSelection({type: selectionType, payload: {value}});
        }
        return null;
    });

    return title ? <RhsTitle/> : <RhsView/>;
};

export default RhsHarness;
