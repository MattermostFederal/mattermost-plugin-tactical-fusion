import React, {useState} from 'react';

import {POST_PROPS_KEY, POST_PROPS_VERSION} from './post_props';
import {DecoratorPostBody} from './PostBody';
import {register, _resetForTesting as resetRegistry} from './registry';
import type {Decorator} from './types';

const InlineView: React.FC<{payload: {value: string}}> = ({payload}) => (
    <div data-testid='fixture-inline'>{payload.value}</div>
);

const Throwing: React.FC<{payload: {value: string}}> = () => {
    throw new Error('the inline view exploded');
};

const Panel: React.FC<{payload: {value: string}}> = ({payload}) => (
    <div>{payload.value}</div>
);

/**
 * `empty` is an Inline that is DECLARED and renders nothing, which is different
 * from `none` (no Inline declared at all) and is the steady state of every post
 * stamped before an admin turned the inline map off. Post.Type survives an edit
 * and nothing un-stamps it, so those posts keep routing here forever.
 */
type Kind = 'inline' | 'throwing' | 'none' | 'empty';

const EmptyInline: React.FC<{payload: {value: string}}> = () => null;

function fixture(kind: Kind): Decorator<{value: string}> {
    let Inline: Decorator<{value: string}>['Inline'];
    if (kind === 'inline') {
        Inline = InlineView;
    } else if (kind === 'throwing') {
        Inline = Throwing;
    } else if (kind === 'empty') {
        Inline = EmptyInline;
    }

    return {
        type: 'fix',
        fromParams: (params) => {
            const value = params.get('v');
            return value === null ? null : {value};
        },
        summary: (payload) => payload.value,
        style: {color: '#000', background: '#fff'},
        Panel,
        postType: 'custom_fix',
        Inline,
    };
}

interface Props {
    message: string;

    /** The f and v the props claim, or null for a post carrying no props. */
    propsF?: string | null;
    propsV?: string | null;

    /** Overrides, for the two shapes that must be refused. */
    propsVersion?: number;
    propsType?: string;

    kind?: Kind;
    compactDisplay?: boolean;
}

/**
 * Harness for the post body.
 *
 * Playwright runs the test body in Node and mounts in the browser, so the
 * registry is populated in here and everything else arrives as serializable
 * props.
 */
const PostBodyHarness: React.FC<Props> = ({
    message,
    propsF = 'ddh',
    propsV = '34.0561N,118.2500W',
    propsVersion = POST_PROPS_VERSION,
    propsType = 'fix',
    kind = 'inline',
    compactDisplay,
}) => {
    useState(() => {
        resetRegistry();
        register(fixture(kind));
        return true;
    });

    let postProps: Record<string, unknown> | undefined;
    if (propsF !== null && propsV !== null) {
        postProps = {
            [POST_PROPS_KEY]: {
                version: propsVersion,
                type: propsType,
                f: propsF,
                v: propsV,
            },
        };
    }

    return (
        <div data-testid='post-body'>
            <DecoratorPostBody
                post={{message, props: postProps}}
                compactDisplay={compactDisplay}
            />
        </div>
    );
};

export default PostBodyHarness;
