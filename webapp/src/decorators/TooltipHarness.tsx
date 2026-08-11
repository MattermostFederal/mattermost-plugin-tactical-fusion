import React, {useState} from 'react';

import {register, _resetForTesting as resetRegistry} from './registry';
import {DecoratorTooltip} from './Tooltip';
import type {Decorator} from './types';

const HoverCard: React.FC<{payload: {value: string}}> = ({payload}) => (
    <div data-testid='fixture-hover'>{payload.value}</div>
);

const Panel: React.FC<{payload: {value: string}}> = ({payload}) => (
    <div>{payload.value}</div>
);

function fixture(type: string, withHover: boolean): Decorator<{value: string}> {
    return {
        type,
        fromParams: (params) => {
            const value = params.get('v');
            return value === null ? null : {value};
        },
        summary: (payload) => payload.value,
        style: {color: '#000', background: '#fff'},
        Panel,
        Hover: withHover ? HoverCard : undefined,
    };
}

interface Props {

    /** Href to hand the tooltip, relative to the site root. */
    href: string;

    /** Whether the reader is pointing at the link. */
    show?: boolean;

    /** Whether the registered decorator declares a hover card. */
    withHover?: boolean;
}

/**
 * Harness for the hover tooltip.
 *
 * Playwright runs the test body in Node but mounts the component in the
 * browser, so the registry has to be populated in here, driven by plain
 * serialisable props.
 */
const TooltipHarness: React.FC<Props> = ({href, show = true, withHover = true}) => {
    useState(() => {
        resetRegistry();
        register(fixture('fix', withHover));
        return null;
    });

    return (
        <DecoratorTooltip
            href={href}
            show={show}
        />
    );
};

export default TooltipHarness;
