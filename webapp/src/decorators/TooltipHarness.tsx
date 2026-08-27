import React, {useState} from 'react';

import {register, _resetForTesting as resetRegistry} from './registry';
import {installDecoratorStyles} from './styles';
import {DecoratorTooltip} from './Tooltip';
import type {Decorator} from './types';

const HoverCard: React.FC<{payload: {value: string}}> = ({payload}) => (
    <div data-testid='fixture-hover'>{payload.value}</div>
);

/**
 * A hover that decides at render that it has nothing to show.
 *
 * Different from declaring no Hover at all, which the registry can see. This one
 * is registered, so the tooltip builds its chrome and only then finds it empty,
 * which is the case the stylesheet's `:empty` rule exists for. The location card
 * does exactly this when an admin turns the panel map off.
 */
const EmptyHover: React.FC<{payload: {value: string}}> = () => null;

const Panel: React.FC<{payload: {value: string}}> = ({payload}) => (
    <div>{payload.value}</div>
);

function hoverFor(withHover: boolean, emptyHover: boolean): Decorator<{value: string}>['Hover'] {
    if (!withHover) {
        return undefined;
    }

    return emptyHover ? EmptyHover : HoverCard;
}

function fixture(type: string, withHover: boolean, emptyHover: boolean): Decorator<{value: string}> {
    return {
        type,
        fromParams: (params) => {
            const value = params.get('v');
            return value === null ? null : {value};
        },
        summary: (payload) => payload.value,
        style: {color: '#000', background: '#fff'},
        Panel,
        Hover: hoverFor(withHover, emptyHover),
    };
}

interface Props {

    /** Href to hand the tooltip, relative to the site root. */
    href: string;

    /** Whether the reader is pointing at the link. */
    show?: boolean;

    /** Whether the registered decorator declares a hover card. */
    withHover?: boolean;

    /** Whether that hover card renders nothing when it is asked to. */
    emptyHover?: boolean;
}

/**
 * Harness for the hover tooltip.
 *
 * Playwright runs the test body in Node but mounts the component in the
 * browser, so the registry has to be populated in here, driven by plain
 * serializable props.
 */
const TooltipHarness: React.FC<Props> = ({href, show = true, withHover = true, emptyHover = false}) => {
    useState(() => {
        resetRegistry();
        register(fixture('fix', withHover, emptyHover));

        // The chrome hides itself through the plugin's stylesheet rather than
        // through an inline style, since `:empty` is a selector and an inline
        // style has nowhere to put one. So the sheet has to be installed here
        // or the rule under test is simply not in the document.
        installDecoratorStyles();
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
